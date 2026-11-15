package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/config"
	"github.com/sascodiego/nan-hackathon-clef/internal/pig"
	"github.com/sascodiego/nan-hackathon-clef/internal/projectbrief"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// prepareBriefHome refuses a run that could only fail, and gives the agent what it needs to
// authenticate: without NAN_KEY there is no key, and without the generated configuration PiG
// looks for a provider that is not there. The failure is reported by name, before a minute of
// retries, because it is the one thing the person running this can fix.
func prepareBriefHome(cfg *config.Config) error {
	if strings.TrimSpace(os.Getenv("NAN_KEY")) == "" {
		return errors.New("brief needs NAN_KEY in the environment: the model authenticates with it (see env.example)")
	}
	if err := pig.WriteConfig(cfg.ProjectBrief.Home, pig.DefaultConfig(config.AllowedChatModels)); err != nil {
		return fmt.Errorf("brief: the agent's configuration cannot be written: %w", err)
	}
	return nil
}

// runBrief describes one cloned repository as a project portrait: the input the alignment node
// already consumes and nothing produced for a real repository.
//
// The document is what the caller asked for, so it goes to stdout alone, verbatim, and every fact
// about it goes to stderr: `gfi brief owner/repo > brief.md` writes exactly the portrait. One run
// per repository (S7), pinned to the commit: a tree at another commit is another question (I14),
// and -force is how a person asks again when the tree moved without a new commit.
func runBrief(args []string) error {
	fs := flag.NewFlagSet("brief", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config.json")
	force := fs.Bool("force", false, "ignore a stored portrait and describe the tree again")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("brief needs OWNER/REPO\n\n%s", usage)
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

	// GitHub is the authority on a repository's name and the store remembers the rename: the
	// portrait of httpie/httpie is the portrait of httpie/cli, under the key the clone uses.
	requested := owner + "/" + name
	if canonical, ok := st.Alias(requested); ok {
		if co, cn, splitErr := splitTarget(canonical); splitErr == nil {
			owner, name = co, cn
			fmt.Fprintf(os.Stderr, "  note        %s is GitHub's name for %s\n", canonical, requested)
		}
	}
	repoKey := store.RepoName(owner, name)

	// The agent's configuration is generated in this job's own home, never inherited from a user's
	// ~/.pig. Doing it here and not inside the extractor keeps the package free of the decision of
	// when a home is written; the command is the only production caller today.
	if err := prepareBriefHome(cfg); err != nil {
		return err
	}

	// The model is read from config.models.project_brief and was validated against the
	// constitutional allowlist when the configuration loaded (I13).
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
		return err
	}

	src := projectbrief.Source{
		Extractor: extractor,
		Root:      cfg.Repos.CloneDir,
		Force:     *force,
	}

	fmt.Fprintf(os.Stderr, "describing %s with %s\n", repoKey, cfg.Models.ProjectBrief)
	res, err := src.Portrait(context.Background(), repoKey)
	if err != nil {
		// An answer that followed no contract is shown: the run already cost minutes, and the
		// document is what tells the next attempt what went wrong.
		var failure *projectbrief.Failure
		if errors.As(err, &failure) && failure.Answer != "" {
			fmt.Fprintf(os.Stderr, "\nthe model answered something that is not a portrait:\n---\n%s\n---\n", failure.Answer)
		}
		return err
	}

	// The portrait verbatim, with nothing added around it: it is the document a consumer reads.
	// A failed write is reported: a portrait that reached half a pipe is not a portrait.
	if _, err := fmt.Fprint(os.Stdout, res.Document.Text); err != nil {
		return fmt.Errorf("brief: writing the portrait: %w", err)
	}

	origin := "fresh"
	if res.Cached {
		origin = "cached"
	}
	fmt.Fprintf(os.Stderr, "  commit      %s\n", short(res.Commit))
	fmt.Fprintf(os.Stderr, "  model       %s\n", res.Model)
	fmt.Fprintf(os.Stderr, "  prompt      %s\n", res.PromptVersion)
	fmt.Fprintf(os.Stderr, "  source      %s\n", origin)
	fmt.Fprintf(os.Stderr, "  took        %s\n", res.Took.Round(time.Millisecond))
	// The citations the clone does not back stay in the document and are reported here: a
	// portrait with its wrong claims cut out would look more reliable than it is (I10).
	fmt.Fprintf(os.Stderr, "  claims      %d unsupported of %d\n",
		len(res.Unsupported), len(res.Document.Items))
	for _, item := range res.Unsupported {
		fmt.Fprintf(os.Stderr, "  unsupported %s — %s\n", item.Evidence, item.Claim)
	}
	if res.Reason != "" {
		fmt.Fprintf(os.Stderr, "  doubt       %s\n", res.Reason)
	}
	return nil
}
