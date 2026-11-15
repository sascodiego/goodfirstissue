package web

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/config"
	"github.com/sascodiego/nan-hackathon-clef/internal/explore"
	"github.com/sascodiego/nan-hackathon-clef/internal/fit"
	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// fitProfileDoc is the approved profile the stored verdict was asked against. The screen never
// shows the document: it shows the fingerprint that names it.
const fitProfileDoc = `## Conocimientos técnicos
- Go — nivel: demostrado — evidencia: «tres años de Go»

## Experiencia
- Mantenimiento de una CLI — nivel: demostrado — evidencia: «cobra»
`

// fitAnswers is the whole transport replaced by a recorded answer: the stored verdict is the one
// the production policy reaches from it, and no test touches the network.
type fitAnswers map[string]nan.Answer

func (j fitAnswers) Ask(_ context.Context, _ string, _ map[string]nan.Question) (map[string]nan.Answer, error) {
	return j, nil
}

// fitChoice is one judge's answer to the only question P3 asks.
func fitChoice(answer fit.Fit, confidence float64) fitAnswers {
	return fitAnswers{fit.QFit: {Type: nan.Choice, Choice: string(answer), Confidence: confidence}}
}

// fitDecision is the pin the block reads on its own in the level tests: every field is distinct
// so a block that showed the wrong one cannot pass.
func fitDecision() store.Decision {
	return store.Decision{
		Key: store.Key{
			Issue:         2568,
			Pass:          store.PassFit,
			Model:         config.DecisionModel,
			PromptVersion: fit.Version() + "+0123456789ab",
			CommitSHA:     testCommit,
		},
		At:        time.Date(2026, 11, 14, 7, 5, 0, 0, time.UTC),
		LatencyMs: 10,
	}
}

// fitIdentity is the exploration the fixture handler reads: the explore contract and the agent
// model its ExploreOptions name. It is the identity of the evidence and half of the question.
func fitIdentity() fit.Exploration {
	return fit.Exploration{PromptVersion: explore.Version(), Model: "gemma4"}
}

// seedFitVerdict runs the real third pass over the fixture issue and leaves the verdict in the
// journal, with a pin whose every field can be read back. The exploration is passed so a test can
// store a row that was read against a different exploration than the one the screen reads.
func seedFitVerdict(t *testing.T, st *store.Store, exploration fit.Exploration) {
	t.Helper()

	journal, err := st.Decisions("spf13__cobra")
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	at := time.Date(2026, 11, 14, 7, 5, 0, 0, time.UTC)
	clock := func() time.Time {
		now := at
		at = at.Add(10 * time.Millisecond)
		return now
	}
	p3, err := fit.New(fit.Options{
		Judge:       fitChoice(fit.FitReady, 0.85),
		Journal:     journal,
		Model:       config.DecisionModel,
		CommitSHA:   testCommit,
		Profile:     fitProfileDoc,
		Exploration: exploration,
		Now:         clock,
	})
	if err != nil {
		t.Fatalf("fit.New() error = %v", err)
	}
	if _, err := p3.Run(context.Background(), fit.Request{
		Issue: fit.Issue{
			Repository: "spf13__cobra",
			Number:     2568,
			Title:      "YAML docs ignore linkHandler for see_also entries",
			Body:       "The link handler is ignored.",
		},
		Repository: fit.Repository{Name: "spf13/cobra", PrimaryLanguage: "Go"},
		Evidence:   fitEvidenceRecord(),
	}); err != nil {
		t.Fatalf("p3.Run() error = %v", err)
	}
}

// seedFitExploration stores the exploration the breakdown is read from, with the key the product
// writes, so the block reads the same row the third pass was fed.
func seedFitExploration(t *testing.T, st *store.Store, exploration fit.Exploration) {
	t.Helper()

	record := storedRecord()
	record.Model = exploration.Model
	seedAnalysis(t, st, store.Key{
		Issue:         2568,
		Pass:          store.PassExplore,
		Model:         exploration.Model,
		PromptVersion: exploration.PromptVersion,
		CommitSHA:     testCommit,
	}, record)
}

// fitBlock cuts the fit block out of the page: the same screen carries the triage, the evidence
// and the thread, so asserting on the whole page would let a sentence from another block pass for
// the verdict.
func fitBlock(t *testing.T, page string) string {
	t.Helper()
	return region(t, page, `aria-label="Encaje"`, "</section>")
}

// S11/I4: the block shows the level and the pin that says which question produced it. The prompt
// carries the profile's fingerprint, because another CV is another question (S6).
func TestFitShowsTheLevelWithItsPin(t *testing.T) {
	st := evidenceStore(t)
	seedProfile(t, st, screenCV, fitProfileDoc)
	seedFitExploration(t, st, fitIdentity())
	seedFitVerdict(t, st, fitIdentity())

	block := fitBlock(t, body(t, exploreHandler(t, st, nil, nil), "/issue/spf13__cobra/2568"))

	for _, want := range []string{
		"recomendada", // el nivel, nunca un decimal en su lugar
		`class="fit-level recommended"`,
		"guardado el <b>2026-11-14</b>", // cuándo se produjo
		"10ms",                          // lo que costó
		config.DecisionModel,            // con qué modelo
		fit.Version(),                   // la versión del prompt
		"adbc8813901b",                  // y sobre qué commit
	} {
		if !strings.Contains(block, want) {
			t.Errorf("falta %q en el bloque de encaje:\n%s", want, block)
		}
	}

	// La versión del prompt que el bloque muestra es el contrato más la huella del perfil: otro CV
	// es otra pregunta, así que el pin la lleva (S6).
	journal, err := st.Decisions("spf13__cobra")
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	_, decision, ok := fit.Recall(journal, 2568, config.DecisionModel, testCommit, fitProfileDoc, fitIdentity())
	if !ok {
		t.Fatal("el veredicto guardado no se puede leer con el perfil aprobado")
	}
	if !strings.HasPrefix(decision.PromptVersion, fit.Version()+"+") {
		t.Errorf("la versión del prompt no lleva la huella del perfil: %q", decision.PromptVersion)
	}
	// Y la de la exploración que produjo la evidencia: sin ella la fila no puede decir contra qué
	// se leyó, y el bloque podría emparejarla con otra (I14).
	if got := strings.Count(decision.PromptVersion, "+"); got != 2 {
		t.Errorf("la versión del prompt guardada = %q lleva %d huellas, want la del perfil y la de la exploración",
			decision.PromptVersion, got)
	}

	// §9.3 no pide probabilidad para una recomendación: mostrarla sería el decimal que ocupa el
	// lugar del veredicto (S2).
	if strings.Contains(block, `class="fit-prob"`) {
		t.Errorf("una recomendación muestra una probabilidad que su fila no pide:\n%s", block)
	}
}

// S2: las cinco filas de §9.3 no se leen igual, y la probabilidad acompaña sólo donde la tabla la
// pide: los needs_* y una lectura retenida. Una lectura retenida dice cuál se retuvo.
func TestFitKeepsTheSevenLevelsApart(t *testing.T) {
	cases := []struct {
		name     string
		result   fit.Result
		level    string
		class    string
		prob     string
		hasProb  bool
		withheld string
		why      string
	}{
		{
			name:   "recomendada",
			result: fit.Decide(fitChoice(fit.FitReady, 0.85)),
			level:  "recomendada", class: "recommended",
		},
		{
			name:   "con acompañamiento",
			result: fit.Decide(fitChoice(fit.FitReadyWithSupport, 0.60)),
			level:  "recomendada con acompañamiento", class: "recommended_with_support",
		},
		{
			name:   "necesita preparación",
			result: fit.Decide(fitChoice(fit.FitNeedsPreparation, 0.80)),
			level:  "necesita preparación", class: "needs_preparation",
			prob: "0,80", hasProb: true,
		},
		{
			name:   "necesita experiencia",
			result: fit.Decide(fitChoice(fit.FitNeedsExperience, 0.77)),
			level:  "necesita experiencia", class: "needs_experience",
			prob: "0,77", hasProb: true,
		},
		{
			// La fila que §9.3 no documenta: un ready por debajo de su propio piso no es un
			// veredicto, y la pantalla dice cuál se retuvo.
			name:   "lectura retenida",
			result: fit.Decide(fitChoice(fit.FitReady, 0.55)),
			level:  "sin veredicto", class: "uncertain",
			prob: "0,55", hasProb: true, withheld: "cubre todo",
		},
		{
			name:   "sin comparación",
			result: fit.Decide(fitChoice(fit.FitInsufficientEvidence, 0.90)),
			level:  "información insuficiente", class: "insufficient_information",
		},
		{
			name:   "sin respuesta",
			result: fit.Result{Level: fit.LevelUndecided, Reason: fit.ReasonEndpointFailed},
			level:  "sin respuesta", class: "undecided",
		},
	}

	seenLevel := map[string]string{}
	seenClass := map[string]string{}
	seenWhy := map[string]string{}

	for _, c := range cases {
		view := newFitView(fit.Analysis{Result: c.result}, fitDecision(), nil)

		if view.Level != c.level {
			t.Errorf("%s: nivel = %q, want %q", c.name, view.Level, c.level)
		}
		if view.Class != c.class {
			t.Errorf("%s: clase = %q, want %q", c.name, view.Class, c.class)
		}
		if other, ok := seenLevel[view.Level]; ok {
			t.Errorf("%s y %s muestran el mismo nivel %q", other, c.name, view.Level)
		}
		seenLevel[view.Level] = c.name
		if other, ok := seenClass[view.Class]; ok {
			t.Errorf("%s y %s comparten la clase %q", other, c.name, view.Class)
		}
		seenClass[view.Class] = c.name
		if view.Why == "" {
			t.Errorf("%s: el nivel no se explica", c.name)
		}
		if other, ok := seenWhy[view.Why]; ok {
			t.Errorf("%s y %s se explican igual: %q", other, c.name, view.Why)
		}
		seenWhy[view.Why] = c.name

		if view.HasProbability != c.hasProb {
			t.Errorf("%s: probabilidad mostrada = %v, want %v", c.name, view.HasProbability, c.hasProb)
		}
		if c.hasProb && view.Probability != c.prob {
			t.Errorf("%s: probabilidad = %q, want %q", c.name, view.Probability, c.prob)
		}
		if c.withheld == "" {
			if view.Withheld != "" {
				t.Errorf("%s: nombró una lectura retenida que no existe: %q", c.name, view.Withheld)
			}
			continue
		}
		if !strings.Contains(view.Withheld, c.withheld) {
			t.Errorf("%s: no dice qué lectura se retuvo: %q", c.name, view.Withheld)
		}
		if !strings.Contains(view.Withheld, c.prob) {
			t.Errorf("%s: no dice con qué confianza se retuvo: %q", c.name, view.Withheld)
		}
	}

	// Las dos ausencias se dicen distinto: una request caída no es un perfil que no alcanza.
	doubt := newFitView(fit.Analysis{Result: fit.Result{
		Level: fit.LevelInsufficientInformation, Reason: fit.ReasonInsufficientProfile,
	}}, fitDecision(), nil)
	if !strings.Contains(doubt.Why, "no describe") {
		t.Errorf("la información insuficiente no nombra el perfil: %q", doubt.Why)
	}
	failed := newFitView(fit.Analysis{Result: fit.Result{
		Level: fit.LevelUndecided, Reason: fit.ReasonEndpointFailed,
	}}, fitDecision(), nil)
	if !strings.Contains(failed.Why, "Clef no contestó") {
		t.Errorf("sin respuesta no dice que Clef no contestó: %q", failed.Why)
	}
	if failed.Probability != "" {
		t.Errorf("sin respuesta muestra una probabilidad inventada: %q", failed.Probability)
	}
}

// S3/S10: el desglose muestra lo que pesó —los requisitos evaluables, con la evidencia que los
// respalda— y las lagunas. El proyecto y el proceso aparecen como costo de onboarding, y se dice
// que no pesaron: nunca un portón.
func TestFitBreakdownShowsWhatWeighedAndWhatIsOnlyOnboarding(t *testing.T) {
	st := evidenceStore(t)
	seedProfile(t, st, screenCV, fitProfileDoc)
	seedFitExploration(t, st, fitIdentity())
	seedFitVerdict(t, st, fitIdentity())

	block := fitBlock(t, body(t, exploreHandler(t, st, nil, nil), "/issue/spf13__cobra/2568"))

	for _, want := range []string{
		`class="fit-reqs"`,          // lo evaluable, en su propia lista
		`class="fit-onboarding"`,    // el costo de onboarding, en la suya
		"Go templates",              // un requisito evaluable
		"doc/yaml_docs.go:44",       // la evidencia que lo respalda
		`class="rel observed"`,      // con la relación que puede afirmar
		"whether the queue matters", // una laguna de la exploración
		"acceptance criteria",       // y lo que la issue no dice
		`class="evid-files-wrap"`,   // la tabla se corre en horizontal a 360 px
		"nunca es un portón",        // se dice que el onboarding no juzgó
	} {
		if !strings.Contains(block, want) {
			t.Errorf("falta %q en el desglose:\n%s", want, block)
		}
	}

	// Los dos baldes de contexto no se cuelan en la lista que pesó.
	evaluable := region(t, block, `class="fit-reqs"`, "</ul>")
	for _, onboarding := range []string{"doc package", "PR with tests"} {
		if strings.Contains(evaluable, onboarding) {
			t.Errorf("%q se presenta como requisito evaluable:\n%s", onboarding, evaluable)
		}
	}
	if !strings.Contains(evaluable, "Go templates") {
		t.Errorf("el requisito evaluable no está en la lista que pesó:\n%s", evaluable)
	}

	onboarding := region(t, block, `class="fit-onboarding"`, "</ul>")
	for _, want := range []string{"doc package", "PR with tests"} {
		if !strings.Contains(onboarding, want) {
			t.Errorf("%q no aparece como costo de onboarding:\n%s", want, onboarding)
		}
	}
	if !strings.Contains(block, "no pesó") && !strings.Contains(block, "no pesaron") {
		t.Errorf("el bloque no dice que el onboarding no pesó:\n%s", block)
	}
}

// S7: sin perfil aprobado no hay contra qué comparar. El bloque lo dice y manda a la pantalla que
// produce uno, en vez de ofrecer un POST /fit que no existe.
func TestFitWithoutAProfileSaysWhatIsMissing(t *testing.T) {
	block := fitBlock(t, body(t, newHandler(t, evidenceStore(t)), "/issue/spf13__cobra/2568"))

	if !strings.Contains(block, "no hay un perfil aprobado") {
		t.Errorf("el bloque no dice que falta el perfil:\n%s", block)
	}
	if !strings.Contains(block, `href="/profile"`) {
		t.Errorf("el bloque no manda a la pantalla del perfil:\n%s", block)
	}
	if strings.Contains(block, `class="fit-level"`) {
		t.Errorf("sin perfil el bloque muestra un nivel:\n%s", block)
	}
	assertNoFitButton(t, block)
}

// S11: con perfil y sin veredicto, el bloque nombra el comando que lo produce. El nivel no se
// inventa: se dice que no hay.
func TestFitWithoutAVerdictNamesTheCommand(t *testing.T) {
	st := evidenceStore(t)
	seedProfile(t, st, screenCV, fitProfileDoc)

	block := fitBlock(t, body(t, exploreHandler(t, st, nil, nil), "/issue/spf13__cobra/2568"))

	if !strings.Contains(block, "Todavía no hay encaje guardado") {
		t.Errorf("el bloque no dice que no hay veredicto:\n%s", block)
	}
	if !strings.Contains(block, "gfi fit spf13/cobra") {
		t.Errorf("el bloque no nombra el comando que produce el veredicto:\n%s", block)
	}
	if strings.Contains(block, `class="fit-level"`) {
		t.Errorf("sin veredicto el bloque muestra un nivel:\n%s", block)
	}
	assertNoFitButton(t, block)
}

// S11: el nivel está guardado con su pin, pero el desglose se lee de la exploración que lo
// alimentó. Si esa exploración ya no está guardada, el veredicto queda y la ausencia se dice.
func TestFitSaysWhenTheBreakdownIsNoLongerStored(t *testing.T) {
	st := evidenceStore(t)
	seedProfile(t, st, screenCV, fitProfileDoc)
	seedFitVerdict(t, st, fitIdentity())

	block := fitBlock(t, body(t, exploreHandler(t, st, nil, nil), "/issue/spf13__cobra/2568"))

	if !strings.Contains(block, `class="fit-level recommended"`) {
		t.Errorf("el veredicto guardado no se muestra:\n%s", block)
	}
	if !strings.Contains(block, "ya no está guardado") {
		t.Errorf("el bloque no dice que el desglose ya no está guardado:\n%s", block)
	}
	if strings.Contains(block, `class="fit-reqs"`) {
		t.Errorf("el bloque muestra un desglose que no tiene:\n%s", block)
	}
}

// I14: el veredicto se leyó contra una exploración, y la pantalla no lo empareja con otra. Una
// fila escrita bajo otra identidad de explore no se lee: el bloque dice que no hay, en vez de
// mostrar el nivel junto a una evidencia que el juez nunca vio.
func TestFitDoesNotPairAVerdictWithAnotherExploration(t *testing.T) {
	st := evidenceStore(t)
	seedProfile(t, st, screenCV, fitProfileDoc)
	// La pantalla lee con gemma4; la fila se escribió cuando el corpus se exploró con otro agente.
	seedFitExploration(t, st, fitIdentity())
	seedFitVerdict(t, st, fit.Exploration{PromptVersion: explore.Version(), Model: "glm5.3-flash"})

	block := fitBlock(t, body(t, exploreHandler(t, st, nil, nil), "/issue/spf13__cobra/2568"))

	if strings.Contains(block, `class="fit-level"`) {
		t.Errorf("el bloque muestra un nivel que la evidencia de esta pantalla no respalda:\n%s", block)
	}
	if strings.Contains(block, "Guardado el") || strings.Contains(block, "guardado el <b>") {
		t.Errorf("el bloque muestra el pin de un veredicto de otra exploración:\n%s", block)
	}
	if !strings.Contains(block, "Todavía no hay encaje guardado") {
		t.Errorf("el bloque no dice que no puede mostrar el veredicto:\n%s", block)
	}
	if !strings.Contains(block, "gfi fit spf13/cobra") {
		t.Errorf("el bloque no nombra el comando que produce uno verificable:\n%s", block)
	}
	if strings.Contains(block, `class="fit-reqs"`) {
		t.Errorf("el bloque muestra un desglose que no respalda al veredicto:\n%s", block)
	}
}

// Sin modelo de explore configurado este servidor no puede nombrar la exploración que la fila
// responde, así que no la lee: se reporta en el log y el bloque queda vacío en vez de inventar la
// identidad (I14).
func TestFitCannotNameTheQuestionWithoutAnExploreModel(t *testing.T) {
	st := evidenceStore(t)
	seedProfile(t, st, screenCV, fitProfileDoc)
	seedFitExploration(t, st, fitIdentity())
	seedFitVerdict(t, st, fitIdentity())

	block := fitBlock(t, body(t, newHandler(t, st), "/issue/spf13__cobra/2568"))

	if strings.Contains(block, `class="fit-level"`) {
		t.Errorf("el bloque mostró un nivel sin poder nombrar la exploración que lo respalda:\n%s", block)
	}
	if !strings.Contains(block, "Todavía no hay encaje guardado") {
		t.Errorf("el bloque no dice que no puede mostrar el veredicto:\n%s", block)
	}
	assertNoFitButton(t, block)
}

// assertNoFitButton pins the absence of a button: there is no POST /fit, so offering one would
// promise a route the product does not serve.
func assertNoFitButton(t *testing.T, block string) {
	t.Helper()
	if strings.Contains(block, "<form") || strings.Contains(block, "action=") {
		t.Errorf("el bloque de encaje ofrece un POST /fit que no existe:\n%s", block)
	}
}

// fitEvidenceRecord is the exploration the seeded verdict was asked about, reused from the
// evidence fixtures so the two blocks cannot disagree about what the code says.
func fitEvidenceRecord() explore.Evidence {
	return *storedRecord().Evidence
}
