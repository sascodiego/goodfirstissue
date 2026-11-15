package triage

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// stubBrief is one BriefSource that answers whatever a test fixes, and counts the questions it was
// asked: the fallback contract is about who answered, so the test has to know.
type stubBrief struct {
	text  string
	err   error
	calls int
}

func (s *stubBrief) Brief(_ context.Context, _ string) (string, error) {
	s.calls++
	if s.err != nil {
		return "", s.err
	}
	return s.text, nil
}

// The primary wins whenever it answers: the portrait is the richer input, and the fallback is only
// there for the times it cannot be produced.
func TestFallbackBriefPrefersThePrimary(t *testing.T) {
	primary := &stubBrief{text: "the portrait"}
	fallback := &stubBrief{text: "the README brief"}
	var reasons []error

	got, err := FallbackBrief{
		Primary:   primary,
		Fallback:  fallback,
		OnFailure: func(err error) { reasons = append(reasons, err) },
	}.Brief(context.Background(), "spf13__cobra")

	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if got != "the portrait" {
		t.Errorf("brief = %q, want the primary's", got)
	}
	if fallback.calls != 0 {
		t.Errorf("the fallback answered %d times, want 0", fallback.calls)
	}
	if len(reasons) != 0 {
		t.Errorf("nothing failed, but the failure hook was called with %v", reasons)
	}
}

// A triage pass over a hundred issues must not be lost because the portrait could not be produced:
// the node reads the cheaper brief, and the caller is told which one it read and why.
func TestFallbackBriefFallsBackAndSaysWhy(t *testing.T) {
	cause := errors.New("the agent stalled")
	fallback := &stubBrief{text: "the README brief"}
	var reasons []error

	got, err := FallbackBrief{
		Primary:   &stubBrief{err: cause},
		Fallback:  fallback,
		OnFailure: func(err error) { reasons = append(reasons, err) },
	}.Brief(context.Background(), "spf13__cobra")

	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if got != "the README brief" {
		t.Errorf("brief = %q, want the fallback's", got)
	}
	if len(reasons) != 1 || !errors.Is(reasons[0], cause) {
		t.Errorf("failure reasons = %v, want the cause once", reasons)
	}
}

// An empty answer is a failure even without an error: a node fed a blank section would judge on
// nothing and report nothing.
func TestFallbackBriefFallsBackOnAnEmptyAnswer(t *testing.T) {
	fallback := &stubBrief{text: "the README brief"}
	var reasons []error

	got, err := FallbackBrief{
		Primary:   &stubBrief{text: "   \n"},
		Fallback:  fallback,
		OnFailure: func(err error) { reasons = append(reasons, err) },
	}.Brief(context.Background(), "spf13__cobra")

	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if got != "the README brief" {
		t.Errorf("brief = %q", got)
	}
	if len(reasons) != 1 || !strings.Contains(reasons[0].Error(), "nothing") {
		t.Errorf("failure reasons = %v, want one saying the primary answered nothing", reasons)
	}
}

func TestFallbackBriefWithoutAPrimaryIsTheFallback(t *testing.T) {
	fallback := &stubBrief{text: "the README brief"}
	called := false

	got, err := FallbackBrief{
		Fallback:  fallback,
		OnFailure: func(error) { called = true },
	}.Brief(context.Background(), "spf13__cobra")

	if err != nil || got != "the README brief" {
		t.Fatalf("got %q, err %v", got, err)
	}
	if called {
		t.Error("a source that was never configured did not fail: nobody should be warned")
	}
}

// Both sources failing is the condition the node already handles: it is not asked, and the feature
// is conditioned saying so.
func TestFallbackBriefReportsWhenBothFail(t *testing.T) {
	fallbackErr := errors.New("the clone is not there")
	_, err := FallbackBrief{
		Primary:  &stubBrief{err: errors.New("the agent stalled")},
		Fallback: &stubBrief{err: fallbackErr},
	}.Brief(context.Background(), "spf13__cobra")

	if !errors.Is(err, fallbackErr) {
		t.Fatalf("err = %v, want the fallback's failure", err)
	}
}

func TestFallbackBriefNeedsAFallback(t *testing.T) {
	if _, err := (FallbackBrief{Primary: &stubBrief{text: "x"}}).Brief(context.Background(), "a__b"); err == nil {
		t.Fatal("a fallback brief without a fallback must refuse")
	}
}
