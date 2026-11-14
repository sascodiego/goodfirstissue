// Comando gfi: el binario único de GoodFirstIssue.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/config"
	"github.com/sascodiego/nan-hackathon-clef/internal/explore"
	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/indexer"
	"github.com/sascodiego/nan-hackathon-clef/internal/pig"
	"github.com/sascodiego/nan-hackathon-clef/internal/repo"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
	"github.com/sascodiego/nan-hackathon-clef/internal/web"
)

// shutdownTimeout es lo que se espera a que terminen las peticiones en vuelo.
const shutdownTimeout = 5 * time.Second

const usage = `gfi — GoodFirstIssue

  gfi [-addr ADDR] [-config FILE]   start the server
  gfi config example               print the default configuration
  gfi config check [FILE]          validate a configuration file
  gfi fetch [-config FILE] OWNER/REPO    fetch the open issues into the store
  gfi index [-config FILE] OWNER/REPO    clone if needed and build the code graph
  gfi explore [-config FILE] OWNER/REPO ISSUE   run the read-only agent on one issue
  gfi repos [-config FILE]                       list the known repositories
  gfi version                      print the build identity
`

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(os.Args[1:]); err != nil {
		slog.Error("gfi", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "config":
			return runConfig(args[1:])
		case "fetch":
			return runFetch(args[1:])
		case "index":
			return runIndex(args[1:])
		case "explore":
			return runExplore(args[1:])
		case "repos":
			return runRepos(args[1:])
		case "version":
			fmt.Println("gfi dev")
			return nil
		case "help", "-h", "--help":
			fmt.Print(usage)
			return nil
		}
	}
	return runServer(args)
}

func runConfig(args []string) error {
	if len(args) == 0 || args[0] == "example" {
		b, err := config.Example()
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(b)
		return err
	}
	if args[0] != "check" {
		return fmt.Errorf("unknown config command %q\n\n%s", args[0], usage)
	}

	path := "config.json"
	if len(args) > 1 {
		path = args[1]
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	if err := cfg.Resolve("."); err != nil {
		return err
	}
	fmt.Printf("ok: %s\n", path)
	fmt.Printf("  server      %s\n", cfg.Server.Addr)
	fmt.Printf("  clones      %s\n", cfg.Repos.CloneDir)
	fmt.Printf("  store       %s\n", cfg.Store.Dir)
	fmt.Printf("  pig home    %s\n", cfg.Explore.Home)
	fmt.Printf("  explore     %s · %s\n", cfg.Explore.Model, cfg.Explore.Binary)
	fmt.Printf("  tools       %v\n", cfg.Explore.Tools)
	return nil
}

func runServer(args []string) error {
	fs := flag.NewFlagSet("gfi", flag.ContinueOnError)
	addr := fs.String("addr", "", "listen address (defaults to config server.addr)")
	configPath := fs.String("config", "", "path to config.json")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := config.Default()
	if *configPath != "" {
		loaded, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		cfg = loaded
		if err := cfg.EnsureDirs(); err != nil {
			return err
		}
	}
	if *addr != "" {
		cfg.Server.Addr = *addr
	}

	st, err := store.New(cfg.Store.Dir)
	if err != nil {
		return err
	}

	handler, err := web.NewHandler(web.Options{
		Store:   st,
		Fetcher: &ghdata.Client{},
		GitHub: ghdata.Options{
			MaxIssues: cfg.GitHub.MaxIssues,
			PageSize:  cfg.GitHub.PageSize,
		},
		Timeout: cfg.GitHub.Timeout.Get(),
	})
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("servidor arriba", "addr", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("apagando")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// loadConfig reads the file when given, applies defaults otherwise, and makes the
// directories the application owns.
func loadConfig(path string) (*config.Config, error) {
	cfg := config.Default()
	if path != "" {
		loaded, err := config.Load(path)
		if err != nil {
			return nil, err
		}
		cfg = loaded
	}
	if err := cfg.Resolve("."); err != nil {
		return nil, err
	}
	if err := cfg.EnsureDirs(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// splitTarget reads the OWNER/REPO argument.
func splitTarget(target string) (owner, name string, err error) {
	parts := strings.Split(target, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("target %q must look like OWNER/REPO", target)
	}
	return parts[0], parts[1], nil
}

func targetFlags(name string, args []string) (*config.Config, string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config.json")
	if err := fs.Parse(args); err != nil {
		return nil, "", err
	}
	if fs.NArg() != 1 {
		return nil, "", fmt.Errorf("%s needs exactly one OWNER/REPO argument\n\n%s", name, usage)
	}
	cfg, err := loadConfig(*configPath)
	return cfg, fs.Arg(0), err
}

// runFetch stores the open issues of one repository. No label filter is applied: the
// whole open set is the corpus.
func runFetch(args []string) error {
	cfg, target, err := targetFlags("fetch", args)
	if err != nil {
		return err
	}
	owner, name, err := splitTarget(target)
	if err != nil {
		return err
	}
	st, err := store.New(cfg.Store.Dir)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.GitHub.Timeout.Get())
	defer cancel()

	snap, err := (&ghdata.Client{}).Fetch(ctx, owner, name, ghdata.Options{
		MaxIssues: cfg.GitHub.MaxIssues,
		PageSize:  cfg.GitHub.PageSize,
	})
	empty := errors.Is(err, ghdata.ErrNoIssues)
	if err != nil && !empty {
		return err
	}
	if snap == nil {
		return err
	}

	// GitHub is the authority on a repository's name: a request for httpie/httpie answers
	// httpie/cli. Keying the store by the requested name would split one repository into
	// two directories the first time it is renamed.
	canonOwner, canonName := owner, name
	if co, cn, splitErr := splitTarget(snap.Repository.NameWithOwner); splitErr == nil {
		canonOwner, canonName = co, cn
	}
	if canonOwner != owner || canonName != name {
		fmt.Printf("  note        GitHub reports the canonical name %s/%s\n", canonOwner, canonName)
	}

	repoName := store.RepoName(canonOwner, canonName)
	if err := st.SaveSnapshot(repoName, snap); err != nil {
		return err
	}
	if err := st.SaveAlias(owner+"/"+name, canonOwner+"/"+canonName); err != nil {
		return err
	}

	if err := st.SaveRepo(store.RepoEntry{
		Name:      snap.Repository.NameWithOwner,
		Key:       repoName,
		CommitSHA: snap.Repository.CommitSHA,
		TotalOpen: snap.TotalOpen,
		Issues:    len(snap.Issues),
	}); err != nil {
		return err
	}

	fmt.Printf("fetched %s\n", snap.Repository.NameWithOwner)
	fmt.Printf("  commit      %s\n", snap.Repository.CommitSHA)
	fmt.Printf("  open        %d\n", snap.TotalOpen)
	fmt.Printf("  stored      %d issues, truncated=%v\n", len(snap.Issues), snap.Truncated)
	fmt.Printf("  file        %s\n", st.Path(repoName+"/snapshot.json"))
	if empty {
		fmt.Println("  note        the repository has no open issues")
	}
	return nil
}

// runIndex clones the repository when needed and builds its code graph.
func runIndex(args []string) error {
	cfg, target, err := targetFlags("index", args)
	if err != nil {
		return err
	}
	owner, name, err := splitTarget(target)
	if err != nil {
		return err
	}
	st, err := store.New(cfg.Store.Dir)
	if err != nil {
		return err
	}

	// Indexing reads the snapshot first: it carries the canonical name and the commit the
	// graph must be pinned to. Without it there is no corpus to index for.
	requested := owner + "/" + name
	if canonical, ok := st.Alias(requested); ok {
		if co, cn, splitErr := splitTarget(canonical); splitErr == nil {
			owner, name = co, cn
			fmt.Printf("  note        %s is GitHub's name for %s\n", canonical, requested)
		}
	}

	repoKey := store.RepoName(owner, name)
	snap, err := st.LoadSnapshot(repoKey)
	if err != nil {
		return fmt.Errorf("no snapshot for %s: run `gfi fetch %s` first", requested, requested)
	}
	if co, cn, splitErr := splitTarget(snap.Repository.NameWithOwner); splitErr == nil {
		owner, name = co, cn
		repoKey = store.RepoName(owner, name)
	}

	dir := cfg.RepoDir(owner, name)
	url := "https://github.com/" + owner + "/" + name + ".git"

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Indexer.Timeout.Get())
	defer cancel()

	cloner := &repo.Cloner{}
	sha, cloned, err := cloner.Ensure(ctx, url, dir)
	if err != nil {
		return err
	}

	stats, err := (&indexer.Indexer{Binary: cfg.Indexer.Binary}).Prepare(ctx, dir)
	if err != nil {
		return err
	}

	record := store.IndexRecord{
		Repository: repoKey,
		CommitSHA:  sha,
		Path:       dir,
		Stats:      stats,
	}
	if err := st.SaveIndex(record.Repository, record); err != nil {
		return err
	}

	fmt.Printf("indexed %s/%s\n", owner, name)
	fmt.Printf("  cloned      %v\n", cloned)
	fmt.Printf("  commit      %s\n", sha)
	fmt.Printf("  files       %d\n", stats.Files)
	fmt.Printf("  nodes       %d\n", stats.Nodes)
	fmt.Printf("  edges       %d\n", stats.Edges)
	fmt.Printf("  path        %s\n", dir)
	return nil
}

// runExplore turns one issue from the frozen corpus into evidence, reusing an identical
// previous analysis instead of paying for it twice.
func runExplore(args []string) error {
	fs := flag.NewFlagSet("explore", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config.json")
	force := fs.Bool("force", false, "ignore a cached analysis and run it again")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("explore needs OWNER/REPO and an issue number\n\n%s", usage)
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	owner, name, err := splitTarget(fs.Arg(0))
	if err != nil {
		return err
	}
	var number int
	if _, err := fmt.Sscanf(fs.Arg(1), "%d", &number); err != nil || number <= 0 {
		return fmt.Errorf("issue %q is not a number", fs.Arg(1))
	}

	st, err := store.New(cfg.Store.Dir)
	if err != nil {
		return err
	}
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
	var issue *ghdata.Issue
	for i := range snap.Issues {
		if snap.Issues[i].Number == number {
			issue = &snap.Issues[i]
			break
		}
	}
	if issue == nil {
		return fmt.Errorf("issue #%d is not in the snapshot of %s", number, repoKey)
	}

	rec, err := st.LoadIndex(repoKey)
	if err != nil {
		return fmt.Errorf("no index for %s: run `gfi index %s` first", requested, requested)
	}
	if rec.CommitSHA != snap.Repository.CommitSHA {
		fmt.Printf("  warning     index is at %s but the snapshot pins %s\n",
			short(rec.CommitSHA), short(snap.Repository.CommitSHA))
	}

	// Everything that can change the answer is part of the key, so a changed prompt, a
	// changed model or a moved commit misses instead of reusing a stale verdict.
	cacheKey := store.Key{
		Issue:         number,
		Pass:          store.PassExplore,
		Model:         cfg.Explore.Model,
		PromptVersion: explore.Version(),
		CommitSHA:     snap.Repository.CommitSHA,
	}
	decisions, err := st.Decisions(repoKey)
	if err != nil {
		return err
	}
	if !*force {
		if cached, ok := decisions.Get(cacheKey); ok {
			var previous explore.Record
			if err := json.Unmarshal(cached.Verdict, &previous); err == nil && previous.Evidence != nil {
				fmt.Printf("cached      %s#%d analysed %s, %s of work reused\n",
					repoKey, number, cached.At.Format("2006-01-02 15:04 UTC"),
					(time.Duration(cached.LatencyMs) * time.Millisecond).Round(time.Millisecond))
				fmt.Printf("              prompt %s · model %s · commit %s\n",
					cacheKey.PromptVersion, cacheKey.Model, short(cacheKey.CommitSHA))
				fmt.Printf("              use -force to run it again\n")
				printEvidence(&previous)
				return nil
			}
			// A verdict that no longer decodes is not reused; the run below replaces it.
			fmt.Printf("  stale       the stored analysis is unreadable, running again\n")
		}
	}

	// The application owns the agent's configuration: generated, never inherited.
	if err := pig.WriteConfig(cfg.Explore.Home, pig.DefaultConfig(config.AllowedChatModels)); err != nil {
		return err
	}
	skillPath, err := explore.Prepare(cfg.Explore.Home)
	if err != nil {
		return err
	}

	fmt.Printf("exploring %s#%d with %s\n", repoKey, number, cfg.Explore.Model)
	out, err := pig.Run(context.Background(), pig.Request{
		Dir:           rec.Path,
		Prompt:        explore.PromptFor(snap.Repository.NameWithOwner, snap.Repository.CommitSHA, issue.Number, issue.Title, issue.Body, issue.Labels),
		Model:         cfg.Explore.Model,
		Tools:         cfg.Explore.Tools,
		SystemPrompts: []string{explore.Prompt},
		Skills:        []string{skillPath},
		Home:          cfg.Explore.Home,
		IdleWarn:      cfg.Explore.IdleWarn.Get(),
		IdleKill:      cfg.Explore.IdleKill.Get(),
		Budget:        cfg.Explore.Budget.Get(),
		OnIdle: func(silence time.Duration) {
			fmt.Printf("  idle        no output for %s: waiting for the model\n", silence.Round(time.Second))
		},
	})
	if err != nil {
		return err
	}

	r := explore.Record{
		Repository: repoKey,
		CommitSHA:  snap.Repository.CommitSHA,
		Issue:      number,
		Model:      cfg.Explore.Model,
		Metrics: explore.Metrics{
			TotalMs:    out.Total.Milliseconds(),
			FirstMs:    out.FirstEvent.Milliseconds(),
			LongestGap: out.LongestGap.Milliseconds(),
			ToolCalls:  len(out.ToolCalls),
			Stalled:    out.Stalled,
			TimedOut:   out.TimedOut,
			Complete:   out.Complete(),
		},
	}
	fmt.Printf("  complete    %v (exit %d, %d tool calls, %s)\n",
		r.Metrics.Complete, out.ExitCode, r.Metrics.ToolCalls, out.Total.Round(time.Millisecond))
	if !r.Metrics.Complete {
		fmt.Printf("  stalled=%v timedout=%v stderr=%s\n", out.Stalled, out.TimedOut, out.Stderr)
	}

	evidence, parseErr := explore.Parse(out.Answer)
	if parseErr == nil {
		r.Evidence = evidence
		r.Violations = evidence.Verify(explore.Traces(out.ToolCalls))
	}

	verdict, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := decisions.Put(store.Decision{Key: cacheKey, LatencyMs: r.Metrics.TotalMs, Verdict: verdict}); err != nil {
		return err
	}
	if compacted, cerr := decisions.Compact(); cerr != nil {
		fmt.Printf("  warning     journal compaction failed: %v\n", cerr)
	} else if compacted {
		fmt.Printf("  compacted   journal rewritten with %d analyses\n", decisions.Len())
	}

	if parseErr != nil {
		return parseErr
	}
	printEvidence(&r)
	// One fact, one place: the journal is the only copy of this analysis. A second file
	// beside it would drift from the cached verdict the moment either one changed.
	fmt.Printf("  journal     %s\n", st.Path(repoKey+"/decisions.jsonl"))
	return nil
}

// printEvidence reports one analysis. Cached and fresh runs print the same thing, so a
// reused result cannot be mistaken for a weaker one.
func printEvidence(r *explore.Record) {
	ev := r.Evidence
	observed := 0
	for _, f := range ev.Files {
		if f.Relation == explore.RelationObserved {
			observed++
		}
	}
	fmt.Printf("  intent      %s\n", ev.Intent)
	fmt.Printf("  files       %d (%d observed) · %d call paths · %d tests\n",
		len(ev.Files), observed, len(ev.CallPaths), len(ev.Tests))
	fmt.Printf("  requirements knowledge=%d project=%d process=%d\n",
		len(ev.Requirements.Knowledge), len(ev.Requirements.Project), len(ev.Requirements.Process))
	fmt.Printf("  blast       files=%d symbols=%d public_api=%v\n",
		ev.BlastRadius.Files, ev.BlastRadius.Symbols, ev.BlastRadius.PublicAPI)
	fmt.Printf("  doubts      unknowns=%d missing=%d\n", len(ev.Unknowns), len(ev.MissingInformation))
	if len(r.Violations) > 0 {
		fmt.Printf("  VIOLATIONS  %d observed claims without trace support: %v\n", len(r.Violations), r.Violations)
	} else {
		fmt.Printf("  violations  none: every observed claim is backed by the trace\n")
	}
}

// runRepos lists the repositories the store knows about.
func runRepos(args []string) error {
	fs := flag.NewFlagSet("repos", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	st, err := store.New(cfg.Store.Dir)
	if err != nil {
		return err
	}
	repos, err := st.Repos()
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Printf("no repositories yet: run `gfi fetch OWNER/REPO`\n")
		return nil
	}

	fmt.Printf("%-26s %-14s %7s %7s  %s\n", "REPOSITORY", "COMMIT", "OPEN", "STORED", "FETCHED")
	for _, e := range repos {
		analyses := 0
		if d, derr := st.Decisions(e.Key); derr == nil {
			analyses = d.Len()
		}
		fmt.Printf("%-26s %-14s %7d %7d  %s  (%d analysed)\n",
			e.Name, short(e.CommitSHA), e.TotalOpen, e.Issues,
			e.FetchedAt.UTC().Format("2006-01-02 15:04"), analyses)
	}
	return nil
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
