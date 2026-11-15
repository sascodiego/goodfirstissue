package fit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// Judge is the transport the third pass talks to. It is an interface because a test and the product
// must run the same pass without a network: a second copy of the pipeline for measuring it would be
// a copy that could disagree with the product.
type Judge interface {
	Ask(ctx context.Context, state string, questions map[string]nan.Question) (map[string]nan.Answer, error)
}

// Exploration names the exploration whose evidence the state's requirements came from: the explore
// package's prompt version and the agent model that produced it.
//
// It is part of the question, not metadata about it. The state P3 sends carries that exploration's
// requirements, so the same issue explored under another prompt version or another agent is a state
// the judge has never read: reusing a verdict produced from one exploration for another would
// present an answer to one question as the answer to a different one (I14). The profile travels in
// the key for exactly the same reason (S6).
type Exploration struct {
	// PromptVersion is the exploration contract's fingerprint, explore.Version().
	PromptVersion string
	// Model is the agent model that produced the exploration.
	Model string
}

// Known reports whether the identity can be named at all. An empty half is not "any exploration":
// a key that fingerprinted an absence would reuse one verdict against every tree and every agent.
func (e Exploration) Known() bool {
	return strings.TrimSpace(e.PromptVersion) != "" && strings.TrimSpace(e.Model) != ""
}

// fingerprint names the exploration inside the journal key without storing what it was.
func (e Exploration) fingerprint() string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(e.PromptVersion) + "\x00" + strings.TrimSpace(e.Model)))
	return hex.EncodeToString(sum[:])[:12]
}

// Options configures one fit pass.
type Options struct {
	// Judge answers the question. Required.
	Judge Judge
	// Journal reuses finished verdicts. Nil disables caching, which is what a test that counts
	// requests wants.
	Journal *store.Decisions
	// Model is the decision model, read by the caller from the constitution (I13). Required.
	Model string
	// CommitSHA is the commit the verdict is pinned to (I4). Required when caching, because a key
	// without it would reuse a verdict produced from another tree.
	CommitSHA string
	// Profile is the saved profile's document, verbatim: the text the person approved. Empty means
	// no profile was ever saved, which is a level and not an error (S7).
	Profile string
	// Exploration names the exploration the state's evidence came from. It is required when a
	// journal is given, for the same reason the profile is: a stored verdict has to declare the
	// whole question it answers, and which exploration it was read against is part of that question
	// (I14).
	Exploration Exploration
	// Portrait is the project portrait. Empty means none was produced, and the state says so where
	// the technical stack would have been.
	Portrait string
	// Now is the clock, injected so a test can assert the stored time without waiting.
	Now func() time.Time
}

// P3 is the third pass: one request per issue, one level, stored when there is a journal.
type P3 struct {
	judge       Judge
	journal     *store.Decisions
	model       string
	commitSHA   string
	profile     string
	exploration Exploration
	portrait    string
	now         func() time.Time
}

// New validates the settings. It fails closed on a missing model, and on a journal without the pin,
// without the profile or without the exploration: every one of those would produce a stored verdict
// nobody can attribute to a question (I4, S6, I14).
func New(opts Options) (*P3, error) {
	switch {
	case opts.Judge == nil:
		return nil, errors.New("fit: a judge is required")
	case strings.TrimSpace(opts.Model) == "":
		return nil, errors.New("fit: a model is required: this package never picks one (I13)")
	case opts.Journal != nil && strings.TrimSpace(opts.CommitSHA) == "":
		return nil, errors.New("fit: a cached verdict needs the commit it is pinned to (I4)")
	case opts.Journal != nil && strings.TrimSpace(opts.Profile) == "":
		// Without a profile there is no comparison, so there is no question to store: a cached row
		// would present an absence of evidence as a decision about the issue (S7).
		return nil, errors.New("fit: a cached verdict needs the profile it was asked against (S6)")
	case opts.Journal != nil && !opts.Exploration.Known():
		// The state carries the exploration's requirements, so which exploration it was is part of
		// the question. A row stored without it would be reused against evidence the judge never
		// read, which is the one thing I14 exists to prevent.
		return nil, errors.New("fit: a cached verdict needs the exploration its evidence came from (I14)")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &P3{
		judge:       opts.Judge,
		journal:     opts.Journal,
		model:       strings.TrimSpace(opts.Model),
		commitSHA:   strings.TrimSpace(opts.CommitSHA),
		profile:     strings.TrimSpace(opts.Profile),
		exploration: opts.Exploration,
		portrait:    opts.Portrait,
		now:         now,
	}, nil
}

// Analysis is one issue's fit verdict with what it rests on. Answers travel so a caller can show
// the reading behind the level, and so the stored row shows what a fresh run would have shown.
type Analysis struct {
	Result
	// Answers is what the judge answered, keyed by question id. Empty when there was no request.
	Answers map[string]nan.Answer
	// Model, PromptVersion and CommitSHA name the question this verdict answers (I4, I14).
	// PromptVersion carries the exploration's fingerprint and the profile's as well as the
	// contract's, because the same issue against another CV, or read against another exploration,
	// is another question (S6, I14).
	Model         string
	PromptVersion string
	CommitSHA     string
	// Cached reports that the verdict came from the journal and cost no request.
	Cached bool
	// At is when the verdict was first produced. A reused verdict keeps its original time, because
	// the interface never presents a stored answer as fresh (I14).
	At time.Time
}

// key is the journal identity of this exact question.
func (p *P3) key(number int) store.Key {
	return keyFor(number, p.model, promptVersion(p.profile, p.exploration), p.commitSHA)
}

// stored is what the journal keeps: the level and the answer behind it, so a reuse shows what a
// fresh run would have shown.
type stored struct {
	Result  Result                `json:"result"`
	Answers map[string]nan.Answer `json:"answers,omitempty"`
}

// Run decides one issue's fit against the profile in effect.
//
// A failure of the request is not a level: the result is what the product shows (undecided, I2), and
// the returned error is the cause, for the log. It is not stored either — a stored row would make a
// non-answer look like a pinned decision, and the next run has to be able to read a real answer.
func (p *P3) Run(ctx context.Context, req Request) (Analysis, error) {
	analysis := Analysis{Model: p.model, PromptVersion: promptVersion(p.profile, p.exploration), CommitSHA: p.commitSHA}

	// S7: no profile means no comparison. Asking anyway would invite the judge to invent one, and a
	// verdict is the one thing this product never fabricates.
	if p.profile == "" {
		analysis.Result = Result{
			Level:       LevelInsufficientInformation,
			Reason:      ReasonNoProfile,
			Probability: 1,
		}
		analysis.At = p.now().UTC()
		return analysis, nil
	}

	if cached, ok := p.lookup(req); ok {
		cached.Model, cached.PromptVersion, cached.CommitSHA = p.model, promptVersion(p.profile, p.exploration), p.commitSHA
		return cached, nil
	}

	started := p.now()
	answers, err := p.judge.Ask(ctx, State(req, p.profile, p.portrait), Questions())
	if err != nil {
		analysis.Result = Result{Level: LevelUndecided, Reason: ReasonEndpointFailed}
		return analysis, fmt.Errorf("fit: asking about %s#%d: %w", req.Issue.Repository, req.Issue.Number, err)
	}

	analysis.Answers = answers
	analysis.Result = Decide(answers)
	analysis.At = started.UTC()

	// Nothing that is not a verdict is stored, and storing it is not an error either: the analysis is
	// what was asked for, and the journal only saves a future request.
	if analysis.Level != LevelUndecided {
		_ = p.remember(req, analysis, p.now().Sub(started))
	}
	return analysis, nil
}

// lookup returns a stored verdict for this exact question, if there is one. A damaged entry is a
// miss, not a failed analysis: the pass asks again instead of reporting a broken journal. The
// reading itself lives in Recall, which the screen calls too.
func (p *P3) lookup(req Request) (Analysis, bool) {
	analysis, _, ok := Recall(p.journal, req.Issue.Number, p.model, p.commitSHA, p.profile, p.exploration)
	return analysis, ok
}

func (p *P3) remember(req Request, analysis Analysis, latency time.Duration) error {
	if p.journal == nil {
		return nil
	}
	verdict, err := json.Marshal(stored{Result: analysis.Result, Answers: analysis.Answers})
	if err != nil {
		return fmt.Errorf("fit: encode verdict: %w", err)
	}
	return p.journal.Put(store.Decision{
		Key:       p.key(req.Issue.Number),
		At:        analysis.At,
		LatencyMs: latency.Milliseconds(),
		Verdict:   verdict,
	})
}

// keyFor is the journal identity of this exact question. It is the only place that assembles one,
// so the pass that writes a row and the screen that reads it cannot disagree about what they hold.
//
// The profile does not get a field of its own: store.Key is the identity of a pass over an issue,
// and its contract does not change. The CV travels inside PromptVersion, which is where the product
// already declares which question was asked (I14).
func keyFor(number int, model, promptVersion, commitSHA string) store.Key {
	return store.Key{
		Issue:         number,
		Pass:          store.PassFit,
		Model:         strings.TrimSpace(model),
		PromptVersion: promptVersion,
		CommitSHA:     strings.TrimSpace(commitSHA),
	}
}

// promptVersion names the question: the contract's fingerprint, the profile's and the
// exploration's. The three travel inside store.Key.PromptVersion instead of new fields, because the
// identity of a pass over one issue is what store.Key names and its contract does not change.
//
// The exploration belongs here and not in metadata: the state P3 sends carries that exploration's
// requirements, so re-exploring the issue moves the input the verdict was read from. A row that
// did not fingerprint it would answer for evidence nobody read (I14).
func promptVersion(profile string, exploration Exploration) string {
	return Version() + "+" + profileFingerprint(profile) + "+" + exploration.fingerprint()
}

// profileFingerprint names the profile without storing it. The journal lives beside the corpus and
// is not a place for someone's CV; a hash is enough to tell two documents apart.
func profileFingerprint(profile string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(profile)))
	return hex.EncodeToString(sum[:])[:12]
}

// Recall returns the verdict the journal holds for this exact question, together with the stored
// decision that carries when it was produced and what it cost, because a verdict without its pin
// reads as fresh.
//
// A miss is not a failure: another model, another profile, another exploration, another prompt or
// another commit answers a different question, and a damaged entry is damage rather than an answer.
// The caller shows that there is nothing stored instead of presenting an empty level as a decision.
func Recall(journal *store.Decisions, number int, model, commitSHA, profile string, exploration Exploration) (Analysis, store.Decision, bool) {
	if journal == nil {
		return Analysis{}, store.Decision{}, false
	}
	decision, ok := journal.Get(keyFor(number, model, promptVersion(profile, exploration), commitSHA))
	if !ok {
		return Analysis{}, store.Decision{}, false
	}

	var entry stored
	if err := json.Unmarshal(decision.Verdict, &entry); err != nil {
		return Analysis{}, store.Decision{}, false
	}
	if entry.Result.Level == "" {
		// A stored envelope without a level is damage, not an answer: showing it would mean
		// presenting an empty level as a decision.
		return Analysis{}, store.Decision{}, false
	}

	return Analysis{
		Result:  entry.Result,
		Answers: entry.Answers,
		Cached:  true,
		At:      decision.At,
	}, decision, true
}

// ClientJudge adapts the decision client to Judge. It is the only adapter the product uses; a test
// passes its own.
type ClientJudge struct {
	Client *nan.Client
	Model  string
}

// Ask sends the question and returns the answers by question id.
func (c ClientJudge) Ask(ctx context.Context, state string, questions map[string]nan.Question) (map[string]nan.Answer, error) {
	if c.Client == nil {
		return nil, errors.New("fit: the client judge needs a client")
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
