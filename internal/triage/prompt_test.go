package triage

import (
	_ "embed"
	"strings"
	"testing"
)

// The contract and the code that sends it are two copies of the same wording, and the
// copy that drifts is always the one nobody runs. These tests make the drift fail.

//go:embed prompt.md
var promptMarkdown string

//go:embed criteria.md
var criteriaMarkdown string

// frozenQuestions is the contract's question list. The benchmark may carry experimental
// questions beside them; those are not frozen and must not be presented as if they were.
var frozenQuestions = []string{
	"kind",
	"self_contained",
	"has_evidence",
	"unambiguous",
	"done_defined",
	"reports_defect",
	"evidence_sufficient",
	"only_for_requester",
	"fits_direction",
}

func bankQuestion(t *testing.T, id string) question {
	t.Helper()
	for _, q := range append(append([]question{}, substanceQuestions...), alignmentQuestions...) {
		if q.ID == id {
			return q
		}
	}
	t.Fatalf("question %q is frozen but the bank does not ask it", id)
	return question{}
}

// Every frozen question must appear in the contract with the exact wording that was
// measured. A "roughly equivalent" prompt is a different prompt, and the numbers in the
// issue would no longer describe what ships.
func TestContractCarriesEveryFrozenQuestionVerbatim(t *testing.T) {
	for _, id := range frozenQuestions {
		q := bankQuestion(t, id)
		if !strings.Contains(promptMarkdown, q.Instructions) {
			t.Errorf("prompt.md does not carry question %q with the measured wording:\n%s", id, q.Instructions)
		}
	}
}

// The frozen list is the contract's whole question set: a question in the bank that is
// not frozen is an experiment, and one that is frozen but missing from the bank would
// measure something else.
func TestFrozenQuestionSetIsClosed(t *testing.T) {
	for _, id := range frozenQuestions {
		q := bankQuestion(t, id)
		if q.Criteria == nil && q.Type == "choice" {
			t.Errorf("question %q is a choice without criteria: the judge would be guessing", id)
		}
		if q.Criteria != nil && q.Type != "choice" {
			t.Errorf("question %q carries criteria but is not a choice", id)
		}
		for _, description := range q.Criteria {
			if len(description) < 20 {
				t.Errorf("question %q has a criteria description too thin to discriminate: %q", id, description)
			}
		}
	}
}

// These files are read by the judge, so they cannot name the machinery around it: a model
// name, an endpoint, or code sample is a leak from our world into its instructions.
func TestContractsNameNoMachinery(t *testing.T) {
	forbidden := []string{"clef", "nan.builders", "systemone", "```go", "```json", "package ", "import (", "func "}

	for name, body := range map[string]string{
		"prompt.md":   withoutDesignRecord(promptMarkdown),
		"criteria.md": withoutDesignRecord(criteriaMarkdown),
	} {
		lower := strings.ToLower(body)
		for _, token := range forbidden {
			if strings.Contains(lower, strings.ToLower(token)) {
				t.Errorf("%s names %q, which belongs to us and not to the contract", name, token)
			}
		}
	}
}

// The Spanish design record at the top of each file is for us, not for the judge, so it
// is excluded from the checks above: it may cite measurements and case identifiers.
func withoutDesignRecord(body string) string {
	trimmed := strings.TrimLeft(body, " \n")
	if !strings.HasPrefix(trimmed, "<!--") {
		return body
	}
	end := strings.Index(trimmed, "-->")
	if end < 0 {
		return ""
	}
	return trimmed[end+len("-->"):]
}
