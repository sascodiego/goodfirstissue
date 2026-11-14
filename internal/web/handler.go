package web

import (
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// Los activos viven dentro del binario: un solo archivo, sin archivos sueltos.
// Los parciales van nombrados: si la carpeta desaparece, el build falla en vez
// de arrancar sin los bloques que las pantallas enganchan.
//
//go:embed templates/*.html templates/partials/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// Handler sirve las pantallas y los activos estáticos.
type Handler struct {
	screens map[string]*template.Template
	static  http.Handler
	store   *store.Store
}

// NewHandler parsea las plantillas y prepara el servidor de estáticos.
// Un store nil es válido: la pantalla muestra el estado «sin corpus» en vez de
// fallar, que es lo que ve alguien que recién compila el binario.
func NewHandler(st *store.Store) (*Handler, error) {
	screens, err := parseScreens(templateFS, screenNames)
	if err != nil {
		return nil, err
	}
	assets, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	return &Handler{screens: screens, static: http.FileServerFS(assets), store: st}, nil
}

// render ejecuta una pantalla con su layout. Cada una tiene su propio set, así
// que el layout resuelve «content» a la de esta pantalla y a ninguna otra.
func (h *Handler) render(w http.ResponseWriter, screen string, data any) {
	if err := h.screens[screen].ExecuteTemplate(w, "layout", data); err != nil {
		// La cabecera ya salió: solo queda registrarlo.
		slog.Error("render", "pantalla", screen, "err", err)
	}
}
