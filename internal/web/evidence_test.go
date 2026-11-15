package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/explore"
	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// testCommit is the corpus pin these tests use: the same commit the whole fixture world is
// frozen at.
const testCommit = "adbc8813901bba65827259daa8e22ff94ec1f30e"

// evidenceStore siembra un corpus con commit y una issue, que es lo que el bloque de
// evidencia necesita para poder buscar el análisis guardado.
func evidenceStore(t *testing.T) *store.Store {
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
			UpdatedAt: time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC),
		}},
	}
	if err := st.SaveSnapshot("spf13__cobra", snap); err != nil {
		t.Fatalf("SaveSnapshot() error = %v", err)
	}
	return st
}

// storedRecord is one analysis as the journal keeps it: evidence plus what it cost.
func storedRecord() explore.Record {
	return explore.Record{
		Repository: "spf13__cobra",
		CommitSHA:  testCommit,
		Issue:      2568,
		Model:      "gemma4",
		Evidence: &explore.Evidence{
			Intent: "ignore hidden and see_also entries when aligning the yaml docs",
			Files: []explore.File{{
				Path: "doc/yaml_docs.go", Symbol: "GenYamlCustom", Line: 44,
				Reason: "renders the see_also block", Relation: explore.RelationObserved,
			}},
			CallPaths: [][]string{{"GenYamlCustom", "yamlDoc"}},
			Tests:     []string{"doc/yaml_docs_test.go:TestGenYaml"},
			Requirements: explore.Requirements{
				Knowledge: []string{"Go templates"},
				Project:   []string{"doc package"},
				Process:   []string{"PR with tests"},
			},
			BlastRadius:        explore.BlastRadius{Files: 1, Symbols: 1},
			Unknowns:           []string{"whether the queue matters"},
			MissingInformation: []string{"acceptance criteria"},
		},
		Metrics: explore.Metrics{Complete: true, TotalMs: 109386},
	}
}

func seedAnalysis(t *testing.T, st *store.Store, key store.Key, record explore.Record) {
	t.Helper()

	verdict, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	decisions, err := st.Decisions("spf13__cobra")
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	if err := decisions.Put(store.Decision{
		Key:       key,
		LatencyMs: record.Metrics.TotalMs,
		Verdict:   verdict,
		At:        time.Date(2026, 11, 14, 3, 33, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
}

// S1/S6: el bloque muestra el archivo con path:line, su relación y el pin que dice cuándo y
// bajo qué pregunta se produjo. Sin el pin, una evidencia guardada se lee como fresca.
func TestIssuePageShowsTheStoredAnalysisWithItsPin(t *testing.T) {
	st := evidenceStore(t)
	seedAnalysis(t, st, store.Key{
		Issue: 2568, Pass: store.PassExplore, Model: "gemma4",
		PromptVersion: explore.Version(), CommitSHA: testCommit,
	}, storedRecord())

	page := body(t, exploreHandler(t, st, nil, nil), "/issue/spf13__cobra/2568")

	for _, want := range []string{
		"doc/yaml_docs.go:44",                // archivo con línea
		"GenYamlCustom",                      // el símbolo
		`class="rel observed"`,               // la relación, con su clase
		"ignore hidden and see_also entries", // la intención
		"guardado el <b>2026-11-14</b>",      // cuándo se produjo
		"1m49.386s",                          // lo que costó
		"gemma4",                             // con qué modelo
		explore.Version(),                    // con qué versión del prompt
		"adbc8813901b",                       // y sobre qué commit
		"doc/yaml_docs_test.go:TestGenYaml",  // los tests
		"PR with tests",                      // los tres baldes
		"whether the queue matters",          // las dudas
		"acceptance criteria",                // lo que la issue no dice
		"Volver a analizar",                  // se ofrece rehacerlo, no analizarlo por primera vez
	} {
		if !strings.Contains(page, want) {
			t.Errorf("falta %q en la pantalla de la issue", want)
		}
	}
}

// S6 (decisión A): un «observed» que la traza no respalda se muestra como probable. Lo que
// una herramienta no devolvió en esa corrida no se puede presentar como visto.
func TestEvidenceDowngradesAnObservedTheTraceDoesNotBack(t *testing.T) {
	st := evidenceStore(t)
	record := storedRecord()
	record.Violations = []explore.Violation{{Path: "doc/yaml_docs.go"}}
	seedAnalysis(t, st, store.Key{
		Issue: 2568, Pass: store.PassExplore, Model: "gemma4",
		PromptVersion: explore.Version(), CommitSHA: testCommit,
	}, record)

	page := body(t, exploreHandler(t, st, nil, nil), "/issue/spf13__cobra/2568")

	if strings.Contains(page, `class="rel observed"`) {
		t.Error("un observed sin respaldo en la traza se mostró como observed")
	}
	if !strings.Contains(page, `class="rel probable"`) {
		t.Error("el archivo degradado no se muestra como probable")
	}
	if !strings.Contains(page, "la traza de la corrida no lo respalda") {
		t.Error("la pantalla no dice por qué se degradó la relación")
	}
}

// Sin análisis guardado el bloque no queda vacío: dice qué falta y ofrece la acción.
func TestIssuePageOffersTheAnalysisWhenThereIsNoEvidence(t *testing.T) {
	page := body(t, exploreHandler(t, evidenceStore(t), nil, nil), "/issue/spf13__cobra/2568")

	if !strings.Contains(page, "Analizar esta issue") {
		t.Error("sin evidencia la pantalla no ofrece analizarla")
	}
	if !strings.Contains(page, "Todavía no hay evidencia guardada") {
		t.Error("sin evidencia la pantalla no lo dice")
	}
	if strings.Contains(page, "Qué toca esta issue</h2>") && strings.Contains(page, `class="rel`) {
		t.Error("sin evidencia no hay relaciones que mostrar")
	}
}

// I4: un análisis de otra pregunta (otro prompt) no se muestra como si respondiera esta.
func TestIssuePageIgnoresAnAnalysisOfAnotherQuestion(t *testing.T) {
	st := evidenceStore(t)
	seedAnalysis(t, st, store.Key{
		Issue: 2568, Pass: store.PassExplore, Model: "gemma4",
		PromptVersion: "otro-prompt", CommitSHA: testCommit,
	}, storedRecord())

	page := body(t, exploreHandler(t, st, nil, nil), "/issue/spf13__cobra/2568")

	if strings.Contains(page, "doc/yaml_docs.go:44") {
		t.Error("se mostró un análisis que responde otra pregunta")
	}
	if !strings.Contains(page, "Analizar esta issue") {
		t.Error("sin evidencia de esta pregunta la pantalla no ofrece analizarla")
	}
}

// S4/S11/S12: mientras la cola trabaja, la tabla muestra el estado por issue, la pantalla
// ofrece cancelar y vaciar, y el pie dice la verdad sobre el pipe que todavía falta.
func TestDashboardShowsTheQueueAndThePipeItCannotRun(t *testing.T) {
	st, key := indexedCorpus(t, 1)
	fake := newFakeStage().held()
	defer fake.release()
	h := exploreHandler(t, st, fake.stage, nil)

	rec := post(t, h, "/explore", url.Values{"repo": {key}, "issue": {"1"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /explore status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	waitFor(t, "que la corrida entre", func() bool { return fakeStageRunning(h, key, 1) })

	page := body(t, h, "/?repo="+key)
	for _, want := range []string{
		"en curso",                 // el estado de la fila
		"Vaciar la cola",           // descartar lo pendiente
		"Cancelar todo",            // cortar también lo que corre
		"≈ 1m49s",                  // la estimación, con el costo medido
		"a 3 en paralelo",          // a cuántos corre
		`content="5"`,              // el <noscript> que refresca sin JS
		`form="analizar"`,          // la casilla se engancha al formulario del análisis
		`action="/explore/cancel"`, // cada fila puede cancelarse
		"triaje preliminar",        // el pie nombra la compuerta que falta
	} {
		if !strings.Contains(page, want) {
			t.Errorf("falta %q en el dashboard", want)
		}
	}
	if strings.Contains(page, "todavía no está conectado: llega con") {
		t.Error("el pie sigue diciendo que el análisis no está conectado")
	}
}

// fakeStageRunning reports whether the queue says one row is running.
func fakeStageRunning(h *Handler, key string, issue int) bool {
	data, err := buildPageWith(h.store, url.Values{"repo": {key}}, h.queue)
	if err != nil {
		return false
	}
	for _, row := range data.Rows {
		if row.Number != issue {
			continue
		}
		for _, stage := range row.Stages {
			if stage.Stage == stageExplore {
				return stage.State == jobRunning
			}
		}
	}
	return false
}

// S11: el fragmento de filas es HTML renderizado por el servidor, no un contrato JSON que
// después haya que mantener en dos lugares.
func TestRowsFragmentRendersOnlyTheRows(t *testing.T) {
	st := evidenceStore(t)
	rec := get(t, exploreHandler(t, st, nil, nil), "/explore/rows?repo=spf13__cobra")

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /explore/rows status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html…", ct)
	}
	fragment := rec.Body.String()
	if !strings.Contains(fragment, "<tr>") {
		t.Error("el fragmento no trae filas")
	}
	if strings.Contains(fragment, "<html") || strings.Contains(fragment, "<body") {
		t.Error("el fragmento trae la página entera")
	}
	if !strings.Contains(fragment, "YAML docs ignore linkHandler") {
		t.Error("el fragmento no trae el título de la issue")
	}
}
