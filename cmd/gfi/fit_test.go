package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/config"
	"github.com/sascodiego/nan-hackathon-clef/internal/explore"
	"github.com/sascodiego/nan-hackathon-clef/internal/fit"
	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// These tests never touch the network: the judge, the profile, the exploration evidence and the
// portrait are injected. The pass, the journal, the policy and the first pass are the production
// ones, so a disagreement with the product cannot hide behind a test double.
const (
	// fitProfile is the approved profile the third pass compares against. A second document is a
	// second question, not a fresher answer (S6).
	fitProfile = `## Conocimientos técnicos
- Go — nivel: demostrado — evidencia: «tres años de Go»
`
	otherFitProfile = `## Conocimientos técnicos
- Rust — nivel: demostrado — evidencia: «cinco años de Rust»
`
	// fitExploreModel is the agent model the stored explorations were produced with. The fit verb
	// reads them with the same key the product writes, so the two cannot disagree.
	fitExploreModel = "glm5.3-flash"
)

// fitJudge answers the only question P3 asks. It is the whole transport: the run keeps its own
// tally, so the pass itself stays the production one.
type fitJudge struct {
	choice     fit.Fit
	confidence float64
	fail       bool
	calls      int
	// states keeps what the pass actually sent, which is how the label test proves what was
	// never read (I6).
	states []string
}

func newFitJudge() *fitJudge {
	return &fitJudge{choice: fit.FitReady, confidence: 0.9}
}

func (j *fitJudge) Ask(_ context.Context, state string, questions map[string]nan.Question) (map[string]nan.Answer, error) {
	j.calls++
	j.states = append(j.states, state)
	if j.fail {
		return nil, errors.New("the decision endpoint is down")
	}
	if _, ok := questions[fit.QFit]; !ok {
		return nil, errors.New("the third pass asked something that is not the fit question")
	}
	return map[string]nan.Answer{
		fit.QFit: {Type: nan.Choice, Choice: string(j.choice), Confidence: j.confidence},
	}, nil
}

// seedTriage runs the production first pass over one snapshot, so the fit verb reads the P1
// verdicts a real run would have left behind.
func seedTriage(t *testing.T, st *store.Store, snap *ghdata.Snapshot) {
	t.Helper()
	if _, out, err := runFixture(t, st, snap, newFakeJudge(), &fakeBrief{body: "# cobra"}, 0); err != nil {
		t.Fatalf("seeding the first pass: %v\n%s", err, out)
	}
}

// fitIdentity is the exploration the fixture corpus was produced with: the explore contract and
// the agent model, which is both how the evidence is found and half of the question P3 answers.
func fitIdentity() fit.Exploration {
	return fit.Exploration{PromptVersion: explore.Version(), Model: fitExploreModel}
}

// seedExplorations stores the evidence the third pass reads, with the key the product writes.
func seedExplorations(t *testing.T, st *store.Store, identity fit.Exploration, numbers ...int) {
	t.Helper()

	journal, err := st.Decisions(testKey)
	if err != nil {
		t.Fatalf("journal: %v", err)
	}

	for _, number := range numbers {
		verdict, err := json.Marshal(explore.Record{
			Repository: testKey,
			CommitSHA:  testCommit,
			Issue:      number,
			Model:      identity.Model,
			Evidence: &explore.Evidence{
				Intent: "skip the hooks",
				Files: []explore.File{{
					Path: "command.go", Symbol: "Execute", Line: 42,
					Reason: "the hook chain", Relation: explore.RelationObserved,
				}},
				Requirements: explore.Requirements{
					Knowledge: []string{"read a flag chain in Go"},
					Project:   []string{"know how cobra builds commands"},
					Process:   []string{"sign the CLA"},
				},
				Unknowns: []string{"the integration test was not found"},
			},
			Metrics: explore.Metrics{Complete: true, TotalMs: 2000},
		})
		if err != nil {
			t.Fatalf("marshal record of #%d: %v", number, err)
		}
		key := store.Key{
			Issue:         number,
			Pass:          store.PassExplore,
			Model:         identity.Model,
			PromptVersion: identity.PromptVersion,
			CommitSHA:     testCommit,
		}
		if err := journal.Put(store.Decision{Key: key, LatencyMs: 2000, Verdict: verdict}); err != nil {
			t.Fatalf("store the exploration of #%d: %v", number, err)
		}
	}
}

// runFitFixture runs the verb over one fixture and returns what it measured and printed. The
// journal is re-read from disk on every call, so a second call sees what a second process would.
func runFitFixture(t *testing.T, st *store.Store, snap *ghdata.Snapshot, judge fit.Judge, document string, exploration fit.Exploration, limit int) (fitSummary, string, error) {
	t.Helper()
	return runFitFixtureAt(t, st, snap, judge, document, exploration, limit, stepClock())
}

// runFitFixtureAt is the same run with the clock injected, so a test can prove that a reused
// verdict reports the time it was produced and not the time it was read (I14).
func runFitFixtureAt(t *testing.T, st *store.Store, snap *ghdata.Snapshot, judge fit.Judge, document string, exploration fit.Exploration, limit int, clock func() time.Time) (fitSummary, string, error) {
	t.Helper()

	journal, err := st.Decisions(testKey)
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	var out bytes.Buffer
	run := fitRun{
		repoKey:     testKey,
		snapshot:    snap,
		journal:     journal,
		judge:       judge,
		profile:     document,
		portrait:    "# cobra",
		model:       config.DecisionModel,
		commitSHA:   testCommit,
		exploration: exploration,
		limit:       limit,
		out:         &out,
		now:         clock,
	}
	sum, runErr := run.run(context.Background())
	return sum, out.String(), runErr
}

// S7: without an approved profile there is no comparison, so the verb refuses before spending a
// request and names where a profile is produced.
func TestFitWithoutAProfileRefusesAndSpendsNothing(t *testing.T) {
	st := newStore(t)
	snap := corpusOf(1)
	seedTriage(t, st, snap)
	seedExplorations(t, st, fitIdentity(), 1)
	judge := newFitJudge()

	sum, out, err := runFitFixture(t, st, snap, judge, "", fitIdentity(), 0)
	if err == nil {
		t.Fatalf("fit ran without an approved profile\n%s", out)
	}
	if !strings.Contains(err.Error(), "approved profile") {
		t.Errorf("the refusal does not name what is missing: %v", err)
	}
	if !strings.Contains(err.Error(), "/profile") {
		t.Errorf("the refusal does not say how to produce a profile: %v", err)
	}
	if judge.calls != 0 {
		t.Errorf("the pass asked %d times without a profile, want 0", judge.calls)
	}
	if sum.Requests != 0 {
		t.Errorf("requests = %d, want 0", sum.Requests)
	}
	if out != "" {
		t.Errorf("a refused run printed a summary:\n%s", out)
	}
}

// The acceptance criterion of the pass: the same corpus twice, and the second run asks for
// nothing and says so.
func TestFitSecondRunCostsNothing(t *testing.T) {
	st := newStore(t)
	snap := corpusOf(1, 2, 3)
	seedTriage(t, st, snap)
	seedExplorations(t, st, fitIdentity(), 1, 2, 3)

	first, out, err := runFitFixture(t, st, snap, newFitJudge(), fitProfile, fitIdentity(), 0)
	if err != nil {
		t.Fatalf("first run: %v\n%s", err, out)
	}
	if first.Requests != 3 || first.Cached != 0 {
		t.Fatalf("first run: requests=%d cached=%d, want 3 and 0\n%s", first.Requests, first.Cached, out)
	}
	if first.Levels[fit.LevelRecommended] != 3 {
		t.Fatalf("levels = %v, want three %q\n%s", first.Levels, fit.LevelRecommended, out)
	}
	if !strings.Contains(out, "requests    3\n") {
		t.Fatalf("first run did not report its requests:\n%s", out)
	}

	second, out, err := runFitFixture(t, st, snap, newFitJudge(), fitProfile, fitIdentity(), 0)
	if err != nil {
		t.Fatalf("second run: %v\n%s", err, out)
	}
	if second.Requests != 0 {
		t.Fatalf("second run made %d requests, want 0\n%s", second.Requests, out)
	}
	if second.Cached != 3 {
		t.Fatalf("second run reports %d cached, want 3\n%s", second.Cached, out)
	}
	if !strings.Contains(out, "requests    0 (3 answered from the journal)") {
		t.Fatalf("second run did not say it cost nothing:\n%s", out)
	}
}

// S6/I14: another CV is another question. A different profile must miss, and the original one
// must still hit, so the miss is the profile and not a broken key.
func TestFitMissesWhenTheProfileChanged(t *testing.T) {
	st := newStore(t)
	snap := corpusOf(1)
	seedTriage(t, st, snap)
	seedExplorations(t, st, fitIdentity(), 1)

	first, out, err := runFitFixture(t, st, snap, newFitJudge(), fitProfile, fitIdentity(), 0)
	if err != nil {
		t.Fatalf("first run: %v\n%s", err, out)
	}
	if first.Requests != 1 {
		t.Fatalf("first run: requests=%d, want 1\n%s", first.Requests, out)
	}

	changed, out, err := runFitFixture(t, st, snap, newFitJudge(), otherFitProfile, fitIdentity(), 0)
	if err != nil {
		t.Fatalf("run with another profile: %v\n%s", err, out)
	}
	if changed.Requests != 1 || changed.Cached != 0 {
		t.Fatalf("another profile reused a verdict: requests=%d cached=%d\n%s",
			changed.Requests, changed.Cached, out)
	}

	again, out, err := runFitFixture(t, st, snap, newFitJudge(), fitProfile, fitIdentity(), 0)
	if err != nil {
		t.Fatalf("third run: %v\n%s", err, out)
	}
	if again.Requests != 0 || again.Cached != 1 {
		t.Fatalf("the original profile missed: requests=%d cached=%d\n%s",
			again.Requests, again.Cached, out)
	}
}

// I14: the evidence the state carries is the exploration's, so another exploration of the same
// issue is another question. A re-explored issue must miss the journal and ask again, and the
// exploration it was read against must still hit.
func TestFitAnotherExplorationIsAnotherQuestion(t *testing.T) {
	st := newStore(t)
	snap := corpusOf(1)
	seedTriage(t, st, snap)
	seedExplorations(t, st, fitIdentity(), 1)

	first, out, err := runFitFixture(t, st, snap, newFitJudge(), fitProfile, fitIdentity(), 0)
	if err != nil {
		t.Fatalf("first run: %v\n%s", err, out)
	}
	if first.Requests != 1 {
		t.Fatalf("first run: requests=%d, want 1\n%s", first.Requests, out)
	}

	// The issue is explored again under another prompt version: the corpus now holds a record
	// under that identity, and the verdict read against the old one answers another question.
	reExplored := fit.Exploration{PromptVersion: "9f8e7d6c5b4a", Model: fitExploreModel}
	seedExplorations(t, st, reExplored, 1)

	second, out, err := runFitFixture(t, st, snap, newFitJudge(), fitProfile, reExplored, 0)
	if err != nil {
		t.Fatalf("run after the re-exploration: %v\n%s", err, out)
	}
	if second.Requests != 1 || second.Cached != 0 {
		t.Fatalf("the verdict of another exploration was reused: requests=%d cached=%d\n%s",
			second.Requests, second.Cached, out)
	}

	// And the first identity still hits, so the miss is the exploration and not a broken key.
	again, out, err := runFitFixture(t, st, snap, newFitJudge(), fitProfile, fitIdentity(), 0)
	if err != nil {
		t.Fatalf("third run: %v\n%s", err, out)
	}
	if again.Requests != 0 || again.Cached != 1 {
		t.Fatalf("the original exploration missed: requests=%d cached=%d\n%s",
			again.Requests, again.Cached, out)
	}
}

// The other half of I14: the same identity twice costs nothing and reports the time the verdict
// was produced, never the time it was read.
func TestFitReusesTheVerdictAndKeepsItsOriginalTime(t *testing.T) {
	st := newStore(t)
	snap := corpusOf(1)
	seedTriage(t, st, snap)
	seedExplorations(t, st, fitIdentity(), 1)

	if _, out, err := runFitFixture(t, st, snap, newFitJudge(), fitProfile, fitIdentity(), 0); err != nil {
		t.Fatalf("first run: %v\n%s", err, out)
	}
	journal, err := st.Decisions(testKey)
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	_, stored, ok := fit.Recall(journal, 1, config.DecisionModel, testCommit, fitProfile, fitIdentity())
	if !ok {
		t.Fatal("the verdict of this exploration was not stored")
	}
	if stored.At.IsZero() {
		t.Fatal("the stored verdict carries no time")
	}
	// The stored row declares the whole question: the contract's fingerprint, the profile's and the
	// exploration's. Without the third one the row cannot say which evidence it was read against.
	if got := strings.Count(stored.PromptVersion, "+"); got != 2 {
		t.Errorf("prompt version %q carries %d fingerprints, want the profile's and the exploration's (I14)",
			stored.PromptVersion, got)
	}

	later := func() time.Time { return time.Date(2031, 6, 1, 12, 0, 0, 0, time.UTC) }
	reused, out, err := runFitFixtureAt(t, st, snap, newFitJudge(), fitProfile, fitIdentity(), 0, later)
	if err != nil {
		t.Fatalf("second run: %v\n%s", err, out)
	}
	if reused.Requests != 0 || reused.Cached != 1 {
		t.Fatalf("the same exploration asked again: requests=%d cached=%d\n%s",
			reused.Requests, reused.Cached, out)
	}
	if !strings.Contains(out, "requests    0 (1 answered from the journal)") {
		t.Fatalf("the second run did not say the journal answered it:\n%s", out)
	}

	reloaded, err := st.Decisions(testKey)
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	_, after, ok := fit.Recall(reloaded, 1, config.DecisionModel, testCommit, fitProfile, fitIdentity())
	if !ok {
		t.Fatal("the reused verdict disappeared")
	}
	if !after.At.Equal(stored.At) {
		t.Errorf("the reused verdict reports %v, want the original %v (I14)", after.At, stored.At)
	}
}

// The run cannot name the exploration it read, so it refuses instead of inventing one: a key with
// an empty fingerprint would reuse one verdict across every tree and every agent (I14).
func TestFitWithoutAnExplorationIdentityRefuses(t *testing.T) {
	st := newStore(t)
	snap := corpusOf(1)
	seedTriage(t, st, snap)
	seedExplorations(t, st, fitIdentity(), 1)
	judge := newFitJudge()

	sum, out, err := runFitFixture(t, st, snap, judge, fitProfile, fit.Exploration{}, 0)
	if err == nil {
		t.Fatalf("fit ran without an exploration identity\n%s", out)
	}
	if !strings.Contains(err.Error(), "explore model") {
		t.Errorf("the refusal does not name what is missing: %v", err)
	}
	if judge.calls != 0 {
		t.Errorf("the pass asked %d times without an exploration identity, want 0", judge.calls)
	}
	if sum.Requests != 0 {
		t.Errorf("requests = %d, want 0", sum.Requests)
	}
	if out != "" {
		t.Errorf("a refused run printed a summary:\n%s", out)
	}
}

// The selection is conservative and says so: only the issues the triage recommended, and only
// those with a stored exploration, reach the third pass. Everything else is counted and named.
func TestFitSkipsWhatTheThirdPassHasNothingToAskAbout(t *testing.T) {
	st := newStore(t)

	triaged := corpusOf(1, 2, 3, 4)
	triaged.Issues[1].State = "CLOSED"
	triaged.Issues[1].StateReason = "COMPLETED"
	triaged.Issues[2].Body = ""
	seedTriage(t, st, triaged)
	// #4 is a candidate the exploration never reached: there is no evidence to weigh.
	seedExplorations(t, st, fitIdentity(), 1, 3)

	// The fit run reads a snapshot that also holds #5, which the first pass never saw.
	snap := corpusOf(1, 2, 3, 4, 5)
	snap.Issues[1].State = "CLOSED"
	snap.Issues[1].StateReason = "COMPLETED"
	snap.Issues[2].Body = ""

	judge := newFitJudge()
	sum, out, err := runFitFixture(t, st, snap, judge, fitProfile, fitIdentity(), 0)
	if err != nil {
		t.Fatalf("skipping is not failing: %v\n%s", err, out)
	}
	if judge.calls != 1 || sum.Fitted != 1 {
		t.Fatalf("the judge was asked %d times for %d fitted, want 1 and 1\n%s",
			judge.calls, sum.Fitted, out)
	}
	if sum.Skipped != 4 {
		t.Fatalf("skipped=%d, want 4\n%s", sum.Skipped, out)
	}
	for reason, want := range map[string]int{
		"MARCADA":               2,
		"no stored triage":      1,
		"no stored exploration": 1,
	} {
		if got := sum.Skips[reason]; got != want {
			t.Errorf("skipped %q = %d, want %d\n%s", reason, got, want, out)
		}
	}
	if !strings.Contains(out, "skipped     4 issues:") {
		t.Fatalf("the summary does not say how many were skipped:\n%s", out)
	}
	if !strings.Contains(out, "MARCADA 2") || !strings.Contains(out, "no stored exploration 1") {
		t.Fatalf("the summary does not say why they were skipped:\n%s", out)
	}
	if !strings.Contains(out, "only CANDIDATA issues") {
		t.Fatalf("the summary does not say what the selection was:\n%s", out)
	}
}

// I2: a failed request is not a verdict. The run keeps going, the count makes the failure
// impossible to miss, and the exit code is not silent.
func TestFitIsUndecidedWhenTheJudgeFails(t *testing.T) {
	st := newStore(t)
	snap := corpusOf(1, 2)
	seedTriage(t, st, snap)
	seedExplorations(t, st, fitIdentity(), 1, 2)

	judge := newFitJudge()
	judge.fail = true

	sum, out, err := runFitFixture(t, st, snap, judge, fitProfile, fitIdentity(), 0)
	if err == nil {
		t.Fatalf("a failed decision must be reported, not swallowed\n%s", out)
	}
	if judge.calls != 2 {
		t.Errorf("the run stopped at the first failure: %d of 2\n%s", judge.calls, out)
	}
	if sum.Fitted != 2 || sum.Failed != 2 {
		t.Errorf("fitted=%d failed=%d, want 2 and 2\n%s", sum.Fitted, sum.Failed, out)
	}
	if sum.Levels[fit.LevelUndecided] != 2 {
		t.Errorf("levels = %v, want two %q\n%s", sum.Levels, fit.LevelUndecided, out)
	}
	if !strings.Contains(out, "undecided 2") {
		t.Errorf("the breakdown hides the failure:\n%s", out)
	}
	if !strings.Contains(out, "2 failed") {
		t.Errorf("the result line hides the failure:\n%s", out)
	}
	if !strings.Contains(err.Error(), "undecided") {
		t.Errorf("the exit code does not say what failed: %v", err)
	}
}

// I6: the maintainer's label is never read. It does not travel in the state and it does not move
// the level.
func TestFitNeverReadsTheLabel(t *testing.T) {
	st := newStore(t)
	labeled := corpusOf(1)
	labeled.Issues[0].Labels = []string{"good first issue", "✭ help wanted ✭"}
	seedTriage(t, st, labeled)
	seedExplorations(t, st, fitIdentity(), 1)

	judge := newFitJudge()
	sum, out, err := runFitFixture(t, st, labeled, judge, fitProfile, fitIdentity(), 0)
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if len(judge.states) != 1 {
		t.Fatalf("the pass sent %d states, want 1", len(judge.states))
	}
	for _, label := range []string{"good first issue", "help wanted"} {
		if strings.Contains(judge.states[0], label) {
			t.Errorf("the label %q reached the state (I6)", label)
		}
	}
	if sum.Levels[fit.LevelRecommended] != 1 {
		t.Errorf("levels = %v, want one %q\n%s", sum.Levels, fit.LevelRecommended, out)
	}

	// The same profile against the same issue with no label: the same level.
	plain := newStore(t)
	bare := corpusOf(1)
	seedTriage(t, plain, bare)
	seedExplorations(t, plain, fitIdentity(), 1)
	unlabeled, out, err := runFitFixture(t, plain, bare, newFitJudge(), fitProfile, fitIdentity(), 0)
	if err != nil {
		t.Fatalf("run without labels: %v\n%s", err, out)
	}
	if unlabeled.Levels[fit.LevelRecommended] != sum.Levels[fit.LevelRecommended] {
		t.Errorf("the label changed the level: %v then %v", sum.Levels, unlabeled.Levels)
	}
}

// -limit bounds the work without filtering anything: the corpus still reports how many issues the
// snapshot holds.
func TestFitLimitCutsTheCorpus(t *testing.T) {
	st := newStore(t)
	snap := corpusOf(1, 2, 3, 4)
	seedTriage(t, st, snap)
	seedExplorations(t, st, fitIdentity(), 1, 2, 3, 4)

	judge := newFitJudge()
	sum, out, err := runFitFixture(t, st, snap, judge, fitProfile, fitIdentity(), 2)
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if sum.Scanned != 2 || sum.Corpus != 4 {
		t.Fatalf("scanned=%d corpus=%d, want 2 and 4\n%s", sum.Scanned, sum.Corpus, out)
	}
	if judge.calls != 2 {
		t.Fatalf("the judge was asked %d times, want 2\n%s", judge.calls, out)
	}
	if !strings.Contains(out, "corpus      2 of 4 issues (limit 2)") {
		t.Fatalf("the limit is not reported as bounding the corpus:\n%s", out)
	}
}
