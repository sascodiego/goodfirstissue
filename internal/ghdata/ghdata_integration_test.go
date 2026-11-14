package ghdata

import (
	"os"
	"os/exec"
	"testing"
)

// TestIntegrationFetchCobra hits the real API through the gh CLI.
//
//	GFI_INTEGRATION=1 go test ./internal/ghdata/ -run Integration -v
func TestIntegrationFetchCobra(t *testing.T) {
	if os.Getenv("GFI_INTEGRATION") == "" {
		t.Skip("set GFI_INTEGRATION=1 to reach GitHub")
	}
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skipf("gh not installed: %v", err)
	}

	snap, err := (&Client{}).Fetch(t.Context(), "spf13", "cobra", Options{MaxIssues: 5})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	t.Logf("repo=%s sha=%.12s open=%d fetched=%d",
		snap.Repository.NameWithOwner, snap.Repository.CommitSHA, snap.TotalOpen, len(snap.Issues))

	if snap.Repository.CommitSHA == "" {
		t.Error("no commit SHA: results could not be pinned (I4)")
	}
	if len(snap.Issues) != 5 {
		t.Errorf("got %d issues, want 5", len(snap.Issues))
	}
	if snap.TotalOpen <= 5 {
		t.Errorf("TotalOpen = %d, want the repository's real count", snap.TotalOpen)
	}

	// A label filter would have left only labelled issues here.
	var unlabelled int
	for _, iss := range snap.Issues {
		if len(iss.Labels) == 0 {
			unlabelled++
		}
		if iss.Number == 0 || iss.Title == "" {
			t.Errorf("issue without identity: %+v", iss)
		}
	}
	t.Logf("issues without any label: %d (a label filter would have excluded them)", unlabelled)
}
