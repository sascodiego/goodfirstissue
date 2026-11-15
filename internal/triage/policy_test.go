package triage

import (
	"strings"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

// One test per row of the policy. The rows are the product, so a change to a threshold or to
// the order of the chain has to break a named test instead of quietly moving a verdict.

const half = 0.5

// noul and choice build the answers a service would return. Tests never reach the network:
// the policy is a pure function of the dossier and the answers (S8).
func noul(v float64) nan.Answer { return nan.Answer{Type: nan.Noul, Noul: v} }

func choice(label string, v float64) nan.Answer {
	return nan.Answer{Type: nan.Choice, Choice: label, Confidence: v}
}

// wellFormed is an issue that passes every gate, so each test can fail exactly one rule.
func wellFormed() map[string]nan.Answer {
	return map[string]nan.Answer{
		QKind:               choice(KindBug, 0.9),
		QSelfContained:      noul(0.9),
		QHasEvidence:        noul(0.9),
		QUnambiguous:        noul(0.9),
		QDoneDefined:        noul(0.9),
		QReportsDefect:      noul(0.9),
		QEvidenceSufficient: noul(0.9),
	}
}

func aBug() Issue {
	return Issue{
		Repository: "spf13/cobra",
		Number:     1,
		Author:     "someone",
		Title:      "Commands lose their parent when added twice",
		Body:       "Adding the same command twice makes the second one shadow the first.",
	}
}

func TestTheWellFormedBaselineIsACandidate(t *testing.T) {
	got := Decide(aBug(), wellFormed())
	if got.Outcome != Candidate {
		t.Fatalf("outcome = %q (%s), want %q", got.Outcome, got.Reason, Candidate)
	}
	if got.Kind != KindBug {
		t.Errorf("kind = %q, want %q", got.Kind, KindBug)
	}
}

// A deterministic fact answers the issue before any judgement, and it keeps its own reason:
// the UI has to be able to say which fact it was.
func TestEveryPreGateResolvesWithItsOwnReason(t *testing.T) {
	tests := []struct {
		name  string
		issue Issue
		why   string
	}{
		{
			name:  "closed as completed",
			issue: func() Issue { i := aBug(); i.GitHubState = "CLOSED"; i.GitHubStateReason = "COMPLETED"; return i }(),
			why:   ReasonAlreadyDone,
		},
		{
			name:  "closed as not planned",
			issue: func() Issue { i := aBug(); i.GitHubState = "CLOSED"; i.GitHubStateReason = "NOT_PLANNED"; return i }(),
			why:   ReasonDeclined,
		},
		{
			name:  "closed for another reason",
			issue: func() Issue { i := aBug(); i.GitHubState = "CLOSED"; i.GitHubStateReason = "DUPLICATE"; return i }(),
			why:   ReasonClosed,
		},
		{
			name:  "an open pull request already works on it",
			issue: func() Issue { i := aBug(); i.LinkedPRs = []LinkedPR{{Number: 9, State: "OPEN"}}; return i }(),
			why:   ReasonAlreadyWorking,
		},
		{
			name:  "nothing to read",
			issue: func() Issue { i := aBug(); i.Body = "   \n\t "; return i }(),
			why:   ReasonNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(tt.issue, wellFormed())
			if got.Outcome != Marked {
				t.Fatalf("outcome = %q, want %q", got.Outcome, Marked)
			}
			if got.Reason != tt.why {
				t.Fatalf("reason = %q, want %q", got.Reason, tt.why)
			}
		})
	}
}

// A pre-gate outranks a judgement, even a doubtful one: it does not depend on being able to
// classify the text (criteria.md, G1 before G0).
func TestAPreGateOutranksADoubt(t *testing.T) {
	issue := aBug()
	issue.LinkedPRs = []LinkedPR{{Number: 9, State: "OPEN"}}
	answers := wellFormed()
	answers[QEvidenceSufficient] = noul(0.1)

	got := Decide(issue, answers)
	if got.Outcome != Marked || got.Reason != ReasonAlreadyWorking {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Marked, ReasonAlreadyWorking)
	}
}

// I10: an issue the pass cannot judge is a doubt in voice, never a discard.
func TestInsufficientEvidenceIsDoubtAndNotADiscard(t *testing.T) {
	answers := wellFormed()
	answers[QEvidenceSufficient] = noul(0.4)

	got := Decide(aBug(), answers)
	if got.Outcome != Doubt {
		t.Fatalf("outcome = %q, want %q", got.Outcome, Doubt)
	}
	if len(got.Missing) == 0 {
		t.Fatal("a doubt must say what would settle it")
	}
}

func TestALackOfContextIsMarked(t *testing.T) {
	answers := wellFormed()
	answers[QSelfContained] = noul(0.3)

	got := Decide(aBug(), answers)
	if got.Outcome != Marked || got.Reason != ReasonLacksContext {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Marked, ReasonLacksContext)
	}
}

// §7.3: without a reproduction there is no bug to find. The gate is specific to a bug: a
// documentation request needs no reproduction and must not be punished for lacking one.
func TestABugWithoutReproductionIsMarked(t *testing.T) {
	answers := wellFormed()
	answers[QHasEvidence] = noul(0.2)

	if got := Decide(aBug(), answers); got.Outcome != Marked || got.Reason != ReasonNoReproduction {
		t.Fatalf("bug: outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Marked, ReasonNoReproduction)
	}

	answers[QKind] = choice(KindOther, 0.9)
	if got := Decide(aBug(), answers); got.Outcome != Candidate {
		t.Fatalf("documentation: outcome = %q (%s), want %q", got.Outcome, got.Reason, Candidate)
	}
}

func TestSpamIsMarked(t *testing.T) {
	answers := wellFormed()
	answers[QKind] = choice(KindSpam, 0.9)

	got := Decide(aBug(), answers)
	if got.Outcome != Marked || got.Reason != ReasonNotCodeWork {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Marked, ReasonNotCodeWork)
	}
}

// Documentation, tests and refactors are exactly the work a first contribution looks like,
// so the category that describes them is a candidate (criteria.md).
func TestDocumentationTestsAndProcessAreCandidates(t *testing.T) {
	answers := wellFormed()
	answers[QKind] = choice(KindOther, 0.9)

	if got := Decide(aBug(), answers); got.Outcome != Candidate {
		t.Fatalf("outcome = %q (%s), want %q", got.Outcome, got.Reason, Candidate)
	}
}

// §7.3: a finish or a reading still to be agreed is a condition, not a rejection, and the
// condition travels with the verdict.
func TestAnUndefinedFinishOrAnAmbiguousReadingIsConditional(t *testing.T) {
	tests := []struct {
		name    string
		fail    string
		missing string
	}{
		{"finish not defined", QDoneDefined, QDoneDefined},
		{"two readings", QUnambiguous, QUnambiguous},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answers := wellFormed()
			answers[tt.fail] = noul(0.4)

			got := Decide(aBug(), answers)
			if got.Outcome != Conditional {
				t.Fatalf("outcome = %q (%s), want %q", got.Outcome, got.Reason, Conditional)
			}
			if !contains(got.Missing, tt.missing) {
				t.Fatalf("missing = %v, want it to name %q", got.Missing, tt.missing)
			}
		})
	}
}

func TestBothQualityGatesAreNamedAtOnce(t *testing.T) {
	answers := wellFormed()
	answers[QDoneDefined] = noul(0.1)
	answers[QUnambiguous] = noul(0.1)

	got := Decide(aBug(), answers)
	if got.Outcome != Conditional || !contains(got.Missing, QDoneDefined) || !contains(got.Missing, QUnambiguous) {
		t.Fatalf("outcome = %q missing = %v, want both names", got.Outcome, got.Missing)
	}
}

// §7.7: a feature whose project position is unknown asks the maintainer first. The alignment
// node that would decide it is deferred (§7.5), and the deferred node must not silently
// become "candidate".
func TestAFeatureWithoutAProjectPositionIsConditional(t *testing.T) {
	answers := wellFormed()
	answers[QKind] = choice(KindFeature, 0.9)

	got := Decide(aBug(), answers)
	if got.Outcome != Conditional || got.Reason != ReasonAskMaintainer {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Conditional, ReasonAskMaintainer)
	}
}

// §7.6: the cheap guardrail. A "bug" that reports no defect is a feature request in disguise,
// and it is reported as reclassified so the screen can say it.
func TestAFeatureDisguisedAsABugIsReclassified(t *testing.T) {
	answers := wellFormed()
	answers[QReportsDefect] = noul(0.2)

	got := Decide(aBug(), answers)
	if got.Outcome != Conditional || got.Reason != ReasonAskMaintainer {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Conditional, ReasonAskMaintainer)
	}
	if !got.Reclassified {
		t.Fatal("a reclassification must be reported: the never-instructions say so")
	}
	if got.Kind != KindFeature {
		t.Fatalf("kind = %q, want %q", got.Kind, KindFeature)
	}
}

// A missing reproduction is a property of the issue; a doubt about its kind is a property of
// our reading. The first marks, the second doubts, and the order decides which one is shown.
func TestAReproductionGateDoesNotOverrideADoubt(t *testing.T) {
	answers := wellFormed()
	answers[QHasEvidence] = noul(0.1)
	answers[QEvidenceSufficient] = noul(0.2)

	if got := Decide(aBug(), answers); got.Outcome != Doubt {
		t.Fatalf("outcome = %q (%s), want %q", got.Outcome, got.Reason, Doubt)
	}
}

// LIMITES.md §3: a question is derived to a second round that is specified and not
// implemented. It is neither a candidate nor a discard, so it gets its own outcome.
func TestAQuestionIsDerivedNotDiscarded(t *testing.T) {
	answers := wellFormed()
	answers[QKind] = choice(KindQuestion, 0.9)

	got := Decide(aBug(), answers)
	if got.Outcome != Derived {
		t.Fatalf("outcome = %q (%s), want %q", got.Outcome, got.Reason, Derived)
	}
	if got.Reason != ReasonSecondRoundPending {
		t.Fatalf("reason = %q, want %q", got.Reason, ReasonSecondRoundPending)
	}
}

// A response that omits a question the chain needs is not a verdict, and it is not a doubt
// about the issue either: it is an answer we could not read (I2).
func TestAMissingAnswerIsNeverAVerdict(t *testing.T) {
	for _, id := range []string{QKind, QEvidenceSufficient, QSelfContained, QDoneDefined, QUnambiguous} {
		t.Run(id, func(t *testing.T) {
			answers := wellFormed()
			delete(answers, id)

			got := Decide(aBug(), answers)
			if got.Outcome != Undecided {
				t.Fatalf("outcome = %q (%s), want %q", got.Outcome, got.Reason, Undecided)
			}
		})
	}
}

// The threshold is a strict comparison: exactly 0.5 passes. A boundary that flipped would
// move every borderline issue without anyone editing the policy.
func TestTheThresholdIsStrictAtHalf(t *testing.T) {
	answers := wellFormed()
	for id := range answers {
		answers[id] = noul(half)
	}
	answers[QKind] = choice(KindBug, half)

	if got := Decide(aBug(), answers); got.Outcome != Candidate {
		t.Fatalf("outcome = %q (%s), want %q: 0.5 must pass", got.Outcome, got.Reason, Candidate)
	}
}

// §7.7: no outcome is shown without its probability, and the probability shown is the one
// that decided it.
func TestTheOutcomeCarriesTheProbabilityThatDecidedIt(t *testing.T) {
	answers := wellFormed()
	answers[QEvidenceSufficient] = noul(0.3)

	got := Decide(aBug(), answers)
	if got.Probability != 0.3 {
		t.Fatalf("probability = %v, want the evidence_sufficient answer 0.3", got.Probability)
	}
}

// A deterministic resolution is not a probability: it is a fact, and it says so with 1.
func TestAPreGateReportsCertainty(t *testing.T) {
	issue := aBug()
	issue.GitHubState = "CLOSED"
	issue.GitHubStateReason = "COMPLETED"

	if got := Decide(issue, wellFormed()); got.Probability != 1 {
		t.Fatalf("probability = %v, want 1 for a deterministic fact", got.Probability)
	}
}

// I6 as a structural property: the label list never reaches the substance dossier, so it
// cannot move the kind or the quality. This is not a promise to be trusted but a shape that
// can be checked.
func TestTheLabelCannotMoveTheVerdict(t *testing.T) {
	labelSets := [][]string{
		nil,
		{"good first issue"},
		{"bug", "urgent"},
		{"enhancement", "help wanted", "status/accepted"},
		{"wontfix", "duplicate", "invalid"},
	}

	for _, labels := range labelSets {
		issue := aBug()
		issue.Labels = labels
		if dossier := SubstanceDossier(issue); containsLabel(dossier, labels) {
			t.Fatalf("labels %v reached the substance dossier", labels)
		}
		if got := Decide(issue, wellFormed()); got.Outcome != Candidate || got.Kind != KindBug {
			t.Fatalf("labels %v moved the verdict to %q/%q", labels, got.Outcome, got.Kind)
		}
	}
}

func containsLabel(dossier string, labels []string) bool {
	for _, label := range labels {
		if label != "" && strings.Contains(dossier, label) {
			return true
		}
	}
	return false
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
