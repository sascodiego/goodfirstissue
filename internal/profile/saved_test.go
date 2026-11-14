package profile_test

import (
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/profile"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	return st
}

// The profile in effect is the text the user approved, not the raw extraction: it survives a
// restart and comes back verbatim, including whatever the user edited.
func TestSaveAndLoadRoundTrip(t *testing.T) {
	st := newStore(t)

	if err := profile.Save(st, validDocument); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, ok, err := profile.Load(st)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !ok {
		t.Fatal("Load() reports no saved profile")
	}
	if got != validDocument {
		t.Errorf("Load() = %q, want the saved document verbatim", got)
	}
}

// A later approval replaces the earlier one.
func TestSaveReplacesThePreviousProfile(t *testing.T) {
	st := newStore(t)
	first := "## Conocimientos técnicos\n- Go — nivel: declarado — evidencia: «Go»\n"
	second := "## Conocimientos técnicos\n- Python — nivel: declarado — evidencia: «Python»\n"

	if err := profile.Save(st, first); err != nil {
		t.Fatalf("Save(first) error = %v", err)
	}
	if err := profile.Save(st, second); err != nil {
		t.Fatalf("Save(second) error = %v", err)
	}
	got, _, err := profile.Load(st)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != second {
		t.Errorf("Load() = %q, want the second document", got)
	}
}

// Nothing saved is not a failure: the screen opens empty, it does not show an error.
func TestLoadWithoutASavedProfileIsNotAnError(t *testing.T) {
	got, ok, err := profile.Load(newStore(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if ok || got != "" {
		t.Errorf("Load() = %q/%v, want no profile and no error", got, ok)
	}
}

// What gets saved has to be renderable later, so an edited document is checked with the same
// skeleton as a generated one.
func TestSaveRejectsAnUnreadableDocument(t *testing.T) {
	if err := profile.Save(newStore(t), "me pareció un buen candidato, tiene experiencia"); err == nil {
		t.Fatal("Save() accepted prose")
	}
}

// There is nothing to approve in a profile that says the CV carried nothing: saving it would
// present an extraction that never happened as the profile in effect.
func TestSaveRejectsAnInsufficientProfile(t *testing.T) {
	if err := profile.Save(newStore(t), profile.Insufficient); err == nil {
		t.Fatal("Save() accepted an insufficient profile")
	}
}

func TestSaveRequiresAStore(t *testing.T) {
	if err := profile.Save(nil, validDocument); err == nil {
		t.Fatal("Save() accepted no store")
	}
	if _, _, err := profile.Load(nil); err == nil {
		t.Fatal("Load() accepted no store")
	}
}
