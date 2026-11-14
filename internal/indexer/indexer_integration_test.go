package indexer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegrationPrepareCobra indexes a real clone with the real binary.
//
//	GFI_INTEGRATION=1 go test ./internal/indexer/ -run Integration -v
func TestIntegrationPrepareCobra(t *testing.T) {
	if os.Getenv("GFI_INTEGRATION") == "" {
		t.Skip("set GFI_INTEGRATION=1 to run codegraph")
	}
	if _, err := exec.LookPath("codegraph"); err != nil {
		t.Skipf("codegraph not installed: %v", err)
	}
	repo, err := filepath.Abs("../../var/repos/cobra")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(repo); err != nil {
		t.Skipf("clone not present: %v", err)
	}

	stats, err := (&Indexer{}).Prepare(t.Context(), repo)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Logf("files=%d nodes=%d edges=%d", stats.Files, stats.Nodes, stats.Edges)
	if stats.Nodes == 0 || stats.Edges == 0 || stats.Files == 0 {
		t.Errorf("stats = %+v, want a populated index", stats)
	}

	// The clone must stay clean: the exclusion lives in .git/info/exclude.
	out, err := exec.Command("git", "-C", repo, "status", "--porcelain").Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("the clone was modified:\n%s", out)
	}
}
