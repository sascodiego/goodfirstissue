package repo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsRepoDetectsDirectoryAndWorktree(t *testing.T) {
	dir := t.TempDir()
	if IsRepo(dir) {
		t.Error("an empty directory is not a repository")
	}

	// A worktree records .git as a file, not a directory.
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /elsewhere\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !IsRepo(dir) {
		t.Error("a .git file must count as a repository")
	}
}

func TestEnsureClonesWhenMissing(t *testing.T) {
	var calls []string
	c := &Cloner{Run: func(_ context.Context, dir string, args ...string) ([]byte, error) {
		calls = append(calls, dir+" | "+strings.Join(args, " "))
		if args[0] == "rev-parse" {
			return []byte("adbc8813901b\n"), nil
		}
		return nil, nil
	}}

	sha, cloned, err := c.Ensure(t.Context(), "https://github.com/spf13/cobra.git", t.TempDir())
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if !cloned {
		t.Error("cloned = false, want true for a fresh directory")
	}
	if sha != "adbc8813901b" {
		t.Errorf("sha = %q, want the checked-out commit", sha)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %v, want clone then rev-parse", calls)
	}
	if !strings.Contains(calls[0], "clone --depth 1") {
		t.Errorf("first call = %q, want a shallow clone", calls[0])
	}
}

func TestEnsureReusesExistingClone(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var calls []string
	c := &Cloner{Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		return []byte("deadbeef\n"), nil
	}}

	sha, cloned, err := c.Ensure(t.Context(), "https://example/x.git", dir)
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if cloned {
		t.Error("cloned = true, want false: an existing clone must not be re-cloned")
	}
	if sha != "deadbeef" {
		t.Errorf("sha = %q", sha)
	}
	if len(calls) != 1 || calls[0] != "rev-parse HEAD" {
		t.Errorf("calls = %v, want only rev-parse", calls)
	}
}

func TestEnsureValidatesInput(t *testing.T) {
	c := &Cloner{}
	if _, _, err := c.Ensure(t.Context(), "", "/tmp/x"); err == nil {
		t.Error("want an error for a missing url")
	}
	if _, _, err := c.Ensure(t.Context(), "https://example/x.git", ""); err == nil {
		t.Error("want an error for a missing dir")
	}
}

func TestEnsurePropagatesCloneFailure(t *testing.T) {
	c := &Cloner{Run: func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("network down")
	}}
	if _, _, err := c.Ensure(t.Context(), "https://example/x.git", t.TempDir()); err == nil {
		t.Fatal("want the clone failure to surface")
	}
}

func TestHeadRejectsEmptyOutput(t *testing.T) {
	c := &Cloner{Run: func(context.Context, string, ...string) ([]byte, error) {
		return []byte("  \n"), nil
	}}
	if _, err := c.Head(t.Context(), t.TempDir()); err == nil {
		t.Fatal("want an error when git reports no commit")
	}
}

// RecentSubjects is what the alignment node reads to see where a project is actually going. It
// has to return subjects, not hashes with noise: a blank line in the middle must not become an
// empty commit in the brief.
func TestRecentSubjectsReadsTheSubjects(t *testing.T) {
	c := &Cloner{Run: func(_ context.Context, dir string, args ...string) ([]byte, error) {
		if dir != "/clone" {
			t.Errorf("git ran in %q, want /clone", dir)
		}
		want := "log --no-decorate --oneline -20"
		if got := strings.Join(args, " "); got != want {
			t.Errorf("git %q, want %q", got, want)
		}
		return []byte("abc1234 fix the parser\n\ndef5678 add a flag\n\n"), nil
	}}

	got, err := c.RecentSubjects(context.Background(), "/clone", 20)
	if err != nil {
		t.Fatalf("RecentSubjects: %v", err)
	}
	want := []string{"abc1234 fix the parser", "def5678 add a flag"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("subjects = %v, want %v", got, want)
	}
}

func TestRecentSubjectsRejectsAnEmptyCount(t *testing.T) {
	if _, err := (&Cloner{}).RecentSubjects(context.Background(), "/clone", 0); err == nil {
		t.Fatal("a commit count of zero was accepted: the brief would ask git for nothing")
	}
}

func TestRecentSubjectsReportsAGitFailure(t *testing.T) {
	c := &Cloner{Run: func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("not a repository")
	}}
	if _, err := c.RecentSubjects(context.Background(), "/clone", 20); err == nil {
		t.Fatal("a failed git call was reported as a success")
	}
}
