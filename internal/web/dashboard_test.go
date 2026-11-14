package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// Sin corpus la pantalla no miente con ceros: explica qué falta.
func TestDashboardWithoutCorpus(t *testing.T) {
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
func TestDashboardWithoutStore(t *testing.T) {
	if rec := get(t, newHandler(t, nil), "/"); rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// El dashboard se arma con el corpus: repos en el rail, cifras y filas reales.
func TestDashboardRendersCorpus(t *testing.T) {
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

// El formulario del rail vive en templates/partials/add-repo.html: si el
// parcial se desengancha, la pantalla pierde el alta y nada más lo nota.
func TestDashboardRendersAddRepoPartial(t *testing.T) {
	page := body(t, newHandler(t, corpus(t)), "/?repo=spf13__cobra")

	for _, want := range []string{
		`<form class="add" aria-label="Agregar repositorio">`,
		`<input id="nuevo"`,
		`gfi fetch owner/nombre`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("el parcial add-repo no está enganchado: falta %q", want)
		}
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
