package store

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"time"
)

// registryName lists the repositories the app knows about, so the list is a fact in the
// store instead of an inference from the directory layout.
const registryName = "repos.json"

// RepoEntry is one known repository.
type RepoEntry struct {
	Name      string    `json:"name"`
	Key       string    `json:"key"`
	CommitSHA string    `json:"commit_sha"`
	TotalOpen int       `json:"total_open"`
	Issues    int       `json:"issues"`
	FetchedAt time.Time `json:"fetched_at"`
}

type registry struct {
	Repos []RepoEntry `json:"repos"`
}

// SaveRepo records or updates one repository. An existing entry with the same key is
// replaced, never duplicated.
func (s *Store) SaveRepo(entry RepoEntry) error {
	if entry.Key == "" {
		return errors.New("store: repo entry has no key")
	}
	if entry.Name == "" {
		entry.Name = entry.Key
	}
	if entry.FetchedAt.IsZero() {
		entry.FetchedAt = time.Now().UTC()
	}

	lock, err := s.lock(registryName)
	if err != nil {
		return err
	}
	defer lock.release()

	current, err := s.readRegistry()
	if err != nil {
		return err
	}

	replaced := false
	for i := range current.Repos {
		if current.Repos[i].Key == entry.Key {
			current.Repos[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		current.Repos = append(current.Repos, entry)
	}
	sort.Slice(current.Repos, func(i, j int) bool { return current.Repos[i].Name < current.Repos[j].Name })

	return s.Write(registryName, current)
}

// RemoveRepo drops one repository from the registry, and does nothing when the key is not
// there.
//
// Existe por el renombrado: GitHub manda sobre el nombre, así que refrescar httpie/httpie
// guarda el snapshot bajo httpie__cli. Sin borrar la entrada vieja, el rail lista el mismo
// repositorio dos veces y una de las dos con un pin que ya no existe.
func (s *Store) RemoveRepo(key string) error {
	if key == "" {
		return errors.New("store: repo key is required")
	}

	lock, err := s.lock(registryName)
	if err != nil {
		return err
	}
	defer lock.release()

	current, err := s.readRegistry()
	if err != nil {
		return err
	}

	kept := current.Repos[:0]
	removed := false
	for _, entry := range current.Repos {
		if entry.Key == key {
			removed = true
			continue
		}
		kept = append(kept, entry)
	}
	if !removed {
		return nil
	}
	current.Repos = kept
	return s.Write(registryName, current)
}

// Repos lists the known repositories, ordered by name.
func (s *Store) Repos() ([]RepoEntry, error) {
	current, err := s.readRegistry()
	if err != nil {
		return nil, err
	}
	return current.Repos, nil
}

// Repo returns one known repository by store key.
func (s *Store) Repo(key string) (RepoEntry, bool, error) {
	repos, err := s.Repos()
	if err != nil {
		return RepoEntry{}, false, err
	}
	for _, entry := range repos {
		if entry.Key == key {
			return entry, true, nil
		}
	}
	return RepoEntry{}, false, nil
}

func (s *Store) readRegistry() (registry, error) {
	var current registry
	if !s.Exists(registryName) {
		return current, nil
	}
	if err := s.Read(registryName, &current); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return registry{}, nil
		}
		return current, fmt.Errorf("store: registry: %w", err)
	}
	return current, nil
}
