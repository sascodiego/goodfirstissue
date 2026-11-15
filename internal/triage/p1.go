package triage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// Judge is the transport the first pass talks to. It is an interface because the benchmark
// and the tests must run the same chain without a network: a second copy of the pipeline for
// measuring it would be a copy that could disagree with the product (S2).
type Judge interface {
	Ask(ctx context.Context, state string, questions map[string]nan.Question) (map[string]nan.Answer, error)
}

// Options configures one first pass.
type Options struct {
	// Judge answers the questions. Required.
	Judge Judge
	// Journal reuses finished analyses. Nil disables caching, which is what the benchmark
	// and a test that counts requests want.
	Journal *store.Decisions
	// Model is the decision model, read by the caller from the constitution (I13). Required.
	Model string
	// CommitSHA is the commit the verdict is pinned to (I4). Required when caching, because a
	// key without it would reuse a verdict produced from another tree.
	CommitSHA string
	// Now is the clock, injected so a test can assert the stored time without waiting.
	Now func() time.Time
}

// P1 is the first pass: one request per issue, one verdict, stored when there is a journal.
type P1 struct {
	judge     Judge
	journal   *store.Decisions
	model     string
	commitSHA string
	now       func() time.Time
}

// New validates the settings. It fails closed on a missing model or a journal without a pin:
// both would produce a verdict nobody can attribute to a question.
func New(opts Options) (*P1, error) {
	switch {
	case opts.Judge == nil:
		return nil, errors.New("triage: a judge is required")
	case strings.TrimSpace(opts.Model) == "":
		return nil, errors.New("triage: a model is required: this package never picks one (I13)")
	case opts.Journal != nil && strings.TrimSpace(opts.CommitSHA) == "":
		return nil, errors.New("triage: a cached verdict needs the commit it is pinned to (I4)")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &P1{
		judge:     opts.Judge,
		journal:   opts.Journal,
		model:     strings.TrimSpace(opts.Model),
		commitSHA: strings.TrimSpace(opts.CommitSHA),
		now:       now,
	}, nil
}

// Analysis is one issue's first pass with the evidence it rests on. Answers travel so a caller
// can show what the judge said, and so the benchmark scores the same values the product uses.
type Analysis struct {
	Result
	// Answers is what the judge answered, keyed by question id. Empty when a pre-gate resolved
	// the issue without a request.
	Answers map[string]nan.Answer
	// Model, PromptVersion and CommitSHA name the question this verdict answers (I4, I14).
	Model         string
	PromptVersion string
	CommitSHA     string
	// Cached reports that the verdict came from the journal and cost no request.
	Cached bool
	// At is when the verdict was first produced. A cached verdict keeps its original time,
	// because the interface never presents a stored answer as fresh (I14).
	At time.Time
}

// key is the journal identity of this exact question.
func (p *P1) key(issue Issue) store.Key {
	return store.Key{
		Issue:         issue.Number,
		Pass:          store.PassTriage,
		Model:         p.model,
		PromptVersion: Version(),
		CommitSHA:     p.commitSHA,
	}
}

// stored is what the journal keeps: the verdict and the answers behind it, so a reuse shows
// the same evidence a fresh run would.
type stored struct {
	Result  Result                `json:"result"`
	Answers map[string]nan.Answer `json:"answers"`
}

// Run triages one issue.
//
// A failure of the request is not a verdict about the issue: the result is already the one the
// product shows (undecided, I2), and the returned error is the cause, for the log.
func (p *P1) Run(ctx context.Context, issue Issue) (Analysis, error) {
	analysis := Analysis{Model: p.model, PromptVersion: Version(), CommitSHA: p.commitSHA}

	// A deterministic fact resolves the issue without a dossier and without a request.
	if pre := Prescreen(issue); pre.Skip {
		analysis.Result = Result{Outcome: Marked, Reason: pre.Reason, Probability: 1}
		analysis.At = p.now().UTC()
		return analysis, nil
	}

	if cached, ok := p.lookup(issue); ok {
		cached.Model, cached.PromptVersion, cached.CommitSHA = p.model, Version(), p.commitSHA
		return cached, nil
	}

	started := p.now()
	answers, err := p.judge.Ask(ctx, SubstanceDossier(issue), Questions())
	latency := p.now().Sub(started)
	if err != nil {
		analysis.Result = Result{Outcome: Undecided, Reason: ReasonEndpointFailed}
		return analysis, fmt.Errorf("triage: asking about %s#%d: %w", issue.Repository, issue.Number, err)
	}

	analysis.Result = Decide(issue, answers)
	analysis.Answers = answers
	analysis.At = started.UTC()

	// The verdict stands even when it cannot be stored: the analysis is what was asked for,
	// and the journal only saves a future request.
	_ = p.remember(issue, analysis, latency)

	return analysis, nil
}

// lookup returns a stored verdict for this exact question, if there is one. A damaged entry is
// a miss, not a failed analysis: the pipeline asks again instead of reporting a broken journal.
func (p *P1) lookup(issue Issue) (Analysis, bool) {
	if p.journal == nil {
		return Analysis{}, false
	}
	decision, ok := p.journal.Get(p.key(issue))
	if !ok {
		return Analysis{}, false
	}
	var entry stored
	if err := json.Unmarshal(decision.Verdict, &entry); err != nil {
		return Analysis{}, false
	}
	if entry.Result.Outcome == "" {
		// A stored envelope without a verdict is damage, not an answer: showing it would mean
		// presenting an empty outcome as a decision.
		return Analysis{}, false
	}
	return Analysis{
		Result:  entry.Result,
		Answers: entry.Answers,
		Cached:  true,
		At:      decision.At,
	}, true
}

func (p *P1) remember(issue Issue, analysis Analysis, latency time.Duration) error {
	if p.journal == nil {
		return nil
	}
	verdict, err := json.Marshal(stored{Result: analysis.Result, Answers: analysis.Answers})
	if err != nil {
		return fmt.Errorf("triage: encode verdict: %w", err)
	}
	return p.journal.Put(store.Decision{
		Key:       p.key(issue),
		At:        analysis.At,
		LatencyMs: latency.Milliseconds(),
		Verdict:   verdict,
	})
}

// ClientJudge adapts the decision client to Judge. It is the only adapter the product uses;
// a test or the benchmark passes its own (S8).
type ClientJudge struct {
	Client *nan.Client
	Model  string
}

// Ask sends the questions and returns the answers by question id.
func (c ClientJudge) Ask(ctx context.Context, state string, questions map[string]nan.Question) (map[string]nan.Answer, error) {
	if c.Client == nil {
		return nil, errors.New("triage: the client judge needs a client")
	}
	resp, err := c.Client.SystemOne(ctx, nan.SystemOneRequest{
		Model:     c.Model,
		State:     state,
		Questions: questions,
	})
	if err != nil {
		return nil, err
	}
	return resp.Answers, nil
}
