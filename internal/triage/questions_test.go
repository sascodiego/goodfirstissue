package triage

import (
	"sort"
	"strings"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

// The question set lives in questions.go and the wording lives in prompt.md. These two are
// the same contract seen twice, so these tests are the wedge that keeps them from drifting:
// there is no third copy anywhere, and the benchmark reads this one.

var frozenQuestionIDs = []string{
	QKind, QSelfContained, QHasEvidence, QUnambiguous,
	QDoneDefined, QReportsDefect, QEvidenceSufficient,
}

func sortedKeys(m map[string]nan.Question) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// P1's set is closed. A question that is asked but not frozen is an experiment leaking into
// production; one that is frozen but not asked is a sound the UI would have to invent.
func TestP1AsksExactlyTheFrozenQuestions(t *testing.T) {
	got := sortedKeys(Questions())
	want := append([]string{}, frozenQuestionIDs...)
	sort.Strings(want)

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("P1 asks %v, want %v", got, want)
	}
}

// Every question must appear in the contract with the exact wording that was measured.
// A "roughly equivalent" prompt is a different prompt, and the numbers in the issue would
// no longer describe what ships.
//
// The comparison collapses whitespace because prompt.md is markdown prose: its line breaks
// are layout, not wording. A changed word still fails; a reflowed paragraph does not, and a
// guard that fails on a reflow is a guard everyone learns to skip.
func TestEveryFrozenQuestionAppearsVerbatimInTheContract(t *testing.T) {
	contract := normalized(Prompt)
	for id, q := range Questions() {
		if !strings.Contains(contract, normalized(q.Instructions)) {
			t.Errorf("prompt.md does not carry question %q with the measured wording:\n%s", id, q.Instructions)
		}
		for label, description := range choiceCriteria(q) {
			if !strings.Contains(contract, normalized(description)) {
				t.Errorf("prompt.md does not carry criterion %q of %q verbatim:\n%s", label, id, description)
			}
		}
	}
}

// normalized is the contract's text with its layout removed: words and single spaces.
func normalized(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// choiceCriteria reads the criteria of a choice question without asserting its type: the
// shape test below is the one that owns that assertion.
func choiceCriteria(q nan.Question) map[string]string {
	criteria, ok := q.Criteria.(map[string]string)
	if !ok {
		return nil
	}
	return criteria
}

// The answer shape decides what the service sends back, so a question whose type and
// criteria disagree would ask for an answer nothing can read.
func TestQuestionShapesAreWellFormed(t *testing.T) {
	for id, q := range Questions() {
		criteria := choiceCriteria(q)
		switch {
		case id == QKind && q.Type != nan.Choice:
			t.Errorf("question %q is %q, want %q", id, q.Type, nan.Choice)
		case id != QKind && q.Type != nan.Noul:
			t.Errorf("question %q is %q, want %q", id, q.Type, nan.Noul)
		}
		if q.Type == nan.Choice && len(criteria) == 0 {
			t.Errorf("question %q is a choice without criteria: the judge would be guessing", id)
		}
		if q.Type != nan.Choice && criteria != nil {
			t.Errorf("question %q carries criteria but is not a choice", id)
		}
		for _, description := range criteria {
			if len(description) < 20 {
				t.Errorf("question %q has a criteria description too thin to discriminate: %q", id, description)
			}
		}
	}
}

// kind's criteria are the category ids the policy switches on: renaming one silently would
// make every bug fall through to the default branch of the chain.
func TestKindOffersTheFiveCategoriesThePolicyReads(t *testing.T) {
	criteria := choiceCriteria(Questions()[QKind])
	want := []string{KindBug, KindFeature, KindQuestion, KindSpam, KindOther}
	got := make([]string, 0, len(criteria))
	for id := range criteria {
		got = append(got, id)
	}
	sort.Strings(got)
	sort.Strings(want)

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("kind offers %v, want %v", got, want)
	}
}

// The deferred node is specified (§7.5) and measured, but P1 does not send it: its questions
// are kept beside the frozen set, never inside it.
func TestTheDeferredNodeIsNotPartOfP1(t *testing.T) {
	p1 := Questions()
	for id := range AlignmentQuestions() {
		if _, asked := p1[id]; asked {
			t.Errorf("P1 asks %q, which belongs to the deferred alignment node (§7.5)", id)
		}
	}
	if _, asked := p1["intent"]; asked {
		t.Error("P1 asks intent, which left the first pass (LIMITES.md §2)")
	}
}

// A caller that mutates the returned map must not be able to rewrite the frozen contract for
// everyone else in the process.
func TestTheFrozenSetCannotBeMutatedByACaller(t *testing.T) {
	first := Questions()
	sentinel := nan.NoulQuestion("a question nobody froze")
	first[QKind] = sentinel
	for id := range first {
		first[id] = sentinel
	}

	if got := Questions()[QKind]; got.Instructions == sentinel.Instructions {
		t.Fatal("a caller rewrote the frozen question set")
	}
	if len(Questions()) != len(frozenQuestionIDs) {
		t.Fatalf("a caller changed the size of the frozen set: %d", len(Questions()))
	}
}

// The version is part of the cache key, so it has to be stable, short, and a hash: a verdict
// produced by another contract answers another question and must miss (I14).
func TestVersionIsAStableShortFingerprint(t *testing.T) {
	got := Version()
	if got != Version() {
		t.Fatal("Version is not stable across calls")
	}
	if len(got) != 12 {
		t.Fatalf("Version = %q, want 12 hex characters", got)
	}
	if strings.Trim(got, "0123456789abcdef") != "" {
		t.Fatalf("Version = %q, want lowercase hex", got)
	}
}

// The fingerprint must move when the contract moves, which is the whole reason it exists.
func TestVersionFollowsTheContract(t *testing.T) {
	base := Version()

	saved := Prompt
	Prompt = saved + "\n<!-- one word more -->"
	t.Cleanup(func() { Prompt = saved })

	if Version() == base {
		t.Fatal("Version did not change when the contract did: a stale answer would be reused")
	}
}
