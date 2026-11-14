package config

import (
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("the defaults are invalid: %v", err)
	}
}

// The generated example must load back into exactly the defaults, or the documentation
// and the code have drifted.
func TestExampleRoundTripsToDefaults(t *testing.T) {
	b, err := Example()
	if err != nil {
		t.Fatalf("Example() error = %v", err)
	}
	path := write(t, string(b))
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load(example) error = %v\n%s", err, b)
	}
	if !reflect.DeepEqual(got, Default()) {
		t.Errorf("example did not round-trip\n got: %+v\nwant: %+v", got, Default())
	}
}

// A partial file only names what changes; everything else keeps its default.
func TestLoadKeepsDefaultsForOmittedFields(t *testing.T) {
	path := write(t, `{"explore":{"model":"gemma4"}}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Explore.Model != "gemma4" {
		t.Errorf("explore.model = %q, want gemma4", cfg.Explore.Model)
	}
	if cfg.Explore.IdleKill.Get() != 90*time.Second {
		t.Errorf("idle_kill = %v, want the default", cfg.Explore.IdleKill.Get())
	}
	if cfg.Server.Addr != DefaultAddr {
		t.Errorf("addr = %q, want the default", cfg.Server.Addr)
	}
}

// A typo must fail loudly instead of silently using a default.
func TestLoadRejectsUnknownField(t *testing.T) {
	path := write(t, `{"explore":{"modle":"gemma4"}}`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("want an error for an unknown field")
	}
	if !strings.Contains(err.Error(), "modle") {
		t.Errorf("err = %v, want it to name the offending key", err)
	}
}

func TestLoadRejectsTrailingContent(t *testing.T) {
	path := write(t, `{"server":{"addr":":8080"}} {"more":true}`)
	if _, err := Load(path); err == nil {
		t.Fatal("want an error for trailing content")
	}
}

func TestLoadRejectsModelOutsideAllowlist(t *testing.T) {
	for _, model := range []string{"qwen3.8-max", "gpt-4o", ""} {
		path := write(t, `{"explore":{"model":"`+model+`"}}`)
		_, err := Load(path)
		if err == nil {
			t.Errorf("model %q was accepted, want a fail-closed rejection", model)
			continue
		}
		if !strings.Contains(err.Error(), "explore.model") {
			t.Errorf("model %q: err = %v, want it to name the field", model, err)
		}
	}
}

// Exploration is read-only; a configuration cannot grant a write surface.
func TestLoadRejectsWriteCapableTools(t *testing.T) {
	for _, tool := range []string{"bash", "edit", "write", "MultiEdit", "*", "shell"} {
		body := `{"explore":{"tools":["read","` + tool + `"]}}`
		if _, err := Load(write(t, body)); err == nil {
			t.Errorf("tool %q was accepted: exploration would stop being read-only", tool)
		}
	}
}

func TestLoadRejectsEmptyTools(t *testing.T) {
	if _, err := Load(write(t, `{"explore":{"tools":[]}}`)); err == nil {
		t.Fatal("want an error: an agent with no tools cannot explore")
	}
}

func TestValidateRejectsIncoherentTimeouts(t *testing.T) {
	cfg := Default()
	cfg.Explore.IdleKill = Duration(5 * time.Second) // below idle_warn (20s)
	if err := cfg.Validate(); err == nil {
		t.Error("idle_kill below idle_warn was accepted")
	}

	cfg = Default()
	cfg.Explore.Budget = Duration(10 * time.Second) // below idle_kill (90s)
	if err := cfg.Validate(); err == nil {
		t.Error("a budget below idle_kill was accepted")
	}
}

func TestDurationAcceptsStringsAndSeconds(t *testing.T) {
	cases := map[string]time.Duration{
		`{"explore":{"budget":"1m30s"}}`: 90 * time.Second,
		`{"explore":{"budget":180}}`:     180 * time.Second,
	}
	for body, want := range cases {
		cfg, err := Load(write(t, body))
		if err != nil {
			t.Fatalf("%s: Load() error = %v", body, err)
		}
		if got := cfg.Explore.Budget.Get(); got != want {
			t.Errorf("%s: budget = %v, want %v", body, got, want)
		}
	}

	if _, err := Load(write(t, `{"explore":{"budget":"tarde"}}`)); err == nil {
		t.Error("a nonsense duration was accepted")
	}
}

func TestResolveMakesPathsAbsolute(t *testing.T) {
	cfg := Default()
	if err := cfg.Resolve("/srv/app"); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	for name, got := range map[string]string{
		"clone": cfg.Repos.CloneDir,
		"store": cfg.Store.Dir,
		"home":  cfg.Explore.Home,
	} {
		if !filepath.IsAbs(got) {
			t.Errorf("%s = %q, want an absolute path", name, got)
		}
	}
	if want := "/srv/app/var/repos"; cfg.Repos.CloneDir != want {
		t.Errorf("clone dir = %q, want %q", cfg.Repos.CloneDir, want)
	}
}

func TestEnsureDirsCreatesTheOwnedDirectories(t *testing.T) {
	base := t.TempDir()
	cfg := Default()
	if err := cfg.Resolve(base); err != nil {
		t.Fatal(err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() error = %v", err)
	}
	for _, dir := range []string{cfg.Repos.CloneDir, cfg.Store.Dir, cfg.Explore.Home} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("%s was not created", dir)
		}
	}
}

func TestRepoDirNamespacesByOwner(t *testing.T) {
	cfg := Default()
	got := cfg.RepoDir("spf13", "cobra")
	if want := filepath.Join(DefaultCloneDir, "spf13__cobra"); got != want {
		t.Errorf("RepoDir() = %q, want %q", got, want)
	}
}

// clef decides on another endpoint; it cannot drive the agent even though it is in the
// constitutional allowlist.
func TestLoadRejectsClefAsTheExploreModel(t *testing.T) {
	for _, body := range []string{
		`{"explore":{"model":"clef"}}`,
		`{"models":{"explore":"clef"}}`,
		`{"models":{"profile_extract":"clef"}}`,
	} {
		_, err := Load(write(t, body))
		if err == nil {
			t.Errorf("%s was accepted: clef is not a chat model", body)
			continue
		}
		if !strings.Contains(err.Error(), "chat model") {
			t.Errorf("%s: err = %v, want it to say clef is not a chat model", body, err)
		}
	}
}

// El servidor escucha en todas las interfaces por defecto, para poder abrirlo
// desde otra máquina del mismo segmento sin recompilar. La dirección sigue
// siendo configurable, y eso es lo que permite cerrarlo a loopback si hace falta.
func TestDefaultAddrListensOnAllInterfaces(t *testing.T) {
	host, port, err := net.SplitHostPort(DefaultAddr)
	if err != nil {
		t.Fatalf("DefaultAddr = %q, no es host:puerto: %v", DefaultAddr, err)
	}
	if host != "0.0.0.0" {
		t.Errorf("host = %q, want 0.0.0.0 para aceptar conexiones de la red", host)
	}
	if port != "7117" {
		t.Errorf("port = %q, want 7117", port)
	}

	got, err := Load(write(t, `{"server":{"addr":"127.0.0.1:9999"}}`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Server.Addr != "127.0.0.1:9999" {
		t.Errorf("addr = %q, want the configured 127.0.0.1:9999", got.Server.Addr)
	}
}
