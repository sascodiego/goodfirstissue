package triage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/config"
	"github.com/sascodiego/nan-hackathon-clef/internal/repo"
)

// The brief is the only thing the alignment node is given about the project, so these tests fix
// both halves: what it declares, what it has been building, and what happens when only one half
// can be read.

// fakeGit answers the git calls the brief makes, and records them.
type fakeGit struct {
	output []byte
	err    error
	dirs   []string
	args   [][]string
}

func (f *fakeGit) run(_ context.Context, dir string, args ...string) ([]byte, error) {
	f.dirs = append(f.dirs, dir)
	f.args = append(f.args, args)
	if f.err != nil {
		return nil, f.err
	}
	return f.output, nil
}

// cloneWith builds the layout production actually has: the clone of spf13/cobra lives in
// <Root>/spf13__cobra, never in <Root>/spf13/cobra. A fixture with the slash pins a layout no
// caller produces, and then the test agrees with a bug instead of catching it.
func cloneWith(t *testing.T, readme string) (string, *fakeGit) {
	t.Helper()
	root := t.TempDir()
	git := &fakeGit{output: []byte("abc1234 fix the parser\n def5678 add a flag\n")}
	writeClone(t, filepath.Join(root, "spf13__cobra"), readme)
	return root, git
}

func writeClone(t *testing.T, dir, readme string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if readme != "" {
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
			t.Fatalf("write readme: %v", err)
		}
	}
}

func briefWith(root string, git *fakeGit) CloneBrief {
	return CloneBrief{Root: root, Git: &repo.Cloner{Run: git.run}}
}

func TestTheBriefCarriesBothHalves(t *testing.T) {
	root, git := cloneWith(t, "Cobra is a CLI library.")

	got, err := briefWith(root, git).Brief(context.Background(), "spf13__cobra")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}

	for _, want := range []string{
		"# spf13__cobra",
		"## What the project says it is (README)",
		"Cobra is a CLI library.",
		"## What it has been building lately (last 20 commit subjects)",
		"abc1234 fix the parser",
		"def5678 add a flag",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the brief does not carry %q:\n%s", want, got)
		}
	}
}

// The revealed half is the one that cannot be missing: a project with no README still has a
// history, and that history is where its real direction lives.
func TestAMissingReadmeIsSaidOutLoud(t *testing.T) {
	root, git := cloneWith(t, "")

	got, err := briefWith(root, git).Brief(context.Background(), "spf13__cobra")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if !strings.Contains(got, noneMarker) {
		t.Errorf("a missing README was not said out loud:\n%s", got)
	}
	if !strings.Contains(got, "abc1234 fix the parser") {
		t.Error("the history did not reach the brief")
	}
}

// A README that eats the whole dossier budget would leave no room for the request, so it is cut
// and the cut is visible.
func TestALongReadmeIsCutAndSaidSo(t *testing.T) {
	root, git := cloneWith(t, strings.Repeat("filler ", maxBriefReadmeChars))

	got, err := briefWith(root, git).Brief(context.Background(), "spf13__cobra")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if !strings.Contains(got, briefTruncated) {
		t.Error("a truncated README does not say it was truncated")
	}
	if strings.Count(got, "filler") > maxBriefReadmeChars {
		t.Error("the README was not cut at all")
	}
}

func TestAGitFailureIsAnError(t *testing.T) {
	root, git := cloneWith(t, "README")
	git.err = errors.New("not a repository")

	if _, err := briefWith(root, git).Brief(context.Background(), "spf13__cobra"); err == nil {
		t.Fatal("a brief with no readable history was reported as a success")
	}
}

func TestTheBriefNeedsTheCloneDirectory(t *testing.T) {
	if _, err := (CloneBrief{}).Brief(context.Background(), "spf13__cobra"); err == nil {
		t.Fatal("the brief accepted a configuration with no clone directory")
	}
}

// The brief reads the direction, not the whole repository: the arguments are part of the contract
// with git, and a silent change of them changes what the node judges.
func TestTheBriefAsksGitForTheRecentSubjectsOnly(t *testing.T) {
	root, git := cloneWith(t, "README")

	if _, err := briefWith(root, git).Brief(context.Background(), "spf13__cobra"); err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if len(git.args) != 1 {
		t.Fatalf("git was called %d times, want 1", len(git.args))
	}
	for _, want := range []string{"log", "--oneline", "--no-decorate"} {
		if !hasArg(git.args[0], want) {
			t.Errorf("git was called with %v, want it to carry %q", git.args[0], want)
		}
	}
	want := filepath.Join(root, "spf13__cobra")
	if git.dirs[0] != want {
		t.Errorf("git ran in %q, want %q", git.dirs[0], want)
	}
}

// The key on disk is owner__repo, and the two shapes of the name are the same repository: the
// caller that has store.RepoName at hand and the one that only has nameWithOwner both have to
// land on the same directory and the same header.
func TestTheBriefAcceptsBothShapesOfTheRepositoryName(t *testing.T) {
	root, _ := cloneWith(t, "Cobra is a CLI library.")

	var dirs []string
	var headers []string
	for _, repository := range []string{"spf13__cobra", "spf13/cobra"} {
		git := &fakeGit{output: []byte("abc1234 fix the parser\n")}
		got, err := briefWith(root, git).Brief(context.Background(), repository)
		if err != nil {
			t.Fatalf("Brief(%q): %v", repository, err)
		}
		dirs = append(dirs, git.dirs[0])
		headers = append(headers, strings.SplitN(got, "\n", 2)[0])
	}

	if dirs[0] != dirs[1] {
		t.Errorf("the two shapes read different directories: %q and %q", dirs[0], dirs[1])
	}
	if want := filepath.Join(root, "spf13__cobra"); dirs[0] != want {
		t.Errorf("the brief read %q, want %q", dirs[0], want)
	}
	if headers[0] != headers[1] {
		t.Errorf("the two shapes write different headers: %q and %q", headers[0], headers[1])
	}
	if headers[0] != "# spf13__cobra" {
		t.Errorf("the header is %q, want %q", headers[0], "# spf13__cobra")
	}
}

// This is the test the convention was missing: it ties the two producers of the same key. The
// clone is built where config.RepoDir puts it, and the brief has to find it there. Change either
// side and this fails, instead of the alignment node degrading in silence.
func TestTheBriefReadsTheDirectoryRepoDirBuilds(t *testing.T) {
	cfg := config.Default()
	root := t.TempDir()
	cfg.Repos.CloneDir = root

	dir := cfg.RepoDir("spf13", "cobra")
	writeClone(t, dir, "Cobra is a CLI library.")

	for _, repository := range []string{"spf13__cobra", "spf13/cobra"} {
		git := &fakeGit{output: []byte("abc1234 fix the parser\n")}
		got, err := briefWith(root, git).Brief(context.Background(), repository)
		if err != nil {
			t.Fatalf("Brief(%q): %v", repository, err)
		}
		if git.dirs[0] != dir {
			t.Errorf("Brief(%q) read %q, want the clone at %q", repository, git.dirs[0], dir)
		}
		if !strings.Contains(got, "Cobra is a CLI library.") {
			t.Errorf("Brief(%q) did not find the README config.RepoDir placed at %q:\n%s", repository, dir, got)
		}
	}
}

// hasArg reports whether git was called with one exact argument.
func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
