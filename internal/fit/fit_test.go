package fit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// The pass is where the product and the policy meet: one request, one level, one stored row. These
// tests fix the answers and count the calls, so the network is never part of the proof.

const (
	testModel  = "clef"
	testCommit = "0123456789abcdef0123456789abcdef01234567"
	// otherProfile is a different person: the same issue asked against it is another question.
	otherProfile = `## Conocimientos técnicos
- Rust — nivel: declarado — evidencia: «un curso»`
)

// fakeJudge is the whole transport, replaced by a recorded answer.
type fakeJudge struct {
	answers map[string]nan.Answer
	err     error
	calls   int
	states  []string
}

func (f *fakeJudge) Ask(_ context.Context, state string, _ map[string]nan.Question) (map[string]nan.Answer, error) {
	f.calls++
	f.states = append(f.states, state)
	if f.err != nil {
		return nil, f.err
	}
	return f.answers, nil
}

func newJournal(t *testing.T) *store.Decisions {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	journal, err := st.Decisions("spf13__cobra")
	if err != nil {
		t.Fatalf("Decisions: %v", err)
	}
	return journal
}

func newP3(t *testing.T, opts Options) *P3 {
	t.Helper()
	if opts.Judge == nil {
		opts.Judge = &fakeJudge{answers: answered(FitReady, 0.9)}
	}
	if opts.Model == "" {
		opts.Model = testModel
	}
	if opts.CommitSHA == "" {
		opts.CommitSHA = testCommit
	}
	if opts.Profile == "" {
		opts.Profile = testProfile
	}
	p3, err := New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p3
}

// New fails closed on every setting that would produce a verdict nobody can attribute to a
// question: no transport, no model, and a journal the verdict cannot be pinned in.
func TestNewFailsClosed(t *testing.T) {
	cases := map[string]Options{
		"no judge":                {Model: testModel, CommitSHA: testCommit, Profile: testProfile},
		"no model":                {Judge: &fakeJudge{}, CommitSHA: testCommit, Profile: testProfile},
		"a blank model":           {Judge: &fakeJudge{}, Model: "   ", CommitSHA: testCommit, Profile: testProfile},
		"a journal with no pin":   {Judge: &fakeJudge{}, Model: testModel, Journal: newJournal(t), Profile: testProfile},
		"a journal with no CV":    {Judge: &fakeJudge{}, Model: testModel, CommitSHA: testCommit, Journal: newJournal(t)},
		"a journal and no commit": {Judge: &fakeJudge{}, Model: testModel, Journal: newJournal(t), Profile: testProfile},
	}
	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			if p3, err := New(opts); err == nil {
				t.Fatalf("New accepted %+v and returned %v", opts, p3)
			}
		})
	}

	if _, err := New(Options{Judge: &fakeJudge{}, Model: testModel, CommitSHA: testCommit, Profile: testProfile}); err != nil {
		t.Fatalf("New refused a complete set of settings: %v", err)
	}
}

// I4, I14: the key names the question. The pass is fit, the commit is the pin, and the prompt
// version is the contract's fingerprint.
func TestTheKeyCarriesTheFitPassAndThePin(t *testing.T) {
	key := keyFor(13, testModel, promptVersion(testProfile), testCommit)

	if key.Pass != store.PassFit {
		t.Errorf("pass = %q, want %q", key.Pass, store.PassFit)
	}
	if key.Issue != 13 || key.Model != testModel || key.CommitSHA != testCommit {
		t.Errorf("key = %+v, want the issue, the model and the commit it was asked at", key)
	}
	if !strings.HasPrefix(key.PromptVersion, Version()) {
		t.Errorf("prompt version %q does not start with the contract's %q", key.PromptVersion, Version())
	}
}

// S6: the same issue against another CV is another question. The profile's fingerprint goes inside
// PromptVersion, because the contract of store.Key does not change.
func TestAnotherProfileIsAnotherPromptVersion(t *testing.T) {
	if promptVersion(testProfile) == promptVersion(otherProfile) {
		t.Fatal("two different profiles produced the same prompt version: a stored verdict would be reused as fresh (I14)")
	}
	if !strings.HasPrefix(promptVersion(otherProfile), Version()) {
		t.Error("the profile's fingerprint replaced the contract's fingerprint instead of following it")
	}
	if promptVersion(testProfile) == Version() {
		t.Error("the profile left no trace in the prompt version")
	}
}

// The second run must not reach the judge at all: a stored verdict is the whole point.
func TestASecondRunCostsNoRequestAndSaysItWasReused(t *testing.T) {
	journal := newJournal(t)
	judge := &fakeJudge{answers: answered(FitReady, 0.9)}
	p3 := newP3(t, Options{Judge: judge, Journal: journal})

	first, err := p3.Run(context.Background(), aRequest())
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if first.Cached {
		t.Fatal("a fresh verdict reported itself as reused")
	}
	if first.Level != LevelRecommended {
		t.Fatalf("level = %q (%s), want %q", first.Level, first.Reason, LevelRecommended)
	}

	second, err := p3.Run(context.Background(), aRequest())
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if judge.calls != 1 {
		t.Fatalf("the judge was called %d times, want 1", judge.calls)
	}
	if !second.Cached {
		t.Fatal("the second run did not report itself as reused")
	}
	if second.Level != first.Level || second.Reason != first.Reason || second.Probability != first.Probability {
		t.Errorf("the reused verdict is %+v, want the stored %+v", second.Result, first.Result)
	}
	if !second.At.Equal(first.At) {
		t.Errorf("a reused verdict reports %v as its time, want the original %v: a stored answer is never presented as fresh (I14)", second.At, first.At)
	}
}

// The same issue, another CV: the question changed, so the journal must miss. Two people sharing
// one store is the normal case, and one person's verdict is never the other's.
func TestAnotherProfileMissesTheJournal(t *testing.T) {
	journal := newJournal(t)
	judge := &fakeJudge{answers: answered(FitReady, 0.9)}

	first := newP3(t, Options{Judge: judge, Journal: journal, Profile: testProfile})
	if _, err := first.Run(context.Background(), aRequest()); err != nil {
		t.Fatalf("first run: %v", err)
	}

	second := newP3(t, Options{Judge: judge, Journal: journal, Profile: otherProfile})
	got, err := second.Run(context.Background(), aRequest())
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if judge.calls != 2 {
		t.Fatalf("the judge was called %d times, want 2: another CV is another question", judge.calls)
	}
	if got.Cached {
		t.Fatal("a verdict produced for another profile was reused")
	}
	if journal.Len() != 2 {
		t.Errorf("the journal holds %d rows, want one per profile", journal.Len())
	}
}

// S7: without an approved profile there is no comparison, so no request and no invented answer.
func TestWithoutAProfileTheLevelIsInsufficientInformationAndCostsNoRequest(t *testing.T) {
	judge := &fakeJudge{err: errors.New("the judge must not be called without a profile")}
	// Built without the helper: the helper fills in a profile, and this is the one case that needs
	// no profile at all.
	p3, err := New(Options{Judge: judge, Model: testModel, CommitSHA: testCommit})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := p3.Run(context.Background(), aRequest())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if judge.calls != 0 {
		t.Fatalf("the judge was called %d times without a profile", judge.calls)
	}
	if got.Level != LevelInsufficientInformation {
		t.Fatalf("level = %q (%s), want %q", got.Level, got.Reason, LevelInsufficientInformation)
	}
	if got.Reason != ReasonNoProfile {
		t.Errorf("reason = %q, want %q: what is missing is the profile", got.Reason, ReasonNoProfile)
	}
	if got.Answer != "" {
		t.Errorf("answer = %q, want none: nobody was asked", got.Answer)
	}
	if got.Probability != 1 {
		t.Errorf("probability = %v, want 1: a level reached without asking is certain", got.Probability)
	}
}

// I2: a request that failed is not a verdict about the person or the issue. It is not stored
// either, because a stored row would make a non-answer look like a pinned decision.
func TestAFailedRequestIsUndecidedAndNotStored(t *testing.T) {
	journal := newJournal(t)
	judge := &fakeJudge{err: errors.New("503")}
	p3 := newP3(t, Options{Judge: judge, Journal: journal})

	got, err := p3.Run(context.Background(), aRequest())
	if err == nil {
		t.Fatal("a failure must be reported to the caller")
	}
	if got.Level != LevelUndecided || got.Reason != ReasonEndpointFailed {
		t.Fatalf("level = %q (%s), want %q (%s)", got.Level, got.Reason, LevelUndecided, ReasonEndpointFailed)
	}
	if journal.Len() != 0 {
		t.Fatalf("a failed request was stored as a verdict (%d rows)", journal.Len())
	}

	if _, err := p3.Run(context.Background(), aRequest()); err == nil {
		t.Fatal("the second run must retry instead of reading a stored failure")
	}
	if judge.calls != 2 {
		t.Fatalf("the judge was called %d times, want 2: nothing was stored to skip", judge.calls)
	}
}

// The same rule for an answer we cannot read: it is a defect of the response, and asking again is
// how the next answer gets read.
func TestAnUnreadableAnswerIsNotStored(t *testing.T) {
	journal := newJournal(t)
	judge := &fakeJudge{answers: map[string]nan.Answer{"other": {Type: nan.Choice, Choice: string(FitReady), Confidence: 0.9}}}
	p3 := newP3(t, Options{Judge: judge, Journal: journal})

	got, err := p3.Run(context.Background(), aRequest())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Level != LevelUndecided || got.Reason != ReasonIncompleteAnswer {
		t.Fatalf("level = %q (%s), want %q (%s)", got.Level, got.Reason, LevelUndecided, ReasonIncompleteAnswer)
	}
	if journal.Len() != 0 {
		t.Fatalf("an unreadable answer was stored as a verdict (%d rows)", journal.Len())
	}
}

// A verdict that is a verdict travels with everything needed to attribute it (I4).
func TestAFreshVerdictIsStoredWithItsPin(t *testing.T) {
	p3 := newP3(t, Options{})
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	p3.now = func() time.Time { return at }

	got, err := p3.Run(context.Background(), aRequest())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Model != testModel {
		t.Errorf("model = %q, want %q", got.Model, testModel)
	}
	if got.PromptVersion != promptVersion(testProfile) {
		t.Errorf("prompt version = %q, want the contract plus the profile's fingerprint", got.PromptVersion)
	}
	if got.CommitSHA != testCommit {
		t.Errorf("commit = %q, want %q", got.CommitSHA, testCommit)
	}
	if !got.At.Equal(at) {
		t.Errorf("at = %v, want %v", got.At, at)
	}
}

// The judge reads exactly the state of §9.1, composed from this request and this profile: the
// question and the text it is asked about cannot drift apart.
func TestTheJudgeReadsTheComposedState(t *testing.T) {
	judge := &fakeJudge{answers: answered(FitReadyWithSupport, 0.8)}
	p3 := newP3(t, Options{Judge: judge, Portrait: testPortrait})

	if _, err := p3.Run(context.Background(), aRequest()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(judge.states) != 1 {
		t.Fatalf("the judge saw %d states, want 1", len(judge.states))
	}
	if want := State(aRequest(), testProfile, testPortrait); judge.states[0] != want {
		t.Errorf("the state sent is not the composed one:\n--- got ---\n%s\n--- want ---\n%s", judge.states[0], want)
	}
}

// The read side answers the same question the write side stored, and a miss on the profile is a
// miss: a screen showing another person's verdict would be the worst failure of the journal (I14).
func TestRecallServesTheStoredVerdictAndMissesOnAnotherProfile(t *testing.T) {
	journal := newJournal(t)
	p3 := newP3(t, Options{Journal: journal})
	first, err := p3.Run(context.Background(), aRequest())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got, decision, ok := Recall(journal, aRequest().Issue.Number, testModel, testCommit, testProfile)
	if !ok {
		t.Fatal("the stored verdict was not found")
	}
	if !got.Cached {
		t.Error("a recalled verdict did not report itself as reused")
	}
	if got.Level != first.Level || !decision.At.Equal(first.At) {
		t.Errorf("recalled %+v at %v, want the stored %+v at %v", got.Result, decision.At, first.Result, first.At)
	}

	if _, _, ok := Recall(journal, aRequest().Issue.Number, testModel, testCommit, otherProfile); ok {
		t.Fatal("a verdict stored for one profile was returned for another")
	}
	if _, _, ok := Recall(nil, aRequest().Issue.Number, testModel, testCommit, testProfile); ok {
		t.Fatal("a journal that does not exist answered with a verdict")
	}
}

// S9: the label is not a fit criterion. The same CV against the same issue with another label is
// the same question, and it has to produce the same level and the same state — the state check is
// the stronger half, because it says the judge never saw the difference either.
func TestTheLabelDoesNotChangeTheLevel(t *testing.T) {
	judge := &fakeJudge{answers: answered(FitReadyWithSupport, 0.8)}
	p3 := newP3(t, Options{Judge: judge})

	labelled := aRequest()
	labelled.Issue.Labels = []string{"good first issue", "difficulty: easy"}
	unlabelled := aRequest()
	unlabelled.Issue.Labels = []string{"help wanted", "needs design"}

	first, err := p3.Run(context.Background(), labelled)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := p3.Run(context.Background(), unlabelled)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	if first.Result != second.Result {
		t.Errorf("the label moved the verdict: %+v against %+v", first.Result, second.Result)
	}
	if judge.states[0] != judge.states[1] {
		t.Error("the label reached the state: two labels produced two different questions")
	}
}
