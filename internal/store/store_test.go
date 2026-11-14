package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/indexer"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return s
}

func TestWriteThenReadRoundTrips(t *testing.T) {
	s := newStore(t)
	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	in := payload{Name: "cobra", Count: 262}

	if err := s.Write("repos/cobra.json", in); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	var out payload
	if err := s.Read("repos/cobra.json", &out); err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if out != in {
		t.Errorf("round trip = %+v, want %+v", out, in)
	}
	if !s.Exists("repos/cobra.json") {
		t.Error("Exists() = false after a write")
	}
}

func TestWriteCreatesParentDirectories(t *testing.T) {
	s := newStore(t)
	if err := s.Write("a/b/c.json", map[string]bool{"ok": true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), "a", "b", "c.json")); err != nil {
		t.Fatalf("nested file missing: %v", err)
	}
}

// A reader must never see a partial file, and no temporary file may survive a write.
func TestWriteLeavesNoTemporaryFiles(t *testing.T) {
	s := newStore(t)
	for i := 0; i < 3; i++ {
		if err := s.Write("x.json", map[string]int{"n": i}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	entries, err := os.ReadDir(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}
}

// An older or newer on-disk format must be refused, not decoded into something wrong.
func TestReadRejectsForeignSchema(t *testing.T) {
	s := newStore(t)
	body := `{"schema": 999, "saved_at": "2026-01-01T00:00:00Z", "data": {}}`
	if err := os.WriteFile(s.Path("old.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	err := s.Read("old.json", &out)
	if err == nil || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("err = %v, want a schema complaint", err)
	}
}

// A payload with an unknown field means the file is ours but written by another version.
func TestReadRejectsUnknownPayloadField(t *testing.T) {
	s := newStore(t)
	envelope := map[string]any{
		"schema":   SchemaVersion,
		"saved_at": time.Now().UTC(),
		"data":     map[string]any{"known": "x", "surprise": 1},
	}
	b, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Path("p.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Known string `json:"known"`
	}
	if err := s.Read("p.json", &out); err == nil {
		t.Fatal("want an error for the unknown payload field")
	}
}

func TestWriteRejectsAbsoluteName(t *testing.T) {
	s := newStore(t)
	if err := s.Write("/etc/passwd", map[string]bool{}); err == nil {
		t.Fatal("want an error: names are relative to the store")
	}
}

func TestSavedAtIsRecorded(t *testing.T) {
	s := newStore(t)
	before := time.Now().UTC().Add(-time.Second)
	if err := s.Write("t.json", map[string]string{"a": "b"}); err != nil {
		t.Fatal(err)
	}
	at, err := s.SavedAt("t.json")
	if err != nil {
		t.Fatal(err)
	}
	if at.Before(before) {
		t.Errorf("SavedAt = %v, want it after %v", at, before)
	}
}

func TestSnapshotRoundTrips(t *testing.T) {
	s := newStore(t)
	repo := RepoName("spf13", "cobra")
	snap := &ghdata.Snapshot{
		Repository: ghdata.Repository{
			NameWithOwner: "spf13/cobra",
			CommitSHA:     "adbc8813901b",
			DiskUsageKB:   2107,
		},
		TotalOpen: 5,
		Issues: []ghdata.Issue{{
			Number: 1391,
			Title:  "404 links",
			Labels: []string{"good-first-issue"},
			LinkedPRs: []ghdata.PullRequest{
				{Number: 1984, State: "OPEN"},
			},
		}},
	}

	if err := s.SaveSnapshot(repo, snap); err != nil {
		t.Fatalf("SaveSnapshot() error = %v", err)
	}
	if !s.HasSnapshot(repo) {
		t.Error("HasSnapshot() = false after saving")
	}
	got, err := s.LoadSnapshot(repo)
	if err != nil {
		t.Fatalf("LoadSnapshot() error = %v", err)
	}
	if got.Repository.CommitSHA != snap.Repository.CommitSHA {
		t.Errorf("commit = %q, want the pinned one", got.Repository.CommitSHA)
	}
	if len(got.Issues) != 1 || got.Issues[0].Labels[0] != "good-first-issue" {
		t.Errorf("labels lost: %+v", got.Issues)
	}
	if len(got.Issues[0].LinkedPRs) != 1 {
		t.Errorf("linked PRs lost: %+v", got.Issues[0])
	}
}

func TestIndexRecordPinsTheCommit(t *testing.T) {
	s := newStore(t)
	repo := RepoName("spf13", "cobra")
	rec := IndexRecord{
		Repository: repo,
		CommitSHA:  "adbc8813901b",
		Path:       "/tmp/cobra",
		Stats:      indexer.Stats{Files: 41, Nodes: 910, Edges: 4051},
	}
	if err := s.SaveIndex(repo, rec); err != nil {
		t.Fatalf("SaveIndex() error = %v", err)
	}
	got, err := s.LoadIndex(repo)
	if err != nil {
		t.Fatalf("LoadIndex() error = %v", err)
	}
	if got.CommitSHA != rec.CommitSHA || got.Stats.Nodes != 910 {
		t.Errorf("got %+v, want the pinned record", got)
	}
}

func TestNewRejectsEmptyDir(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("want an error for an empty directory")
	}
}

// A renamed repository must stay one entry, reachable by the name it was asked for.
func TestAliasResolvesRenamedRepository(t *testing.T) {
	s := newStore(t)
	if _, ok := s.Alias("httpie/httpie"); ok {
		t.Fatal("an empty store has no aliases")
	}
	if err := s.SaveAlias("httpie/httpie", "httpie/cli"); err != nil {
		t.Fatalf("SaveAlias() error = %v", err)
	}
	got, ok := s.Alias("httpie/httpie")
	if !ok || got != "httpie/cli" {
		t.Fatalf("Alias() = %q,%v, want httpie/cli,true", got, ok)
	}
	// Asking for the canonical name is not a rename.
	if err := s.SaveAlias("httpie/cli", "httpie/cli"); err != nil {
		t.Fatalf("SaveAlias(same) error = %v", err)
	}
	if _, ok := s.Alias("httpie/cli"); ok {
		t.Error("an identity mapping was stored")
	}
}

func TestAliasSurvivesASecondWrite(t *testing.T) {
	s := newStore(t)
	for _, pair := range [][2]string{
		{"httpie/httpie", "httpie/cli"},
		{"a/b", "a/c"},
	} {
		if err := s.SaveAlias(pair[0], pair[1]); err != nil {
			t.Fatalf("SaveAlias(%v) error = %v", pair, err)
		}
	}
	for _, pair := range [][2]string{
		{"httpie/httpie", "httpie/cli"},
		{"a/b", "a/c"},
	} {
		if got, ok := s.Alias(pair[0]); !ok || got != pair[1] {
			t.Errorf("Alias(%q) = %q,%v, want %q", pair[0], got, ok, pair[1])
		}
	}
}
