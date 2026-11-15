package config

import (
	"fmt"
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
		"clone":   cfg.Repos.CloneDir,
		"store":   cfg.Store.Dir,
		"home":    cfg.Explore.Home,
		"profile": cfg.Profile.Home,
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
	for _, dir := range []string{cfg.Repos.CloneDir, cfg.Store.Dir, cfg.Explore.Home, cfg.Profile.Home} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("%s was not created", dir)
		}
	}
}

// The CV extraction is its own job with its own agent home: it must not share PIG_HOME
// with exploration, because that home is rewritten on every run and two runs would race
// on it.
func TestProfileSectionHasItsOwnDefaults(t *testing.T) {
	cfg := Default()
	if cfg.Profile.Binary == "" {
		t.Error("profile.binary is empty")
	}
	if cfg.Profile.Home == cfg.Explore.Home {
		t.Errorf("profile.home = %q is the exploration home", cfg.Profile.Home)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("the profile defaults are invalid: %v", err)
	}
}

// The same coherence rules as exploration: a kill threshold has to be harder than the
// warning, and the budget has to leave room for the kill to happen.
func TestValidateRejectsIncoherentProfileTimeouts(t *testing.T) {
	for name, mutate := range map[string]func(*Profile){
		"idle_kill below idle_warn": func(p *Profile) { p.IdleKill = Duration(5 * time.Second) },
		"budget below idle_kill":    func(p *Profile) { p.Budget = Duration(10 * time.Second) },
		"zero idle":                 func(p *Profile) { p.IdleWarn, p.IdleKill = 0, 0 },
		"zero budget":               func(p *Profile) { p.Budget = 0 },
	} {
		cfg := Default()
		mutate(&cfg.Profile)
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestValidateRejectsEmptyProfileHome(t *testing.T) {
	cfg := Default()
	cfg.Profile.Home = " "
	if err := cfg.Validate(); err == nil {
		t.Error("an empty profile.home was accepted")
	}

	cfg = Default()
	cfg.Profile.Binary = ""
	if err := cfg.Validate(); err == nil {
		t.Error("an empty profile.binary was accepted")
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

// El tope de análisis en paralelo sale a tres por defecto: es la decisión sellada de la
// #12 (S3), y un archivo parcial que no lo nombre tiene que seguir corriendo.
func TestExploreWorkersDefaultsToThree(t *testing.T) {
	if got := Default().Explore.Workers; got != 3 {
		t.Errorf("explore.workers por defecto = %d, want 3", got)
	}

	cfg, err := Load(write(t, `{"explore":{"model":"gemma4"}}`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Explore.Workers != 3 {
		t.Errorf("explore.workers = %d, want el default 3", cfg.Explore.Workers)
	}
}

// El paralelismo tiene techo y piso: fuera de 1..3 el proceso no arranca, en vez de
// levantar decenas de procesos pig contra la suscripción.
func TestExploreWorkersMustBeBetweenOneAndThree(t *testing.T) {
	for _, workers := range []string{"0", "4", "-1", "10"} {
		_, err := Load(write(t, `{"explore":{"workers":`+workers+`}}`))
		if err == nil {
			t.Errorf("workers=%s fue aceptado, want un rechazo", workers)
			continue
		}
		msg := err.Error()
		if !strings.Contains(msg, "explore.workers") || !strings.Contains(msg, "1") || !strings.Contains(msg, "3") {
			t.Errorf("workers=%s: err = %v, want que nombre el campo y el rango 1..3", workers, msg)
		}
	}

	for _, workers := range []string{"1", "2", "3"} {
		cfg, err := Load(write(t, `{"explore":{"workers":`+workers+`}}`))
		if err != nil {
			t.Errorf("workers=%s fue rechazado: %v", workers, err)
			continue
		}
		if got := fmt.Sprint(cfg.Explore.Workers); got != workers {
			t.Errorf("workers = %s, want %s", got, workers)
		}
	}
}

// S12: the portrait runs on a chat model from the constitutional allowlist, chosen per action like
// every other generative job. The default is the one the user sealed.
func TestProjectBriefDefaultsToTheAllowedChatModel(t *testing.T) {
	cfg := Default()
	if got, want := cfg.Models.ProjectBrief, "deepseek-v4-flash"; got != want {
		t.Errorf("models.project_brief = %q, want %q", got, want)
	}
	if got := cfg.ProjectBrief.Home; got == cfg.Explore.Home || got == cfg.Profile.Home {
		t.Errorf("project_brief.home = %q: the job rewrites its home, so it cannot share one", got)
	}
	if len(cfg.ProjectBrief.Tools) == 0 {
		t.Error("project_brief.tools is empty: the agent could not read the clone")
	}
}

// I13: a model outside the list does not start the process, instead of failing halfway through an
// analysis.
func TestLoadRejectsAProjectBriefModelOutsideTheAllowlist(t *testing.T) {
	for _, body := range []string{
		`{"models":{"project_brief":"qwen3.8-max"}}`,
		`{"models":{"project_brief":"clef"}}`,
		`{"models":{"project_brief":""}}`,
	} {
		_, err := Load(write(t, body))
		if err == nil {
			t.Errorf("%s was accepted", body)
			continue
		}
		if !strings.Contains(err.Error(), "models.project_brief") {
			t.Errorf("%s: err = %v, want it to name the field", body, err)
		}
	}
}

// The job reads a third-party repository with an agent, so its tool surface is a read-only
// allowlist for the same reason exploration's is.
func TestLoadRejectsAProjectBriefToolThatCanWrite(t *testing.T) {
	_, err := Load(write(t, `{"project_brief":{"tools":["read","bash"]}}`))
	if err == nil {
		t.Fatal("a portable bash was accepted")
	}
	if !strings.Contains(err.Error(), "project_brief.tools") {
		t.Errorf("err = %v, want it to name the field", err)
	}
}

func TestProjectBriefDurationOrderIsValidated(t *testing.T) {
	_, err := Load(write(t, `{"project_brief":{"idle_warn":"2m0s","idle_kill":"1m0s"}}`))
	if err == nil {
		t.Fatal("a kill threshold below the warning threshold was accepted")
	}
	if !strings.Contains(err.Error(), "project_brief.idle_kill") {
		t.Errorf("err = %v", err)
	}
}
