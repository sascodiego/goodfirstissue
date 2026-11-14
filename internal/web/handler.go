package web

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
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

// Fetcher brings one repository's open issues. *ghdata.Client is the real one and the
// signature is its own, so a double needs no adapter: the interface exists to keep the
// tests off the network, not to abstract GitHub away.
type Fetcher interface {
	Fetch(ctx context.Context, owner, name string, opts ghdata.Options) (*ghdata.Snapshot, error)
}

// defaultFetchTimeout bounds one fetch when the caller does not say. The real thing takes
// about eleven seconds per repository; a zero timeout would cancel every request.
const defaultFetchTimeout = 60 * time.Second

// Options are the handler's dependencies. They are passed, never looked up, so a test can
// run the screens with a store in a temporary directory and a fetcher that answers from a
// fixture.
type Options struct {
	Store   *store.Store
	Fetcher Fetcher
	GitHub  ghdata.Options
	Timeout time.Duration
}

// Handler sirve las pantallas y los activos estáticos.
type Handler struct {
	screens map[string]*template.Template
	static  http.Handler
	store   *store.Store
	fetcher Fetcher
	github  ghdata.Options
	timeout time.Duration
}

// NewHandler parsea las plantillas y prepara el servidor de estáticos.
// Un store nil es válido: la pantalla muestra el estado «sin corpus» en vez de
// fallar, que es lo que ve alguien que recién compila el binario. Un fetcher nil
// también: las pantallas se sirven igual y solo el alta responde que no está
// disponible, en vez de tumbar el arranque del binario.
func NewHandler(opts Options) (*Handler, error) {
	screens, err := parseScreens(templateFS, screenNames)
	if err != nil {
		return nil, err
	}
	assets, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultFetchTimeout
	}
	return &Handler{
		screens: screens,
		static:  http.FileServerFS(assets),
		store:   opts.Store,
		fetcher: opts.Fetcher,
		github:  opts.GitHub,
		timeout: timeout,
	}, nil
}

// render ejecuta una pantalla con su layout. Cada una tiene su propio set, así
// que el layout resuelve «content» a la de esta pantalla y a ninguna otra.
func (h *Handler) render(w http.ResponseWriter, screen string, data any) {
	if err := h.screens[screen].ExecuteTemplate(w, "layout", data); err != nil {
		// La cabecera ya salió: solo queda registrarlo.
		slog.Error("render", "pantalla", screen, "err", err)
	}
}
