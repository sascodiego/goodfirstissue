// Package explore turns one issue into an evidence package by delegating to the
// read-only agent. It owns the prompt, the output contract and the validation of what
// comes back against the tool trace.
package explore

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sascodiego/nan-hackathon-clef/internal/pig"
)

// The contract always travels in the system prompt: a skill is an invitation the model
// may skip, and this is the part it cannot skip.
//
//go:embed prompt.md
var Prompt string

// The skill is the same doctrine in Pi-native form, registered so the agent can re-read
// it if it drifts.
//
//go:embed skills/explore/SKILL.md
var Skill string

// Relation values a file may claim. Only "observed" requires the trace to back it.
const (
	RelationObserved   = "observed"
	RelationProbable   = "probable"
	RelationUnverified = "unverified"
)

// File is one piece of code the issue touches.
type File struct {
	Path     string `json:"path"`
	Symbol   string `json:"symbol"`
	Line     int    `json:"line"`
	Reason   string `json:"reason"`
	Relation string `json:"relation"`
}

// Requirements are facts about the work, in three buckets that are acquired differently.
type Requirements struct {
	Knowledge []string `json:"knowledge"`
	Project   []string `json:"project"`
	Process   []string `json:"process"`
}

// BlastRadius is how far the change reaches.
type BlastRadius struct {
	Files     int  `json:"files"`
	Symbols   int  `json:"symbols"`
	PublicAPI bool `json:"public_api"`
}

// Evidence is the output contract of the exploration.
type Evidence struct {
	Intent             string       `json:"intent"`
	Files              []File       `json:"files"`
	CallPaths          [][]string   `json:"call_paths"`
	Tests              []string     `json:"tests"`
	Requirements       Requirements `json:"requirements"`
	BlastRadius        BlastRadius  `json:"blast_radius"`
	Unknowns           []string     `json:"unknowns"`
	MissingInformation []string     `json:"missing_information"`
}

// Prepare writes the skill into the agent's own directory so --skill has a real path.
// The prompt is passed inline, so it needs no file.
func Prepare(home string) (string, error) {
	dir := filepath.Join(home, "skills", "explore")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("explore: create %s: %w", dir, err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(Skill), 0o600); err != nil {
		return "", fmt.Errorf("explore: write %s: %w", path, err)
	}
	return path, nil
}

// PromptFor builds the task: the repository, the commit it is pinned to, and the issue verbatim.
// No labels: a label is another person's opinion about difficulty, and this pass establishes what
// the code requires, which no label can answer (§1).
func PromptFor(repository, commitSHA string, number int, title, body string) string {
	return fmt.Sprintf(`Repository: %s at commit %s

Issue #%d: %s

%s`, repository, commitSHA, number, title, body)
}

// Parse extracts and validates the evidence object from the agent's answer.
func Parse(answer string) (*Evidence, error) {
	start := strings.IndexByte(answer, '{')
	end := strings.LastIndexByte(answer, '}')
	if start < 0 || end <= start {
		return nil, errors.New("explore: the answer contains no JSON object")
	}

	var ev Evidence
	dec := json.NewDecoder(strings.NewReader(answer[start : end+1]))
	// The contract forbids extra fields; a stray one means the shape drifted.
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ev); err != nil {
		return nil, fmt.Errorf("explore: decode evidence: %w", err)
	}
	if err := ev.Validate(); err != nil {
		return nil, err
	}
	return &ev, nil
}

// Validate enforces the contract's hard rules.
func (e *Evidence) Validate() error {
	if strings.TrimSpace(e.Intent) == "" {
		return errors.New("explore: intent is empty")
	}
	if len(e.Files) == 0 {
		return errors.New("explore: files is empty: nothing was found")
	}
	for i, f := range e.Files {
		switch {
		case f.Path == "":
			return fmt.Errorf("explore: files[%d] has no path", i)
		case filepath.IsAbs(f.Path):
			return fmt.Errorf("explore: files[%d] path %q is absolute", i, f.Path)
		case strings.Contains(f.Path, ".."):
			return fmt.Errorf("explore: files[%d] path %q escapes the repository", i, f.Path)
		}
		switch f.Relation {
		case RelationObserved, RelationProbable, RelationUnverified:
		case "":
			// Un archivo sin relación declarada cae en el balde más débil: no se verificó.
			// Rechazar el paquete entero perdería una corrida de minutos por un campo que el
			// modelo omitió, y `unverified` nunca exagera lo que se sabe.
			e.Files[i].Relation = RelationUnverified
		default:
			return fmt.Errorf("explore: files[%d] relation %q is not one of observed/probable/unverified", i, f.Relation)
		}
	}
	return nil
}

// Traces flattens the tool calls into the text an observed claim is checked against.
// The arguments matter as much as the result: a read reports its path in the arguments.
func Traces(calls []pig.ToolCall) []string {
	out := make([]string, 0, len(calls)*2)
	for _, c := range calls {
		out = append(out, c.Name+" "+string(c.Args))
		if c.Result != "" {
			out = append(out, c.Result)
		}
	}
	return out
}

// Violation is an "observed" claim the trace does not support.
type Violation struct {
	Path string
}

// Verify checks every observed claim against what the tools actually returned. The agent
// is told the trace is checked; this is that check.
func (e *Evidence) Verify(traces []string) []Violation {
	corpus := strings.Join(traces, "\n")
	var out []Violation
	for _, f := range e.Files {
		if f.Relation != RelationObserved {
			continue
		}
		base := filepath.Base(f.Path)
		if !strings.Contains(corpus, f.Path) && !strings.Contains(corpus, base) {
			out = append(out, Violation{Path: f.Path})
		}
	}
	return out
}

// Metrics records how the run behaved in time, so an idle or stalled agent is visible in
// the stored artifact instead of being guessed at.
type Metrics struct {
	TotalMs    int64 `json:"total_ms"`
	FirstMs    int64 `json:"first_event_ms"`
	LongestGap int64 `json:"longest_gap_ms"`
	ToolCalls  int   `json:"tool_calls"`
	Stalled    bool  `json:"stalled"`
	TimedOut   bool  `json:"timed_out"`
	Complete   bool  `json:"complete"`
}

// Record is one stored exploration: the evidence plus everything needed to trust it.
type Record struct {
	Repository string      `json:"repository"`
	CommitSHA  string      `json:"commit_sha"`
	Issue      int         `json:"issue"`
	Model      string      `json:"model"`
	Evidence   *Evidence   `json:"evidence"`
	Violations []Violation `json:"violations"`
	Metrics    Metrics     `json:"metrics"`
}

// TaskContract names the shape of the task PromptFor builds. It reaches the fingerprint and
// never the model: a result produced from a differently shaped task answers another question,
// and reusing it would present a stale answer as fresh (I14).
const TaskContract = "task-v2-no-label"

// Version fingerprints the contract, the skill and the task builder. Results cached under one
// version miss under another, because a verdict produced by another question is not this one.
func Version() string {
	sum := sha256.Sum256([]byte(Prompt + "\x00" + Skill + "\x00" + TaskContract))
	return hex.EncodeToString(sum[:])[:12]
}
