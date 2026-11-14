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
			if len(f.calls) != 1 || f.calls[0] != "spf13/cobra" {
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
