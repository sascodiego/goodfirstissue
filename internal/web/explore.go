package web

// La cola de análisis del corpus.
//
// La cola vive en el proceso del servidor, no en la petición: navegar a otra pantalla no
// cambia nada, volver muestra el estado tal como está, y el tope de corridas en paralelo lo
// decide Go en vez de la interfaz. La página solamente renderiza lo que la cola sabe.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/explore"
	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// Las etapas de la escalera. Hoy solo explore está cableada: el triaje llega con #11 y el
// encaje con #13, y hasta entonces la fila los muestra como «no aplica» en vez de inventar
// un estado (I2). La identidad de un job es (repo, issue, etapa), así que agregar una etapa
// es agregar un nombre acá y una entrada en el escalón, no reescribir la cola.
const (
	stageExplore = "explore"
	stageTriage  = "triage"
	stageFit     = "fit"
)

// Los estados de un job. Ninguno es un veredicto sobre la issue: son hechos sobre lo que
// este proceso hizo con ella.
const (
	jobQueued   = "queued"
	jobRunning  = "running"
	jobReady    = "ready"
	jobReused   = "reused"
	jobError    = "error"
	jobCanceled = "canceled"
)

// stageNone marca una etapa que todavía no existe: la fila dice «no aplica» en vez de
// mostrar un estado que nadie produjo.
const stageNone = "none"

// Los textos que la fila muestra por etapa. Viven acá, junto al estado que nombran, para
// que la plantilla no tenga que traducir códigos ni inventar un fallback.
const (
	stateTextNotApplicable = "no aplica"
	stateTextUntouched     = "sin analizar"
)

// ladderStages es el orden en el que la tabla pinta las etapas, de la primera a la última.
var ladderStages = []string{stageExplore, stageTriage, stageFit}

// exploreMaxIssues es el techo de un POST (S14): cada issue es una corrida de pig, y una
// lista de mil no entra en la demo ni en la suscripción.
const exploreMaxIssues = 50

// defaultExploreWorkers es el paralelismo de la cola cuando nadie lo configura. El techo
// coincide con config.MaxExploreWorkers: un worker es un proceso pig.
const (
	defaultExploreWorkers = 3
	exploreWorkersCap     = 3
)

// StageRequest es una etapa concreta de una fila concreta. Todo lo que no está aquí lo
// resuelve el runner desde el store, con las mismas reglas que usa el CLI.
type StageRequest struct {
	// Repo es la clave del store del corpus, e.g. spf13__cobra.
	Repo  string
	Issue int
	Stage string
}

// StageOutcome es lo que una corrida terminada aporta al estado de la fila.
type StageOutcome struct {
	// Reused is true when the journal already answered this exact question, and At is
	// when that answer was produced: a reused result is announced with its age, because
	// knowing when it was produced is part of trusting it (I14).
	Reused bool
	At     time.Time
	// Latency is how long the finished run took, or how long the reused one did.
	Latency time.Duration
}

// StageRunner runs one stage. It is injected through Options: no test runs pig, touches
// the network or needs a clone. progress reports how long the agent's pipe has been
// silent, which is what the running row shows (S10).
type StageRunner func(ctx context.Context, req StageRequest, progress func(silence time.Duration)) (StageOutcome, error)

// ExploreOptions is what the real stage needs to run the agent. It is copied out of the
// configuration at startup and never looked up again: an empty Model means the queue is not
// configured, and the POST refuses instead of starting nothing.
type ExploreOptions struct {
	Model  string
	Models []string
	Tools  []string
	Home   string
	Binary string

	IdleWarn time.Duration
	IdleKill time.Duration
	Budget   time.Duration
}

// jobKey identifica una etapa de una fila. Agregar triaje o encaje no cambia la forma de
// esta clave: cambia el tercer campo.
type jobKey struct {
	Repo  string
	Issue int
	Stage string
}

// job es lo que la cola sabe de una etapa de una fila. Guarda cuándo empezó el estado
// actual, cuándo el agente produjo la última línea (para el silencio) y, cuando terminó, la
// latencia de la corrida o la fecha del análisis que se reutilizó.
type job struct {
	key   jobKey
	state string
	// note explica un error o un motivo: la fila lo muestra en vez de adivinar.
	note string
	// at is when the current state started.
	at time.Time
	// lastOutput is the last moment the agent produced output. The running row shows how
	// long ago that was, which is silence by pipe and not by clock (S10).
	lastOutput time.Time
	latency    time.Duration
	reusedAt   time.Time

	ctx    context.Context
	cancel context.CancelFunc
}

func (j *job) request() StageRequest {
	return StageRequest{Repo: j.key.Repo, Issue: j.key.Issue, Stage: j.key.Stage}
}

// stageView es lo que la página ve de una etapa de una fila. Nada de esto es un veredicto
// sobre la issue: es lo que la cola hizo con ella.
type stageView struct {
	Stage string
	State string
	// StateText is the phrase the row shows: en cola, en curso, listo, reutilizado,
	// cancelado, error, no aplica.
	StateText string
	// Silence is how many seconds the agent's pipe has been silent while running.
	Silence int
	// Duration is how long the finished run took.
	Duration string
	// Age and At say how old a reused analysis is, and when it was produced.
	Age string
	At  string
	// Note is the free text that explains an error or a reason.
	Note string
}

// queue is a FIFO of jobs with a fixed number of runners.
//
// One dispatcher goroutine owns the pending list and starts at most workers jobs at once; a
// kick signal is enough to wake it. There is no channel per job: the state lives under the
// mutex and the page reads it, which is what makes the queue survive navigation.
type queue struct {
	mu      sync.Mutex
	stage   StageRunner
	workers int
	jobs    map[jobKey]*job
	pending []jobKey
	kick    chan struct{}
	running int
	base    context.Context
}

func newQueue(stage StageRunner, workers int) *queue {
	if workers <= 0 {
		workers = defaultExploreWorkers
	}
	if workers > exploreWorkersCap {
		workers = exploreWorkersCap
	}
	return &queue{
		stage:   stage,
		workers: workers,
		jobs:    make(map[jobKey]*job),
		kick:    make(chan struct{}, 1),
		base:    context.Background(),
	}
}

// start launches the single dispatcher. Todo el ciclo de vida de los jobs cuelga de él.
func (q *queue) start() {
	go q.dispatch()
}

// kickOnce despierta al despachador sin esperar turno: una señal ya pendiente es suficiente.
func (q *queue) kickOnce() {
	select {
	case q.kick <- struct{}{}:
	default:
	}
}

func (q *queue) dispatch() {
	for range q.kick {
		for {
			j := q.next()
			if j == nil {
				break
			}
			go q.work(j)
		}
	}
}

// next takes the first queued job that fits in the parallelism budget and marks it running.
func (q *queue) next() *job {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.pending) > 0 && q.running < q.workers {
		key := q.pending[0]
		q.pending = q.pending[1:]

		j := q.jobs[key]
		if j == nil || j.state != jobQueued {
			continue
		}
		ctx, cancel := context.WithCancel(q.base)
		now := time.Now()
		j.state = jobRunning
		j.at = now
		j.lastOutput = now
		j.ctx = ctx
		j.cancel = cancel
		q.running++
		return j
	}
	return nil
}

// work runs one job to its end. La corrida no escribe en la página: escribe en el job, y la
// próxima lectura de la página muestra el resultado.
func (q *queue) work(j *job) {
	defer func() {
		q.mu.Lock()
		q.running--
		q.mu.Unlock()
		// Al liberar un lugar puede haber algo esperando hace rato.
		q.kickOnce()
	}()

	progress := func(silence time.Duration) {
		q.mu.Lock()
		if j.state == jobRunning {
			j.lastOutput = time.Now().Add(-silence)
		}
		q.mu.Unlock()
	}

	stage := q.stage
	if stage == nil {
		q.finish(j, StageOutcome{}, errors.New("la cola no tiene runner configurado"))
		return
	}

	var (
		out StageOutcome
		err error
	)
	if j.ctx != nil {
		out, err = stage(j.ctx, j.request(), progress)
	} else {
		err = errors.New("la cola arrancó un job sin contexto")
	}
	q.finish(j, out, err)
}

// finish writes the end state of a job. Un job cancelado se queda cancelado: nunca se
// reporta como listo, porque la corrida que lo respondía ya no existe.
func (q *queue) finish(j *job, out StageOutcome, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if j.cancel != nil {
		j.cancel()
	}
	if j.state == jobCanceled {
		return
	}

	j.at = time.Now()
	switch {
	case err != nil:
		j.state = jobError
		j.note = err.Error()
	case out.Reused:
		j.state = jobReused
		j.reusedAt = out.At
		j.latency = out.Latency
	default:
		j.state = jobReady
		j.latency = out.Latency
	}
}

// enqueue adds one stage and reports whether it was added. Una issue ya en cola o en curso
// no se duplica (S14): se devuelve false y se deja como está.
func (q *queue) enqueue(repo string, issue int, stage string) bool {
	if q == nil {
		return false
	}

	q.mu.Lock()
	key := jobKey{Repo: repo, Issue: issue, Stage: stage}
	if current, ok := q.jobs[key]; ok && (current.state == jobQueued || current.state == jobRunning) {
		q.mu.Unlock()
		return false
	}
	q.jobs[key] = &job{key: key, state: jobQueued, at: time.Now()}
	q.pending = append(q.pending, key)
	q.mu.Unlock()

	q.kickOnce()
	return true
}

// fail records a job that never runs and says why. Es lo que usa el POST para explicar
// por qué una issue quedó sin analizar, sin gastar una corrida en averiguarlo.
func (q *queue) fail(repo string, issue int, stage, note string) {
	if q == nil {
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	key := jobKey{Repo: repo, Issue: issue, Stage: stage}
	if current, ok := q.jobs[key]; ok && (current.state == jobQueued || current.state == jobRunning) {
		// Lo que ya está en marcha sabe más que un rechazo posterior: no se pisa.
		return
	}
	q.jobs[key] = &job{key: key, state: jobError, note: note, at: time.Now()}
}

// cancel stops one row's stage: la marca cancelada, saca de la cola lo pendiente y corta el
// contexto de lo que corre.
//
// El contexto es el mismo que `pig.Run` deriva para `exec.CommandContext`: cancelarlo mata
// el proceso hijo, y `explore.Session.Analyze` ya no escribe en el journal cuando su ctx
// está cancelado, así que acá no se reimplementa nada de eso.
func (q *queue) cancel(repo string, issue int) bool {
	if q == nil {
		return false
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	canceled := false
	for key := range q.jobs {
		if key.Repo != repo || key.Issue != issue {
			continue
		}
		if q.cancelLocked(q.jobs[key]) {
			canceled = true
		}
	}
	return canceled
}

// cancelLocked marca cancelado lo que todavía se puede detener. Un job ya terminado no se
// reescribe: lo cancelado es lo que estaba en la cola o en curso.
func (q *queue) cancelLocked(j *job) bool {
	switch j.state {
	case jobQueued:
		q.dropPendingLocked(j.key)
	case jobRunning:
		if j.cancel != nil {
			j.cancel()
		}
	default:
		return false
	}
	j.state = jobCanceled
	j.note = "cancelado"
	j.at = time.Now()
	return true
}

// dropPendingLocked quita una clave de la cola de espera. El trabajo en curso no está en
// esa lista, así que vaciar nunca lo toca.
func (q *queue) dropPendingLocked(key jobKey) {
	kept := q.pending[:0]
	for _, pending := range q.pending {
		if pending != key {
			kept = append(kept, pending)
		}
	}
	q.pending = kept
}

// drain descarta lo que espera en un repositorio, deja quieto lo que corre, y devuelve
// cuántos jobs sacó (S9).
func (q *queue) drain(repo string) int {
	if q == nil {
		return 0
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	dropped := 0
	for _, key := range append([]jobKey(nil), q.pending...) {
		if key.Repo != repo {
			continue
		}
		j := q.jobs[key]
		if j == nil || j.state != jobQueued {
			continue
		}
		q.dropPendingLocked(key)
		j.state = jobCanceled
		j.note = "sacado de la cola"
		j.at = time.Now()
		dropped++
	}
	return dropped
}

// cancelAll vacía la cola de un repositorio y corta lo que está en curso: cancelar una
// corrida es cortarle el proceso, no esperar a que termine. Devuelve cuántos jobs detuvo.
func (q *queue) cancelAll(repo string) int {
	if q == nil {
		return 0
	}

	stopped := q.drain(repo)

	q.mu.Lock()
	defer q.mu.Unlock()
	for key, j := range q.jobs {
		if key.Repo != repo || j.state != jobRunning {
			continue
		}
		if j.cancel != nil {
			j.cancel()
		}
		j.state = jobCanceled
		j.note = "cancelado"
		j.at = time.Now()
		stopped++
	}
	return stopped
}

// ladder returns the stage views of one row, in the order the table paints them. Una
// escalera nil es válida: la pantalla de issue arma el rail sin cola y las etapas dicen
// «sin analizar» en vez de fallar.
func (q *queue) ladder(repo string, issue int) []stageView {
	out := make([]stageView, 0, len(ladderStages))
	for _, stage := range ladderStages {
		out = append(out, q.stageFor(repo, issue, stage))
	}
	return out
}

// stageFor reads one rung. Las etapas que todavía no existen dicen «no aplica»: no hay
// runner, no hay resultado y no hay nada que afirmar (I2, S12).
func (q *queue) stageFor(repo string, issue int, stage string) stageView {
	v := stageView{Stage: stage}

	if stage != stageExplore {
		v.State = stageNone
		v.StateText = stateTextNotApplicable
		return v
	}
	if q == nil {
		v.StateText = stateTextUntouched
		return v
	}

	q.mu.Lock()
	j := q.jobs[jobKey{Repo: repo, Issue: issue, Stage: stage}]
	if j == nil {
		q.mu.Unlock()
		v.StateText = stateTextUntouched
		return v
	}
	state, note := j.state, j.note
	lastOutput, latency, reusedAt := j.lastOutput, j.latency, j.reusedAt
	q.mu.Unlock()

	now := time.Now()
	v.State = state
	v.Note = note
	switch state {
	case jobQueued:
		v.StateText = "en cola"
	case jobRunning:
		v.StateText = "en curso"
		// El silencio se mide por el pipe, no por reloj (S10): son los segundos que hace
		// que el agente no produce una línea.
		if seconds := int(now.Sub(lastOutput).Seconds()); seconds > 0 {
			v.Silence = seconds
		}
	case jobReady:
		v.StateText = "listo"
		v.Duration = formatLatency(latency)
	case jobReused:
		v.StateText = "reutilizado"
		v.Age = ageText(now.Sub(reusedAt))
		v.At = formatDate(reusedAt)
	case jobError:
		v.StateText = "error"
	case jobCanceled:
		v.StateText = "cancelado"
	}
	return v
}

// formatLatency is how long a finished run took, in the unit a person reads.
func formatLatency(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	return d.Round(time.Millisecond).String()
}

// ageText says how old a reused analysis is. Una fecha absoluta sola obliga a hacer la
// cuenta; la edad sola pierde cuándo se produjo, por eso la fila lleva las dos.
func ageText(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "hace un momento"
	case d < time.Hour:
		return fmt.Sprintf("hace %d min", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("hace %d h", int(d.Hours()))
	default:
		return fmt.Sprintf("hace %d días", int(d.Hours()/24))
	}
}

// exploreSelected atiende POST /explore: encola el análisis de las issues seleccionadas.
//
// El orden de los controles importa: primero la cola, después el repositorio, después la
// entrada, y recién al final lo que se puede saber issue por issue. Cada rechazo deja el
// motivo en la fila correspondiente en vez de mandar al usuario a leer un log.
func (h *Handler) exploreSelected(w http.ResponseWriter, r *http.Request) {
	if rejectCrossSite(w, r) {
		return
	}
	if h.queue == nil {
		http.Error(w, "la cola de análisis no está disponible: este binario arrancó sin cola",
			http.StatusServiceUnavailable)
		return
	}

	key := strings.TrimSpace(r.PostFormValue("repo"))
	entry, ok, err := h.repoEntry(key)
	if err != nil {
		slog.Error("leer el registro", "key", key, "err", err)
		http.Error(w, "no se pudo leer el corpus", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "ese repositorio no está en el corpus; agregalo primero", http.StatusNotFound)
		return
	}

	numbers, err := parseIssueNumbers(r.PostForm["issue"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// El tope se mira antes de gastar cualquier otra comprobación: 51 issues no se encolan
	// ni se analizan, y la pantalla lo dice con cuántas se pidieron (S14).
	if len(numbers) > exploreMaxIssues {
		redirectBack(w, r, entry.Key, "limite", len(numbers))
		return
	}

	// Sin modelo la cola no está configurada: se rechaza con el motivo por fila en vez de
	// encolar algo que no puede correr (§17.7).
	if h.explore.Model == "" {
		h.refuseAll(entry.Key, numbers, "no hay modelo configurado para el análisis: poné explore.model en config.json")
		redirectBack(w, r, entry.Key, "analizando", 0)
		return
	}

	// Y sin clave, la corrida fallaría seguro. Se dice antes de gastarla (S7).
	if h.getenv == nil || h.getenv("NAN_KEY") == "" {
		h.refuseAll(entry.Key, numbers, "falta NAN_KEY: exportá la clave de NaN y volvé a intentar")
		redirectBack(w, r, entry.Key, "analizando", 0)
		return
	}

	snap, err := snapshotOf(h.store, entry.Key)
	if err != nil {
		slog.Error("leer el corpus", "key", entry.Key, "err", err)
		http.Error(w, "no se pudo leer el corpus", http.StatusInternalServerError)
		return
	}

	queued := 0
	for _, number := range numbers {
		if note := h.exploreBlocker(entry, snap, number); note != "" {
			h.queue.fail(entry.Key, number, stageExplore, note)
			continue
		}
		if h.queue.enqueue(entry.Key, number, stageExplore) {
			queued++
		}
	}

	// Vuelve enseguida: la corrida sigue en la cola, y la pantalla la muestra al pedirla.
	redirectBack(w, r, entry.Key, "analizando", queued)
}

// exploreCancel atiende POST /explore/cancel: detiene una etapa de una fila, esté esperando
// o corriendo.
func (h *Handler) exploreCancel(w http.ResponseWriter, r *http.Request) {
	if rejectCrossSite(w, r) {
		return
	}
	if h.queue == nil {
		http.Error(w, "la cola de análisis no está disponible: este binario arrancó sin cola",
			http.StatusServiceUnavailable)
		return
	}

	key := strings.TrimSpace(r.PostFormValue("repo"))
	entry, ok, err := h.repoEntry(key)
	if err != nil {
		slog.Error("leer el registro", "key", key, "err", err)
		http.Error(w, "no se pudo leer el corpus", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "ese repositorio no está en el corpus; agregalo primero", http.StatusNotFound)
		return
	}

	number, err := parseIssueNumber(r.PostFormValue("issue"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.queue.cancel(entry.Key, number)

	redirectBack(w, r, entry.Key, "cancelado", -1)
}

// exploreClear atiende POST /explore/clear: descarta lo que espera y deja quieto lo que
// corre, que es justo lo que «vaciar la cola» quiere decir (S9).
func (h *Handler) exploreClear(w http.ResponseWriter, r *http.Request) {
	if rejectCrossSite(w, r) {
		return
	}
	if h.queue == nil {
		http.Error(w, "la cola de análisis no está disponible: este binario arrancó sin cola",
			http.StatusServiceUnavailable)
		return
	}

	key := strings.TrimSpace(r.PostFormValue("repo"))
	entry, ok, err := h.repoEntry(key)
	if err != nil {
		slog.Error("leer el registro", "key", key, "err", err)
		http.Error(w, "no se pudo leer el corpus", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "ese repositorio no está en el corpus; agregalo primero", http.StatusNotFound)
		return
	}

	redirectBack(w, r, entry.Key, "vaciada", h.queue.drain(entry.Key))
}

// exploreCancelAll atiende POST /explore/cancel-all: cancela lo que corre y vacía lo que
// espera, para el repositorio del formulario y para ningún otro.
func (h *Handler) exploreCancelAll(w http.ResponseWriter, r *http.Request) {
	if rejectCrossSite(w, r) {
		return
	}
	if h.queue == nil {
		http.Error(w, "la cola de análisis no está disponible: este binario arrancó sin cola",
			http.StatusServiceUnavailable)
		return
	}

	key := strings.TrimSpace(r.PostFormValue("repo"))
	entry, ok, err := h.repoEntry(key)
	if err != nil {
		slog.Error("leer el registro", "key", key, "err", err)
		http.Error(w, "no se pudo leer el corpus", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "ese repositorio no está en el corpus; agregalo primero", http.StatusNotFound)
		return
	}

	redirectBack(w, r, entry.Key, "cancelado", h.queue.cancelAll(entry.Key))
}

// exploreRows atiende GET /explore/rows: vuelve a renderizar el cuerpo de la tabla con los
// mismos datos que la pantalla del corpus, para que el navegador cambie el <tbody> sin
// inventar un contrato JSON que después haya que mantener.
func (h *Handler) exploreRows(w http.ResponseWriter, r *http.Request) {
	data, err := buildPageWith(h.store, r.URL.Query(), h.queue)
	switch {
	case errors.Is(err, errUnknownRepo):
		http.Error(w, "ese repositorio no está en el corpus; traelo con: gfi fetch owner/nombre", http.StatusNotFound)
		return
	case err != nil:
		slog.Error("armar las filas", "err", err)
		http.Error(w, "no se pudo leer el corpus", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Solo las filas. La plantilla «rows» llega con la tabla en vivo; hasta entonces esta
	// ruta responde igual y deja el error en el log, en vez de faltar del mux.
	if err := h.screens[screenDashboard].ExecuteTemplate(w, "rows", data); err != nil {
		slog.Error("render filas", "err", err)
	}
}

// refuseAll deja un job en error por issue, sin runner: la fila explica el motivo y nada se
// gasta en averiguarlo.
func (h *Handler) refuseAll(repo string, numbers []int, note string) {
	for _, number := range numbers {
		h.queue.fail(repo, number, stageExplore, note)
	}
}

// exploreBlocker devuelve el motivo por el que esa issue no se puede encolar hoy, y "" cundo
// se puede. Los dos motivos que importan nombran el comando que los arregla (S13).
func (h *Handler) exploreBlocker(entry store.RepoEntry, snap *ghdata.Snapshot, number int) string {
	if snap == nil {
		return fmt.Sprintf("el corpus de %s todavía no está: corré gfi fetch %s", entry.Name, entry.Name)
	}
	issue, ok := issueIn(snap, number)
	if !ok {
		return fmt.Sprintf("la issue #%d no está en el corpus guardado: corré gfi fetch %s", number, entry.Name)
	}

	in := explore.Input{
		Repository:    entry.Key,
		NameWithOwner: snap.Repository.NameWithOwner,
		CommitSHA:     snap.Repository.CommitSHA,
		Issue:         issue,
	}
	_, err := explore.IndexFor(h.store, in)
	switch {
	case err == nil:
		return ""
	case errors.Is(err, explore.ErrNoIndex):
		return fmt.Sprintf("no hay índice del código: corré gfi index %s", entry.Name)
	}
	var stale *explore.StaleIndexError
	if errors.As(err, &stale) {
		return fmt.Sprintf("el índice es de otro commit (grafo %s · corpus %s): corré gfi index %s",
			shortSHA(stale.Index), shortSHA(stale.Corpus), entry.Name)
	}
	slog.Error("leer el índice", "key", entry.Key, "err", err)
	return fmt.Sprintf("no se pudo leer el índice: %v", err)
}

// stageRunner is the real stage: it resolves the corpus and the issue from the store, pins
// the code graph to the corpus commit and runs one analysis with the same session the CLI
// uses, so a result produced by the screen is reusable by `gfi explore` and back.
//
// Nada de este archivo decide qué mirar: el agente recibe el texto de la issue y su
// allowlist, igual que en el CLI (I8).
func (h *Handler) stageRunner(ctx context.Context, req StageRequest, progress func(time.Duration)) (StageOutcome, error) {
	if req.Stage != stageExplore {
		return StageOutcome{}, fmt.Errorf("la etapa %q todavía no existe: solo %s corre hoy", req.Stage, stageExplore)
	}
	if h.store == nil {
		return StageOutcome{}, errors.New("no hay store: no se puede resolver el corpus")
	}
	if h.explore.Model == "" {
		return StageOutcome{}, errors.New("no hay modelo configurado para el análisis: poné explore.model en config.json")
	}

	entry, ok, err := h.repoEntry(req.Repo)
	if err != nil {
		return StageOutcome{}, err
	}
	if !ok {
		return StageOutcome{}, fmt.Errorf("el repositorio %s no está en el corpus: corré gfi fetch", req.Repo)
	}

	snap, err := h.store.LoadSnapshot(entry.Key)
	if err != nil {
		return StageOutcome{}, fmt.Errorf("no se pudo leer el corpus de %s: corré gfi fetch %s: %w", entry.Name, entry.Name, err)
	}
	issue, ok := issueIn(snap, req.Issue)
	if !ok {
		return StageOutcome{}, fmt.Errorf("la issue #%d no está en el corpus de %s: corré gfi fetch %s",
			req.Issue, entry.Name, entry.Name)
	}

	in := explore.Input{
		Repository:    entry.Key,
		NameWithOwner: snap.Repository.NameWithOwner,
		CommitSHA:     snap.Repository.CommitSHA,
		Issue:         issue,
	}
	rec, err := explore.IndexFor(h.store, in)
	switch {
	case errors.Is(err, explore.ErrNoIndex):
		return StageOutcome{}, fmt.Errorf("no hay índice del código: corré gfi index %s", entry.Name)
	case err != nil:
		var stale *explore.StaleIndexError
		if errors.As(err, &stale) {
			return StageOutcome{}, fmt.Errorf("el índice es de otro commit (grafo %s · corpus %s): corré gfi index %s",
				shortSHA(stale.Index), shortSHA(stale.Corpus), entry.Name)
		}
		return StageOutcome{}, err
	}
	// El agente lee el árbol que el índice leyó, y el registro se pinnea al commit del
	// corpus: un resultado nunca se firma con código que nadie abrió (I4).
	in.Dir = rec.Path

	sess := &explore.Session{
		Store:     h.store,
		Model:     h.explore.Model,
		Models:    h.explore.Models,
		Tools:     h.explore.Tools,
		Home:      h.explore.Home,
		Binary:    h.explore.Binary,
		IdleWarn:  h.explore.IdleWarn,
		IdleKill:  h.explore.IdleKill,
		Budget:    h.explore.Budget,
		OnSilence: progress,
	}

	res, err := sess.Analyze(ctx, in)
	if res == nil {
		return StageOutcome{}, err
	}
	if res.Reused {
		out := StageOutcome{Reused: true}
		if res.Previous != nil {
			out.At = res.Previous.At
			out.Latency = time.Duration(res.Previous.LatencyMs) * time.Millisecond
		}
		return out, nil
	}

	record := res.Record
	latency := time.Duration(record.Metrics.TotalMs) * time.Millisecond
	outcome := StageOutcome{Latency: latency}

	// El registro quedó guardado igual: la corrida existió y sus métricas son evidencia
	// (I10). Lo que no existe es un resultado que la fila pueda llamar «listo».
	switch {
	case record.Metrics.Stalled:
		return outcome, errors.New("el agente se quedó en silencio y se cortó (stalled); el registro quedó guardado")
	case record.Metrics.TimedOut:
		return outcome, errors.New("el agente agotó el presupuesto de tiempo (timed out); el registro quedó guardado")
	case record.Evidence == nil:
		if err == nil {
			return outcome, errors.New("la respuesta del agente no se pudo interpretar (unparseable answer); el registro quedó guardado")
		}
		return outcome, fmt.Errorf("la respuesta del agente no se pudo interpretar (unparseable answer); el registro quedó guardado: %w", err)
	case !record.Metrics.Complete:
		code := 0
		if res.Outcome != nil {
			code = res.Outcome.ExitCode
		}
		return outcome, fmt.Errorf("el agente no terminó completo (exit %d); el registro quedó guardado", code)
	}
	if err := ctx.Err(); err != nil {
		return StageOutcome{}, err
	}
	return outcome, nil
}

// repoEntry resolves a store key the same way the screens do, so a POST cannot act on a
// repository the page would not show.
func (h *Handler) repoEntry(key string) (store.RepoEntry, bool, error) {
	if h.store == nil || key == "" {
		return store.RepoEntry{}, false, nil
	}
	return h.store.Repo(key)
}

// issueIn finds one issue in the frozen corpus, by number only: numbers are the identity
// GitHub gives them and the identity this queue uses.
func issueIn(snap *ghdata.Snapshot, number int) (ghdata.Issue, bool) {
	for _, issue := range snap.Issues {
		if issue.Number == number {
			return issue, true
		}
	}
	return ghdata.Issue{}, false
}

// parseIssueNumbers reads the repeated «issue» field: numbers only, deduplicated in the
// order they arrived, so selecting the same row twice is one job (S14).
func parseIssueNumbers(raw []string) ([]int, error) {
	seen := make(map[int]bool, len(raw))
	numbers := make([]int, 0, len(raw))
	for _, value := range raw {
		number, err := parseIssueNumber(value)
		if err != nil {
			return nil, err
		}
		if seen[number] {
			continue
		}
		seen[number] = true
		numbers = append(numbers, number)
	}
	if len(numbers) == 0 {
		return nil, errors.New("seleccioná al menos una issue")
	}
	return numbers, nil
}

func parseIssueNumber(raw string) (int, error) {
	number, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("la issue %q no es un número", raw)
	}
	return number, nil
}

// redirectBack closes a POST with a 303 to the repository screen, with the code of the
// message and the count that goes with it. Un n negativo deja el número afuera: cancelar
// una fila no tiene cuántas. Un código desconocido no se muestra (ver estadoText).
func redirectBack(w http.ResponseWriter, r *http.Request, key, estado string, n int) {
	values := url.Values{}
	values.Set("repo", key)
	values.Set("estado", estado)
	if n >= 0 {
		values.Set("n", strconv.Itoa(n))
	}
	http.Redirect(w, r, "/?"+values.Encode(), http.StatusSeeOther)
}

// counts reports how much of one repository is waiting and how much is running. La pantalla
// lo usa para decir cuánto falta, en vez de mostrar la cola como filas sueltas.
func (q *queue) counts(repo string) (queued, running int) {
	if q == nil {
		return 0, 0
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	for key, j := range q.jobs {
		if key.Repo != repo {
			continue
		}
		switch j.state {
		case jobQueued:
			queued++
		case jobRunning:
			running++
		}
	}
	return queued, running
}

// workerCount is how many analyses run at once, so an estimate can divide by it.
func (q *queue) workerCount() int {
	if q == nil || q.workers <= 0 {
		return defaultExploreWorkers
	}
	return q.workers
}

// exploreRunEstimate es el costo medido de un análisis: 1m49.386s en la corrida real de
// spf13__cobra#1272, la que está en el journal. La estimación se arma con ese número medido
// y no con una intuición, porque es lo que la pantalla le promete a quien aprieta el botón.
const exploreRunEstimate = 109 * time.Second

// estimateText says how long the queue ahead of the user is likely to take, dividing by the
// workers that actually run and rounding up: una corrida empezada no se comparte.
func estimateText(queued, running, workers int) string {
	if queued+running == 0 {
		return ""
	}
	if workers <= 0 {
		workers = defaultExploreWorkers
	}
	rounds := (queued + running + workers - 1) / workers
	total := time.Duration(rounds) * exploreRunEstimate
	return "≈ " + total.Round(time.Second).String()
}
