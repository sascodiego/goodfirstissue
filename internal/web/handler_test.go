package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// El servidor tiene que poder levantar y renderizar la cáscara sin configuración.
func TestIndexRendersShell(t *testing.T) {
	h, err := NewHandler()
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html…", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "GoodFirstIssue") {
		t.Errorf("el cuerpo no menciona el producto:\n%s", body)
	}
	if !strings.Contains(body, "/static/app.css") {
		t.Errorf("el shell no enlaza el CSS embebido:\n%s", body)
	}
}

// El CSS tiene que servirse desde el binario, sin archivos sueltos al lado.
func TestStaticServesCSS(t *testing.T) {
	h, err := NewHandler()
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/app.css", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /static/app.css status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", ct)
	}
	if rec.Body.Len() == 0 {
		t.Error("el CSS servido está vacío")
	}
}

// Una ruta desconocida no puede caer en el index.
func TestUnknownRouteIs404(t *testing.T) {
	h, err := NewHandler()
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/no-existe", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /no-existe status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
