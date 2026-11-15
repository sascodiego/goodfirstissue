package profile

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// revisionsName is the store document that keeps every CV that was pasted, with what the model
// produced for it.
const revisionsName = "profile/revisions.json"

// revisionsWrite serializes the read-modify-write. The store is on disk, so two extractions
// sharing it would otherwise lose an entry; losing one would erase the history this document
// exists to keep.
var revisionsWrite sync.Mutex

// Revision is one pasted CV and what came out of it.
//
// It is written once and never edited: it answers "what did the model say about this text",
// which is what makes the history worth keeping. What the user approves is elsewhere, in the
// document in effect (Save), because "what the model said" and "what the person accepted" are
// two facts and one file cannot hold both without one of them lying.
type Revision struct {
	CV            string    `json:"cv"`
	Document      string    `json:"document"`
	Model         string    `json:"model"`
	PromptVersion string    `json:"prompt_version"`
	At            time.Time `json:"at"`
}

type revisionsFile struct {
	Entries []Revision `json:"entries"`
}

// SaveRevision appends one extraction to the history.
func SaveRevision(st *store.Store, rev Revision) error {
	switch {
	case st == nil:
		return errors.New("profile: a store is required")
	case strings.TrimSpace(rev.CV) == "":
		return errors.New("profile: a revision without the CV records nothing")
	case strings.TrimSpace(rev.Document) == "":
		return errors.New("profile: a revision without the document records nothing")
	}
	if rev.At.IsZero() {
		rev.At = time.Now().UTC()
	}

	revisionsWrite.Lock()
	defer revisionsWrite.Unlock()

	var file revisionsFile
	if err := st.Read(revisionsName, &file); err != nil && !errors.Is(err, os.ErrNotExist) {
		// A damaged history is not something to overwrite in silence: that would erase the
		// only copy of every past CV.
		return fmt.Errorf("profile: revisions: %w", err)
	}
	file.Entries = append(file.Entries, rev)
	if err := st.Write(revisionsName, file); err != nil {
		return fmt.Errorf("profile: revisions: %w", err)
	}
	return nil
}

// Revisions returns the history, oldest first.
func Revisions(st *store.Store) ([]Revision, error) {
	if st == nil {
		return nil, errors.New("profile: a store is required")
	}
	var file revisionsFile
	if err := st.Read(revisionsName, &file); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("profile: revisions: %w", err)
	}
	return file.Entries, nil
}

// CurrentRevision returns the CV in effect: the last one pasted, with the document it produced.
// ok is false before the first CV, which is the normal state of a fresh install and not an error.
func CurrentRevision(st *store.Store) (Revision, bool, error) {
	all, err := Revisions(st)
	if err != nil {
		return Revision{}, false, err
	}
	if len(all) == 0 {
		return Revision{}, false, nil
	}
	return all[len(all)-1], true, nil
}
