package pig

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

// Defaults for one exploration run. The kill threshold is generous on purpose: a single
// model turn can legitimately take ten seconds.
const (
	DefaultIdleWarn = 20 * time.Second
	DefaultIdleKill = 90 * time.Second
	DefaultBudget   = 180 * time.Second
)

// Request describes one read-only exploration run.
type Request struct {
	// Dir is the repository clone and becomes the child's working directory.
	Dir string
	// Prompt is the task. It is passed after "--" so a leading dash cannot be read as a flag.
	Prompt string
	// Model is a model ID from the allowlist; the "nan/" provider prefix is added here.
	Model string
	// Tools is the complete allowlist, including any MCP tool as mcp__server__tool.
	Tools []string
	// SystemPrompts are appended to PiG's own prompt, in order. They always apply.
	SystemPrompts []string
	// Skills are absolute paths registered as re-readable skills.
	Skills []string
	// Home is this application's PIG_HOME. It is never inherited from the environment.
	Home string

	IdleWarn time.Duration
	IdleKill time.Duration
	Budget   time.Duration

	// OnIdle is called once when the child has been silent past IdleWarn.
	OnIdle func(silence time.Duration)

	// Binary overrides the executable. Empty means "pig" from PATH.
	Binary string
}

// Outcome is what one run produced, including how it behaved in time.
type Outcome struct {
	Answer    string
	ToolCalls []ToolCall
	// Error is the agent's own reason when a turn ended in failure, empty otherwise.
	Error string

	FirstEvent time.Duration
	LongestGap time.Duration
	Total      time.Duration

	// Stalled is true when the child went silent past IdleKill and was killed.
	Stalled bool
	// TimedOut is true when the whole-run budget expired before the child finished.
	TimedOut bool
	ExitCode int
	Stderr   string
}

// Complete reports whether the agent finished on its own within its limits.
func (o *Outcome) Complete() bool {
	return !o.Stalled && !o.TimedOut && o.ExitCode == 0 && o.Error == ""
}

// Error reports a failure to start or an unexpected process error. A stalled, timed-out
// or non-zero agent is a valid Outcome: the caller decides what to do with incomplete
// evidence, and never discards it.
type Error struct {
	Op  string
	Err error
}

func (e *Error) Error() string { return "pig: " + e.Op + ": " + e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// Run executes one exploration and parses its event stream.
//
// It never mutates the repository: the caller supplies a tool allowlist with no write
// surface, and the working directory is the clone.
func Run(ctx context.Context, req Request) (*Outcome, error) {
	applyDefaults(&req)
	switch {
	case req.Dir == "":
		return nil, &Error{Op: "run", Err: errors.New("dir is required")}
	case req.Model == "":
		return nil, &Error{Op: "run", Err: errors.New("model is required")}
	case req.Home == "":
		return nil, &Error{Op: "run", Err: errors.New("home is required")}
	}

	runCtx, cancel := context.WithTimeout(ctx, req.Budget)
	defer cancel()

	bin := req.Binary
	if bin == "" {
		bin = "pig"
	}

	cmd := exec.CommandContext(runCtx, bin, buildArgs(req)...)
	cmd.Dir = req.Dir
	cmd.Env = envWith(os.Environ(), "PIG_HOME="+req.Home)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, &Error{Op: "stdout pipe", Err: err}
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, &Error{Op: "start", Err: err}
	}

	var (
		res      Result
		lastSeen atomic.Int64
		first    time.Duration
		longest  time.Duration
	)
	lastSeen.Store(start.UnixNano())

	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		sc := bufio.NewScanner(stdout)
		// A single streamed message update can be large; do not let the scanner bail.
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for sc.Scan() {
			now := time.Now()
			lastSeen.Store(now.UnixNano())
			if first == 0 {
				first = now.Sub(start)
			}
			res.consume(sc.Bytes())
		}
	}()

	mon := newStallMonitor(req.IdleWarn, req.IdleKill)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var stalled, timedOut bool
loop:
	for {
		select {
		case <-runCtx.Done():
			// The kill branch below breaks out immediately, so reaching here means the
			// whole-run budget expired rather than the stall monitor firing.
			timedOut = true
			break loop
		case <-scanDone:
			break loop
		case <-ticker.C:
			silence := time.Since(time.Unix(0, lastSeen.Load()))
			if silence > longest {
				longest = silence
			}
			warn, kill := mon.observe(silence)
			if warn && req.OnIdle != nil {
				req.OnIdle(silence)
			}
			if kill {
				stalled = true
				cancel()
				break loop
			}
		}
	}

	<-scanDone
	waitErr := cmd.Wait()

	out := &Outcome{
		Answer:     res.Answer,
		Error:      res.Error,
		ToolCalls:  res.ToolCalls,
		FirstEvent: first,
		LongestGap: longest,
		Total:      time.Since(start),
		Stalled:    stalled,
		TimedOut:   timedOut,
		Stderr:     strings.TrimSpace(stderr.String()),
	}

	var exit *exec.ExitError
	switch {
	case waitErr == nil:
		out.ExitCode = 0
	case errors.As(waitErr, &exit):
		// A killed child exits with a signal; that is expected when we stopped it.
		out.ExitCode = exit.ExitCode()
	default:
		return nil, &Error{Op: "wait", Err: waitErr}
	}
	return out, nil
}

// buildArgs assembles the command line. The order matters: flags first, then the task
// after "--", so an issue whose body starts with a dash is not parsed as an option.
func buildArgs(req Request) []string {
	args := []string{
		"-p",
		"--mode", "json",
		"--provider", "nan",
		"--model", "nan/" + req.Model,
	}
	if len(req.Tools) == 0 {
		// No tools is a job that only moves text: nothing to read, nothing to run, no
		// repository to graph. Without its own branch an empty allowlist would become
		// "--tools " and still load the MCP extension, which starts a code-graph server
		// for a task that never looks at code.
		args = append(args, "-nt", "--no-extensions", "--no-mcp")
	} else {
		args = append(args,
			"--tools", strings.Join(req.Tools, ","),
			// --no-extensions also disables the builtin MCP extension, so re-enable exactly
			// that one. Without it the agent never sees the code graph.
			"--no-extensions", "-e", "builtin:mcp",
		)
	}
	args = append(args,
		"--no-session",
		"--no-skills",
		"-nc",
		"-np",
		"--no-approve",
	)
	for _, p := range req.SystemPrompts {
		args = append(args, "--append-system-prompt", p)
	}
	for _, s := range req.Skills {
		args = append(args, "--skill", s)
	}
	return append(args, "--", req.Prompt)
}

// envWith returns base with each key replaced. A duplicated key would be ambiguous, so
// inherited values are removed rather than shadowed: a foreign PIG_HOME must not win.
func envWith(base []string, kv ...string) []string {
	replace := make(map[string]bool, len(kv))
	for _, e := range kv {
		if k, _, ok := strings.Cut(e, "="); ok {
			replace[k] = true
		}
	}
	out := make([]string, 0, len(base)+len(kv))
	for _, e := range base {
		if k, _, ok := strings.Cut(e, "="); ok && replace[k] {
			continue
		}
		out = append(out, e)
	}
	return append(out, kv...)
}

func applyDefaults(req *Request) {
	if req.IdleWarn == 0 {
		req.IdleWarn = DefaultIdleWarn
	}
	if req.IdleKill == 0 {
		req.IdleKill = DefaultIdleKill
	}
	if req.Budget == 0 {
		req.Budget = DefaultBudget
	}
}

// DefaultTools is the read-only surface: no bash, no edit, no write.
func DefaultTools() []string {
	return []string{"read", "ls", "grep", "mcp__codegraph__codegraph_explore"}
}
