package store

import (
	"fmt"
	"strings"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/indexer"
)

// Corpus names stay in one place so a rename cannot silently orphan older files.
const (
	snapshotName = "snapshot.json"
	indexName    = "index.json"
)

// SaveSnapshot stores a fetched snapshot under the repository's directory.
func (s *Store) SaveSnapshot(repo string, snap *ghdata.Snapshot) error {
	if snap == nil {
		return fmt.Errorf("store: nil snapshot for %s", repo)
	}
	return s.Write(snapshotNameFor(repo), snap)
}

// LoadSnapshot reads a previously stored snapshot.
func (s *Store) LoadSnapshot(repo string) (*ghdata.Snapshot, error) {
	var snap ghdata.Snapshot
	if err := s.Read(snapshotNameFor(repo), &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

// HasSnapshot reports whether the repository already has a frozen corpus.
func (s *Store) HasSnapshot(repo string) bool {
	return s.Exists(snapshotNameFor(repo))
}

// IndexRecord pins the index to the commit it was built from (I4): a graph of another
// commit is a graph of another program.
type IndexRecord struct {
	Repository string        `json:"repository"`
	CommitSHA  string        `json:"commit_sha"`
	Path       string        `json:"path"`
	Stats      indexer.Stats `json:"stats"`
}

// SaveIndex records the index statistics of one prepared clone.
func (s *Store) SaveIndex(repo string, rec IndexRecord) error {
	return s.Write(indexNameFor(repo), rec)
}

// LoadIndex reads the index record of one repository.
func (s *Store) LoadIndex(repo string) (IndexRecord, error) {
	var rec IndexRecord
	err := s.Read(indexNameFor(repo), &rec)
	return rec, err
}

func snapshotNameFor(repo string) string { return repo + "/" + snapshotName }
func indexNameFor(repo string) string    { return repo + "/" + indexName }

// RepoName is the on-disk directory name of one repository. It replaces the slash so the
// store has a flat, predictable layout.
func RepoName(owner, name string) string { return owner + "__" + name }

// RepoKey is the same on-disk name, computed from the canonical repository name GitHub hands
// out (owner/repo). It is the one place the conversion from the canonical name to the key
// lives: whatever holds a nameWithOwner converts here instead of joining it into a path, which
// would point at a directory no clone of ours has.
func RepoKey(canonical string) string {
	return strings.ReplaceAll(strings.TrimSpace(canonical), "/", "__")
}
