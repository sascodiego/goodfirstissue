package explore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/pig"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// Input is everything that makes one analysis the question it answers. Anything that can
// change the answer travels here, because the key is built from it (I4).
type Input struct {
	// Repository is the store key of the corpus, e.g. spf13__cobra.
	Repository string
	// NameWithOwner is how the issue names the project, verbatim from GitHub.
	NameWithOwner string
	// Dir is the clone the agent reads. It must be at CommitSHA.
	Dir string
	// CommitSHA is the pin the corpus was frozen at.
	CommitSHA string
	Issue     ghdata.Issue
	// Force ignores a stored answer and runs the agent again.
	Force bool
}

// Result is what one analysis produced and how it was obtained.
type Result struct {
	Record  *Record
	Outcome *pig.Outcome
	// Reused is true when the journal already held this exact question.
	Reused bool
	// Previous is the journal line that answered it, when Reused. It carries when the
	// analysis was produced, which a fresh run cannot say.
	Previous *store.Decision
	// Compacted reports that the journal was rewritten during this call.
	Compacted bool
	// StaleCache reports that a stored answer existed but no longer decodes, so the run
	// below replaced it. A caller may say so instead of pretending nothing was there.
	StaleCache bool
}

// Session runs one analysis with every input that can change the answer fixed. The CLI and
// the web server share it so a result produced by one is reusable by the other: the cache
// key, the journal write and the validation against the trace live here once.
type Session struct {
	Store *store.Store
	// Model is the agent's model, from the allowlist.
	Model string
	// Models is the allowlist the agent's own configuration is generated from.
	Models []string
	Tools  []string
	// Home is this application's PIG_HOME. It is generated, never inherited.
	Home   string
	Binary string

	IdleWarn time.Duration
	IdleKill time.Duration
	Budget   time.Duration

	// Run replaces pig.Run. Tests inject a double here, so no test needs the binary, the
	// network or a clone.
	Run func(ctx context.Context, req pig.Request) (*pig.Outcome, error)

	// OnIdle is called once when the agent goes silent past IdleWarn.
	OnIdle func(silence time.Duration)
	// OnSilence is called on every tick with how long the pipe has been silent, so a
	// screen can show the seconds without polling the process.
	OnSilence func(silence time.Duration)
	// OnRun is called when a fresh run starts. A reused answer never calls it, so a caller
	// can report what is actually about to happen.
	OnRun func()

	// Logger receives one line per stored or reused analysis. Nil means silent: a library
	// does not pick the process's logger, and the CLI already prints what it needs.
	Logger *slog.Logger
}

// Key is the question this analysis answers. A changed prompt, model or commit is another
// question and must miss instead of reusing a stale verdict (I14).
func (s *Session) Key(in Input) store.Key {
	return store.Key{
		Issue:         in.Issue.Number,
		Pass:          store.PassExplore,
		Model:         s.Model,
		PromptVersion: Version(),
		CommitSHA:     in.CommitSHA,
	}
}

// Analyze answers one issue, reusing an identical previous answer instead of paying for it
// twice.
//
// When the caller cancelled the run, nothing is stored and the context error is returned: a
// stopped job answers no question, and a journal line saying otherwise would be a result
// nobody produced.
//
// When the agent answered but the answer does not parse, the record is still stored — the
// run happened and its metrics and trace are evidence (I10) — and both the Result and the
// parse error are returned.
func (s *Session) Analyze(ctx context.Context, in Input) (*Result, error) {
	switch {
	case s.Store == nil:
		return nil, errors.New("explore: session has no store")
	case in.Repository == "":
		return nil, errors.New("explore: input has no repository key")
	case in.CommitSHA == "":
		return nil, errors.New("explore: input has no commit to pin the result to")
	case in.Issue.Number <= 0:
		return nil, errors.New("explore: input has no issue")
	}

	decisions, err := s.Store.Decisions(in.Repository)
	if err != nil {
		return nil, err
	}

	key := s.Key(in)
	stale := false
	if cached, ok := decisions.Get(key); ok && !in.Force {
		var previous Record
		if err := json.Unmarshal(cached.Verdict, &previous); err == nil && previous.Evidence != nil {
			s.logger().Info("explore: análisis reutilizado",
				"repo", in.Repository, "issue", in.Issue.Number,
				"prompt", key.PromptVersion, "model", key.Model,
				"commit", shortSHA(key.CommitSHA),
				"produced_at", cached.At.UTC().Format(time.RFC3339))
			return &Result{Record: &previous, Reused: true, Previous: &cached}, nil
		}
		// A verdict that no longer decodes is not reused: the run below replaces it.
		stale = true
	}

	// The application owns the agent's configuration: generated on every run, so an old or
	// foreign file cannot contaminate it.
	if err := pig.WriteConfig(s.Home, pig.DefaultConfig(s.Models)); err != nil {
		return nil, err
	}
	skillPath, err := Prepare(s.Home)
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		// Una corrida cancelada no se guarda: se dice, para que el silencio no pase por
		// un resultado que nunca existió.
		s.logger().Warn("explore: corrida cancelada, no se guardó nada",
			"repo", in.Repository, "issue", in.Issue.Number, "reason", err.Error())
		return nil, err
	}

	run := s.Run
	if run == nil {
		run = pig.Run
	}
	if s.OnRun != nil {
		s.OnRun()
	}
	out, err := run(ctx, pig.Request{
		Dir:           in.Dir,
		Prompt:        PromptFor(in.NameWithOwner, in.CommitSHA, in.Issue.Number, in.Issue.Title, in.Issue.Body, in.Issue.Labels),
		Model:         s.Model,
		Tools:         s.Tools,
		Home:          s.Home,
		Binary:        s.Binary,
		IdleWarn:      s.IdleWarn,
		IdleKill:      s.IdleKill,
		Budget:        s.Budget,
		SystemPrompts: []string{Prompt},
		Skills:        []string{skillPath},
		OnIdle:        s.OnIdle,
		OnProgress:    s.OnSilence,
	})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r := Record{
		Repository: in.Repository,
		CommitSHA:  in.CommitSHA,
		Issue:      in.Issue.Number,
		Model:      s.Model,
		Metrics: Metrics{
			TotalMs:    out.Total.Milliseconds(),
			FirstMs:    out.FirstEvent.Milliseconds(),
			LongestGap: out.LongestGap.Milliseconds(),
			ToolCalls:  len(out.ToolCalls),
			Stalled:    out.Stalled,
			TimedOut:   out.TimedOut,
			Complete:   out.Complete(),
		},
	}
	evidence, parseErr := Parse(out.Answer)
	if parseErr == nil {
		r.Evidence = evidence
		r.Violations = evidence.Verify(Traces(out.ToolCalls))
	}

	verdict, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	if err := decisions.Put(store.Decision{Key: key, LatencyMs: r.Metrics.TotalMs, Verdict: verdict}); err != nil {
		return nil, err
	}
	s.logger().Info("explore: análisis guardado",
		"repo", in.Repository, "issue", in.Issue.Number,
		"prompt", key.PromptVersion, "model", key.Model,
		"commit", shortSHA(key.CommitSHA),
		"ms", r.Metrics.TotalMs, "tool_calls", r.Metrics.ToolCalls,
		"complete", r.Metrics.Complete, "stalled", r.Metrics.Stalled,
		"timed_out", r.Metrics.TimedOut, "violations", len(r.Violations))

	compacted, err := decisions.Compact()
	if err != nil {
		return nil, fmt.Errorf("explore: compact the journal: %w", err)
	}
	if compacted {
		s.logger().Info("explore: journal compactado", "repo", in.Repository)
	}

	res := &Result{Record: &r, Outcome: out, Compacted: compacted, StaleCache: stale}
	if parseErr != nil {
		return res, parseErr
	}
	return res, nil
}

// logger returns the session's logger, or a silent one: a library never picks the
// process's, so a caller that does not ask for lines gets none.
func (s *Session) logger() *slog.Logger {
	if s.Logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return s.Logger
}

// ErrNoIndex means the repository has no code graph, so the agent would explore nothing.
var ErrNoIndex = errors.New("explore: the repository has no code graph")

// StaleIndexError reports a graph built from another commit than the corpus pins.
type StaleIndexError struct {
	Corpus string
	Index  string
}

func (e *StaleIndexError) Error() string {
	return fmt.Sprintf("explore: the graph is at %s but the corpus pins %s", shortSHA(e.Index), shortSHA(e.Corpus))
}

// IndexFor returns the code graph that belongs to this corpus.
//
// A graph from another commit is refused rather than warned about: the record pins the
// commit and the analysis is evidence about that tree, so exploring another one would label
// the result with code nobody read (I4).
func IndexFor(st *store.Store, in Input) (store.IndexRecord, error) {
	if st == nil {
		return store.IndexRecord{}, errors.New("explore: no store")
	}
	rec, err := st.LoadIndex(in.Repository)
	if errors.Is(err, os.ErrNotExist) {
		return store.IndexRecord{}, ErrNoIndex
	}
	if err != nil {
		if !st.Exists(in.Repository + "/index.json") {
			return store.IndexRecord{}, ErrNoIndex
		}
		return store.IndexRecord{}, err
	}
	if rec.CommitSHA != in.CommitSHA {
		return rec, &StaleIndexError{Corpus: in.CommitSHA, Index: rec.CommitSHA}
	}
	return rec, nil
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
