package fit

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

// QFit is the one question P3 asks. It is a wire key, not prose: the answer map is keyed by it,
// the policy switches on it, and the stored verdict names it. Renaming it is a contract change.
const QFit = "fit"

// fitInstructions is what the judge is asked. It names the comparison and nothing else: the five
// options already carry the scale, and an instruction that described it again would be a second
// copy of §9.2 that could drift from the first.
var fitInstructions = "Where does this developer's approved profile stand against the technical " +
	"requirements this issue demands?"

// fitCriteria is the five answers of §9.2, verbatim from the design of #13. They are points on one
// line — the cost of closing the technical gap, from zero to infinite — and they are not re-worded
// here: the descriptions are the contract, and the level the screen shows is read from them.
var fitCriteria = map[Fit]string{
	FitReady:                "The profile covers every technical requirement the issue demands. No gap to close.",
	FitReadyWithSupport:     "A technical gap exists but can be closed while doing the work, with a maintainer answering questions.",
	FitNeedsPreparation:     "A technical gap must be closed before starting: study or practice outside this issue.",
	FitNeedsExperience:      "A technical gap cannot be closed within the horizon of this contribution; it requires months of prior experience in the specific domain.",
	FitInsufficientEvidence: "The profile does not describe the technical capabilities this issue requires, so no comparison is possible.",
}

// Criteria returns the five descriptions, so the screen can label the same options the judge was
// given. The map is a fresh copy on every call: a caller cannot rewrite the frozen contract for
// the rest of the process.
func Criteria() map[Fit]string {
	out := make(map[Fit]string, len(fitCriteria))
	for label, description := range fitCriteria {
		out[label] = description
	}
	return out
}

// Questions is what P3 asks, in one request (S1). The map is a fresh copy on every call, for the
// same reason as Criteria.
func Questions() map[string]nan.Question {
	return map[string]nan.Question{QFit: nan.ChoiceQuestion(fitInstructions, criteriaAsStrings())}
}

func criteriaAsStrings() map[string]string {
	out := make(map[string]string, len(fitCriteria))
	for label, description := range fitCriteria {
		out[string(label)] = description
	}
	return out
}

// Version fingerprints the contract: the instruction and the five descriptions. It is part of the
// cache key, so a reworded option misses instead of reusing a verdict produced by another
// question (I14). The ids are sorted because the criteria are a map, and a fingerprint that
// depended on iteration order would miss at random.
func Version() string {
	var b strings.Builder
	b.WriteString(fitInstructions)
	for _, label := range sortedLabels() {
		b.WriteString("\x00")
		b.WriteString(string(label))
		b.WriteString("\x00")
		b.WriteString(fitCriteria[label])
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])[:12]
}

func sortedLabels() []Fit {
	labels := make([]string, 0, len(fitCriteria))
	for label := range fitCriteria {
		labels = append(labels, string(label))
	}
	sort.Strings(labels)

	out := make([]Fit, 0, len(labels))
	for _, label := range labels {
		out = append(out, Fit(label))
	}
	return out
}
