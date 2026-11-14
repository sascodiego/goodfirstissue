package profile

import (
	"errors"
	"fmt"
	"os"

	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// currentName is the store document that holds the profile in effect.
const currentName = "profile/current.json"

type savedFile struct {
	Document string `json:"document"`
}

// Save stores the profile in effect.
//
// What is saved is the text the user approved, not the raw extraction: the document is reviewed
// and corrected before it is used (§17.8), so this is the only artifact `fit` may read. It is
// checked with the same skeleton a generated one gets, because a hand edit can break the shape the
// screen renders by.
func Save(st *store.Store, document string) error {
	if st == nil {
		return errors.New("profile: a store is required")
	}
	parsed, err := Parse(document)
	if err != nil {
		return err
	}
	if parsed.Insufficient {
		return errors.New("profile: nothing to save: the document carries no item")
	}
	if err := st.Write(currentName, savedFile{Document: document}); err != nil {
		return fmt.Errorf("profile: save: %w", err)
	}
	return nil
}

// Load returns the profile in effect. ok is false when nothing was saved yet, which is the normal
// state before the first CV and not an error.
func Load(st *store.Store) (document string, ok bool, err error) {
	if st == nil {
		return "", false, errors.New("profile: a store is required")
	}
	var file savedFile
	if err := st.Read(currentName, &file); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("profile: load: %w", err)
	}
	return file.Document, file.Document != "", nil
}
