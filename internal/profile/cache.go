package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// cacheName is the store document that keeps extraction answers.
const cacheName = "profile/cache.json"

// cacheEntry is one answer, kept with the two facts that produced it so an entry can be read back
// without guessing what wrote it.
type cacheEntry struct {
	Answer        string `json:"answer"`
	Model         string `json:"model"`
	PromptVersion string `json:"prompt_version"`
}

type cacheFile struct {
	Entries map[string]cacheEntry `json:"entries"`
}

// cacheKey names the question, not the answer: the CV text, the model and the prompt version are
// everything that can change the document, so changing any of them must miss (I14).
//
// I14 lists five parts — issue, pass, model, prompt, commit — because it was written for issues.
// A CV has no issue and no commit, and the three that remain are the ones this extraction's own
// criterion names. This is a deliberate narrowing, not an oversight.
func cacheKey(cv, model, version string) string {
	sum := sha256.Sum256([]byte(cv + "\x00" + model + "\x00" + version))
	return hex.EncodeToString(sum[:])[:16]
}

// lookup returns the answer stored for key. A miss, an unreadable file and a damaged entry are all
// the same thing here: ask the model. The cache never becomes a source of truth.
func (e *Extractor) lookup(key string) (string, bool) {
	if e.store == nil {
		return "", false
	}
	var file cacheFile
	if err := e.store.Read(cacheName, &file); err != nil {
		return "", false
	}
	entry, ok := file.Entries[key]
	if !ok || entry.Answer == "" {
		return "", false
	}
	return entry.Answer, true
}

// remember stores one answer. The read-modify-write is serialized because two extractions can
// share an Extractor; losing an entry costs a future request, not correctness.
func (e *Extractor) remember(key, answer string) error {
	if e.store == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	var file cacheFile
	if err := e.store.Read(cacheName, &file); err != nil || file.Entries == nil {
		file = cacheFile{Entries: map[string]cacheEntry{}}
	}
	file.Entries[key] = cacheEntry{
		Answer:        answer,
		Model:         e.model,
		PromptVersion: Version(),
	}
	if err := e.store.Write(cacheName, file); err != nil {
		return fmt.Errorf("profile: cache: %w", err)
	}
	return nil
}
