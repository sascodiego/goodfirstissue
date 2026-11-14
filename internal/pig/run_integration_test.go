package pig

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRunAgainstClone exercises the real agent through this package.
//
// It needs the pig binary, a NAN_KEY and an indexed clone, so it is opt-in:
//
//	GFI_INTEGRATION=1 NAN_KEY=... go test ./internal/pig/ -run Integration -v
//
// The repository under test is var/repos/cobra, cloned and indexed at setup time.
func TestIntegrationAgainstClone(t *testing.T) {
	if os.Getenv("GFI_INTEGRATION") == "" {
		t.Skip("set GFI_INTEGRATION=1 to run against the real agent")
	}
	if _, err := exec.LookPath("pig"); err != nil {
		t.Skipf("pig not installed: %v", err)
	}
	key := os.Getenv("NAN_KEY")
	if key == "" {
		t.Skip("NAN_KEY not set")
	}
	repo, err := filepath.Abs("../../var/repos/cobra")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(repo); err != nil {
		t.Skipf("clone not present at %s: %v", repo, err)
	}

	home := t.TempDir()
	if err := WriteConfig(home, DefaultConfig([]string{"deepseek-v4-flash"})); err != nil {
		t.Fatalf("WriteConfig() error = %v", err)
	}

	out, err := Run(t.Context(), Request{
		Dir:           repo,
		Prompt:        "Which file defines the AddCommand method? Answer with the relative path and the line number.",
		Model:         "deepseek-v4-flash",
		Tools:         DefaultTools(),
		SystemPrompts: []string{"Be brief. Answer in one line."},
		Home:          home,
		Budget:        120 * time.Second,
		IdleWarn:      15 * time.Second,
		IdleKill:      60 * time.Second,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	t.Logf("exit=%d complete=%v first=%v gap=%v total=%v tools=%v",
		out.ExitCode, out.Complete(), out.FirstEvent.Round(time.Millisecond),
		out.LongestGap.Round(time.Millisecond), out.Total.Round(time.Millisecond), usedNames(out))

	if !out.Complete() {
		t.Fatalf("run did not complete: stalled=%v timedOut=%v exit=%d stderr=%s",
			out.Stalled, out.TimedOut, out.ExitCode, truncate(out.Stderr, 400))
	}
	if len(out.ToolCalls) == 0 {
		t.Fatal("the agent used no tools")
	}
	if strings.TrimSpace(out.Answer) == "" {
		t.Error("the agent produced no answer")
	}
	if out.FirstEvent <= 0 {
		t.Error("FirstEvent was not recorded")
	}
	// The clone must be untouched: the allowlist has no write surface.
	assertCleanTree(t, repo)
}

// TestStallWatchdogKillsSilentChild proves the watchdog against a child that never
// speaks, without depending on the agent: a script that ignores its arguments and blocks.
func TestStallWatchdogKillsSilentChild(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh available")
	}
	home := t.TempDir()
	fake := filepath.Join(home, "silent-agent")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexec sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var warned bool

	start := time.Now()
	out, err := Run(t.Context(), Request{
		Dir:      t.TempDir(),
		Prompt:   "unused",
		Model:    "gemma4",
		Home:     home,
		Binary:   fake,
		Budget:   30 * time.Second,
		IdleWarn: 300 * time.Millisecond,
		IdleKill: 600 * time.Millisecond,
		OnIdle:   func(time.Duration) { warned = true },
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !out.Stalled {
		t.Fatalf("out.Stalled = false, want true (exit=%d)", out.ExitCode)
	}
	if !warned {
		t.Error("OnIdle never fired before the kill")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("the watchdog took %v, it should stop near the kill threshold", elapsed)
	}
	if out.Complete() {
		t.Error("a killed run must not report Complete")
	}
}

func usedNames(out *Outcome) []string {
	names := make([]string, 0, len(out.ToolCalls))
	for _, tc := range out.ToolCalls {
		names = append(names, tc.Name)
	}
	return names
}

func assertCleanTree(t *testing.T, repo string) {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "status", "--porcelain")
	b, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" || strings.Contains(line, ".codegraph") {
			continue
		}
		t.Errorf("the clone was modified: %s", line)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
