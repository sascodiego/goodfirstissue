package triage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	answers map[string]nan.Answer
	// alignment answers the node's questions, when the state is the alignment dossier.
	alignment map[string]nan.Answer
	err       error
	// failAlignment fails only the node's request, which is the smaller failure.
	failAlignment bool
	calls         int
	states        []string
	questions     []map[string]nan.Question
}

func (f *fakeJudge) Ask(_ context.Context, state string, questions map[string]nan.Question) (map[string]nan.Answer, error) {
	f.calls++
	f.states = append(f.states, state)
	f.questions = append(f.questions, questions)
	if f.err != nil {
		return nil, f.err
	}
	if strings.Contains(state, "PROJECT DIRECTION") {
		if f.failAlignment {
			return nil, errors.New("the alignment request failed")
		}
		return f.alignment, nil
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

// fakeBrief is the project's direction, recorded instead of read.
type fakeBrief struct {
	body  string
	err   error
	calls int
	repos []string
}

func (f *fakeBrief) Brief(_ context.Context, repository string) (string, error) {
	f.calls++
	f.repos = append(f.repos, repository)
	if f.err != nil {
		return "", f.err
	}
	return f.body, nil
}

// fakeSecondRound is the deferred round, installed to prove the seam.
type fakeSecondRound struct {
	result Result
	ok     bool
	err    error
	calls  int
}

func (f *fakeSecondRound) Resolve(_ context.Context, _ Issue, _ Analysis) (Result, bool, error) {
	f.calls++
	return f.result, f.ok, f.err
}

func aFeatureIssue() Issue {
	issue := aBug()
	issue.Title = "Add a --json flag to every command"
	issue.Body = "It would help scripts if every command could print JSON."
	return issue
}

func featureAnswers() map[string]nan.Answer {
	answers := wellFormed()
	answers[QKind] = choice(KindFeature, 0.9)
	return answers
}

func newP1With(t *testing.T, opts Options) *P1 {
	t.Helper()
	if opts.Model == "" {
		opts.Model = testModel
	}
	if opts.CommitSHA == "" {
		opts.CommitSHA = testCommit
	}
	p1, err := New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p1
}

// A bug is a deviation from what the project already wants, so there is nothing to ask the
// project's direction. Spending a request on it would be spending one for nothing.
func TestABugNeverAsksTheAlignmentNode(t *testing.T) {
	brief := &fakeBrief{body: "the project's direction"}
	judge := &fakeJudge{answers: wellFormed()}

	if _, err := newP1With(t, Options{Judge: judge, Brief: brief}).Run(context.Background(), aBug()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if brief.calls != 0 {
		t.Fatalf("the brief was read %d times for a bug", brief.calls)
	}
	if judge.calls != 1 {
		t.Fatalf("requests = %d, want 1", judge.calls)
	}
}

// The only branch a feature takes: a second request, over the alignment dossier, with the two
// frozen questions and the project's own direction inside the state.
func TestAFeatureAsksTheNodeWithTheProjectDirection(t *testing.T) {
	brief := &fakeBrief{body: "the project is moving toward scriptable output"}
	judge := &fakeJudge{
		answers:   featureAnswers(),
		alignment: map[string]nan.Answer{QOnlyForRequester: noul(0.1), QFitsDirection: noul(0.9)},
	}

	got, err := newP1With(t, Options{Judge: judge, Brief: brief}).Run(context.Background(), aFeatureIssue())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if judge.calls != 2 {
		t.Fatalf("requests = %d, want 2: the substance request and the node's", judge.calls)
	}
	if want := AlignmentDossier(aFeatureIssue(), brief.body); judge.states[1] != want {
		t.Fatalf("the node's state is not the alignment dossier:\n%s", judge.states[1])
	}
	if !strings.Contains(judge.states[1], brief.body) {
		t.Fatal("the project's direction did not reach the node")
	}
	if got, want := len(judge.questions[1]), len(AlignmentQuestions()); got != want {
		t.Fatalf("the node asked %d questions, want %d", got, want)
	}
	if got.Outcome != Candidate {
		t.Fatalf("outcome = %q (%s), want %q", got.Outcome, got.Reason, Candidate)
	}
	if len(got.Alignment) == 0 {
		t.Fatal("the analysis did not keep the node's answers")
	}
}

// Without a source for the direction the node cannot be asked, and the verdict says so instead of
// pretending the project has no position.
func TestAFeatureWithoutABriefSaysTheNodeWasUnavailable(t *testing.T) {
	judge := &fakeJudge{answers: featureAnswers()}

	got, err := newP1With(t, Options{Judge: judge}).Run(context.Background(), aFeatureIssue())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if judge.calls != 1 {
		t.Fatalf("requests = %d, want 1: the node cannot be asked", judge.calls)
	}
	if got.Outcome != Conditional || got.Reason != ReasonAlignmentUnavailable {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Conditional, ReasonAlignmentUnavailable)
	}
}

// A brief that cannot be read degrades the reading; it does not throw the verdict away. The
// degradation is reported, because a silent one is a silent lie about how well we read.
func TestABriefThatCannotBeReadDegradesTheVerdict(t *testing.T) {
	brief := &fakeBrief{err: errors.New("no clone")}
	judge := &fakeJudge{answers: featureAnswers()}

	got, err := newP1With(t, Options{Judge: judge, Brief: brief}).Run(context.Background(), aFeatureIssue())
	if err == nil {
		t.Fatal("a degraded reading must be reported")
	}
	if got.Outcome != Conditional || got.Reason != ReasonAlignmentUnavailable {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Conditional, ReasonAlignmentUnavailable)
	}
}

func TestAnAlignmentRequestThatFailsDegradesTheVerdict(t *testing.T) {
	judge := &fakeJudge{answers: featureAnswers(), failAlignment: true}

	got, err := newP1With(t, Options{Judge: judge, Brief: &fakeBrief{body: "direction"}}).Run(context.Background(), aFeatureIssue())
	if err == nil {
		t.Fatal("a failed node request must be reported")
	}
	if got.Outcome != Conditional || got.Reason != ReasonAlignmentUnavailable {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Conditional, ReasonAlignmentUnavailable)
	}
}

// The seam, uninstalled: a question is DERIVADA and nothing else happens. This is the state the
// product ships in.
func TestAQuestionStaysDerivedWithoutTheSecondRound(t *testing.T) {
	answers := wellFormed()
	answers[QKind] = choice(KindQuestion, 0.9)
	judge := &fakeJudge{answers: answers}

	got, err := newP1With(t, Options{Judge: judge}).Run(context.Background(), aBug())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Outcome != Derived {
		t.Fatalf("outcome = %q (%s), want %q", got.Outcome, got.Reason, Derived)
	}
}

// The seam, installed: installing the round later is one type and one field, and this test is what
// makes that claim true instead of aspirational.
func TestTheSecondRoundCanSettleAQuestion(t *testing.T) {
	answers := wellFormed()
	answers[QKind] = choice(KindQuestion, 0.9)
	judge := &fakeJudge{answers: answers}
	round := &fakeSecondRound{result: Result{Outcome: Candidate, Probability: 0.8}, ok: true}

	got, err := newP1With(t, Options{Judge: judge, SecondRound: round}).Run(context.Background(), aBug())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if round.calls != 1 {
		t.Fatalf("the round ran %d times, want 1", round.calls)
	}
	if got.Outcome != Candidate {
		t.Fatalf("outcome = %q, want the round's %q", got.Outcome, Candidate)
	}
	if judge.calls != 1 {
		t.Fatalf("the first round asked %d times, want 1", judge.calls)
	}
}

// A round that cannot settle the issue leaves the first verdict alone: DERIVADA is a valid
// outcome, not a pending state that a missing answer can erase.
func TestASecondRoundThatCannotSettleLeavesTheFirstVerdict(t *testing.T) {
	answers := wellFormed()
	answers[QKind] = choice(KindQuestion, 0.9)

	got, err := newP1With(t, Options{Judge: &fakeJudge{answers: answers}, SecondRound: &fakeSecondRound{ok: false}}).Run(context.Background(), aBug())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Outcome != Derived {
		t.Fatalf("outcome = %q, want %q", got.Outcome, Derived)
	}
}

func TestASecondRoundThatFailsDoesNotLoseTheVerdict(t *testing.T) {
	answers := wellFormed()
	answers[QKind] = choice(KindQuestion, 0.9)
	round := &fakeSecondRound{err: errors.New("round failed")}

	got, err := newP1With(t, Options{Judge: &fakeJudge{answers: answers}, SecondRound: round}).Run(context.Background(), aBug())
	if err == nil {
		t.Fatal("a failed round must be reported")
	}
	if got.Outcome != Derived {
		t.Fatalf("outcome = %q, want %q", got.Outcome, Derived)
	}
}

// The round runs only for what it is for. A bug must not reach it, and neither must a pre-gated
// issue: both are already settled.
func TestTheSecondRoundRunsOnlyForADerivedQuestion(t *testing.T) {
	round := &fakeSecondRound{result: Result{Outcome: Candidate}, ok: true}

	if _, err := newP1With(t, Options{Judge: &fakeJudge{answers: wellFormed()}, SecondRound: round}).Run(context.Background(), aBug()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	issue := aBug()
	issue.GitHubState, issue.GitHubStateReason = "CLOSED", "COMPLETED"
	if _, err := newP1With(t, Options{Judge: &fakeJudge{answers: wellFormed()}, SecondRound: round}).Run(context.Background(), issue); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if round.calls != 0 {
		t.Fatalf("the round ran %d times for issues it cannot settle", round.calls)
	}
}

// A cached verdict keeps the node's answers: the interface shows the same evidence a fresh run
// showed, and a reuse that dropped it would silently lose the reason for the verdict.
func TestTheCachedVerdictKeepsTheNodeEvidence(t *testing.T) {
	journal, err := newTestStore(t).Decisions("spf13/cobra")
	if err != nil {
		t.Fatalf("Decisions: %v", err)
	}
	opts := func() Options {
		return Options{
			Judge: &fakeJudge{
				answers:   featureAnswers(),
				alignment: map[string]nan.Answer{QOnlyForRequester: noul(0.1), QFitsDirection: noul(0.9)},
			},
			Brief:   &fakeBrief{body: "direction"},
			Journal: journal,
		}
	}

	if _, err := newP1With(t, opts()).Run(context.Background(), aFeatureIssue()); err != nil {
		t.Fatalf("first run: %v", err)
	}

	second := opts()
	second.Judge = &fakeJudge{err: errors.New("the judge must not be called again")}
	got, err := newP1With(t, second).Run(context.Background(), aFeatureIssue())
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if !got.Cached {
		t.Fatal("the verdict was not reused")
	}
	if _, ok := got.Alignment[QFitsDirection]; !ok {
		t.Fatal("the cached verdict lost the node's answers")
	}
}

// A pre-gate resolves without a request, and it still resolves: the verdict is journaled like
// any other one. A report that reads the journal would otherwise count an issue the pipeline
// already answered as one nobody has looked at (issue #41).
func TestAPreResolvedIssueIsStoredSoTheJournalCanCountIt(t *testing.T) {
	st := newTestStore(t)
	journal, err := st.Decisions("spf13/cobra")
	if err != nil {
		t.Fatalf("Decisions: %v", err)
	}

	judge := &fakeJudge{err: errors.New("the judge must not be called")}
	p1 := newP1(t, judge, journal)

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
	if got.Cached {
		t.Fatal("a pre-gate computed on the spot reported itself as reused")
	}

	// The key is assembled by hand rather than with p1.key: a test that asks the code under
	// test what the key is cannot show that the key is the whole question (I4, I14).
	key := store.Key{
		Issue:         issue.Number,
		Pass:          store.PassTriage,
		Model:         testModel,
		PromptVersion: Version(),
		CommitSHA:     testCommit,
	}
	decision, ok := journal.Get(key)
	if !ok {
		t.Fatalf("the pre-gate verdict was not journaled under %s", key.ID())
	}
	if decision.LatencyMs != 0 {
		t.Fatalf("latency = %dms, want 0: the verdict cost no request", decision.LatencyMs)
	}

	var entry stored
	if err := json.Unmarshal(decision.Verdict, &entry); err != nil {
		t.Fatalf("verdict: %v", err)
	}
	if entry.Result.Outcome != Marked || entry.Result.Reason != ReasonAlreadyDone || entry.Result.Probability != 1 {
		t.Fatalf("stored %q (%s) at %v, want %q (%s) at 1",
			entry.Result.Outcome, entry.Result.Reason, entry.Result.Probability, Marked, ReasonAlreadyDone)
	}
	if len(entry.Answers) != 0 {
		t.Fatalf("the pre-gate stored %d answers, want none", len(entry.Answers))
	}
}

// The second run of a pre-resolved issue is answered from the journal: same verdict, the time
// it was first produced, and one line in the journal instead of two. Presenting a stored answer
// as fresh is what I14 forbids, and `Put` appends without deduplicating, so a second `remember`
// would grow the journal.
func TestAPreResolvedIssueIsAnsweredFromTheJournalOnTheNextRun(t *testing.T) {
	st := newTestStore(t)
	journal, err := st.Decisions("spf13/cobra")
	if err != nil {
		t.Fatalf("Decisions: %v", err)
	}

	issue := aBug()
	issue.GitHubState = "CLOSED"
	issue.GitHubStateReason = "COMPLETED"

	at := time.Date(2026, 11, 15, 3, 0, 0, 0, time.UTC)
	first := newP1With(t, Options{
		Judge:   &fakeJudge{err: errors.New("the judge must not be called")},
		Journal: journal,
		Now:     func() time.Time { return at },
	})
	if _, err := first.Run(context.Background(), issue); err != nil {
		t.Fatalf("first run: %v", err)
	}

	path := st.Path(filepath.Join("spf13", "cobra", "decisions.jsonl"))
	if n := journalLines(t, path); n != 1 {
		t.Fatalf("the journal holds %d lines after one run, want 1", n)
	}

	second := newP1With(t, Options{
		Judge:   &fakeJudge{err: errors.New("the judge must not be called again")},
		Journal: journal,
		Now:     func() time.Time { return at.Add(time.Hour) },
	})
	again, err := second.Run(context.Background(), issue)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if again.Cached != true {
		t.Fatal("a pre-gate answered from the journal must say so")
	}
	if !again.At.Equal(at) {
		t.Fatalf("cached at %v, first produced at %v: a stored answer keeps its own time", again.At, at)
	}
	if again.Outcome != Marked || again.Reason != ReasonAlreadyDone {
		t.Fatalf("cached %q (%s), want %q (%s)", again.Outcome, again.Reason, Marked, ReasonAlreadyDone)
	}
	if n := journalLines(t, path); n != 1 {
		t.Fatalf("the journal holds %d lines after two runs, want 1", n)
	}
}

// The journal is pinned to the commit and the prompt, not to the issue's own facts: a corpus
// refreshed at the same commit can bring a pull request that another person opened. The
// deterministic fact wins over the stored verdict, because recommending taken work is the one
// mistake the pre-gate exists to prevent (§7.7, row 0 before row 1).
func TestAPreResolvedIssueWhoseNativeFactMovedIsWrittenAgain(t *testing.T) {
	st := newTestStore(t)
	journal, err := st.Decisions("spf13/cobra")
	if err != nil {
		t.Fatalf("Decisions: %v", err)
	}

	judge := &fakeJudge{err: errors.New("the judge must not be called")}
	p1 := newP1(t, judge, journal)

	empty := aBug()
	empty.Body = ""
	if _, err := p1.Run(context.Background(), empty); err != nil {
		t.Fatalf("first run: %v", err)
	}

	working := aBug()
	working.LinkedPRs = []LinkedPR{{Number: 7, State: "OPEN"}}

	again, err := p1.Run(context.Background(), working)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if again.Reason != ReasonAlreadyWorking {
		t.Fatalf("reason = %q, want %q: the native fact moved and wins over the stored reason",
			again.Reason, ReasonAlreadyWorking)
	}
	if again.Cached {
		t.Fatal("a verdict recomputed from a fact that moved is not reused")
	}

	key := store.Key{
		Issue:         working.Number,
		Pass:          store.PassTriage,
		Model:         testModel,
		PromptVersion: Version(),
		CommitSHA:     testCommit,
	}
	decision, ok := journal.Get(key)
	if !ok {
		t.Fatalf("the moved verdict was not journaled under %s", key.ID())
	}
	var entry stored
	if err := json.Unmarshal(decision.Verdict, &entry); err != nil {
		t.Fatalf("verdict: %v", err)
	}
	if entry.Result.Reason != ReasonAlreadyWorking {
		t.Fatalf("the journal still answers %q, want %q", entry.Result.Reason, ReasonAlreadyWorking)
	}
}

// journalLines counts the rows a run actually wrote. Len() counts distinct keys, and Put
// appends without deduplicating, so only the file shows whether a run wrote once or twice.
func journalLines(t *testing.T, path string) int {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	return strings.Count(string(body), "\n")
}

// A screen reads a verdict without running the pass, so the reading has to be one function that
// builds the same key the writer built (I4, I14). If it were two, a reader would eventually ask a
// different question than the one the journal answered.
func TestRecallReturnsTheVerdictTheJournalHoldsForTheFullQuestion(t *testing.T) {
	st := newTestStore(t)
	journal, err := st.Decisions("spf13/cobra")
	if err != nil {
		t.Fatalf("Decisions: %v", err)
	}

	judge := &fakeJudge{answers: wellFormed()}
	if _, err := newP1(t, judge, journal).Run(context.Background(), aBug()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	recalled, decision, ok := Recall(journal, aBug().Number, testModel, testCommit)
	if !ok {
		t.Fatal("Recall missed a verdict the journal holds for this exact question")
	}
	if !recalled.Cached {
		t.Fatal("a recalled verdict must say it was reused (I14)")
	}
	if recalled.Outcome != Candidate {
		t.Fatalf("outcome = %q (%s), want %q", recalled.Outcome, recalled.Reason, Candidate)
	}
	if len(recalled.Answers) != len(Questions()) {
		t.Fatalf("recalled %d answers, want %d: the screen shows what was asked",
			len(recalled.Answers), len(Questions()))
	}
	if !recalled.At.Equal(decision.At) {
		t.Fatalf("recalled at %v, stored at %v", recalled.At, decision.At)
	}

	// The pin is part of the question: another model answers something else.
	if _, _, ok := Recall(journal, aBug().Number, "another-model", testCommit); ok {
		t.Fatal("Recall answered for a model that never produced this verdict")
	}
	if _, _, ok := Recall(nil, aBug().Number, testModel, testCommit); ok {
		t.Fatal("Recall answered without a journal")
	}
}
