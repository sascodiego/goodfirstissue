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
	count, checked := lines[evidence.Path]
	if !checked {
		count = countLinesInClone(root, evidence.Path)
		lines[evidence.Path] = count
	}
	return count >= evidence.Line
}

// countLinesInClone opens the cited file, proves it is the one that was validated, and counts its
// lines. Anything it cannot prove is -1, which no citation matches.
//
// Resolving a path and then opening it would be two syscalls with a window between them: a link
// swapped in that window would be read from wherever it now points. So the file is opened and its
// descriptor compared with the identity of the path that was checked — os.SameFile — and a path
// that changed under us fails closed instead of being followed (I3).
func countLinesInClone(root, cited string) int {
	resolved, validated, ok := resolveInClone(root, cited)
	if !ok {
		return -1
	}
	file, err := os.Open(resolved)
	if err != nil {
		return -1
	}
	defer func() { _ = file.Close() }()

	opened, err := file.Stat()
	if err != nil || !os.SameFile(validated, opened) {
		return -1
	}

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

// resolveInClone turns a citation into a real path inside the clone, together with the identity of
// the file it found, so the caller can prove that what it opens is what was checked.
func resolveInClone(root, cited string) (string, os.FileInfo, bool) {
	joined := filepath.Join(root, filepath.FromSlash(cited))
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", nil, false
	}
	inside := resolved == root || strings.HasPrefix(resolved, root+string(os.PathSeparator))
	if !inside {
		return "", nil, false
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", nil, false
	}
	return resolved, info, true
}
