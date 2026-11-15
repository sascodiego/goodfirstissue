package triage

import (
	_ "embed"
)

// Prompt is the contract the judge reads. It is embedded rather than loaded so the binary
// answers with the wording it was built with: a file that changes after the build cannot
// silently alter what a running process asks (I4).
//
//go:embed prompt.md
var Prompt string

// Criteria is the frozen mechanism. It reaches the fingerprint and never the judge: it says
// what each answer permits, which is our business and not the model's.
//
//go:embed criteria.md
var Criteria string

// PolicyVersion names the mechanism, and it is written down in criteria.md as well. The two
// are checked against each other by the tests, so a version bump cannot be forgotten.
const PolicyVersion = "p1-1"
