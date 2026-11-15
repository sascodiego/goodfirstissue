package fit

import (
	"math"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

// One test per row of §9.3. The rows are the product, so a change to a threshold or to the order
// of the table has to break a named test instead of quietly moving a level.

// answered builds the answer a service returns for the one question P3 asks. Tests never reach the
// network: the policy is a pure function of that answer (S12).
func answered(label Fit, confidence float64) map[string]nan.Answer {
	return map[string]nan.Answer{QFit: {Type: nan.Choice, Choice: string(label), Confidence: confidence}}
}

// levelOf fails with the reason in view, because a wrong level and a wrong reason are different
// defects and a bare Level comparison hides which one happened.
func levelOf(t *testing.T, got Result, want Level) {
	t.Helper()
	if got.Level != want {
		t.Fatalf("level = %q (%s), want %q", got.Level, got.Reason, want)
	}
}

// §9.3 row 1: an answer that found nothing to compare is not a weak fit. The reason names the
// profile, because the opportunity is not what was missing (S7).
func TestInsufficientEvidenceIsInsufficientInformation(t *testing.T) {
	got := Decide(answered(FitInsufficientEvidence, 0.9))

	levelOf(t, got, LevelInsufficientInformation)
	if got.Reason != ReasonInsufficientProfile {
		t.Errorf("reason = %q, want %q: the missing half is the profile", got.Reason, ReasonInsufficientProfile)
	}
	if got.Answer != FitInsufficientEvidence {
		t.Errorf("answer = %q, want %q", got.Answer, FitInsufficientEvidence)
	}
}

// §9.3 row 2: below 0.45 no answer becomes a level, whichever answer it was.
func TestAnAnswerBelowTheFloorIsUncertain(t *testing.T) {
	for _, label := range []Fit{FitReady, FitReadyWithSupport, FitNeedsPreparation, FitNeedsExperience} {
		t.Run(string(label), func(t *testing.T) {
			got := Decide(answered(label, 0.44))

			levelOf(t, got, LevelUncertain)
			if got.Reason != ReasonLowConfidence {
				t.Errorf("reason = %q, want %q", got.Reason, ReasonLowConfidence)
			}
			if got.Probability != 0.44 {
				t.Errorf("probability = %v, want the answer's 0.44", got.Probability)
			}
		})
	}
}

// The floor is inclusive: exactly 0.45 is a level, and 0.4499 is not.
func TestTheFloorIsInclusive(t *testing.T) {
	levelOf(t, Decide(answered(FitNeedsPreparation, 0.45)), LevelNeedsPreparation)
	levelOf(t, Decide(answered(FitNeedsPreparation, 0.4499)), LevelUncertain)
}

// §9.3 row 3: an unqualified yes needs 0.70.
func TestReadyAtItsOwnFloorIsRecommended(t *testing.T) {
	got := Decide(answered(FitReady, 0.70))

	levelOf(t, got, LevelRecommended)
	if got.Probability != 0.70 {
		t.Errorf("probability = %v, want the answer's 0.70: the level never replaces the number", got.Probability)
	}
}

// §9.3 row 4: with a maintainer answering questions the same gap is a recommendation at 0.50.
func TestReadyWithSupportAtItsOwnFloorIsRecommendedWithSupport(t *testing.T) {
	got := Decide(answered(FitReadyWithSupport, 0.50))

	levelOf(t, got, LevelRecommendedWithSupport)
	if got.Probability != 0.50 {
		t.Errorf("probability = %v, want the answer's 0.50", got.Probability)
	}
}

// The two rows that share a threshold are not the same row: 0.50 is enough for support and not
// enough for ready.
func TestTheSameProbabilityMeansDifferentThingsPerAnswer(t *testing.T) {
	levelOf(t, Decide(answered(FitReady, 0.50)), LevelUncertain)
	levelOf(t, Decide(answered(FitReadyWithSupport, 0.50)), LevelRecommendedWithSupport)
}

// The table gives each answer a floor and says nothing about what lies between the floor and it.
// Inventing a level there would be inventing a verdict, so the answer is withheld (I10).
func TestReadyBetweenTheFloorAndItsOwnIsWithheld(t *testing.T) {
	for _, confidence := range []float64{0.45, 0.60, 0.69} {
		got := Decide(answered(FitReady, confidence))
		levelOf(t, got, LevelUncertain)
		if got.Answer != FitReady {
			t.Errorf("at %v: answer = %q, want the answer kept", confidence, got.Answer)
		}
	}
}

func TestReadyWithSupportBetweenTheFloorAndItsOwnIsWithheld(t *testing.T) {
	levelOf(t, Decide(answered(FitReadyWithSupport, 0.49)), LevelUncertain)
}

// §9.3 row 5: the needs_* answers are shown as themselves, with their probability.
func TestNeedsPreparationKeepsItsLevelAndProbability(t *testing.T) {
	got := Decide(answered(FitNeedsPreparation, 0.83))

	levelOf(t, got, LevelNeedsPreparation)
	if got.Probability != 0.83 {
		t.Errorf("probability = %v, want the answer's 0.83", got.Probability)
	}
}

func TestNeedsExperienceKeepsItsLevelAndProbability(t *testing.T) {
	got := Decide(answered(FitNeedsExperience, 0.61))

	levelOf(t, got, LevelNeedsExperience)
	if got.Probability != 0.61 {
		t.Errorf("probability = %v, want the answer's 0.61", got.Probability)
	}
}

// The two needs_* answers are shown as themselves, which means the level and the answer are the
// same word by design (§9.3 "se muestra fit con su probabilidad").
func TestTheNeedsAnswersAreTheirOwnLevel(t *testing.T) {
	if string(FitNeedsPreparation) != string(LevelNeedsPreparation) {
		t.Errorf("fit %q and level %q are the same row of §9.3 and must read the same", FitNeedsPreparation, LevelNeedsPreparation)
	}
	if string(FitNeedsExperience) != string(LevelNeedsExperience) {
		t.Errorf("fit %q and level %q are the same row of §9.3 and must read the same", FitNeedsExperience, LevelNeedsExperience)
	}
}

// The row order of §9.3 is part of the table: insufficient_evidence comes before the floor, so an
// unconfident answer that found nothing to compare still says the profile is what is missing.
func TestInsufficientEvidenceOutranksTheFloor(t *testing.T) {
	levelOf(t, Decide(answered(FitInsufficientEvidence, 0.20)), LevelInsufficientInformation)
}

// I2: an answer we cannot read is not a fit. Every one of these is a defect of the response, not a
// judgement about the person or the issue.
func TestAnUnreadableAnswerIsUndecided(t *testing.T) {
	cases := []struct {
		name    string
		answers map[string]nan.Answer
	}{
		{"no answer at all", nil},
		{"another question only", map[string]nan.Answer{"other": {Type: nan.Choice, Choice: string(FitReady), Confidence: 0.9}}},
		{"wrong type", map[string]nan.Answer{QFit: {Type: nan.Noul, Noul: 0.9}}},
		{"no label", map[string]nan.Answer{QFit: {Type: nan.Choice, Choice: "", Confidence: 0.9}}},
		{"a sixth answer", map[string]nan.Answer{QFit: {Type: nan.Choice, Choice: "probably_fine", Confidence: 0.9}}},
		{"negative trust", answered(FitReady, -0.1)},
		{"trust beyond one", answered(FitReady, 1.5)},
		{"unreadable number", answered(FitReady, math.NaN())},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Decide(c.answers)
			levelOf(t, got, LevelUndecided)
			if got.Reason != ReasonIncompleteAnswer {
				t.Errorf("reason = %q, want %q", got.Reason, ReasonIncompleteAnswer)
			}
		})
	}
}

// Every level the policy emits is one of the constants, so the screen never has to render a value
// nobody defined.
func TestThePolicyOnlyEmitsDeclaredLevels(t *testing.T) {
	declared := map[Level]bool{
		LevelRecommended: true, LevelRecommendedWithSupport: true,
		LevelNeedsPreparation: true, LevelNeedsExperience: true,
		LevelUncertain: true, LevelInsufficientInformation: true, LevelUndecided: true,
	}
	for _, label := range []Fit{FitReady, FitReadyWithSupport, FitNeedsPreparation, FitNeedsExperience, FitInsufficientEvidence, "invented"} {
		for _, confidence := range []float64{0, 0.4500, 0.50, 0.69, 0.70, 1} {
			got := Decide(answered(label, confidence))
			if !declared[got.Level] {
				t.Fatalf("%q at %v produced level %q, which is not a declared level", label, confidence, got.Level)
			}
		}
	}
}
