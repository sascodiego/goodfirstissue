// Package nan speaks the NaN HTTP API.
//
// It covers the two endpoints the application needs and nothing else:
//
//   - POST /v1/systemone answers decision questions with Clef. One request at a time,
//     always (I1): the queue lives here, in the client, so three screens asking for triage
//     still put a single request on the wire without knowing about each other.
//   - POST /v1/chat/completions runs the allowlisted chat models in parallel. Four
//     simultaneous requests were measured answering 200, so this path is not serialized.
//
// The two regimes are deliberately not the same. Confusing them would either serialize the
// exploration for nothing or eat 429 during the demo (PROYECTO.md §6.9).
//
// This package never decides which model to use: the request carries it, and the
// constitutional allowlist belongs to internal/config (I13). It also never reads the
// environment: the key arrives in Options.
package nan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultSystemOneURL and DefaultChatURL are the documented endpoints.
	DefaultSystemOneURL = "https://api.nan.builders/v1/systemone"
	DefaultChatURL      = "https://api.nan.builders/v1/chat/completions"

	// MinTimeout is the API's documented minimum of 50 s. A shorter client timeout aborts
	// requests the service is still answering, which looks like a service failure and is
	// not one.
	MinTimeout = 50 * time.Second
	// DefaultTimeout applies when Options.Timeout is zero.
	DefaultTimeout = 60 * time.Second

	// DefaultMaxAttempts is I1's ceiling: "máximo 5 intentos". Attempts, not retries.
	DefaultMaxAttempts = 5

	// retryBase is the first wait after a 5xx or a network failure; it doubles per attempt.
	retryBase = 500 * time.Millisecond
	// maxRetryWait caps that growth so one call cannot stall a screen for minutes.
	maxRetryWait = 8 * time.Second
	// rateLimitWait is I1's fixed wait for a 429. It is not exponential: the API asks for a
	// second, and a second is what it gets.
	rateLimitWait = time.Second

	// readLimit bounds how much of a response body is read. Answers are small; a body past
	// this is truncated, and a truncated answer fails to decode rather than silently
	// reaching a decision.
	readLimit = 1 << 20
)

// Options configures a Client. Only APIKey, Timeout and MaxAttempts are required to be
// explicit in spirit: a partial Options takes documented defaults, the way config.json
// takes default values for omitted fields.
type Options struct {
	// SystemOneURL and ChatURL default to the documented endpoints.
	SystemOneURL string
	ChatURL      string

	// APIKey is the NaN key. The caller reads it from the environment (NAN_KEY); this
	// package never touches the environment and never writes the key to disk.
	APIKey string

	// Timeout bounds one attempt. It is the caller's per-endpoint setting: the two
	// endpoints do not share a budget, and binding the chat path to clef.timeout is a
	// mistake this field does not make for you. Values below MinTimeout are rejected.
	Timeout time.Duration

	// MaxAttempts is the total number of attempts, not the retries after the first. Zero
	// takes DefaultMaxAttempts; one disables retries.
	MaxAttempts int

	// HTTPClient overrides the transport. Nil builds one with Timeout.
	HTTPClient *http.Client

	// Sleep waits between attempts. Nil uses a real timer. Tests inject it so the backoff
	// schedule is observable without spending the seconds it describes.
	Sleep func(ctx context.Context, d time.Duration) error
}

// Client is safe for concurrent use. Both endpoints share it; only /v1/systemone is
// serialized.
type Client struct {
	systemOneURL string
	chatURL      string
	apiKey       string
	maxAttempts  int
	http         *http.Client
	sleep        func(context.Context, time.Duration) error

	// one is the single-in-flight slot for /v1/systemone. A channel, not a mutex, so that
	// waiting for the wire honors the caller's context instead of blocking it.
	one chan struct{}
}

// New validates the settings and builds a client. Every rejection happens here, at
// construction, and not twenty calls later (§17.7).
func New(opts Options) (*Client, error) {
	cfgErr := func(format string, a ...any) error {
		return &Error{Op: "new", Kind: KindConfig, Err: fmt.Errorf(format, a...)}
	}

	switch {
	case strings.TrimSpace(opts.APIKey) == "":
		return nil, cfgErr("api_key is required: the caller reads NAN_KEY and passes it here")
	case opts.Timeout < 0:
		return nil, cfgErr("timeout %s must not be negative", opts.Timeout)
	case opts.Timeout > 0 && opts.Timeout < MinTimeout:
		return nil, cfgErr("timeout %s is below the API minimum %s", opts.Timeout, MinTimeout)
	case opts.MaxAttempts < 0:
		return nil, cfgErr("max_attempts %d must not be negative", opts.MaxAttempts)
	}

	systemOneURL := opts.SystemOneURL
	if systemOneURL == "" {
		systemOneURL = DefaultSystemOneURL
	}
	chatURL := opts.ChatURL
	if chatURL == "" {
		chatURL = DefaultChatURL
	}
	for _, u := range []string{systemOneURL, chatURL} {
		if err := validURL(u); err != nil {
			return nil, cfgErr("%v", err)
		}
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	attempts := opts.MaxAttempts
	if attempts == 0 {
		attempts = DefaultMaxAttempts
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	sleep := opts.Sleep
	if sleep == nil {
		sleep = sleepContext
	}

	return &Client{
		systemOneURL: systemOneURL,
		chatURL:      chatURL,
		apiKey:       opts.APIKey,
		maxAttempts:  attempts,
		http:         client,
		sleep:        sleep,
		one:          make(chan struct{}, 1),
	}, nil
}

func validURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("url %q is not a URL: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url %q must be http or https", raw)
	}
	if u.Host == "" {
		return fmt.Errorf("url %q has no host", raw)
	}
	return nil
}

// Timeout returns the per-attempt timeout the client was built with.
func (c *Client) Timeout() time.Duration { return c.http.Timeout }

// MaxAttempts returns the total attempt ceiling the client was built with.
func (c *Client) MaxAttempts() int { return c.maxAttempts }

// post sends one JSON request and decodes the answer, retrying where the policy allows.
// serial makes the call hold the single-in-flight slot for each attempt.
func (c *Client) post(ctx context.Context, op, target string, body, out any, serial bool) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return &Error{Op: op, Kind: KindRequest, Err: err}
	}

	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return contextError(op, attempt-1, err)
		}

		if serial {
			if err := c.acquire(ctx); err != nil {
				return contextError(op, attempt-1, err)
			}
		}
		failure, retry := c.attempt(ctx, op, target, payload, out)
		if serial {
			c.release()
		}
		if failure == nil {
			return nil
		}
		failure.Attempts = attempt
		if !retry || attempt == c.maxAttempts {
			return failure
		}
		if err := c.sleep(ctx, waitFor(failure, attempt)); err != nil {
			return contextError(op, attempt, err)
		}
	}
	return &Error{Op: op, Kind: KindRequest, Err: errors.New("no attempt was made")}
}

// attempt sends exactly one request.
func (c *Client) attempt(ctx context.Context, op, target string, payload []byte, out any) (*Error, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return &Error{Op: op, Kind: KindRequest, Err: err}, false
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return transportError(op, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, readLimit))

	if resp.StatusCode/100 != 2 {
		return statusError(op, resp.StatusCode, raw)
	}
	if readErr != nil {
		return &Error{Op: op, Kind: KindNetwork, Status: resp.StatusCode, Err: readErr}, true
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return &Error{
			Op:     op,
			Kind:   KindDecode,
			Status: resp.StatusCode,
			Body:   truncate(string(raw), MaxErrorBody),
			Err:    err,
		}, false
	}
	return nil, false
}

// transportError classifies a failure that produced no response.
func transportError(op string, err error) (*Error, bool) {
	switch {
	case errors.Is(err, context.Canceled):
		return &Error{Op: op, Kind: KindCanceled, Err: err}, false
	case errors.Is(err, context.DeadlineExceeded):
		// The response that arrives late is discarded, so the attempt failed; another
		// attempt is worth making when the caller's own deadline is what expired, the loop
		// top notices before sending again.
		return &Error{Op: op, Kind: KindTimeout, Err: err}, true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &Error{Op: op, Kind: KindTimeout, Err: err}, true
	}
	return &Error{Op: op, Kind: KindNetwork, Err: err}, true
}

// contextError reports the caller going away. It is never retried: the context is already
// done, so another attempt could not run.
func contextError(op string, attempts int, err error) *Error {
	kind := KindCanceled
	if errors.Is(err, context.DeadlineExceeded) {
		kind = KindTimeout
	}
	return &Error{Op: op, Kind: kind, Attempts: attempts, Err: err}
}

// waitFor is the retry schedule. A 429 gets I1's fixed second; everything else backs off
// exponentially. No jitter: the queue keeps a single request on the wire, so there is no
// herd to spread, and a deterministic schedule is one more thing a test can pin.
func waitFor(e *Error, attempt int) time.Duration {
	if e.Kind == KindRateLimited {
		return rateLimitWait
	}
	wait := retryBase << (attempt - 1)
	if wait > maxRetryWait || wait <= 0 {
		return maxRetryWait
	}
	return wait
}

// acquire takes the single-in-flight slot, or gives up when the caller does.
func (c *Client) acquire(ctx context.Context) error {
	select {
	case c.one <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) release() { <-c.one }

// sleepContext is the default wait: a timer that surrenders to the caller's context.
func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
