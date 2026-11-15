package triage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sascodiego/nan-hackathon-clef/internal/repo"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// BriefSource supplies the project's own direction for the alignment node (§7.5). It is an
// interface because the benchmark must feed the frozen briefs it measured with, and because a
// clone that is not there is a condition to report, never a crash.
type BriefSource interface {
	Brief(ctx context.Context, repository string) (string, error)
}

// The brief has two halves, and the second is the one that earns its keep: what a project says
// it is can be aspirational, while what it has been merging for the last months is what it is
// actually becoming. Comparing the two catches the sharpest case — the feature the project has
// been dismantling.
const (
	// maxBriefReadmeChars bounds the declared half. The dossier has its own ceiling and a
	// README that eats all of it would leave no room for the request.
	maxBriefReadmeChars = 2000
	// briefCommits is how much history the revealed half reads.
	briefCommits = 20

	briefTruncated = " [truncated]"
)

// readmeNames is the order in which a project's self-description is looked for. A repository
// without any of them still gets the revealed half, which is the half that cannot be missing.
var readmeNames = []string{"README.md", "README.rst", "README.markdown", "README.txt", "README"}

// CloneBrief builds the brief from a local clone: the declared direction from its README and the
// revealed one from its recent history. Both are already on disk, so it costs no request.
type CloneBrief struct {
	// Root is where the clones live: <Root>/<owner>__<repo>.
	Root string
	// Git inspects the clone. Nil uses git from PATH.
	Git *repo.Cloner
}

// Brief renders the project's direction for one repository.
//
// The repository may arrive in either shape — the canonical owner/repo GitHub hands out, or the
// on-disk key store.RepoName produces — and both name the same clone. The key is computed once
// here so the directory and the header cannot disagree.
//
// A missing README is not an error: the history still speaks, and a project whose history cannot
// be read either is the only case that reports one.
func (c CloneBrief) Brief(ctx context.Context, repository string) (string, error) {
	if strings.TrimSpace(c.Root) == "" {
		return "", errors.New("triage: the brief needs the directory the clones live in")
	}
	key := store.RepoKey(repository)
	dir := filepath.Join(c.Root, key)

	git := c.Git
	if git == nil {
		git = &repo.Cloner{}
	}
	subjects, err := git.RecentSubjects(ctx, dir, briefCommits)
	if err != nil {
		return "", fmt.Errorf("triage: brief of %s: %w", repository, err)
	}

	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(key)
	b.WriteString("\n\n## What the project says it is (README)\n")
	b.WriteString(readReadme(dir))
	b.WriteString("\n\n## What it has been building lately (last ")
	fmt.Fprint(&b, briefCommits)
	b.WriteString(" commit subjects)\n")
	for _, subject := range subjects {
		b.WriteString(subject)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// readReadme returns the project's own description, cut and marked. An absent one reports so
// explicitly: an empty section reads as "we forgot", which is worse than "there was none".
func readReadme(dir string) string {
	for _, name := range readmeNames {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		text := strings.TrimSpace(string(body))
		if text == "" {
			continue
		}
		if len(text) > maxBriefReadmeChars {
			return clipBytes(text, maxBriefReadmeChars-len(briefTruncated)) + briefTruncated
		}
		return text
	}
	return noneMarker
}
