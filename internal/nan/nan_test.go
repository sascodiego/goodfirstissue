package nan_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

const testKey = "test-key-not-a-secret"

// sampleQuestions is the smallest valid decision request: one yes/no question.
var sampleQuestions = map[string]nan.Question{"urgent": nan.NoulQuestion("Is this urgent?")}

// testClient bundles a client with the server it talks to.
type testClient struct {
	Client *nan.Client
	Server *httptest.Server
}

// newTestClient starts a test server and points a client at it. Tweaks adjust the options
// before the client is built, which is how a test injects a fake clock.
func newTestClient(t *testing.T, h http.HandlerFunc, tweaks ...func(*nan.Options)) testClient {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	opts := nan.Options{
		SystemOneURL: srv.URL + "/v1/systemone",
		ChatURL:      srv.URL + "/v1/chat/completions",
		APIKey:       testKey,
		Timeout:      nan.MinTimeout,
		MaxAttempts:  5,
	}
	for _, tweak := range tweaks {
		tweak(&opts)
	}
	c, err := nan.New(opts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return testClient{Client: c, Server: srv}
}

// newClient is the common case: no tweaks.
func newClient(t *testing.T, h http.HandlerFunc) (*nan.Client, *httptest.Server) {
	t.Helper()
	tc := newTestClient(t, h)
	return tc.Client, tc.Server
}

// AsError asserts err is this package's typed error and returns it.
func asError(t *testing.T, err error) *nan.Error {
	t.Helper()
	var e *nan.Error
	if !errors.As(err, &e) {
		t.Fatalf("error is not *nan.Error: %#v", err)
	}
	return e
}

// S4: docs/api.md fija 50 s como mínimo. Por debajo es configuración inválida, no un aviso.
func TestNewRejectsTimeoutBelowTheAPIMinimum(t *testing.T) {
	_, err := nan.New(nan.Options{APIKey: testKey, Timeout: nan.MinTimeout - time.Second})
	e := asError(t, err)
	if e.Kind != nan.KindConfig {
		t.Errorf("Kind = %q, want %q", e.Kind, nan.KindConfig)
	}
	if !strings.Contains(err.Error(), "50s") {
		t.Errorf("error does not name the minimum: %v", err)
	}
}

func TestNewAcceptsTheMinimumTimeout(t *testing.T) {
	if _, err := nan.New(nan.Options{APIKey: testKey, Timeout: nan.MinTimeout}); err != nil {
		t.Fatalf("New(50s) error = %v", err)
	}
}

// An omitted timeout takes the documented default, which is above the floor.
func TestNewDefaultsTheTimeout(t *testing.T) {
	c, err := nan.New(nan.Options{APIKey: testKey})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c == nil {
		t.Fatal("New() returned no client")
	}
}

// S6: the package never decides which model to use; a request without one is invalid.
func TestNewRequiresAPIKey(t *testing.T) {
	_, err := nan.New(nan.Options{Timeout: nan.MinTimeout})
	e := asError(t, err)
	if e.Kind != nan.KindConfig {
		t.Errorf("Kind = %q, want %q", e.Kind, nan.KindConfig)
	}
	if !strings.Contains(err.Error(), "api_key") {
		t.Errorf("error does not name the missing setting: %v", err)
	}
}

func TestNewRejectsNegativeMaxAttempts(t *testing.T) {
	_, err := nan.New(nan.Options{APIKey: testKey, Timeout: nan.MinTimeout, MaxAttempts: -1})
	if e := asError(t, err); e.Kind != nan.KindConfig {
		t.Errorf("Kind = %q, want %q", e.Kind, nan.KindConfig)
	}
}

func TestNewRejectsAMalformedURL(t *testing.T) {
	_, err := nan.New(nan.Options{APIKey: testKey, Timeout: nan.MinTimeout, SystemOneURL: "://nope"})
	if e := asError(t, err); e.Kind != nan.KindConfig {
		t.Errorf("Kind = %q, want %q", e.Kind, nan.KindConfig)
	}
}

// S3: 401 invalid_api_key is the code docs/api.md gets wrong (it says 404), and it is
// never retried: retrying a rejected key only burns the queue.
func TestUnauthorizedIsTypedAndNeverRetried(t *testing.T) {
	var calls atomic.Int32
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid_api_key"}}`))
	})

	_, err := c.SystemOne(context.Background(), nan.SystemOneRequest{
		Model:     "clef",
		State:     "a ticket",
		Questions: sampleQuestions,
	})
	e := asError(t, err)
	if e.Kind != nan.KindUnauthorized {
		t.Errorf("Kind = %q, want %q", e.Kind, nan.KindUnauthorized)
	}
	if e.Status != http.StatusUnauthorized {
		t.Errorf("Status = %d, want 401", e.Status)
	}
	if e.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", e.Attempts)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("server saw %d requests, want 1", got)
	}
	if !strings.Contains(e.Body, "invalid_api_key") {
		t.Errorf("Body = %q, want the API's message", e.Body)
	}
}

// A malformed request is our bug to fix, not something a retry can repair.
func TestBadRequestIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"malformed json"}}`))
	})

	_, err := c.SystemOne(context.Background(), nan.SystemOneRequest{Model: "clef", State: "x", Questions: sampleQuestions})
	if e := asError(t, err); e.Kind != nan.KindBadRequest {
		t.Errorf("Kind = %q, want %q", e.Kind, nan.KindBadRequest)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("server saw %d requests, want 1", got)
	}
}

// The message is what the app shows when it answers `undecided`, so it is pinned here.
func TestErrorMessageNamesTheOperationKindAndStatus(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid_api_key"}}`))
	})

	_, err := c.SystemOne(context.Background(), nan.SystemOneRequest{Model: "clef", State: "x", Questions: sampleQuestions})
	want := "nan: systemone: unauthorized (401): invalid_api_key"
	if err.Error() != want {
		t.Errorf("Error() = %q\n           want %q", err.Error(), want)
	}
}

// An error body is evidence for the user, never a place to dump megabytes into the UI.
func TestErrorBodyIsBounded(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(strings.Repeat("x", 8192)))
	})

	_, err := c.SystemOne(context.Background(), nan.SystemOneRequest{Model: "clef", State: "x", Questions: sampleQuestions})
	e := asError(t, err)
	if len(e.Body) > nan.MaxErrorBody {
		t.Errorf("Body has %d bytes, want at most %d", len(e.Body), nan.MaxErrorBody)
	}
}

// A 200 whose body is not the documented envelope is a protocol failure, not a decision.
func TestUndecodableBodyIsTyped(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json at all"))
	})

	_, err := c.SystemOne(context.Background(), nan.SystemOneRequest{Model: "clef", State: "x", Questions: sampleQuestions})
	if e := asError(t, err); e.Kind != nan.KindDecode {
		t.Errorf("Kind = %q, want %q", e.Kind, nan.KindDecode)
	}
}

// S6: the package refuses to send a request it cannot describe, instead of letting the API
// answer 400 for something the caller could have caught.
func TestSystemOneRefusesAnIncompleteRequest(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("the server was called with an invalid request")
	})

	cases := map[string]nan.SystemOneRequest{
		"no model":     {State: "x", Questions: sampleQuestions},
		"no state":     {Model: "clef", Questions: sampleQuestions},
		"no questions": {Model: "clef", State: "x"},
	}
	for name, req := range cases {
		_, err := c.SystemOne(context.Background(), req)
		if e := asError(t, err); e.Kind != nan.KindRequest {
			t.Errorf("%s: Kind = %q, want %q", name, e.Kind, nan.KindRequest)
		}
	}
}

// The API takes at most 64 questions per request. Grouping is free; exceeding it is a
// guaranteed 400.
func TestSystemOneRefusesMoreQuestionsThanTheAPIAllows(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("the server was called with too many questions")
	})

	questions := make(map[string]nan.Question, nan.MaxQuestions+1)
	for i := 0; i <= nan.MaxQuestions; i++ {
		questions[fmt.Sprintf("q%d", i)] = nan.NoulQuestion("Is it urgent?")
	}
	_, err := c.SystemOne(context.Background(), nan.SystemOneRequest{
		Model: "clef", State: "x", Questions: questions,
	})
	if e := asError(t, err); e.Kind != nan.KindRequest {
		t.Errorf("Kind = %q, want %q", e.Kind, nan.KindRequest)
	}
	if !strings.Contains(err.Error(), "64") {
		t.Errorf("error does not name the ceiling: %v", err)
	}
}
