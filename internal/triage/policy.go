package triage

import (
	"sort"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

// Outcome is what the first pass decided about one issue. These strings are wire values: the
// screen renders its own wording from them, so a translation never moves a verdict.
type Outcome string

const (
	// Candidate is worth exploring and can be recommended.
	Candidate Outcome = "CANDIDATA"
	// Conditional is an opportunity with an external condition missing; Reason names it and
	// Missing names the answers that would settle it.
	Conditional Outcome = "CONDICIONADA"
	// Marked is not recommended now, with the reason and the evidence in view. It is not
	// terminal: it keeps its reason and a later round may overturn it.
	Marked Outcome = "MARCADA"
	// Doubt is "we cannot say, and here is what we would need" (I10). It is a first-class
	// outcome, never a hidden abstention and never a discard.
	Doubt Outcome = "DUDA"
	// Derived is a question the first pass does not settle: the second round that would decide
	// whether the answer becomes work is specified and not installed (LIMITES.md §3). It is
	// neither a candidate nor a discard.
	Derived Outcome = "DERIVADA"
	// Undecided is a failure of the request or an answer we cannot read, never a verdict about
	// the issue (I2).
	Undecided Outcome = "undecided"
)

// Why an issue resolved the way it did. Wire values, for the same reason as the outcomes.
const (
	ReasonNotCodeWork          = "not_code_work"
	ReasonSecondRoundPending   = "second_round_pending"
	ReasonAskMaintainer        = "ask_maintainer"
	ReasonServesOnlyRequester  = "serves_only_requester"
	ReasonAlignmentUnavailable = "alignment_unavailable"
	ReasonInsufficientEvidence = "insufficient_evidence"
	ReasonIncompleteAnswer     = "incomplete_answer"
	ReasonEndpointFailed       = "endpoint_failed"
)

const (
	// threshold is the cut for a yes/no question. Strict: exactly 0.5 passes, so a borderline
	// issue is not moved by a rounding.
	threshold = 0.5
	// alignmentThreshold is the cut for the alignment node, measured at 0.6 (§7.5).
	alignmentThreshold = 0.6
)

// Result is one issue's first pass. It carries the probability of the answer that decided the
// outcome, because the interface never shows a state without one (§7.7).
type Result struct {
	// Outcome is what the pass decided.
	Outcome Outcome
	// Reason is why, as a wire value. Empty when the outcome needs no explanation beyond the
	// outcome itself.
	Reason string
	// Missing names the questions that would settle a doubt. Empty otherwise.
	Missing []string
	// Kind is the kind the pass read. Empty when the issue was resolved before any judgement.
	Kind string
	// Probability is the probability of the answer behind the outcome. A deterministic fact
	// reports 1: it is not a probability, it is certainty.
	Probability float64
}

// Decide walks the measured chain over one issue, the answers one request returned, and the
// alignment node's answers when it was consulted. It is a pure function: no client, no store,
// no clock, so the benchmark and the product cannot disagree about what an answer means (S2).
//
// The chain is the one the benchmark measured and INFORME-TRIAJE.md reported:
//
//	pre-gates → evidence_sufficient → kind → feature: alignment node (0.6)
//
// The §7.3 quality questions and the §7.6 reclassification are deliberately not gates. They are
// still asked and still measured, and the measurement said they do not hold: the report's own
// conclusion is that one of them answers worse than always answering the same way. A question
// that is asked but not applied is a measurement; a question that is applied unbidden is a
// policy nobody validated.
//
// alignment is nil when the node was not consulted: either the kind is not a feature, or no
// source for the project's direction was available.
func Decide(issue Issue, answers, alignment map[string]nan.Answer) Result {
	if pre := Prescreen(issue); pre.Skip {
		// A deterministic fact resolves the issue without a question, and it keeps its own
		// reason: the interface has to be able to name the fact.
		return Result{Outcome: Marked, Reason: pre.Reason, Probability: 1}
	}

	kind, ok := choiceOf(answers, QKind)
	if !ok {
		return incomplete()
	}
	// I10: a text that cannot be judged is a doubt in voice, never a discard.
	sufficient, ok := noulOf(answers, QEvidenceSufficient)
	if !ok {
		return incomplete()
	}
	if sufficient < threshold {
		return Result{
			Outcome:     Doubt,
			Reason:      ReasonInsufficientEvidence,
			Missing:     []string{QEvidenceSufficient},
			Kind:        kind,
			Probability: sufficient,
		}
	}

	switch kind {
	case KindSpam:
		// No real request and no usable information: there is nothing to explore.
		return Result{Outcome: Marked, Reason: ReasonNotCodeWork, Kind: kind, Probability: confidenceOf(answers, QKind)}
	case KindQuestion:
		// A question becomes work only with the comments, in the round that is specified and
		// not installed. Until then it is derived, never discarded.
		return Result{Outcome: Derived, Reason: ReasonSecondRoundPending, Kind: kind, Probability: confidenceOf(answers, QKind)}
	case KindBug, KindOther:
		// A bug is a deviation from what the project already wants, and documentation, tests
		// and refactors are exactly what a first contribution looks like. Neither needs the
		// alignment node.
		return Result{Outcome: Candidate, Kind: kind, Probability: confidenceOf(answers, QKind)}
	case KindFeature:
		return alignFeature(kind, answers, alignment)
	default:
		// A label outside the frozen criteria is not a kind we can reason about.
		return incomplete()
	}
}

// alignFeature is the only branch that asks whether a request belongs to the project (§7.5).
// A feature can be clear, approved as a topic, and still be worth nothing to anyone but the
// person asking.
func alignFeature(kind string, answers, alignment map[string]nan.Answer) Result {
	if len(alignment) == 0 {
		// The node was not consulted, and a deferred node must not quietly become a
		// recommendation. The condition is asking the maintainer first.
		return Result{
			Outcome:     Conditional,
			Reason:      ReasonAlignmentUnavailable,
			Kind:        kind,
			Probability: confidenceOf(answers, QKind),
		}
	}

	onlyForRequester, ok := noulOf(alignment, QOnlyForRequester)
	if !ok {
		return incomplete()
	}
	if onlyForRequester >= alignmentThreshold {
		return Result{
			Outcome:     Marked,
			Reason:      ReasonServesOnlyRequester,
			Kind:        kind,
			Probability: onlyForRequester,
		}
	}

	fitsDirection, ok := noulOf(alignment, QFitsDirection)
	if !ok {
		return incomplete()
	}
	if fitsDirection >= alignmentThreshold {
		return Result{Outcome: Candidate, Kind: kind, Probability: fitsDirection}
	}
	// The project's position is not settled, either because it has none or because it is
	// moving away: both ask the maintainer before anyone writes code.
	return Result{Outcome: Conditional, Reason: ReasonAskMaintainer, Kind: kind, Probability: fitsDirection}
}

// incomplete is the answer to a response that omits a question the chain needs, or to a label
// outside the frozen criteria: it is not a verdict about the issue and not a doubt about it
// either, it is an answer we cannot read.
func incomplete() Result {
	return Result{Outcome: Undecided, Reason: ReasonIncompleteAnswer}
}

func noulOf(answers map[string]nan.Answer, id string) (float64, bool) {
	a, ok := answers[id]
	if !ok || a.Type != nan.Noul {
		return 0, false
	}
	return a.Noul, true
}

func choiceOf(answers map[string]nan.Answer, id string) (string, bool) {
	a, ok := answers[id]
	if !ok || a.Type != nan.Choice || a.Choice == "" {
		return "", false
	}
	return a.Choice, true
}

// confidenceOf is the probability of the winning label: for a choice answer, Confidence is
// exactly that and adds nothing (§17.6).
func confidenceOf(answers map[string]nan.Answer, id string) float64 {
	return answers[id].Confidence
}

// SortedMissing returns the conditions in a stable order, for the report and the tests.
func SortedMissing(result Result) []string {
	out := append([]string{}, result.Missing...)
	sort.Strings(out)
	return out
}
