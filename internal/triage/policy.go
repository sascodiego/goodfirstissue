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
	// Conditional is an opportunity with an external condition missing; Missing names it.
	Conditional Outcome = "CONDICIONADA"
	// Marked is not recommended now, with the reason and the evidence in view. It is not
	// terminal: it keeps its reason and a later round may overturn it.
	Marked Outcome = "MARCADA"
	// Doubt is "we cannot say, and here is what we would need" (I10). It is a first-class
	// outcome, never a hidden abstention and never a discard.
	Doubt Outcome = "DUDA"
	// Derived is a question the first pass does not settle: the second round that would
	// decide whether the answer becomes work is specified and not implemented
	// (LIMITES.md §3). It is neither a candidate nor a discard.
	Derived Outcome = "DERIVADA"
	// Undecided is a failure of the request, not a verdict about the issue (I2).
	Undecided Outcome = "undecided"
)

// Why an issue resolved the way it did. Wire values, for the same reason as the outcomes.
const (
	ReasonLacksContext         = "lacks_context"
	ReasonNoReproduction       = "no_reproduction"
	ReasonNotCodeWork          = "not_code_work"
	ReasonSecondRoundPending   = "second_round_pending"
	ReasonAskMaintainer        = "ask_maintainer"
	ReasonInsufficientEvidence = "insufficient_evidence"
	ReasonIncompleteAnswer     = "incomplete_answer"
	ReasonEndpointFailed       = "endpoint_failed"
)

// threshold is the single cut the whole chain uses. It is a strict comparison: exactly 0.5
// passes, so a borderline issue is not moved by a rounding.
const threshold = 0.5

// Result is one issue's first pass. It carries the probability of the answer that decided the
// outcome, because the interface never shows a state without one (§7.7).
type Result struct {
	// Outcome is what the pass decided.
	Outcome Outcome
	// Reason is why, as a wire value. Empty when the outcome needs no explanation beyond
	// the outcome itself.
	Reason string
	// Missing names what would settle a conditional or a doubt. Empty otherwise.
	Missing []string
	// Kind is the kind the pass read, after the §7.6 reclassification. Empty when the issue
	// was resolved before any judgement.
	Kind string
	// Reclassified reports that a "bug" turned out to be a feature request (§7.6), so the
	// interface can say it rather than silently changing the category.
	Reclassified bool
	// Probability is the probability of the answer behind the outcome. A deterministic fact
	// reports 1: it is not a probability, it is certainty.
	Probability float64
}

// Decide walks the frozen chain of the policy over one issue and the answers one request
// returned. It is a pure function: no client, no store, no clock, so the benchmark and the
// product cannot disagree about what an answer means (S2).
//
// The order is the policy's, and it is not commutative:
//
//	pre-gates → evidence_sufficient → quality (§7.3) → kind → finish and reading → feature
func Decide(issue Issue, answers map[string]nan.Answer) Result {
	if pre := Prescreen(issue); pre.Skip {
		// A deterministic fact resolves the issue without a question, and it keeps its own
		// reason: the interface has to be able to name the fact (G1 before G0).
		return Result{Outcome: Marked, Reason: pre.Reason, Probability: 1}
	}

	kind, ok := choiceOf(answers, QKind)
	if !ok {
		return incomplete()
	}
	result := Result{Kind: kind}

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

	// §7.3, first two rows: the issue has to be approachable, and a bug has to be findable.
	if selfContained, ok := noulOf(answers, QSelfContained); !ok {
		return incomplete()
	} else if selfContained < threshold {
		return Result{Outcome: Marked, Reason: ReasonLacksContext, Kind: kind, Probability: selfContained}
	}

	if kind == KindBug {
		hasEvidence, ok := noulOf(answers, QHasEvidence)
		if !ok {
			return incomplete()
		}
		if hasEvidence < threshold {
			// Without a reproduction there is no bug to find. Being unreproducible is not the
			// same as being unfindable, but the first pass cannot tell them apart.
			return Result{Outcome: Marked, Reason: ReasonNoReproduction, Kind: kind, Probability: hasEvidence}
		}

		// §7.6, the cheap guardrail: a "bug" that reports no defect is a feature request in
		// disguise, and it continues as one.
		reportsDefect, ok := noulOf(answers, QReportsDefect)
		if !ok {
			return incomplete()
		}
		if reportsDefect < threshold {
			kind = KindFeature
			result.Kind = kind
			result.Reclassified = true
		}
	}

	// The kind's own gates. A question is derived, spam is marked, and the rest continue to
	// the finish check. Documentation, tests and refactors are real work: they are exactly
	// what a first contribution looks like.
	switch kind {
	case KindSpam:
		label, ok := choiceOf(answers, QKind)
		if !ok {
			return incomplete()
		}
		return Result{Outcome: Marked, Reason: ReasonNotCodeWork, Kind: label, Probability: confidenceOf(answers, QKind)}
	case KindQuestion:
		return Result{Outcome: Derived, Reason: ReasonSecondRoundPending, Kind: kind, Probability: confidenceOf(answers, QKind)}
	}

	// §7.3, third row: finishing and reading are conditions, not rejections, and the pass
	// names each one that is missing so the screen can say what to agree on.
	missing, readable := qualityGaps(answers)
	if !readable {
		return incomplete()
	}
	if len(missing) > 0 {
		return Result{
			Outcome:      Conditional,
			Missing:      missing,
			Kind:         kind,
			Reclassified: result.Reclassified,
			Probability:  worstNoul(answers, missing),
		}
	}

	if kind == KindFeature {
		// §7.7 with the alignment node deferred (§7.5): the project's position is unknown, so
		// the condition is asking the maintainer first. A deferred node must not quietly turn
		// into a recommendation.
		return Result{
			Outcome:      Conditional,
			Reason:       ReasonAskMaintainer,
			Kind:         kind,
			Reclassified: result.Reclassified,
			Probability:  confidenceOf(answers, QKind),
		}
	}

	return Result{
		Outcome:      Candidate,
		Kind:         kind,
		Reclassified: result.Reclassified,
		Probability:  confidenceOf(answers, QKind),
	}
}

// qualityGaps is §7.3's third row. Both are conditions on the same axis — what would make the
// result acceptable — so both are reported at once instead of one per run.
//
// A question the response omits is not a gap: it is a response we cannot read, and reading it
// as "the condition is missing" would invent a condition the service never stated (I2).
func qualityGaps(answers map[string]nan.Answer) (missing []string, readable bool) {
	for _, id := range []string{QDoneDefined, QUnambiguous} {
		v, ok := noulOf(answers, id)
		if !ok {
			return nil, false
		}
		if v < threshold {
			missing = append(missing, id)
		}
	}
	return missing, true
}

// incomplete is the answer to a response that omits a question the chain needs: it is not a
// verdict about the issue and not a doubt about it either, it is an answer we cannot read.
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

// worstNoul is the weakest answer among the conditions that fired: the outcome rests on all of
// them, so it cannot claim more certainty than the weakest one.
func worstNoul(answers map[string]nan.Answer, ids []string) float64 {
	worst := 1.0
	for _, id := range ids {
		if v, ok := noulOf(answers, id); ok && v < worst {
			worst = v
		}
	}
	return worst
}

// SortedMissing returns the conditions in a stable order, for the report and the tests. The
// policy builds them in a fixed order already; this exists so a caller never depends on it.
func SortedMissing(result Result) []string {
	out := append([]string{}, result.Missing...)
	sort.Strings(out)
	return out
}
