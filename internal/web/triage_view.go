package web

// El informe de triaje de la pantalla de issue.
//
// No decide nada: muestra lo que P1 ya decidió sobre esta issue y con qué preguntas. La
// lectura del journal usa la misma clave que lo escribió (I4, I14), y una probabilidad que no
// existe se muestra como ausencia —nunca un veredicto por defecto (I2)—.

import (
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/config"
	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
	"github.com/sascodiego/nan-hackathon-clef/internal/triage"
)

// declaredKindCut es el corte medido de §7.5: el mismo 0,6 del nodo de alineación, nombrado
// una sola vez acá. Por debajo, la lectura del kind no está decidida y la declaración del
// autor no se presenta como contradicción: sería afirmar más de lo que se midió.
const declaredKindCut = 0.6

// missingProbability es lo que se muestra cuando no hay probabilidad. Un cero se leería como
// un valor medido, y no lo es.
const missingProbability = "—"

// triageView es el bloque completo: el veredicto con su probabilidad, las preguntas que se
// hicieron con su respuesta, la declaración del autor, sus etiquetas verbatim y el pin que
// dice cuándo y bajo qué pregunta se produjo.
type triageView struct {
	Outcome string // la palabra del veredicto, tal como la pantalla la dice
	Reason  string // el motivo del cable, cuando el veredicto necesita uno
	Class   string // la forma del veredicto: cada modo de no recomendar se pinta distinto
	// Probability es la probabilidad de la respuesta que decidió, con coma decimal. Es «—»
	// cuando no hubo respuesta: una request caída no tiene probabilidad que mostrar.
	Probability string
	Why         string

	// Missing son las preguntas que faltan para salir de la duda, con el texto de la
	// pantalla: el id del cable no se le muestra a nadie.
	Missing []string

	Questions []triageQuestion

	// Contradiction se llena sólo cuando la lectura está decidida: por debajo del corte, el
	// desacuerdo con el autor no se presenta como contradicción.
	Contradiction  string
	Kind           string // el kind leído, en castellano
	Declared       string // el kind que el autor declaró en el título, en castellano
	HasKind        bool
	HasDeclaration bool

	// Labels son las etiquetas originales, verbatim y en el orden del tracker: dato del
	// mantenedor, jamás insumo de la decisión (I6).
	Labels    []string
	HasLabels bool

	Analyzed string
	Latency  string
	Model    string
	Prompt   string
	Commit   string
}

// triageQuestion is one question that was asked, with the answer it received and its value.
// Answer is the chosen category for a choice answer and stays empty for a yes/no one, whose
// only value is its probability.
type triageQuestion struct {
	ID      string
	Text    string
	Answer  string
	Value   string
	Decided bool
}

// attachTriage busca el veredicto guardado de esta issue y lo deja listo para la plantilla.
// Sin store, sin corpus o sin veredicto no hace nada: el bloque dice que no hay, que es la
// verdad, en vez de mostrar un veredicto vacío.
func (h *Handler) attachTriage(page *issuePage) {
	if h.store == nil || page.CommitSHA == "" || page.Active.Key == "" {
		return
	}

	decisions, err := h.store.Decisions(page.Active.Key)
	if err != nil {
		slog.Error("leer el journal de la issue", "key", page.Active.Key, "err", err)
		return
	}

	// El modelo de decisión lo fija la constitución, no config.json (I13).
	analysis, decision, ok := triage.Recall(decisions, page.Issue.Number, config.DecisionModel, page.CommitSHA)
	if !ok {
		return
	}

	page.Triage = newTriageView(analysis, decision, page.Issue)
	page.HasTriage = true
}

// kindConfidence es la confianza de la respuesta que dijo qué es la issue. Es el número que la
// contradicción tiene que citar, porque es el mismo que la fila del kind muestra arriba: en una
// feature, Result.Probability es la del nodo de alineación y mide otra cosa, así que usarla pondría
// dos números distintos sobre el mismo juicio.
func kindConfidence(answers map[string]nan.Answer) (float64, bool) {
	a, ok := answers[triage.QKind]
	if !ok || a.Type != nan.Choice {
		return 0, false
	}
	return a.Confidence, true
}

// newTriageView projects a stored verdict into what the screen can honestly say.
func newTriageView(analysis triage.Analysis, decision store.Decision, issue issueView) triageView {
	result := analysis.Result
	view := triageView{
		Outcome:     string(result.Outcome),
		Reason:      result.Reason,
		Class:       outcomeClass(result.Outcome),
		Probability: probabilityText(result.Probability),
		Why:         whyText(result),
		Missing:     missingText(result.Missing),
		Analyzed:    formatDate(decision.At),
		Latency:     latencyText(time.Duration(decision.LatencyMs) * time.Millisecond),
		Model:       decision.Model,
		Prompt:      decision.PromptVersion,
		Commit:      shortSHA(decision.CommitSHA),
	}

	if result.Outcome == triage.Undecided {
		// Sin veredicto no hay probabilidad: la request falló o la respuesta no se puede
		// leer. Inventar una sería el veredicto por defecto que I2 prohíbe.
		view.Probability = missingProbability
	}

	view.Questions = questionRows(analysis.Answers, analysis.Alignment, decidingQuestion(analysis))

	declared := ""
	if _, d := triage.NormalizeTitle(issue.Title); d != "" {
		declared = kindText(d)
	}
	view.HasDeclaration = declared != ""
	view.Declared = declared
	if result.Kind != "" {
		view.HasKind = true
		view.Kind = kindText(result.Kind)
	}
	if view.HasDeclaration && view.HasKind && declared != view.Kind {
		if confidence, ok := kindConfidence(analysis.Answers); ok && confidence >= declaredKindCut {
			view.Contradiction = fmt.Sprintf(
				"El autor también puede mentir: el título declara «%s» y el triaje leyó «%s» con %s.",
				declared, view.Kind, probabilityText(confidence))
		}
	}

	for _, label := range issue.Labels {
		view.Labels = append(view.Labels, label.Text)
	}
	view.HasLabels = len(view.Labels) > 0

	return view
}

// outcomeClass gives each verdict its own shape. The four ways of not recommending have to be
// told apart before the sentence is read, so they never share a class.
func outcomeClass(outcome triage.Outcome) string {
	switch outcome {
	case triage.Candidate:
		return "candidate"
	case triage.Conditional:
		return "conditional"
	case triage.Marked:
		return "marked"
	case triage.Doubt:
		return "doubt"
	case triage.Derived:
		return "derived"
	case triage.Undecided:
		return "undecided"
	}
	return "unknown"
}

// whyText dice en una frase qué significa el veredicto. Cada motivo conserva la suya: dos
// razones distintas que se leen igual son una razón sola.
func whyText(result triage.Result) string {
	switch result.Outcome {
	case triage.Candidate:
		return "Vale la pena explorarla: hay trabajo concreto que una primera contribución puede hacer."
	case triage.Doubt:
		if result.Reason == triage.ReasonInsufficientEvidence {
			return "El texto no alcanza para decidir qué se está pidiendo. No se descarta: se duda en voz alta y se dice qué falta (I10)."
		}
	case triage.Derived:
		if result.Reason == triage.ReasonSecondRoundPending {
			return "Es una consulta, y la ronda que la convierte en trabajo todavía no está instalada: queda derivada, ni recomendada ni descartada."
		}
	case triage.Conditional:
		switch result.Reason {
		case triage.ReasonAlignmentUnavailable:
			return "Es una funcionalidad y no se pudo leer hacia dónde va el proyecto: la condición es preguntarle al mantenedor antes de escribir código."
		case triage.ReasonAskMaintainer:
			return "La posición del proyecto sobre esta funcionalidad no está saldada: hay que preguntarle al mantenedor antes de escribir código."
		}
	case triage.Marked:
		switch result.Reason {
		case triage.ReasonAlreadyDone:
			return "El proyecto cerró la issue como completada: el trabajo ya está hecho y no queda nada para explorar."
		case triage.ReasonDeclined:
			return "El proyecto cerró la issue como no planeada: ya dijo que no, y volver a preguntar no es triaje."
		case triage.ReasonAlreadyWorking:
			return "Hay un pull request abierto sobre esta issue: alguien ya está trabajando en esto y recomendarla sería duplicar el trabajo."
		case triage.ReasonClosed:
			return "La issue está cerrada: quedó fuera de alcance, cualquiera haya sido el motivo del cierre."
		case triage.ReasonNoContent:
			return "La issue no tiene cuerpo: no hay ningún pedido que juzgar, así que no hay nada que recomendar."
		case triage.ReasonNotCodeWork:
			return "Lo que pide no es trabajo de código ni trae información usable: no hay nada para explorar."
		case triage.ReasonServesOnlyRequester:
			return "El pedido responde a la situación de quien lo abre: al resto de los usuarios del proyecto no le aporta nada."
		}
	case triage.Undecided:
		switch result.Reason {
		case triage.ReasonEndpointFailed:
			return "Clef no contestó, así que no hay veredicto ni probabilidad que mostrar: se puede volver a correr. Una request caída no es un juicio sobre la issue (I2)."
		case triage.ReasonIncompleteAnswer:
			return "La respuesta no trae una pregunta que la cadena necesita: no se puede leer, y sin lectura no hay probabilidad que mostrar."
		}
	}
	return "El veredicto está guardado, pero su motivo no está entre los que esta pantalla sabe explicar."
}

// probabilityText formatea una probabilidad como se lee en esta pantalla: dos decimales con
// coma, que es el separador decimal en castellano.
func probabilityText(p float64) string {
	return strings.Replace(strconv.FormatFloat(p, 'f', 2, 64), ".", ",", 1)
}

// questionTexts es el texto de pantalla de cada pregunta congelada. El id es el contrato del
// cable; esto es lo único que traduce.
var questionTexts = map[string]string{
	triage.QKind:               "Qué pide la issue",
	triage.QSelfContained:      "Se explica sola",
	triage.QHasEvidence:        "Trae la evidencia para reproducirla",
	triage.QUnambiguous:        "Tiene una sola lectura",
	triage.QDoneDefined:        "Define cuándo está terminada",
	triage.QReportsDefect:      "Reporta un defecto que ya existe",
	triage.QEvidenceSufficient: "El texto alcanza para decidir qué es",
	triage.QOnlyForRequester:   "Sólo le sirve a quien pregunta",
	triage.QFitsDirection:      "Va en la dirección del proyecto",
}

func questionText(id string) string {
	if text, ok := questionTexts[id]; ok {
		return text
	}
	return id
}

// questionOrder es el orden congelado: las siete de substancia primero y las dos de
// alineación después, cuando el nodo corrió. Nunca un número fijo en pantalla.
var questionOrder = []string{
	triage.QKind,
	triage.QSelfContained,
	triage.QHasEvidence,
	triage.QUnambiguous,
	triage.QDoneDefined,
	triage.QReportsDefect,
	triage.QEvidenceSufficient,
	triage.QOnlyForRequester,
	triage.QFitsDirection,
}

// questionRows builds one row per answered question. Answers is the source and the order is
// the frozen one: nobody sees a question that was not asked.
func questionRows(answers, alignment map[string]nan.Answer, decided string) []triageQuestion {
	rows := make([]triageQuestion, 0, len(answers)+len(alignment))
	seen := make(map[string]bool, len(answers)+len(alignment))

	add := func(id string, a nan.Answer) {
		seen[id] = true
		row := triageQuestion{ID: id, Text: questionText(id), Decided: id == decided}
		switch a.Type {
		case nan.Choice:
			if id == triage.QKind {
				row.Answer = kindText(a.Choice)
			} else {
				row.Answer = a.Choice
			}
			row.Value = probabilityText(a.Confidence)
		case nan.Noul:
			row.Value = probabilityText(a.Noul)
		default:
			// La cadena sólo pregunta noul y choice. Una forma que no se sabe leer se dice
			// ausente en vez de mostrar un cero que nadie midió.
			row.Value = missingProbability
		}
		rows = append(rows, row)
	}

	for _, id := range questionOrder {
		if a, ok := answers[id]; ok {
			add(id, a)
			continue
		}
		if a, ok := alignment[id]; ok {
			add(id, a)
		}
	}

	// Una respuesta que el orden congelado no nombra se muestra igual, después del resto: se
	// preguntó, y esconderla sería tapar lo que el juez dijo.
	for _, source := range []map[string]nan.Answer{answers, alignment} {
		extra := make([]string, 0, len(source))
		for id := range source {
			if !seen[id] {
				extra = append(extra, id)
			}
		}
		sort.Strings(extra)
		for _, id := range extra {
			add(id, source[id])
		}
	}

	return rows
}

// missingText traduce los ids que la política nombró como faltantes al texto de la pantalla,
// en el orden en que la política los nombró.
func missingText(missing []string) []string {
	if len(missing) == 0 {
		return nil
	}
	out := make([]string, 0, len(missing))
	for _, id := range missing {
		out = append(out, questionText(id))
	}
	return out
}

// kindText nombra las categorías del kind como las lee una persona. Los ids son del cable y no
// se traducen en ningún otro lugar.
func kindText(kind string) string {
	switch kind {
	case triage.KindBug:
		return "defecto"
	case triage.KindFeature:
		return "funcionalidad"
	case triage.KindQuestion:
		return "consulta"
	case triage.KindSpam:
		return "spam"
	case triage.KindOther:
		return "otro trabajo"
	}
	return kind
}

// decidingQuestion says which answer produced the verdict, following the chain in policy.Decide:
// the pre-gates resolve without a question, the sufficiency check decides a doubt, the kind
// decides everything up to a feature, and a feature is decided by the alignment node. The
// other questions are measured and never applied, so marking the deciding row is what keeps a
// measurement from reading as a gate.
func decidingQuestion(analysis triage.Analysis) string {
	result := analysis.Result
	if result.Outcome == triage.Undecided || len(analysis.Answers) == 0 {
		return ""
	}
	switch result.Reason {
	case triage.ReasonServesOnlyRequester:
		return triage.QOnlyForRequester
	case triage.ReasonInsufficientEvidence:
		return triage.QEvidenceSufficient
	}
	if result.Kind == triage.KindFeature && len(analysis.Alignment) > 0 {
		if _, ok := analysis.Alignment[triage.QFitsDirection]; ok {
			return triage.QFitsDirection
		}
	}
	return triage.QKind
}
