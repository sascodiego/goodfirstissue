// Package store persists the frozen corpus and everything derived from it.
//
// Every write is atomic: a crashed run must never leave a half-written file that a later
// run reads as valid. Files are wrapped in an envelope carrying the schema version so a
// format change fails loudly instead of decoding into garbage.
package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SchemaVersion is the on-disk format. Bump it when a payload shape changes: an older
// file then refuses to load rather than decoding into something wrong (I4).
const SchemaVersion = 1

// Store writes into one directory. The zero value is unusable; use New.
type Store struct {
	dir string
}

// New returns a store rooted at dir. It does not create the directory.
func New(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("store: dir is required")
	}
	return &Store{dir: dir}, nil
}

// Dir is the root of the store.
func (s *Store) Dir() string { return s.dir }

// Path resolves a name relative to the store root. Names are always relative.
func (s *Store) Path(name string) string { return filepath.Join(s.dir, name) }

// Exists reports whether a stored file is present.
func (s *Store) Exists(name string) bool {
	info, err := os.Stat(s.Path(name))
	return err == nil && info.Mode().IsRegular()
}

type envelope struct {
	Schema  int             `json:"schema"`
	SavedAt time.Time       `json:"saved_at"`
	Data    json.RawMessage `json:"data"`
}

// Write stores v under name, atomically.
func (s *Store) Write(name string, v any) error {
	if name == "" {
		return errors.New("store: name is required")
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("store: name %q must be relative", name)
	}

	payload, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("store: encode %s: %w", name, err)
	}
	body, err := json.MarshalIndent(envelope{
		Schema:  SchemaVersion,
		SavedAt: time.Now().UTC(),
		Data:    payload,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("store: encode envelope %s: %w", name, err)
	}

	full := s.Path(name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("store: create dir for %s: %w", name, err)
	}

	// Write beside the target and rename: rename is atomic on one filesystem, so a
	// reader sees either the previous file or the new one, never a partial write.
	tmp, err := os.CreateTemp(filepath.Dir(full), ".tmp-"+filepath.Base(full)+"-*")
	if err != nil {
		return fmt.Errorf("store: temp for %s: %w", name, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(append(body, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("store: write %s: %w", name, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("store: sync %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("store: close %s: %w", name, err)
	}
	if err := os.Rename(tmpName, full); err != nil {
		return fmt.Errorf("store: rename into %s: %w", name, err)
	}
	return nil
}

// Read loads v from name. Unknown fields are rejected: a file written by a newer schema
// must not be read as if it were ours.
func (s *Store) Read(name string, v any) error {
	b, err := os.ReadFile(s.Path(name))
	if err != nil {
		return fmt.Errorf("store: read %s: %w", name, err)
	}

	var env envelope
	if err := json.Unmarshal(b, &env); err != nil {
		return fmt.Errorf("store: decode %s: %w", name, err)
	}
	if env.Schema != SchemaVersion {
		return fmt.Errorf("store: %s has schema %d, want %d: regenerate the corpus", name, env.Schema, SchemaVersion)
	}

	dec := json.NewDecoder(bytes.NewReader(env.Data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("store: decode %s payload: %w", name, err)
	}
	return nil
}

// SavedAt reports when name was written.
func (s *Store) SavedAt(name string) (time.Time, error) {
	b, err := os.ReadFile(s.Path(name))
	if err != nil {
		return time.Time{}, fmt.Errorf("store: read %s: %w", name, err)
	}
	var env envelope
	if err := json.Unmarshal(b, &env); err != nil {
		return time.Time{}, fmt.Errorf("store: decode %s: %w", name, err)
	}
	return env.SavedAt, nil
}
