// Package profile turns a pasted CV into a document a person can read, correct and approve.
//
// The application decides nothing here: it hands the CV to one chat model and asks for a fixed
// skeleton — education, technical knowledge, experience, languages — where every item says how
// it is supported: a quote copied from the CV, or the assumption it was derived from. The
// document is the artifact the user reviews and edits before it is used to judge fit, so the
// point is that nothing arrives as a hidden inference.
//
// It deliberately does not produce JSON. The consumer is Clef, which reads prose in the P3
// state (PROYECTO.md §9.1), and the artifact the user edits is the same text Clef reads; an
// intermediate shape would only add a renderer between the model and the person (§17.8).
package profile

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

//go:embed prompt.md
var Prompt string

// Section headings. They are fixed vocabulary, not content: the P3 state template (§9.1) and
// the screen both render by them.
const (
	SectionEducation  = "Formación"
	SectionKnowledge  = "Conocimientos técnicos"
	SectionExperience = "Experiencia"
	SectionLanguages  = "Idiomas"
)

// Sections is the complete set, in the order the document must present them.
var Sections = []string{SectionEducation, SectionKnowledge, SectionExperience, SectionLanguages}

// Insufficient is what a model answers when the CV carries nothing technical. It is a word and
// not an empty list because an empty list cannot be told apart from a bug (I10).
const Insufficient = "insufficient_information"

// Level says how the CV supports an item. It is not a measure of skill: §9.3 forbids showing a
// difficulty scale for the issue, and the product never places a person on one.
type Level string

const (
	// Declared is something the CV asserts without backing it with a role or a project.
	Declared Level = "declarado"
	// Demonstrated is something the CV backs with a role, a project or a period.
	Demonstrated Level = "demostrado"
	// Inferred is something the model derived. Assumption says from what.
	Inferred Level = "inferido"
)

// Levels is the complete set, weakest support first.
var Levels = []Level{Declared, Demonstrated, Inferred}

// Item is one line of the document.
type Item struct {
	Section string
	Name    string
	Level   Level
	// Evidence is the quote copied from the CV, without the guillemets. Empty for an inferred
	// item.
	Evidence string
	// Assumption is what an inferred item was derived from. Empty for a quoted item.
	Assumption string
	// Sourced is set by Audit: the quote was found in the CV. Inferred items are never
	// sourced, and never counted as unsourced either.
	Sourced bool
}

// Document is one extraction: the text the user reviews plus the structure behind it.
type Document struct {
	// Text is the answer verbatim. It is what the user sees and edits.
	Text string
	// Items is the parsed structure, used to count, to audit and to render.
	Items []Item
	// Insufficient is true when there was nothing technical to report, whether the model said
	// so or produced sections with no items.
	Insufficient bool
}

// Version hashes the prompt. It is part of the cache key, so changing a rule makes every stored
// extraction miss instead of reusing an answer given under different instructions (I14).
func Version() string {
	sum := sha256.Sum256([]byte(Prompt))
	return hex.EncodeToString(sum[:])[:12]
}

// Parse reads the model's answer and enforces the skeleton. It is strict on purpose: the screen
// renders by section and by item, so an improvised heading or a paragraph would have nowhere to
// go. Prose is not lost — an answer that cannot be parsed is an error the caller reports, never
// a silently trimmed document.
func Parse(answer string) (*Document, error) {
	trimmed := strings.TrimSpace(answer)
	switch {
	case trimmed == "":
		return nil, errors.New("profile: the answer is empty")
	case strings.EqualFold(trimmed, Insufficient):
		return &Document{Text: answer, Insufficient: true}, nil
	}

	doc := &Document{Text: answer}
	current := ""
	for i, raw := range strings.Split(answer, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "## "):
			name := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if !knownSection(name) {
				return nil, fmt.Errorf("profile: line %d: unknown section %q, want one of %v", i+1, name, Sections)
			}
			current = name
		case strings.HasPrefix(line, "- "):
			if current == "" {
				return nil, fmt.Errorf("profile: line %d: item outside a section", i+1)
			}
			item, err := parseItem(current, line)
			if err != nil {
				return nil, fmt.Errorf("profile: line %d: %w", i+1, err)
			}
			doc.Items = append(doc.Items, item)
		default:
			return nil, fmt.Errorf("profile: line %d: %q is neither a section nor an item: one line per item, no prose", i+1, line)
		}
	}

	if current == "" {
		return nil, fmt.Errorf("profile: the answer has no section, want %v", Sections)
	}
	// Sections with no items anywhere carry nothing to review. That is the same doubt as the
	// model saying so, and it is reported the same way.
	doc.Insufficient = len(doc.Items) == 0
	return doc, nil
}

func knownSection(name string) bool {
	for _, section := range Sections {
		if section == name {
			return true
		}
	}
	return false
}

func knownLevel(level Level) bool {
	for _, want := range Levels {
		if level == want {
			return true
		}
	}
	return false
}

// parseItem reads one line: a name, then the fields. Exactly one of evidencia or supuesto is
// required, because an item either shows where it came from or admits that it was inferred.
func parseItem(section, line string) (Item, error) {
	segments := strings.Split(strings.TrimPrefix(line, "- "), " — ")

	// The name may itself contain " — " ("Licenciatura en Sistemas — Universidad X"), so the
	// name is every leading segment that carries no field: fields are recognised by their
	// colon, not by position.
	item := Item{Section: section}
	first := 0
	for ; first < len(segments); first++ {
		if _, _, isField := strings.Cut(segments[first], ":"); isField {
			break
		}
		if item.Name != "" {
			item.Name += " — "
		}
		item.Name += strings.TrimSpace(segments[first])
	}
	switch {
	case item.Name == "":
		return Item{}, errors.New("item has no name")
	case first == len(segments):
		return Item{}, fmt.Errorf("item %q has no fields: want name — nivel: … — evidencia: «…»", item.Name)
	}

	for _, field := range segments[first:] {
		key, value, ok := strings.Cut(field, ":")
		if !ok {
			return Item{}, fmt.Errorf("item %q: %q is not a field: want nivel, evidencia or supuesto", item.Name, field)
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "nivel":
			level := Level(strings.ToLower(strings.TrimSpace(value)))
			if !knownLevel(level) {
				return Item{}, fmt.Errorf("item %q: unknown nivel %q, want one of %v", item.Name, strings.TrimSpace(value), Levels)
			}
			item.Level = level
		case "evidencia":
			item.Evidence = trimGuillemets(strings.TrimSpace(value))
		case "supuesto":
			item.Assumption = strings.TrimSpace(value)
		default:
			return Item{}, fmt.Errorf("item %q: unknown field %q, want nivel, evidencia or supuesto", item.Name, strings.TrimSpace(key))
		}
	}

	switch {
	case item.Level == "":
		return Item{}, fmt.Errorf("item %q has no nivel:, want one of %v", item.Name, Levels)
	case item.Level == Inferred && item.Assumption == "":
		return Item{}, fmt.Errorf("item %q is inferido and does not say supuesto: what it came from", item.Name)
	case item.Level != Inferred && item.Evidence == "":
		return Item{}, fmt.Errorf("item %q is %s and has no evidencia: a quote from the CV", item.Name, item.Level)
	case item.Evidence != "" && item.Assumption != "":
		return Item{}, fmt.Errorf("item %q has both evidencia: and supuesto: it is either quoted or inferred", item.Name)
	}
	return item, nil
}

// trimGuillemets removes the quotes the format asks around a quote, so comparisons work against
// the CV text itself.
func trimGuillemets(s string) string {
	return strings.Trim(s, "«»\"“”'")
}
