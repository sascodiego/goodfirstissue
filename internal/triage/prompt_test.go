package triage

import (
	"strings"
	"testing"
)

// The contract and the questions that are actually sent are two copies of the same wording,
// and the copy that drifts is always the one nobody runs. The guard that keeps them in step
// lives with the production question set (questions_test.go); what remains here are the
// promises the contract makes about itself.

// These files are read by the judge, so they cannot name the machinery around it: a model
// name, an endpoint, or code sample is a leak from our world into its instructions.
func TestContractsNameNoMachinery(t *testing.T) {
	forbidden := []string{"clef", "nan.builders", "systemone", "```go", "```json", "package ", "import (", "func "}

	for name, body := range map[string]string{
		"prompt.md":   withoutDesignRecord(Prompt),
		"criteria.md": withoutDesignRecord(Criteria),
	} {
		lower := strings.ToLower(body)
		for _, token := range forbidden {
			if strings.Contains(lower, strings.ToLower(token)) {
				t.Errorf("%s names %q, which belongs to us and not to the contract", name, token)
			}
		}
	}
}

// The mechanism is named in two places, and the cache key depends on the name matching what
// the document declares: a silent bump in one of them would serve answers from another policy.
func TestThePolicyVersionIsTheOneTheMechanismDeclares(t *testing.T) {
	if !strings.Contains(Criteria, "Policy version: `"+PolicyVersion+"`") {
		t.Fatalf("criteria.md does not declare policy version %q", PolicyVersion)
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
