package projectbrief_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/pig"
	"github.com/sascodiego/nan-hackathon-clef/internal/projectbrief"
)

const (
	testModel  = "deepseek-v4-flash"
	testCommit = "3f2a1c9be7d4a0f1c2e3d4b5a6978879a0b1c2d3"
)

// agent is the injected runner. Every test uses it instead of starting a process: the package's
// job is to build the run and read its answer, not to drive PiG (S8).
type agent struct {
	mu   sync.Mutex
	reqs []pig.Request
	out  *pig.Outcome
	err  error
	// pause blocks inside the run, so a test can observe what happens while it is in flight.
	pause func()
}

func (a *agent) run(_ context.Context, req pig.Request) (*pig.Outcome, error) {
	if a.pause != nil {
		a.pause()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.reqs = append(a.reqs, req)
	switch {
	case a.err != nil:
		return nil, a.err
	case a.out != nil:
		return a.out, nil
	}
	return &pig.Outcome{Answer: validAnswer, Total: 41 * time.Second}, nil
}

func (a *agent) calls() []pig.Request {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]pig.Request(nil), a.reqs...)
}

// clone is a directory that looks like a repository holding what the baseline answer cites, with
// more lines than it cites: the portrait of an intact clone is fully supported.
func clone(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	fillClone(t, dir)
	return dir
}

// fillClone writes the files the baseline answer cites into an existing directory.
func fillClone(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, count := range map[string]int{
		"README.md":       6,
		"go.mod":          6,
		"docs/history.md": 20,
		"Makefile":        20,
		"cmd/root.go":     20,
		"CONTRIBUTING.md": 60,
		"MAINTAINERS.md":  20,
	} {
		write(t, dir, name, numberedLines(count))
	}
}

// numberedLines is a file of n lines, so a citation of line n is the last one it has.
func numberedLines(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newExtractor(t *testing.T, a *agent) *projectbrief.Extractor {
	t.Helper()
	e, err := projectbrief.New(projectbrief.Options{
		Run:   a.run,
		Model: testModel,
		Home:  t.TempDir(),
		Tools: []string{"read", "ls", "grep"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return e
}

func TestExtractRunsOneAgentAndKeepsTheAnswerVerbatim(t *testing.T) {
	a := &agent{}
	e := newExtractor(t, a)
	dir := clone(t)

	res, err := e.Extract(context.Background(), projectbrief.Input{
		Repository: "spf13__cobra", CommitSHA: testCommit, Dir: dir,
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	calls := a.calls()
	if len(calls) != 1 {
		t.Fatalf("runs = %d, want 1", len(calls))
	}
	req := calls[0]
	if req.Dir != dir {
		t.Errorf("the agent must read the clone, dir = %q", req.Dir)
	}
	if req.Model != testModel {
		t.Errorf("model = %q, want %q", req.Model, testModel)
	}
	if len(req.SystemPrompts) != 1 || req.SystemPrompts[0] != projectbrief.Prompt {
		t.Error("the rules must travel as the system prompt, embedded and unmodified")
	}
	if strings.Join(req.Tools, ",") != "read,ls,grep" {
		t.Errorf("tools = %v", req.Tools)
	}
	// The task names what is being described and at which commit: the same clone at another
	// commit is another question (I14).
	for _, want := range []string{"spf13__cobra", testCommit} {
		if !strings.Contains(req.Prompt, want) {
			t.Errorf("the task does not name %q: %q", want, req.Prompt)
		}
	}

	if res.Document.Text != validAnswer {
		t.Error("the answer must travel verbatim: it is the text a consumer reads")
	}
	if res.Commit != testCommit || res.Model != testModel {
		t.Errorf("result = %+v", res)
	}
	if res.PromptVersion != projectbrief.Version() {
		t.Errorf("prompt version = %q", res.PromptVersion)
	}
	if res.Cached {
		t.Error("a fresh run is not cached")
	}
	if res.Took != 41*time.Second {
		t.Errorf("the cost must be measured, took = %s", res.Took)
	}
}

// S4: a portrait is pinned to the commit it read. Without the commit there is nothing to pin.
func TestExtractRefusesAPortraitWithoutACommit(t *testing.T) {
	e := newExtractor(t, &agent{})
	_, err := e.Extract(context.Background(), projectbrief.Input{
		Repository: "spf13__cobra", Dir: clone(t),
	})
	if err == nil {
		t.Fatal("a portrait without a commit cannot be pinned (I14)")
	}
}

func TestExtractRefusesWithoutACloneToRead(t *testing.T) {
	e := newExtractor(t, &agent{})
	_, err := e.Extract(context.Background(), projectbrief.Input{
		Repository: "spf13__cobra", CommitSHA: testCommit,
	})
	if err == nil {
		t.Fatal("a portrait without a directory reads nothing")
	}
}

// S9: a clone with nothing in it costs no request and says so, instead of inventing a project.
func TestExtractSaysSoForAnEmptyCloneAndSpendsNoRun(t *testing.T) {
	a := &agent{}
	e := newExtractor(t, a)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := e.Extract(context.Background(), projectbrief.Input{
		Repository: "empty__repo", CommitSHA: testCommit, Dir: dir,
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := len(a.calls()); got != 0 {
		t.Errorf("runs = %d, want 0: an empty clone has nothing to read", got)
	}
	if !res.Document.Insufficient() {
		t.Error("an empty clone yields a portrait that doubts every section")
	}
	if res.Reason == "" {
		t.Error("doubt has to be spoken (I10)")
	}
	if res.Commit != testCommit {
		t.Error("even a doubtful portrait is pinned to the commit it read")
	}
}

// The document for an empty clone is the skeleton with every section doubting itself, so Parse
// reads it back like any other answer instead of a second hand-built shape.
func TestInsufficientPortraitParsesAsADoubtfulPortrait(t *testing.T) {
	doc, err := projectbrief.Parse(projectbrief.InsufficientPortrait())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !doc.Insufficient() || len(doc.Unsupported) != 6 {
		t.Errorf("unsupported = %v", doc.Unsupported)
	}
}

func TestExtractReportsARejectedKey(t *testing.T) {
	a := &agent{out: &pig.Outcome{Error: "401: Invalid API key."}}
	e := newExtractor(t, a)

	_, err := e.Extract(context.Background(), projectbrief.Input{
		Repository: "spf13__cobra", CommitSHA: testCommit, Dir: clone(t),
	})
	var failure *projectbrief.Failure
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want a Failure", err)
	}
	if failure.Kind != projectbrief.KindInvalidKey {
		t.Errorf("kind = %q, want %q", failure.Kind, projectbrief.KindInvalidKey)
	}
	if failure.Detail == "" {
		t.Error("the screen needs the API's own words, not a claim")
	}
}

func TestExtractFailsWhenTheAnswerDoesNotFollowTheSkeleton(t *testing.T) {
	a := &agent{out: &pig.Outcome{Answer: "El proyecto es una librería de CLI.\n"}}
	e := newExtractor(t, a)

	_, err := e.Extract(context.Background(), projectbrief.Input{
		Repository: "spf13__cobra", CommitSHA: testCommit, Dir: clone(t),
	})
	if err == nil {
		t.Fatal("prose is not a portrait")
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Errorf("the error must point at the line: %v", err)
	}
}

func TestExtractFailsOnAnIncompleteRun(t *testing.T) {
	a := &agent{out: &pig.Outcome{Stalled: true}}
	e := newExtractor(t, a)

	_, err := e.Extract(context.Background(), projectbrief.Input{
		Repository: "spf13__cobra", CommitSHA: testCommit, Dir: clone(t),
	})
	var failure *projectbrief.Failure
	if !errors.As(err, &failure) || failure.Kind != projectbrief.KindAgent {
		t.Fatalf("err = %v, want an agent failure", err)
	}
	if !strings.Contains(failure.Detail, "silent") {
		t.Errorf("detail = %q", failure.Detail)
	}
}

func TestExtractFailsWhenTheAgentFinishesWithoutAnAnswer(t *testing.T) {
	a := &agent{out: &pig.Outcome{Answer: "  "}}
	e := newExtractor(t, a)

	_, err := e.Extract(context.Background(), projectbrief.Input{
		Repository: "spf13__cobra", CommitSHA: testCommit, Dir: clone(t),
	})
	if err == nil {
		t.Fatal("an empty answer is not a portrait")
	}
}

// One portrait per repository at a time: two runs would read the same clone twice and rewrite the
// same agent home.
func TestExtractRunsOneRepositoryAtATime(t *testing.T) {
	release := make(chan struct{})
	var inFlight, peak int32
	a := &agent{}
	a.pause = func() {
		now := atomic.AddInt32(&inFlight, 1)
		for {
			old := atomic.LoadInt32(&peak)
			if now <= old || atomic.CompareAndSwapInt32(&peak, old, now) {
				break
			}
		}
		<-release
		atomic.AddInt32(&inFlight, -1)
	}
	e := newExtractor(t, a)
	dir := clone(t)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.Extract(context.Background(), projectbrief.Input{
				Repository: "spf13__cobra", CommitSHA: testCommit, Dir: dir,
			}); err != nil {
				t.Errorf("Extract: %v", err)
			}
		}()
	}
	// Let the first run enter and block, then release everybody.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := atomic.LoadInt32(&peak); got != 1 {
		t.Errorf("concurrent runs = %d, want 1", got)
	}
}

func TestNewRequiresThePartsOfARun(t *testing.T) {
	cases := map[string]projectbrief.Options{
		"no run":   {Model: testModel, Home: "h", Tools: []string{"read"}},
		"no model": {Run: (&agent{}).run, Home: "h", Tools: []string{"read"}},
		"no home":  {Run: (&agent{}).run, Model: testModel, Tools: []string{"read"}},
		"no tools": {Run: (&agent{}).run, Model: testModel, Home: "h"},
	}
	for name, opts := range cases {
		if _, err := projectbrief.New(opts); err == nil {
			t.Errorf("%s: New must refuse", name)
		}
	}
}

// replaceOnce swaps one citation for another, which is how a test fabricates the claim a strict
// audit has to catch.
func replaceOnce(t *testing.T, answer, old, replacement string) string {
	t.Helper()
	if !strings.Contains(answer, old) {
		t.Fatalf("%q is not in the answer", old)
	}
	return strings.Replace(answer, old, replacement, 1)
}

// outcomeOf is a completed run that answered with text.
func outcomeOf(answer string) *pig.Outcome {
	return &pig.Outcome{Answer: answer, Total: 41 * time.Second}
}

// A run costs minutes: when the answer follows no contract, the answer comes back with the failure
// so the next attempt can be diagnosed instead of being run blind.
func TestExtractHandsBackTheAnswerItCouldNotRead(t *testing.T) {
	answer := "El proyecto es una librería de CLI.\nNo sigue el contrato.\n"
	a := &agent{out: outcomeOf(answer)}
	e := newExtractor(t, a)

	_, err := e.Extract(context.Background(), projectbrief.Input{
		Repository: "spf13__cobra", CommitSHA: testCommit, Dir: clone(t),
	})
	var failure *projectbrief.Failure
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want a Failure", err)
	}
	if failure.Answer != answer {
		t.Errorf("answer = %q, want the model's own text", failure.Answer)
	}
	if !strings.Contains(failure.Detail, "line 1") {
		t.Errorf("detail = %q, want the parser's line", failure.Detail)
	}
}
