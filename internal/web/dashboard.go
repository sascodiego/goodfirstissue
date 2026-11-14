package web

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// Estados de una fila. Solo dos se pueden afirmar hoy, y ninguna es un veredicto:
// "taken" es un hecho medido (hay una PR abierta vinculada) y "pending" es la
// ausencia de triaje. Los estados que dependen de Clef —no es trabajo,
// condicionada— llegan cuando exista internal/triage; inventarlos acá violaría
// I2 (nunca un veredicto por defecto) e I10 (nunca descartar por falta de prueba).
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
	Estado   string
	Commit   string
	Fetched  string
	Language string
	// Toggles del filtro, ya resueltos como URL para que funcionen sin JS.
	FreeHref    string
	NoLabelHref string
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

// dashboard sirve la pantalla del corpus.
func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
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
	h.render(w, screenDashboard, data)
}

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
	data.Estado = estadoText(q.Get("estado"))
	data.FreeHref = toggleStateURL(f, active.Key)
	data.NoLabelHref = toggleNoLabelURL(f, active.Key)
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

// estadoText traduce el código que deja el POST en la URL. Un valor que no está en la
// lista no se muestra: la pantalla no repite texto que vino de afuera.
func estadoText(code string) string {
	switch code {
	case "alta":
		return "Se agregó"
	case "refresco":
		return "Refrescado"
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

func newRow(is ghdata.Issue) rowView {
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
