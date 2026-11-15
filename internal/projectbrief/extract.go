package projectbrief

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/pig"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// RunFunc is one agent run. pig.Run is the real one; a test injects a fake so no test starts a
// process (S8). This is the transport seam: the prompt, the parser and the pinning are the same
// whatever runs the model (I7).
type RunFunc func(ctx context.Context, req pig.Request) (*pig.Outcome, error)

// Options configures an Extractor.
//
// The model is not validated here: the constitutional allowlist belongs to internal/config (I13),
// and a second copy would drift from it. What this package refuses is a run with no model at all.
type Options struct {
	// Run executes one portrait. Required.
	Run RunFunc
	// Model is the extraction model, read by the caller from config.models.project_brief.
	Model string
	// Binary is the agent executable. Empty means "pig" from PATH.
	Binary string
	// Home is this application's PIG_HOME for this job. Required: PiG's configuration is
	// generated there and must never be inherited from a user's ~/.pig. It is separate from the
	// exploration home because both jobs rewrite it on every run.
	Home string
	// Tools is the complete read surface the agent gets. Required: a portrait of a repository
	// that cannot be read is a portrait invented from the title.
	Tools []string
	// IdleWarn, IdleKill and Budget bound the run. Zero takes PiG's defaults.
	IdleWarn time.Duration
	IdleKill time.Duration
	Budget   time.Duration
	// Store keeps the portraits under projectbrief/cache.json, keyed by the commit they read.
	// Nil disables caching, which is what a test that counts runs wants.
	Store *store.Store
}

// Input names what is being described: one repository, at one commit, in one clone.
//
// The commit is required. A portrait that cannot say which tree it read cannot be told apart from
// a stale one, and I14 says a result is pinned to the commit or it is not a result.
type Input struct {
	Repository string
	CommitSHA  string
	Dir        string
	// Force ignores a stored portrait and runs again. It is what a caller asks for when the tree
	// changed without moving the commit: the question is the same and the answer is not. The
	// fresh answer replaces the stored one, so the next reader is not served the older portrait.
	Force bool
}

// validate refuses an input that cannot produce a pinned portrait.
func (in Input) validate() error {
	switch {
	case strings.TrimSpace(in.Repository) == "":
		return errors.New("projectbrief: a repository is required: the portrait names what it describes")
	case strings.TrimSpace(in.CommitSHA) == "":
		return errors.New("projectbrief: a commit is required: a portrait is pinned to the tree it read (I14)")
	case strings.TrimSpace(in.Dir) == "":
		return errors.New("projectbrief: a directory is required: the portrait is read from a clone")
	}
	info, err := os.Stat(in.Dir)
	if err != nil {
		return fmt.Errorf("projectbrief: clone %s: %w", in.Dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("projectbrief: clone %s is not a directory", in.Dir)
	}
	return nil
}

// Extractor turns one cloned repository into one portrait. It is safe for concurrent use.
type Extractor struct {
	run      RunFunc
	model    string
	binary   string
	home     string
	tools    []string
	idleWarn time.Duration
	idleKill time.Duration
	budget   time.Duration
	store    *store.Store

	// one is the single-in-flight slot. Two runs at once would read the same clone twice and
	// rewrite the same PIG_HOME twice, which is the shape of a corrupted agent home and a double
	// budget charge for one artifact.
	one chan struct{}
}

// New validates the settings.
func New(opts Options) (*Extractor, error) {
	switch {
	case opts.Run == nil:
		return nil, errors.New("projectbrief: a run function is required: pig.Run is the real one")
	case strings.TrimSpace(opts.Model) == "":
		return nil, errors.New("projectbrief: a model is required: the caller reads config.models.project_brief")
	case strings.TrimSpace(opts.Home) == "":
		return nil, errors.New("projectbrief: a home is required: the agent's configuration is generated there")
	case len(opts.Tools) == 0:
		return nil, errors.New("projectbrief: a tool list is required: the agent has to read the clone")
	}
	// PiG resolves its configuration from its own working directory, and this job makes that
	// directory the home: a relative home would be looked up twice and the agent would fail
	// before reaching the model.
	home, err := filepath.Abs(opts.Home)
	if err != nil {
		return nil, fmt.Errorf("projectbrief: home %q: %w", opts.Home, err)
	}
	return &Extractor{
		run:      opts.Run,
		model:    opts.Model,
		binary:   opts.Binary,
		home:     home,
		tools:    append([]string(nil), opts.Tools...),
		idleWarn: opts.IdleWarn,
		idleKill: opts.IdleKill,
		budget:   opts.Budget,
		store:    opts.Store,
		one:      make(chan struct{}, 1),
	}, nil
}

// Result is one portrait, with the facts a consumer needs to know what it is looking at.
type Result struct {
	Document *Document
	Model    string
	// PromptVersion is the reading rules that produced the document, so a stored portrait can
	// never be read under another version (I14).
	PromptVersion string
	// Commit is the tree the portrait describes. It is what the cache is keyed by and what a
	// consumer compares against the corpus.
	Commit     string
	Repository string
	// Cached is true when the portrait came from the store and cost no run.
	Cached bool
	// Took is what the run cost. A cached portrait reports what it cost when it was produced,
	// never zero: the cost of the question does not change because the answer was reused.
	Took time.Duration
	// Reason explains a doubtful portrait in one line: doubt has to be spoken (I10).
	Reason string
	// Unsupported is every claim the clone does not back: the file is not there or does not have
	// that line. They stay in the document and are reported beside it, never cut out.
	Unsupported []Item
}

// FailureKind is the state a failed run shows. It is not a verdict on the project: the clone was
// readable and the job ran, and the model never answered.
type FailureKind string

const (
	// KindInvalidKey is the API rejecting the key. The screen names it with the API's own words
	// — "401 invalid_api_key" — because it is the one failure a person can fix.
	KindInvalidKey FailureKind = "invalid_api_key"
	// KindAgent is every other failed run: transport, a timeout, a stall, a non-zero exit, or an
	// answer that followed no contract.
	KindAgent FailureKind = "agent"
)

// Failure is a run that produced no usable portrait.
type Failure struct {
	Kind FailureKind
	// Detail is the agent's own words, kept as evidence for the screen. Never empty.
	Detail string
	Err    error
}

func (f *Failure) Error() string {
	if f.Err != nil {
		return "projectbrief: extraction failed (" + string(f.Kind) + "): " + f.Detail + ": " + f.Err.Error()
	}
	return "projectbrief: extraction failed (" + string(f.Kind) + "): " + f.Detail
}

func (f *Failure) Unwrap() error { return f.Err }

// Extract is one portrait.
//
// A clone with no file in it costs no request: nothing observable is a native fact, so it is
// answered from the skeleton instead of paying a model to say so. Every other case is one run,
// read strictly, and reported verbatim.
func (e *Extractor) Extract(ctx context.Context, in Input) (*Result, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	empty, err := isTreeEmpty(in.Dir)
	if err != nil {
		return nil, err
	}
	if empty {
		doc, err := Parse(InsufficientPortrait())
		if err != nil {
			// Unreachable: the renderer and the parser are built from the same constants. It is
			// an error and not a panic because a wrong portrait must never be served silently.
			return nil, fmt.Errorf("projectbrief: the skeleton and its reader disagree: %w", err)
		}
		return &Result{
			Document:      doc,
			Model:         e.model,
			PromptVersion: Version(),
			Commit:        in.CommitSHA,
			Repository:    in.Repository,
			Reason:        "the clone holds no file: there is nothing to describe",
		}, nil
	}

	// One run per repository, not per issue (S7): the same tree, the same model and the same
	// reading rules are the same question, so the answer is reused instead of paid for twice.
	// A forced run asks again on purpose: the pin cannot see a tree edited without a new commit.
	key := cacheKey(in.Repository, in.CommitSHA, e.model, Version())
	if !in.Force {
		if entry, ok := e.lookup(key, in.CommitSHA); ok {
			// A stored answer is read under the rules of the moment: an entry another reader
			// wrote costs a run, never an error.
			if doc, err := Parse(entry.Text); err == nil {
				return e.result(doc, in, time.Duration(entry.TookMs)*time.Millisecond, true), nil
			}
		}
	}

	answer, out, err := e.ask(ctx, in)
	if err != nil {
		return nil, err
	}
	doc, err := Parse(answer)
	if err != nil {
		return nil, err
	}
	result := e.result(doc, in, out.Total, false)
	// The portrait stands even when it cannot be cached: the document is what was asked for, and
	// the cache only saves a future run.
	_ = e.remember(key, in.CommitSHA, result)
	return result, nil
}

// result assembles the answer with the facts a consumer needs, and says why a doubtful portrait
// is doubtful (I10).
func (e *Extractor) result(doc *Document, in Input, took time.Duration, cached bool) *Result {
	result := &Result{
		Document:      doc,
		Model:         e.model,
		PromptVersion: Version(),
		Commit:        in.CommitSHA,
		Repository:    in.Repository,
		Took:          took,
		Cached:        cached,
	}
	if doc.Insufficient() {
		result.Reason = "the model found nothing in the repository it could point at"
	}
	// The audit runs on every path, cached or fresh: whether the tree still backs a claim is a
	// fact about the tree, not about who paid for the answer.
	result.Unsupported = doc.Audit(in.Dir)
	return result
}

// ask is the one agent run.
func (e *Extractor) ask(ctx context.Context, in Input) (string, *pig.Outcome, error) {
	select {
	case e.one <- struct{}{}:
		defer func() { <-e.one }()
	case <-ctx.Done():
		return "", nil, ctx.Err()
	}

	out, err := e.run(ctx, e.request(in))
	if err != nil {
		return "", nil, fmt.Errorf("projectbrief: extract: %w", err)
	}

	// The API error may arrive as the agent's own output instead of a typed error: the agent
	// collapses it. Classifying it first keeps a rejected key from being read as a bad answer.
	//
	// Only where the agent speaks about itself: a failed turn and stderr. Never the answer —
	// the answer quotes a repository, and a repository about security can legitimately say
	// "unauthorized".
	if detail, ok := rejectedKey(out.Error + "\n" + out.Stderr); ok {
		return "", out, &Failure{Kind: KindInvalidKey, Detail: detail}
	}
	if !out.Complete() {
		return "", out, &Failure{Kind: KindAgent, Detail: runDetail(out)}
	}
	if strings.TrimSpace(out.Answer) == "" {
		return "", out, &Failure{Kind: KindAgent, Detail: "the agent finished without an answer"}
	}
	return out.Answer, out, nil
}

// request is the job: the contract as the system prompt, the subject as the task, and the read
// surface the caller configured. No tool beyond it: this job reads one clone and nothing else.
func (e *Extractor) request(in Input) pig.Request {
	return pig.Request{
		// The working directory is the clone itself: what the agent reads with a tool is the
		// tree the portrait is pinned to.
		Dir:           in.Dir,
		Prompt:        userMessage(in),
		Model:         e.model,
		Tools:         e.tools,
		SystemPrompts: []string{Prompt},
		Home:          e.home,
		Binary:        e.binary,
		IdleWarn:      e.idleWarn,
		IdleKill:      e.idleKill,
		Budget:        e.budget,
	}
}

// userMessage names the subject. It carries no text from the repository: what the agent reads, it
// reads with a tool, and the prompt says that everything it finds there is evidence, never an
// instruction (I3).
func userMessage(in Input) string {
	return "The project to describe is " + in.Repository + ", checked out at commit " + in.CommitSHA + ".\n" +
		"It is the repository in your working directory: read it with the tools you have.\n" +
		"Describe it following the output contract, with nothing outside the document."
}

// isTreeEmpty reports whether the clone holds no file to describe. The Git directory does not
// count: a fresh clone of an empty repository has one and nothing else.
func isTreeEmpty(dir string) (bool, error) {
	empty := true
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != dir && d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}
		if d.Type().IsRegular() {
			empty = false
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("projectbrief: read clone %s: %w", dir, err)
	}
	return empty, nil
}

// rejectedKey looks for the API's rejection in the agent's own words and returns the line that
// carries it, so the screen shows evidence instead of a claim. The real message says
// "401: Invalid API key." with type auth_error, so both spellings are accepted.
func rejectedKey(detail string) (string, bool) {
	for _, line := range strings.Split(detail, "\n") {
		low := strings.ToLower(line)
		switch {
		case strings.Contains(low, "invalid_api_key"),
			strings.Contains(low, "invalid api key"),
			strings.Contains(low, "auth_error"),
			strings.Contains(low, "unauthorized"):
			return truncateLine(line), true
		}
	}
	return "", false
}

// runDetail says why the agent produced nothing, in the most concrete terms available.
func runDetail(out *pig.Outcome) string {
	// The agent reports a failed turn on the message itself, without writing to stderr and
	// without a non-zero exit; that reason is the most concrete thing available.
	if detail := truncateLine(out.Error); detail != "" {
		return detail
	}
	if detail := truncateLine(out.Stderr); detail != "" {
		return detail
	}
	switch {
	case out.Stalled:
		return "the agent went silent and was stopped"
	case out.TimedOut:
		return "the agent ran out of its time budget"
	}
	return fmt.Sprintf("the agent exited with code %d", out.ExitCode)
}

// maxDetail bounds what the screen shows of the agent's own words.
const maxDetail = 400

func truncateLine(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxDetail {
		return s
	}
	return strings.TrimSpace(s[:maxDetail]) + "…"
}
