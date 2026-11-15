package triage

import (
	"context"
	"errors"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// The pipeline is where the product and the benchmark meet: one function, one request, one
// stored verdict. These tests fix the answers and count the calls, so the network is never
// part of the proof (S8).

const (
	testModel  = "clef"
	testCommit = "0123456789abcdef0123456789abcdef01234567"
)

// fakeJudge is the whole transport, replaced by a recorded answer.
type fakeJudge struct {
	answers   map[string]nan.Answer
	err       error
	calls     int
	states    []string
	questions []map[string]nan.Question
}

func (f *fakeJudge) Ask(_ context.Context, state string, questions map[string]nan.Question) (map[string]nan.Answer, error) {
	f.calls++
	f.states = append(f.states, state)
	f.questions = append(f.questions, questions)
	if f.err != nil {
		return nil, f.err
	}
	return f.answers, nil
}

func newP1(t *testing.T, judge Judge, journal *store.Decisions) *P1 {
	t.Helper()
	p1, err := New(Options{Judge: judge, Journal: journal, Model: testModel, CommitSHA: testCommit})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p1
}

func TestAResolvedIssueSpendsNoRequest(t *testing.T) {
	judge := &fakeJudge{err: errors.New("the judge must not be called")}
	p1 := newP1(t, judge, nil)

	issue := aBug()
	issue.GitHubState = "CLOSED"
	issue.GitHubStateReason = "COMPLETED"

	got, err := p1.Run(context.Background(), issue)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if judge.calls != 0 {
		t.Fatalf("the judge was called %d times for an issue a pre-gate resolves", judge.calls)
	}
	if got.Outcome != Marked || got.Reason != ReasonAlreadyDone {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Marked, ReasonAlreadyDone)
	}
}

// The pipeline asks the production set and sends the substance dossier: the same two inputs
// the benchmark uses, which is the reason no divergence is possible (S1).
func TestThePipelineSendsTheFrozenQuestionsOverTheSubstanceDossier(t *testing.T) {
	judge := &fakeJudge{answers: wellFormed()}
	p1 := newP1(t, judge, nil)

	issue := aBug()
	issue.Labels = []string{"good first issue", "bug"}

	if _, err := p1.Run(context.Background(), issue); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(judge.questions) != 1 {
		t.Fatalf("asked %d times, want 1", len(judge.questions))
	}
	if got, want := len(judge.questions[0]), len(Questions()); got != want {
		t.Fatalf("asked %d questions, want the frozen %d", got, want)
	}
	for id := range Questions() {
		if _, asked := judge.questions[0][id]; !asked {
			t.Errorf("question %q was not asked", id)
		}
	}
	if want := SubstanceDossier(issue); judge.states[0] != want {
		t.Fatalf("the state sent is not the substance dossier:\n%s", judge.states[0])
	}
	if containsLabel(judge.states[0], issue.Labels) {
		t.Fatal("the labels reached the state under judgement")
	}
}

// I2: a request that failed is not a verdict about the issue. The result is the one the
// product shows, and the error is for the log.
func TestAFailedRequestIsUndecidedAndNotAVerdict(t *testing.T) {
	judge := &fakeJudge{err: errors.New("503")}
	p1 := newP1(t, judge, nil)

	got, err := p1.Run(context.Background(), aBug())
	if err == nil {
		t.Fatal("a failure must be reported to the caller")
	}
	if got.Outcome != Undecided || got.Reason != ReasonEndpointFailed {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Undecided, ReasonEndpointFailed)
	}
}

func TestAFreshAnalysisIsStoredAndThenReused(t *testing.T) {
	st := newTestStore(t)
	journal, err := st.Decisions("spf13/cobra")
	if err != nil {
		t.Fatalf("Decisions: %v", err)
	}

	first := &fakeJudge{answers: wellFormed()}
	got, err := newP1(t, first, journal).Run(context.Background(), aBug())
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if got.Cached {
		t.Fatal("a fresh analysis reported itself as cached")
	}
	if got.Outcome != Candidate {
		t.Fatalf("outcome = %q (%s), want %q", got.Outcome, got.Reason, Candidate)
	}

	// The second run must not reach the judge at all: a stored verdict is the whole point.
	second := &fakeJudge{err: errors.New("the judge must not be called again")}
	again, err := newP1(t, second, journal).Run(context.Background(), aBug())
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if second.calls != 0 {
		t.Fatalf("the judge was called %d times for a cached verdict", second.calls)
	}
	if !again.Cached {
		t.Fatal("a reused verdict must say so: the interface never presents a cache as fresh (I14)")
	}
	if again.Outcome != got.Outcome || again.Probability != got.Probability {
		t.Fatalf("cached %q/%v, fresh %q/%v", again.Outcome, again.Probability, got.Outcome, got.Probability)
	}
}

// A verdict produced by another model, another prompt or another commit answers another
// question, so it has to miss instead of being reused (I14).
func TestTheCacheKeyMovesWithTheModelThePromptAndTheCommit(t *testing.T) {
	tests := []struct {
		name  string
		build func(*Options)
	}{
		{"another model", func(o *Options) { o.Model = "gemma4" }},
		{"another commit", func(o *Options) { o.CommitSHA = "fedcba9876543210fedcba9876543210fedcba98" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newTestStore(t)
			journal, err := st.Decisions("spf13/cobra")
			if err != nil {
				t.Fatalf("Decisions: %v", err)
			}

			first := &fakeJudge{answers: wellFormed()}
			if _, err := newP1(t, first, journal).Run(context.Background(), aBug()); err != nil {
				t.Fatalf("first run: %v", err)
			}

			opts := Options{Judge: &fakeJudge{answers: wellFormed()}, Journal: journal, Model: testModel, CommitSHA: testCommit}
			tt.build(&opts)
			other, err := New(opts)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got, err := other.Run(context.Background(), aBug())
			if err != nil {
				t.Fatalf("second run: %v", err)
			}
			if got.Cached {
				t.Fatal("a verdict from another question was reused")
			}
		})
	}
}

// An entry that does not carry a verdict is not a verdict: serving it would show an empty
// outcome, which is worse than spending one request. It is a miss, and the pipeline asks again.
func TestAnEntryWithoutAVerdictIsAMiss(t *testing.T) {
	st := newTestStore(t)
	journal, err := st.Decisions("spf13/cobra")
	if err != nil {
		t.Fatalf("Decisions: %v", err)
	}
	key := store.Key{Issue: aBug().Number, Pass: store.PassTriage, Model: testModel, PromptVersion: Version(), CommitSHA: testCommit}
	if err := journal.Put(store.Decision{Key: key, Verdict: []byte(`{"unexpected":true}`)}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	judge := &fakeJudge{answers: wellFormed()}
	got, err := newP1(t, judge, journal).Run(context.Background(), aBug())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if judge.calls != 1 {
		t.Fatalf("judge calls = %d, want a fresh request after a damaged entry", judge.calls)
	}
	if got.Cached {
		t.Fatal("a damaged entry was served as an answer")
	}
}

// The pin is not decoration: a journal entry with no commit in its key would be reused across
// two different trees (I4).
func TestCachingWithoutACommitIsRefused(t *testing.T) {
	st := newTestStore(t)
	journal, err := st.Decisions("spf13/cobra")
	if err != nil {
		t.Fatalf("Decisions: %v", err)
	}

	if _, err := New(Options{Judge: &fakeJudge{}, Journal: journal, Model: testModel}); err == nil {
		t.Fatal("New accepted a journal with no commit to pin the verdict to")
	}
}

func TestAModelIsRequired(t *testing.T) {
	if _, err := New(Options{Judge: &fakeJudge{}, Model: " "}); err == nil {
		t.Fatal("New accepted an empty model: this package never picks one (I13)")
	}
}

func TestAJudgeIsRequired(t *testing.T) {
	if _, err := New(Options{Model: testModel}); err == nil {
		t.Fatal("New accepted no judge")
	}
}

// The analysis says which question it answered: the version, the model and the commit travel
// with the verdict so a stale value can never be presented as fresh (I14).
func TestTheAnalysisNamesTheQuestionItAnswered(t *testing.T) {
	judge := &fakeJudge{answers: wellFormed()}
	got, err := newP1(t, judge, nil).Run(context.Background(), aBug())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Model != testModel || got.PromptVersion != Version() || got.CommitSHA != testCommit {
		t.Fatalf("analysis pinned to %q/%q/%q", got.Model, got.PromptVersion, got.CommitSHA)
	}
	if len(got.Answers) != len(Questions()) {
		t.Fatalf("the analysis kept %d answers, want %d", len(got.Answers), len(Questions()))
	}
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return st
}
