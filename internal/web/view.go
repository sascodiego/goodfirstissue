package web

import (
	"errors"
	"net/url"
	"sort"
	"strings"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// errUnknownRepo distingue «ese repositorio no está en el corpus» de «no pude
// leerlo». Lo primero es un 404 honesto; lo segundo, un 500.
var errUnknownRepo = errors.New("repositorio desconocido")

// Estados de una fila. Solo dos se pueden afirmar hoy, y ninguna es un veredicto:
// "taken" es un hecho medido (hay una PR abierta vinculada) y "pending" es la
// ausencia de triaje. Los estados que dependen de Clef —no es trabajo,
// condicionada— llegan cuando exista internal/triage; inventarlos acá violaría
// I2 (nunca un veredicto por defecto) e I10 (nunca descartar por falta de prueba).
const (
	stateTaken   = "taken"
	statePending = "pending"
)

const (
	filterAll   = ""
	filterTaken = "taken"
	filterFree  = "free"
)

// maxLabelOptions acota el desplegable de etiquetas. Cobra tiene decenas de
// etiquetas libres; el filtro es una ayuda, no un catálogo.
const maxLabelOptions = 25

// pageData es lo único que ven las plantillas.
type pageData struct {
	Title    string
	Repos    []repoView
	HasRepo  bool
	Active   repoView
	Stats    statsView
	Rows     []rowView
	Labels   []string
	Filter   filter
	Shown    int
	Total    int
	Commit   string
	Fetched  string
	Language string
	// Toggles del filtro, ya resueltos como URL para que funcionen sin JS.
	FreeHref    string
	NoLabelHref string
}

type repoView struct {
	Name     string
	Key      string
	Open     int
	Stored   int
	Free     int
	Analyses int
	Active   bool
	Href     string
}

// statsView son las cifras del encabezado. Todas se cuentan sobre el corpus
// guardado, no sobre el filtro activo: el filtro dice qué se mira, no cuánto hay.
type statsView struct {
	Free           int
	Taken          int
	WithoutLabel   int
	WithoutComment int
	Analyses       int
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
}

type labelView struct {
	Text string
	// Hit solo cambia el énfasis visual de la etiqueta que declara dificultad.
	// Es presentación, nunca un filtro ni una señal para ninguna decisión (I6).
	Hit bool
}

// filter es el estado de los controles, tal como viaja en la URL.
type filter struct {
	Query   string
	State   string
	Label   string
	NoLabel bool
}

func parseFilter(q url.Values) filter {
	f := filter{
		Query:   strings.TrimSpace(q.Get("q")),
		State:   q.Get("state"),
		Label:   q.Get("label"),
		NoLabel: q.Get("nolabel") == "1",
	}
	if f.State != filterTaken && f.State != filterFree {
		f.State = filterAll
	}
	return f
}

// url devuelve la misma vista del corpus con este filtro, para links y toggles.
func (f filter) url(repoKey string) string {
	v := url.Values{}
	v.Set("repo", repoKey)
	if f.Query != "" {
		v.Set("q", f.Query)
	}
	if f.State != filterAll {
		v.Set("state", f.State)
	}
	if f.Label != "" {
		v.Set("label", f.Label)
	}
	if f.NoLabel {
		v.Set("nolabel", "1")
	}
	return "/?" + v.Encode()
}

func (f filter) withoutState() filter       { f.State = filterAll; return f }
func (f filter) withState(s string) filter  { f.State = s; return f }
func (f filter) withoutNoLabel() filter     { f.NoLabel = false; return f }
func (f filter) withNoLabel() filter        { f.NoLabel = true; return f }

// buildPage arma la pantalla. Un store nil o vacío no es un error: es la app
// recién instalada, y la pantalla lo dice en vez de mentir con ceros.
func buildPage(st *store.Store, q url.Values) (pageData, error) {
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
	data.FreeHref = toggleStateURL(f, active.Key)
	data.NoLabelHref = toggleNoLabelURL(f, active.Key)
	data.Commit = shortSHA(active.CommitSHA)
	data.Fetched = active.FetchedAt.UTC().Format("2006-01-02")

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
	data.Stats = countStats(st, active.Key, snap.Issues)
	data.Labels = distinctLabels(snap.Issues)

	for _, is := range snap.Issues {
		if !f.matches(is) {
			continue
		}
		data.Rows = append(data.Rows, newRow(is))
	}
	data.Shown = len(data.Rows)

	return data, nil
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

func (f filter) matches(is ghdata.Issue) bool {
	if f.State == filterTaken && !hasOpenPR(is) {
		return false
	}
	if f.State == filterFree && hasOpenPR(is) {
		return false
	}
	if f.NoLabel && len(is.Labels) > 0 {
		return false
	}
	if f.Label != "" && !hasLabel(is, f.Label) {
		return false
	}
	if f.Query != "" {
		needle := strings.ToLower(f.Query)
		if !strings.Contains(strings.ToLower(is.Title), needle) &&
			!strings.Contains(strings.ToLower(is.Body), needle) {
			return false
		}
	}
	return true
}

func newRow(is ghdata.Issue) rowView {
	r := rowView{
		Number:     is.Number,
		Title:      is.Title,
		URL:        is.URL,
		Comments:   is.CommentCount,
		NoComments: is.CommentCount == 0,
		Updated:    is.UpdatedAt.UTC().Format("2006-01-02"),
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
	for _, l := range is.Labels {
		r.Labels = append(r.Labels, labelView{Text: l, Hit: declaresDifficulty(l)})
	}
	return r
}

func countStats(st *store.Store, key string, issues []ghdata.Issue) statsView {
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
	}
	s.Analyses = analysesOf(st, key)
	return s
}

// analysesOf cuenta registros del diario, no issues distintas: un mismo issue
// analizado bajo otro prompt es otro análisis y otra pregunta (I14).
func analysesOf(st *store.Store, key string) int {
	d, err := st.Decisions(key)
	if err != nil {
		return 0
	}
	return d.Len()
}

// snapshotOf devuelve nil, nil cuando el repositorio está registrado pero
// todavía no tiene corpus: eso es un fetch pendiente, no un fallo. Un archivo
// presente pero ilegible sí es un error y se propaga.
func snapshotOf(st *store.Store, key string) (*ghdata.Snapshot, error) {
	if !st.HasSnapshot(key) {
		return nil, nil
	}
	return st.LoadSnapshot(key)
}

func countFree(snap *ghdata.Snapshot) int {
	if snap == nil {
		return 0
	}
	free := 0
	for _, is := range snap.Issues {
		if !hasOpenPR(is) {
			free++
		}
	}
	return free
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

func pickRepo(st *store.Store, repos []store.RepoEntry, requested string) (store.RepoEntry, bool) {
	if requested == "" {
		return repos[0], true
	}
	for _, e := range repos {
		// La URL puede traer la clave del store o el nombre del repositorio;
		// las dos formas son reconocibles y ninguna es ambigua.
		if e.Key == requested || e.Name == requested {
			return e, true
		}
	}
	return store.RepoEntry{}, false
}

func indexOfRepo(repos []repoView, key string) int {
	for i, r := range repos {
		if r.Key == key {
			return i
		}
	}
	return 0
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

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
