package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/config"
	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
	"github.com/sascodiego/nan-hackathon-clef/internal/triage"
)

// These tests never touch the network: the judge and the brief are injected, and the corpus
// is a plain snapshot. The policy, the dossiers, the journal and the pre-gates are the
// production ones, so a disagreement with the product cannot hide behind a test double.
const (
	testRepo   = "spf13/cobra"
	testKey    = "spf13__cobra"
	testCommit = "0123456789abcdef0123456789abcdef01234567"
)

// fakeJudge answers the frozen questions deterministically. Dispatch is on the question set,
// because that is the only signal the production first pass gives: the substance request
// carries QKind and the alignment node carries its own pair.
type fakeJudge struct {
	kind       string
	sufficient float64
	fits       float64
	onlyFor    float64
	fail       bool
	requests   int
}

func newFakeJudge() *fakeJudge {
	return &fakeJudge{kind: triage.KindBug, sufficient: 0.9, fits: 0.9, onlyFor: 0.1}
}

func (f *fakeJudge) Ask(_ context.Context, _ string, questions map[string]nan.Question) (map[string]nan.Answer, error) {
	f.requests++
	if f.fail {
		return nil, errors.New("the decision endpoint is down")
	}
	out := make(map[string]nan.Answer, len(questions))
	if _, substance := questions[triage.QKind]; substance {
		out[triage.QKind] = nan.Answer{Type: nan.Choice, Choice: f.kind, Confidence: 0.86}
		for id, q := range questions {
			if q.Type != nan.Noul {
				continue
			}
			value := 0.9
			if id == triage.QEvidenceSufficient {
				value = f.sufficient
			}
			out[id] = nan.Answer{Type: nan.Noul, Noul: value}
		}
		return out, nil
	}
	out[triage.QOnlyForRequester] = nan.Answer{Type: nan.Noul, Noul: f.onlyFor}
	out[triage.QFitsDirection] = nan.Answer{Type: nan.Noul, Noul: f.fits}
	return out, nil
}

// fakeBrief stands in for the clone. An empty body means the clone could not be read, which
// is the condition the run has to report rather than hide.
type fakeBrief struct {
	body     string
	requests int
}

func (b *fakeBrief) Brief(_ context.Context, _ string) (string, error) {
	b.requests++
	if b.body == "" {
		return "", errors.New("no clone for this repository")
	}
	return b.body, nil
}

// corpusOf builds a snapshot the pre-gates leave alone: open, with a body.
func corpusOf(numbers ...int) *ghdata.Snapshot {
	snap := &ghdata.Snapshot{Repository: ghdata.Repository{NameWithOwner: testRepo, CommitSHA: testCommit}}
	for _, n := range numbers {
		snap.Issues = append(snap.Issues, ghdata.Issue{
			Number: n,
			Title:  "Add a flag to skip hooks",
			Body:   "Running this in a sandbox needs a way to skip the hooks.",
			State:  "OPEN",
			Author: "someone",
		})
	}
	return snap
}

// stepClock makes the elapsed time deterministic instead of wall-clock dependent.
func stepClock() func() time.Time {
	at := time.Date(2026, 11, 15, 3, 0, 0, 0, time.UTC)
	return func() time.Time {
		at = at.Add(10 * time.Millisecond)
		return at
	}
}

// newStore is the journal's home: a temp directory, never the repository's var/.
func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	return st
}

// runFixture runs the verb over one fixture and returns what it measured and printed. The
// journal is re-read from disk on every call, so a second call sees what a second process
// would see.
func runFixture(t *testing.T, st *store.Store, snap *ghdata.Snapshot, judge triage.Judge, brief triage.BriefSource, limit int) (triageSummary, string, error) {
	t.Helper()
	journal, err := st.Decisions(testKey)
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	var out bytes.Buffer
	run := triageRun{
		repoKey:   testKey,
		snapshot:  snap,
		journal:   journal,
		judge:     judge,
		brief:     brief,
		model:     config.DecisionModel,
		commitSHA: testCommit,
		limit:     limit,
		out:       &out,
		now:       stepClock(),
	}
	sum, runErr := run.run(context.Background())
	return sum, out.String(), runErr
}

// TestTriageSecondRunCostsNothing is the acceptance criterion: the same corpus twice, and the
// second run asks for nothing and says so.
func TestTriageSecondRunCostsNothing(t *testing.T) {
	st := newStore(t)
	snap := corpusOf(1, 2, 3)

	first, out, err := runFixture(t, st, snap, newFakeJudge(), &fakeBrief{body: "# cobra"}, 0)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if first.Requests != 3 || first.Cached != 0 {
		t.Fatalf("first run: requests=%d cached=%d, want 3 and 0\n%s", first.Requests, first.Cached, out)
	}
	if !strings.Contains(out, "requests    3\n") {
		t.Fatalf("first run did not report its requests:\n%s", out)
	}

	second, out, err := runFixture(t, st, snap, newFakeJudge(), &fakeBrief{body: "# cobra"}, 0)
	if err != nil {
		t.Fatalf("second run: %v", err)
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
	if len(second.Outcomes) != len(first.Outcomes) {
		t.Fatalf("the breakdown changed between runs: %v then %v", first.Outcomes, second.Outcomes)
	}
}

// TestTriageLimitCutsTheCorpus pins that -limit bounds the work without filtering anything:
// the corpus still reports how many issues the snapshot holds.
func TestTriageLimitCutsTheCorpus(t *testing.T) {
	st := newStore(t)
	judge := newFakeJudge()

	sum, out, err := runFixture(t, st, corpusOf(1, 2, 3, 4, 5), judge, &fakeBrief{body: "# cobra"}, 2)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if sum.Triaged != 2 {
		t.Fatalf("triaged %d issues, want 2\n%s", sum.Triaged, out)
	}
	if sum.Corpus != 5 {
		t.Fatalf("corpus reports %d issues, want 5\n%s", sum.Corpus, out)
	}
	if judge.requests != 2 {
		t.Fatalf("the judge was asked %d times for 2 bug issues, want 2\n%s", judge.requests, out)
	}
	if !strings.Contains(out, "corpus      2 of 5 issues (limit 2)") {
		t.Fatalf("the limit is not reported as bounding the corpus:\n%s", out)
	}
}

// TestTriageReportsTheAlignmentNodeMissing is the degradation criterion: a feature with no
// readable clone is CONDICIONADA with its reason, never a recommendation, and the run says
// which condition was missing.
func TestTriageReportsTheAlignmentNodeMissing(t *testing.T) {
	st := newStore(t)
	judge := newFakeJudge()
	judge.kind = triage.KindFeature

	// The production adapter, pointed at a directory with no clone in it.
	clone := triage.CloneBrief{Root: filepath.Join(t.TempDir(), "clones")}

	sum, out, err := runFixture(t, st, corpusOf(7), judge, clone, 0)
	if err != nil {
		t.Fatalf("a degraded node must not fail the run: %v", err)
	}
	if got := sum.Outcomes[triage.Conditional]; got != 1 {
		t.Fatalf("CONDICIONADA=%d, want 1\n%s", got, out)
	}
	if got := sum.Outcomes[triage.Candidate]; got != 0 {
		t.Fatalf("a missing node recommended %d features\n%s", got, out)
	}
	if got := sum.Reasons[triage.ReasonAlignmentUnavailable]; got != 1 {
		t.Fatalf("the reason alignment_unavailable is not counted: %v\n%s", sum.Reasons, out)
	}
	if sum.Degraded != 1 || sum.Failed != 0 {
		t.Fatalf("degraded=%d failed=%d, want 1 and 0\n%s", sum.Degraded, sum.Failed, out)
	}
	if !strings.Contains(out, "alignment_unavailable 1") {
		t.Fatalf("the summary does not name the missing condition:\n%s", out)
	}
}

// TestTriageIsUndecidedWhenTheJudgeFails pins I2 in the verb: a failed request is not a
// verdict, the remaining issues are still triaged, and the exit is not silent.
func TestTriageIsUndecidedWhenTheJudgeFails(t *testing.T) {
	st := newStore(t)
	judge := newFakeJudge()
	judge.fail = true

	sum, out, err := runFixture(t, st, corpusOf(1), judge, &fakeBrief{body: "# cobra"}, 0)
	if err == nil {
		t.Fatalf("a failed decision must be reported, not swallowed\n%s", out)
	}
	if got := sum.Outcomes[triage.Undecided]; got != 1 {
		t.Fatalf("undecided=%d, want 1\n%s", got, out)
	}
	if sum.Failed != 1 {
		t.Fatalf("failed=%d, want 1\n%s", sum.Failed, out)
	}
	if !strings.Contains(out, "undecided 1") {
		t.Fatalf("the breakdown hides the failure:\n%s", out)
	}
}

// TestTriagePrescreenCostsNoRequest pins that a deterministic fact still resolves without the
// model, which is what makes the first pass cheap on a real corpus.
func TestTriagePrescreenCostsNoRequest(t *testing.T) {
	st := newStore(t)
	judge := newFakeJudge()
	snap := corpusOf(9)
	snap.Issues[0].State = "CLOSED"
	snap.Issues[0].StateReason = "COMPLETED"

	sum, out, err := runFixture(t, st, snap, judge, &fakeBrief{body: "# cobra"}, 0)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if judge.requests != 0 {
		t.Fatalf("the pre-gate asked the judge %d times, want 0\n%s", judge.requests, out)
	}
	if !strings.Contains(out, "corpus      1 issue\n") {
		t.Fatalf("a corpus of one reads as a plural:\n%s", out)
	}
	if got := sum.Outcomes[triage.Marked]; got != 1 {
		t.Fatalf("MARCADA=%d, want 1\n%s", got, out)
	}
	if got := sum.Reasons[triage.ReasonAlreadyDone]; got != 1 {
		t.Fatalf("the native reason is not kept: %v\n%s", sum.Reasons, out)
	}
}

// TestTriageWithoutTheKeyFailsClosed pins the fail-closed criterion: the verb refuses before
// it touches the corpus, and the message names what is missing.
func TestTriageWithoutTheKeyFailsClosed(t *testing.T) {
	t.Setenv("NAN_KEY", "")

	err := runTriage([]string{"spf13/cobra"})
	if err == nil {
		t.Fatal("triage started without NAN_KEY")
	}
	if !strings.Contains(err.Error(), "NAN_KEY") {
		t.Fatalf("the message does not name what is missing: %v", err)
	}
	if _, statErr := os.Stat("var"); statErr == nil {
		t.Fatal("the verb created state before failing closed")
	}
}

// TestToTriageIssueKeepsTheThread pins the adapter: what the first pass can decide on is
// everything the tracker returned, so a dropped field would silently weaken the dossier.
func TestToTriageIssueKeepsTheThread(t *testing.T) {
	in := ghdata.Issue{
		Number:            42,
		Title:             "Fix the help output",
		Body:              "The help line is cut.",
		State:             "CLOSED",
		StateReason:       "NOT_PLANNED",
		Author:            "author",
		AuthorAssociation: "MEMBER",
		Labels:            []string{"good first issue", "help wanted"},
		Comments: []ghdata.Comment{
			{Author: "maintainer", Body: "we would take a patch", AuthorAssociation: "MEMBER"},
		},
		LinkedPRs: []ghdata.PullRequest{{Number: 43, State: "OPEN", Merged: false}},
	}

	got := toTriageIssue(testRepo, in)
	switch {
	case got.Repository != testRepo || got.Number != 42:
		t.Fatalf("repository/number lost: %+v", got)
	case got.Title != in.Title || got.Body != in.Body || got.Author != in.Author:
		t.Fatalf("text lost: %+v", got)
	case got.GitHubState != "CLOSED" || got.GitHubStateReason != "NOT_PLANNED":
		t.Fatalf("native state lost: %+v", got)
	case len(got.Labels) != 2 || got.Labels[0] != "good first issue":
		t.Fatalf("labels lost or reordered: %v", got.Labels)
	case len(got.Comments) != 1 || got.Comments[0].AuthorAssociation != "MEMBER":
		t.Fatalf("comments lost their standing: %v", got.Comments)
	case len(got.LinkedPRs) != 1 || got.LinkedPRs[0].Number != 43 || got.LinkedPRs[0].State != "OPEN":
		t.Fatalf("linked pull requests lost: %v", got.LinkedPRs)
	}
}

// The node reads the portrait when it can be produced, and the free README brief when it cannot:
// a pass over a hundred issues is never aborted over the richer input.
//
// What is asserted here is the decision, not the fallback's text: triage.TestFallbackBrief* owns
// that contract, and CloneBrief has its own test. This one fails if a portrait that cannot run is
// still offered as the primary source.
func TestAlignmentBriefDegradesToTheReadmeBriefWithoutAKey(t *testing.T) {
	// Without the key no portrait can run, and nothing may be written on its behalf.
	t.Setenv("NAN_KEY", "")
	cfg := config.Default()
	cfg.Repos.CloneDir = t.TempDir()
	cfg.Store.Dir = t.TempDir()
	cfg.ProjectBrief.Home = t.TempDir()
	st, err := store.New(cfg.Store.Dir)
	if err != nil {
		t.Fatal(err)
	}

	brief := alignmentBrief(cfg, st, testKey)
	fallback, ok := brief.(triage.FallbackBrief)
	if !ok {
		t.Fatalf("override = %T, want triage.FallbackBrief", brief)
	}
	if fallback.Primary != nil {
		t.Error("a portrait that cannot run must not be the primary source")
	}
	clone, ok := fallback.Fallback.(triage.CloneBrief)
	if !ok {
		t.Fatalf("fallback = %T, want triage.CloneBrief", fallback.Fallback)
	}
	if clone.Root != cfg.Repos.CloneDir {
		t.Errorf("the fallback reads %q, want the clone directory", clone.Root)
	}
	if _, err := os.Stat(filepath.Join(cfg.ProjectBrief.Home, "agent")); err == nil {
		t.Error("no agent home may be written for a portrait that cannot run")
	}
}

// With a key the portrait is the primary source, so the node reads evidence with citations instead
// of a README dump.
func TestAlignmentBriefPrefersThePortraitWhenItCanRun(t *testing.T) {
	t.Setenv("NAN_KEY", "test-key")
	cfg := config.Default()
	cfg.Repos.CloneDir = t.TempDir()
	cfg.Store.Dir = t.TempDir()
	cfg.ProjectBrief.Home = t.TempDir()
	st, err := store.New(cfg.Store.Dir)
	if err != nil {
		t.Fatal(err)
	}

	brief := alignmentBrief(cfg, st, testKey)
	fallback, ok := brief.(triage.FallbackBrief)
	if !ok {
		t.Fatalf("override = %T, want triage.FallbackBrief", brief)
	}
	if fallback.Primary == nil {
		t.Fatal("with a key the portrait is the primary source")
	}
	if _, err := os.Stat(filepath.Join(cfg.ProjectBrief.Home, "agent", "models.json")); err != nil {
		t.Errorf("the portrait's agent home was not configured: %v", err)
	}
}

// A pre-gate verdict is a verdict: the second run over a corpus of closed issues reports that
// the journal answered them, instead of printing zero requests for a corpus it did work on.
// Without this, the breakdown the report reads disagrees with what the verb did (issue #41).
func TestTriageCountsAPreResolvedIssueAsAnsweredFromTheJournal(t *testing.T) {
	st := newStore(t)
	snap := corpusOf(1)
	snap.Issues[0].State = "CLOSED"
	snap.Issues[0].StateReason = "COMPLETED"

	first, out, err := runFixture(t, st, snap, newFakeJudge(), &fakeBrief{body: "# cobra"}, 0)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if first.Requests != 0 {
		t.Fatalf("first run made %d requests for an issue a pre-gate resolves\n%s", first.Requests, out)
	}
	if first.Cached != 0 {
		t.Fatalf("first run reported %d cached, want 0\n%s", first.Cached, out)
	}
	if first.Outcomes[triage.Marked] != 1 {
		t.Fatalf("outcomes = %v, want one %q\n%s", first.Outcomes, triage.Marked, out)
	}
	if !strings.Contains(out, "requests    0\n") {
		t.Fatalf("first run did not report that it spent nothing:\n%s", out)
	}

	second, out, err := runFixture(t, st, snap, newFakeJudge(), &fakeBrief{body: "# cobra"}, 0)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if second.Requests != 0 {
		t.Fatalf("second run made %d requests, want 0\n%s", second.Requests, out)
	}
	if second.Cached != 1 {
		t.Fatalf("second run reports %d cached, want 1\n%s", second.Cached, out)
	}
	if second.Outcomes[triage.Marked] != 1 {
		t.Fatalf("the breakdown changed between runs: %v then %v", first.Outcomes, second.Outcomes)
	}
	if !strings.Contains(out, "requests    0 (1 answered from the journal)") {
		t.Fatalf("second run did not say the journal answered it:\n%s", out)
	}
}
