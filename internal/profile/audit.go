package profile

import "strings"

// Audit checks every quote against the CV text and returns the items it could not find.
//
// It never removes anything: an unverifiable quote is evidence for the user, not for us to
// delete (I10, S9). Inferred items carry no quote, so they are neither sourced nor suspect.
func (d *Document) Audit(cv string) []Item {
	haystack := normalizeWhitespace(cv)
	var unsourced []Item
	for i := range d.Items {
		item := &d.Items[i]
		if item.Level == Inferred {
			continue
		}
		item.Sourced = strings.Contains(haystack, normalizeWhitespace(item.Evidence))
		if !item.Sourced {
			unsourced = append(unsourced, *item)
		}
	}
	return unsourced
}

// normalizeWhitespace collapses every run of whitespace to one space, and nothing else: a quote
// that reaches us re-wrapped still matches, while a quote with different words or different case
// is a different quote.
func normalizeWhitespace(s string) string { return strings.Join(strings.Fields(s), " ") }
