package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

var fetchedAt = time.Date(2026, 11, 14, 9, 30, 0, 0, time.UTC)

// corpus siembra un store temporal con las formas que importan: un repo con
// issues de los tres tipos que hoy se pueden afirmar, otro con corpus propio
// para probar el cambio de repositorio, y uno registrado sin corpus.
//
// Los repositorios se listan por nombre, así que el activo por defecto es
// httpie/cli y no spf13/cobra: los tests que hablan de cobra lo dicen explícito.
func corpus(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}

	add := func(name, key string, open, issues int) {
		t.Helper()
		err := st.SaveRepo(store.RepoEntry{
			Name:      name,
			Key:       key,
			CommitSHA: "adbc8813901bba65827259daa8e22ff94ec1f30e",
			TotalOpen: open,
			Issues:    issues,
			FetchedAt: fetchedAt,
		})
		if err != nil {
			t.Fatalf("SaveRepo(%s) error = %v", name, err)
		}
	}

	add("spf13/cobra", "spf13__cobra", 262, 3)
	add("httpie/cli", "httpie__cli", 192, 1)
	add("zeta/empty", "zeta__empty", 10, 0)

	cobra := &ghdata.Snapshot{
		Repository: ghdata.Repository{NameWithOwner: "spf13/cobra", PrimaryLanguage: "Go"},
		TotalOpen:  262,
		Issues: []ghdata.Issue{
			{
				Number:       2568,
				Title:        "YAML docs ignore linkHandler for see_also entries",
				Body:         "The link handler is ignored.",
				UpdatedAt:    time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC),
				CommentCount: 0,
			},
			{
				Number:       1391,
				Title:        "404 links on cobra.dev",
				UpdatedAt:    time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
				CommentCount: 17,
				Labels:       []string{"kind/documentation", "good-first-issue"},
				LinkedPRs:    []ghdata.PullRequest{{Number: 1984, State: "OPEN"}},
			},
			{
				Number:       1995,
				Title:        "Question about custom templates",
				UpdatedAt:    time.Date(2025, 10, 27, 0, 0, 0, 0, time.UTC),
				CommentCount: 3,
				Labels:       []string{"kind/support"},
			},
		},
	}
	if err := st.SaveSnapshot("spf13__cobra", cobra); err != nil {
		t.Fatalf("SaveSnapshot(cobra) error = %v", err)
	}

	httpie := &ghdata.Snapshot{
		Repository: ghdata.Repository{NameWithOwner: "httpie/cli", PrimaryLanguage: "Python"},
		TotalOpen:  192,
		Issues: []ghdata.Issue{{
			Number:       1978,
			Title:        "log_error ignores --quiet flag",
			UpdatedAt:    time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC),
			CommentCount: 2,
			LinkedPRs:    []ghdata.PullRequest{{Number: 2001, State: "OPEN"}},
		}},
	}
	if err := st.SaveSnapshot("httpie__cli", httpie); err != nil {
		t.Fatalf("SaveSnapshot(httpie) error = %v", err)
	}

	return st
}

func newHandler(t *testing.T, st *store.Store) *Handler {
	t.Helper()
	h, err := NewHandler(st)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return h
}

func get(t *testing.T, h *Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func body(t *testing.T, h *Handler, target string) string {
	t.Helper()
	return get(t, h, target).Body.String()
}

// Sin corpus la pantalla no miente con ceros: explica qué falta.
func TestIndexWithoutCorpus(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}

	rec := get(t, newHandler(t, st), "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", rec.Code, http.StatusOK)
	}
	page := rec.Body.String()
	if !strings.Contains(page, "Todavía no hay repositorios") {
		t.Errorf("la pantalla vacía no lo dice:\n%s", page)
	}
	if !strings.Contains(page, "gfi fetch") {
		t.Error("la pantalla vacía no indica cómo traer el primer repositorio")
	}
}

// Un handler sin store también renderiza: es el binario recién compilado.
func TestIndexWithoutStore(t *testing.T) {
	if rec := get(t, newHandler(t, nil), "/"); rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// El dashboard se arma con el corpus: repos en el rail, cifras y filas reales.
func TestIndexRendersCorpus(t *testing.T) {
	rec := get(t, newHandler(t, corpus(t)), "/?repo=spf13__cobra")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html…", ct)
	}
	page := rec.Body.String()

	for _, want := range []string{
		"spf13/cobra",                  // el repositorio activo
		"httpie/cli",                   // el otro, en el rail
		"adbc8813901b",                 // el commit pinneado, corto
		"YAML docs ignore linkHandler", // una fila
		"good-first-issue",             // etiqueta verbatim
		"2026-11-11",                   // la fecha de actualización
		"/static/app.css",
		"/static/app.js",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("falta %q en la página", want)
		}
	}

	// Dos libres (sin PR) y una tomada: se cuentan sobre el corpus, no sobre el filtro.
	if !strings.Contains(page, `<dd class="live">2</dd>`) {
		t.Error("la cifra de libres no es la esperada")
	}
	if !strings.Contains(page, `<dd class="taken">1</dd>`) {
		t.Error("la cifra de tomadas no es la esperada")
	}
}

// El repositorio por defecto es el primero por nombre, y cambiar de repo cambia
// las filas: el rail no es decorativo.
func TestRepoSelection(t *testing.T) {
	h := newHandler(t, corpus(t))

	def := body(t, h, "/")
	if !strings.Contains(def, "log_error ignores --quiet flag") {
		t.Error("el repositorio por defecto no es el primero del rail")
	}

	other := body(t, h, "/?repo="+url.QueryEscape("spf13/cobra"))
	if !strings.Contains(other, "404 links on cobra.dev") {
		t.Error("no se pudo seleccionar el repositorio por su nombre")
	}
	if strings.Contains(other, "log_error ignores --quiet flag") {
		t.Error("la página mezcla filas de dos repositorios")
	}
}

// El estado de una fila solo afirma hechos medidos. Nada de veredictos inventados.
func TestRowStateOnlyAssertsMeasuredFacts(t *testing.T) {
	page := body(t, newHandler(t, corpus(t)), "/?repo=spf13__cobra")

	if !strings.Contains(page, "#1984") {
		t.Error("la issue con PR abierta no muestra el número de PR")
	}
	if !strings.Contains(page, ">tomada<") {
		t.Error("la issue con PR abierta no figura como tomada")
	}
	if !strings.Contains(page, ">sin triajar<") {
		t.Error("una issue sin triaje debería decirlo, no inventar un veredicto")
	}
	for _, invented := range []string{">candidata<", ">no es trabajo<", ">condicionada<"} {
		if strings.Contains(page, invented) {
			t.Errorf("la fila afirma %s sin que el triaje haya corrido", invented)
		}
	}
}

func TestFilterByState(t *testing.T) {
	h := newHandler(t, corpus(t))

	taken := body(t, h, "/?repo=spf13__cobra&state=taken")
	if !strings.Contains(taken, "404 links on cobra.dev") {
		t.Error("state=taken no devolvió la issue con PR")
	}
	if strings.Contains(taken, "YAML docs ignore linkHandler") {
		t.Error("state=taken devolvió una issue sin PR")
	}

	free := body(t, h, "/?repo=spf13__cobra&state=free")
	if !strings.Contains(free, "YAML docs ignore linkHandler") {
		t.Error("state=free no devolvió la issue libre")
	}
	if strings.Contains(free, "404 links on cobra.dev") {
		t.Error("state=free devolvió la issue con PR")
	}
}

func TestFilterByLabelAndNoLabel(t *testing.T) {
	h := newHandler(t, corpus(t))

	byLabel := body(t, h, "/?repo=spf13__cobra&label=good-first-issue")
	if !strings.Contains(byLabel, "404 links on cobra.dev") {
		t.Error("el filtro por etiqueta no devolvió la issue que la lleva")
	}
	if strings.Contains(byLabel, "Question about custom templates") {
		t.Error("el filtro por etiqueta devolvió una issue que no la lleva")
	}

	noLabel := body(t, h, "/?repo=spf13__cobra&nolabel=1")
	if !strings.Contains(noLabel, "YAML docs ignore linkHandler") {
		t.Error("nolabel no devolvió la issue sin etiquetas")
	}
	if strings.Contains(noLabel, "404 links on cobra.dev") {
		t.Error("nolabel devolvió una issue con etiquetas")
	}
}

func TestFilterByQuery(t *testing.T) {
	h := newHandler(t, corpus(t))

	hits := body(t, h, "/?repo=spf13__cobra&q=template")
	if !strings.Contains(hits, "Question about custom templates") {
		t.Error("la búsqueda no encontró la issue por su título")
	}
	if strings.Contains(hits, "404 links on cobra.dev") {
		t.Error("la búsqueda devolvió una issue que no menciona el término")
	}

	// La búsqueda mira el cuerpo, no solo el título.
	if inBody := body(t, h, "/?repo=spf13__cobra&q=link%20handler"); !strings.Contains(inBody, "YAML docs ignore linkHandler") {
		t.Error("la búsqueda no miró el cuerpo de la issue")
	}
}

// Un filtro que no deja nada lo dice, en vez de mostrar una tabla vacía sin explicación.
func TestFilterWithNoMatchesSaysSo(t *testing.T) {
	page := body(t, newHandler(t, corpus(t)), "/?repo=spf13__cobra&q=zzzznoexiste")
	if !strings.Contains(page, "Ninguna issue coincide") {
		t.Error("el resultado vacío del filtro no se explica")
	}
}

// Un repositorio registrado sin corpus es un fetch pendiente, no un error.
func TestRepoWithoutCorpus(t *testing.T) {
	rec := get(t, newHandler(t, corpus(t)), "/?repo=zeta__empty")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "está vacío") {
		t.Error("el repositorio sin corpus no lo dice")
	}
}

// Un repo que no está en el corpus no se sustituye por otro en silencio.
func TestUnknownRepoIsNotFound(t *testing.T) {
	rec := get(t, newHandler(t, corpus(t)), "/?repo=no__existe")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if !strings.Contains(rec.Body.String(), "gfi fetch") {
		t.Error("el 404 no dice cómo traer ese repositorio")
	}
}

// html/template escapa el contenido del repositorio: un título con marcado no
// se ejecuta en la página. El repositorio es evidencia no confiable (I3).
func TestTitlesAreEscaped(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	if err := st.SaveRepo(store.RepoEntry{Name: "x/y", Key: "x__y", TotalOpen: 1, Issues: 1}); err != nil {
		t.Fatalf("SaveRepo() error = %v", err)
	}
	snap := &ghdata.Snapshot{
		Repository: ghdata.Repository{NameWithOwner: "x/y"},
		Issues: []ghdata.Issue{{
			Number: 1,
			Title:  `<script>alert(1)</script>`,
			Labels: []string{`<img src=x onerror=alert(1)>`},
		}},
	}
	if err := st.SaveSnapshot("x__y", snap); err != nil {
		t.Fatalf("SaveSnapshot() error = %v", err)
	}

	page := body(t, newHandler(t, st), "/?repo=x__y")
	if strings.Contains(page, "<script>alert(1)</script>") {
		t.Error("el título se renderizó sin escapar")
	}
	if strings.Contains(page, "<img src=x onerror") {
		t.Error("la etiqueta se renderizó sin escapar")
	}
	if !strings.Contains(page, "&lt;script&gt;") {
		t.Error("se esperaba el título escapado en la página")
	}
}

// Los activos siguen saliendo del binario, ahora también el JS.
func TestStaticServesAssets(t *testing.T) {
	h := newHandler(t, nil)

	for _, tc := range []struct{ path, ctype string }{
		{"/static/app.css", "text/css"},
		{"/static/app.js", "javascript"},
	} {
		rec := get(t, h, tc.path)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want %d", tc.path, rec.Code, http.StatusOK)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, tc.ctype) {
			t.Errorf("GET %s Content-Type = %q, want %q", tc.path, ct, tc.ctype)
		}
		if rec.Body.Len() == 0 {
			t.Errorf("GET %s devolvió un cuerpo vacío", tc.path)
		}
	}
}

// Una ruta desconocida no puede caer en el index.
func TestUnknownRouteIs404(t *testing.T) {
	if rec := get(t, newHandler(t, nil), "/no-existe"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /no-existe status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
