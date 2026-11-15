package web

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/config"
	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
	"github.com/sascodiego/nan-hackathon-clef/internal/triage"
)

// triageDecision is the pin the block reads. A verdict without its pin reads as fresh (I4),
// so the fixture carries a distinct value in every field.
func triageDecision() store.Decision {
	return store.Decision{
		Key: store.Key{
			Issue:         2568,
			Pass:          store.PassTriage,
			Model:         config.DecisionModel,
			PromptVersion: "3f2a9c1b7d4e",
			CommitSHA:     testCommit,
		},
		At:        time.Date(2026, 11, 14, 7, 5, 0, 0, time.UTC),
		LatencyMs: 1234,
	}
}

// substanceAnswers is one judge's answer to the frozen seven, with the kind read and its
// confidence. The values are fixed so no test ever needs a request.
func substanceAnswers(kind string, confidence float64) map[string]nan.Answer {
	return map[string]nan.Answer{
		triage.QKind:               {Type: nan.Choice, Choice: kind, Confidence: confidence},
		triage.QSelfContained:      {Type: nan.Noul, Noul: 0.91},
		triage.QHasEvidence:        {Type: nan.Noul, Noul: 0.74},
		triage.QUnambiguous:        {Type: nan.Noul, Noul: 0.66},
		triage.QDoneDefined:        {Type: nan.Noul, Noul: 0.55},
		triage.QReportsDefect:      {Type: nan.Noul, Noul: 0.88},
		triage.QEvidenceSufficient: {Type: nan.Noul, Noul: 0.93},
	}
}

// triagedIssue is the issue the block reads: its title carries the author's declaration and
// its labels are the maintainer's data, verbatim.
func triagedIssue(title string, labels ...string) issueView {
	return newIssueView(ghdata.Issue{Number: 2568, Title: title, Body: "the link handler is ignored", Labels: labels})
}

// bugAnalysis is a bug the chain resolved on its own: seven answered questions, no alignment.
func bugAnalysis() triage.Analysis {
	answers := substanceAnswers(triage.KindBug, 0.86)
	issue := triage.Issue{Repository: "spf13__cobra", Number: 2568, Title: "crash on --help", Body: "it crashes"}
	return triage.Analysis{Result: triage.Decide(issue, answers, nil), Answers: answers}
}

// featureAnalysis is a feature that reached the alignment node: nine answered questions.
func featureAnalysis() triage.Analysis {
	answers := substanceAnswers(triage.KindFeature, 0.81)
	alignment := map[string]nan.Answer{
		triage.QOnlyForRequester: {Type: nan.Noul, Noul: 0.12},
		triage.QFitsDirection:    {Type: nan.Noul, Noul: 0.72},
	}
	issue := triage.Issue{Repository: "spf13__cobra", Number: 2568, Title: "add a flag", Body: "please add it"}
	return triage.Analysis{
		Result:    triage.Decide(issue, answers, alignment),
		Answers:   answers,
		Alignment: alignment,
	}
}

func questionByID(t *testing.T, rows []triageQuestion, id string) triageQuestion {
	t.Helper()
	for _, row := range rows {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("la pregunta %q no está en la tabla", id)
	return triageQuestion{}
}

// S1: el veredicto nunca aparece sin su probabilidad. Un hecho determinista reporta 1 porque
// es certeza, y una request fallida no tiene probabilidad: se dice en vez de inventarla.
func TestTriageNeverShowsAVerdictWithoutItsProbability(t *testing.T) {
	view := newTriageView(bugAnalysis(), triageDecision(), triagedIssue("crash on --help"))
	if view.Outcome != string(triage.Candidate) {
		t.Errorf("outcome = %q, want %q", view.Outcome, triage.Candidate)
	}
	if view.Probability != "0,86" {
		t.Errorf("probabilidad = %q, want %q", view.Probability, "0,86")
	}
	if view.Class != "candidate" {
		t.Errorf("clase = %q, want candidate", view.Class)
	}

	fact := triage.Analysis{Result: triage.Result{
		Outcome: triage.Marked, Reason: triage.ReasonAlreadyDone, Probability: 1,
	}}
	if got := newTriageView(fact, triageDecision(), triagedIssue("done")).Probability; got != "1,00" {
		t.Errorf("probabilidad de un hecho determinista = %q, want %q", got, "1,00")
	}

	failed := triage.Analysis{Result: triage.Result{
		Outcome: triage.Undecided, Reason: triage.ReasonEndpointFailed,
	}}
	view = newTriageView(failed, triageDecision(), triagedIssue("crash on --help"))
	if view.Probability != "—" {
		t.Errorf("probabilidad de una request fallida = %q, want %q", view.Probability, "—")
	}
	if !strings.Contains(view.Why, "Clef no contestó") {
		t.Errorf("la pantalla no dice que Clef no contestó: %q", view.Why)
	}
	if !strings.Contains(view.Why, "volver a correr") {
		t.Errorf("la pantalla no dice que se puede volver a correr: %q", view.Why)
	}
}

// S2: las preguntas son las que se preguntaron, en el orden congelado, y nunca un número
// fijo: un bug muestra siete, una feature con alineación nueve.
func TestTriageListsTheQuestionsThatWereAsked(t *testing.T) {
	bug := newTriageView(bugAnalysis(), triageDecision(), triagedIssue("crash on --help"))
	if len(bug.Questions) != 7 {
		t.Fatalf("preguntas de un bug = %d, want 7", len(bug.Questions))
	}
	frozen := []string{
		triage.QKind, triage.QSelfContained, triage.QHasEvidence, triage.QUnambiguous,
		triage.QDoneDefined, triage.QReportsDefect, triage.QEvidenceSufficient,
	}
	for i, id := range frozen {
		if bug.Questions[i].ID != id {
			t.Errorf("pregunta %d = %q, want %q", i, bug.Questions[i].ID, id)
		}
	}

	feature := newTriageView(featureAnalysis(), triageDecision(), triagedIssue("add a flag"))
	if len(feature.Questions) != 9 {
		t.Fatalf("preguntas de una feature con alineación = %d, want 9", len(feature.Questions))
	}
	if feature.Questions[7].ID != triage.QOnlyForRequester || feature.Questions[8].ID != triage.QFitsDirection {
		t.Errorf("las dos de alineación no van al final: %q, %q",
			feature.Questions[7].ID, feature.Questions[8].ID)
	}

	partial := featureAnalysis()
	delete(partial.Answers, triage.QDoneDefined)
	delete(partial.Alignment, triage.QOnlyForRequester)
	if got := len(newTriageView(partial, triageDecision(), triagedIssue("add a flag")).Questions); got != 7 {
		t.Errorf("preguntas sin dos respuestas = %d, want 7: nadie ve una pregunta que no se hizo", got)
	}
}

// S3: cada respuesta viaja con su valor, y la que decidió el veredicto se distingue de las
// que sólo se midieron.
func TestTriageRendersEachAnswerWithItsValue(t *testing.T) {
	view := newTriageView(bugAnalysis(), triageDecision(), triagedIssue("crash on --help"))

	kind := questionByID(t, view.Questions, triage.QKind)
	if kind.Answer != "defecto" {
		t.Errorf("la etiqueta elegida = %q, want %q", kind.Answer, "defecto")
	}
	if kind.Value != "0,86" {
		t.Errorf("la confianza de la etiqueta = %q, want %q", kind.Value, "0,86")
	}

	sufficiency := questionByID(t, view.Questions, triage.QEvidenceSufficient)
	if sufficiency.Value != "0,93" {
		t.Errorf("el valor de una respuesta noul = %q, want %q", sufficiency.Value, "0,93")
	}
	if sufficiency.Answer != "" {
		t.Errorf("una respuesta noul no elige etiqueta: %q", sufficiency.Answer)
	}

	if got := decidedIDs(view.Questions); got != triage.QKind {
		t.Errorf("la respuesta que decidió el bug = %q, want %q", got, triage.QKind)
	}

	doubtAnswers := substanceAnswers(triage.KindBug, 0.86)
	doubtAnswers[triage.QEvidenceSufficient] = nan.Answer{Type: nan.Noul, Noul: 0.21}
	doubt := triage.Analysis{
		Result:  triage.Decide(triage.Issue{Number: 2568, Body: "x"}, doubtAnswers, nil),
		Answers: doubtAnswers,
	}
	if got := decidedIDs(newTriageView(doubt, triageDecision(), triagedIssue("x")).Questions); got != triage.QEvidenceSufficient {
		t.Errorf("la respuesta que decidió la duda = %q, want %q", got, triage.QEvidenceSufficient)
	}

	if got := decidedIDs(newTriageView(featureAnalysis(), triageDecision(), triagedIssue("x")).Questions); got != triage.QFitsDirection {
		t.Errorf("la respuesta que decidió la feature = %q, want %q", got, triage.QFitsDirection)
	}
}

// decidedIDs returns the ids of the rows the screen marks as the ones that decided, joined so
// a failure message names all of them at once.
func decidedIDs(rows []triageQuestion) string {
	var out []string
	for _, row := range rows {
		if row.Decided {
			out = append(out, row.ID)
		}
	}
	return strings.Join(out, ",")
}

// S4: cuatro abstenciones, cuatro arreglos. Se distinguen en forma y en texto, cada una nombra
// el motivo real del código, y la duda nombra en castellano las preguntas que faltan.
func TestTriageKeepsTheFourAbstentionsApart(t *testing.T) {
	cases := []struct {
		name  string
		view  triageView
		class string
	}{
		{"duda", newTriageView(triage.Analysis{Result: triage.Result{
			Outcome: triage.Doubt, Reason: triage.ReasonInsufficientEvidence,
			Missing: []string{triage.QEvidenceSufficient, triage.QSelfContained}, Probability: 0.21,
		}}, triageDecision(), triagedIssue("x")), "doubt"},
		{"derivada", newTriageView(triage.Analysis{Result: triage.Result{
			Outcome: triage.Derived, Reason: triage.ReasonSecondRoundPending, Kind: triage.KindQuestion, Probability: 0.77,
		}}, triageDecision(), triagedIssue("x")), "derived"},
		{"condicionada", newTriageView(triage.Analysis{Result: triage.Result{
			Outcome: triage.Conditional, Reason: triage.ReasonAlignmentUnavailable, Kind: triage.KindFeature, Probability: 0.8,
		}}, triageDecision(), triagedIssue("x")), "conditional"},
		{"sin respuesta", newTriageView(triage.Analysis{Result: triage.Result{
			Outcome: triage.Undecided, Reason: triage.ReasonEndpointFailed,
		}}, triageDecision(), triagedIssue("x")), "undecided"},
	}

	classes := map[string]string{}
	whys := map[string]string{}
	for _, c := range cases {
		if c.view.Class != c.class {
			t.Errorf("%s: clase = %q, want %q", c.name, c.view.Class, c.class)
		}
		if other, ok := classes[c.view.Class]; ok {
			t.Errorf("%s y %s comparten la clase %q", other, c.name, c.view.Class)
		}
		classes[c.view.Class] = c.name
		if other, ok := whys[c.view.Why]; ok {
			t.Errorf("%s y %s se explican igual: %q", other, c.name, c.view.Why)
		}
		whys[c.view.Why] = c.name
	}

	// El motivo real del código se lee tal cual: la pantalla no lo renombra.
	for _, want := range []triage.Result{{Reason: triage.ReasonAlignmentUnavailable}, {Reason: triage.ReasonAskMaintainer},
		{Reason: triage.ReasonIncompleteAnswer}} {
		view := newTriageView(triage.Analysis{Result: triage.Result{
			Outcome: triage.Conditional, Reason: want.Reason,
		}}, triageDecision(), triagedIssue("x"))
		if view.Reason != want.Reason {
			t.Errorf("motivo = %q, want %q", view.Reason, want.Reason)
		}
	}

	// Las dos condiciones se dicen distinto: pedir al mantenedor no es no poder leer el rumbo.
	unavailable := newTriageView(triage.Analysis{Result: triage.Result{
		Outcome: triage.Conditional, Reason: triage.ReasonAlignmentUnavailable,
	}}, triageDecision(), triagedIssue("x"))
	ask := newTriageView(triage.Analysis{Result: triage.Result{
		Outcome: triage.Conditional, Reason: triage.ReasonAskMaintainer,
	}}, triageDecision(), triagedIssue("x"))
	if unavailable.Why == ask.Why {
		t.Error("alignment_unavailable y ask_maintainer se explican igual")
	}

	// Y las dos formas de undecided también: una request caída no es una respuesta ilegible.
	failed := newTriageView(triage.Analysis{Result: triage.Result{
		Outcome: triage.Undecided, Reason: triage.ReasonEndpointFailed,
	}}, triageDecision(), triagedIssue("x"))
	unreadable := newTriageView(triage.Analysis{Result: triage.Result{
		Outcome: triage.Undecided, Reason: triage.ReasonIncompleteAnswer,
	}}, triageDecision(), triagedIssue("x"))
	if failed.Why == unreadable.Why {
		t.Error("endpoint_failed e incomplete_answer se explican igual")
	}

	// La duda nombra lo que falta con el texto de la pantalla, no con el id del cable.
	doubt := newTriageView(triage.Analysis{Result: triage.Result{
		Outcome: triage.Doubt, Reason: triage.ReasonInsufficientEvidence,
		Missing: []string{triage.QEvidenceSufficient, triage.QSelfContained}, Probability: 0.21,
	}}, triageDecision(), triagedIssue("x"))
	wantMissing := []string{questionText(triage.QEvidenceSufficient), questionText(triage.QSelfContained)}
	if len(doubt.Missing) != len(wantMissing) {
		t.Fatalf("preguntas faltantes = %v, want %v", doubt.Missing, wantMissing)
	}
	for i, want := range wantMissing {
		if doubt.Missing[i] != want {
			t.Errorf("pregunta faltante %d = %q, want %q", i, doubt.Missing[i], want)
		}
		if doubt.Missing[i] == triage.QEvidenceSufficient {
			t.Errorf("la pregunta faltante quedó con el id del cable: %q", doubt.Missing[i])
		}
	}
}

// S5: MARCADA conserva el motivo que la resolvió, y los cinco del pre-gate se leen distinto:
// cinco frases para el mismo veredicto serían una sola razón.
func TestTriageNamesEachPreGateReason(t *testing.T) {
	reasons := []string{
		triage.ReasonAlreadyDone,
		triage.ReasonDeclined,
		triage.ReasonAlreadyWorking,
		triage.ReasonClosed,
		triage.ReasonNoContent,
	}

	seen := make(map[string]string, len(reasons))
	for _, reason := range reasons {
		view := newTriageView(triage.Analysis{Result: triage.Result{
			Outcome: triage.Marked, Reason: reason, Probability: 1,
		}}, triageDecision(), triagedIssue("x"))

		if view.Outcome != string(triage.Marked) {
			t.Errorf("%s: outcome = %q, want %q", reason, view.Outcome, triage.Marked)
		}
		if view.Reason != reason {
			t.Errorf("motivo = %q, want %q", view.Reason, reason)
		}
		if view.Why == "" {
			t.Errorf("%s: MARCADA no dice por qué", reason)
		}
		if other, ok := seen[view.Why]; ok {
			t.Errorf("%s y %s se leen igual: %q", other, reason, view.Why)
		}
		seen[view.Why] = reason
	}

	// El spam también conserva su motivo, que no es un hecho del pre-gate.
	spam := newTriageView(triage.Analysis{Result: triage.Result{
		Outcome: triage.Marked, Reason: triage.ReasonNotCodeWork, Probability: 0.9,
	}}, triageDecision(), triagedIssue("x"))
	if spam.Reason != triage.ReasonNotCodeWork || spam.Why == "" {
		t.Errorf("el spam perdió su motivo: %q / %q", spam.Reason, spam.Why)
	}
}

// S6: el kind declarado se contrasta con el leído sólo cuando la lectura está decidida. En el
// corte y arriba se dice la contradicción; abajo no se presenta como tal.
func TestTriageContrastsTheDeclaredKindOnlyAboveTheCut(t *testing.T) {
	// El corte cae sobre la confianza de la respuesta que dijo qué es la issue, no sobre
	// Result.Probability: en una feature esa es la probabilidad del nodo de alineación, que mide
	// otra cosa. Citarla sería poner dos números distintos al mismo juicio, y la fila del kind ya
	// muestra el suyo arriba.
	declaredVsRead := func(confidence, alignment float64) triageView {
		return newTriageView(triage.Analysis{
			Result: triage.Result{Outcome: triage.Candidate, Kind: triage.KindFeature, Probability: alignment},
			Answers: map[string]nan.Answer{
				triage.QKind: {Type: nan.Choice, Choice: triage.KindFeature, Confidence: confidence},
			},
		}, triageDecision(), triagedIssue("bug: crash on --help"))
	}

	at := declaredVsRead(0.6, 0.9)
	if !strings.Contains(at.Contradiction, "El autor también puede mentir") {
		t.Errorf("en el corte no se dice la contradicción: %q", at.Contradiction)
	}
	if !strings.Contains(at.Contradiction, "defecto") || !strings.Contains(at.Contradiction, "funcionalidad") {
		t.Errorf("la contradicción no nombra los dos kinds: %q", at.Contradiction)
	}
	if !strings.Contains(at.Contradiction, "0,60") {
		t.Errorf("la contradicción no dice con qué probabilidad se leyó: %q", at.Contradiction)
	}

	below := declaredVsRead(0.59, 0.9)
	if below.Contradiction != "" {
		t.Errorf("por debajo del corte se presentó como contradicción: %q", below.Contradiction)
	}
	if below.Declared != "defecto" {
		t.Errorf("la declaración del autor se perdió: %q", below.Declared)
	}

	// El nodo de alineación puede estar muy seguro de otra cosa; no vuelve segura la lectura del
	// kind, y antes lo hacía.
	alignedOnly := declaredVsRead(0.4, 0.95)
	if alignedOnly.Contradiction != "" {
		t.Errorf("la probabilidad del nodo de alineación levantó la contradicción: %q", alignedOnly.Contradiction)
	}

	agreeing := newTriageView(triage.Analysis{
		Result: triage.Result{Outcome: triage.Candidate, Kind: triage.KindBug, Probability: 0.9},
		Answers: map[string]nan.Answer{
			triage.QKind: {Type: nan.Choice, Choice: triage.KindBug, Confidence: 0.9},
		},
	}, triageDecision(), triagedIssue("bug: crash on --help"))
	if agreeing.Contradiction != "" {
		t.Errorf("coincidir con el autor se mostró como contradicción: %q", agreeing.Contradiction)
	}

	undeclared := newTriageView(bugAnalysis(), triageDecision(), triagedIssue("crash on --help"))
	if undeclared.HasDeclaration || undeclared.Contradiction != "" {
		t.Errorf("un título sin prefijo no declara nada: %v / %q", undeclared.HasDeclaration, undeclared.Contradiction)
	}
}

// S7: la etiqueta original viaja verbatim, como dato del mantenedor y nunca como insumo de la
// decisión (I6).
func TestTriageKeepsTheLabelsVerbatim(t *testing.T) {
	labels := []string{"good-first-issue", "✭ help wanted ✭", "kind/Bug"}
	view := newTriageView(bugAnalysis(), triageDecision(), triagedIssue("crash on --help", labels...))

	if !view.HasLabels {
		t.Fatal("las etiquetas no llegaron al bloque")
	}
	if len(view.Labels) != len(labels) {
		t.Fatalf("etiquetas = %v, want %v", view.Labels, labels)
	}
	for i, want := range labels {
		if view.Labels[i] != want {
			t.Errorf("etiqueta %d = %q, want %q", i, view.Labels[i], want)
		}
	}

	if free := newTriageView(bugAnalysis(), triageDecision(), triagedIssue("crash on --help")); free.HasLabels {
		t.Error("una issue sin etiquetas dice tener etiquetas")
	}
}

// S8: el bloque declara el pin de la respuesta (I4, I14). Un veredicto sin su pin se lee como
// fresco, así que cada campo sale del registro y no del reloj.
func TestTriageShowsThePinOfTheStoredVerdict(t *testing.T) {
	view := newTriageView(bugAnalysis(), triageDecision(), triagedIssue("crash on --help"))

	for _, want := range []struct {
		field string
		got   string
	}{
		{"analizado", view.Analyzed},
		{"latencia", view.Latency},
		{"modelo", view.Model},
		{"prompt", view.Prompt},
		{"commit", view.Commit},
	} {
		if want.got == "" {
			t.Errorf("el pin no declara %s", want.field)
		}
	}

	if view.Analyzed != "2026-11-14" {
		t.Errorf("analizado = %q, want 2026-11-14", view.Analyzed)
	}
	if view.Latency != "1.234s" {
		t.Errorf("latencia = %q, want 1.234s", view.Latency)
	}
	if view.Model != config.DecisionModel {
		t.Errorf("modelo = %q, want %q", view.Model, config.DecisionModel)
	}
	if view.Prompt != "3f2a9c1b7d4e" {
		t.Errorf("prompt = %q, want 3f2a9c1b7d4e", view.Prompt)
	}
	if view.Commit != testCommit[:12] {
		t.Errorf("commit = %q, want %q", view.Commit, testCommit[:12])
	}
}

// triageStore siembra el corpus que el informe necesita: commit pineado, una issue con cuerpo
// y sus etiquetas verbatim.
func triageStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	if err := st.SaveRepo(store.RepoEntry{
		Name: "spf13/cobra", Key: "spf13__cobra", CommitSHA: testCommit,
		TotalOpen: 1, Issues: 1, FetchedAt: fetchedAt,
	}); err != nil {
		t.Fatalf("SaveRepo() error = %v", err)
	}
	snap := &ghdata.Snapshot{
		Repository: ghdata.Repository{
			NameWithOwner: "spf13/cobra", CommitSHA: testCommit, PrimaryLanguage: "Go",
		},
		TotalOpen: 1,
		Issues: []ghdata.Issue{{
			Number:    2568,
			Title:     "YAML docs ignore linkHandler for see_also entries",
			Body:      "The link handler is ignored.",
			URL:       "https://github.com/spf13/cobra/issues/2568",
			Labels:    []string{"good-first-issue", "kind/bug"},
			UpdatedAt: time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC),
		}},
	}
	if err := st.SaveSnapshot("spf13__cobra", snap); err != nil {
		t.Fatalf("SaveSnapshot() error = %v", err)
	}
	return st
}

// fixedJudge is the whole transport replaced by a recorded answer: the stored verdict is the
// one policy.Decide produces from these answers, and no test touches the network.
type fixedJudge map[string]nan.Answer

func (j fixedJudge) Ask(_ context.Context, _ string, _ map[string]nan.Question) (map[string]nan.Answer, error) {
	return j, nil
}

// seedTriage runs the real first pass over the fixture issue and leaves the verdict in the
// journal, with a pin whose every field can be read back.
func seedTriage(t *testing.T, st *store.Store) {
	t.Helper()

	journal, err := st.Decisions("spf13__cobra")
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	at := time.Date(2026, 11, 14, 7, 5, 0, 0, time.UTC)
	clock := func() time.Time {
		now := at
		at = at.Add(1200 * time.Millisecond)
		return now
	}
	p1, err := triage.New(triage.Options{
		Judge:     fixedJudge(substanceAnswers(triage.KindBug, 0.86)),
		Journal:   journal,
		Model:     config.DecisionModel,
		CommitSHA: testCommit,
		Now:       clock,
	})
	if err != nil {
		t.Fatalf("triage.New() error = %v", err)
	}
	if _, err := p1.Run(context.Background(), triage.Issue{
		Repository: "spf13__cobra",
		Number:     2568,
		Title:      "YAML docs ignore linkHandler for see_also entries",
		Body:       "The link handler is ignored.",
		Labels:     []string{"good-first-issue", "kind/bug"},
	}); err != nil {
		t.Fatalf("p1.Run() error = %v", err)
	}
}
