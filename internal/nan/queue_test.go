package nan_test

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

// counter records how many requests the test server has in flight at once.
type concurrency struct {
	mu       sync.Mutex
	inFlight int
	max      int
}

func (c *concurrency) enter() {
	c.mu.Lock()
	c.inFlight++
	if c.inFlight > c.max {
		c.max = c.inFlight
	}
	c.mu.Unlock()
}

func (c *concurrency) leave() {
	c.mu.Lock()
	c.inFlight--
	c.mu.Unlock()
}

func (c *concurrency) peak() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.max
}

// S1, the test that protects I1: two goroutines calling SystemOne at the same time never
// put two requests in flight.
func TestSystemOneNeverHasTwoRequestsInFlight(t *testing.T) {
	var seen concurrency
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen.enter()
		defer seen.leave()
		// Long enough that an unserialized pair would certainly overlap.
		time.Sleep(20 * time.Millisecond)
		_, _ = w.Write([]byte(`{"answers":{}}`))
	})

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = c.SystemOne(context.Background(), nan.SystemOneRequest{
				Model: "clef", State: "x", Questions: sampleQuestions,
			})
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d error = %v: the second caller must wait, not fail", i, err)
		}
	}
	if peak := seen.peak(); peak != 1 {
		t.Errorf("the server saw %d requests in flight, want 1", peak)
	}
}

// S7: the queue belongs to the client, so two screens share one slot without coordinating.
func TestTheQueueIsSharedByIndependentCallers(t *testing.T) {
	var seen concurrency
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen.enter()
		defer seen.leave()
		time.Sleep(10 * time.Millisecond)
		_, _ = w.Write([]byte(`{"answers":{}}`))
	})

	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = c.SystemOne(context.Background(), nan.SystemOneRequest{
				Model: "clef", State: "x", Questions: sampleQuestions,
			})
		}()
	}
	wg.Wait()

	if peak := seen.peak(); peak != 1 {
		t.Errorf("the server saw %d requests in flight across six callers, want 1", peak)
	}
}

// The slot is held per attempt, not across a backoff wait: a sleeping retry must not block
// the next caller. Without this, one flaky request would stall every screen behind it.
func TestTheWireIsFreeWhileARetryWaits(t *testing.T) {
	var calls atomic.Int32
	sleeping := make(chan struct{})
	resume := make(chan struct{})

	tc := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"answers":{}}`))
	}, func(opts *nan.Options) {
		opts.Sleep = func(ctx context.Context, d time.Duration) error {
			close(sleeping)
			<-resume
			return nil
		}
	})
	c := tc.Client

	first := make(chan error, 1)
	go func() {
		_, err := c.SystemOne(context.Background(), nan.SystemOneRequest{
			Model: "clef", State: "x", Questions: sampleQuestions,
		})
		first <- err
	}()

	<-sleeping // the first caller is now in its backoff

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.SystemOne(ctx, nan.SystemOneRequest{
		Model: "clef", State: "x", Questions: sampleQuestions,
	}); err != nil {
		close(resume)
		t.Fatalf("the second caller could not use the wire during a backoff: %v", err)
	}
	close(resume)

	if err := <-first; err != nil {
		t.Fatalf("the retried caller failed: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("server saw %d requests, want 3 (1 failed + 1 from the other caller + 1 retry)", got)
	}
}
