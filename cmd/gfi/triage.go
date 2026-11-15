package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/config"
	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
	"github.com/sascodiego/nan-hackathon-clef/internal/pig"
	"github.com/sascodiego/nan-hackathon-clef/internal/projectbrief"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
	"github.com/sascodiego/nan-hackathon-clef/internal/triage"
)

// runTriage is the product's first pass over a frozen corpus. It is the verb the benchmark
// never had: the chain, the questions and the policy live in internal/triage, and this
// command is the only shipped path that runs them over a whole snapshot.
//
// It refuses to start without the decision key. A first pass that booted and then died on the
// eleventh issue would have written half a journal and burned half the quota to say what it
// already knew at the first line (I2).
func runTriage(args []string) error {
	fs := flag.NewFlagSet("triage", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config.json")
	limit := fs.Int("limit", 0, "triage at most N issues (0 means every issue in the snapshot)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("triage needs exactly one OWNER/REPO argument\n\n%s", usage)
	}
	if *limit < 0 {
		return fmt.Errorf("limit %d must not be negative", *limit)
	}

	key := strings.TrimSpace(os.Getenv("NAN_KEY"))
	if key == "" {
		return fmt.Errorf("triage needs NAN_KEY in the environment: " +
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

	// GitHub is the authority on the repository's name: a snapshot stored under the requested
	// name may belong to the canonical one, and the journal is keyed by it.
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

	// The retry budget is configured as retries; the client counts total attempts, the first
	// one included. Adding the first attempt here is what keeps clef.max_retries = 5 meaning
	// five retries rather than four.
	client, err := nan.New(nan.Options{
		SystemOneURL: cfg.Clef.URL,
		APIKey:       key,
		Timeout:      cfg.Clef.Timeout.Get(),
		MaxAttempts:  cfg.Clef.MaxRetries + 1,
	})
	if err != nil {
		return fmt.Errorf("the decision client refuses the configuration: %w", err)
	}

	run := triageRun{
		repoKey:  repoKey,
		snapshot: snap,
		journal:  journal,
		judge:    triage.ClientJudge{Client: client, Model: config.DecisionModel},
		// The node reads the project's portrait, and the README brief when the portrait cannot be
		// produced. A clone that is not there is a condition the run reports, never a crash.
		brief:     alignmentBrief(cfg, st, repoKey),
		model:     config.DecisionModel,
		commitSHA: snap.Repository.CommitSHA,
		limit:     *limit,
		out:       os.Stdout,
		now:       time.Now,
	}
	_, err = run.run(context.Background())
	return err
}

// alignmentBrief is what the alignment node reads: the project portrait, and the free README brief
// as the fallback when the portrait cannot be produced.
//
// The portrait costs one agent run per repository and commit, spent the first time a feature
// reaches the node and reused for the rest of the batch; the fallback costs no request at all. A
// pass over a hundred issues is never aborted over the richer input: the degradation is reported
// and the node reads the cheaper one.
func alignmentBrief(cfg *config.Config, st *store.Store, repoKey string) triage.BriefSource {
	brief := triage.FallbackBrief{
		Fallback: triage.CloneBrief{Root: cfg.Repos.CloneDir},
		OnFailure: func(err error) {
			slog.Warn("the alignment node reads the README brief: the portrait was not produced",
				"repo", repoKey, "err", err)
		},
	}
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
		slog.Warn("the portrait is not configured: the alignment node reads the README brief", "err", err)
		return brief
	}
	if err := prepareBriefHome(cfg); err != nil {
		slog.Warn("the portrait cannot run: the alignment node reads the README brief", "err", err)
		return brief
	}
	brief.Primary = projectbrief.Source{Extractor: extractor, Root: cfg.Repos.CloneDir}
	return brief
}

// triageRun is one pass over one snapshot. Everything it needs is a field so a test can run
// the production chain with a judge and a brief of its own and no network at all.
type triageRun struct {
	repoKey   string
	snapshot  *ghdata.Snapshot
	journal   *store.Decisions
	judge     triage.Judge
	brief     triage.BriefSource
	model     string
	commitSHA string
	limit     int
	out       io.Writer
	now       func() time.Time
}

// triageSummary is what the run measured. The first three counts are the ones the README's
// measured cost is built from: requests made, verdicts reused, and the breakdown the report
// has to match.
type triageSummary struct {
	Corpus   int
	Triaged  int
	Requests int
	Cached   int
	Failed   int
	Degraded int
	Elapsed  time.Duration
	Outcomes map[triage.Outcome]int
	Reasons  map[string]int
}

// run triages the corpus and prints what it cost.
//
// A failure of one issue never stops the corpus: the run keeps going, the journal keeps
// whatever the endpoint could answer, and the summary carries the count. The error returned at
// the end is the exit code, so a run that could not decide everything says so instead of
// passing for a complete measurement.
func (r triageRun) run(ctx context.Context) (triageSummary, error) {
	sum := triageSummary{
		Corpus:   len(r.snapshot.Issues),
		Outcomes: make(map[triage.Outcome]int),
		Reasons:  make(map[string]int),
	}

	issues := r.snapshot.Issues
	if r.limit > 0 && r.limit < len(issues) {
		issues = issues[:r.limit]
	}

	counting := &countingJudge{inner: r.judge}
	p1, err := triage.New(triage.Options{
		Judge:     counting,
		Journal:   r.journal,
		Model:     r.model,
		CommitSHA: r.commitSHA,
		Brief:     r.brief,
		Now:       r.now,
	})
	if err != nil {
		return sum, err
	}

	started := r.now()
	for _, issue := range issues {
		analysis, runErr := p1.Run(ctx, toTriageIssue(r.repoKey, issue))
		sum.Triaged++
		sum.Outcomes[analysis.Outcome]++
		if analysis.Reason != "" {
			sum.Reasons[analysis.Reason]++
		}
		switch {
		case analysis.Cached:
			sum.Cached++
		case analysis.Outcome == triage.Undecided:
			// The request failed: it is not a verdict about the issue (I2), so the run keeps
			// going and the count is what makes the failure impossible to miss.
			sum.Failed++
		case runErr != nil:
			// The verdict stands; what could not be read is the project's direction.
			sum.Degraded++
		}
	}
	sum.Requests = counting.calls
	sum.Elapsed = r.now().Sub(started)

	r.print(sum)

	if sum.Failed > 0 {
		return sum, fmt.Errorf("triage: %d of %d issues stayed undecided: "+
			"the decision endpoint failed and a failure is not a verdict (I2)", sum.Failed, sum.Triaged)
	}
	return sum, nil
}

// print reports the run in the shape the README's cost table can quote: issues, requests,
// reuse and the breakdown by outcome and reason.
func (r triageRun) print(sum triageSummary) {
	_, _ = fmt.Fprintf(r.out, "triaged %s\n", r.repoKey)
	_, _ = fmt.Fprintf(r.out, "  corpus      %s\n", corpusLine(sum))
	if sum.Cached > 0 {
		_, _ = fmt.Fprintf(r.out, "  requests    %d (%d answered from the journal)\n", sum.Requests, sum.Cached)
	} else {
		_, _ = fmt.Fprintf(r.out, "  requests    %d\n", sum.Requests)
	}
	_, _ = fmt.Fprintf(r.out, "  elapsed     %s\n", sum.Elapsed.Round(time.Millisecond))
	_, _ = fmt.Fprintf(r.out, "  result      %d triaged · %d failed · %d degraded\n",
		sum.Triaged-sum.Failed, sum.Failed, sum.Degraded)
	_, _ = fmt.Fprintf(r.out, "  outcomes    %s\n", countLine(outcomeCounts(sum.Outcomes)))
	_, _ = fmt.Fprintf(r.out, "  reasons     %s\n", countLine(sum.Reasons))
}

// corpusLine says how much of the snapshot the run read, and names the cap when it applied.
func corpusLine(sum triageSummary) string {
	switch {
	case sum.Corpus == 0:
		return "0 issues: run `gfi fetch` on a repository with open issues"
	case sum.Triaged < sum.Corpus:
		return fmt.Sprintf("%d of %d issues (limit %d)", sum.Triaged, sum.Corpus, sum.Triaged)
	default:
		return issues(sum.Corpus)
	}
}

// issues is the one place the count is written with its noun, so a corpus of one does not
// read as a bug in the summary.
func issues(n int) string {
	if n == 1 {
		return "1 issue"
	}
	return fmt.Sprintf("%d issues", n)
}

// countLine renders a breakdown in a stable order, so two runs of the same corpus diff
// cleanly and the numbers in the report can be compared line by line.
func countLine(counts map[string]int) string {
	if len(counts) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, counts[k]))
	}
	return strings.Join(parts, " · ")
}

func outcomeCounts(counts map[triage.Outcome]int) map[string]int {
	out := make(map[string]int, len(counts))
	for outcome, n := range counts {
		out[string(outcome)] = n
	}
	return out
}

// countingJudge is the run's own tally. The client retries inside one Ask, so this counts
// decision requests as the first pass spends them: one per issue, plus the alignment node for
// a feature.
type countingJudge struct {
	inner triage.Judge
	calls int
}

func (c *countingJudge) Ask(ctx context.Context, state string, questions map[string]nan.Question) (map[string]nan.Answer, error) {
	c.calls++
	return c.inner.Ask(ctx, state, questions)
}

// toTriageIssue adapts one stored issue to what the first pass reads. The decision package
// takes a plain value on purpose, so this is where the tracker's shape stops: every field a
// dossier can use is carried, and the label list is copied verbatim (I6: it is text the
// maintainer wrote, never a filter).
func toTriageIssue(repoKey string, in ghdata.Issue) triage.Issue {
	out := triage.Issue{
		Repository:        repoKey,
		Number:            in.Number,
		Author:            in.Author,
		Title:             in.Title,
		Body:              in.Body,
		Labels:            in.Labels,
		GitHubState:       in.State,
		GitHubStateReason: in.StateReason,
	}
	for _, c := range in.Comments {
		out.Comments = append(out.Comments, triage.Comment{
			Author:            c.Author,
			Body:              c.Body,
			AuthorAssociation: c.AuthorAssociation,
		})
	}
	for _, pr := range in.LinkedPRs {
		out.LinkedPRs = append(out.LinkedPRs, triage.LinkedPR{
			Number: pr.Number,
			State:  pr.State,
			Merged: pr.Merged,
		})
	}
	return out
}
