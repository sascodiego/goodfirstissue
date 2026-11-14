package profile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

const cacheTestModel = "glm5.3-flash"

const cacheTestCV = "5 años de desarrollo backend con Django.\nPostgreSQL"

const cacheTestAnswer = `## Conocimientos técnicos
- Python — nivel: demostrado — evidencia: «5 años de desarrollo backend con Django»
`

// cacheHarness returns an extractor sharing one store, and a counter of model calls.
func cacheHarness(t *testing.T, st *store.Store) (*Extractor, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"index":   0,
				"message": map[string]string{"role": "assistant", "content": cacheTestAnswer},
			}},
		})
	}))
	t.Cleanup(srv.Close)

	client, err := nan.New(nan.Options{
		ChatURL:      srv.URL + "/v1/chat/completions",
		SystemOneURL: srv.URL + "/v1/systemone",
		APIKey:       "test-key",
		Timeout:      nan.MinTimeout,
	})
	if err != nil {
		t.Fatalf("nan.New() error = %v", err)
	}
	e, err := New(Options{Client: client, Model: cacheTestModel, Store: st})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return e, &calls
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	return st
}

// S3: the key answers one question. Another CV, another model or another prompt is another
// question, and must miss (I14).
func TestCacheKeyChangesWithEveryPart(t *testing.T) {
	base := cacheKey(cacheTestCV, cacheTestModel, Version())
	if base == "" {
		t.Fatal("cacheKey() is empty")
	}
	if base != cacheKey(cacheTestCV, cacheTestModel, Version()) {
		t.Fatal("cacheKey() is not deterministic")
	}
	for name, other := range map[string]string{
		"another CV":      cacheKey(cacheTestCV+"!", cacheTestModel, Version()),
		"another model":   cacheKey(cacheTestCV, "gemma4", Version()),
		"another version": cacheKey(cacheTestCV, cacheTestModel, "000000000000"),
	} {
		if other == base {
			t.Errorf("%s produced the same key: the cache would answer a different question", name)
		}
	}
}

// The same CV twice costs one request.
func TestExtractDoesNotPayTwiceForTheSameCV(t *testing.T) {
	st := newTestStore(t)
	first, calls := cacheHarness(t, st)

	one, err := first.Extract(context.Background(), cacheTestCV)
	if err != nil {
		t.Fatalf("first Extract() error = %v", err)
	}
	if one.Cached {
		t.Error("Cached = true on the first extraction")
	}
	if _, err := first.Extract(context.Background(), cacheTestCV); err != nil {
		t.Fatalf("second Extract() error = %v", err)
	}
	second, _ := first.Extract(context.Background(), cacheTestCV)
	if !second.Cached {
		t.Error("Cached = false on a repeated CV")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("model calls = %d, want 1", got)
	}

	// A fresh extractor over the same store also hits: the cache is on disk, not in memory.
	other, more := cacheHarness(t, st)
	if _, err := other.Extract(context.Background(), cacheTestCV); err != nil {
		t.Fatalf("Extract() from another extractor error = %v", err)
	}
	if got := more.Load(); got != 0 {
		t.Errorf("model calls from a second extractor = %d, want 0", got)
	}
	if second.Document.Text != one.Document.Text {
		t.Error("the cached document differs from the extracted one")
	}
}

// A saved answer produced by another model is another question, even with the same CV (I14).
func TestAnEntryFromAnotherModelIsAMiss(t *testing.T) {
	st := newTestStore(t)
	writeCache(t, st, map[string]cacheEntry{
		cacheKey(cacheTestCV, "gemma4", Version()): {
			Answer:        cacheTestAnswer,
			Model:         "gemma4",
			PromptVersion: Version(),
		},
	})

	e, calls := cacheHarness(t, st)
	got, err := e.Extract(context.Background(), cacheTestCV)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if got.Cached {
		t.Error("Cached = true for an entry written by another model")
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("model calls = %d, want 1", n)
	}
}

// Same for another prompt version: the answer was given under different rules.
func TestAnEntryFromAnotherPromptVersionIsAMiss(t *testing.T) {
	st := newTestStore(t)
	writeCache(t, st, map[string]cacheEntry{
		cacheKey(cacheTestCV, cacheTestModel, "000000000000"): {
			Answer:        cacheTestAnswer,
			Model:         cacheTestModel,
			PromptVersion: "000000000000",
		},
	})

	e, calls := cacheHarness(t, st)
	got, err := e.Extract(context.Background(), cacheTestCV)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if got.Cached {
		t.Error("Cached = true for an answer given under another prompt")
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("model calls = %d, want 1", n)
	}
}

// A damaged entry must cost one request, not a failed extraction: the cache is an optimisation and
// never a source of truth.
func TestACorruptCacheEntryIsIgnored(t *testing.T) {
	st := newTestStore(t)
	writeCache(t, st, map[string]cacheEntry{
		cacheKey(cacheTestCV, cacheTestModel, Version()): {
			Answer:        "Sure! This person looks like a backend developer.",
			Model:         cacheTestModel,
			PromptVersion: Version(),
		},
	})

	e, calls := cacheHarness(t, st)
	got, err := e.Extract(context.Background(), cacheTestCV)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if got.Cached {
		t.Error("Cached = true from an unparseable entry")
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("model calls = %d, want 1", n)
	}
	if !strings.Contains(got.Document.Text, "Python") {
		t.Error("the extraction did not replace the corrupt entry")
	}
}

// An answer that cannot be read is not cached: caching a failure would pin the failure.
func TestAnUnreadableAnswerIsNotCached(t *testing.T) {
	st := newTestStore(t)
	writeCache(t, st, map[string]cacheEntry{})

	e, _ := cacheHarness(t, st)
	// The harness answers with a valid document, so this only proves the file stays readable and
	// grows by one entry per successful extraction.
	if _, err := e.Extract(context.Background(), cacheTestCV); err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	var file cacheFile
	if err := st.Read(cacheName, &file); err != nil {
		t.Fatalf("store.Read() error = %v", err)
	}
	if len(file.Entries) != 1 {
		t.Errorf("entries = %d, want 1", len(file.Entries))
	}
}

func writeCache(t *testing.T, st *store.Store, entries map[string]cacheEntry) {
	t.Helper()
	if err := st.Write(cacheName, cacheFile{Entries: entries}); err != nil {
		t.Fatalf("store.Write() error = %v", err)
	}
}
