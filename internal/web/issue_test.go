package web

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/config"
	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// issueCorpus siembra el corpus con la forma de la 1272 de cobra: tres
// etiquetas, una PR abierta, cuerpo con marcado hostil y cuatro comentarios.
// Es el caso del criterio de aceptación de la issue #6.
func issueCorpus(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}

	for _, e := range []store.RepoEntry{
		{
			Name:      "spf13/cobra",
			Key:       "spf13__cobra",
			CommitSHA: "adbc8813901bba65827259daa8e22ff94ec1f30e",
			TotalOpen: 262,
			Issues:    1,
			FetchedAt: fetchedAt,
		},
		{Name: "httpie/cli", Key: "httpie__cli", TotalOpen: 192, Issues: 0, FetchedAt: fetchedAt},
	} {
		if err := st.SaveRepo(e); err != nil {
			t.Fatalf("SaveRepo(%s) error = %v", e.Name, err)
		}
	}

	cobra := &ghdata.Snapshot{
		Repository: ghdata.Repository{NameWithOwner: "spf13/cobra", PrimaryLanguage: "Go"},
		TotalOpen:  262,
		Issues: []ghdata.Issue{{
			Number:            1272,
			Title:             "Hidden Commands Should be Ignored When Calculating Use Indentation",
			State:             "OPEN",
			URL:               "https://github.com/spf13/cobra/issues/1272",
			Author:            "jpmcb",
			AuthorAssociation: "MEMBER",
			CreatedAt:         time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC),
			UpdatedAt:         time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC),
			Labels:            []string{"kind/bug", "good-first-issue", "lifecycle/needs-pr"},
			CommentCount:      4,
			LinkedPRs: []ghdata.PullRequest{
				{Number: 1984, State: "OPEN", URL: "https://github.com/spf13/cobra/pull/1984"},
				{Number: 1900, State: "CLOSED", URL: "https://github.com/spf13/cobra/pull/1900", Merged: true},
			},
			Body: "The `Use` line has one space too many.\n\n<script>alert('x')</script>\n<img src=x onerror=alert(1)>\n",
			Comments: []ghdata.Comment{
				{Author: "marckhouzam", Body: "Reproduced on main.", AuthorAssociation: "MEMBER", CreatedAt: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)},
				{Author: "jpmcb", Body: "[the fix](https://example.com/pr) is small.", AuthorAssociation: "MEMBER", CreatedAt: time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC)},
				{Author: "sclevine", Body: "Any update?", AuthorAssociation: "NONE", CreatedAt: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
				{Author: "jpmcb", Body: "Still open, PRs welcome.", AuthorAssociation: "MEMBER", CreatedAt: time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)},
			},
		}},
	}
	if err := st.SaveSnapshot("spf13__cobra", cobra); err != nil {
		t.Fatalf("SaveSnapshot(cobra) error = %v", err)
	}
	return st
}

// El criterio de aceptación, tal como está escrito en la issue: título,
// etiquetas verbatim, estado, PR, fecha, cuerpo y los cuatro comentarios.
func TestIssueRendersTheWholeThread(t *testing.T) {
	rec := get(t, newHandler(t, issueCorpus(t)), "/issue/spf13__cobra/1272")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /issue/spf13__cobra/1272 status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html…", ct)
	}
	page := rec.Body.String()

	for _, want := range []string{
		"#1272",                         // el número
		"Hidden Commands Should be",     // el título
		"kind/bug",                      // etiquetas verbatim, sin normalizar
		"good-first-issue",              //
		"lifecycle/needs-pr",            //
		"abierta",                       // el estado
		"#1984",                         // la PR vinculada
		"2026-11-11",                    // actualizada
		"2026-03-04",                    // abierta
		"<code>Use</code>",              // el cuerpo pasó por el render
		"Reproduced on main.",           // los comentarios
		"Any update?",                   //
		"Still open, PRs welcome.",      //
		`href="https://example.com/pr"`, // un enlace del cuerpo, permitido
		"4 comentarios",                 // el conteo
		"Abrir en GitHub",               // el enlace a GitHub, como acción secundaria
	} {
		if !strings.Contains(page, want) {
			t.Errorf("falta %q en la pantalla", want)
		}
	}

	// Los cuatro comentarios, con su autor.
	if got := strings.Count(page, `class="comment"`); got != 4 {
		t.Errorf("comentarios renderizados = %d, want 4", got)
	}
	for _, author := range []string{"marckhouzam", "sclevine"} {
		if !strings.Contains(page, author) {
			t.Errorf("falta el autor %q", author)
		}
	}
}

// El cuerpo hostil no ejecuta nada en la pantalla, igual que en el renderer
// suelto: acá se prueba el mismo contrato a través de la ruta.
func TestIssueScreenEscapesHostileBody(t *testing.T) {
	page := body(t, newHandler(t, issueCorpus(t)), "/issue/spf13__cobra/1272")

	for _, forbidden := range []string{"<script>alert", "<img", `<img src=x`} {
		if strings.Contains(page, forbidden) {
			t.Errorf("la pantalla dejó pasar %q", forbidden)
		}
	}
	if !strings.Contains(page, "&lt;script&gt;alert(&#39;x&#39;)&lt;/script&gt;") {
		t.Errorf("el cuerpo hostil no quedó visible y escapado:\n%s", page)
	}
}

// Dos 404 distintos, y ninguno disfrazado: ni el repositorio ausente se
// sustituye por otro, ni la issue ausente cae en la primera del corpus.
func TestIssueNotFound(t *testing.T) {
	h := newHandler(t, issueCorpus(t))

	for _, target := range []string{
		"/issue/spf13__cobra/999999", // la issue no existe
		"/issue/nope__nope/1272",     // el repositorio no está en el corpus
		"/issue/spf13__cobra/abc",    // el número no es un número
	} {
		if rec := get(t, h, target); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", target, rec.Code, http.StatusNotFound)
		}
	}
}

// El enlace a GitHub sigue existiendo, pero deja de ser el camino principal.
func TestIssueGitHubLinkIsSecondary(t *testing.T) {
	page := body(t, newHandler(t, issueCorpus(t)), "/issue/spf13__cobra/1272")

	want := `<a class="btn" href="https://github.com/spf13/cobra/issues/1272" rel="noopener noreferrer">Abrir en GitHub</a>`
	if !strings.Contains(page, want) {
		t.Errorf("falta el enlace secundario a GitHub: %s", want)
	}
}

// La pantalla no pierde el repositorio: el rail sigue ahí y la vuelta al
// dashboard lo conserva.
func TestIssueKeepsRepoContext(t *testing.T) {
	page := body(t, newHandler(t, issueCorpus(t)), "/issue/spf13__cobra/1272")

	for _, want := range []string{
		`href="/?repo=spf13__cobra"`, // la vuelta, con el repo
		"spf13/cobra",                // el repositorio activo, nombrado en la pantalla
		"262 abiertas",               // su corpus, no el de otro
	} {
		if !strings.Contains(page, want) {
			t.Errorf("falta %q: la pantalla perdió el contexto de repositorio", want)
		}
	}
}

// S9/S11: con un veredicto guardado el bloque lo muestra con su probabilidad y su pin; sin
// él lo dice y nombra el comando, y en ningún caso ofrece un POST /triage que no existe.
func TestIssueScreenShowsTheTriageAndSaysWhenThereIsNone(t *testing.T) {
	st := triageStore(t)
	seedTriage(t, st)

	page := body(t, newHandler(t, st), "/issue/spf13__cobra/2568")
	for _, want := range []string{
		`aria-label="Informe de triaje"`,   // el bloque existe
		"CANDIDATA",                        // el veredicto, con su palabra
		"0,86",                             // su probabilidad, con coma decimal
		"Reporta un defecto que ya existe", // una de las preguntas que se hizo
		"guardado el <b>2026-11-14</b>",    // el pin: cuándo
		"1.2s",                             // el pin: cuánto costó
		config.DecisionModel,               // el pin: con qué modelo
		"kind/bug",                         // la etiqueta del mantenedor, verbatim
		`class="tr-labels"`,                // y en su propio lugar, marcada como dato
		"no el insumo de la decisión",      // se dice que la etiqueta no juzga (I6)
		`class="tr-outcome candidate"`,     // la forma del veredicto, en su clase
		`class="tr-q decided"`,             // la fila que decidió se distingue de las medidas
		"7 preguntas medidas",              // las que se hicieron, no una lista fija
		"Preguntas medidas",                // y el rótulo de la tabla, corto para que no se recorte
		`evid-files-wrap`,                  // la tabla se corre en horizontal a 360 px
	} {
		if !strings.Contains(page, want) {
			t.Errorf("falta %q en el informe de triaje", want)
		}
	}
	if strings.Contains(page, `action="/triage"`) {
		t.Error("el bloque ofrece un POST /triage que no existe")
	}

	empty := body(t, newHandler(t, triageStore(t)), "/issue/spf13__cobra/2568")
	if strings.Contains(empty, `class="tr-questions"`) {
		t.Error("sin veredicto guardado el bloque muestra una tabla de preguntas")
	}
	if !strings.Contains(empty, "Todavía no hay triaje guardado") {
		t.Error("sin veredicto guardado el bloque no lo dice")
	}
	if !strings.Contains(empty, "gfi triage spf13/cobra") {
		t.Error("sin veredicto guardado el bloque no nombra el comando")
	}
	if strings.Contains(empty, `action="/triage"`) {
		t.Error("sin veredicto el bloque ofrece un POST /triage que no existe")
	}
}

// Los tres puntos de inclusión de la ola 2 existen en el set de plantillas,
// aunque hoy no impriman nada. Sin ellos, tres issues distintas editan el
// mismo archivo y chocan entre sí.
func TestIssueScreenLeavesTheThreeAnchors(t *testing.T) {
	sets, err := parseScreens(templateFS, screenNames)
	if err != nil {
		t.Fatalf("parseScreens() error = %v", err)
	}
	issue, ok := sets[screenIssue]
	if !ok {
		t.Fatalf("no hay set para la pantalla %q", screenIssue)
	}

	for _, anchor := range []string{"triage", "evidence", "fit"} {
		if issue.Lookup(anchor) == nil {
			t.Errorf("la plantilla %q no existe: la ola 2 no tiene dónde enganchar", anchor)
		}
	}
}
