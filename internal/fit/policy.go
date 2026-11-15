// Package fit is the third pass: it decides how far one developer's approved profile goes
// towards the technical requirements of one issue.
//
// The question is asked once per issue, and what the screen shows is a level, never a decimal
// (§9.3). The package mirrors internal/triage piece by piece: a pure policy that turns an answer
// into a level, the frozen questions, the state that reaches the judge, and the pass that owns
// the request and the journal. Everything that costs a request is at the edge; the middle is a
// function of values.
package fit

import (
	"math"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

// Fit is the answer the judge gives to the one question P3 asks. These are wire values: they are
// the criteria ids Clef answers with and the ids the policy switches on, so renaming one is a
// contract change and a translation never moves a level.
type Fit string

const (
	// FitReady means the profile covers every technical requirement, with no gap to close.
	FitReady Fit = "ready"
	// FitReadyWithSupport means a technical gap exists and closes while doing the work, with a
	// maintainer answering questions.
	FitReadyWithSupport Fit = "ready_with_support"
	// FitNeedsPreparation means a technical gap has to be closed before starting.
	FitNeedsPreparation Fit = "needs_preparation"
	// FitNeedsExperience means the gap cannot be closed within the horizon of this contribution.
	FitNeedsExperience Fit = "needs_experience"
	// FitInsufficientEvidence means the profile does not describe what this issue requires, so
	// there is nothing to compare. It is a fact about the profile, not about the opportunity.
	FitInsufficientEvidence Fit = "insufficient_evidence"
)

// Level is what the screen shows. It is a closed vocabulary of its own because two of the five
// answers are not shown as themselves: an answer below the confidence floor is not a verdict, and
// a failure of the request says so instead of guessing (I2).
type Level string

const (
	// LevelRecommended is a fit the answer supported with confidence.
	LevelRecommended Level = "recommended"
	// LevelRecommendedWithSupport is a fit whose gap closes during the work.
	LevelRecommendedWithSupport Level = "recommended_with_support"
	// LevelNeedsPreparation and LevelNeedsExperience are shown as the answer itself, with its
	// probability: they are not softened into a recommendation and not hidden either.
	LevelNeedsPreparation Level = "needs_preparation"
	LevelNeedsExperience  Level = "needs_experience"
	// LevelUncertain is "no verdict": the answer was not confident enough to be shown (§9.3, I10).
	LevelUncertain Level = "uncertain"
	// LevelInsufficientInformation is the absence of a comparison, and the reason names the
	// profile rather than the opportunity (§9.3, S7).
	LevelInsufficientInformation Level = "insufficient_information"
	// LevelUndecided is a request that failed or an answer we cannot read, never a fit (I2).
	LevelUndecided Level = "undecided"
)

// Why a level was reached. Wire values, for the same reason as the levels: the screen renders its
// own wording from them.
const (
	// ReasonInsufficientProfile accompanies an answer that found nothing to compare.
	ReasonInsufficientProfile = "insufficient_profile"
	// ReasonNoProfile accompanies the level reached before any request, when no profile is saved.
	ReasonNoProfile = "no_profile"
	// ReasonLowConfidence accompanies a level that was withheld because the answer was below its
	// own floor.
	ReasonLowConfidence = "low_confidence"
	// ReasonIncompleteAnswer is an answer that is missing, of the wrong shape, or outside the five.
	ReasonIncompleteAnswer = "incomplete_answer"
	// ReasonEndpointFailed is the request itself failing (I2).
	ReasonEndpointFailed = "endpoint_failed"
)

const (
	// floor is the line below which no answer becomes a level at all (§9.3).
	floor = 0.45
	// readyFloor is what an unqualified "ready" needs (§9.3).
	readyFloor = 0.70
	// supportFloor is what "ready_with_support" needs: it is easier to say yes when a maintainer
	// answers questions, so its floor is lower (§9.3).
	supportFloor = 0.50
)

// Result is one fit verdict.
type Result struct {
	// Level is what the screen shows.
	Level Level
	// Reason is why, as a wire value. Empty when the level needs no explanation beyond itself.
	Reason string
	// Answer is the answer the judge gave, kept so the screen can say which reading produced the
	// level. Empty when there was no answer to read.
	Answer Fit
	// Probability is the confidence the judge put on that answer. A level reached without asking
	// reports 1: it is not a probability, it is certainty.
	Probability float64
}

// Decide maps the answer one request returned to the level the screen shows, row by row of §9.3.
// It is a pure function — no client, no store, no clock — so the level a test pins is the level
// the product shows (S12).
//
// The rows are read in the order the table lists them, which only matters for
// insufficient_evidence: its row comes first because an answer that found nothing to compare is
// not a weak fit, it is the absence of a comparison.
//
// One row is not in the table: "ready" between the floor and its own 0.70, and
// ready_with_support between the floor and 0.50. The table gives each answer a floor and gives no
// row for what lies between the floor and it, so the only reading that does not invent a verdict
// is none: the answer is withheld and the level says the judgement was not confident enough.
func Decide(answers map[string]nan.Answer) Result {
	answer, confidence, ok := read(answers)
	if !ok {
		return Result{Level: LevelUndecided, Reason: ReasonIncompleteAnswer}
	}
	switch answer {
	case FitInsufficientEvidence:
		return Result{
			Level:       LevelInsufficientInformation,
			Reason:      ReasonInsufficientProfile,
			Answer:      answer,
			Probability: confidence,
		}
	case FitReady, FitReadyWithSupport, FitNeedsPreparation, FitNeedsExperience:
	default:
		// An id outside the five is not an answer this policy can reason about, however
		// confident it sounds.
		return Result{Level: LevelUndecided, Reason: ReasonIncompleteAnswer}
	}

	if confidence < floor {
		return withheld(answer, confidence)
	}

	switch answer {
	case FitReady:
		if confidence >= readyFloor {
			return Result{Level: LevelRecommended, Answer: answer, Probability: confidence}
		}
		return withheld(answer, confidence)
	case FitReadyWithSupport:
		if confidence >= supportFloor {
			return Result{Level: LevelRecommendedWithSupport, Answer: answer, Probability: confidence}
		}
		return withheld(answer, confidence)
	default:
		// needs_preparation and needs_experience are shown as themselves: they are the answer,
		// and their probability travels with them.
		return Result{Level: Level(answer), Answer: answer, Probability: confidence}
	}
}

// withheld is the level for an answer that is above the floor and below its own: the judgement did
// not reach the confidence the row asks for, so it is not shown as a verdict, and the answer it
// came from is kept so the screen can say which reading was withheld.
func withheld(answer Fit, confidence float64) Result {
	return Result{Level: LevelUncertain, Reason: ReasonLowConfidence, Answer: answer, Probability: confidence}
}

// read pulls the one answer out of the response. A missing entry, another type, an empty id and a
// confidence outside [0,1] are all the same thing here: a response this policy cannot read, which
// is a defect of the transport and never a judgement (I2). An id outside the five is refused by the
// switch in Decide, for the same reason.
func read(answers map[string]nan.Answer) (Fit, float64, bool) {
	a, ok := answers[QFit]
	if !ok || a.Type != nan.Choice || a.Choice == "" {
		return "", 0, false
	}
	if math.IsNaN(a.Confidence) || a.Confidence < 0 || a.Confidence > 1 {
		return "", 0, false
	}
	return Fit(a.Choice), a.Confidence, true
}
