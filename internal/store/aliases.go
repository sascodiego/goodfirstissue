package store

import (
	"errors"
	"fmt"
	"os"
)

// aliasesName keeps the renaming record at the store root, so every command resolves the
// same way without asking GitHub again. It is separate from the repository registry:
// renames and the list of known repositories are different facts.
const aliasesName = "aliases.json"

type aliasFile struct {
	// Map goes from a requested target ("owner/name") to GitHub's canonical one.
	Map map[string]string `json:"map"`
}

// SaveAlias records that requested is known to GitHub as canonical. Repositories get
// renamed; without this, one repository would be fetched under two names.
func (s *Store) SaveAlias(requested, canonical string) error {
	if requested == canonical {
		return nil
	}
	current, err := s.loadAliases()
	if err != nil {
		return err
	}
	if current.Map == nil {
		current.Map = make(map[string]string)
	}
	current.Map[requested] = canonical
	return s.Write(aliasesName, current)
}

// Alias returns the canonical target for a requested one, if it was ever renamed.
func (s *Store) Alias(requested string) (string, bool) {
	current, err := s.loadAliases()
	if err != nil || current.Map == nil {
		return "", false
	}
	canonical, ok := current.Map[requested]
	return canonical, ok
}

func (s *Store) loadAliases() (aliasFile, error) {
	var file aliasFile
	if !s.Exists(aliasesName) {
		return file, nil
	}
	if err := s.Read(aliasesName, &file); err != nil && !errors.Is(err, os.ErrNotExist) {
		return file, fmt.Errorf("store: aliases: %w", err)
	}
	return file, nil
}
