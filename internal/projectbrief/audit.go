package projectbrief

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Audit checks every citation against the clone the portrait describes, marks the ones the tree
// backs, and returns the ones it does not.
//
// This is the half that makes the artifact evidence instead of prose: a claim is supported when the
// file exists inside the clone and has that line. Unsupported claims are reported and never
// dropped — I10 says doubt is spoken, and a portrait with its wrong claims cut out would look more
// reliable than it is.
//
// Nothing here follows a link out of the clone. The parser already refuses an absolute path and
// one that climbs, but a clone can carry a symlink of its own, and a portrait of a third-party
// repository must never read the host through it (I3).
func (d *Document) Audit(cloneDir string) []Item {
	root, err := filepath.EvalSymlinks(cloneDir)
	if err != nil {
		root = filepath.Clean(cloneDir)
	}
	// One read per file, however many claims cite it.
	lines := map[string]int{}
	var unsupported []Item
	for i := range d.Items {
		item := &d.Items[i]
		item.Sourced = citationExists(root, item.Evidence, lines)
		if !item.Sourced {
			unsupported = append(unsupported, *item)
		}
	}
	return unsupported
}

// citationExists reports whether the cited file is inside the clone and carries that line.
func citationExists(root string, evidence Evidence, lines map[string]int) bool {
	path, ok := resolveInClone(root, evidence.Path)
	if !ok {
		return false
	}
	count, read := lines[path]
	if !read {
		count = countLines(path)
		lines[path] = count
	}
	return count >= evidence.Line
}

// resolveInClone turns a citation into a real path, and refuses anything that leaves the clone.
func resolveInClone(root, cited string) (string, bool) {
	joined := filepath.Join(root, filepath.FromSlash(cited))
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", false
	}
	inside := resolved == root || strings.HasPrefix(resolved, root+string(os.PathSeparator))
	if !inside {
		return "", false
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return resolved, true
}

// countLines is how many lines the file has, or -1 when it cannot be read. A line beyond the end
// of a file is not evidence: the claim points at nothing.
func countLines(path string) int {
	file, err := os.Open(path)
	if err != nil {
		return -1
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	// A generated file with a very long line is still a file: raise the scanner's ceiling instead
	// of reporting the claim as unsupported for it.
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return -1
	}
	return count
}
