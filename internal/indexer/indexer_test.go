package indexer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const initOut = "\n●  910 nodes, 4,051 edges in 650ms\n└  Done\n"

const statusOut = `
CodeGraph Status

Index Statistics:
  Files:     41
  Nodes:     910
  Edges:     4,051
  DB Size:   3.83 MB
`

func TestPreparePrefersStatusNumbers(t *testing.T) {
	var calls []string
	in := &Indexer{Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		calls = append(calls, joined)
		if strings.HasPrefix(joined, "status") {
			return []byte(statusOut), nil
		}
		return []byte(initOut), nil
	}}

	stats, err := in.Prepare(t.Context(), t.TempDir())
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if stats != (Stats{Files: 41, Nodes: 910, Edges: 4051}) {
		t.Errorf("stats = %+v, want the status figures", stats)
	}
	if len(calls) != 2 || !strings.HasPrefix(calls[0], "init -y") {
		t.Errorf("calls = %v, want init then status", calls)
	}
}

// Without a status, init's own line is the only source. It has no file count.
func TestPrepareFallsBackToInitCounts(t *testing.T) {
	in := &Indexer{Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(args, " "), "status") {
			return nil, errors.New("boom")
		}
		return []byte(initOut), nil
	}}

	stats, err := in.Prepare(t.Context(), t.TempDir())
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if stats.Nodes != 910 || stats.Edges != 4051 || stats.Files != 0 {
		t.Errorf("stats = %+v, want nodes and edges from init", stats)
	}
}

func TestPrepareRejectsUnreadableStats(t *testing.T) {
	in := &Indexer{Run: func(context.Context, string, ...string) ([]byte, error) {
		return []byte("something else entirely"), nil
	}}
	if _, err := in.Prepare(t.Context(), t.TempDir()); err == nil {
		t.Fatal("want an error when neither status nor init reports statistics")
	}
}

// Re-running init on an existing index prints no counts; status must still answer.
func TestPrepareHandlesAlreadyIndexedOutput(t *testing.T) {
	in := &Indexer{Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(args, " "), "status") {
			return []byte(statusOut), nil
		}
		return []byte("┌  Initializing CodeGraph\n└  Already indexed\n"), nil
	}}
	stats, err := in.Prepare(t.Context(), t.TempDir())
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if stats.Files != 41 {
		t.Errorf("stats = %+v, want them taken from status", stats)
	}
}

// The index must not appear as an untracked change in a repository that is not ours.
func TestExcludeFromGitIsIdempotent(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	exclude := filepath.Join(repo, ".git", "info", "exclude")
	if err := os.WriteFile(exclude, []byte("*.bak\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		if err := excludeFromGit(repo); err != nil {
			t.Fatalf("excludeFromGit() error = %v", err)
		}
	}

	b, err := os.ReadFile(exclude)
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if got := strings.Count(body, ".codegraph/"); got != 1 {
		t.Errorf(".codegraph/ appears %d times, want 1:\n%s", got, body)
	}
	if !strings.Contains(body, "*.bak") {
		t.Error("the existing exclusion was lost")
	}
	if !strings.HasSuffix(body, "\n") {
		t.Error("the file must end with a newline")
	}
}

func TestExcludeFromGitToleratesNonRepository(t *testing.T) {
	if err := excludeFromGit(t.TempDir()); err != nil {
		t.Fatalf("a directory without .git must be tolerated, got %v", err)
	}
}

func TestPrepareRequiresPath(t *testing.T) {
	if _, err := (&Indexer{}).Prepare(t.Context(), ""); err == nil {
		t.Fatal("want an error for an empty path")
	}
}
