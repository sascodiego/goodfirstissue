package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"
)

// Pass is one analysis step whose result can be reused.
type Pass string

// The decision passes are fixed to clef by the constitution; explore is the only one that
// runs another model. They are named here so a stored verdict says which question it
// answers.
const (
	PassTriage      Pass = "triage"
	PassAlignment   Pass = "alignment"
	PassLabelHonest Pass = "label_honest"
	PassFit         Pass = "fit"
	PassExplore     Pass = "explore"
)

// Key identifies one finished piece of analysis.
//
// Everything that can change the answer lives in the key. A verdict produced by another
// prompt, another model, or another commit answers a different question, so it must miss
// instead of being silently reused (I4).
type Key struct {
	Issue         int    `json:"issue"`
	Pass          Pass   `json:"pass"`
	Model         string `json:"model"`
	PromptVersion string `json:"prompt_version"`
	CommitSHA     string `json:"commit_sha"`
}

// ID is the journal's lookup key. It must change whenever any field changes.
func (k Key) ID() string {
	return fmt.Sprintf("%d|%s|%s|%s|%s", k.Issue, k.Pass, k.Model, k.PromptVersion, k.CommitSHA)
}

// Decision is one stored analysis result. The verdict stays opaque on purpose: the store
// has no business knowing what a triage looks like.
type Decision struct {
	Key
	At        time.Time       `json:"at"`
	LatencyMs int64           `json:"latency_ms"`
	Verdict   json.RawMessage `json:"verdict"`
}

// Decisions is one repository's journal projected in memory: read once, queried many
// times, written through to disk on every Put.
//
// It is safe for concurrent use: HTTP handlers serve reads while an analysis writes.
type Decisions struct {
	store *Store
	repo  string

	mu    sync.RWMutex
	byKey map[string]Decision
	lines int
}

// Decisions loads a repository's journal. A missing journal is not an error: it means
// nothing has been analysed yet.
func (s *Store) Decisions(repo string) (*Decisions, error) {
	d := &Decisions{store: s, repo: repo, byKey: make(map[string]Decision)}

	b, err := os.ReadFile(s.Path(decisionsNameFor(repo)))
	if errors.Is(err, os.ErrNotExist) {
		return d, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: read journal of %s: %w", repo, err)
	}

	var raw []string
	for _, line := range bytes.Split(b, []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			raw = append(raw, string(line))
		}
	}
	for i, line := range raw {
		var dec Decision
		if err := json.Unmarshal([]byte(line), &dec); err != nil {
			// A crash mid-append can only damage the last line, and that line was never
			// acknowledged. Anywhere else, the journal is corrupt and must not be trusted.
			if i == len(raw)-1 {
				break
			}
			return nil, fmt.Errorf("store: journal of %s line %d: %w", repo, i+1, err)
		}
		d.byKey[dec.ID()] = dec
		d.lines++
	}
	return d, nil
}

// Get returns a finished analysis, if this exact question was already answered.
func (d *Decisions) Get(k Key) (Decision, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	dec, ok := d.byKey[k.ID()]
	return dec, ok
}

// Len is how many distinct analyses are reusable.
func (d *Decisions) Len() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.byKey)
}

// Put appends one result and makes it reusable. The write is durable before it returns, so
// a result that was reported is never lost by a restart.
func (d *Decisions) Put(dec Decision) error {
	if len(dec.Verdict) == 0 {
		return errors.New("store: decision has no verdict")
	}
	if dec.Issue <= 0 {
		return errors.New("store: decision has no issue")
	}
	if dec.Pass == "" {
		return errors.New("store: decision has no pass")
	}
	if dec.At.IsZero() {
		dec.At = time.Now().UTC()
	}

	// Hold the lock across the append: two writers must not interleave a line.
	d.mu.Lock()
	defer d.mu.Unlock()

	line, err := json.Marshal(dec)
	if err != nil {
		return fmt.Errorf("store: encode decision: %w", err)
	}
	if err := d.store.appendLine(decisionsNameFor(d.repo), line); err != nil {
		return err
	}
	d.byKey[dec.ID()] = dec
	d.lines++
	return nil
}

// Compact rewrites the journal with one line per live key once it holds more than twice as
// many lines as reusable analyses. Reports whether it did anything.
func (d *Decisions) Compact() (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.lines < 64 || d.lines <= 2*len(d.byKey) {
		return false, nil
	}

	keys := make([]string, 0, len(d.byKey))
	for k := range d.byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		line, err := json.Marshal(d.byKey[k])
		if err != nil {
			return false, fmt.Errorf("store: encode decision: %w", err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}

	name := decisionsNameFor(d.repo)
	if err := d.store.writeRaw(name, buf.Bytes()); err != nil {
		return false, err
	}
	d.lines = len(d.byKey)
	return true, nil
}

func decisionsNameFor(repo string) string { return repo + "/decisions.jsonl" }

// appendLine adds one line under an exclusive lock. Separate processes (the server and a
// CLI command) may append at the same time; without the lock they would interleave.
func (s *Store) appendLine(name string, line []byte) error {
	full := s.Path(name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("store: create dir for %s: %w", name, err)
	}

	lock, err := s.lock(name)
	if err != nil {
		return err
	}
	defer lock.release()

	f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("store: open %s: %w", name, err)
	}
	defer func() { _ = f.Close() }()

	// One write ending in a newline: a torn write can only damage the last line.
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("store: append %s: %w", name, err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("store: sync %s: %w", name, err)
	}
	return nil
}

// writeRaw replaces a file atomically, for formats that are not one envelope.
func (s *Store) writeRaw(name string, body []byte) error {
	full := s.Path(name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("store: create dir for %s: %w", name, err)
	}

	lock, err := s.lock(name)
	if err != nil {
		return err
	}
	defer lock.release()

	tmp, err := os.CreateTemp(filepath.Dir(full), ".tmp-"+filepath.Base(full)+"-*")
	if err != nil {
		return fmt.Errorf("store: temp for %s: %w", name, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(body); err != nil {
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

type fileLock struct{ f *os.File }

// lock takes an exclusive advisory lock on name+".lock". It is released when the returned
// fileLock is released, which must happen before the process exits.
func (s *Store) lock(name string) (*fileLock, error) {
	path := s.Path(name + ".lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("store: create dir for %s: %w", name, err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("store: open lock for %s: %w", name, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("store: lock %s: %w", name, err)
	}
	return &fileLock{f: f}, nil
}

func (l *fileLock) release() {
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	_ = l.f.Close()
}
