package profile_test

import (
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/profile"
)

const sampleCV = `Curriculum
Licenciatura en Sistemas, Universidad X, 2015-2020

Experiencia
5 años de desarrollo backend con Django.
PostgreSQL

Idiomas
inglés técnico`

// S1/S9: a quote that is in the CV is sourced. Whitespace and line wrapping are forgiven,
// because a CV wraps and a model re-flows what it copies.
func TestAuditFindsTheQuoteInTheCV(t *testing.T) {
	doc, err := profile.Parse(validDocument)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if unsourced := doc.Audit(sampleCV); len(unsourced) != 0 {
		t.Fatalf("unsourced = %d, want 0: every quote is in the CV\n%+v", len(unsourced), unsourced)
	}
	for _, item := range doc.Items {
		if item.Level == profile.Inferred {
			continue
		}
		if !item.Sourced {
			t.Errorf("%q is quoted from the CV but Sourced = false", item.Name)
		}
	}
}

// A wrapped quote still matches: the CV wraps the sentence, the document puts it on one line
// because the skeleton is one line per item.
func TestAuditForgivesWhitespace(t *testing.T) {
	wrapped := "Licenciatura en Sistemas,\n   Universidad X"
	doc, err := profile.Parse("## Formación\n- Licenciatura — nivel: demostrado — evidencia: «Licenciatura en Sistemas, Universidad X»\n")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if unsourced := doc.Audit(wrapped); len(unsourced) != 0 {
		t.Errorf("a quote that only differs in whitespace was flagged: %+v", unsourced)
	}
}

// S9: a quote the CV does not contain is reported, never dropped in silence: the user decides
// whether their CV is incomplete or the model invented it.
func TestAuditFlagsAQuoteThatIsNotInTheCV(t *testing.T) {
	doc, err := profile.Parse("## Conocimientos técnicos\n- Kubernetes — nivel: demostrado — evidencia: «8 años orquestando Kubernetes»\n")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	unsourced := doc.Audit(sampleCV)
	if len(unsourced) != 1 {
		t.Fatalf("unsourced = %d, want 1", len(unsourced))
	}
	if unsourced[0].Name != "Kubernetes" {
		t.Errorf("unsourced item = %q, want Kubernetes", unsourced[0].Name)
	}
	if doc.Items[0].Sourced {
		t.Error("Sourced = true for a quote that is not in the CV")
	}
	// The item stays in the document: it is the user who removes it.
	if len(doc.Items) != 1 || doc.Items[0].Name != "Kubernetes" {
		t.Error("the item was dropped from the document")
	}
}

// A quote with different words or different case is a different quote.
func TestAuditIsExactExceptForWhitespace(t *testing.T) {
	doc, err := profile.Parse("## Conocimientos técnicos\n- Python — nivel: demostrado — evidencia: «5 años de desarrollo backend con python»\n")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if unsourced := doc.Audit(sampleCV); len(unsourced) != 1 {
		t.Error("a lowercased quote was accepted: only whitespace is forgiven")
	}
}

// An inferred item is not quoted, so it is neither sourced nor suspect: the assumption is what
// the user reviews.
func TestAuditIgnoresInferredItems(t *testing.T) {
	doc, err := profile.Parse(validDocument)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	doc.Audit(sampleCV)

	for _, item := range doc.Items {
		if item.Level == profile.Inferred && item.Sourced {
			t.Errorf("%q is inferred and cannot be sourced", item.Name)
		}
	}
}

// A document with sections but no items has nothing to review, which is the same doubt the model
// words as insufficient_information (S2). Reporting it as a valid empty profile would be a lie by
// omission.
func TestParseTreatsAnEmptyDocumentAsInsufficient(t *testing.T) {
	doc, err := profile.Parse("## Formación\n\n## Conocimientos técnicos\n\n## Experiencia\n\n## Idiomas\n")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !doc.Insufficient {
		t.Error("Insufficient = false for a document with no items")
	}
}
