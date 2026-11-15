package fit

import (
	"strings"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

// The wording is the contract the judge answers, and §9.2 freezes it: the five descriptions are
// points on one line — the cost of closing the technical gap, from zero to infinite — and they
// are not re-worded here. These tests are the wedge that keeps the program from drifting away from
// the design of #13.

// designed is §9.2 verbatim. It is written out in full because a "roughly equivalent" description
// is a different question, and the level the screen shows would no longer be the measured one.
var designed = map[Fit]string{
	FitReady:                "The profile covers every technical requirement the issue demands. No gap to close.",
	FitReadyWithSupport:     "A technical gap exists but can be closed while doing the work, with a maintainer answering questions.",
	FitNeedsPreparation:     "A technical gap must be closed before starting: study or practice outside this issue.",
	FitNeedsExperience:      "A technical gap cannot be closed within the horizon of this contribution; it requires months of prior experience in the specific domain.",
	FitInsufficientEvidence: "The profile does not describe the technical capabilities this issue requires, so no comparison is possible.",
}

// P3 asks one question and no other: the level is one per issue (S1), and a second question about
// the same line would be overlap by construction (§9.2).
func TestP3AsksExactlyOneChoiceQuestion(t *testing.T) {
	questions := Questions()
	if len(questions) != 1 {
		t.Fatalf("P3 asks %d questions, want 1", len(questions))
	}
	question, ok := questions[QFit]
	if !ok {
		t.Fatalf("P3 does not ask %q", QFit)
	}
	if question.Type != nan.Choice {
		t.Fatalf("the fit question is %q, want %q: a score would fake a precision the model does not have (§9.3)", question.Type, nan.Choice)
	}
}

// The five options are the frozen ids. A sixth would be an experiment leaking into production; a
// missing one is a level the policy can never reach.
func TestTheFiveAnswersAreTheFrozenOnes(t *testing.T) {
	criteria, ok := Questions()[QFit].Criteria.(map[string]string)
	if !ok {
		t.Fatalf("the criteria are %T, want map[string]string", Questions()[QFit].Criteria)
	}
	if len(criteria) != len(designed) {
		t.Fatalf("the question offers %d options, want %d", len(criteria), len(designed))
	}
	for label := range designed {
		if _, ok := criteria[string(label)]; !ok {
			t.Errorf("the question does not offer %q", label)
		}
	}
}

func TestTheDescriptionsAreTheDesignedOnesVerbatim(t *testing.T) {
	criteria := Criteria()
	for label, want := range designed {
		if got := criteria[label]; got != want {
			t.Errorf("description of %q:\n got %q\nwant %q", label, got, want)
		}
	}
}

// §9.2: none of the descriptions mentions beginners, juniors, levels, years, or whether the person
// has contributed before. Being new is not an acceptance criterion (§1.2), so the wording cannot
// smuggle one in.
func TestTheQuestionNeverMentionsSeniority(t *testing.T) {
	forbidden := []string{"beginner", "junior", "senior", "novice", "expert", "years", "year of", "level"}
	text := strings.ToLower(Questions()[QFit].Instructions)
	for label, description := range Criteria() {
		text += "\n" + strings.ToLower(string(label)) + "\n" + strings.ToLower(description)
	}
	for _, word := range forbidden {
		if strings.Contains(text, word) {
			t.Errorf("the question carries %q: seniority is never a fit criterion (§1.2)", word)
		}
	}
}

// The contract is a value, not a handle: a caller that edits the criteria it received must not
// rewrite what the next request asks.
func TestTheQuestionIsAFreshCopy(t *testing.T) {
	first := Questions()[QFit]
	criteria, ok := first.Criteria.(map[string]string)
	if !ok {
		t.Fatalf("the criteria are %T, want map[string]string", first.Criteria)
	}
	criteria[string(FitReady)] = "rewritten by a caller"

	if got := Criteria()[FitReady]; got == "rewritten by a caller" {
		t.Fatal("editing the returned criteria rewrote the frozen contract")
	}
	asked, ok := Questions()[QFit].Criteria.(map[string]string)
	if !ok {
		t.Fatalf("the criteria are %T, want map[string]string", Questions()[QFit].Criteria)
	}
	if asked[string(FitReady)] == "rewritten by a caller" {
		t.Fatal("editing the returned criteria rewrote what P3 asks")
	}
}

// Version is part of the cache key, so it has to fingerprint the wording: a description that
// changes without a version change would reuse a verdict produced by another question (I14).
func TestVersionFingerprintsTheWording(t *testing.T) {
	base := Version()
	if len(base) != 12 {
		t.Fatalf("Version = %q, want a 12-character fingerprint", base)
	}
	if again := Version(); again != base {
		t.Fatalf("Version is not stable: %q then %q", base, again)
	}

	original := fitCriteria[FitReady]
	fitCriteria[FitReady] = original + " Reworded."
	t.Cleanup(func() { fitCriteria[FitReady] = original })
	if Version() == base {
		t.Error("a reworded description did not change the version")
	}
}

func TestVersionFingerprintsTheInstruction(t *testing.T) {
	base := Version()

	original := fitInstructions
	fitInstructions = original + " And say so briefly."
	t.Cleanup(func() { fitInstructions = original })
	if Version() == base {
		t.Error("a reworded instruction did not change the version")
	}
}
