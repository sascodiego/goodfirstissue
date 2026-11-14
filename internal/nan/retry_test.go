package nan_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

// waits records the backoff schedule instead of sleeping it. A retry test must not spend
// the seconds it asserts.
type waits struct {
	mu   sync.Mutex
	seen []time.Duration
}

func (w *waits) sleep(ctx context.Context, d time.Duration) error {
	w.mu.Lock()
	w.seen = append(w.seen, d)
	w.mu.Unlock()
	return nil
}

func (w *waits) list() []time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]time.Duration(nil), w.seen...)
}

// S5: a 5xx is retried, and the wait grows instead of hammering the service.
func TestServerErrorIsRetriedWithBackoff(t *testing.T) {
	var calls atomic.Int32
	w := &waits{}
	tc := newTestClient(t, func(rw http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = rw.Write([]byte(`{"answers":{}}`))
	}, func(opts *nan.Options) { opts.Sleep = w.sleep })

	res, err := tc.Client.SystemOne(context.Background(), nan.SystemOneRequest{
		Model: "clef", State: "x", Questions: sampleQuestions,
	})
	if err != nil {
		t.Fatalf("SystemOne() error = %v: two 5xx responses must be retried", err)
	}
	if res == nil {
		t.Fatal("SystemOne() returned no response")
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("server saw %d requests, want 3", got)
	}
	want := []time.Duration{500 * time.Millisecond, time.Second}
	if got := w.list(); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("waits = %v, want %v", got, want)
	}
}

// S5: when the attempts run out, the caller gets a typed error it can explain, not a
// generic failure and never a default verdict (I2).
func TestExhaustedAttemptsAreTyped(t *testing.T) {
	var calls atomic.Int32
	tc := newTestClient(t, func(rw http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		rw.WriteHeader(http.StatusInternalServerError)
		_, _ = rw.Write([]byte(`{"error":{"message":"upstream failed"}}`))
	}, func(opts *nan.Options) {
		opts.MaxAttempts = 5
		opts.Sleep = func(ctx context.Context, d time.Duration) error { return nil }
	})

	_, err := tc.Client.SystemOne(context.Background(), nan.SystemOneRequest{
		Model: "clef", State: "x", Questions: sampleQuestions,
	})
	e := asError(t, err)
	if e.Kind != nan.KindServer {
		t.Errorf("Kind = %q, want %q: the cause is what the app has to report", e.Kind, nan.KindServer)
	}
	if e.Attempts != 5 {
		t.Errorf("Attempts = %d, want the ceiling of 5", e.Attempts)
	}
	if got := calls.Load(); got != 5 {
		t.Errorf("server saw %d requests, want 5", got)
	}
	if !e.Retryable() {
		t.Error("Retryable() = false: a 5xx is retryable, it just ran out of attempts")
	}
}

// S9 (I1): a 429 waits one second, not an exponential backoff.
func TestRateLimitWaitsOneSecond(t *testing.T) {
	var calls atomic.Int32
	w := &waits{}
	tc := newTestClient(t, func(rw http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			rw.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = rw.Write([]byte(`{"answers":{}}`))
	}, func(opts *nan.Options) { opts.Sleep = w.sleep })

	if _, err := tc.Client.SystemOne(context.Background(), nan.SystemOneRequest{
		Model: "clef", State: "x", Questions: sampleQuestions,
	}); err != nil {
		t.Fatalf("SystemOne() error = %v", err)
	}
	seen := w.list()
	if len(seen) != 1 || seen[0] != time.Second {
		t.Errorf("waits = %v, want exactly one wait of 1s", seen)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("server saw %d requests, want 2", got)
	}
}

// S5: a transport failure is a network failure and it is retried, not reported as a
// decision.
func TestNetworkErrorIsRetriedAndTyped(t *testing.T) {
	w := &waits{}
	tc := newTestClient(t, func(rw http.ResponseWriter, r *http.Request) {
		t.Error("the server should be unreachable")
	}, func(opts *nan.Options) {
		opts.Sleep = w.sleep
		opts.MaxAttempts = 2
	})
	tc.Server.Close() // every attempt now fails at the transport

	_, err := tc.Client.SystemOne(context.Background(), nan.SystemOneRequest{
		Model: "clef", State: "x", Questions: sampleQuestions,
	})
	e := asError(t, err)
	if e.Kind != nan.KindNetwork {
		t.Errorf("Kind = %q, want %q", e.Kind, nan.KindNetwork)
	}
	if e.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", e.Attempts)
	}
	if got := len(w.list()); got != 1 {
		t.Errorf("waits = %d, want 1 wait between the two attempts", got)
	}
}

// A canceled caller is not a service failure and must not be retried: another attempt
// could not run.
func TestCanceledContextIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	tc := newTestClient(t, func(rw http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = rw.Write([]byte(`{"answers":{}}`))
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := tc.Client.SystemOne(ctx, nan.SystemOneRequest{
		Model: "clef", State: "x", Questions: sampleQuestions,
	})
	e := asError(t, err)
	if e.Kind != nan.KindCanceled {
		t.Errorf("Kind = %q, want %q", e.Kind, nan.KindCanceled)
	}
	if e.Attempts != 0 {
		t.Errorf("Attempts = %d, want 0: nothing was sent", e.Attempts)
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("server saw %d requests, want 0", got)
	}
}

// S4: the caller's deadline is honored end to end, and the test server is cancelable so the
// test does not leak a goroutine waiting on a response nobody will read.
func TestCallerDeadlineIsHonored(t *testing.T) {
	stop := make(chan struct{})
	observed := make(chan struct{})
	var once sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// Drain the body first: net/http only watches the connection for a client abort once
		// the request body is consumed, so without this the handler would never learn that
		// the caller gave up.
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
			once.Do(func() { close(observed) })
		case <-stop:
		}
	}))
	defer srv.Close()
	defer close(stop) // LIFO: the handler leaves before Close waits for it

	c, err := nan.New(nan.Options{
		SystemOneURL: srv.URL + "/v1/systemone",
		ChatURL:      srv.URL + "/v1/chat/completions",
		APIKey:       testKey,
		Timeout:      nan.MinTimeout,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = c.SystemOne(ctx, nan.SystemOneRequest{
		Model: "clef", State: "x", Questions: sampleQuestions,
	})
	elapsed := time.Since(start)

	e := asError(t, err)
	if e.Kind != nan.KindTimeout {
		t.Errorf("Kind = %q, want %q", e.Kind, nan.KindTimeout)
	}
	if e.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1: the context was already done for any retry", e.Attempts)
	}
	if elapsed > 5*time.Second {
		t.Errorf("SystemOne() took %v: the caller's deadline was ignored", elapsed)
	}
	select {
	case <-observed:
	case <-time.After(2 * time.Second):
		t.Error("the test server never saw the cancellation: the client left the connection open")
	}
}

// S4: the configured timeout reaches the transport, and it is never below the API minimum.
func TestConfiguredTimeoutReachesTheTransport(t *testing.T) {
	tc := newTestClient(t, func(rw http.ResponseWriter, r *http.Request) {},
		func(opts *nan.Options) { opts.Timeout = 60 * time.Second })
	if got := tc.Client.Timeout(); got != 60*time.Second {
		t.Errorf("Timeout() = %v, want the configured 60s", got)
	}
	if tc.Client.Timeout() < nan.MinTimeout {
		t.Errorf("Timeout() = %v, below the API minimum %v", tc.Client.Timeout(), nan.MinTimeout)
	}
	if got := tc.Client.MaxAttempts(); got != 5 {
		t.Errorf("MaxAttempts() = %d, want 5", got)
	}
}
