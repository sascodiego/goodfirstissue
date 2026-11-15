package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/config"
)

// The command has to refuse a target it cannot name before it loads anything: a missing or an
// extra argument must never reach the agent, and the message has to say how the command is
// called. Nothing here starts a process, which is why the happy path is not tested: it would run
// a real agent over a repository.
func TestRunBriefNeedsExactlyOneTarget(t *testing.T) {
	cases := map[string][]string{
		"no argument":   {},
		"two arguments": {"spf13/cobra", "httpie/cli"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			err := runBrief(args)
			if err == nil {
				t.Fatalf("runBrief(%v) = nil, want an error naming the argument", args)
			}
			if !strings.Contains(err.Error(), "brief needs OWNER/REPO") {
				t.Errorf("err = %q, want it to start with the usage of the command", err)
			}
		})
	}
}

// The run has to be prepared before the agent starts: without a key there is nothing to
// authenticate with, and without the generated configuration PiG looks for a provider that is not
// there. Both failures are named before a minute of retries.
func TestPrepareBriefHomeWritesTheAgentConfiguration(t *testing.T) {
	t.Setenv("NAN_KEY", "test-key")
	cfg := config.Default()
	cfg.ProjectBrief.Home = t.TempDir()

	if err := prepareBriefHome(cfg); err != nil {
		t.Fatalf("prepareBriefHome: %v", err)
	}
	// The models file is the one that decides which models the agent may run: it is generated
	// from the constitutional allowlist, never from the environment.
	body, err := os.ReadFile(filepath.Join(cfg.ProjectBrief.Home, "agent", "models.json"))
	if err != nil {
		t.Fatalf("models.json: %v", err)
	}
	for _, want := range append([]string{"nan", "NAN_KEY"}, config.AllowedChatModels...) {
		if !strings.Contains(string(body), want) {
			t.Errorf("models.json does not carry %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(cfg.ProjectBrief.Home, "agent", "mcp.json")); err != nil {
		t.Errorf("mcp.json: %v", err)
	}
}

func TestPrepareBriefHomeNeedsTheKey(t *testing.T) {
	t.Setenv("NAN_KEY", "")
	cfg := config.Default()
	cfg.ProjectBrief.Home = t.TempDir()

	err := prepareBriefHome(cfg)
	if err == nil {
		t.Fatal("a run without NAN_KEY was prepared")
	}
	if !strings.Contains(err.Error(), "NAN_KEY") {
		t.Errorf("err = %v, want it to name what is missing", err)
	}
	if _, statErr := os.Stat(filepath.Join(cfg.ProjectBrief.Home, "agent")); statErr == nil {
		t.Error("nothing was written before the key was checked")
	}
}
