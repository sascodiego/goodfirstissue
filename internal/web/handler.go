package web

import (
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
)

// Los activos viven dentro del binario: un solo archivo, sin archivos sueltos.
//
//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// Handler sirve las pantallas y los activos estáticos.
type Handler struct {
	tmpl   *template.Template
	static http.Handler
}

// NewHandler parsea las plantillas y prepara el servidor de estáticos.
func NewHandler() (*Handler, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	assets, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	return &Handler{tmpl: tmpl, static: http.FileServerFS(assets)}, nil
}

// Routes arma el mux. Las rutas usan los patrones con método de Go 1.22.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.index)
	mux.Handle("GET /static/", http.StripPrefix("/static/", h.static))
	return mux
}

func (h *Handler) index(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := page{Title: "GoodFirstIssue"}
	if err := h.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		// La cabecera ya salió: solo queda registrarlo.
		slog.Error("render layout", "err", err)
	}
}

type page struct {
	Title string
}
