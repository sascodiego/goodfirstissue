package fit

import (
	"strings"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/explore"
)

// The state is the whole of what the judge sees, so what it carries and what it drops is the
// product. These tests fix both: the six sections of §9.1, the markers that say what may weigh,
// and the absences that are written down instead of being dropped.

// headings is §9.1 in order. The order is part of the template: a reader that knows the first
// heading knows what follows.
var headings = []string{
	"## ISSUE",
	"## REPOSITORIO",
	"## ÁREAS DE LA ISSUE",
	"## EVIDENCIA DE TODA LA ISSUE",
	"## LAGUNAS",
	"## DESARROLLADOR",
}

const testProfile = `## Conocimientos técnicos
- Go — nivel: demostrado — evidencia: «dos años de backend»
## Experiencia
- Pagos — nivel: declarado — evidencia: «backend de pagos»`

const testPortrait = `TÉCNICO: Stack técnico
PROYECTO: Qué es, Arquitectura y componentes, Convenciones y cómo se construye, Dirección: qué acepta y qué no, Límites de lo observable

## Qué es
- Una librería para construir aplicaciones de línea de comandos — evidencia: README.md:4

## Stack técnico
- Go, sin dependencias de terceros — evidencia: go.mod:3
- ` + "`go test ./...`" + ` corre la suite — evidencia: Makefile:12

## Arquitectura y componentes
- Cada comando es un paquete propio bajo cmd/ — evidencia: CONTRIBUTING.md:22
`

func anIssue() Issue {
	return Issue{
		Repository: "spf13__cobra",
		Number:     13,
		Title:      "bug: Commands lose their parent when added twice",
		Body:       "Adding the same command twice makes the second one shadow the first.",
		Labels:     []string{"good first issue", "help wanted", "difficulty: easy"},
	}
}

func anEvidence() explore.Evidence {
	return explore.Evidence{
		Intent:    "Adding a command twice must keep both parents.",
		Files:     []explore.File{{Path: "internal/migrations/runner.go", Symbol: "Runner.Apply", Line: 44, Reason: "define el orden de aplicación.", Relation: explore.RelationObserved}},
		CallPaths: [][]string{{"AddCommand", "Commands", "NamePadding"}},
		Tests:     []string{"command_test.go:TestAddCommandTwice"},
		Requirements: explore.Requirements{
			Knowledge: []string{"ciclo de migraciones en bases relacionales"},
			Project:   []string{"el orden de aplicación vive en internal/migrations/runner.go:44"},
			Process:   []string{"PR con tests; CI corre go test ./..."},
		},
		Unknowns:           []string{"qué debe pasar con migraciones ya aplicadas"},
		MissingInformation: []string{"la issue no dice qué motores de base están en alcance"},
	}
}

func aRequest() Request {
	return Request{
		Issue:      anIssue(),
		Repository: Repository{Name: "spf13/cobra", PrimaryLanguage: "Go"},
		Evidence:   anEvidence(),
	}
}

// whole is the state of a request with everything present, so each test can remove exactly one
// thing and watch what changes.
func whole() string { return State(aRequest(), testProfile, testPortrait) }

func TestTheStateCarriesTheSixSectionsInOrder(t *testing.T) {
	state := whole()
	at := -1
	for _, heading := range headings {
		next := strings.Index(state, heading)
		if next < 0 {
			t.Fatalf("the state has no %q section:\n%s", heading, state)
		}
		if next < at {
			t.Fatalf("%q arrives out of place:\n%s", heading, state)
		}
		at = next
	}
}

// The declared prefix is the author's claim about the kind, not a fact about the work. P1 strips it
// and P3 reads the same title, through the same function, for the same reason.
func TestTheTitleLosesTheDeclaredPrefix(t *testing.T) {
	state := whole()
	if strings.Contains(state, "bug: Commands") {
		t.Error("the declared prefix reached the state")
	}
	if !strings.Contains(state, "Title: Commands lose their parent when added twice") {
		t.Errorf("the title is not the normalized one:\n%s", state)
	}
}

// I6: the label is not a fact of the judgement. It has no place in the template, and the state must
// not grow one because the tracker happened to send it.
func TestALabelNeverSurfaces(t *testing.T) {
	state := whole()
	for _, label := range anIssue().Labels {
		if strings.Contains(state, label) {
			t.Errorf("the label %q reached the state", label)
		}
	}
}

// S3: knowledge is what can weigh; project and process are there to be understood and never to
// gate. The markers are the whole of that rule inside the text, so they are checked one by one.
func TestEveryRequirementCarriesItsMarker(t *testing.T) {
	state := whole()
	want := []string{
		"- knowledge: ciclo de migraciones en bases relacionales [evaluable]",
		"- project: el orden de aplicación vive en internal/migrations/runner.go:44 [contexto]",
		"- process: PR con tests; CI corre go test ./... [contexto]",
	}
	for _, line := range want {
		if !strings.Contains(state, line) {
			t.Errorf("the state does not carry %q:\n%s", line, state)
		}
	}
	if got := strings.Count(state, "[evaluable]"); got != 1 {
		t.Errorf("%d requirements are marked evaluable, want only the knowledge one", got)
	}
	if got := strings.Count(state, "[contexto]"); got != 2 {
		t.Errorf("%d requirements are marked contexto, want the project and process ones", got)
	}
}

// S5: the three facts, and only the section of the portrait its own code marks technical. The
// portrait's project half says in code that it is never a fit gate, so its claims must not travel.
func TestTheRepositorySectionIsThreeFactsAndTheStack(t *testing.T) {
	state := whole()
	for _, want := range []string{
		"Name: spf13/cobra",
		"Primary language: Go",
		"Tests: yes (command_test.go:TestAddCommandTwice)",
		"Stack técnico:",
		"- Go, sin dependencias de terceros — evidencia: go.mod:3",
		"- `go test ./...` corre la suite — evidencia: Makefile:12",
	} {
		if !strings.Contains(state, want) {
			t.Errorf("the state does not carry %q:\n%s", want, state)
		}
	}
	if strings.Contains(state, "Cada comando es un paquete propio bajo cmd/") {
		t.Error("a project-knowledge claim of the portrait reached the state: that half is never a fit gate (§13)")
	}
}

// A missing portrait is a fact about this build, not a reason to hand the judge a section that
// looks empty: the section stays, and says in one named place that there was no portrait.
func TestAMissingPortraitDegradesInOneNamedPlace(t *testing.T) {
	state := State(aRequest(), testProfile, "")
	if strings.Contains(state, "Stack técnico:\n- ") {
		t.Fatalf("a missing portrait still produced stack claims:\n%s", state)
	}
	if got := strings.Count(state, noPortraitMarker); got != 1 {
		t.Fatalf("the missing portrait is named %d times, want exactly 1:\n%s", got, state)
	}
	for _, heading := range headings {
		if !strings.Contains(state, heading) {
			t.Errorf("the missing portrait dropped the %q section:\n%s", heading, state)
		}
	}
}

// A portrait that exists and has no technical section is a different absence, and it is named
// differently: the two are different things to fix.
func TestAPortraitWithoutTheStackSectionSaysSo(t *testing.T) {
	portrait := strings.Replace(testPortrait, "## Stack técnico\n- Go, sin dependencias de terceros — evidencia: go.mod:3\n- `go test ./...` corre la suite — evidencia: Makefile:12\n", "", 1)
	state := State(aRequest(), testProfile, portrait)
	if !strings.Contains(state, noStackMarker) {
		t.Errorf("a portrait without its technical section is not named:\n%s", state)
	}
}

// S7: an empty profile is the state the product starts in, so it has to be visible as empty. An
// omitted section reads as a bug; a named absence reads as "nothing was saved yet".
func TestAnEmptyProfileIsVisiblyEmpty(t *testing.T) {
	state := State(aRequest(), "", testPortrait)
	if !strings.Contains(state, headings[5]) {
		t.Fatalf("the developer section was dropped instead of being shown empty:\n%s", state)
	}
	after, ok := afterHeading(state, headings[5])
	if !ok {
		t.Fatalf("the developer section has no body:\n%s", state)
	}
	if strings.TrimSpace(after) != noProfileMarker {
		t.Errorf("an empty profile reads %q, want %q", strings.TrimSpace(after), noProfileMarker)
	}
}

// A saved profile is what the person approved, so it travels as it is: re-rendering it would show
// the judge a document nobody reviewed.
func TestTheProfileTravelsAsItIs(t *testing.T) {
	if state := whole(); !strings.Contains(state, testProfile) {
		t.Errorf("the profile was not carried verbatim:\n%s", state)
	}
}

// Both sources of a gap travel, in order, because a gap is the other half of what the level says
// (I10).
func TestTheGapsCarryBothSources(t *testing.T) {
	state := whole()
	unknown := strings.Index(state, "- qué debe pasar con migraciones ya aplicadas")
	missing := strings.Index(state, "- la issue no dice qué motores de base están en alcance")
	if unknown < 0 || missing < 0 {
		t.Fatalf("a gap is missing:\n%s", state)
	}
	if missing < unknown {
		t.Error("the gaps are out of order")
	}
}

// Nothing to report is written down, never invented: an empty list under a heading reads as a
// section that forgot to render.
func TestEmptySectionsSayTheyAreEmpty(t *testing.T) {
	req := Request{
		Issue:      Issue{Repository: "spf13__cobra", Number: 13},
		Repository: Repository{},
	}
	state := State(req, "", "")
	for _, heading := range headings {
		if !strings.Contains(state, heading) {
			t.Fatalf("the empty request dropped the %q section:\n%s", heading, state)
		}
	}
	for _, want := range []string{noTitleMarker, noBodyMarker, noneMarker, noProfileMarker, noPortraitMarker} {
		if !strings.Contains(state, want) {
			t.Errorf("the state does not name the absence %q:\n%s", want, state)
		}
	}
}

// An exploration that found no test for this area says exactly that: "no" alone would be a claim
// about the project, and the evidence only supports a claim about this exploration.
func TestNoTestInTheEvidenceIsSaidInWords(t *testing.T) {
	req := aRequest()
	req.Evidence.Tests = nil
	if state := State(req, testProfile, testPortrait); !strings.Contains(state, "Tests: no (not found by the exploration)") {
		t.Errorf("an area with no test does not say so:\n%s", state)
	}
}

// A file the exploration listed without explaining it keeps its line: the reason is missing, not
// the file, and a dangling separator would read as an empty claim.
func TestAFileWithoutAReasonDoesNotDangle(t *testing.T) {
	req := aRequest()
	req.Evidence.Files = []explore.File{{Path: "internal/migrations/runner.go", Relation: explore.RelationObserved}}
	if state := State(req, testProfile, testPortrait); !strings.Contains(state, "- internal/migrations/runner.go [observed]\n") {
		t.Errorf("a file without a reason is not rendered as one:\n%s", state)
	}
}

// The six sections are the whole of the shape: nothing before the first heading and no seventh
// heading invented along the way. The profile brings its own headings, and it brings them because
// it travels verbatim, so those are the only ones allowed past the six.
func TestNothingTravelsOutsideTheSixSections(t *testing.T) {
	state := whole()
	if !strings.HasPrefix(state, headings[0]+"\n") {
		t.Errorf("the state does not open with %q:\n%s", headings[0], state)
	}

	seen := map[string]bool{}
	for _, heading := range headings {
		seen[heading] = true
	}
	for _, line := range strings.Split(state, "\n") {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		if seen[line] || strings.Contains(testProfile, line) {
			continue
		}
		t.Errorf("%q is a section of the state that §9.1 does not have", line)
	}

	at := -1
	for _, heading := range headings {
		index := strings.Index(state, heading+"\n")
		if index < at {
			t.Errorf("%q arrives out of place", heading)
		}
		at = index
	}
}

// afterHeading returns what follows a heading, up to the next one.
func afterHeading(state, heading string) (string, bool) {
	at := strings.Index(state, heading)
	if at < 0 {
		return "", false
	}
	rest := state[at+len(heading):]
	if next := strings.Index(rest, "\n## "); next >= 0 {
		rest = rest[:next]
	}
	return rest, true
}
