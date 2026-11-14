package web

import (
	"embed"
	"html/template"
	"io/fs"
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
