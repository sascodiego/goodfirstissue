package projectbrief

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// cacheName is the store document that keeps portraits.
const cacheName = "projectbrief/cache.json"

// cacheWrite serializes the cache read-modify-write across every Extractor in the process: the
// store is on disk, so a per-instance lock would still lose an entry between two of them. Cross
// process access is out of scope — one binary, one store.
var cacheWrite sync.Mutex

// cacheEntry is one portrait, kept with every fact that produced it, so an entry can be read back
// without guessing what wrote it.
type cacheEntry struct {
	Text          string    `json:"text"`
	Model         string    `json:"model"`
	PromptVersion string    `json:"prompt_version"`
	CommitSHA     string    `json:"commit_sha"`
	TookMs        int64     `json:"took_ms"`
	At            time.Time `json:"at"`
}

type cacheFile struct {
	Entries map[string]cacheEntry `json:"entries"`
}

// cacheKey names the question, not the answer: the repository, the commit it is checked out at,
// the model and the reading rules are everything that can change the portrait, so changing any of
// them must miss (I14). The commit is what makes a portrait stale the moment the tree moves.
func cacheKey(repository, commit, model, version string) string {
	sum := sha256.Sum256([]byte(repository + "\x00" + commit + "\x00" + model + "\x00" + version))
	return hex.EncodeToString(sum[:])[:16]
}

// lookup returns the entry stored for key, and only when it names the same tree, the same model
// and the same reading rules.
//
// The key already carries all three, so this is a second check on a file a person can edit: a
// portrait of another commit presented as fresh is the one failure I14 exists to prevent, and it
// costs nothing to refuse it. A miss, an unreadable file and a damaged entry are all the same
// thing here: ask the model.
func (e *Extractor) lookup(key, commit string) (cacheEntry, bool) {
	if e.store == nil {
		return cacheEntry{}, false
	}
	var file cacheFile
	if err := e.store.Read(cacheName, &file); err != nil {
		return cacheEntry{}, false
	}
	entry, ok := file.Entries[key]
	switch {
	case !ok || strings.TrimSpace(entry.Text) == "":
		return cacheEntry{}, false
	case entry.CommitSHA != commit:
		return cacheEntry{}, false
	case entry.Model != e.model:
		return cacheEntry{}, false
	case entry.PromptVersion != Version():
		return cacheEntry{}, false
	}
	return entry, true
}

// remember stores one portrait. The read-modify-write is serialized because two extractors can
// share a store; losing an entry costs a future run, not correctness.
func (e *Extractor) remember(key, commit string, result *Result) error {
	if e.store == nil {
		return nil
	}
	cacheWrite.Lock()
	defer cacheWrite.Unlock()

	var file cacheFile
	if err := e.store.Read(cacheName, &file); err != nil || file.Entries == nil {
		file = cacheFile{Entries: map[string]cacheEntry{}}
	}
	file.Entries[key] = cacheEntry{
		Text:          result.Document.Text,
		Model:         e.model,
		PromptVersion: Version(),
		CommitSHA:     commit,
		TookMs:        result.Took.Milliseconds(),
		At:            time.Now().UTC(),
	}
	if err := e.store.Write(cacheName, file); err != nil {
		return fmt.Errorf("projectbrief: cache: %w", err)
	}
	return nil
}
