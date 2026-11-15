// Package projectbrief turns one cloned repository into the portrait the alignment node reads.
//
// The alignment node already consumes a project brief (§7.5) and today nothing produces one for
// a real repository: the benchmark feeds four hand-written briefs, so the node is measurable and
// not runnable. This package closes that hole with the same shape internal/profile uses — one
// model run, a fixed skeleton, prose instead of JSON, the answer verbatim — because the consumer
// is Clef and what it receives is a string (PROYECTO.md §17.8).
//
// The artifact carries one distinction that decides whether the product keeps its promise. What a
// project *can* do — languages, frameworks, toolchain, the commands that build and test it — is
// technical knowledge, and it is the only half that may cross with the CV. What a project *wants
// to be*, what it accepts, how it is built there — project knowledge, which reaches the
// onboarding cost and the alignment node and is never a gate (§1.2, §13). Every section carries
// exactly one nature and the document declares the split, so no consumer can mix them by reading
// a heading. Section is where that constant lives.
package projectbrief

// Nature says which of the two buckets a section feeds. It is not a rendering hint: it is the
// boundary the product promises (§2, §13).
type Nature string

const (
	// NatureTechnical is knowledge of a technology, which says whether a person *can* do the
	// work. It crosses with the CV, in the balde called knowledge.
	NatureTechnical Nature = "tecnico"
	// NatureProject is knowledge of this system, which says what it costs to join. It is never
	// a fit gate: knowing Go is technical capacity, knowing where this project keeps its
	// padding is onboarding time.
	NatureProject Nature = "proyecto"
)

// Section headings. They are fixed vocabulary, not content: this is the skeleton the issue fixes
// and nothing else may be added, because every consumer branches on it.
const (
	SectionWhat         = "Qué es"
	SectionStack        = "Stack técnico"
	SectionArchitecture = "Arquitectura y componentes"
	SectionConventions  = "Convenciones y cómo se construye"
	SectionDirection    = "Dirección: qué acepta y qué no"
	SectionLimits       = "Límites de lo observable"
)

// Section is one heading and the bucket it feeds.
type Section struct {
	Name   string
	Nature Nature
}

// Sections is the complete skeleton, in the order the document must present it.
//
// SectionStack is the only technical one, and it is deliberately the only one: the issue's
// parenthetical for SectionConventions names build, test and lint, and those *commands* are
// technical — but what the section describes is where things live and how the project works,
// which is onboarding. Splitting a section by sentence would put two natures in one heading,
// which is exactly what the split exists to prevent.
var Sections = []Section{
	{SectionWhat, NatureProject},
	{SectionStack, NatureTechnical},
	{SectionArchitecture, NatureProject},
	{SectionConventions, NatureProject},
	{SectionDirection, NatureProject},
	{SectionLimits, NatureProject},
}

// Insufficient is the word a section answers with when the repository cannot support it. It is a
// word and not an empty section because an empty section cannot be told apart from a bug (I10).
const Insufficient = "insufficient_information"

// The two lines that open the document and declare the split. Parse validates them against
// Sections instead of trusting them: the model writes the declaration, the code owns it.
const (
	technicalKey = "TÉCNICO:"
	projectKey   = "PROYECTO:"
)

// NatureOf reports the bucket of a section name. The empty string for an unknown one, which is
// also how callers tell a known name from an invented one.
func NatureOf(name string) Nature {
	for _, section := range Sections {
		if section.Name == name {
			return section.Nature
		}
	}
	return ""
}
