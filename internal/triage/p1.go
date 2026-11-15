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

// Judge is the transport the first pass talks to. It is an interface because the benchmark and
// the tests must run the same chain without a network: a second copy of the pipeline for
// measuring it would be a copy that could disagree with the product (S2).
type Judge interface {
	Ask(ctx context.Context, state string, questions map[string]nan.Question) (map[string]nan.Answer, error)
}

// SecondRound is the deferred pass that decides whether a question becomes work (LIMITES.md §3).
//
// It is a seam, not a feature: installed, it receives the first round's reading and may settle
// the issue. Nil means the round is not installed, and a question stays DERIVADA, which is a
// valid outcome rather than a pending state. Adding the round later is one type with Resolve and
// one field here: no question, dossier, journal, benchmark or policy changes.
type SecondRound interface {
	// Resolve returns the verdict the second round reached. ok false means it could not settle
	// the issue, and the first round's DERIVADA stands.
	Resolve(ctx context.Context, issue Issue, first Analysis) (result Result, ok bool, err error)
}

// Options configures one first pass.
type Options struct {
	// Judge answers the questions. Required.
	Judge Judge
	// Journal reuses finished analyses. Nil disables caching, which is what the benchmark and a
	// test that counts requests want.
	Journal *store.Decisions
	// Model is the decision model, read by the caller from the constitution (I13). Required.
	Model string
	// CommitSHA is the commit the verdict is pinned to (I4). Required when caching, because a key
	// without it would reuse a verdict produced from another tree.
	CommitSHA string
	// Brief supplies the project's direction for the alignment node (§7.5). Nil means the node
	// cannot be asked, and a feature is then CONDICIONADA saying exactly that.
	Brief BriefSource
	// SecondRound is the deferred round for questions. Nil leaves them DERIVADA.
	SecondRound SecondRound
	// Now is the clock, injected so a test can assert the stored time without waiting.
	Now func() time.Time
}

// P1 is the first pass: one request per issue, a second one only for a feature, one verdict,
// stored when there is a journal.
type P1 struct {
	judge       Judge
	journal     *store.Decisions
	brief       BriefSource
	secondRound SecondRound
	model       string
	commitSHA   string
	now         func() time.Time
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
		judge:       opts.Judge,
		journal:     opts.Journal,
		brief:       opts.Brief,
		secondRound: opts.SecondRound,
		model:       strings.TrimSpace(opts.Model),
		commitSHA:   strings.TrimSpace(opts.CommitSHA),
		now:         now,
	}, nil
}

// Analysis is one issue's first pass with the evidence it rests on. Answers travel so a caller can
// show what the judge said, and so the benchmark scores the same values the product uses.
type Analysis struct {
	Result
	// Answers is what the judge answered about the substance dossier, keyed by question id. Empty
	// when a pre-gate resolved the issue without a request.
	Answers map[string]nan.Answer
	// Alignment is what the node answered, when it was asked. Nil means it was not.
	Alignment map[string]nan.Answer
	// Model, PromptVersion and CommitSHA name the question this verdict answers (I4, I14).
	Model         string
	PromptVersion string
	CommitSHA     string
	// Cached reports that the verdict came from the journal and cost no request.
	Cached bool
	// At is when the verdict was first produced. A cached verdict keeps its original time, because
	// the interface never presents a stored answer as fresh (I14).
	At time.Time
}

// key is the journal identity of this exact question. The assembly lives in keyFor, beside the
// reader, so the pass and the screens cannot answer different questions from the same file.
func (p *P1) key(issue Issue) store.Key {
	return keyFor(issue.Number, p.model, p.commitSHA)
}

// stored is what the journal keeps: the verdict and the evidence behind it, so a reuse shows what
// a fresh run would have shown.
type stored struct {
	Result    Result                `json:"result"`
	Answers   map[string]nan.Answer `json:"answers,omitempty"`
	Alignment map[string]nan.Answer `json:"alignment,omitempty"`
}

// Run triages one issue.
//
// A failure of the first request is not a verdict about the issue: the result is already the one
// the product shows (undecided, I2), and the returned error is the cause, for the log.
//
// A failure of the alignment node is smaller and does not erase the verdict: the issue still
// appears, as a feature whose project position could not be read. The error is returned all the
// same, because a degraded reading has to be visible to whoever is paying attention.
func (p *P1) Run(ctx context.Context, issue Issue) (Analysis, error) {
	analysis := Analysis{Model: p.model, PromptVersion: Version(), CommitSHA: p.commitSHA}

	// A deterministic fact resolves the issue without a dossier and without a request. It is a
	// verdict all the same, so it is journaled with zero latency and no answers: a report that
	// reads the journal would otherwise count the issues nobody has to look at as untriaged.
	if pre := Prescreen(issue); pre.Skip {
		analysis.Result = Result{Outcome: Marked, Reason: pre.Reason, Probability: 1}
		analysis.At = p.now().UTC()

		// The journal is pinned to the commit and the prompt, not to the issue's own facts: a
		// refreshed corpus can bring a linked pull request at the same pin. The fact wins over
		// the stored verdict (§7.7, row 0 before row 1); an unchanged one keeps its own time,
		// so the next run says it was reused (I14) instead of presenting it as fresh.
		if reused, ok := p.lookup(issue); ok && reused.Outcome == Marked && reused.Reason == pre.Reason {
			reused.Model, reused.PromptVersion, reused.CommitSHA = p.model, Version(), p.commitSHA
			return reused, nil
		}

		// The verdict stands even when it cannot be stored, exactly as below.
		_ = p.remember(issue, analysis, 0)
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

	alignment, degradation := p.align(ctx, issue, answers)
	analysis.Answers = answers
	analysis.Alignment = alignment
	analysis.Result = Decide(issue, answers, alignment)

	if analysis.Outcome == Derived && p.secondRound != nil {
		settled, ok, err := p.secondRound.Resolve(ctx, issue, analysis)
		switch {
		case err != nil:
			// The round could not finish, and a valid outcome is not lost because of it.
			degradation = errors.Join(degradation, fmt.Errorf("triage: second round of %s#%d: %w", issue.Repository, issue.Number, err))
		case ok:
			analysis.Result = settled
		}
	}

	analysis.At = started.UTC()

	// The verdict stands even when it cannot be stored: the analysis is what was asked for, and the
	// journal only saves a future request.
	_ = p.remember(issue, analysis, latency)

	if degradation != nil {
		return analysis, degradation
	}
	return analysis, nil
}

// align asks the node that is the only branch a feature takes. It runs only for a feature: a bug is
// a deviation from what the project already wants, and a documentation request is not a feature at
// all, so neither has anything to ask the project's direction.
func (p *P1) align(ctx context.Context, issue Issue, answers map[string]nan.Answer) (map[string]nan.Answer, error) {
	if p.brief == nil {
		return nil, nil
	}
	if kind, ok := choiceOf(answers, QKind); !ok || kind != KindFeature {
		return nil, nil
	}

	brief, err := p.brief.Brief(ctx, issue.Repository)
	if err != nil {
		return nil, fmt.Errorf("triage: reading the direction of %s: %w", issue.Repository, err)
	}
	alignment, err := p.judge.Ask(ctx, AlignmentDossier(issue, brief), AlignmentQuestions())
	if err != nil {
		return nil, fmt.Errorf("triage: asking where %s is going: %w", issue.Repository, err)
	}
	return alignment, nil
}

// lookup returns a stored verdict for this exact question, if there is one. A damaged entry is a
// miss, not a failed analysis: the pipeline asks again instead of reporting a broken journal. The
// reading itself lives in Recall, which the screens call too.
func (p *P1) lookup(issue Issue) (Analysis, bool) {
	analysis, _, ok := Recall(p.journal, issue.Number, p.model, p.commitSHA)
	return analysis, ok
}

func (p *P1) remember(issue Issue, analysis Analysis, latency time.Duration) error {
	if p.journal == nil {
		return nil
	}
	verdict, err := json.Marshal(stored{Result: analysis.Result, Answers: analysis.Answers, Alignment: analysis.Alignment})
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

// ClientJudge adapts the decision client to Judge. It is the only adapter the product uses; a test
// or the benchmark passes its own (S8).
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
