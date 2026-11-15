package projectbrief

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sascodiego/nan-hackathon-clef/internal/repo"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
	"github.com/sascodiego/nan-hackathon-clef/internal/triage"
)

// Source produces the portrait the alignment node reads, out of the clones this application
// already holds. It is the whole point of the artifact: the node is measurable today because the
// benchmark feeds it four hand-written briefs, and this is what produces one for a real
// repository.
//
// It implements triage.BriefSource, which is the seam the node already consumes, so
// AlignmentDossier keeps its signature (S6). What the node gets is the portrait verbatim: the
// unsupported citations are reported to whoever asked, never cut out of the text, because a
// portrait edited before it is read would hide exactly what needs looking at (I10).
type Source struct {
	// Extractor runs or reuses one portrait. Required.
	Extractor *Extractor
	// Root is where the clones live: <Root>/<owner>__<repo>.
	Root string
	// Git resolves the commit the clone is checked out at. Nil uses git from PATH.
	Git *repo.Cloner
}

// Source is a BriefSource, checked here so the seam cannot drift unnoticed.
var _ triage.BriefSource = Source{}

// Portrait is one portrait with everything a caller needs to report it.
func (s Source) Portrait(ctx context.Context, repository string) (*Result, error) {
	switch {
	case s.Extractor == nil:
		return nil, errors.New("projectbrief: a source needs an extractor")
	case strings.TrimSpace(s.Root) == "":
		return nil, errors.New("projectbrief: a source needs the directory the clones live in")
	}
	// The repository may arrive in either shape — the canonical owner/repo GitHub hands out, or
	// the on-disk key store.RepoName produces — and both name the same clone. The key is computed
	// once here so the directory and the portrait's own name cannot disagree.
	key := store.RepoKey(repository)
	dir := filepath.Join(s.Root, key)

	git := s.Git
	if git == nil {
		git = &repo.Cloner{}
	}
	// The commit is read from the clone itself, not from a stored record: the portrait describes
	// the tree that is there now (I14).
	commit, err := git.Head(ctx, dir)
	if err != nil {
		return nil, fmt.Errorf("projectbrief: portrait of %s: %w: clone it first", repository, err)
	}
	return s.Extractor.Extract(ctx, Input{Repository: key, CommitSHA: commit, Dir: dir})
}

// Brief renders the portrait for one repository, which is what the alignment node consumes.
func (s Source) Brief(ctx context.Context, repository string) (string, error) {
	result, err := s.Portrait(ctx, repository)
	if err != nil {
		return "", err
	}
	return result.Document.Text, nil
}
