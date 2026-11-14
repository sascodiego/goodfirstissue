package nan

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// MaxErrorBody bounds how much of a rejected response is kept. The body is evidence for
// the message the user sees, not a place to buffer a megabyte.
const MaxErrorBody = 512

// Kind says why a request failed, in terms a screen can show. It exists because I2
// forbids a verdict when Clef does not answer: the caller has to be able to say why it is
// answering `undecided`, and "error" is not a reason.
type Kind string

const (
	// KindConfig is a rejected setting: the client was never built.
	KindConfig Kind = "config"
	// KindRequest is a request this package refused to send, or could not encode.
	KindRequest Kind = "request"
	// KindUnauthorized is 401. docs/api.md claims 404 for a bad key; the service answers
	// 401 invalid_api_key. It is never retried: a rejected key does not recover.
	KindUnauthorized Kind = "unauthorized"
	// KindBadRequest is any other non-retryable 4xx: a malformed body, a wrong URL.
	KindBadRequest Kind = "bad_request"
	// KindRateLimited is 429, the "one request at a time" answer. It is retried once per
	// second up to the attempt ceiling (I1).
	KindRateLimited Kind = "rate_limited"
	// KindServer is a 5xx: the service is broken, not the request.
	KindServer Kind = "server"
	// KindNetwork is a transport failure that never reached an answer.
	KindNetwork Kind = "network"
	// KindTimeout is an attempt that ran out of time. The response that arrived late is
	// discarded, so this is a failure, never a partial success.
	KindTimeout Kind = "timeout"
	// KindCanceled is the caller giving up. Not retried, not a service problem.
	KindCanceled Kind = "canceled"
	// KindDecode is a 2xx whose body is not the documented envelope.
	KindDecode Kind = "decode"
)

// Error is the only failure type this package returns. Op names the call ("systemone",
// "chat"), Attempts counts what was actually sent, and Body keeps the API's own message:
// this is what lets the caller answer `undecided` with a motivo instead of a shrug.
type Error struct {
	Op     string
	Kind   Kind
	Status int
	Body   string
	// Attempts is the number of requests sent before giving up. Zero means nothing was
	// sent: the context was already done.
	Attempts int
	Err      error
}

// Error renders the failure the way the app shows it.
func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("nan: ")
	if e.Op != "" {
		b.WriteString(e.Op)
		b.WriteString(": ")
	}
	b.WriteString(string(e.Kind))
	if e.Status != 0 {
		fmt.Fprintf(&b, " (%d)", e.Status)
	}
	if e.Attempts > 1 {
		fmt.Fprintf(&b, " after %d attempts", e.Attempts)
	}
	if body := strings.TrimSpace(e.Body); body != "" {
		b.WriteString(": ")
		b.WriteString(body)
	}
	if e.Err != nil {
		b.WriteString(": ")
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

// Unwrap exposes the transport or decoding failure under the typed one.
func (e *Error) Unwrap() error { return e.Err }

// Retryable reports whether another attempt could help. It is exposed so a caller can
// explain a failure without re-deriving the policy from Status.
func (e *Error) Retryable() bool {
	switch e.Kind {
	case KindRateLimited, KindServer, KindNetwork, KindTimeout:
		return true
	default:
		return false
	}
}

// statusError maps a rejected status to its Kind and whether another attempt can help.
func statusError(op string, status int, raw []byte) (*Error, bool) {
	e := &Error{Op: op, Status: status, Body: errorBody(raw)}
	switch {
	case status == http.StatusUnauthorized:
		e.Kind = KindUnauthorized
		return e, false
	case status == http.StatusTooManyRequests:
		e.Kind = KindRateLimited
		return e, true
	case status >= 500:
		e.Kind = KindServer
		return e, true
	default:
		e.Kind = KindBadRequest
		return e, false
	}
}

// apiErrorEnvelope is the shape the service uses for rejections. The API's own message is
// what the user should read; the raw body is a fallback for anything unexpected.
type apiErrorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func errorBody(raw []byte) string {
	var env apiErrorEnvelope
	if err := json.Unmarshal(raw, &env); err == nil && strings.TrimSpace(env.Error.Message) != "" {
		return truncate(env.Error.Message, MaxErrorBody)
	}
	return truncate(string(raw), MaxErrorBody)
}

// truncate keeps a body readable and bounded. The ellipsis counts against the limit, so the
// result never exceeds it.
func truncate(s string, limit int) string {
	s = strings.TrimSpace(s)
	if len(s) <= limit {
		return s
	}
	const ellipsis = "…"
	cut := limit - len(ellipsis)
	// Cut on a rune boundary so a truncated body is still valid text.
	for cut > 0 && !utf8Start(s[cut]) {
		cut--
	}
	return s[:cut] + ellipsis
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }
