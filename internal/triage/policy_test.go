package triage

import (
	"strings"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

// One test per row of the measured chain. The rows are the product, so a change to a threshold
// or to the order of the chain has to break a named test instead of quietly moving a verdict.

// noul and choice build the answers a service would return. Tests never reach the network: the
// policy is a pure function of the dossier and the answers (S8).
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

// aligned is what the node answers for a feature the project is moving toward.
func aligned() map[string]nan.Answer {
	return map[string]nan.Answer{QOnlyForRequester: noul(0.1), QFitsDirection: noul(0.9)}
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

func aFeature() map[string]nan.Answer {
	answers := wellFormed()
	answers[QKind] = choice(KindFeature, 0.9)
	return answers
}

func TestTheWellFormedBaselineIsACandidate(t *testing.T) {
	got := Decide(aBug(), wellFormed(), nil)
	if got.Outcome != Candidate {
		t.Fatalf("outcome = %q (%s), want %q", got.Outcome, got.Reason, Candidate)
	}
	if got.Kind != KindBug {
		t.Errorf("kind = %q, want %q", got.Kind, KindBug)
	}
}

// A deterministic fact answers the issue before any judgement, and it keeps its own reason: the
// UI has to be able to say which fact it was.
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
			got := Decide(tt.issue, wellFormed(), nil)
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
// classify the text.
func TestAPreGateOutranksADoubt(t *testing.T) {
	issue := aBug()
	issue.LinkedPRs = []LinkedPR{{Number: 9, State: "OPEN"}}
	answers := wellFormed()
	answers[QEvidenceSufficient] = noul(0.1)

	got := Decide(issue, answers, nil)
	if got.Outcome != Marked || got.Reason != ReasonAlreadyWorking {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Marked, ReasonAlreadyWorking)
	}
}

// I10: an issue the pass cannot judge is a doubt in voice, never a discard.
func TestInsufficientEvidenceIsDoubtAndNotADiscard(t *testing.T) {
	answers := wellFormed()
	answers[QEvidenceSufficient] = noul(0.4)

	got := Decide(aBug(), answers, nil)
	if got.Outcome != Doubt {
		t.Fatalf("outcome = %q, want %q", got.Outcome, Doubt)
	}
	if len(got.Missing) == 0 {
		t.Fatal("a doubt must say what would settle it")
	}
}

func TestSpamIsMarked(t *testing.T) {
	answers := wellFormed()
	answers[QKind] = choice(KindSpam, 0.9)

	got := Decide(aBug(), answers, nil)
	if got.Outcome != Marked || got.Reason != ReasonNotCodeWork {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Marked, ReasonNotCodeWork)
	}
}

// Documentation, tests and refactors are exactly the work a first contribution looks like.
func TestDocumentationTestsAndProcessAreCandidates(t *testing.T) {
	answers := wellFormed()
	answers[QKind] = choice(KindOther, 0.9)

	if got := Decide(aBug(), answers, nil); got.Outcome != Candidate {
		t.Fatalf("outcome = %q (%s), want %q", got.Outcome, got.Reason, Candidate)
	}
}

// LIMITES.md §3: a question is derived to a round that is specified and not installed. It is
// neither a candidate nor a discard, so it gets its own outcome.
func TestAQuestionIsDerivedNotDiscarded(t *testing.T) {
	answers := wellFormed()
	answers[QKind] = choice(KindQuestion, 0.9)

	got := Decide(aBug(), answers, nil)
	if got.Outcome != Derived || got.Reason != ReasonSecondRoundPending {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Derived, ReasonSecondRoundPending)
	}
}

// §7.5: the alignment node is the only branch a feature takes. A project moving toward the
// request makes it a candidate.
func TestAFeatureTheProjectIsMovingTowardIsACandidate(t *testing.T) {
	got := Decide(aBug(), aFeature(), aligned())
	if got.Outcome != Candidate || got.Kind != KindFeature {
		t.Fatalf("outcome = %q (%s) kind %q, want %q", got.Outcome, got.Reason, got.Kind, Candidate)
	}
}

// A request that serves only the person asking is not a contribution, however clear it is.
func TestAFeatureThatServesOnlyTheRequesterIsMarked(t *testing.T) {
	alignment := aligned()
	alignment[QOnlyForRequester] = noul(0.9)

	got := Decide(aBug(), aFeature(), alignment)
	if got.Outcome != Marked || got.Reason != ReasonServesOnlyRequester {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Marked, ReasonServesOnlyRequester)
	}
}

// The project fixing no position is not a refusal: it is a condition, and the condition is
// asking the maintainer before anyone writes code.
func TestAFeatureTheProjectHasNoPositionOnIsConditional(t *testing.T) {
	alignment := map[string]nan.Answer{QOnlyForRequester: noul(0.1), QFitsDirection: noul(0.3)}

	got := Decide(aBug(), aFeature(), alignment)
	if got.Outcome != Conditional || got.Reason != ReasonAskMaintainer {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Conditional, ReasonAskMaintainer)
	}
}

// A feature the node could not be asked about is not a recommendation either: a deferred node
// must not quietly become one, so the verdict says the node was unavailable.
func TestAFeatureWithoutTheNodeSaysSo(t *testing.T) {
	got := Decide(aBug(), aFeature(), nil)
	if got.Outcome != Conditional || got.Reason != ReasonAlignmentUnavailable {
		t.Fatalf("outcome = %q (%s), want %q (%s)", got.Outcome, got.Reason, Conditional, ReasonAlignmentUnavailable)
	}
}

// The alignment cut is 0.6 and it is strict: exactly 0.6 passes.
func TestTheAlignmentThresholdIsSixTyStrict(t *testing.T) {
	alignment := map[string]nan.Answer{QOnlyForRequester: noul(0.1), QFitsDirection: noul(alignmentThreshold)}
	if got := Decide(aBug(), aFeature(), alignment); got.Outcome != Candidate {
		t.Fatalf("outcome = %q (%s), want %q at exactly %v", got.Outcome, got.Reason, Candidate, alignmentThreshold)
	}

	alignment = map[string]nan.Answer{QOnlyForRequester: noul(alignmentThreshold), QFitsDirection: noul(0.9)}
	if got := Decide(aBug(), aFeature(), alignment); got.Outcome != Marked {
		t.Fatalf("outcome = %q (%s), want %q at exactly %v", got.Outcome, got.Reason, Marked, alignmentThreshold)
	}
}

// The §7.3 quality answers and the §7.6 guardrail are measured but not applied. This is the test
// that pins the decision: applying them again would move 29 issues out of CANDIDATA in the
// benchmark, which is exactly the divergence this feature removed.
func TestTheUnvalidatedQualityQuestionsDoNotMoveTheOutcome(t *testing.T) {
	answers := wellFormed()
	answers[QSelfContained] = noul(0.1)
	answers[QHasEvidence] = noul(0.1)
	answers[QUnambiguous] = noul(0.1)
	answers[QDoneDefined] = noul(0.1)
	answers[QReportsDefect] = noul(0.1)

	got := Decide(aBug(), answers, nil)
	if got.Outcome != Candidate {
		t.Fatalf("outcome = %q (%s), want %q: the quality questions are measured, not applied", got.Outcome, got.Reason, Candidate)
	}
}

// A response that omits a question the chain needs is not a verdict, and not a doubt about the
// issue either: it is an answer we could not read (I2).
func TestAMissingAnswerIsNeverAVerdict(t *testing.T) {
	for _, id := range []string{QKind, QEvidenceSufficient} {
		t.Run(id, func(t *testing.T) {
			answers := wellFormed()
			delete(answers, id)

			if got := Decide(aBug(), answers, nil); got.Outcome != Undecided {
				t.Fatalf("outcome = %q (%s), want %q", got.Outcome, got.Reason, Undecided)
			}
		})
	}

	for _, id := range []string{QOnlyForRequester, QFitsDirection} {
		t.Run("feature/"+id, func(t *testing.T) {
			alignment := aligned()
			delete(alignment, id)

			if got := Decide(aBug(), aFeature(), alignment); got.Outcome != Undecided {
				t.Fatalf("outcome = %q (%s), want %q", got.Outcome, got.Reason, Undecided)
			}
		})
	}
}

// A label outside the frozen criteria is not a kind the policy can reason about, so it is not
// silently mapped onto one that exists.
func TestAnUnknownKindIsNotAVerdict(t *testing.T) {
	answers := wellFormed()
	answers[QKind] = choice("refactor", 0.9)

	if got := Decide(aBug(), answers, nil); got.Outcome != Undecided {
		t.Fatalf("outcome = %q (%s), want %q", got.Outcome, got.Reason, Undecided)
	}
}

// The threshold is a strict comparison: exactly 0.5 passes. A boundary that flipped would move
// every borderline issue without anyone editing the policy.
func TestTheThresholdIsStrictAtHalf(t *testing.T) {
	answers := wellFormed()
	answers[QEvidenceSufficient] = noul(threshold)

	if got := Decide(aBug(), answers, nil); got.Outcome != Candidate {
		t.Fatalf("outcome = %q (%s), want %q: 0.5 must pass", got.Outcome, got.Reason, Candidate)
	}
}

// No outcome is shown without its probability, and the probability shown is the one that
// decided it.
func TestTheOutcomeCarriesTheProbabilityThatDecidedIt(t *testing.T) {
	answers := wellFormed()
	answers[QEvidenceSufficient] = noul(0.3)
	if got := Decide(aBug(), answers, nil); got.Probability != 0.3 {
		t.Fatalf("probability = %v, want the evidence_sufficient answer 0.3", got.Probability)
	}

	alignment := aligned()
	alignment[QFitsDirection] = noul(0.7)
	if got := Decide(aBug(), aFeature(), alignment); got.Probability != 0.7 {
		t.Fatalf("probability = %v, want the fits_direction answer 0.7", got.Probability)
	}
}

// A deterministic resolution is not a probability: it is a fact, and it says so with 1.
func TestAPreGateReportsCertainty(t *testing.T) {
	issue := aBug()
	issue.GitHubState = "CLOSED"
	issue.GitHubStateReason = "COMPLETED"

	if got := Decide(issue, wellFormed(), nil); got.Probability != 1 {
		t.Fatalf("probability = %v, want 1 for a deterministic fact", got.Probability)
	}
}

// I6 as a structural property: the label list never reaches the substance dossier, so it cannot
// move the kind or the quality. This is not a promise to be trusted but a shape that is checked.
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
		if got := Decide(issue, wellFormed(), nil); got.Outcome != Candidate || got.Kind != KindBug {
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
