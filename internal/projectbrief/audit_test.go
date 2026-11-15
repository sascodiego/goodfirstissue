package projectbrief_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/projectbrief"
)

// S2: sin evidencia no es retrato. Every claim is checked against the clone it names, and the ones
// the tree does not back are reported beside the document instead of being cut out.
func TestExtractAuditsEveryCitationAgainstTheClone(t *testing.T) {
	answer := validAnswer
	// One claim cites a file that is not there, and one cites a line past the end of a file that
	// is. Everything else is intact.
	answer = replaceOnce(t, answer, "docs/history.md:12", "docs/missing.md:1")
	answer = replaceOnce(t, answer, "Makefile:12", "Makefile:999")

	a := &agent{out: nil}
	a.out = outcomeOf(answer)
	e := newExtractor(t, a)

	res, err := e.Extract(context.Background(), projectbrief.Input{
		Repository: "spf13__cobra", CommitSHA: testCommit, Dir: clone(t),
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(res.Unsupported) != 2 {
		t.Fatalf("unsupported = %+v, want the two fabricated citations", res.Unsupported)
	}
	for _, item := range res.Unsupported {
		if item.Evidence.Path != "docs/missing.md" && !(item.Evidence.Path == "Makefile" && item.Evidence.Line == 999) {
			t.Errorf("unexpected unsupported claim %+v", item)
		}
	}
	// The claims stay in the text: what the reader must know is that the portrait is not fully
	// backed, not a portrait with its holes hidden.
	if res.Document.Text != answer {
		t.Error("the audit reports, it does not edit")
	}
	if len(res.Document.Items) != 7 {
		t.Errorf("items = %d, want 7", len(res.Document.Items))
	}
}

func TestDocumentAuditMarksEverySupportedClaim(t *testing.T) {
	doc, err := projectbrief.Parse(validAnswer)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	unsupported := doc.Audit(clone(t))
	if len(unsupported) != 0 {
		t.Errorf("unsupported = %+v, want none in an intact clone", unsupported)
	}
	for _, item := range doc.Items {
		if !item.Sourced {
			t.Errorf("claim %+v is not marked as backed by the clone", item)
		}
	}
}

// A clone can carry a link out of itself. A portrait of a third-party repository must never follow
// it: the citation is refused even though the file it points at exists (I3).
func TestDocumentAuditRefusesALinkOutOfTheClone(t *testing.T) {
	dir := clone(t)
	outside := filepath.Join(t.TempDir(), "secreto.txt")
	if err := os.WriteFile(outside, []byte("contenido del host\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.txt")); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}

	doc, err := projectbrief.Parse(replaceOnce(t, validAnswer, "README.md:3", "link.txt:1"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	unsupported := doc.Audit(dir)
	if len(unsupported) != 1 || unsupported[0].Evidence.Path != "link.txt" {
		t.Fatalf("unsupported = %+v, want the link refused", unsupported)
	}
	if unsupported[0].Sourced {
		t.Error("a file outside the clone is not evidence about the project")
	}
}

func TestDocumentAuditDoesNotCountADirectoryAsEvidence(t *testing.T) {
	doc, err := projectbrief.Parse(replaceOnce(t, validAnswer, "README.md:3", "docs:1"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if unsupported := doc.Audit(clone(t)); len(unsupported) != 1 {
		t.Errorf("unsupported = %+v, want the directory refused", unsupported)
	}
}

// A line beyond the end of the file points at nothing, and a claim that points at nothing is not
// evidence even when the file exists.
func TestDocumentAuditCountsTheLastLineAsEvidence(t *testing.T) {
	dir := t.TempDir()
	fillClone(t, dir)
	doc, err := projectbrief.Parse(replaceOnce(t, validAnswer, "docs/history.md:12", "docs/history.md:20"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if unsupported := doc.Audit(dir); len(unsupported) != 0 {
		t.Errorf("the last line of a file is evidence, got %+v", unsupported)
	}

	doc, err = projectbrief.Parse(replaceOnce(t, validAnswer, "docs/history.md:12", "docs/history.md:21"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if unsupported := doc.Audit(dir); len(unsupported) != 1 {
		t.Errorf("past the last line is not evidence, got %+v", unsupported)
	}
}
