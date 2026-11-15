package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/config"
	"github.com/sascodiego/nan-hackathon-clef/internal/explore"
	"github.com/sascodiego/nan-hackathon-clef/internal/fit"
	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
	"github.com/sascodiego/nan-hackathon-clef/internal/pig"
	"github.com/sascodiego/nan-hackathon-clef/internal/profile"
	"github.com/sascodiego/nan-hackathon-clef/internal/projectbrief"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
	"github.com/sascodiego/nan-hackathon-clef/internal/triage"
)

// errNoProfile is the refusal of S7: without an approved profile there is no comparison, so the
// only honest run is no run. It says how a profile is produced, because a refusal that does not
// say what to do next is a dead end.
var errNoProfile = errors.New("fit needs an approved profile: nothing was requested. " +
	"extract a CV, review it and approve it in the profile screen (/profile) first")

// errNoExploration is the other half of the question (I14): the state carries the requirements of
// one exploration, so without its identity the stored row could not say what it answers. It is a
// refusal, never an invented identity: an empty fingerprint would reuse one verdict everywhere.
var errNoExploration = errors.New("fit needs the explore model from the configuration: " +
	"the question the verdict answers includes the exploration its evidence came from, " +
	"and nothing was requested")

// The reasons an issue never reaches the third pass. They are printed verbatim, so the summary
// says what the selection left out and not only how much.
const (
	// skipNoTriage is an issue the first pass never decided. Nothing is known about whether it
	// is worth fitting, and a guess would be spent on the corpus's whole backlog.
	skipNoTriage = "no stored triage"
	// skipNoExploration is a candidate the exploration never reached: there is no evidence to
	// weigh against the profile, so the request would only ask the judge to invent one.
	skipNoExploration = "no stored exploration"
)

// runFit is the product's third pass over a frozen corpus: one request per issue, one level, and
// the cost printed like the first pass prints it.
//
// It refuses to start without the decision key or without an approved profile. A run that booted
// and then died halfway would have written half a journal and burned half the quota to say what
// it already knew at the first line (I2, S7).
func runFit(args []string) error {
	fs := flag.NewFlagSet("fit", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config.json")
	limit := fs.Int("limit", 0, "fit at most N issues (0 means every issue in the snapshot)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("fit needs exactly one OWNER/REPO argument\n\n%s", usage)
	}
	if *limit < 0 {
		return fmt.Errorf("limit %d must not be negative", *limit)
	}

	key := strings.TrimSpace(os.Getenv("NAN_KEY"))
	if key == "" {
		return fmt.Errorf("fit needs NAN_KEY in the environment: " +
			"it is what authenticates the decision model, and nothing was requested")
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	owner, name, err := splitTarget(fs.Arg(0))
	if err != nil {
		return err
	}
	st, err := store.New(cfg.Store.Dir)
	if err != nil {
		return err
	}

	// GitHub is the authority on the repository's name: a snapshot stored under the requested name
	// may belong to the canonical one, and the journal is keyed by it.
	requested := owner + "/" + name
	if canonical, ok := st.Alias(requested); ok {
		if co, cn, splitErr := splitTarget(canonical); splitErr == nil {
			owner, name = co, cn
		}
	}
	repoKey := store.RepoName(owner, name)
	snap, err := st.LoadSnapshot(repoKey)
	if err != nil {
		return fmt.Errorf("no snapshot for %s: run `gfi fetch %s` first", requested, requested)
	}
	if co, cn, splitErr := splitTarget(snap.Repository.NameWithOwner); splitErr == nil {
		repoKey = store.RepoName(co, cn)
	}
	if strings.TrimSpace(snap.Repository.CommitSHA) == "" {
		return fmt.Errorf("the snapshot of %s pins no commit: run `gfi fetch %s` again", requested, requested)
	}

	journal, err := st.Decisions(repoKey)
	if err != nil {
		return err
	}

	// The profile is the question (S6). Without one there is nothing to compare against, and the
	// verb refuses before the client exists, so a missing profile costs nothing.
	document, ok, err := profile.Load(st)
	if err != nil {
		return err
	}
	if !ok {
		return errNoProfile
	}

	// The identity of the evidence the state carries. Without it the run cannot name the question,
	// so it refuses before the client exists instead of fingerprinting an absence (I14).
	exploration := fit.Exploration{PromptVersion: explore.Version(), Model: cfg.Explore.Model}
	if !exploration.Known() {
		return errNoExploration
	}

	// The retry budget is configured as retries; the client counts total attempts, the first one
	// included. Adding the first attempt here is what keeps clef.max_retries = 5 meaning five
	// retries rather than four.
	client, err := nan.New(nan.Options{
		SystemOneURL: cfg.Clef.URL,
		APIKey:       key,
		Timeout:      cfg.Clef.Timeout.Get(),
		MaxAttempts:  cfg.Clef.MaxRetries + 1,
	})
	if err != nil {
		return fmt.Errorf("the decision client refuses the configuration: %w", err)
	}

	ctx := context.Background()
	run := fitRun{
		repoKey:  repoKey,
		snapshot: snap,
		journal:  journal,
		judge:    fit.ClientJudge{Client: client, Model: config.DecisionModel},
		profile:  document,
		// The evidence the state carries is the exploration's, so its identity is half of the
		// question the stored row answers (I14). It is also the key half that reads the row.
		exploration: exploration,
		// The portrait is read once for the whole corpus: it describes the tree, not the issue.
		portrait:  fitPortrait(cfg, st)(ctx, repoKey),
		model:     config.DecisionModel,
		commitSHA: snap.Repository.CommitSHA,
		limit:     *limit,
		out:       os.Stdout,
		now:       time.Now,
	}
	_, err = run.run(ctx)
	return err
}

// fitPortrait reads the technical half of the project portrait, which is the only part of it that
// crosses with the profile (S5). A repository without a portrait is a condition the run reports:
// the state says the stack was never extracted and the pass asks anyway (I10). A portrait that
// cannot be produced never aborts a pass over a whole corpus.
func fitPortrait(cfg *config.Config, st *store.Store) func(context.Context, string) string {
	none := func(context.Context, string) string { return "" }

	extractor, err := projectbrief.New(projectbrief.Options{
		Run:      pig.Run,
		Model:    cfg.Models.ProjectBrief,
		Binary:   cfg.ProjectBrief.Binary,
		Home:     cfg.ProjectBrief.Home,
		Tools:    cfg.ProjectBrief.Tools,
		IdleWarn: cfg.ProjectBrief.IdleWarn.Get(),
		IdleKill: cfg.ProjectBrief.IdleKill.Get(),
		Budget:   cfg.ProjectBrief.Budget.Get(),
		Store:    st,
	})
	if err != nil {
		slog.Warn("the portrait is not configured: the fit state travels without a technical stack", "err", err)
		return none
	}
	if err := prepareBriefHome(cfg); err != nil {
		slog.Warn("the portrait cannot run: the fit state travels without a technical stack", "err", err)
		return none
	}

	source := projectbrief.Source{Extractor: extractor, Root: cfg.Repos.CloneDir}
	return func(ctx context.Context, repository string) string {
		result, err := source.Portrait(ctx, repository)
		if err != nil {
			slog.Warn("the portrait was not produced: the fit state travels without a technical stack",
				"repo", repository, "err", err)
			return ""
		}
		return result.Document.Text
	}
}

// fitRun is one pass over one snapshot. Everything it needs is a field so a test can run the
// production pass with a judge, a profile and an exploration of its own and no network at all.
type fitRun struct {
	repoKey  string
	snapshot *ghdata.Snapshot
	journal  *store.Decisions
	judge    fit.Judge
	// profile is the saved profile's document, verbatim. Empty means there is nothing to compare
	// against and the run refuses.
	profile string
	// portrait is the project portrait, read once for the corpus. Empty means none was produced.
	portrait string
	// model is the decision model from the constitution (I13), never config.json.
	model     string
	commitSHA string
	// exploration is the identity of the evidence the pass reads: its key half, so a run that read
	// another exploration would read nothing, and its question half, because the state carries that
	// exploration's requirements (I14).
	exploration fit.Exploration
	limit       int
	out         io.Writer
	now         func() time.Time
}

// fitSummary is what the run measured. The counts are the same shape the first pass reports, with
// one addition: what the conservative selection left out, and why.
type fitSummary struct {
	Corpus   int
	Scanned  int
	Fitted   int
	Requests int
	Cached   int
	Failed   int
	Skipped  int
	Elapsed  time.Duration
	Levels   map[fit.Level]int
	Skips    map[string]int
}

// run decides the fit of the stored corpus and prints what it cost.
//
// A failure of one issue never stops the corpus, and it never becomes a level: the result is
// undecided (I2), the count makes it impossible to miss, and the error returned at the end is the
// exit code so a run that could not decide everything does not pass for a complete measurement.
func (r fitRun) run(ctx context.Context) (fitSummary, error) {
	sum := fitSummary{
		Corpus: len(r.snapshot.Issues),
		Levels: make(map[fit.Level]int),
		Skips:  make(map[string]int),
	}

	// S7: without a profile there is no comparison to make, so nothing is asked. The pass is not
	// built either: fit.New refuses a journal without one, which is the same rule one level down.
	if strings.TrimSpace(r.profile) == "" {
		return sum, errNoProfile
	}

	// Same rule one level down: the pass cannot fall back to an exploration it was not told about,
	// so the run refuses instead of reading and storing a question it cannot name (I14).
	if !r.exploration.Known() {
		return sum, errNoExploration
	}

	issues := r.snapshot.Issues
	if r.limit > 0 && r.limit < len(issues) {
		issues = issues[:r.limit]
	}
	sum.Scanned = len(issues)

	counting := &countingFitJudge{inner: r.judge}
	p3, err := fit.New(fit.Options{
		Judge:       counting,
		Journal:     r.journal,
		Model:       r.model,
		CommitSHA:   r.commitSHA,
		Profile:     r.profile,
		Exploration: r.exploration,
		Portrait:    r.portrait,
		Now:         r.now,
	})
	if err != nil {
		return sum, err
	}

	started := r.now()
	for _, issue := range issues {
		// The selection is conservative on purpose: only a candidate the first pass recommended,
		// and only one the exploration reached, is worth a request. Anything else is counted.
		if reason := r.worth(issue); reason != "" {
			sum.Skipped++
			sum.Skips[reason]++
			continue
		}

		evidence, ok := r.evidenceFor(issue.Number)
		if !ok {
			sum.Skipped++
			sum.Skips[skipNoExploration]++
			continue
		}

		analysis, _ := p3.Run(ctx, fit.Request{
			Issue: fit.Issue{
				Repository: r.repoKey,
				Number:     issue.Number,
				Title:      issue.Title,
				Body:       issue.Body,
				// The label is copied because the corpus copies it verbatim, never because the
				// judgement reads it (I6): the state has no field for it.
				Labels: issue.Labels,
			},
			Repository: fit.Repository{
				Name:            r.snapshot.Repository.NameWithOwner,
				PrimaryLanguage: r.snapshot.Repository.PrimaryLanguage,
			},
			Evidence: evidence,
		})
		sum.Fitted++
		sum.Levels[analysis.Level]++
		switch {
		case analysis.Cached:
			sum.Cached++
		case analysis.Level == fit.LevelUndecided:
			// The request failed: it is not a level about the issue (I2), so the run keeps going
			// and the count is what makes the failure impossible to miss.
			sum.Failed++
		}
	}
	sum.Requests = counting.calls
	sum.Elapsed = r.now().Sub(started)

	r.print(sum)

	if sum.Failed > 0 {
		return sum, fmt.Errorf("fit: %d of %d issues stayed undecided: "+
			"the decision endpoint failed and a failure is not a verdict (I2)", sum.Failed, sum.Fitted)
	}
	return sum, nil
}

// worth is the first half of the selection: it reports why an issue is not worth a request, and
// the empty string when it is.
//
// The stored P1 verdict is what decides, and only a recommendation opens the third pass. The
// other outcomes are decisions the first pass already made — marked, doubted, conditioned,
// derived — and asking a second, more expensive question about them would present a level where
// the product already said something else. An issue the first pass never saw is not "not
// recommended": it is unknown, which is the same thing for this run.
func (r fitRun) worth(issue ghdata.Issue) string {
	verdict, _, ok := triage.Recall(r.journal, issue.Number, config.DecisionModel, r.commitSHA)
	switch {
	case !ok:
		return skipNoTriage
	case verdict.Outcome != triage.Candidate:
		return string(verdict.Outcome)
	}
	return ""
}

// evidenceFor reads the exploration the issue's requirements came from. The key is built from the
// same identity the fit key fingerprints, so the row this run reads and the question it stores
// cannot disagree; explore.Session.Key builds the identical key when the identity is the
// configured one (I4).
func (r fitRun) evidenceFor(number int) (explore.Evidence, bool) {
	if r.journal == nil || !r.exploration.Known() {
		return explore.Evidence{}, false
	}
	decision, ok := r.journal.Get(store.Key{
		Issue:         number,
		Pass:          store.PassExplore,
		Model:         r.exploration.Model,
		PromptVersion: r.exploration.PromptVersion,
		CommitSHA:     r.commitSHA,
	})
	if !ok {
		return explore.Evidence{}, false
	}

	var record explore.Record
	if err := json.Unmarshal(decision.Verdict, &record); err != nil || record.Evidence == nil {
		// A record that does not decode is not evidence: it is a reason to skip, exactly like an
		// exploration that was never stored.
		return explore.Evidence{}, false
	}
	return *record.Evidence, true
}

// print reports the run in the same shape the first pass uses: issues, requests, reuse, and the
// breakdown, plus what the conservative selection left out.
func (r fitRun) print(sum fitSummary) {
	_, _ = fmt.Fprintf(r.out, "fitted %s\n", r.repoKey)
	_, _ = fmt.Fprintf(r.out, "  corpus      %s\n", fitCorpusLine(sum))
	if sum.Cached > 0 {
		_, _ = fmt.Fprintf(r.out, "  requests    %d (%d answered from the journal)\n", sum.Requests, sum.Cached)
	} else {
		_, _ = fmt.Fprintf(r.out, "  requests    %d\n", sum.Requests)
	}
	_, _ = fmt.Fprintf(r.out, "  elapsed     %s\n", sum.Elapsed.Round(time.Millisecond))
	_, _ = fmt.Fprintf(r.out, "  result      %d fitted · %d failed\n", sum.Fitted-sum.Failed, sum.Failed)
	_, _ = fmt.Fprintf(r.out, "  levels      %s\n", countLine(levelCounts(sum.Levels)))
	if sum.Skipped > 0 {
		_, _ = fmt.Fprintf(r.out, "  skipped     %s: %s\n", issues(sum.Skipped), countLine(sum.Skips))
		_, _ = fmt.Fprintf(r.out, "  selection   only CANDIDATA issues with a stored exploration reached the third pass\n")
	}
}

// fitCorpusLine says how much of the snapshot the run read, and names the cap when it applied.
func fitCorpusLine(sum fitSummary) string {
	switch {
	case sum.Corpus == 0:
		return "0 issues: run `gfi fetch` on a repository with open issues"
	case sum.Scanned < sum.Corpus:
		return fmt.Sprintf("%d of %d issues (limit %d)", sum.Scanned, sum.Corpus, sum.Scanned)
	default:
		return issues(sum.Corpus)
	}
}

// levelCounts renders the levels with their wire names, so the summary can be quoted in the
// README's cost table next to the first pass's outcomes.
func levelCounts(levels map[fit.Level]int) map[string]int {
	out := make(map[string]int, len(levels))
	for level, n := range levels {
		out[string(level)] = n
	}
	return out
}

// countingFitJudge is the run's own tally. The client retries inside one Ask, so this counts
// decision requests as the third pass spends them: one per issue.
type countingFitJudge struct {
	inner fit.Judge
	calls int
}

func (c *countingFitJudge) Ask(ctx context.Context, state string, questions map[string]nan.Question) (map[string]nan.Answer, error) {
	c.calls++
	return c.inner.Ask(ctx, state, questions)
}
