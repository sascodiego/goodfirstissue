package explore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/pig"
)

const validAnswer = "Aquí va el resultado:\n" + `{
  "intent": "hidden commands should not widen the usage column",
  "files": [
    {"path": "command.go", "symbol": "Command.AddCommand", "line": 1342, "reason": "why", "relation": "observed"},
    {"path": "cobra.go", "symbol": "rpad", "line": 174, "reason": "consumer", "relation": "probable"}
  ],
  "call_paths": [["AddCommand", "Commands", "NamePadding"]],
  "tests": ["command_test.go:TestHidden"],
  "requirements": {"knowledge": ["go templates"], "project": ["usage template"], "process": ["go test ./..."]},
  "blast_radius": {"files": 2, "symbols": 8, "public_api": true},
  "unknowns": ["no se vio el recálculo"],
  "missing_information": ["criterio de aceptación"]
}`

func TestParseAcceptsTheContract(t *testing.T) {
	ev, err := Parse(validAnswer)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(ev.Files) != 2 || ev.Files[0].Relation != RelationObserved {
		t.Errorf("files = %+v", ev.Files)
	}
	if len(ev.CallPaths) != 1 || len(ev.CallPaths[0]) != 3 {
		t.Errorf("call paths = %+v", ev.CallPaths)
	}
	if ev.Requirements.Process[0] != "go test ./..." {
		t.Errorf("requirements = %+v", ev.Requirements)
	}
}

func TestParseRejectsProseWithoutJSON(t *testing.T) {
	if _, err := Parse("no encontré nada útil"); err == nil {
		t.Fatal("want an error when the answer has no object")
	}
}

// A field the contract does not define means the shape drifted; better to notice.
func TestParseRejectsUnknownField(t *testing.T) {
	body := strings.Replace(validAnswer, `"files": [`, `"difficulty": "easy", "files": [`, 1)
	_, err := Parse(body)
	if err == nil || !strings.Contains(err.Error(), "difficulty") {
		t.Fatalf("err = %v, want it to name the stray field", err)
	}
}

func TestValidateRejectsEscapingAndAbsolutePaths(t *testing.T) {
	for _, path := range []string{"../secrets.go", "/etc/passwd"} {
		ev := &Evidence{Intent: "x", Files: []File{{Path: path, Relation: RelationObserved}}}
		if err := ev.Validate(); err == nil {
			t.Errorf("path %q was accepted", path)
		}
	}
}

func TestValidateRejectsUnknownRelation(t *testing.T) {
	ev := &Evidence{Intent: "x", Files: []File{{Path: "a.go", Relation: "certain"}}}
	if err := ev.Validate(); err == nil {
		t.Fatal("an invented relation was accepted")
	}
}

func TestValidateRejectsEmptyFiles(t *testing.T) {
	ev := &Evidence{Intent: "x"}
	if err := ev.Validate(); err == nil {
		t.Fatal("an evidence package with no files was accepted")
	}
}

// An observed claim must be backed by something a tool returned.
func TestVerifyFlagsUnbackedObservedClaims(t *testing.T) {
	ev := &Evidence{
		Intent: "x",
		Files: []File{
			{Path: "command.go", Relation: RelationObserved},
			{Path: "invented.go", Relation: RelationObserved},
			{Path: "guessed.go", Relation: RelationProbable},
		},
	}
	traces := []string{
		`{"args":{"path":"command.go","offset":1590}}`,
		"command.go:1342: func (c *Command) AddCommand(",
	}

	got := ev.Verify(traces)
	if len(got) != 1 || got[0].Path != "invented.go" {
		t.Fatalf("violations = %+v, want only invented.go", got)
	}
}

func TestVerifyIgnoresNonObservedClaims(t *testing.T) {
	ev := &Evidence{Intent: "x", Files: []File{{Path: "nowhere.go", Relation: RelationUnverified}}}
	if got := ev.Verify(nil); len(got) != 0 {
		t.Fatalf("violations = %+v, want none", got)
	}
}

func TestPrepareWritesTheSkill(t *testing.T) {
	home := t.TempDir()
	path, err := Prepare(home)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	if !strings.Contains(string(b), "read-only-code-explorer") {
		t.Errorf("skill body does not look like the skill:\n%s", b)
	}
	if filepath.Dir(path) != filepath.Join(home, "skills", "explore") {
		t.Errorf("skill written to %s, want it inside the agent home", path)
	}
}

func TestPromptKeepsLabelsVerbatim(t *testing.T) {
	got := PromptFor("spf13/cobra", "abc123", 1272, "Hidden Commands", "body text",
		[]string{"good-first-issue", "\u2709 help wanted"})
	for _, want := range []string{"spf13/cobra", "abc123", "#1272", "Hidden Commands", "good-first-issue", "✉ help wanted", "body text"} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q:\n%s", want, got)
		}
	}
}

func TestPromptSaysNoneWhenUnlabelled(t *testing.T) {
	if !strings.Contains(PromptFor("a/b", "sha", 1, "t", "b", nil), "(none)") {
		t.Error("an unlabelled issue must say so explicitly")
	}
}

// The embedded contract must still be the one the prompt file holds.
func TestEmbeddedPromptIsNotEmpty(t *testing.T) {
	if len(Prompt) < 500 {
		t.Errorf("prompt.md is only %d bytes: the embed is probably wrong", len(Prompt))
	}
	if !strings.Contains(Prompt, "evidence") {
		t.Error("prompt.md does not look like the explore contract")
	}
}

func TestTraceHelperMatchesPigToolCalls(t *testing.T) {
	calls := []pig.ToolCall{{Name: "read", Args: []byte(`{"path":"command.go"}`)}}
	traces := Traces(calls)
	if len(traces) != 1 || !strings.Contains(traces[0], "command.go") {
		t.Fatalf("traces = %+v", traces)
	}
}
