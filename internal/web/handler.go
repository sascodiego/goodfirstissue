package web

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/profile"
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
	// Extractor es el trabajo de extracción del CV. Nil significa que este binario arrancó
	// sin agente usable —sin NAN_KEY, sin pig—: la pantalla se sirve igual y solo los POST
	// del perfil lo dicen.
	Extractor *profile.Extractor

	// Stage runs one stage of the explore queue. Tests inject a double here, so no test
	// needs pig, the network or a clone; nil means the real runner built from Explore.
	Stage StageRunner
	// Getenv reads the environment. Tests inject a double; nil means os.Getenv.
	Getenv func(string) string
	// Workers is how many stages run at once. Zero means the default three, and the
	// ceiling stays at three whatever the caller asks for (S3).
	Workers int
	// Explore is what the real stage needs to run the agent: the model, the allowlist and
	// the paths. An empty Model means the queue is not configured.
	Explore ExploreOptions
	// Logger receives the queue's structured lines. Nil means slog.Default(). A test
	// injects one to read what a person would read when debugging.
	Logger *slog.Logger
}

// Handler sirve las pantallas y los activos estáticos.
type Handler struct {
	screens   map[string]*template.Template
	static    http.Handler
	store     *store.Store
	fetcher   Fetcher
	github    ghdata.Options
	timeout   time.Duration
	extractor *profile.Extractor
	// queue is server state, not request state: navigating away does not stop an analysis
	// and coming back shows it as it is.
	queue   *queue
	explore ExploreOptions
	getenv  func(string) string
	log     *slog.Logger
}

// NewHandler parsea las plantillas y prepara el servidor de estáticos, además de la cola
// de análisis y su despachador.
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
	getenv := opts.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	h := &Handler{
		screens:   screens,
		static:    http.FileServerFS(assets),
		store:     opts.Store,
		fetcher:   opts.Fetcher,
		github:    opts.GitHub,
		timeout:   timeout,
		extractor: opts.Extractor,
		explore:   opts.Explore,
		getenv:    getenv,
		log:       logger,
	}

	// Un runner inyectado gana: así un test nunca corre pig. Sin runner inyectado se usa
	// el real, que resuelve el corpus y arranca la sesión de internal/explore.
	stage := opts.Stage
	if stage == nil {
		stage = h.stageRunner
	}
	h.queue = newQueue(stage, opts.Workers, logger)
	h.queue.start()

	return h, nil
}

// render ejecuta una pantalla con su layout. Cada una tiene su propio set, así
// que el layout resuelve «content» a la de esta pantalla y a ninguna otra.
func (h *Handler) render(w http.ResponseWriter, screen string, data any) {
	if err := h.screens[screen].ExecuteTemplate(w, "layout", data); err != nil {
		// La cabecera ya salió: solo queda registrarlo.
		slog.Error("render", "pantalla", screen, "err", err)
	}
}
