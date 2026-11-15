package web

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// errUnknownIssue distingue «esa issue no está en el corpus guardado» de «no
// pude leerlo». Es un 404, y nunca se sustituye por otra issue del repositorio:
// el silencio haría creer que se está mirando la que se pidió.
var errUnknownIssue = errors.New("issue desconocida")

// issuePage es lo único que ve la plantilla de la pantalla de issue.
type issuePage struct {
	Title    string
	BackHref string
	HasRepo  bool
	Active   repoView
	Corpus   string
	Commit   string
	// CommitSHA es el pin completo: el bloque de evidencia busca el análisis con esta clave,
	// así que necesita el identificador entero y no el corto del rail.
	CommitSHA string
	Issue     issueView
	// Evidence es el bloque de evidencia guardada, y HasEvidence dice si hay algo que leer.
	Evidence    evidenceView
	HasEvidence bool
	// Triage es el informe de triaje guardado, y HasTriage dice si hay un veredicto que leer.
	// Sin él el bloque lo dice y nombra el comando en vez de quedar vacío (S9).
	Triage    triageView
	HasTriage bool
	// CanAnalyze dice si esta pantalla puede ofrecer el botón de analizar: hay cola, hay
	// modelo y hay clave. Ofrecerlo sin eso sería prometer otra cosa.
	CanAnalyze bool
	// Live mantiene el layout igual que en el dashboard: una issue no tiene cola propia.
	Live bool
}

// issueView es una issue preparada para leerla: el cuerpo y los comentarios ya
// pasaron por el render seguro, y las etiquetas viajan verbatim.
type issueView struct {
	Number            int
	Title             string
	URL               string
	State             string // clase CSS, en minúsculas
	StateText         string
	Author            string
	AuthorAssociation string
	Created           string
	Updated           string
	Labels            []labelView
	HasLabel          bool
	Body              template.HTML
	HasBody           bool
	CommentCount      int
	Comments          []commentView
	PRs               []prView
	HasPR             bool
}

type commentView struct {
	Author      string
	Association string
	Body        template.HTML
	Created     string
}

type prView struct {
	Number int
	URL    string
	State  string
	Text   string
}

// issue sirve la pantalla de una issue del corpus.
func (h *Handler) issue(w http.ResponseWriter, r *http.Request) {
	data, err := buildIssuePage(h.store, r.PathValue("repo"), r.PathValue("number"))
	switch {
	case errors.Is(err, errUnknownRepo):
		http.Error(w, "ese repositorio no está en el corpus; traelo con: gfi fetch owner/nombre", http.StatusNotFound)
		return
	case errors.Is(err, errUnknownIssue):
		http.Error(w, "esa issue no está en el corpus guardado; corré gfi fetch para refrescarlo", http.StatusNotFound)
		return
	case err != nil:
		slog.Error("armar la pantalla de issue", "err", err)
		http.Error(w, "no se pudo leer el corpus", http.StatusInternalServerError)
		return
	}

	h.attachTriage(&data)
	h.attachEvidence(&data)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.render(w, screenIssue, data)
}

// buildIssuePage arma la pantalla. El rail es el mismo del dashboard y se arma
// con la misma función —no con una copia— para que el contexto de repositorio
// no se bifurque entre dos pantallas.
func buildIssuePage(st *store.Store, repo, rawNumber string) (issuePage, error) {
	var data issuePage

	number, err := strconv.Atoi(strings.TrimSpace(rawNumber))
	if err != nil || number <= 0 {
		return data, errUnknownIssue
	}
	if repo == "" || st == nil {
		return data, errUnknownRepo
	}

	shell, err := buildPage(st, url.Values{"repo": {repo}})
	if err != nil {
		return data, err // errUnknownRepo incluido
	}
	data.HasRepo = shell.HasRepo
	data.Active = shell.Active
	data.BackHref = filter{}.url(shell.Active.Key)
	data.Corpus = shell.Fetched
	data.Commit = shell.Commit

	snap, err := snapshotOf(st, shell.Active.Key)
	if err != nil {
		return data, err
	}
	if snap == nil {
		// El repositorio está registrado y todavía no tiene corpus: la issue no
		// puede estar ahí.
		return data, errUnknownIssue
	}

	is, ok := findIssue(snap.Issues, number)
	if !ok {
		return data, errUnknownIssue
	}

	data.CommitSHA = snap.Repository.CommitSHA
	data.Issue = newIssueView(is)
	data.Title = fmt.Sprintf("#%d %s · %s", is.Number, is.Title, shell.Active.Name)
	return data, nil
}

func findIssue(issues []ghdata.Issue, number int) (ghdata.Issue, bool) {
	for _, is := range issues {
		if is.Number == number {
			return is, true
		}
	}
	return ghdata.Issue{}, false
}

func newIssueView(is ghdata.Issue) issueView {
	body := strings.TrimSpace(is.Body)
	v := issueView{
		Number:            is.Number,
		Title:             is.Title,
		URL:               is.URL,
		State:             strings.ToLower(is.State),
		StateText:         issueStateText(is.State),
		Author:            is.Author,
		AuthorAssociation: is.AuthorAssociation,
		Created:           formatDate(is.CreatedAt),
		Updated:           formatDate(is.UpdatedAt),
		HasLabel:          len(is.Labels) > 0,
		HasBody:           body != "",
		CommentCount:      is.CommentCount,
	}
	if v.HasBody {
		v.Body = renderMarkdown(body)
	}
	for _, l := range is.Labels {
		v.Labels = append(v.Labels, labelView{Text: l, Hit: declaresDifficulty(l)})
	}
	for _, pr := range is.LinkedPRs {
		v.PRs = append(v.PRs, prView{
			Number: pr.Number,
			URL:    pr.URL,
			State:  strings.ToLower(pr.State),
			Text:   prStateText(pr),
		})
	}
	v.HasPR = len(v.PRs) > 0
	for _, c := range is.Comments {
		v.Comments = append(v.Comments, commentView{
			Author:      c.Author,
			Association: c.AuthorAssociation,
			Body:        renderMarkdown(strings.TrimSpace(c.Body)),
			Created:     formatDate(c.CreatedAt),
		})
	}
	return v
}

// issueStateText traduce el estado de GitHub a la pantalla. Es el estado del
// hilo, no un veredicto sobre la issue: eso llega con el triaje.
func issueStateText(state string) string {
	switch strings.ToLower(state) {
	case "open":
		return "abierta"
	case "closed":
		return "cerrada"
	case "":
		return "sin estado"
	}
	return strings.ToLower(state)
}

func prStateText(pr ghdata.PullRequest) string {
	switch {
	case pr.Merged:
		return "mergeada"
	case strings.EqualFold(pr.State, "OPEN"):
		return "abierta"
	case strings.EqualFold(pr.State, "CLOSED"):
		return "cerrada"
	}
	return strings.ToLower(pr.State)
}
