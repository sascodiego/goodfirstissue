package projectbrief

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"strings"
)

// Prompt is the extraction contract. It is embedded and never assembled at run time: it is the
// document's only authority on shape, so it ships with the binary that parses against it.
//
//go:embed prompt.md
var Prompt string

// schemaVersion changes whenever Parse or the evidence rules change the way an answer is read.
// Without it, a stricter parser would keep serving answers written under looser rules: the same
// question read another way, which I14 forbids.
const schemaVersion = "1"

// Version hashes the prompt and the reading rules. It travels in every result and, from the next
// task on, in the cache key: changing either makes a stored portrait miss instead of being
// presented as fresh (I14).
func Version() string {
	sum := sha256.Sum256([]byte(schemaVersion + "\x00" + Prompt))
	return hex.EncodeToString(sum[:])[:12]
}

// InsufficientPortrait is the document for a clone that holds nothing to describe: the fixed
// skeleton with every section admitting it cannot be supported.
//
// It is rendered from the same constants Parse enforces, and then Parse reads it back like any
// other answer — one shape, one reader. A hand-built Document here would be a second way to be a
// portrait, and the two would drift.
func InsufficientPortrait() string {
	var b strings.Builder
	b.WriteString(technicalKey)
	b.WriteString(" ")
	b.WriteString(strings.Join(namesOf(NatureTechnical), ", "))
	b.WriteString("\n")
	b.WriteString(projectKey)
	b.WriteString(" ")
	b.WriteString(strings.Join(namesOf(NatureProject), ", "))
	b.WriteString("\n")
	for _, section := range Sections {
		b.WriteString("\n## ")
		b.WriteString(section.Name)
		b.WriteString("\n")
		b.WriteString(Insufficient)
		b.WriteString("\n")
	}
	return b.String()
}
