package pig

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
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

// S10: idle is measured as silence of the pipe, and the caller can see it live. The
// one-shot warning is not enough for a screen that has to say "waiting for the model, 12s".
func TestRunReportsTheCurrentSilenceOnEveryTick(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake agent is a shell script")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-pig")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nsleep 1\n"), 0o755); err != nil {
		t.Fatalf("fake agent: %v", err)
	}

	var mu sync.Mutex
	var seen []time.Duration
	out, err := Run(context.Background(), Request{
		Binary:   bin,
		Dir:      dir,
		Model:    "deepseek-v4-flash",
		Home:     dir,
		Prompt:   "explore",
		IdleWarn: 100 * time.Millisecond,
		IdleKill: 10 * time.Second,
		Budget:   20 * time.Second,
		OnProgress: func(silence time.Duration) {
			mu.Lock()
			seen = append(seen, silence)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out.Stalled {
		t.Fatal("a child that finishes within the limits was reported as stalled")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seen) < 2 {
		t.Fatalf("OnProgress fired %d times for a silent child", len(seen))
	}
	if seen[len(seen)-1] <= seen[0] {
		t.Fatalf("the reported silence did not grow: %v", seen)
	}
}
