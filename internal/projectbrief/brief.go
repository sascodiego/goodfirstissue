package projectbrief

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// Evidence is where one claim comes from: a path relative to the clone, and the line that carries
// it. Without it a claim is the model's opinion wearing a portrait's clothes.
type Evidence struct {
	Path string
	Line int
}

// String renders the citation the way the document writes it.
func (e Evidence) String() string {
	return e.Path + ":" + strconv.Itoa(e.Line)
}

// Item is one claim of the portrait and its citation.
type Item struct {
	// Section is the heading the claim appears under; NatureOf(Section) says which bucket it
	// feeds.
	Section string
	Claim   string
	// Evidence is the citation the model wrote. Audit says whether the clone backs it; a claim
	// whose citation does not exist is reported, never dropped (I10).
	Evidence Evidence
	// Sourced is set by Audit: the cited file exists in the clone and has that line.
	Sourced bool
}

// Document is one portrait: the text a consumer receives, plus the structure behind it.
type Document struct {
	// Text is the answer verbatim. It is what the alignment node and the explore context
	// would read, so trimming it here would edit the model's own output before anyone saw it.
	Text string
	// Items is the parsed citations, used to audit, to count and to branch by nature.
	Items []Item
	// Unsupported names the sections the answer declared insufficient_information for, in
	// document order. A section is either supported or doubtful: never silently empty.
	Unsupported []string
}

// ItemsIn returns the claims under one heading.
func (d *Document) ItemsIn(section string) []Item {
	var items []Item
	for _, item := range d.Items {
		if item.Section == section {
			items = append(items, item)
		}
	}
	return items
}

// Insufficient reports whether the portrait doubts itself everywhere. That is the answer for an
// empty clone, and it is a report, not a failure.
func (d *Document) Insufficient() bool {
	return len(d.Unsupported) == len(Sections)
}

// Parse reads the model's answer and enforces the skeleton. It is strict on purpose: a consumer
// branches by section and by nature, so an improvised heading, a paragraph or an uncited claim
// would have nowhere to go. Refusing is the honest failure — a portrait nobody can audit is worse
// than no portrait.
func Parse(answer string) (*Document, error) {
	if strings.TrimSpace(answer) == "" {
		return nil, errors.New("projectbrief: the answer is empty")
	}
	// The whole answer may be one word: a clone with nothing to describe is a portrait that
	// says so, not an error.
	if strings.EqualFold(strings.TrimSpace(answer), Insufficient) {
		return &Document{Text: answer, Unsupported: namesOf(NatureTechnical, NatureProject)}, nil
	}

	doc := &Document{Text: answer}
	var (
		// declared counts the split lines seen, so a second copy or a late one is caught.
		declaredTechnical bool
		declaredProject   bool
		// next is the section Parse expects next: the skeleton is fixed, so its order is part
		// of it.
		next    int
		current string
	)
	for i, raw := range strings.Split(answer, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			continue

		case strings.HasPrefix(line, technicalKey), strings.HasPrefix(line, projectKey):
			technical := strings.HasPrefix(line, technicalKey)
			key := projectKey
			nature := NatureProject
			if technical {
				key, nature = technicalKey, NatureTechnical
			}
			if err := declare(line, key, nature, current != ""); err != nil {
				return nil, fmt.Errorf("projectbrief: line %d: %w", i+1, err)
			}
			if technical {
				if declaredTechnical {
					return nil, fmt.Errorf("projectbrief: line %d: %s is declared twice", i+1, technicalKey)
				}
				declaredTechnical = true
				continue
			}
			if !declaredTechnical {
				// The order is fixed for the same reason the sections' is: a consumer that
				// reads the first line knows what follows.
				return nil, fmt.Errorf("projectbrief: line %d: %s must come after %s", i+1, projectKey, technicalKey)
			}
			if declaredProject {
				return nil, fmt.Errorf("projectbrief: line %d: %s is declared twice", i+1, projectKey)
			}
			declaredProject = true

		case strings.HasPrefix(line, "## "):
			name := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if !declaredTechnical || !declaredProject {
				return nil, fmt.Errorf("projectbrief: line %d: section %q arrives before the split is declared", i+1, name)
			}
			index := sectionIndex(name)
			if index < 0 {
				return nil, fmt.Errorf("projectbrief: line %d: unknown section %q, want one of %v", i+1, name, namesOf(NatureTechnical, NatureProject))
			}
			if index != next {
				return nil, fmt.Errorf("projectbrief: line %d: section %q out of place: the order is %v",
					i+1, name, namesOf(NatureTechnical, NatureProject))
			}
			next++
			current = name

		case strings.HasPrefix(line, "- "):
			if current == "" {
				return nil, fmt.Errorf("projectbrief: line %d: item outside a section", i+1)
			}
			if contains(doc.Unsupported, current) {
				return nil, fmt.Errorf("projectbrief: line %d: section %q says %s and also cites something", i+1, current, Insufficient)
			}
			item, err := parseItem(current, line)
			if err != nil {
				return nil, fmt.Errorf("projectbrief: line %d: %w", i+1, err)
			}
			doc.Items = append(doc.Items, item)

		case strings.EqualFold(line, Insufficient):
			if current == "" {
				return nil, fmt.Errorf("projectbrief: line %d: doubt outside a section", i+1)
			}
			if contains(doc.Unsupported, current) {
				return nil, fmt.Errorf("projectbrief: line %d: section %q doubts twice", i+1, current)
			}
			if len(doc.ItemsIn(current)) > 0 {
				return nil, fmt.Errorf("projectbrief: line %d: section %q cites something and also doubts", i+1, current)
			}
			doc.Unsupported = append(doc.Unsupported, current)

		default:
			return nil, fmt.Errorf("projectbrief: line %d: %q is neither a section nor a claim: one line per claim, no prose", i+1, line)
		}
	}

	switch {
	case current == "":
		return nil, fmt.Errorf("projectbrief: the answer has no section, want %v", namesOf(NatureTechnical, NatureProject))
	case next != len(Sections):
		// A portrait that skips a section hides the question instead of answering it.
		return nil, fmt.Errorf("projectbrief: section %q is missing", Sections[next].Name)
	}
	// A section that is neither cited nor doubtful would read as "we forgot", which is the one
	// thing I10 forbids.
	for _, section := range Sections {
		if len(doc.ItemsIn(section.Name)) == 0 && !contains(doc.Unsupported, section.Name) {
			return nil, fmt.Errorf("projectbrief: section %q has no claim and does not say %s", section.Name, Insufficient)
		}
	}
	return doc, nil
}

// declare checks one split line against the code's own classification. The declaration is written
// by the model, so it is validated, never trusted: a portrait that calls a project section
// technical would hand the alignment node a fit gate made of onboarding cost.
func declare(line, key string, nature Nature, insideSection bool) error {
	if insideSection {
		return fmt.Errorf("%s arrives after the first section: the split opens the document", key)
	}
	got := splitList(strings.TrimSpace(strings.TrimPrefix(line, key)))
	want := namesOf(nature)
	if len(got) != len(want) {
		return fmt.Errorf("%s declares %v, want exactly %v", key, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			return fmt.Errorf("%s declares %v, want exactly %v", key, got, want)
		}
	}
	return nil
}

// splitList reads a comma-separated declaration. The separators are the document's own, so a
// heading that contains a comma would have to change both — which the fixed skeleton forbids.
func splitList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ", ")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// parseItem reads one claim line: the claim, then the evidence field. Exactly one field is
// required, and it comes last, so a claim may contain the separator without being misread.
func parseItem(section, line string) (Item, error) {
	body := strings.TrimSpace(strings.TrimPrefix(line, "- "))
	segments := strings.Split(body, " — ")
	for i := len(segments) - 1; i >= 0; i-- {
		key, value, ok := strings.Cut(segments[i], ":")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "evidencia") {
			continue
		}
		claim := strings.TrimSpace(strings.Join(segments[:i], " — "))
		if claim == "" {
			return Item{}, errors.New("an item has no claim: only evidence, with nothing it supports")
		}
		if rest := strings.TrimSpace(strings.Join(segments[i+1:], " — ")); rest != "" {
			return Item{}, fmt.Errorf("item %q: %q comes after the evidence and has nowhere to go", claim, rest)
		}
		evidence, err := parseEvidence(value)
		if err != nil {
			return Item{}, fmt.Errorf("item %q: %w", claim, err)
		}
		return Item{Section: section, Claim: claim, Evidence: evidence}, nil
	}
	return Item{}, fmt.Errorf("item %q has no evidencia: without a citation it is the model's opinion", body)
}

// parseEvidence reads <path>:<line>. The path is validated here, once, because every consumer
// afterwards opens it: an absolute path or one that climbs out of the clone would read the host,
// and a portrait of a third-party repository must never do that (I3).
func parseEvidence(value string) (Evidence, error) {
	raw := strings.TrimSpace(value)
	cut := strings.LastIndexByte(raw, ':')
	if cut <= 0 {
		return Evidence{}, fmt.Errorf("evidencia %q is not <path>:<line>", raw)
	}
	path := strings.TrimSpace(raw[:cut])
	number := strings.TrimSpace(raw[cut+1:])
	line, err := strconv.Atoi(number)
	if err != nil || line <= 0 {
		return Evidence{}, fmt.Errorf("evidencia %q has no line number, want <path>:<line>", raw)
	}
	if err := validatePath(path); err != nil {
		return Evidence{}, err
	}
	return Evidence{Path: path, Line: line}, nil
}

// validatePath keeps a citation inside the clone.
func validatePath(path string) error {
	switch {
	case path == "":
		return errors.New("evidencia has no path")
	case strings.HasPrefix(path, "/"), filepath.IsAbs(path), strings.HasPrefix(path, `\`):
		return fmt.Errorf("evidencia %q is absolute: a citation names a file of the clone", path)
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".." {
			return fmt.Errorf("evidencia %q climbs out of the clone", path)
		}
	}
	return nil
}

// sectionIndex is the position of a heading in the fixed skeleton, or -1.
func sectionIndex(name string) int {
	for i, section := range Sections {
		if section.Name == name {
			return i
		}
	}
	return -1
}

// namesOf concatenates sections of several natures, in document order, which is how the document
// names the whole skeleton.
func namesOf(natures ...Nature) []string {
	var names []string
	for _, section := range Sections {
		for _, nature := range natures {
			if section.Nature == nature {
				names = append(names, section.Name)
			}
		}
	}
	return names
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
