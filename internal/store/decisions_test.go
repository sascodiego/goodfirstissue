package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func key(issue int) Key {
	return Key{
		Issue:         issue,
		Pass:          PassTriage,
		Model:         "clef",
		PromptVersion: "p1",
		CommitSHA:     "abc123",
	}
}

func verdict(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal verdict: %v", err)
	}
	return b
}

// The whole point: work already done is not repeated after a restart.
func TestDecisionSurvivesReopeningTheStore(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d, err := s.Decisions("a__b")
	if err != nil {
		t.Fatalf("Decisions() error = %v", err)
	}
	if _, ok := d.Get(key(7)); ok {
		t.Fatal("an empty journal returned a hit")
	}
	if err := d.Put(Decision{Key: key(7), Verdict: verdict(t, map[string]any{"state": "actionable"})}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// A new process opens the store from scratch.
	reopened, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d2, err := reopened.Decisions("a__b")
	if err != nil {
		t.Fatalf("Decisions() error = %v", err)
	}
	got, ok := d2.Get(key(7))
	if !ok {
		t.Fatal("the decision did not survive reopening the store")
	}
	var payload map[string]any
	if err := json.Unmarshal(got.Verdict, &payload); err != nil {
		t.Fatalf("verdict is not readable: %v", err)
	}
	if payload["state"] != "actionable" {
		t.Errorf("verdict = %v", payload)
	}
	if got.At.IsZero() {
		t.Error("Put did not stamp the decision")
	}
}

// A verdict produced by another prompt answers another question.
func TestDecisionMissesOnAnyChangedKeyField(t *testing.T) {
	s := newStore(t)
	d, err := s.Decisions("a__b")
	if err != nil {
		t.Fatalf("Decisions() error = %v", err)
	}
	if err := d.Put(Decision{Key: key(7), Verdict: verdict(t, "x")}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	cases := map[string]Key{
		"issue":          {Issue: 8, Pass: PassTriage, Model: "clef", PromptVersion: "p1", CommitSHA: "abc123"},
		"pass":           {Issue: 7, Pass: PassFit, Model: "clef", PromptVersion: "p1", CommitSHA: "abc123"},
		"model":          {Issue: 7, Pass: PassTriage, Model: "glm5.3-flash", PromptVersion: "p1", CommitSHA: "abc123"},
		"prompt version": {Issue: 7, Pass: PassTriage, Model: "clef", PromptVersion: "p2", CommitSHA: "abc123"},
		"commit":         {Issue: 7, Pass: PassTriage, Model: "clef", PromptVersion: "p1", CommitSHA: "def456"},
	}
	for name, k := range cases {
		if _, ok := d.Get(k); ok {
			t.Errorf("%s changed but the decision was still reused", name)
		}
	}
}

func TestPutRejectsIncompleteDecisions(t *testing.T) {
	s := newStore(t)
	d, err := s.Decisions("a__b")
	if err != nil {
		t.Fatalf("Decisions() error = %v", err)
	}
	for name, dec := range map[string]Decision{
		"no verdict": {Key: key(1)},
		"no issue":   {Key: Key{Pass: PassTriage}, Verdict: verdict(t, "x")},
		"no pass":    {Key: Key{Issue: 1}, Verdict: verdict(t, "x")},
	} {
		if err := d.Put(dec); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// A crash mid-append can only damage the last line, and that line was never acknowledged.
func TestTornLastLineIsTolerated(t *testing.T) {
	s := newStore(t)
	d, err := s.Decisions("a__b")
	if err != nil {
		t.Fatalf("Decisions() error = %v", err)
	}
	if err := d.Put(Decision{Key: key(1), Verdict: verdict(t, "uno")}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	path := s.Path("a__b/decisions.jsonl")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	if _, err := f.WriteString(`{"issue":2,"pass":"tri`); err != nil {
		t.Fatalf("append torn line: %v", err)
	}
	_ = f.Close()

	d2, err := s.Decisions("a__b")
	if err != nil {
		t.Fatalf("Decisions() error = %v; a torn last line must not fail the load", err)
	}
	if _, ok := d2.Get(key(1)); !ok {
		t.Error("the acknowledged decision was lost")
	}
}

func TestCorruptMiddleLineIsLoud(t *testing.T) {
	s := newStore(t)
	path := s.Path("a__b/decisions.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `{"issue":1,"pass":"triage","model":"clef","prompt_version":"p1","commit_sha":"abc123","at":"2026-01-01T00:00:00Z","verdict":"x"}
esto no es json
{"issue":2,"pass":"triage","model":"clef","prompt_version":"p1","commit_sha":"abc123","at":"2026-01-01T00:00:00Z","verdict":"y"}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write journal: %v", err)
	}
	if _, err := s.Decisions("a__b"); err == nil {
		t.Fatal("a corrupt journal was loaded silently")
	}
}

func TestLastWriteWins(t *testing.T) {
	s := newStore(t)
	d, err := s.Decisions("a__b")
	if err != nil {
		t.Fatalf("Decisions() error = %v", err)
	}
	if err := d.Put(Decision{Key: key(1), Verdict: verdict(t, "viejo")}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if err := d.Put(Decision{Key: key(1), Verdict: verdict(t, "nuevo")}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	d2, err := s.Decisions("a__b")
	if err != nil {
		t.Fatalf("Decisions() error = %v", err)
	}
	got, ok := d2.Get(key(1))
	if !ok {
		t.Fatal("no decision after two writes")
	}
	if string(got.Verdict) != `"nuevo"` {
		t.Errorf("verdict = %s, want the last one", got.Verdict)
	}
	if d2.Len() != 1 {
		t.Errorf("Len() = %d, want 1 distinct analysis", d2.Len())
	}
}

func TestCompactCollapsesDuplicateKeys(t *testing.T) {
	s := newStore(t)
	d, err := s.Decisions("a__b")
	if err != nil {
		t.Fatalf("Decisions() error = %v", err)
	}
	for i := 0; i < 200; i++ {
		if err := d.Put(Decision{Key: key(i%10 + 1), Verdict: verdict(t, i)}); err != nil {
			t.Fatalf("Put() error = %v", err)
		}
	}
	if compacted, err := d.Compact(); err != nil || !compacted {
		t.Fatalf("Compact() = %v,%v, want it to compact 200 lines into 10", compacted, err)
	}
	if d.Len() != 10 {
		t.Errorf("Len() = %d, want 10", d.Len())
	}

	d2, err := s.Decisions("a__b")
	if err != nil {
		t.Fatalf("Decisions() error = %v", err)
	}
	if d2.Len() != 10 {
		t.Fatalf("after compaction the journal holds %d analyses, want 10", d2.Len())
	}
	if d2.lines != 10 {
		t.Errorf("the compacted file has %d lines, want 10", d2.lines)
	}

	// Compaction is idempotent and stays out of the way when it would not pay.
	if compacted, err := d.Compact(); err != nil || compacted {
		t.Errorf("a second Compact() = %v,%v, want a no-op", compacted, err)
	}
}

// The server and a CLI command may append at the same time.
func TestConcurrentAppendsKeepEveryDecision(t *testing.T) {
	s := newStore(t)
	d, err := s.Decisions("a__b")
	if err != nil {
		t.Fatalf("Decisions() error = %v", err)
	}

	const writers, each = 8, 25
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				k := key(w*each + i + 1)
				if err := d.Put(Decision{Key: k, Verdict: verdict(t, w)}); err != nil {
					errs <- err
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent Put() error = %v", err)
	}

	d2, err := s.Decisions("a__b")
	if err != nil {
		t.Fatalf("Decisions() error = %v", err)
	}
	if want := writers * each; d2.Len() != want {
		t.Fatalf("Len() = %d, want %d: lines were lost or interleaved", d2.Len(), want)
	}
}

func TestJournalIsHumanReadable(t *testing.T) {
	s := newStore(t)
	d, err := s.Decisions("a__b")
	if err != nil {
		t.Fatalf("Decisions() error = %v", err)
	}
	if err := d.Put(Decision{Key: key(1), Verdict: verdict(t, "x"), At: time.Unix(0, 0).UTC()}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	b, err := os.ReadFile(s.Path("a__b/decisions.jsonl"))
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	lines := 0
	for _, line := range splitLines(string(b)) {
		lines++
		var dec Decision
		if err := json.Unmarshal([]byte(line), &dec); err != nil {
			t.Fatalf("line %d is not a decision: %v", lines, err)
		}
		if dec.At.Format(time.RFC3339) != "1970-01-01T00:00:00Z" {
			t.Errorf("timestamp = %s", dec.At)
		}
	}
	if lines != 1 {
		t.Errorf("journal has %d lines, want 1 per decision", lines)
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func ExampleKey_ID() {
	fmt.Println(key(7).ID())
	// Output: 7|triage|clef|p1|abc123
}
