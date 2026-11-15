package projectbrief_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/projectbrief"
	"github.com/sascodiego/nan-hackathon-clef/internal/repo"
	"github.com/sascodiego/nan-hackathon-clef/internal/triage"
)

// headGit answers the one git question the source asks, and records the directory it was asked
// about, so a test can tell the clone was found by the key the store uses.
func headGit(t *testing.T, commit string, seen *string) *repo.Cloner {
	t.Helper()
	return &repo.Cloner{Run: func(_ context.Context, dir string, args ...string) ([]byte, error) {
		if len(args) == 0 || args[0] != "rev-parse" {
			t.Errorf("unexpected git call %v", args)
		}
		if seen != nil {
			*seen = dir
		}
		return []byte(commit + "\n"), nil
	}}
}

// S6: the portrait feeds AlignmentDossier without changing its signature. This test is the seam
// itself: a real portrait, the production dossier, one inserted into the other.
func TestSourceFeedsAlignmentDossier(t *testing.T) {
	a := &agent{}
	e := newExtractor(t, a)
	root := t.TempDir()
	dir := filepath.Join(root, "spf13__cobra")
	fillClone(t, dir)

	var asked string
	src := projectbrief.Source{Extractor: e, Root: root, Git: headGit(t, testCommit, &asked)}

	// The node consumes a BriefSource, so that is what a test asks through.
	var brief triage.BriefSource = src
	text, err := brief.Brief(context.Background(), "spf13/cobra")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if text != validAnswer {
		t.Errorf("the brief is the portrait verbatim, got %q", text)
	}
	// One repository, two shapes of its name: the clone is found by the key store.RepoName makes.
	if asked != dir {
		t.Errorf("asked git about %q, want %q", asked, dir)
	}
	calls := a.calls()
	if len(calls) != 1 {
		t.Fatalf("runs = %d, want 1", len(calls))
	}
	if calls[0].Dir != dir {
		t.Errorf("the agent read %q, want the clone", calls[0].Dir)
	}
	if !strings.Contains(calls[0].Prompt, testCommit) {
		t.Error("the portrait must know the commit it describes (I14)")
	}

	issue := triage.Issue{
		Repository:  "spf13__cobra",
		Number:      7,
		Title:       "Add a flag",
		Body:        "It would help to have one.",
		GitHubState: "OPEN",
	}
	dossier := triage.AlignmentDossier(issue, text)
	if !strings.Contains(dossier, validAnswer) {
		t.Error("the dossier must carry the portrait it was given")
	}
	if !strings.Contains(dossier, "PROJECT DIRECTION") {
		t.Error("the dossier must still be the alignment dossier")
	}
}

// A repository that is not cloned yet is a condition to report, not a crash and not a portrait
// invented from a name.
func TestSourceReportsAMissingClone(t *testing.T) {
	e := newExtractor(t, &agent{})
	src := projectbrief.Source{
		Extractor: e,
		Root:      t.TempDir(),
		Git: &repo.Cloner{Run: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("fatal: not a git repository")
		}},
	}
	_, err := src.Brief(context.Background(), "spf13/cobra")
	if err == nil {
		t.Fatal("a clone that is not there cannot be described")
	}
	for _, want := range []string{"spf13/cobra", "clone it first"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error must say %q: %v", want, err)
		}
	}
}

func TestSourceRefusesWithoutThePartsOfAPortrait(t *testing.T) {
	if _, err := (projectbrief.Source{Root: t.TempDir()}).Portrait(context.Background(), "a/b"); err == nil {
		t.Error("a source without an extractor must refuse")
	}
	if _, err := (projectbrief.Source{Extractor: newExtractor(t, &agent{})}).Portrait(context.Background(), "a/b"); err == nil {
		t.Error("a source without a clone directory must refuse")
	}
}

// What the node is told about a portrait is the portrait; whether the tree backs it is reported to
// whoever asked for it.
func TestSourcePortraitReportsUnsupportedClaims(t *testing.T) {
	answer := replaceOnce(t, validAnswer, "README.md:3", "README.MISSING:3")
	a := &agent{out: outcomeOf(answer)}
	e := newExtractor(t, a)
	root := t.TempDir()
	fillClone(t, filepath.Join(root, "spf13__cobra"))
	src := projectbrief.Source{Extractor: e, Root: root, Git: headGit(t, testCommit, nil)}

	res, err := src.Portrait(context.Background(), "spf13__cobra")
	if err != nil {
		t.Fatalf("Portrait: %v", err)
	}
	if len(res.Unsupported) != 1 || res.Unsupported[0].Evidence.Path != "README.MISSING" {
		t.Errorf("unsupported = %+v", res.Unsupported)
	}
	if res.Commit != testCommit {
		t.Errorf("commit = %q", res.Commit)
	}
}
