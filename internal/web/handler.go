package web

import (
	"embed"
	"errors"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/sascodiego/nan-hackathon-clef/internal/store"
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
	store  *store.Store
}

// NewHandler parsea las plantillas y prepara el servidor de estáticos.
// Un store nil es válido: la pantalla muestra el estado «sin corpus» en vez de
// fallar, que es lo que ve alguien que recién compila el binario.
func NewHandler(st *store.Store) (*Handler, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	assets, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	return &Handler{tmpl: tmpl, static: http.FileServerFS(assets), store: st}, nil
}

// Routes arma el mux. Las rutas usan los patrones con método de Go 1.22.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.index)
	mux.Handle("GET /static/", http.StripPrefix("/static/", h.static))
	return mux
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	data, err := buildPage(h.store, r.URL.Query())
	switch {
	case errors.Is(err, errUnknownRepo):
		// Un repo que no está en el corpus no se sustituye por otro en silencio:
		// el silencio haría creer que se está mirando lo que se pidió.
		http.Error(w, "ese repositorio no está en el corpus; traelo con: gfi fetch owner/nombre", http.StatusNotFound)
		return
	case err != nil:
		slog.Error("armar la página", "err", err)
		http.Error(w, "no se pudo leer el corpus", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		// La cabecera ya salió: solo queda registrarlo.
		slog.Error("render layout", "err", err)
	}
}
