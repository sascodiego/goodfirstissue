package projectbrief_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/pig"
	"github.com/sascodiego/nan-hackathon-clef/internal/projectbrief"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

func newCachedExtractor(t *testing.T, a *agent) (*projectbrief.Extractor, *store.Store) {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e, err := projectbrief.New(projectbrief.Options{
		Run:   a.run,
		Model: testModel,
		Home:  t.TempDir(),
		Tools: []string{"read", "ls", "grep"},
		Store: st,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return e, st
}

// patchCache edits every stored entry, which is how a test plays the part of a file a person can
// write by hand. The store wraps a document in its own envelope, so the entries live one level
// down.
func patchCache(t *testing.T, st *store.Store, edit func(entry map[string]any)) {
	t.Helper()
	body, err := os.ReadFile(st.Path("projectbrief/cache.json"))
	if err != nil {
		t.Fatalf("cache file: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	data, _ := document["data"].(map[string]any)
	entries, _ := data["entries"].(map[string]any)
	if len(entries) == 0 {
		t.Fatalf("the cache holds no entry: %s", body)
	}
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("entry %T", raw)
		}
		edit(entry)
	}
	patched, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(st.Path("projectbrief/cache.json"), patched, 0o644); err != nil {
		t.Fatal(err)
	}
}

// S7: one run per repository, not per issue. The second reader of the same tree pays nothing.
func TestExtractReusesThePortraitOfTheSameCommitAndSpendsNoRun(t *testing.T) {
	a := &agent{}
	e, _ := newCachedExtractor(t, a)
	in := projectbrief.Input{Repository: "spf13__cobra", CommitSHA: testCommit, Dir: clone(t)}

	first, err := e.Extract(context.Background(), in)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	second, err := e.Extract(context.Background(), in)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if got := len(a.calls()); got != 1 {
		t.Fatalf("runs = %d, want 1", got)
	}
	if first.Cached {
		t.Error("the first read is not cached")
	}
	if !second.Cached {
		t.Error("the second read of the same tree must be reused")
	}
	if second.Document.Text != first.Document.Text {
		t.Error("a reused portrait is the same document, not a rendering of it")
	}
	// The cost of the question does not change because the answer was reused: a reader asking
	// "what did this cost" must not be told zero.
	if second.Took != first.Took {
		t.Errorf("took = %s, want %s", second.Took, first.Took)
	}
	if second.Commit != testCommit {
		t.Errorf("commit = %q", second.Commit)
	}
}

// -force: the cache names a question, and a tree that changed without moving the commit is the
// same question with a different answer. A caller that says so pays a new run, and the answer it
// gets is the one left behind: the store must not keep serving the older one.
func TestExtractWithForceRunsAgainAndReplacesTheStoredPortrait(t *testing.T) {
	a := &agent{}
	e, _ := newCachedExtractor(t, a)
	in := projectbrief.Input{Repository: "spf13__cobra", CommitSHA: testCommit, Dir: clone(t)}
	if _, err := e.Extract(context.Background(), in); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	// A second answer that is still a valid portrait: only the prose moved, the citations did
	// not, so nothing but the cache can tell the two runs apart.
	const revised = "Una librería de línea de comandos en Go"
	second := strings.Replace(validAnswer, "Una librería de CLI en Go", revised, 1)
	if second == validAnswer {
		t.Fatal("the revised answer is not a revision")
	}
	a.out = &pig.Outcome{Answer: second, Total: 3 * time.Second}

	forced := in
	forced.Force = true
	res, err := e.Extract(context.Background(), forced)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := len(a.calls()); got != 2 {
		t.Fatalf("runs = %d, want 2: an ignored portrait costs a run", got)
	}
	if res.Cached {
		t.Error("a run that had to happen is not cached")
	}
	if res.Document.Text != second {
		t.Error("the forced run returns the answer it paid for")
	}
	if res.Took != 3*time.Second {
		t.Errorf("took = %s, want the cost of the run that actually happened", res.Took)
	}

	// The portrait the forced run produced is the one stored: asking again without -force is
	// answered from it, for free.
	again, err := e.Extract(context.Background(), in)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := len(a.calls()); got != 2 {
		t.Errorf("runs = %d, want 2: the replacement is reused", got)
	}
	if !again.Cached {
		t.Error("the replaced portrait is what the store keeps")
	}
	if again.Document.Text != second {
		t.Error("the store still serves the portrait the forced run replaced")
	}
}

// S4: the portrait is pinned to the commit. A new commit is a new question, whatever else is
// unchanged.
func TestExtractRecomputesWhenTheCommitChanges(t *testing.T) {
	a := &agent{}
	e, _ := newCachedExtractor(t, a)
	dir := clone(t)

	if _, err := e.Extract(context.Background(), projectbrief.Input{
		Repository: "spf13__cobra", CommitSHA: testCommit, Dir: dir,
	}); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	again, err := e.Extract(context.Background(), projectbrief.Input{
		Repository: "spf13__cobra", CommitSHA: "0000000000000000000000000000000000000000", Dir: dir,
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := len(a.calls()); got != 2 {
		t.Fatalf("runs = %d, want 2: another commit is another question (I14)", got)
	}
	if again.Cached {
		t.Error("a portrait of another commit must never be presented as fresh")
	}
	if again.Commit != "0000000000000000000000000000000000000000" {
		t.Errorf("commit = %q", again.Commit)
	}
}

func TestExtractRecomputesWhenTheModelChanges(t *testing.T) {
	a := &agent{}
	e, st := newCachedExtractor(t, a)
	in := projectbrief.Input{Repository: "spf13__cobra", CommitSHA: testCommit, Dir: clone(t)}
	if _, err := e.Extract(context.Background(), in); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	other, err := projectbrief.New(projectbrief.Options{
		Run: a.run, Model: "glm5.3-flash", Home: t.TempDir(), Tools: []string{"read"}, Store: st,
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := other.Extract(context.Background(), in)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := len(a.calls()); got != 2 {
		t.Fatalf("runs = %d, want 2: another model is another reading", got)
	}
	if res.Cached || res.Model != "glm5.3-flash" {
		t.Errorf("result = %+v", res)
	}
}

// The store is a file a person can edit. An entry that names another tree than the one asked for
// is not served, whatever its key says.
func TestExtractDoesNotServeAPortraitOfAnotherTree(t *testing.T) {
	a := &agent{}
	e, st := newCachedExtractor(t, a)
	in := projectbrief.Input{Repository: "spf13__cobra", CommitSHA: testCommit, Dir: clone(t)}
	if _, err := e.Extract(context.Background(), in); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	patchCache(t, st, func(entry map[string]any) {
		entry["commit_sha"] = "ffffffffffffffffffffffffffffffffffffffff"
	})

	if _, err := e.Extract(context.Background(), in); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := len(a.calls()); got != 2 {
		t.Errorf("runs = %d, want 2: an entry naming another commit is not this question", got)
	}
}

// A damaged entry, or an entry written under other reading rules, costs a run and never an error:
// the cache is a saving, never a source of truth.
func TestExtractRunsAgainWhenTheStoredAnswerIsUnreadable(t *testing.T) {
	cases := map[string]func(entry map[string]any){
		"prose": func(entry map[string]any) { entry["text"] = "El proyecto es una librería de CLI." },
		"version": func(entry map[string]any) {
			entry["prompt_version"] = "000000000000"
		},
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			a := &agent{}
			e, st := newCachedExtractor(t, a)
			in := projectbrief.Input{Repository: "spf13__cobra", CommitSHA: testCommit, Dir: clone(t)}
			if _, err := e.Extract(context.Background(), in); err != nil {
				t.Fatalf("Extract: %v", err)
			}
			patchCache(t, st, edit)

			res, err := e.Extract(context.Background(), in)
			if err != nil {
				t.Fatalf("Extract: %v", err)
			}
			if got := len(a.calls()); got != 2 {
				t.Errorf("runs = %d, want 2", got)
			}
			if res.Cached {
				t.Error("a run that had to happen is not cached")
			}
		})
	}
}

func TestExtractWithoutAStoreNeverCaches(t *testing.T) {
	a := &agent{}
	e := newExtractor(t, a)
	in := projectbrief.Input{Repository: "spf13__cobra", CommitSHA: testCommit, Dir: clone(t)}
	for i := 0; i < 2; i++ {
		res, err := e.Extract(context.Background(), in)
		if err != nil {
			t.Fatalf("Extract: %v", err)
		}
		if res.Cached {
			t.Error("no store means nothing to reuse")
		}
	}
	if got := len(a.calls()); got != 2 {
		t.Errorf("runs = %d, want 2", got)
	}
}

// The measured cost is what the run reported, so a stored portrait keeps saying what it cost.
func TestCachedPortraitKeepsTheCostOfItsRun(t *testing.T) {
	a := &agent{}
	e, _ := newCachedExtractor(t, a)
	in := projectbrief.Input{Repository: "spf13__cobra", CommitSHA: testCommit, Dir: clone(t)}
	if _, err := e.Extract(context.Background(), in); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	res, err := e.Extract(context.Background(), in)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if res.Took != 41*time.Second {
		t.Errorf("took = %s, want 41s", res.Took)
	}
}

// A cache that cannot be written costs a future request, never the portrait the caller asked for.
func TestExtractSurvivesAnUnwritableStore(t *testing.T) {
	a := &agent{}
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// A directory where the cache file should be: the write cannot land.
	if err := os.MkdirAll(filepath.Join(st.Dir(), "projectbrief", "cache.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	e, err := projectbrief.New(projectbrief.Options{
		Run: a.run, Model: testModel, Home: t.TempDir(), Tools: []string{"read"}, Store: st,
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.Extract(context.Background(), projectbrief.Input{
		Repository: "spf13__cobra", CommitSHA: testCommit, Dir: clone(t),
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if res.Document.Text != validAnswer {
		t.Error("the portrait the caller asked for is not the cache's to lose")
	}
}

// A doubtful portrait is a valid answer for its commit, so it is stored like any other: the next
// reader of the same tree pays nothing to be told the same thing.
func TestExtractCachesADoubtfulPortrait(t *testing.T) {
	a := &agent{out: &pig.Outcome{Answer: projectbrief.Insufficient}}
	e, _ := newCachedExtractor(t, a)
	in := projectbrief.Input{Repository: "spf13__cobra", CommitSHA: testCommit, Dir: clone(t)}
	if _, err := e.Extract(context.Background(), in); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	res, err := e.Extract(context.Background(), in)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !res.Cached || !res.Document.Insufficient() {
		t.Errorf("result = %+v", res)
	}
	if res.Reason == "" {
		t.Error("a reused doubtful portrait still says why it doubts")
	}
}
