// Package config holds the application's single source of settings.
//
// The format is JSON on purpose: it needs no third-party module, it is what every other
// component of this system already speaks, and encoding/json can reject unknown fields so
// a typo fails loudly instead of silently falling back to a default. YAML would cost a
// module in go.mod, which PROYECTO.md §17.1 does not allow.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Default paths, relative to the working directory.
const (
	DefaultCloneDir   = "var/repos"
	DefaultStoreDir   = "var/store"
	DefaultPigHomeDir = "var/pig"
	// DefaultProfileHomeDir is PiG's home for the CV extraction job. It is a second home
	// and not a subdirectory of the first: each job regenerates its own configuration.
	DefaultProfileHomeDir = "var/pig-profile"
	// 7117 y no 8080: 8080 es el default de media docena de servidores de desarrollo, y dos
	// procesos peleando por el mismo puerto es la primera hora perdida de una demo. La
	// interfaz sigue siendo 0.0.0.0 porque el binario corre en un VPS y se mira desde afuera.
	DefaultAddr = "0.0.0.0:7117"
)

// AllowedChatModels are the allowlist entries usable through /v1/chat/completions.
// clef is a decision model on a different endpoint: it cannot drive the agent.
var AllowedChatModels = []string{
	"deepseek-v4-flash", "glm5.3-flash", "mimo-v2.6-flash", "gemma4", "qwen3.6",
}

// AllowedModels is the constitutional allowlist (I13): every model the application may
// name at all. Validation fails closed outside it.
var AllowedModels = []string{
	"clef", "deepseek-v4-flash", "glm5.3-flash", "mimo-v2.6-flash", "gemma4", "qwen3.6",
}

// forbiddenTools can write or execute. An exploration agent must not have them, so a
// configuration that grants one is invalid rather than merely unusual.
var forbiddenTools = []string{"bash", "shell", "edit", "write", "multiedit", "*"}

// Config is the whole application configuration.
type Config struct {
	Server  Server  `json:"server"`
	Repos   Repos   `json:"repos"`
	Store   Store   `json:"store"`
	GitHub  GitHub  `json:"github"`
	Indexer Indexer `json:"indexer"`
	Explore Explore `json:"explore"`
	Profile Profile `json:"profile"`
	Clef    Clef    `json:"clef"`
	Models  Models  `json:"models"`
}

// Server holds the HTTP listener.
type Server struct {
	Addr string `json:"addr"`
}

// Repos holds where clones live.
type Repos struct {
	// CloneDir is where repositories are cloned. Each repository gets a subdirectory.
	CloneDir string `json:"clone_dir"`
}

// Store holds where the frozen corpus and results live.
type Store struct {
	Dir string `json:"dir"`
}

// GitHub configures the fetch. There is no label filter: labels are never a criterion.
type GitHub struct {
	MaxIssues int `json:"max_issues"`
	PageSize  int `json:"page_size"`
	// Timeout bounds one fetch, including all pages.
	Timeout Duration `json:"timeout"`
}

// Indexer configures the code graph preparation.
type Indexer struct {
	Binary  string   `json:"binary"`
	Timeout Duration `json:"timeout"`
}

// Explore configures the delegated read-only agent.
type Explore struct {
	Binary string `json:"binary"`
	Model  string `json:"model"`
	// Home is this application's PIG_HOME. It is generated and never inherited.
	Home string `json:"home"`
	// Tools is the complete allowlist. It must not include anything that can write.
	Tools    []string `json:"tools"`
	IdleWarn Duration `json:"idle_warn"`
	IdleKill Duration `json:"idle_kill"`
	Budget   Duration `json:"budget"`
}

// Profile configures the CV extraction job.
//
// It delegates to the same headless agent as exploration, but it is a different job: the
// CV is text, so the run gets no tools and connects no MCP server. The home is separate
// on purpose — PiG's configuration is regenerated on every run, and two jobs sharing one
// home would race on that write.
//
// The model is not here: it is models.profile_extract, one entry per generative action
// (§17.7).
type Profile struct {
	Binary   string   `json:"binary"`
	Home     string   `json:"home"`
	IdleWarn Duration `json:"idle_warn"`
	IdleKill Duration `json:"idle_kill"`
	Budget   Duration `json:"budget"`
}

// Clef configures the decision model endpoint.
type Clef struct {
	URL        string   `json:"url"`
	Timeout    Duration `json:"timeout"`
	MaxRetries int      `json:"max_retries"`
}

// Models maps configurable actions to model IDs. The decision actions (triage,
// alignment, label_honest, fit) are fixed to clef in code and are not configurable.
type Models struct {
	Explore        string `json:"explore"`
	ProfileExtract string `json:"profile_extract"`
}

// Default returns the configuration used when a file omits a field.
func Default() *Config {
	return &Config{
		Server: Server{Addr: DefaultAddr},
		Repos:  Repos{CloneDir: DefaultCloneDir},
		Store:  Store{Dir: DefaultStoreDir},
		GitHub: GitHub{MaxIssues: 150, PageSize: 100, Timeout: Duration(60_000_000_000)},
		Indexer: Indexer{
			Binary:  "codegraph",
			Timeout: Duration(300_000_000_000),
		},
		Explore: Explore{
			Binary:   "pig",
			Model:    "deepseek-v4-flash",
			Home:     DefaultPigHomeDir,
			Tools:    []string{"read", "ls", "grep", "mcp__codegraph__codegraph_explore"},
			IdleWarn: Duration(20_000_000_000),
			IdleKill: Duration(90_000_000_000),
			Budget:   Duration(180_000_000_000),
		},
		Profile: Profile{
			Binary:   "pig",
			Home:     DefaultProfileHomeDir,
			IdleWarn: Duration(30_000_000_000),
			IdleKill: Duration(120_000_000_000),
			Budget:   Duration(180_000_000_000),
		},
		Clef: Clef{
			URL:        "https://api.nan.builders/v1/systemone",
			Timeout:    Duration(60_000_000_000),
			MaxRetries: 5,
		},
		Models: Models{
			Explore:        "deepseek-v4-flash",
			ProfileExtract: "glm5.3-flash",
		},
	}
}

// Load reads path over the defaults, so a partial file only has to name what changes.
// Unknown fields are an error: a misspelled key must not be ignored.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	cfg := Default()
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("config: %s: unexpected trailing content", path)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}
	return cfg, nil
}

// Validate enforces every rule that must hold before anything runs.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Server.Addr) == "" {
		return fmt.Errorf("server.addr is empty")
	}
	if strings.TrimSpace(c.Repos.CloneDir) == "" {
		return fmt.Errorf("repos.clone_dir is empty")
	}
	if strings.TrimSpace(c.Store.Dir) == "" {
		return fmt.Errorf("store.dir is empty")
	}
	if c.GitHub.MaxIssues <= 0 {
		return fmt.Errorf("github.max_issues must be positive, got %d", c.GitHub.MaxIssues)
	}
	if c.GitHub.PageSize <= 0 || c.GitHub.PageSize > 100 {
		return fmt.Errorf("github.page_size must be between 1 and 100, got %d", c.GitHub.PageSize)
	}
	if c.GitHub.Timeout.Get() <= 0 {
		return fmt.Errorf("github.timeout must be positive")
	}
	if strings.TrimSpace(c.Indexer.Binary) == "" {
		return fmt.Errorf("indexer.binary is empty")
	}
	if c.Indexer.Timeout.Get() <= 0 {
		return fmt.Errorf("indexer.timeout must be positive")
	}

	if strings.TrimSpace(c.Explore.Binary) == "" {
		return fmt.Errorf("explore.binary is empty")
	}
	if strings.TrimSpace(c.Explore.Home) == "" {
		return fmt.Errorf("explore.home is empty")
	}
	if err := requireChatModel("explore.model", c.Explore.Model); err != nil {
		return err
	}
	if err := requireChatModel("models.explore", c.Models.Explore); err != nil {
		return err
	}
	if err := requireChatModel("models.profile_extract", c.Models.ProfileExtract); err != nil {
		return err
	}
	if err := validateTools(c.Explore.Tools); err != nil {
		return err
	}
	if c.Explore.IdleWarn.Get() <= 0 || c.Explore.IdleKill.Get() <= 0 || c.Explore.Budget.Get() <= 0 {
		return fmt.Errorf("explore idle and budget durations must be positive")
	}
	if c.Explore.IdleKill.Get() <= c.Explore.IdleWarn.Get() {
		return fmt.Errorf("explore.idle_kill (%s) must be greater than explore.idle_warn (%s)",
			c.Explore.IdleKill.Get(), c.Explore.IdleWarn.Get())
	}
	if c.Explore.Budget.Get() < c.Explore.IdleKill.Get() {
		return fmt.Errorf("explore.budget (%s) must be at least explore.idle_kill (%s)",
			c.Explore.Budget.Get(), c.Explore.IdleKill.Get())
	}

	if strings.TrimSpace(c.Profile.Binary) == "" {
		return fmt.Errorf("profile.binary is empty")
	}
	if strings.TrimSpace(c.Profile.Home) == "" {
		return fmt.Errorf("profile.home is empty")
	}
	if c.Profile.IdleWarn.Get() <= 0 || c.Profile.IdleKill.Get() <= 0 || c.Profile.Budget.Get() <= 0 {
		return fmt.Errorf("profile idle and budget durations must be positive")
	}
	if c.Profile.IdleKill.Get() <= c.Profile.IdleWarn.Get() {
		return fmt.Errorf("profile.idle_kill (%s) must be greater than profile.idle_warn (%s)",
			c.Profile.IdleKill.Get(), c.Profile.IdleWarn.Get())
	}
	if c.Profile.Budget.Get() < c.Profile.IdleKill.Get() {
		return fmt.Errorf("profile.budget (%s) must be at least profile.idle_kill (%s)",
			c.Profile.Budget.Get(), c.Profile.IdleKill.Get())
	}

	if strings.TrimSpace(c.Clef.URL) == "" {
		return fmt.Errorf("clef.url is empty")
	}
	if c.Clef.Timeout.Get() <= 0 {
		return fmt.Errorf("clef.timeout must be positive")
	}
	if c.Clef.MaxRetries < 0 {
		return fmt.Errorf("clef.max_retries must not be negative")
	}
	return nil
}

func requireChatModel(field, model string) error {
	if model == "" {
		return fmt.Errorf("%s is empty", field)
	}
	for _, allowed := range AllowedChatModels {
		if model == allowed {
			return nil
		}
	}
	return fmt.Errorf("%s = %q is not a chat model in the allowlist %v", field, model, AllowedChatModels)
}

func validateTools(tools []string) error {
	if len(tools) == 0 {
		return fmt.Errorf("explore.tools is empty: the agent would have no surface")
	}
	for _, tool := range tools {
		name := strings.ToLower(strings.TrimSpace(tool))
		for _, forbidden := range forbiddenTools {
			if name == forbidden {
				return fmt.Errorf("explore.tools grants %q, which can write: exploration is read-only", tool)
			}
		}
	}
	return nil
}

// Resolve turns the configured directories into absolute paths. It does not create them;
// EnsureDirs does, so loading a configuration has no side effects.
func (c *Config) Resolve(base string) error {
	var err error
	if c.Repos.CloneDir, err = abs(base, c.Repos.CloneDir); err != nil {
		return err
	}
	if c.Store.Dir, err = abs(base, c.Store.Dir); err != nil {
		return err
	}
	if c.Explore.Home, err = abs(base, c.Explore.Home); err != nil {
		return err
	}
	if c.Profile.Home, err = abs(base, c.Profile.Home); err != nil {
		return err
	}
	return nil
}

func abs(base, path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Abs(filepath.Join(base, path))
}

// EnsureDirs creates the directories the application owns.
func (c *Config) EnsureDirs() error {
	for _, dir := range []string{c.Repos.CloneDir, c.Store.Dir, c.Explore.Home, c.Profile.Home} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("config: create %s: %w", dir, err)
		}
	}
	return nil
}

// RepoDir returns the clone directory of one repository.
func (c *Config) RepoDir(owner, name string) string {
	return filepath.Join(c.Repos.CloneDir, owner+"__"+name)
}

// Example renders the default configuration as the documented example file.
func Example() ([]byte, error) {
	b, err := json.MarshalIndent(Default(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("config: encode example: %w", err)
	}
	return append(b, '\n'), nil
}
