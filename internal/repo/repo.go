// Package repo manages the local clones the corpus is built from.
//
// Cloning is the only place this application writes outside its own directories, and it
// only ever writes into the configured clone directory.
package repo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Cloner creates and inspects clones.
type Cloner struct {
	// Binary overrides git. Empty means "git" from PATH.
	Binary string
	// Run replaces the process call. Nil means exec git.
	Run func(ctx context.Context, dir string, args ...string) ([]byte, error)
}

// Ensure makes dir a clone of url, shallow, and returns the checked-out commit.
//
// It is idempotent: an existing clone is inspected, never re-cloned. The returned SHA is
// what every later result is pinned to (I4).
func (c *Cloner) Ensure(ctx context.Context, url, dir string) (sha string, cloned bool, err error) {
	if strings.TrimSpace(url) == "" {
		return "", false, errors.New("repo: url is required")
	}
	if strings.TrimSpace(dir) == "" {
		return "", false, errors.New("repo: dir is required")
	}

	if IsRepo(dir) {
		sha, err := c.Head(ctx, dir)
		return sha, false, err
	}

	if _, err := c.run(ctx, "", "clone", "--depth", "1", "--quiet", url, dir); err != nil {
		return "", false, err
	}
	sha, err = c.Head(ctx, dir)
	return sha, true, err
}

// Head returns the commit currently checked out in dir.
func (c *Cloner) Head(ctx context.Context, dir string) (string, error) {
	out, err := c.run(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return "", fmt.Errorf("repo: git rev-parse returned nothing in %s", dir)
	}
	return sha, nil
}

// IsRepo reports whether dir already holds a Git repository. A worktree records .git as
// a file rather than a directory, so both forms count.
func IsRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func (c *Cloner) run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if c.Run != nil {
		return c.Run(ctx, dir, args...)
	}
	bin := c.Binary
	if bin == "" {
		bin = "git"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("repo: git %s: %w: %s", strings.Join(args, " "), err, firstLine(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
