package profile_test

import (
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/profile"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

func storeForTest(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	return st
}

func revision(cv, document string) profile.Revision {
	return profile.Revision{
		CV:            cv,
		Document:      document,
		Model:         "glm5.3-flash",
		PromptVersion: profile.Version(),
		At:            time.Date(2026, 11, 14, 9, 30, 0, 0, time.UTC),
	}
}

// S13: a resent CV leaves the previous one in history. It is kept as evidence of what the
// model produced for that text, and it is not usable in the MVP.
func TestSaveRevisionKeepsEveryPastCV(t *testing.T) {
	st := storeForTest(t)

	if err := profile.SaveRevision(st, revision("primer CV", validDocument)); err != nil {
		t.Fatalf("SaveRevision() error = %v", err)
	}
	if err := profile.SaveRevision(st, revision("segundo CV", validDocument)); err != nil {
		t.Fatalf("SaveRevision() error = %v", err)
	}

	all, err := profile.Revisions(st)
	if err != nil {
		t.Fatalf("Revisions() error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("revisions = %d, want 2", len(all))
	}
	if all[0].CV != "primer CV" || all[1].CV != "segundo CV" {
		t.Errorf("revisions = %q, %q, want the past CV kept first", all[0].CV, all[1].CV)
	}
}

// S6: the CV in effect is the last one pasted, with the document it produced.
func TestCurrentRevisionIsTheLastOneSaved(t *testing.T) {
	st := storeForTest(t)

	if _, ok, err := profile.CurrentRevision(st); ok || err != nil {
		t.Fatalf("CurrentRevision() on an empty store = %v, %v; want no revision and no error", ok, err)
	}
	if err := profile.SaveRevision(st, revision("primer CV", validDocument)); err != nil {
		t.Fatal(err)
	}
	want := revision("segundo CV", "## Idiomas\n- Inglés — nivel: declarado — evidencia: «inglés»\n")
	want.At = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	if err := profile.SaveRevision(st, want); err != nil {
		t.Fatal(err)
	}

	got, ok, err := profile.CurrentRevision(st)
	if err != nil || !ok {
		t.Fatalf("CurrentRevision() = %v, %v", ok, err)
	}
	if got.CV != want.CV || got.Document != want.Document {
		t.Errorf("current = %+v, want %+v", got, want)
	}
	if !got.At.Equal(want.At) {
		t.Errorf("at = %v, want %v: the screen shows when the CV was stored", got.At, want.At)
	}
}

// A revision without a CV or a document records nothing: an empty entry would show up in the
// history count and mean nothing.
func TestSaveRevisionRefusesAnEmptyRecord(t *testing.T) {
	st := storeForTest(t)

	if err := profile.SaveRevision(nil, revision("cv", validDocument)); err == nil {
		t.Error("SaveRevision() accepted a nil store")
	}
	if err := profile.SaveRevision(st, revision("  ", validDocument)); err == nil {
		t.Error("SaveRevision() accepted an empty CV")
	}
	if err := profile.SaveRevision(st, revision("cv", " \n")); err == nil {
		t.Error("SaveRevision() accepted an empty document")
	}
}
