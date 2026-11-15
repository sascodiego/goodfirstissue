package web

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/sascodiego/nan-hackathon-clef/internal/config"
	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
	"github.com/sascodiego/nan-hackathon-clef/internal/triage"
)

// Estados de una fila. Los que empiezan con «state» son los que la pantalla puede afirmar
// sin veredicto: «taken» es un hecho medido (hay una PR abierta vinculada) y «pending» es la
// ausencia de triaje. Los demás salen del journal y su clase la decide outcomeClass, que es la
// misma que usa el bloque de la issue: un solo mapeo para las dos pantallas.
//
// Un veredicto solo se muestra si está guardado. Inventarlo acá violaría I2 (nunca un veredicto
// por defecto) e I10 (nunca descartar por falta de prueba).
const (
	stateTaken   = "taken"
	statePending = "pending"
)

// maxLabelOptions acota el desplegable de etiquetas. Cobra tiene decenas de
// etiquetas libres; el filtro es una ayuda, no un catálogo.
const maxLabelOptions = 25

// pageData es lo único que ven las plantillas.
type pageData struct {
	Title   string
	Repos   []repoView
	HasRepo bool
	Active  repoView
	Stats   statsView
	Rows    []rowView
	Labels  []string
	Filter  filter
	Shown   int
	Total   int
	// Estado es la frase que confirma el alta o el refresco. Sale de una lista blanca de
	// códigos, nunca del texto que venga en la URL.
	Estado string
	// EstadoN es el número que acompaña al aviso de la URL: cuántas se encolaron,
	// cuántas se descartaron o cuántas se pidieron. Existe solo cuando el código lo usa.
	EstadoN    int
	HasEstadoN bool
	Commit     string
	Fetched    string
	Language   string
	// La cola de análisis del repositorio activo: cuánto espera, cuánto corre y cuánto se
	// promete que falta. Es estado del proceso, no del corpus.
	Queued   int
	Running  int
	Workers  int
	Estimate string
	// Live es true mientras la cola tiene trabajo: sin JS el layout se refresca por
	// <noscript>, y con JS la tabla se cambia sola.
	Live bool
	// Toggles del filtro, ya resueltos como URL para que funcionen sin JS.
	FreeHref    string
	NoLabelHref string
	// CandidateHref lleva a las issues que el triaje recomendo. Es el filtro que hace util al
	// embudo: dice cuantas son y deja verlas sin buscarlas a mano.
	CandidateHref string
	IsCandidate   bool
	// CanFit dice si este binario puede correr el encaje. Sin eso el boton no se dibuja: un
	// boton que no puede cumplir es peor que su ausencia.
	CanFit bool
	// CanIndex dice si este binario puede construir el grafo de codigo del repositorio.
	CanIndex bool
}

// statsView son las cifras del encabezado. Todas se cuentan sobre el corpus
// guardado, no sobre el filtro activo: el filtro dice qué se mira, no cuánto hay.
//
// Las cinco primeras son hechos del snapshot. Las del embudo son otra pregunta: qué
// decidió el triaje, y salen del journal, no del corpus. Un issue puede estar «libre»
// (no tiene PR) y a la vez «candidata» (el triaje lo recomendó), así que mezclarlas
// en un solo número sería responder dos preguntas con una cifra.
type statsView struct {
	Free           int
	Taken          int
	WithoutLabel   int
	WithoutComment int
	Analyses       int

	// El embudo, en el orden en que el triaje descarta: cuántas llegan, cuántas se
	// marcan con su motivo, cuántas quedan derivadas, cuántas condicionadas y
	// cuántas sobreviven como candidatas. Untriaged no es un veredicto: es la
	// ausencia de uno, y se cuenta aparte para no confundirla con una decisión.
	Marked      int
	Derived     int
	Conditional int
	Candidate   int
	Doubt       int
	Undecided   int
	Untriaged   int
}

type rowView struct {
	Number     int
	Title      string
	URL        string
	Labels     []labelView
	HasLabel   bool
	State      string
	StateText  string
	PR         int
	HasPR      bool
	Updated    string
	Comments   int
	NoComments bool
	// Stages es la escalera de etapas de la fila, la que corre la cola. Hoy solo explore
	// está cableada; las demás dicen «no aplica» en vez de inventar un estado (I2).
	Stages []stageView
}

type labelView struct {
	Text string
	// Hit solo cambia el énfasis visual de la etiqueta que declara dificultad.
	// Es presentación, nunca un filtro ni una señal para ninguna decisión (I6).
	Hit bool
}

// dashboard sirve la pantalla del corpus.
func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	data, err := buildPageWith(h.store, r.URL.Query(), h.queue)
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
	// El boton del encaje solo se dibuja si este binario puede correrlo.
	data.CanFit = h.fit != nil
	data.CanIndex = h.index != nil
	h.render(w, screenDashboard, data)
}

// buildPage arma la pantalla sin la cola: es lo que usan las demás pantallas para heredar
// el mismo rail y el mismo contexto de repositorio.
func buildPage(st *store.Store, q url.Values) (pageData, error) {
	return buildPageWith(st, q, nil)
}

// buildPageWith arma la pantalla del corpus con el estado de la cola.
//
// Un store nil o vacío no es un error: es la app recién instalada, y la pantalla lo dice en
// vez de mentir con ceros. Una cola nil tampoco es un error: cada fila muestra la escalera
// con «sin analizar», igual que un proceso recién arrancado.
func buildPageWith(st *store.Store, q url.Values, jobs *queue) (pageData, error) {
	data := pageData{Title: "GoodFirstIssue"}
	f := parseFilter(q)
	data.Filter = f

	if st == nil {
		return data, nil
	}

	repos, err := st.Repos()
	if err != nil {
		return data, err
	}
	if len(repos) == 0 {
		return data, nil
	}

	active, ok := pickRepo(st, repos, q.Get("repo"))
	if !ok {
		return pageData{}, errUnknownRepo
	}

	for _, e := range repos {
		snap, err := snapshotOf(st, e.Key)
		if err != nil {
			return data, err
		}
		data.Repos = append(data.Repos, repoView{
			Name:     e.Name,
			Key:      e.Key,
			Open:     e.TotalOpen,
			Stored:   e.Issues,
			Free:     countFree(snap),
			Analyses: analysesOf(st, e.Key),
			Active:   e.Key == active.Key,
			Href:     filter{}.url(e.Key),
		})
	}

	data.HasRepo = true
	data.Active = data.Repos[indexOfRepo(data.Repos, active.Key)]
	data.Estado = estadoText(q.Get("estado"))
	// El número viaja en la URL junto al código del aviso. Un valor que no sea un número no
	// se muestra: la pantalla no repite texto que vino de afuera.
	if n, err := strconv.Atoi(q.Get("n")); err == nil && n >= 0 {
		data.EstadoN, data.HasEstadoN = n, true
	}
	data.FreeHref = toggleStateURL(f, active.Key)
	data.NoLabelHref = toggleNoLabelURL(f, active.Key)
	data.IsCandidate = f.State == filterCandidate
	data.CandidateHref = f.withState(filterCandidate).url(active.Key)
	if data.IsCandidate {
		data.CandidateHref = f.withoutState().url(active.Key)
	}
	data.Commit = shortSHA(active.CommitSHA)
	data.Fetched = formatDate(active.FetchedAt)

	snap, err := snapshotOf(st, active.Key)
	if err != nil {
		return data, err
	}
	if snap == nil {
		// Registrado pero sin corpus: un fetch pendiente, no un error.
		return data, nil
	}
	data.Language = snap.Repository.PrimaryLanguage
	data.Total = len(snap.Issues)
	// El journal se lee una vez por página. Un journal ilegible no inventa veredictos: la
	// pantalla cae al hecho del snapshot (hay PR o no) y dice «sin triajar» en el resto, que
	// es exactamente lo que sabe.
	journal, err := st.Decisions(active.Key)
	if err != nil {
		slog.Error("leer el journal del corpus", "key", active.Key, "err", err)
		journal = nil
	}
	data.Stats = countStats(st, active.Key, active.CommitSHA, snap.Issues, journal)
	data.Labels = distinctLabels(snap.Issues)

	for _, is := range snap.Issues {
		if !f.matches(is) {
			continue
		}
		result, hasVerdict := verdictOf(journal, is.Number, active.CommitSHA)
		// El filtro de candidatas se resuelve contra el journal: la issue no lleva la
		// respuesta, el registro si.
		if f.State == filterCandidate && (!hasVerdict || result.Outcome != triage.Candidate) {
			continue
		}
		data.Rows = append(data.Rows, newRow(is, result, hasVerdict))
	}
	data.Shown = len(data.Rows)

	// La escalera de cada fila es lo que la cola sabe; no se guarda en el snapshot porque no
	// es un hecho del corpus sino del proceso.
	for i := range data.Rows {
		data.Rows[i].Stages = jobs.ladder(active.Key, data.Rows[i].Number)
	}
	data.Queued, data.Running = jobs.counts(active.Key)
	data.Workers = jobs.workerCount()
	data.Estimate = estimateText(data.Queued, data.Running, data.Workers)
	data.Live = data.Queued+data.Running > 0

	return data, nil
}

// estadoText traduce el código que deja el POST en la URL. Un valor que no está en la
// lista no se muestra: la pantalla no repite texto que vino de afuera.
func estadoText(code string) string {
	switch code {
	case "alta":
		return "Se agregó"
	case "refresco":
		return "Refrescado"
	case "analizando":
		return "Analizando"
	case "limite":
		return fmt.Sprintf("El tope es %d issues por pedido: no se encoló ninguna", exploreMaxIssues)
	case "cancelado":
		return "Cancelado"
	case "vaciada":
		return "Cola vaciada"
	case "encaje":
		return "Encaje calculado sobre las candidatas que tienen exploración"
	case "encaje-error":
		return "El encaje no pudo correr: el registro quedó como estaba"
	case "encaje-sin-clave":
		return "Este binario no puede encajar: arrancó sin configuración o sin NAN_KEY"
	case "indexado":
		return "Índice construido: el análisis ya puede leer el grafo de código"
	case "index-error":
		return "No se pudo construir el índice: el detalle quedó en el registro"
	case "index-sin-config":
		return "Este binario no puede indexar: arrancó sin configuración del indexador"
	}
	return ""
}

func toggleStateURL(f filter, key string) string {
	if f.State == filterFree {
		return f.withoutState().url(key)
	}
	return f.withState(filterFree).url(key)
}

func toggleNoLabelURL(f filter, key string) string {
	if f.NoLabel {
		return f.withoutNoLabel().url(key)
	}
	return f.withNoLabel().url(key)
}

// verdictOf es la única forma en que esta pantalla se entera de lo que decidió el triaje.
// Le pregunta al paquete triage, que es dueño de la identidad de la pregunta (I4, I14): una
// pantalla que armara su propia clave terminaría preguntando algo que la pasada nunca contestó.
func verdictOf(journal *store.Decisions, number int, commit string) (triage.Result, bool) {
	analysis, _, ok := triage.Recall(journal, number, config.DecisionModel, commit)
	if !ok {
		return triage.Result{}, false
	}
	return analysis.Result, true
}

// observe suma una issue al embudo. Cuenta por desenlace, que es el mismo cajón que imprime el
// resumen del CLI; la fila conserva la palabra fina. Contar por la palabra de la fila haría de
// «tomada» y «marcada» dos cajones que el resumen no tiene, y entonces el embudo diría un número
// que el CLI no puede confirmar.
func (s *statsView) observe(result triage.Result, hasVerdict bool) {
	if !hasVerdict {
		s.Untriaged++
		return
	}
	switch result.Outcome {
	case triage.Marked:
		s.Marked++
	case triage.Derived:
		s.Derived++
	case triage.Conditional:
		s.Conditional++
	case triage.Candidate:
		s.Candidate++
	case triage.Doubt:
		s.Doubt++
	case triage.Undecided:
		s.Undecided++
	}
}

// newRow arma una fila. El veredicto guardado gana sobre el hecho del snapshot: es la decisión,
// con su pin. La columna de la PR es independiente y se muestra igual, porque el número de la PR
// es un hecho del corpus, no un veredicto.
func newRow(is ghdata.Issue, result triage.Result, hasVerdict bool) rowView {
	r := rowView{
		Number:     is.Number,
		Title:      is.Title,
		URL:        is.URL,
		Comments:   is.CommentCount,
		NoComments: is.CommentCount == 0,
		Updated:    formatDate(is.UpdatedAt),
		State:      statePending,
		StateText:  "sin triajar",
		HasLabel:   len(is.Labels) > 0,
	}
	if pr, ok := openPR(is); ok {
		r.State = stateTaken
		r.StateText = "tomada"
		r.PR = pr
		r.HasPR = true
	}
	if hasVerdict {
		r.State = outcomeClass(result.Outcome)
		r.StateText = outcomeLabel(result)
	}
	for _, l := range is.Labels {
		r.Labels = append(r.Labels, labelView{Text: l, Hit: declaresDifficulty(l)})
	}
	return r
}

func countStats(st *store.Store, key, commit string, issues []ghdata.Issue, journal *store.Decisions) statsView {
	var s statsView
	for _, is := range issues {
		if hasOpenPR(is) {
			s.Taken++
		} else {
			s.Free++
		}
		if len(is.Labels) == 0 {
			s.WithoutLabel++
		}
		if is.CommentCount == 0 {
			s.WithoutComment++
		}
		result, hasVerdict := verdictOf(journal, is.Number, commit)
		s.observe(result, hasVerdict)
	}
	s.Analyses = analysesOf(st, key)
	return s
}

func distinctLabels(issues []ghdata.Issue) []string {
	count := map[string]int{}
	for _, is := range issues {
		for _, l := range is.Labels {
			count[l]++
		}
	}
	out := make([]string, 0, len(count))
	for l := range count {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool {
		if count[out[i]] != count[out[j]] {
			return count[out[i]] > count[out[j]]
		}
		return out[i] < out[j]
	})
	if len(out) > maxLabelOptions {
		out = out[:maxLabelOptions]
	}
	return out
}

func hasOpenPR(is ghdata.Issue) bool {
	_, ok := openPR(is)
	return ok
}

func openPR(is ghdata.Issue) (int, bool) {
	for _, pr := range is.LinkedPRs {
		if pr.State == "OPEN" {
			return pr.Number, true
		}
	}
	return 0, false
}

func hasLabel(is ghdata.Issue, want string) bool {
	for _, l := range is.Labels {
		if strings.EqualFold(l, want) {
			return true
		}
	}
	return false
}

// declaresDifficulty marca, solo para la vista, las etiquetas con las que un
// mantenedor declara que el trabajo es para alguien que empieza. No normaliza,
// no filtra y no alimenta ninguna decisión: I6 prohíbe que la etiqueta entre en
// el juicio de encaje, no que la interfaz la destaque.
func declaresDifficulty(label string) bool {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "good first issue", "good-first-issue", "good for beginner",
		"help wanted", "help-wanted", "beginner", "starter", "first timers only":
		return true
	}
	return false
}
