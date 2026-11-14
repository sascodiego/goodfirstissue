// Package indexer builds the code graph for a cloned repository.
//
// The graph is a tool for the agent, not a decision engine: this package only prepares
// the index and reports what it contains. Nothing here interprets the code.
package indexer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Stats is what the graph reports about itself after indexing.
type Stats struct {
	Files int
	Nodes int
	Edges int
}

// Indexer prepares the code graph of a clone.
type Indexer struct {
	// Binary overrides the executable. Empty means "codegraph".
	Binary string
	// Run replaces the process call. Nil means exec the binary.
	Run func(ctx context.Context, dir string, args ...string) ([]byte, error)
}

// Prepare indexes repoPath and pins nothing: the caller already holds the commit SHA.
//
// codegraph writes its index inside the clone, which would show up as an untracked
// directory in a repository that is not ours. The exclusion goes in .git/info/exclude,
// never in a .gitignore we would be adding to someone else's project.
func (in *Indexer) Prepare(ctx context.Context, repoPath string) (Stats, error) {
	if repoPath == "" {
		return Stats{}, errors.New("indexer: repoPath is required")
	}
	if err := excludeFromGit(repoPath); err != nil {
		return Stats{}, err
	}

	initOut, err := in.run(ctx, repoPath, "init", "-y", repoPath)
	if err != nil {
		return Stats{}, err
	}

	// Status is the authority on what the index covers. init prints its counts only when
	// it actually builds one: re-running it on an existing index prints neither.
	if statusOut, statusErr := in.run(ctx, repoPath, "status", repoPath); statusErr == nil {
		if fromStatus, ok := parseStatus(string(statusOut)); ok {
			return fromStatus, nil
		}
	}
	if stats, err := parseInitOutput(string(initOut)); err == nil {
		return stats, nil
	}
	return Stats{}, errors.New("indexer: neither status nor init reported the index statistics")
}

var (
	initCounts  = regexp.MustCompile(`([\d,]+)\s+nodes,\s+([\d,]+)\s+edges`)
	statusFiles = regexp.MustCompile(`Files:\s+([\d,]+)`)
	statusNodes = regexp.MustCompile(`Nodes:\s+([\d,]+)`)
	statusEdges = regexp.MustCompile(`Edges:\s+([\d,]+)`)
)

func parseInitOutput(s string) (Stats, error) {
	m := initCounts.FindStringSubmatch(s)
	if m == nil {
		return Stats{}, fmt.Errorf("indexer: unexpected init output: %s", firstLine(s))
	}
	return Stats{Nodes: atoi(m[1]), Edges: atoi(m[2])}, nil
}

func parseStatus(s string) (Stats, bool) {
	files, nodes, edges := statusFiles.FindStringSubmatch(s), statusNodes.FindStringSubmatch(s), statusEdges.FindStringSubmatch(s)
	if files == nil || nodes == nil || edges == nil {
		return Stats{}, false
	}
	return Stats{Files: atoi(files[1]), Nodes: atoi(nodes[1]), Edges: atoi(edges[1])}, true
}

func (in *Indexer) run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if in.Run != nil {
		return in.Run(ctx, dir, args...)
	}
	bin := in.Binary
	if bin == "" {
		bin = "codegraph"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("indexer: %s %s: %w: %s", bin, strings.Join(args, " "), err, firstLine(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// excludeFromGit keeps .codegraph/ out of the clone's status without touching any
// tracked file. It is idempotent.
func excludeFromGit(repoPath string) error {
	gitDir := filepath.Join(repoPath, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		// A worktree has .git as a file pointing elsewhere; either way, nothing to do.
		return nil
	}
	infoDir := filepath.Join(gitDir, "info")
	if err := os.MkdirAll(infoDir, 0o755); err != nil {
		return fmt.Errorf("indexer: create %s: %w", infoDir, err)
	}
	excludePath := filepath.Join(infoDir, "exclude")
	const entry = ".codegraph/"

	existing, err := os.ReadFile(excludePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("indexer: read %s: %w", excludePath, err)
	}
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}
	next := existing
	if len(next) > 0 && next[len(next)-1] != '\n' {
		next = append(next, '\n')
	}
	next = append(next, entry+"\n"...)
	if err := os.WriteFile(excludePath, next, 0o644); err != nil {
		return fmt.Errorf("indexer: write %s: %w", excludePath, err)
	}
	return nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.ReplaceAll(s, ",", ""))
	return n
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
