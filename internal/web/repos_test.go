package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// fakeFetcher answers from memory and records what it was asked for. No test in this file
// touches the network: that is the whole reason the handler takes a Fetcher instead of
// building a client.
type fakeFetcher struct {
	snap  *ghdata.Snapshot
	err   error
	calls []string
}

func (f *fakeFetcher) Fetch(_ context.Context, owner, name string, _ ghdata.Options) (*ghdata.Snapshot, error) {
	f.calls = append(f.calls, owner+"/"+name)
	// Igual que el cliente real: ErrNoIssues viene con un snapshot válido, porque un
	// repositorio sin issues abiertas existe y se guarda con cero.
	return f.snap, f.err
}

func snapshotOfRepo(canonical string, issues int) *ghdata.Snapshot {
	snap := &ghdata.Snapshot{
		Repository: ghdata.Repository{
			NameWithOwner:   canonical,
			PrimaryLanguage: "Go",
			DefaultBranch:   "main",
			CommitSHA:       "aaaabbbbccccdddd",
		},
		TotalOpen: issues,
	}
	for i := range issues {
		snap.Issues = append(snap.Issues, ghdata.Issue{Number: 1000 + i, Title: "an issue"})
	}
	return snap
}

func newRepoHandler(t *testing.T, st *store.Store, f Fetcher) *Handler {
	t.Helper()
	h, err := NewHandler(Options{Store: st, Fetcher: f})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return h
}

func post(t *testing.T, h *Handler, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	return rec
}

// S7, S5: the two accepted forms end in the same store key, and the key comes from the
// canonical name, not from what the user typed.
func TestAddRepoStoresTheSameKeyFromEitherInputForm(t *testing.T) {
	for _, input := range []struct {
		name  string
		value string
	}{
		{"owner and name", "spf13/cobra"},
		{"full https url", "https://github.com/spf13/cobra"},
		{"url with a trailing path", "https://github.com/spf13/cobra/"},
		{"url with .git", "https://github.com/spf13/cobra.git"},
		{"url with an explicit port", "https://github.com:443/spf13/cobra"},
		// GitHub devuelve el nombre canónico, así que las mayúsculas del pedido no abren una
		// segunda clave: la clave sale de la respuesta, no del texto tipeado.
		{"different case", "SPF13/Cobra"},
		{"surrounding spaces", "  spf13/cobra  "},
	} {
		t.Run(input.name, func(t *testing.T) {
			st, err := store.New(t.TempDir())
			if err != nil {
				t.Fatalf("store.New() error = %v", err)
			}
			f := &fakeFetcher{snap: snapshotOfRepo("spf13/cobra", 3)}
			h := newRepoHandler(t, st, f)

			rec := post(t, h, "/repos", url.Values{"nuevo": {input.value}})
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, want 303; body = %q", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Location"); got != "/?repo=spf13__cobra&estado=alta" {
				t.Errorf("Location = %q", got)
			}
			entry, ok, err := st.Repo("spf13__cobra")
			if err != nil {
				t.Fatalf("Repo() error = %v", err)
			}
			if !ok {
				t.Fatal("spf13__cobra is not in the registry")
			}
			if entry.Name != "spf13/cobra" || entry.Issues != 3 || entry.CommitSHA != "aaaabbbbccccdddd" {
				t.Errorf("entry = %+v", entry)
			}
			if !st.HasSnapshot("spf13__cobra") {
				t.Error("the snapshot was not saved")
			}
			// Se pide tal como se escribió, sin reescribir el caso: GitHub resuelve el
			// nombre. La clave guardada, en cambio, sale siempre de la respuesta.
			if len(f.calls) != 1 || !strings.EqualFold(f.calls[0], "spf13/cobra") {
				t.Errorf("fetcher calls = %v", f.calls)
			}
		})
	}
}

// S6, S9: bad input is the user's mistake, so it answers 400 with the reason in the body,
// and it costs no request: validating after calling GitHub would be paying for a typo.
func TestAddRepoAnswersFourHundredAndDoesNotFetchForBadInput(t *testing.T) {
	for _, bad := range []struct {
		value  string
		reason string
	}{
		{"", "escribí"},
		{"cobra", "owner/nombre"},
		{"spf13/", "owner/nombre"},
		{"/cobra", "owner/nombre"},
		{"spf13/cobra/extra", "owner/nombre"},
		{"https://gitlab.com/spf13/cobra", "GitHub"},
		{"https://github.com/spf13", "owner/nombre"},
	} {
		t.Run(bad.value, func(t *testing.T) {
			st, err := store.New(t.TempDir())
			if err != nil {
				t.Fatalf("store.New() error = %v", err)
			}
			f := &fakeFetcher{snap: snapshotOfRepo("spf13/cobra", 1)}
			h := newRepoHandler(t, st, f)

			rec := post(t, h, "/repos", url.Values{"nuevo": {bad.value}})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %q", rec.Code, rec.Body.String())
			}
			if !strings.Contains(strings.ToLower(rec.Body.String()), strings.ToLower(bad.reason)) {
				t.Errorf("body = %q, want it to mention %q", rec.Body.String(), bad.reason)
			}
			if len(f.calls) != 0 {
				t.Errorf("the fetcher was called for invalid input: %v", f.calls)
			}
			if repos, err := st.Repos(); err != nil || len(repos) != 0 {
				t.Errorf("repos = %v, err = %v; want an empty registry", repos, err)
			}
		})
	}
}

// S5, S8: GitHub is the authority on the name. httpie/httpie answers httpie/cli, and the
// alias keeps the store from splitting one repository in two the first time it is renamed.
func TestAddRepoKeepsTheCanonicalNameAndTheAlias(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	f := &fakeFetcher{snap: snapshotOfRepo("httpie/cli", 2)}
	h := newRepoHandler(t, st, f)

	rec := post(t, h, "/repos", url.Values{"nuevo": {"httpie/httpie"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body = %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/?repo=httpie__cli&estado=alta" {
		t.Errorf("Location = %q, want the canonical key", got)
	}
	canonical, ok := st.Alias("httpie/httpie")
	if !ok || canonical != "httpie/cli" {
		t.Errorf("alias = %q, %v; want httpie/cli", canonical, ok)
	}
	entry, ok, err := st.Repo("httpie__cli")
	if err != nil || !ok {
		t.Fatalf("Repo(httpie__cli) = %+v, %v, %v", entry, ok, err)
	}
	if entry.Name != "httpie/cli" {
		t.Errorf("entry.Name = %q, want the canonical name in the rail", entry.Name)
	}
	if _, ok, _ := st.Repo("httpie__httpie"); ok {
		t.Error("the requested name was stored as a second repository")
	}
}

// S6: a name GitHub does not know is a 400, not a 500. The distinction is the one the
// sentinel carries, and this is the test that keeps the handler honest about it.
func TestAddRepoAnswersFourHundredWhenGitHubDoesNotKnowTheRepository(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	f := &fakeFetcher{err: fmt.Errorf("%w: Could not resolve to a Repository", ghdata.ErrRepoNotFound)}
	h := newRepoHandler(t, st, f)

	rec := post(t, h, "/repos", url.Values{"nuevo": {"nope/nope"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "nope/nope") {
		t.Errorf("body = %q, want it to name the repository", rec.Body.String())
	}
	if repos, _ := st.Repos(); len(repos) != 0 {
		t.Errorf("repos = %v, want an empty registry", repos)
	}
}

// Whatever else breaks is ours, not the user's: it answers 500 and does not pretend the
// repository does not exist.
func TestAddRepoAnswersFiveHundredForAFetchThatBroke(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	f := &fakeFetcher{err: errors.New("ghdata: gh: exit status 1: connection refused")}
	h := newRepoHandler(t, st, f)

	rec := post(t, h, "/repos", url.Values{"nuevo": {"spf13/cobra"}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %q", rec.Code, rec.Body.String())
	}
}

// A repository with nothing open is not a failure: it is a real repository with zero open
// issues, and refusing to store it would make the rail lie in the other direction.
func TestAddRepoStoresARepositoryWithNoOpenIssues(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	f := &fakeFetcher{snap: snapshotOfRepo("tiny/repo", 0), err: ghdata.ErrNoIssues}
	h := newRepoHandler(t, st, f)

	rec := post(t, h, "/repos", url.Values{"nuevo": {"tiny/repo"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body = %q", rec.Code, rec.Body.String())
	}
	entry, ok, err := st.Repo("tiny__repo")
	if err != nil || !ok {
		t.Fatalf("Repo(tiny__repo) = %+v, %v, %v", entry, ok, err)
	}
	if entry.Issues != 0 || entry.TotalOpen != 0 {
		t.Errorf("entry = %+v, want zero issues", entry)
	}
	if !st.HasSnapshot("tiny__repo") {
		t.Error("the snapshot was not saved")
	}
}

// S10: «Refrescar» vuelve a consultar de verdad. Antes era un enlace a la misma URL, así
// que el pin y la fecha nunca cambiaban; ahora el registro los mueve y la pantalla los
// muestra.
func TestRefreshMovesThePinAndTheFetchTime(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	old := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
	if err := st.SaveRepo(store.RepoEntry{
		Name:      "spf13/cobra",
		Key:       "spf13__cobra",
		CommitSHA: "111111111111",
		TotalOpen: 40,
		Issues:    40,
		FetchedAt: old,
	}); err != nil {
		t.Fatalf("SaveRepo() error = %v", err)
	}

	fresh := snapshotOfRepo("spf13/cobra", 5)
	fresh.Repository.CommitSHA = "999999999999"
	f := &fakeFetcher{snap: fresh}
	h := newRepoHandler(t, st, f)

	rec := post(t, h, "/repos/spf13__cobra/refresh", nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body = %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/?repo=spf13__cobra&estado=refresco" {
		t.Errorf("Location = %q", got)
	}

	entry, ok, err := st.Repo("spf13__cobra")
	if err != nil || !ok {
		t.Fatalf("Repo() = %+v, %v, %v", entry, ok, err)
	}
	if entry.CommitSHA != "999999999999" {
		t.Errorf("CommitSHA = %q, want the pin moved", entry.CommitSHA)
	}
	if !entry.FetchedAt.After(old) {
		t.Errorf("FetchedAt = %v, want it after %v", entry.FetchedAt, old)
	}
	if entry.Issues != 5 || entry.TotalOpen != 5 {
		t.Errorf("entry = %+v, want the new counts", entry)
	}
	if len(f.calls) != 1 {
		t.Errorf("fetcher calls = %v, want exactly one", f.calls)
	}

	// Y la pantalla lo muestra: el pin del rail es el nuevo.
	page := body(t, h, "/?repo=spf13__cobra")
	if !strings.Contains(page, "999999999999") {
		t.Error("the rail does not show the new pin")
	}
	if strings.Contains(page, "111111111111") {
		t.Error("the rail still shows the old pin")
	}
}

// Refrescar algo que no está en el corpus sería un alta disfrazada: 404 y a otra cosa.
func TestRefreshAnswersNotFoundForARepositoryOutsideTheCorpus(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	f := &fakeFetcher{snap: snapshotOfRepo("nope/nope", 1)}
	h := newRepoHandler(t, st, f)

	rec := post(t, h, "/repos/nope__nope/refresh", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %q", rec.Code, rec.Body.String())
	}
	if len(f.calls) != 0 {
		t.Errorf("the fetcher was called for an unknown repository: %v", f.calls)
	}
}

// Un repositorio que se renombró mientras estaba en el corpus se refresca por su nombre
// canónico y queda bajo la clave nueva, no duplicado.
func TestRefreshFollowsARenameAndKeepsOneEntry(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	if err := st.SaveRepo(store.RepoEntry{
		Name: "httpie/httpie", Key: "httpie__httpie", CommitSHA: "111111111111",
	}); err != nil {
		t.Fatalf("SaveRepo() error = %v", err)
	}
	h := newRepoHandler(t, st, &fakeFetcher{snap: snapshotOfRepo("httpie/cli", 2)})

	rec := post(t, h, "/repos/httpie__httpie/refresh", nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body = %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/?repo=httpie__cli&estado=refresco" {
		t.Errorf("Location = %q, want the new key", got)
	}
	if canonical, ok := st.Alias("httpie/httpie"); !ok || canonical != "httpie/cli" {
		t.Errorf("alias = %q, %v", canonical, ok)
	}

	// El alias solo no alcanza: si la entrada vieja queda en el registro, el rail lista el
	// mismo repositorio dos veces, una con el pin que ya no existe. Se afirma sobre el
	// registro, que es lo que lee la pantalla.
	repos, err := st.Repos()
	if err != nil {
		t.Fatalf("Repos() error = %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("registry has %d entries (%+v), want exactly one", len(repos), repos)
	}
	if repos[0].Key != "httpie__cli" {
		t.Errorf("registry key = %q, want httpie__cli", repos[0].Key)
	}
	if _, ok, _ := st.Repo("httpie__httpie"); ok {
		t.Error("the stale key survived the rename")
	}
}

// S1, S3, S11: los dos controles del rail hacen lo que dicen. El formulario del alta queda
// habilitado y apunta al POST; «Refrescar» deja de ser un enlace a la misma URL y pasa a
// ser un botón que vuelve a consultar.
func TestDashboardOffersTheRealControls(t *testing.T) {
	st := corpus(t)
	h := newRepoHandler(t, st, &fakeFetcher{snap: snapshotOfRepo("spf13/cobra", 1)})

	page := body(t, h, "/?repo=spf13__cobra")

	form := between(t, page, `<form class="add"`, "</form>")
	if form == "" {
		t.Fatal("the rail has no add form")
	}
	if !strings.Contains(form, `action="/repos"`) || !strings.Contains(form, `method="post"`) {
		t.Errorf("the add form does not post to /repos: %s", form)
	}
	if !strings.Contains(form, `name="nuevo"`) {
		t.Error("the add form has no named field")
	}
	if strings.Contains(form, "disabled") {
		t.Errorf("the add form is still disabled: %s", form)
	}
	if !strings.Contains(page, `action="/repos/spf13__cobra/refresh"`) {
		t.Error("Refrescar does not post to the refresh route")
	}
	if strings.Contains(page, `<a class="btn" href`) {
		t.Error("Refrescar is still a link to the same URL")
	}

	// S11: la espera se dice. No hay test de navegador, así que lo que se afirma es el
	// contrato que el cliente necesita para poder hacerlo, y que borrarlo haga fallar algo.
	if !strings.Contains(form, "data-busy") || !strings.Contains(form, `class="busy"`) {
		t.Error("the add form has no busy state to show while it waits")
	}
	if !strings.Contains(page, `class="inline" method="post"`) ||
		!strings.Contains(page, "data-busy") {
		t.Error("the refresh form has no busy state")
	}
	js := body(t, h, "/static/app.js")
	if !strings.Contains(js, "form[data-busy]") {
		t.Error("the client never looks for the busy forms")
	}
}

// S1: la pantalla confirma lo que pasó. El estado viaja como código de una lista blanca, así
// que un valor inventado en la URL no se convierte en texto de la página.
func TestDashboardConfirmsTheAddAndTheRefresh(t *testing.T) {
	st := corpus(t)
	h := newRepoHandler(t, st, &fakeFetcher{snap: snapshotOfRepo("spf13/cobra", 1)})

	if page := body(t, h, "/?repo=spf13__cobra&estado=alta"); !strings.Contains(page, "Se agregó") {
		t.Error("the page does not confirm the add")
	}
	if page := body(t, h, "/?repo=spf13__cobra&estado=refresco"); !strings.Contains(page, "Refrescado") {
		t.Error("the page does not confirm the refresh")
	}
	if page := body(t, h, "/?repo=spf13__cobra&estado=<script>alert(1)</script>"); strings.Contains(page, "<script>alert") {
		t.Error("the estado parameter reached the page")
	}
}

// between devuelve el trozo de page que va desde open hasta close, o "" si no están.
func between(t *testing.T, page, open, close string) string {
	t.Helper()
	start := strings.Index(page, open)
	if start < 0 {
		return ""
	}
	end := strings.Index(page[start:], close)
	if end < 0 {
		return ""
	}
	return page[start : start+end+len(close)]
}

// El primer endpoint que cambia estado no puede aceptar un POST de otra página: el binario
// escucha en 0.0.0.0 por defecto, así que un sitio cualquiera abierto en el mismo navegador
// podría escribir en el store. Se corta por el origen, sin token y sin dependencias.
func TestPostRejectsCrossSiteRequests(t *testing.T) {
	for _, c := range []struct {
		name   string
		header map[string]string
	}{
		{"declared cross-site", map[string]string{"Sec-Fetch-Site": "cross-site"}},
		{"foreign origin", map[string]string{"Origin": "https://evil.example"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			st, err := store.New(t.TempDir())
			if err != nil {
				t.Fatalf("store.New() error = %v", err)
			}
			f := &fakeFetcher{snap: snapshotOfRepo("spf13/cobra", 1)}
			h := newRepoHandler(t, st, f)

			req := httptest.NewRequest(http.MethodPost, "/repos",
				strings.NewReader(url.Values{"nuevo": {"spf13/cobra"}}.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			for k, v := range c.header {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403; body = %q", rec.Code, rec.Body.String())
			}
			if len(f.calls) != 0 {
				t.Errorf("the fetch ran for a cross-site request: %v", f.calls)
			}
			if repos, _ := st.Repos(); len(repos) != 0 {
				t.Errorf("repos = %v, want an empty registry", repos)
			}
		})
	}

	// El control negativo: el mismo POST desde la página propia tiene que pasar.
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	h := newRepoHandler(t, st, &fakeFetcher{snap: snapshotOfRepo("spf13/cobra", 1)})
	req := httptest.NewRequest(http.MethodPost, "/repos",
		strings.NewReader(url.Values{"nuevo": {"spf13/cobra"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Origin", "http://"+req.Host)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("same-origin status = %d, want 303; body = %q", rec.Code, rec.Body.String())
	}
}
