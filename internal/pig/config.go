// Package pig drives the PiG agent as a read-only exploration engine.
//
// The application owns PiG's configuration: it generates the whole tree under its own
// PIG_HOME so the agent never reads, nor inherits, a user's ~/.pig. The provider is
// always NaN, because this is the NaN hackathon.
package pig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the PiG configuration this application owns and regenerates on every run.
type Config struct {
	// BaseURL is the OpenAI-compatible endpoint. Fixed: the provider is always NaN.
	BaseURL string
	// Models are the model IDs offered to the agent. The allowlist itself is enforced
	// by the application, not here.
	Models []string
	// APIKeyEnv is the name of the environment variable holding the key. The value is
	// never written to disk; models.json only references it as $NAME.
	APIKeyEnv string
	// McpCmd and McpArgs launch the code graph as a stdio MCP server.
	McpCmd  string
	McpArgs []string
}

// DefaultConfig returns the configuration for the NaN provider with the code graph
// exposed directly, so the agent gets it as a normal tool instead of through a
// code-execution surface.
//
// The model list is a parameter, not a constant: the allowlist belongs to the
// application, and this package must not hold a second copy that can drift from it.
func DefaultConfig(models []string) Config {
	return Config{
		BaseURL:   "https://api.nan.builders/v1",
		Models:    models,
		APIKeyEnv: "NAN_KEY",
		McpCmd:    "codegraph",
		McpArgs:   []string{"serve", "--mcp"},
	}
}

type providerModel struct {
	ID string `json:"id"`
}

type providerEntry struct {
	Name    string          `json:"name"`
	BaseURL string          `json:"baseUrl"`
	APIKey  string          `json:"apiKey"`
	API     string          `json:"api"`
	Models  []providerModel `json:"models"`
}

type modelsFile struct {
	Providers map[string]providerEntry `json:"providers"`
}

type mcpServer struct {
	Command  string   `json:"command"`
	Args     []string `json:"args"`
	Exposure string   `json:"exposure"`
}

type mcpFile struct {
	Servers map[string]mcpServer `json:"mcpServers"`
}

// WriteConfig creates <home>/agent/{models.json,mcp.json}, replacing whatever was there.
// Regenerating on every run is deliberate: a stale or foreign file cannot leak in.
func WriteConfig(home string, c Config) error {
	if len(c.Models) == 0 {
		return fmt.Errorf("pig: no models configured: the agent would have nothing to run")
	}

	agentDir := filepath.Join(home, "agent")
	if err := os.MkdirAll(agentDir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", agentDir, err)
	}

	models := make([]providerModel, 0, len(c.Models))
	for _, m := range c.Models {
		models = append(models, providerModel{ID: m})
	}
	mf := modelsFile{Providers: map[string]providerEntry{
		"nan": {
			Name:    "NaN",
			BaseURL: c.BaseURL,
			APIKey:  "$" + c.APIKeyEnv,
			API:     "openai-completions",
			Models:  models,
		},
	}}

	cf := mcpFile{Servers: map[string]mcpServer{
		"codegraph": {
			Command:  c.McpCmd,
			Args:     c.McpArgs,
			Exposure: "direct",
		},
	}}

	if err := writeJSON(filepath.Join(agentDir, "models.json"), mf); err != nil {
		return err
	}
	return writeJSON(filepath.Join(agentDir, "mcp.json"), cf)
}

func writeJSON(path string, v any) error {
	// 0o600: the file holds no secret, but it does name one. Keep it owner-only anyway.
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
