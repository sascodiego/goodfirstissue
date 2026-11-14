package web

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"
)

// Un set por pantalla: dos pantallas que definen "content" no se pisan, y las
// dos ven los parciales compartidos. Con un único ParseFS la segunda
// reemplazaría el cuerpo de la primera y las dos renderizarían lo mismo.
func TestScreensShareBaseButNotContent(t *testing.T) {
	fsys := fstest.MapFS{
		"templates/layout.html":        {Data: []byte(`{{define "layout"}}[{{template "content" .}}]{{end}}`)},
		"templates/partials/rail.html": {Data: []byte(`{{define "rail"}}rail{{end}}`)},
		"templates/dashboard.html":     {Data: []byte(`{{define "content"}}dashboard({{template "rail"}}){{end}}`)},
		"templates/issue.html":         {Data: []byte(`{{define "content"}}issue({{template "rail"}}){{end}}`)},
	}

	sets, err := parseScreens(fsys, []string{"dashboard", "issue"})
	if err != nil {
		t.Fatalf("parseScreens() error = %v", err)
	}

	for screen, want := range map[string]string{
		"dashboard": "[dashboard(rail)]",
		"issue":     "[issue(rail)]",
	} {
		var buf bytes.Buffer
		if err := sets[screen].ExecuteTemplate(&buf, "layout", nil); err != nil {
			t.Fatalf("render %s error = %v", screen, err)
		}
		if got := buf.String(); got != want {
			t.Errorf("pantalla %s = %q, want %q", screen, got, want)
		}
	}
}

// Una pantalla sin cuerpo no es una pantalla vacía: es un error, y tiene que
// aparecer al arrancar, no en el primer request con la cabecera ya escrita.
func TestScreenMustDefineContent(t *testing.T) {
	fsys := fstest.MapFS{
		"templates/layout.html":        {Data: []byte(`{{define "layout"}}<{{template "content" .}}>{{end}}`)},
		"templates/partials/rail.html": {Data: []byte(`{{define "rail"}}rail{{end}}`)},
		"templates/muda.html":          {Data: []byte(`{{define "otra"}}nada{{end}}`)},
	}

	_, err := parseScreens(fsys, []string{"muda"})
	if err == nil {
		t.Fatal("parseScreens() aceptó una pantalla que no define \"content\"")
	}
	if !strings.Contains(err.Error(), "content") {
		t.Errorf("el error no nombra lo que falta: %v", err)
	}
}
