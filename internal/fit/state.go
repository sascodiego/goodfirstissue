package fit

import (
	"strings"

	"github.com/sascodiego/nan-hackathon-clef/internal/explore"
	"github.com/sascodiego/nan-hackathon-clef/internal/projectbrief"
	"github.com/sascodiego/nan-hackathon-clef/internal/triage"
)

// Issue is what P3 reads about the issue itself.
type Issue struct {
	// Repository is the store key of the corpus, e.g. spf13__cobra.
	Repository string
	Number     int
	// Title is the issue's own title, prefix included. State strips the declared prefix.
	Title string
	Body  string
	// Labels is never read. It is here so a test can plant the original label and prove the state
	// does not carry it: the label is not a fact of the judgement (I6).
	Labels []string
}

// Repository is the two facts of §9.1 that come from the corpus. The third, Tests, comes from the
// exploration, because what a project has is a fact of the tree and what covers this issue is a
// fact of the evidence.
type Repository struct {
	Name            string
	PrimaryLanguage string
}

// Request is one fit question: the issue, the repository it lives in, and the evidence the
// exploration found under it.
type Request struct {
	Issue      Issue
	Repository Repository
	Evidence   explore.Evidence
}

// The markers that keep an absence visible. A section that renders nothing reads as a section that
// forgot to render; a section that names what is missing reads as a fact about this build (I10).
// The two portrait markers are separate because they ask for different repairs: no portrait at all
// is a repository nobody described, and a portrait without its technical section is a document
// that cannot answer the one question P3 crosses with the CV.
const (
	noTitleMarker    = "(no title)"
	noBodyMarker     = "(no body)"
	noneMarker       = "(none)"
	noProfileMarker  = "(no approved profile)"
	noPortraitMarker = "(no portrait: the technical stack was never extracted)"
	noStackMarker    = "(the portrait does not carry a Stack técnico section)"
)

// The two markers that say what a requirement may weigh (S3). Knowledge is the only bucket that
// crosses with a CV; project and process are the cost of joining, and they are here for the judge
// to understand the work, never to gate it (§13, S10).
const (
	evaluableMarker = "[evaluable]"
	contextMarker   = "[contexto]"
)

// State composes the text P3 sends to the judge, with the six sections of §9.1.
//
// It carries no numbers from the exploration: the only probabilities in the state come back from
// the judge, and the counts here are of things a person can point at (I5).
//
// profile is the saved profile's document, verbatim, and portrait is the project portrait. Both
// may be empty, and each emptiness is written down where it happened rather than removing the
// section it belongs to.
func State(req Request, profile, portrait string) string {
	var b strings.Builder

	writeIssue(&b, req.Issue)
	writeRepository(&b, req.Repository, req.Evidence.Tests, portrait)
	writeAreas(&b, req.Evidence)
	writeEvidence(&b, req.Evidence.Requirements)
	writeGaps(&b, req.Evidence.Unknowns, req.Evidence.MissingInformation)
	writeDeveloper(&b, profile)

	return b.String()
}

func writeIssue(b *strings.Builder, issue Issue) {
	// The declared prefix is the author's claim about the kind of work, and the same function that
	// removes it for P1 removes it here: one rule, one implementation (§9.1, I6).
	title, _ := triage.NormalizeTitle(issue.Title)
	if strings.TrimSpace(title) == "" {
		title = noTitleMarker
	}
	body := strings.TrimSpace(issue.Body)
	if body == "" {
		body = noBodyMarker
	}

	b.WriteString("## ISSUE\n")
	b.WriteString("Title: " + title + "\n")
	b.WriteString("Body:\n" + body + "\n\n")
}

func writeRepository(b *strings.Builder, repo Repository, tests []string, portrait string) {
	b.WriteString("## REPOSITORIO\n")
	b.WriteString("Name: " + valueOr(repo.Name) + "\n")
	b.WriteString("Primary language: " + valueOr(repo.PrimaryLanguage) + "\n")
	b.WriteString("Tests: " + testsFact(tests) + "\n")
	b.WriteString("Stack técnico:\n")
	b.WriteString(stackSection(portrait))
	b.WriteString("\n")
}

// valueOr keeps the field visible when the corpus did not have it: an empty value reads as a
// rendering bug, and a missing fact is a fact.
func valueOr(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return noneMarker
}

// testsFact is the yes/no of §9.1 and, when the answer is no, what that "no" is about. The
// exploration names the tests that cover this area, not the project's suite, so an empty list
// cannot be reported as "this project has no tests".
func testsFact(tests []string) string {
	if len(tests) == 0 {
		return "no (not found by the exploration)"
	}
	return "yes (" + strings.Join(tests, ", ") + ")"
}

// stackSection is the only half of the portrait that crosses with a CV: the section its own code
// marks NatureTechnical (S5). The rest of the portrait is onboarding cost and is never a fit gate,
// so it does not travel.
//
// The section is read as text instead of re-parsing the whole document: the portrait is a saved
// artifact written by another build, and a stricter reading here would throw away a technical
// section that is perfectly readable because a heading somewhere else moved.
func stackSection(portrait string) string {
	text := strings.TrimSpace(portrait)
	if text == "" {
		return noPortraitMarker + "\n"
	}

	heading := "## " + projectbrief.SectionStack
	var (
		lines []string
		seen  bool
	)
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case !seen && line == heading:
			seen = true
		case seen && strings.HasPrefix(line, "## "):
			// The next heading ends the section.
			return renderLines(lines)
		case seen && line != "":
			lines = append(lines, line)
		}
	}
	if !seen {
		return noStackMarker + "\n"
	}
	return renderLines(lines)
}

func renderLines(lines []string) string {
	if len(lines) == 0 {
		return noStackMarker + "\n"
	}
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(line + "\n")
	}
	return b.String()
}

func writeAreas(b *strings.Builder, evidence explore.Evidence) {
	b.WriteString("## ÁREAS DE LA ISSUE\n")
	// S1: the issue is the unit of the verdict. The areas group the evidence and say what the work
	// touches; they are not scored one by one, and the text says so where the judge reads it.
	b.WriteString("La issue toca 1 área: es una sola unidad de veredicto. ")
	b.WriteString("Las áreas agrupan la evidencia; no se puntúan por separado.\n")

	b.WriteString("Files:\n")
	if len(evidence.Files) == 0 {
		b.WriteString(noneMarker + "\n")
	}
	for _, file := range evidence.Files {
		line := "- " + file.Path + " " + relationOf(file)
		// A file the exploration did not explain keeps its path and its relation, and no dangling
		// separator: the absence of a reason is not a reason.
		if reason := strings.TrimSpace(file.Reason); reason != "" {
			line += " — " + reason
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("Call paths:\n")
	if len(evidence.CallPaths) == 0 {
		b.WriteString(noneMarker + "\n")
	}
	for _, path := range evidence.CallPaths {
		b.WriteString("- " + strings.Join(path, " → ") + "\n")
	}
	b.WriteString("\n")
}

// relationOf renders the file's own claim about how it was found. A file that declares nothing
// falls to the weakest bucket, which is what explore does with the same omission.
func relationOf(file explore.File) string {
	if strings.TrimSpace(file.Relation) == "" {
		return "[" + explore.RelationUnverified + "]"
	}
	return "[" + file.Relation + "]"
}

func writeEvidence(b *strings.Builder, req explore.Requirements) {
	b.WriteString("## EVIDENCIA DE TODA LA ISSUE\n")
	b.WriteString("Requirements:\n")

	// The three buckets travel in the order §9.1 lists them, each with the marker that says
	// whether it can weigh. Nothing here is counted, sorted or scored: the judge reads what the
	// exploration wrote.
	buckets := []struct {
		name  string
		mark  string
		items []string
	}{
		{"knowledge", evaluableMarker, req.Knowledge},
		{"project", contextMarker, req.Project},
		{"process", contextMarker, req.Process},
	}
	if len(req.Knowledge)+len(req.Project)+len(req.Process) == 0 {
		b.WriteString(noneMarker + "\n")
	}
	for _, bucket := range buckets {
		for _, item := range bucket.items {
			b.WriteString("- " + bucket.name + ": " + item + " " + bucket.mark + "\n")
		}
	}
	b.WriteString("\n")
}

// writeGaps writes what is not known, from both halves of the exploration: what it could not
// determine and what the issue fails to specify. A gap is not filler and none is invented; when
// there is nothing, the section says that instead of disappearing (I10).
func writeGaps(b *strings.Builder, unknowns, missing []string) {
	b.WriteString("## LAGUNAS\n")
	gaps := dedupe(unknowns, missing)
	if len(gaps) == 0 {
		b.WriteString(noneMarker + "\n\n")
		return
	}
	for _, gap := range gaps {
		b.WriteString("- " + gap + "\n")
	}
	b.WriteString("\n")
}

// dedupe joins both lists in order, dropping a gap that both halves already reported: the same
// sentence twice reads as two different doubts.
func dedupe(lists ...[]string) []string {
	var (
		out  []string
		seen = map[string]bool{}
	)
	for _, list := range lists {
		for _, item := range list {
			line := strings.TrimSpace(item)
			if line == "" || seen[line] {
				continue
			}
			seen[line] = true
			out = append(out, line)
		}
	}
	return out
}

// writeDeveloper is the profile exactly as it was approved. It is not re-rendered and not
// re-parsed: the document the person reviewed is the document the judge reads (§17.8).
func writeDeveloper(b *strings.Builder, profile string) {
	b.WriteString("## DESARROLLADOR\n")
	if text := strings.TrimSpace(profile); text != "" {
		// A blank line keeps the profile's own headings from reading as sections of the state.
		b.WriteString("\n" + text + "\n")
		return
	}
	b.WriteString(noProfileMarker + "\n")
}
