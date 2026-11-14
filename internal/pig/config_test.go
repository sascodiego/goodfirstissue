package pig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The generated configuration must name the endpoint and reference the key, never embed it.
func TestWriteConfigWritesBothFiles(t *testing.T) {
	home := t.TempDir()
	if err := WriteConfig(home, DefaultConfig([]string{"gemma4", "qwen3.6"})); err != nil {
		t.Fatalf("WriteConfig() error = %v", err)
	}

	var models modelsFile
	readJSON(t, filepath.Join(home, "agent", "models.json"), &models)

	nan, ok := models.Providers["nan"]
	if !ok {
		t.Fatalf("provider nan missing, got %v", models.Providers)
	}
	if got, want := nan.BaseURL, "https://api.nan.builders/v1"; got != want {
		t.Errorf("baseUrl = %q, want %q", got, want)
	}
	if got, want := nan.APIKey, "$NAN_KEY"; got != want {
		t.Errorf("apiKey = %q, want %q (the value must never be written)", got, want)
	}
	if got, want := nan.API, "openai-completions"; got != want {
		t.Errorf("api = %q, want %q", got, want)
	}
	if len(nan.Models) != 2 || nan.Models[0].ID != "gemma4" {
		t.Errorf("models = %+v, want exactly the ones given", nan.Models)
	}

	var mcp mcpFile
	readJSON(t, filepath.Join(home, "agent", "mcp.json"), &mcp)

	cg, ok := mcp.Servers["codegraph"]
	if !ok {
		t.Fatalf("server codegraph missing, got %v", mcp.Servers)
	}
	if got, want := cg.Exposure, "direct"; got != want {
		t.Errorf("exposure = %q, want %q: codemode would need an execution tool", got, want)
	}
	if got, want := cg.Command, "codegraph"; got != want {
		t.Errorf("command = %q, want %q", got, want)
	}
}

// Regenerating must replace, never merge: a stale entry cannot survive into a run.
func TestWriteConfigReplacesPreviousContent(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "agent"), 0o700); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(home, "agent", "models.json")
	if err := os.WriteFile(stale, []byte(`{"providers":{"leaked":{"baseUrl":"https://evil.example"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := WriteConfig(home, DefaultConfig([]string{"gemma4", "qwen3.6"})); err != nil {
		t.Fatalf("WriteConfig() error = %v", err)
	}

	var models modelsFile
	readJSON(t, stale, &models)
	if _, leaked := models.Providers["leaked"]; leaked {
		t.Error("a previous provider survived the regeneration")
	}
}

func TestWriteConfigIsOwnerOnly(t *testing.T) {
	home := t.TempDir()
	if err := WriteConfig(home, DefaultConfig([]string{"gemma4", "qwen3.6"})); err != nil {
		t.Fatalf("WriteConfig() error = %v", err)
	}
	info, err := os.Stat(filepath.Join(home, "agent", "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want 600", perm)
	}
}

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(b, into); err != nil {
		t.Fatalf("decode %s: %v\n%s", path, err, b)
	}
}

// An empty model list would produce an agent with nothing to run.
func TestWriteConfigRejectsNoModels(t *testing.T) {
	if err := WriteConfig(t.TempDir(), DefaultConfig(nil)); err == nil {
		t.Fatal("want an error when no models are configured")
	}
}
