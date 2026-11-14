package profile_test

import (
	"strings"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/profile"
)

// A well-formed answer: four sections, one line per item, every item carrying its level and
// either a quote or the assumption it rests on.
const validDocument = `## Formación
- Licenciatura en Sistemas — Universidad X (2015-2020) — nivel: demostrado — evidencia: «Licenciatura en Sistemas, Universidad X, 2015-2020»

## Conocimientos técnicos
- Python — nivel: demostrado — evidencia: «5 años de desarrollo backend con Django»
- API REST — nivel: inferido — supuesto: se deriva de «5 años de desarrollo backend con Django»

## Experiencia
- Backend de pagos, 5 años — nivel: demostrado — evidencia: «5 años de desarrollo backend con Django»

## Idiomas
- Inglés técnico — nivel: declarado — evidencia: «inglés técnico»
`

func TestParseReadsTheFourSectionsAndTheirItems(t *testing.T) {
	doc, err := profile.Parse(validDocument)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if doc.Text != validDocument {
		t.Error("Text is not the answer verbatim: it is what the user reviews")
	}
	if len(doc.Items) != 5 {
		t.Fatalf("items = %d, want 5", len(doc.Items))
	}
	if got := doc.Items[0].Section; got != profile.SectionEducation {
		t.Errorf("first item section = %q, want %q", got, profile.SectionEducation)
	}

	python := doc.Items[1]
	if python.Name != "Python" || python.Level != profile.Demonstrated {
		t.Errorf("item = %+v, want Python as a demonstrated capability", python)
	}
	if python.Evidence != "5 años de desarrollo backend con Django" {
		t.Errorf("evidence = %q, want the quote without the guillemets", python.Evidence)
	}

	inferred := doc.Items[2]
	if inferred.Level != profile.Inferred {
		t.Errorf("level = %q, want %q", inferred.Level, profile.Inferred)
	}
	if inferred.Assumption == "" {
		t.Error("an inferred item must say what it was inferred from")
	}
	if inferred.Evidence != "" {
		t.Errorf("evidence = %q, want none: an inferred item is not quoted", inferred.Evidence)
	}
}

// The skeleton is fixed: a section we did not ask for means the model improvised a shape the
// screen cannot render.
func TestParseRejectsAnUnknownSection(t *testing.T) {
	_, err := profile.Parse("## Hobbies\n- Senderismo — nivel: declarado — evidencia: «me gusta el senderismo»\n")
	if err == nil {
		t.Fatal("Parse() accepted an unknown section")
	}
	if !strings.Contains(err.Error(), "Hobbies") {
		t.Errorf("error does not name the section: %v", err)
	}
}

func TestParseRejectsAnItemOutsideAnySection(t *testing.T) {
	_, err := profile.Parse("- Python — nivel: demostrado — evidencia: «Python»\n")
	if err == nil {
		t.Fatal("Parse() accepted an item outside a section")
	}
}

// Prose between items is what makes a profile unrenderable and bloated; one line per item is
// the ceiling (S8).
func TestParseRejectsProseOutsideItems(t *testing.T) {
	_, err := profile.Parse("## Idiomas\nEl candidato parece tener buen nivel de inglés en general.\n")
	if err == nil {
		t.Fatal("Parse() accepted a paragraph")
	}
}

func TestParseRejectsAnItemWithoutLevel(t *testing.T) {
	_, err := profile.Parse("## Conocimientos técnicos\n- Go — evidencia: «Go»\n")
	if err == nil {
		t.Fatal("Parse() accepted an item without a level")
	}
	if !strings.Contains(err.Error(), "nivel") {
		t.Errorf("error does not name the missing field: %v", err)
	}
}

func TestParseRejectsAnItemWithNeitherQuoteNorAssumption(t *testing.T) {
	_, err := profile.Parse("## Conocimientos técnicos\n- Go — nivel: demostrado\n")
	if err == nil {
		t.Fatal("Parse() accepted an item with no evidence and no assumption")
	}
}

// Either the CV says it, or the model inferred it and says from what. Not both: an item that
// claims a quote and an inference at the same time hides which one the user should check.
func TestParseRejectsAnItemWithBothQuoteAndAssumption(t *testing.T) {
	_, err := profile.Parse("## Conocimientos técnicos\n- Go — nivel: demostrado — evidencia: «Go» — supuesto: del backend\n")
	if err == nil {
		t.Fatal("Parse() accepted an item with both a quote and an assumption")
	}
}

func TestParseRejectsAnUnknownLevel(t *testing.T) {
	_, err := profile.Parse("## Conocimientos técnicos\n- Go — nivel: senior — evidencia: «Go»\n")
	if err == nil {
		t.Fatal("Parse() accepted a seniority level")
	}
	if !strings.Contains(err.Error(), "senior") {
		t.Errorf("error does not name the rejected value: %v", err)
	}
}

// S2: a CV with nothing technical produces doubt, and the model is told to say exactly this.
func TestParseReadsInsufficientInformation(t *testing.T) {
	doc, err := profile.Parse("insufficient_information")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !doc.Insufficient {
		t.Error("Insufficient = false, want true")
	}
	if len(doc.Items) != 0 {
		t.Errorf("items = %d, want none", len(doc.Items))
	}
}

// The version is part of the cache key, so it has to be stable across runs and short enough
// to read.
func TestVersionIsStableAndShort(t *testing.T) {
	v := profile.Version()
	if len(v) != 12 {
		t.Errorf("Version() = %q, want 12 characters", v)
	}
	if v != profile.Version() {
		t.Error("Version() is not deterministic")
	}
	if strings.ContainsAny(v, "ghijklmnopqrstuvwxyzGHIJKLMNOPQRSTUVWXYZ") {
		t.Errorf("Version() = %q, want hexadecimal", v)
	}
}

// The prompt carries the rules the model must follow, and it is the only thing the version
// hashes: if the rules change, every cached extraction must miss.
func TestPromptStatesTheSkeletonAndTheUntrustedRule(t *testing.T) {
	for _, want := range []string{
		profile.SectionEducation,
		profile.SectionKnowledge,
		profile.SectionExperience,
		profile.SectionLanguages,
		"nivel:",
		"evidencia:",
		"supuesto:",
		"insufficient_information",
	} {
		if !strings.Contains(profile.Prompt, want) {
			t.Errorf("prompt.md does not mention %q", want)
		}
	}
}
