package projectbrief

import (
	"strings"
	"testing"
)

// validAnswer is a portrait that follows the contract: the two declaration lines, then the six
// sections in order, each one either citing evidence or saying it cannot.
const validAnswer = "TÉCNICO: Stack técnico\n" +
	"PROYECTO: Qué es, Arquitectura y componentes, Convenciones y cómo se construye, " +
	"Dirección: qué acepta y qué no, Límites de lo observable\n" +
	"\n" +
	"## Qué es\n" +
	"- Una librería de CLI en Go — evidencia: README.md:3\n" +
	"- Nace de un fork de cobra — evidencia: docs/history.md:12\n" +
	"\n" +
	"## Stack técnico\n" +
	"- Go 1.22, sin dependencias de terceros — evidencia: go.mod:3\n" +
	"- `go test ./...` es el comando de test — evidencia: Makefile:12\n" +
	"\n" +
	"## Arquitectura y componentes\n" +
	"- Un paquete por comando — evidencia: cmd/root.go:10\n" +
	"\n" +
	"## Convenciones y cómo se construye\n" +
	"- Los tests viven al lado del código — evidencia: CONTRIBUTING.md:40\n" +
	"\n" +
	"## Dirección: qué acepta y qué no\n" +
	"- Acepta subcomandos nuevos — evidencia: MAINTAINERS.md:8\n" +
	"\n" +
	"## Límites de lo observable\n" +
	"insufficient_information\n"

func TestParseReadsTheFixedSkeleton(t *testing.T) {
	doc, err := Parse(validAnswer)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.Text != validAnswer {
		t.Error("the answer must travel verbatim: it is the text the alignment node reads")
	}
	if len(doc.Items) != 7 {
		t.Fatalf("items = %d, want 7", len(doc.Items))
	}
	first := doc.Items[0]
	if first.Section != SectionWhat || first.Claim != "Una librería de CLI en Go" ||
		first.Evidence.Path != "README.md" || first.Evidence.Line != 3 {
		t.Errorf("first item = %+v", first)
	}
	// The claim may itself carry the field separator: only the evidence field is a field.
	if got := doc.Items[1].Claim; got != "Nace de un fork de cobra" {
		t.Errorf("claim = %q", got)
	}
	if got := doc.Items[3].Claim; got != "`go test ./...` es el comando de test" {
		t.Errorf("claim = %q", got)
	}
	if got, want := doc.Unsupported, []string{SectionLimits}; len(got) != 1 || got[0] != want[0] {
		t.Errorf("unsupported = %v, want %v", got, want)
	}
	if doc.Insufficient() {
		t.Error("one doubtful section is not a doubtful portrait")
	}
}

// S5: the one thing no consumer may get wrong is which half of the portrait crosses with the CV.
func TestSectionsDeclareOneNatureEach(t *testing.T) {
	technical := map[string]bool{}
	project := map[string]bool{}
	for _, section := range Sections {
		switch section.Nature {
		case NatureTechnical:
			technical[section.Name] = true
		case NatureProject:
			project[section.Name] = true
		default:
			t.Fatalf("section %q has nature %q", section.Name, section.Nature)
		}
	}
	if len(technical) != 1 || !technical[SectionStack] {
		t.Errorf("technical sections = %v, want only %q", technical, SectionStack)
	}
	if len(technical)+len(project) != len(Sections) {
		t.Errorf("a section carries two natures or none")
	}
	if project[SectionStack] || !project[SectionDirection] {
		t.Errorf("project sections = %v", project)
	}
}

func TestParseRejectsADeclarationThatContradictsTheCode(t *testing.T) {
	answer := strings.Replace(validAnswer,
		"TÉCNICO: Stack técnico",
		"TÉCNICO: Stack técnico, Dirección: qué acepta y qué no", 1)
	if _, err := Parse(answer); err == nil {
		t.Fatal("a declaration that claims a project section is technical must not parse")
	}
}

func TestParseRejectsAnUnknownSection(t *testing.T) {
	answer := strings.Replace(validAnswer, "## Límites de lo observable", "## Riesgos", 1)
	_, err := Parse(answer)
	if err == nil {
		t.Fatal("an improvised heading has nowhere to go")
	}
	if !strings.Contains(err.Error(), "Riesgos") {
		t.Errorf("the error must name the heading: %v", err)
	}
}

func TestParseRejectsAMissingSection(t *testing.T) {
	answer := strings.Replace(validAnswer,
		"## Límites de lo observable\ninsufficient_information\n", "", 1)
	if _, err := Parse(answer); err == nil {
		t.Fatal("a portrait without its last section must not parse")
	}
}

func TestParseRejectsASectionOutOfOrder(t *testing.T) {
	// Same sections, different order: the skeleton is fixed, so the order is part of it.
	answer := "TÉCNICO: Stack técnico\n" +
		"PROYECTO: Qué es, Arquitectura y componentes, Convenciones y cómo se construye, " +
		"Dirección: qué acepta y qué no, Límites de lo observable\n\n" +
		"## Stack técnico\n- Go — evidencia: go.mod:1\n" +
		"## Qué es\n- Una CLI — evidencia: README.md:1\n" +
		"## Arquitectura y componentes\n- Un paquete — evidencia: cmd/root.go:1\n" +
		"## Convenciones y cómo se construye\n- Al lado — evidencia: CONTRIBUTING.md:1\n" +
		"## Dirección: qué acepta y qué no\n- Acepta — evidencia: MAINTAINERS.md:1\n" +
		"## Límites de lo observable\ninsufficient_information\n"
	if _, err := Parse(answer); err == nil {
		t.Fatal("the sections must arrive in the fixed order")
	}
}

func TestParseRejectsProse(t *testing.T) {
	answer := strings.Replace(validAnswer, "- Un paquete por comando — evidencia: cmd/root.go:10",
		"El proyecto separa cada comando en su paquete.", 1)
	if _, err := Parse(answer); err == nil {
		t.Fatal("a paragraph has nowhere to go")
	}
}

func TestParseRejectsAnItemWithoutEvidence(t *testing.T) {
	answer := strings.Replace(validAnswer, " — evidencia: cmd/root.go:10", "", 1)
	if _, err := Parse(answer); err == nil {
		t.Fatal("without evidence it is the model's opinion, not a portrait")
	}
}

func TestParseRejectsEvidenceThatIsNotPathAndLine(t *testing.T) {
	for _, evidence := range []string{"cmd/root.go", "cmd/root.go:", "cmd/root.go:cero", "cmd/root.go:0",
		"cmd/root.go:-2", "/etc/passwd:1", "../secreto.go:4", "cmd/../../fuera.go:2", ""} {
		answer := strings.Replace(validAnswer, "evidencia: cmd/root.go:10", "evidencia: "+evidence, 1)
		if _, err := Parse(answer); err == nil {
			t.Errorf("evidence %q must not parse", evidence)
		}
	}
}

func TestParseRejectsASectionThatNeitherCitesNorDoubts(t *testing.T) {
	answer := strings.Replace(validAnswer,
		"## Límites de lo observable\ninsufficient_information\n",
		"## Límites de lo observable\n", 1)
	if _, err := Parse(answer); err == nil {
		t.Fatal("a silent section is indistinguishable from a bug (I10)")
	}
}

func TestParseAcceptsADoubtfulPortrait(t *testing.T) {
	doc, err := Parse(Insufficient)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !doc.Insufficient() {
		t.Error("insufficient_information for the whole answer means every section is doubtful")
	}
	if len(doc.Unsupported) != len(Sections) {
		t.Errorf("unsupported = %v, want all %d sections", doc.Unsupported, len(Sections))
	}
	if len(doc.Items) != 0 {
		t.Errorf("a doubtful portrait cites nothing, got %d items", len(doc.Items))
	}
}

func TestParseRejectsAnEmptyAnswer(t *testing.T) {
	if _, err := Parse("   \n\n"); err == nil {
		t.Fatal("an empty answer is an error, not a portrait")
	}
}

// The empty-root judge that reads this document must be able to ask which items cross with the
// CV without re-reading prose.
func TestDocumentGroupsItemsBySection(t *testing.T) {
	doc, err := Parse(validAnswer)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := len(doc.ItemsIn(SectionStack)); got != 2 {
		t.Errorf("items in %q = %d, want 2", SectionStack, got)
	}
	for _, item := range doc.ItemsIn(SectionStack) {
		if NatureOf(item.Section) != NatureTechnical {
			t.Errorf("%q cited an item of the wrong nature", item.Section)
		}
	}
	if got := len(doc.ItemsIn(SectionLimits)); got != 0 {
		t.Errorf("a doubtful section cites nothing, got %d", got)
	}
}
