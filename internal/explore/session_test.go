package explore

import (
	"context"
	"errors"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/pig"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// answer is the smallest evidence object the contract accepts, so a test can run the whole
// session without an agent.
func answer() string {
	return `{"intent":"ignore hidden children when aligning usage",` +
		`"files":[{"path":"command.go","symbol":"AddCommand","line":1342,` +
		`"reason":"computes the padding","relation":"observed"}],` +
		`"call_paths":[["AddCommand","UsageString"]],` +
		`"tests":["command_test.go:TestPadding"],` +
		`"requirements":{"knowledge":["Go"],"project":["command.go"],"process":["PR with tests"]},` +
		`"blast_radius":{"files":1,"symbols":1,"public_api":false},` +
		`"unknowns":[],"missing_information":[]}`
}

func testInput() Input {
	return Input{
		Repository:    "o__r",
		NameWithOwner: "o/r",
		Dir:           "/nonexistent/clone",
		CommitSHA:     "abc123",
		Issue: ghdata.Issue{
			Number: 7,
			Title:  "hidden commands pad the usage",
			Body:   "the help is padded to hidden commands",
			Labels: []string{"good first issue"},
		},
	}
}

func newSession(t *testing.T, run func(context.Context, pig.Request) (*pig.Outcome, error)) (*Session, *store.Store) {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	return &Session{
		Store:  st,
		Model:  "deepseek-v4-flash",
		Models: []string{"deepseek-v4-flash"},
		Tools:  pig.DefaultTools(),
		Home:   t.TempDir(),
		Run:    run,
	}, st
}

func okRun(calls *int) func(context.Context, pig.Request) (*pig.Outcome, error) {
	return func(context.Context, pig.Request) (*pig.Outcome, error) {
		*calls++
		return &pig.Outcome{Answer: answer(), ExitCode: 0}, nil
	}
}

// S5: the key decides. An identical question is answered from the journal, in milliseconds,
// and the agent never runs again.
func TestAnalyzeReusesAnIdenticalQuestion(t *testing.T) {
	calls := 0
	sess, st := newSession(t, okRun(&calls))
	in := testInput()

	first, err := sess.Analyze(context.Background(), in)
	if err != nil {
		t.Fatalf("first analysis: %v", err)
	}
	if first.Reused {
		t.Fatal("the first analysis cannot be reused")
	}
	if first.Record == nil || first.Record.Evidence == nil {
		t.Fatal("the first analysis stored no evidence")
	}
	if first.Record.CommitSHA != in.CommitSHA || first.Record.Issue != in.Issue.Number {
		t.Fatalf("the stored record is not pinned: %+v", first.Record)
	}

	second, err := sess.Analyze(context.Background(), in)
	if err != nil {
		t.Fatalf("second analysis: %v", err)
	}
	if !second.Reused {
		t.Fatal("the same question must be reused, not paid for twice")
	}
	if calls != 1 {
		t.Fatalf("the agent ran %d times for one question", calls)
	}
	if second.Record.Evidence.Intent != first.Record.Evidence.Intent {
		t.Fatalf("the reused record differs from the stored one: %q", second.Record.Evidence.Intent)
	}
	if second.Previous == nil {
		t.Fatal("a reused result has to say when it was produced")
	}

	d, err := st.Decisions(in.Repository)
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	if d.Len() != 1 {
		t.Fatalf("the journal holds %d analyses, want 1", d.Len())
	}
}

// S5: everything that can change the answer is part of the question. A moved commit is
// another question, so it misses instead of reusing a verdict about another tree.
func TestAnalyzeRunsAgainWhenTheQuestionChanges(t *testing.T) {
	calls := 0
	sess, _ := newSession(t, okRun(&calls))
	in := testInput()
	if _, err := sess.Analyze(context.Background(), in); err != nil {
		t.Fatalf("first analysis: %v", err)
	}

	moved := in
	moved.CommitSHA = "def456"
	res, err := sess.Analyze(context.Background(), moved)
	if err != nil {
		t.Fatalf("analysis of the moved commit: %v", err)
	}
	if res.Reused {
		t.Fatal("a verdict over another commit answers another question")
	}
	if calls != 2 {
		t.Fatalf("the agent ran %d times, want 2", calls)
	}
}

// S9: canceling a run is not producing a result. Storing it would report an analysis nobody
// made, under the key of a question that was never answered.
func TestAnalyzeStoresNothingWhenTheCallerStopsIt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	run := func(context.Context, pig.Request) (*pig.Outcome, error) {
		cancel()
		return &pig.Outcome{Answer: answer(), ExitCode: -1}, nil
	}
	sess, st := newSession(t, run)

	if _, err := sess.Analyze(ctx, testInput()); !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	d, err := st.Decisions("o__r")
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	if d.Len() != 0 {
		t.Fatalf("a canceled run stored %d analyses", d.Len())
	}
}

// S5: -force ignores a stored answer on purpose, and says the run replaced it only when
// what was stored could not be read.
func TestForceRunsAgainAndReportsOnlyUnreadableAnswers(t *testing.T) {
	calls := 0
	sess, _ := newSession(t, okRun(&calls))
	in := testInput()
	if _, err := sess.Analyze(context.Background(), in); err != nil {
		t.Fatalf("first analysis: %v", err)
	}

	forced := in
	forced.Force = true
	res, err := sess.Analyze(context.Background(), forced)
	if err != nil {
		t.Fatalf("forced analysis: %v", err)
	}
	if res.Reused {
		t.Fatal("force must run the agent again")
	}
	if calls != 2 {
		t.Fatalf("the agent ran %d times, want 2", calls)
	}
	if res.StaleCache {
		t.Fatal("a readable stored answer is not stale just because it was ignored")
	}
}

// S13: the record pins the commit and the graph, so an analysis over another tree is
// labelled with evidence nobody read.
func TestIndexForRefusesAGraphFromAnotherCommit(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	in := testInput()

	if _, err := IndexFor(st, in); !errors.Is(err, ErrNoIndex) {
		t.Fatalf("a repository without a graph must be refused, got %v", err)
	}

	if err := st.SaveIndex(in.Repository, store.IndexRecord{
		Repository: in.Repository, CommitSHA: "other", Path: in.Dir,
	}); err != nil {
		t.Fatalf("save index: %v", err)
	}
	var stale *StaleIndexError
	if _, err := IndexFor(st, in); !errors.As(err, &stale) {
		t.Fatalf("a graph from another commit must be refused, got %v", err)
	}

	if err := st.SaveIndex(in.Repository, store.IndexRecord{
		Repository: in.Repository, CommitSHA: in.CommitSHA, Path: in.Dir,
	}); err != nil {
		t.Fatalf("save index: %v", err)
	}
	rec, err := IndexFor(st, in)
	if err != nil {
		t.Fatalf("a graph at the corpus commit must pass: %v", err)
	}
	if rec.Path != in.Dir {
		t.Fatalf("the index record lost the clone path: %q", rec.Path)
	}
}
