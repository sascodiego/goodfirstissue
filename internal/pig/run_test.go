package pig

import (
	"strings"
	"testing"
)

func argValue(args []string, flag string) (string, bool) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1], true
		}
	}
	return "", false
}

func TestBuildArgsUsesNaNAndNativeMCP(t *testing.T) {
	req := Request{Model: "glm5.3-flash", Tools: DefaultTools(), Prompt: "explore"}
	args := buildArgs(req)

	if got, _ := argValue(args, "--provider"); got != "nan" {
		t.Errorf("--provider = %q, want nan", got)
	}
	if got, _ := argValue(args, "--model"); got != "nan/glm5.3-flash" {
		t.Errorf("--model = %q, want the provider-qualified spec", got)
	}
	if got, _ := argValue(args, "--mode"); got != "json" {
		t.Errorf("--mode = %q, want json", got)
	}
	// The trap: --no-extensions also kills the builtin MCP extension.
	if got, _ := argValue(args, "-e"); got != "builtin:mcp" {
		t.Errorf("-e = %q, want builtin:mcp", got)
	}
	for _, want := range []string{"-p", "--no-session", "--no-skills", "-nc", "-np", "--no-approve"} {
		if !contains(args, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// A job that only moves text has nothing to read, nothing to run and no repository to
// graph: an empty allowlist must mean exactly that, not an empty --tools value.
func TestBuildArgsWithNoToolsDisablesToolsAndMCP(t *testing.T) {
	args := buildArgs(Request{Model: "glm5.3-flash", Prompt: "extract"})

	if !contains(args, "-nt") && !contains(args, "--no-tools") {
		t.Error("an empty tool list must disable every tool")
	}
	if !contains(args, "--no-mcp") {
		t.Error("an empty tool list must not connect MCP servers")
	}
	for _, forbidden := range []string{"--tools", "-e", "builtin:mcp"} {
		if contains(args, forbidden) {
			t.Errorf("arg %q present with an empty tool list", forbidden)
		}
	}
	// The prompt still goes last, after the separator.
	if args[len(args)-1] != "extract" || args[len(args)-2] != "--" {
		t.Errorf("prompt not after the separator: %v", args[len(args)-3:])
	}
}

// The tool allowlist must name the MCP tool, or the first turn never waits for it.
func TestDefaultToolsAreReadOnlyAndIncludeTheGraph(t *testing.T) {
	tools := strings.Join(DefaultTools(), ",")
	for _, forbidden := range []string{"bash", "edit", "write"} {
		if strings.Contains(tools, forbidden) {
			t.Errorf("DefaultTools exposes %q", forbidden)
		}
	}
	if !strings.Contains(tools, "mcp__codegraph__codegraph_explore") {
		t.Error("DefaultTools does not name the code graph tool")
	}
}

// An issue whose body starts with a dash must survive as a prompt, not as a flag.
func TestBuildArgsSeparatesPromptWithDoubleDash(t *testing.T) {
	req := Request{Model: "gemma4", Tools: DefaultTools(), Prompt: "--pretty=all does not work"}
	args := buildArgs(req)

	sep := -1
	for i, a := range args {
		if a == "--" {
			sep = i
			break
		}
	}
	if sep == -1 {
		t.Fatal(`no "--" separator`)
	}
	if sep != len(args)-2 {
		t.Fatalf(`"--" at %d of %d, want immediately before the prompt`, sep, len(args))
	}
	if args[len(args)-1] != req.Prompt {
		t.Errorf("last arg = %q, want the prompt", args[len(args)-1])
	}
}

// A foreign PIG_HOME must be replaced, not duplicated: Go leaves duplicate keys ambiguous.
func TestEnvWithReplacesInheritedHome(t *testing.T) {
	base := []string{"PATH=/bin", "PIG_HOME=/home/someone/.pig", "NAN_KEY=secret"}
	out := envWith(base, "PIG_HOME=/app/var/pig")

	var homes int
	for _, e := range out {
		if strings.HasPrefix(e, "PIG_HOME=") {
			homes++
			if e != "PIG_HOME=/app/var/pig" {
				t.Errorf("PIG_HOME = %q, want the application's own", e)
			}
		}
	}
	if homes != 1 {
		t.Errorf("found %d PIG_HOME entries, want exactly 1", homes)
	}
	if !contains(out, "NAN_KEY=secret") {
		t.Error("unrelated variables must be preserved")
	}
}

func TestRunValidatesRequiredFields(t *testing.T) {
	for name, req := range map[string]Request{
		"missing dir":   {Model: "gemma4", Home: "/tmp"},
		"missing model": {Dir: "/tmp", Home: "/tmp"},
		"missing home":  {Dir: "/tmp", Model: "gemma4"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Run(t.Context(), req); err == nil {
				t.Fatal("want an error, got nil")
			}
		})
	}
}

func TestOutcomeComplete(t *testing.T) {
	cases := []struct {
		out  Outcome
		want bool
	}{
		{Outcome{}, true},
		{Outcome{Stalled: true}, false},
		{Outcome{TimedOut: true}, false},
		{Outcome{ExitCode: 1}, false},
	}
	for _, c := range cases {
		if got := c.out.Complete(); got != c.want {
			t.Errorf("%+v.Complete() = %v, want %v", c.out, got, c.want)
		}
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// Una corrida cuyo turno terminó en error no completó: el código de salida es 0 porque el
// proceso salió bien, y aun así no hay respuesta que usar.
func TestOutcomeIsIncompleteWhenTheTurnEndedInError(t *testing.T) {
	out := &Outcome{Answer: "", Error: "401: Invalid API key."}
	if out.Complete() {
		t.Error("Complete() = true for a turn that ended in error")
	}
	// Un resultado vacío sin error sigue siendo completo: "complete" dice si la corrida
	// terminó en sus límites, no si trajo respuesta. Quien necesita la respuesta la pide.
	if !(&Outcome{}).Complete() {
		t.Error("Complete() = false for an outcome that simply carried nothing")
	}
}
