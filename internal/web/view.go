package web

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// Este archivo tiene lo que comparten las pantallas: cómo se elige y se lee un
// repositorio del corpus, qué es un repositorio en el rail y cómo viaja un
// filtro en la URL. Lo propio de una pantalla vive en su archivo.

// errUnknownRepo distingue «ese repositorio no está en el corpus» de «no pude
// leerlo». Lo primero es un 404 honesto; lo segundo, un 500.
var errUnknownRepo = errors.New("repositorio desconocido")

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

// filter es el estado de los controles, tal como viaja en la URL.
type filter struct {
	Query   string
	State   string
	Label   string
	NoLabel bool
	// Defaulted dice que la URL no nombro un estado, asi que el ajuste lo elige la pantalla y
	// no quien navega. Existe para que un corpus sin triaje muestre todo en vez de una lista
	// vacia: filtrar por candidatas cuando no hay candidatas no es un default, es un error.
	Defaulted bool
}

const (
	filterAll   = "all"
	filterTaken = "taken"
	filterFree  = "free"
	// filterCandidate no es un hecho del corpus: es lo que decidio el triaje. Se resuelve
	// contra el journal, no contra la issue, asi que buildPageWith la aplica despues de leer
	// el veredicto. Es tambien el estado que la pantalla elige cuando la URL no elige ninguno.
	filterCandidate = "candidate"
)

func parseFilter(q url.Values) filter {
	f := filter{
		Query:   strings.TrimSpace(q.Get("q")),
		State:   q.Get("state"),
		Label:   q.Get("label"),
		NoLabel: q.Get("nolabel") == "1",
	}
	if f.State != filterTaken && f.State != filterFree && f.State != filterCandidate && f.State != filterAll {
		f.State = filterAll
	}
	// Una URL sin `state` no pide «todo»: no pide nada. La pantalla decide, y decide
	// candidatas en cuanto el corpus tenga alguna.
	f.Defaulted = strings.TrimSpace(q.Get("state")) == ""
	return f
}

// url devuelve la misma vista del corpus con este filtro, para links y toggles.
func (f filter) url(repoKey string) string {
	v := url.Values{}
	v.Set("repo", repoKey)
	if f.Query != "" {
		v.Set("q", f.Query)
	}
	// Ni el estado vacio ni candidatas se escriben: la URL corta es la que abre en lo que el
	// triaje recomendo. Todo lo demas se nombra, incluido «todo».
	switch f.State {
	case "", filterCandidate:
	default:
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

func (f filter) withoutState() filter      { f.State = filterAll; return f }
func (f filter) withState(s string) filter { f.State = s; return f }
func (f filter) withoutNoLabel() filter    { f.NoLabel = false; return f }
func (f filter) withNoLabel() filter       { f.NoLabel = true; return f }

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

// formatDate es el único formato de fecha de la app: día, sin hora, en UTC.
// La hora de un evento de GitHub no aporta a la decisión y sí invita a leerla
// como «hace cuánto», que no es lo mismo que «cuándo».
func formatDate(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
