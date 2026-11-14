package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

var fetchedAt = time.Date(2026, 11, 14, 9, 30, 0, 0, time.UTC)

// corpus siembra un store temporal con las formas que importan: un repo con
// issues de los tres tipos que hoy se pueden afirmar, otro con corpus propio
// para probar el cambio de repositorio, y uno registrado sin corpus.
//
// Los repositorios se listan por nombre, así que el activo por defecto es
// httpie/cli y no spf13/cobra: los tests que hablan de cobra lo dicen explícito.
func corpus(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}

	add := func(name, key string, open, issues int) {
		t.Helper()
		err := st.SaveRepo(store.RepoEntry{
			Name:      name,
			Key:       key,
			CommitSHA: "adbc8813901bba65827259daa8e22ff94ec1f30e",
			TotalOpen: open,
			Issues:    issues,
			FetchedAt: fetchedAt,
		})
		if err != nil {
			t.Fatalf("SaveRepo(%s) error = %v", name, err)
		}
	}

	add("spf13/cobra", "spf13__cobra", 262, 3)
	add("httpie/cli", "httpie__cli", 192, 1)
	add("zeta/empty", "zeta__empty", 10, 0)

	cobra := &ghdata.Snapshot{
		Repository: ghdata.Repository{NameWithOwner: "spf13/cobra", PrimaryLanguage: "Go"},
		TotalOpen:  262,
		Issues: []ghdata.Issue{
			{
				Number:       2568,
				Title:        "YAML docs ignore linkHandler for see_also entries",
				Body:         "The link handler is ignored.",
				UpdatedAt:    time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC),
				CommentCount: 0,
			},
			{
				Number:       1391,
				Title:        "404 links on cobra.dev",
				UpdatedAt:    time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
				CommentCount: 17,
				Labels:       []string{"kind/documentation", "good-first-issue"},
				LinkedPRs:    []ghdata.PullRequest{{Number: 1984, State: "OPEN"}},
			},
			{
				Number:       1995,
				Title:        "Question about custom templates",
				UpdatedAt:    time.Date(2025, 10, 27, 0, 0, 0, 0, time.UTC),
				CommentCount: 3,
				Labels:       []string{"kind/support"},
			},
		},
	}
	if err := st.SaveSnapshot("spf13__cobra", cobra); err != nil {
		t.Fatalf("SaveSnapshot(cobra) error = %v", err)
	}

	httpie := &ghdata.Snapshot{
		Repository: ghdata.Repository{NameWithOwner: "httpie/cli", PrimaryLanguage: "Python"},
		TotalOpen:  192,
		Issues: []ghdata.Issue{{
			Number:       1978,
			Title:        "log_error ignores --quiet flag",
			UpdatedAt:    time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC),
			CommentCount: 2,
			LinkedPRs:    []ghdata.PullRequest{{Number: 2001, State: "OPEN"}},
		}},
	}
	if err := st.SaveSnapshot("httpie__cli", httpie); err != nil {
		t.Fatalf("SaveSnapshot(httpie) error = %v", err)
	}

	return st
}

func newHandler(t *testing.T, st *store.Store) *Handler {
	t.Helper()
	h, err := NewHandler(Options{Store: st})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return h
}

func get(t *testing.T, h *Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func body(t *testing.T, h *Handler, target string) string {
	t.Helper()
	return get(t, h, target).Body.String()
}

// Los activos siguen saliendo del binario, ahora también el JS.
func TestStaticServesAssets(t *testing.T) {
	h := newHandler(t, nil)

	for _, tc := range []struct{ path, ctype string }{
		{"/static/app.css", "text/css"},
		{"/static/app.js", "javascript"},
	} {
		rec := get(t, h, tc.path)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want %d", tc.path, rec.Code, http.StatusOK)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, tc.ctype) {
			t.Errorf("GET %s Content-Type = %q, want %q", tc.path, ct, tc.ctype)
		}
		if rec.Body.Len() == 0 {
			t.Errorf("GET %s devolvió un cuerpo vacío", tc.path)
		}
	}
}

// Una ruta desconocida no puede caer en el index.
func TestUnknownRouteIs404(t *testing.T) {
	if rec := get(t, newHandler(t, nil), "/no-existe"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /no-existe status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
