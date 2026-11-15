package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// commitPin is the commit every fixture freezes its corpus at. It is the pseudo-SHA the
// existing corpus tests already use, so a pin that appears in a failed assertion is
// recognisable.
const commitPin = "adbc8813901bba65827259daa8e22ff94ec1f30e"

// exploreCorpus seeds a repository whose corpus is frozen at commitPin, with no code
// graph: the queue's preflight then fails the way a repository that was fetched but never
// indexed fails.
func exploreCorpus(t *testing.T, issues int) (*store.Store, string) {
	t.Helper()

	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	key, name := "demo__repo", "demo/repo"

	list := make([]ghdata.Issue, 0, issues)
	for i := 1; i <= issues; i++ {
		list = append(list, ghdata.Issue{
			Number: i,
			Title:  fmt.Sprintf("issue %d", i),
			Body:   "the body of the issue",
			URL:    fmt.Sprintf("https://github.com/demo/repo/issues/%d", i),
		})
	}

	if err := st.SaveRepo(store.RepoEntry{
		Name:      name,
		Key:       key,
		CommitSHA: commitPin,
		TotalOpen: issues,
		Issues:    issues,
	}); err != nil {
		t.Fatalf("SaveRepo() error = %v", err)
	}
	if err := st.SaveSnapshot(key, &ghdata.Snapshot{
		Repository: ghdata.Repository{NameWithOwner: name, CommitSHA: commitPin, PrimaryLanguage: "Go"},
		TotalOpen:  issues,
		Issues:     list,
	}); err != nil {
		t.Fatalf("SaveSnapshot() error = %v", err)
	}
	return st, key
}

// indexedCorpus adds the code graph pinned to the same commit as the corpus, so the queue
// preflight passes without cloning or indexing anything.
func indexedCorpus(t *testing.T, issues int) (*store.Store, string) {
	t.Helper()

	st, key := exploreCorpus(t, issues)
	if err := st.SaveIndex(key, store.IndexRecord{
		Repository: key,
		CommitSHA:  commitPin,
		Path:       "var/repos/" + key,
	}); err != nil {
		t.Fatalf("SaveIndex() error = %v", err)
	}
	return st, key
}

// fakeStage is the injected runner. No test here runs pig, touches the network or needs a
// clone: the fake records what ran, reports the silence it wants to report, and can hold a
// run open so the queue's intermediate states are observable.
type fakeStage struct {
	mu       sync.Mutex
	runs     []StageRequest
	progress []time.Duration
	outcome  StageOutcome
	err      error

	hold      chan struct{}
	entered   chan StageRequest
	finished  chan StageRequest
	observedC chan struct{}
	once      sync.Once
}

func newFakeStage() *fakeStage {
	return &fakeStage{
		entered:   make(chan StageRequest, 16),
		finished:  make(chan StageRequest, 16),
		observedC: make(chan struct{}, 16),
	}
}

// held makes every run block until release is called.
func (f *fakeStage) held() *fakeStage {
	f.hold = make(chan struct{})
	return f
}

func (f *fakeStage) release() {
	f.once.Do(func() { close(f.hold) })
}

func (f *fakeStage) stage(ctx context.Context, req StageRequest, progress func(time.Duration)) (StageOutcome, error) {
	f.mu.Lock()
	f.runs = append(f.runs, req)
	hold, silences, outcome, want := f.hold, f.progress, f.outcome, f.err
	f.mu.Unlock()

	select {
	case f.entered <- req:
	default:
	}

	for _, silence := range silences {
		progress(silence)
	}

	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
		}
	}
	if err := ctx.Err(); err != nil {
		select {
		case f.observedC <- struct{}{}:
		default:
		}
		select {
		case f.finished <- req:
		default:
		}
		return StageOutcome{}, err
	}
	if want != nil {
		select {
		case f.finished <- req:
		default:
		}
		return StageOutcome{}, want
	}

	select {
	case f.finished <- req:
	default:
	}
	return outcome, nil
}

func (f *fakeStage) ran() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.runs)
}

// exploreHandler builds a handler whose queue runs the injected stage. The defaults are
// what production looks like on a configured machine: a model, a key, and the test corpus.
func exploreHandler(t *testing.T, st *store.Store, stage StageRunner, change func(*Options)) *Handler {
	t.Helper()

	opts := Options{
		Store:  st,
		Stage:  stage,
		Getenv: func(name string) string { return "test-key" },
		Explore: ExploreOptions{
			Model: "gemma4",
		},
	}
	if change != nil {
		change(&opts)
	}

	h, err := NewHandler(opts)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return h
}

// redirectQuery returns the query of a 303 Location, so a test asserts on the code and the
// count instead of on the order the parameters were encoded in.
func redirectQuery(t *testing.T, rec *httptest.ResponseRecorder) url.Values {
	t.Helper()

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Location = %q: %v", rec.Header().Get("Location"), err)
	}
	return u.Query()
}

// rowsOf builds the page data the dashboard renders: that is where the queue's state
// becomes visible. Asserting on the data instead of on the HTML keeps these tests out of
// the row templates, which belong to another task.
func rowsOf(t *testing.T, h *Handler, repoKey string) map[int]rowView {
	t.Helper()

	data, err := buildPageWith(h.store, url.Values{"repo": {repoKey}}, h.queue)
	if err != nil {
		t.Fatalf("buildPageWith() error = %v", err)
	}
	rows := make(map[int]rowView, len(data.Rows))
	for _, r := range data.Rows {
		rows[r.Number] = r
	}
	return rows
}

// exploreStage returns the ladder rung the queue owns today for one row.
func exploreStage(t *testing.T, row rowView) stageView {
	t.Helper()

	for _, s := range row.Stages {
		if s.Stage == stageExplore {
			return s
		}
	}
	t.Fatalf("la fila #%d no expone la etapa %q", row.Number, stageExplore)
	return stageView{}
}

// waitFor polls the page data until cond holds. The queue is server state, so the page is
// the contract a test can watch without reaching into the queue's internals.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timeout esperando %s", what)
}

// S2/S4: the POST returns immediately while the jobs are still queued or running, and the
// three of them finish once the runner lets them.
func TestExploreQueuesSelectedIssues(t *testing.T) {
	st, key := indexedCorpus(t, 3)
	fake := newFakeStage().held()
	defer fake.release()
	h := exploreHandler(t, st, fake.stage, nil)

	rec := post(t, h, "/explore", url.Values{
		"repo":  {key},
		"issue": {"1", "2", "3"},
	})
	got := redirectQuery(t, rec)
	if got.Get("estado") != "analizando" {
		t.Errorf("estado = %q, want analizando", got.Get("estado"))
	}
	if got.Get("n") != "3" {
		t.Errorf("n = %q, want 3 encoladas", got.Get("n"))
	}

	// The POST is already answered and nothing has been released: the three jobs are in
	// the queue, not running as a side effect of the page request.
	rows := rowsOf(t, h, key)
	for _, number := range []int{1, 2, 3} {
		state := exploreStage(t, rows[number]).State
		if state != jobQueued && state != jobRunning {
			t.Fatalf("fila #%d = %q con la corrida retenida, want queued o running", number, state)
		}
	}

	fake.release()
	waitFor(t, "las tres filas listas", func() bool {
		rows := rowsOf(t, h, key)
		for _, number := range []int{1, 2, 3} {
			if exploreStage(t, rows[number]).State != jobReady {
				return false
			}
		}
		return true
	})
}

// S3: the parallelism is capped at three no matter what the configuration asks for.
func TestExploreWorkersCapAtThree(t *testing.T) {
	st, key := indexedCorpus(t, 4)
	fake := newFakeStage().held()
	defer fake.release()
	h := exploreHandler(t, st, fake.stage, func(o *Options) { o.Workers = 10 })

	post(t, h, "/explore", url.Values{
		"repo":  {key},
		"issue": {"1", "2", "3", "4"},
	})

	waitFor(t, "tres corridas en curso", func() bool {
		rows := rowsOf(t, h, key)
		running := 0
		for _, number := range []int{1, 2, 3, 4} {
			if exploreStage(t, rows[number]).State == jobRunning {
				running++
			}
		}
		return running == exploreWorkersCap
	})

	rows := rowsOf(t, h, key)
	waiting := 0
	for _, number := range []int{1, 2, 3, 4} {
		if exploreStage(t, rows[number]).State == jobQueued {
			waiting++
		}
	}
	if waiting != 1 {
		t.Fatalf("en cola = %d, want 1: %d es el tope de corridas en paralelo", waiting, exploreWorkersCap)
	}
}

// S5 at the web boundary: a reused answer shows the state that says so, with how old it is.
func TestExploreReusedAnswerIsAnnounced(t *testing.T) {
	st, key := indexedCorpus(t, 1)
	fake := newFakeStage()
	original := time.Now().Add(-50 * time.Hour)
	fake.outcome = StageOutcome{
		Reused:  true,
		At:      original,
		Latency: 900 * time.Millisecond,
	}
	h := exploreHandler(t, st, fake.stage, nil)

	post(t, h, "/explore", url.Values{"repo": {key}, "issue": {"1"}})

	waitFor(t, "la fila reutilizada", func() bool {
		return exploreStage(t, rowsOf(t, h, key)[1]).State == jobReused
	})
	stage := exploreStage(t, rowsOf(t, h, key)[1])
	if stage.Age != "hace 2 días" {
		t.Errorf("Age = %q, want hace 2 días", stage.Age)
	}
	if stage.At != formatDate(original) {
		t.Errorf("At = %q, want %q", stage.At, formatDate(original))
	}
}

// S7: without NAN_KEY nothing runs and every selected issue says why.
func TestExploreWithoutNANKeyFailsPerIssue(t *testing.T) {
	st, key := indexedCorpus(t, 2)
	fake := newFakeStage()
	h := exploreHandler(t, st, fake.stage, func(o *Options) {
		o.Getenv = func(string) string { return "" }
	})

	rec := post(t, h, "/explore", url.Values{"repo": {key}, "issue": {"1", "2"}})
	if got := redirectQuery(t, rec); got.Get("n") != "0" {
		t.Errorf("n = %q, want 0: sin clave no se encola nada", got.Get("n"))
	}
	if fake.ran() != 0 {
		t.Fatalf("el runner corrió %d veces sin NAN_KEY", fake.ran())
	}

	rows := rowsOf(t, h, key)
	for _, number := range []int{1, 2} {
		stage := exploreStage(t, rows[number])
		if stage.State != jobError {
			t.Errorf("fila #%d = %q, want error", number, stage.State)
		}
		if !strings.Contains(stage.Note, "NAN_KEY") {
			t.Errorf("fila #%d nota = %q, want que nombre NAN_KEY", number, stage.Note)
		}
	}
}

// An unconfigured model is a refusal with a reason, not a silent no-op.
func TestExploreWithoutModelRefusesWithAReason(t *testing.T) {
	st, key := indexedCorpus(t, 1)
	fake := newFakeStage()
	h := exploreHandler(t, st, fake.stage, func(o *Options) {
		o.Explore = ExploreOptions{}
	})

	post(t, h, "/explore", url.Values{"repo": {key}, "issue": {"1"}})

	if fake.ran() != 0 {
		t.Fatalf("el runner corrió %d veces sin modelo configurado", fake.ran())
	}
	stage := exploreStage(t, rowsOf(t, h, key)[1])
	if stage.State != jobError {
		t.Fatalf("fila = %q, want error", stage.State)
	}
	if !strings.Contains(stage.Note, "explore.model") {
		t.Errorf("nota = %q, want que nombre explore.model", stage.Note)
	}
}

// S13: a repository without a code graph is not analysed, and the row says what to run.
func TestExploreWithoutIndexExplainsTheFix(t *testing.T) {
	st, key := exploreCorpus(t, 1) // registered and fetched, never indexed
	fake := newFakeStage()
	h := exploreHandler(t, st, fake.stage, nil)

	post(t, h, "/explore", url.Values{"repo": {key}, "issue": {"1"}})

	if fake.ran() != 0 {
		t.Fatalf("el runner corrió %d veces sin índice", fake.ran())
	}
	stage := exploreStage(t, rowsOf(t, h, key)[1])
	if stage.State != jobError {
		t.Fatalf("fila = %q, want error", stage.State)
	}
	if !strings.Contains(stage.Note, "gfi index demo/repo") {
		t.Errorf("nota = %q, want que indique gfi index demo/repo", stage.Note)
	}
}

// S13: a graph from another commit is another program, so it is refused with the fix.
func TestExploreStaleIndexExplainsTheFix(t *testing.T) {
	st, key := indexedCorpus(t, 1)
	if err := st.SaveIndex(key, store.IndexRecord{
		Repository: key,
		CommitSHA:  "0123456789abcdef0123456789abcdef01234567",
		Path:       "var/repos/" + key,
	}); err != nil {
		t.Fatalf("SaveIndex() error = %v", err)
	}
	fake := newFakeStage()
	h := exploreHandler(t, st, fake.stage, nil)

	post(t, h, "/explore", url.Values{"repo": {key}, "issue": {"1"}})

	if fake.ran() != 0 {
		t.Fatalf("el runner corrió %d veces con el índice en otro commit", fake.ran())
	}
	stage := exploreStage(t, rowsOf(t, h, key)[1])
	if stage.State != jobError {
		t.Fatalf("fila = %q, want error", stage.State)
	}
	if !strings.Contains(stage.Note, "gfi index demo/repo") {
		t.Errorf("nota = %q, want que indique gfi index demo/repo", stage.Note)
	}
}

// S9: draining leaves the running job alone and canceling it kills the run through its
// context. A canceled job never comes back as ready.
func TestExploreCancelDrainsQueuedAndKillsRunning(t *testing.T) {
	st, key := indexedCorpus(t, 3)
	fake := newFakeStage().held()
	defer fake.release()
	h := exploreHandler(t, st, fake.stage, func(o *Options) { o.Workers = 1 })

	post(t, h, "/explore", url.Values{"repo": {key}, "issue": {"1", "2", "3"}})

	waitFor(t, "la primera corrida", func() bool {
		return exploreStage(t, rowsOf(t, h, key)[1]).State == jobRunning
	})

	rec := post(t, h, "/explore/clear", url.Values{"repo": {key}})
	got := redirectQuery(t, rec)
	if got.Get("estado") != "vaciada" {
		t.Errorf("estado = %q, want vaciada", got.Get("estado"))
	}
	if got.Get("n") != "2" {
		t.Errorf("n = %q, want 2 descartadas", got.Get("n"))
	}

	rows := rowsOf(t, h, key)
	for _, number := range []int{2, 3} {
		if state := exploreStage(t, rows[number]).State; state != jobCanceled {
			t.Errorf("fila #%d = %q, want canceled", number, state)
		}
	}
	if state := exploreStage(t, rows[1]).State; state != jobRunning {
		t.Errorf("fila #1 = %q, want running: vaciar no toca lo que corre", state)
	}

	if got := redirectQuery(t, post(t, h, "/explore/cancel", url.Values{"repo": {key}, "issue": {"1"}})); got.Get("estado") != "cancelado" {
		t.Errorf("estado = %q, want cancelado", got.Get("estado"))
	}

	// The fake observes the cancellation through the context it was given.
	select {
	case <-fake.observedC:
	case <-time.After(5 * time.Second):
		t.Fatal("el runner falso no vio la cancelación del contexto")
	}

	if state := exploreStage(t, rowsOf(t, h, key)[1]).State; state != jobCanceled {
		t.Fatalf("fila #1 = %q después de cancelar, want canceled", state)
	}

	// Releasing does not resurrect it: a canceled job never becomes ready.
	fake.release()
	waitFor(t, "la corrida retenida termina", func() bool {
		select {
		case <-fake.finished:
			return true
		default:
			return false
		}
	})
	if state := exploreStage(t, rowsOf(t, h, key)[1]).State; state != jobCanceled {
		t.Fatalf("fila #1 = %q después de soltar la corrida, want canceled", state)
	}
}

// S9: cancel-all stops what runs and empties what waits, for one repository only.
func TestExploreCancelAllStopsEverything(t *testing.T) {
	st, key := indexedCorpus(t, 3)
	fake := newFakeStage().held()
	defer fake.release()
	h := exploreHandler(t, st, fake.stage, func(o *Options) { o.Workers = 1 })

	post(t, h, "/explore", url.Values{"repo": {key}, "issue": {"1", "2", "3"}})
	waitFor(t, "la primera corrida", func() bool {
		return exploreStage(t, rowsOf(t, h, key)[1]).State == jobRunning
	})

	got := redirectQuery(t, post(t, h, "/explore/cancel-all", url.Values{"repo": {key}}))
	if got.Get("n") != "3" {
		t.Errorf("n = %q, want 3 canceladas: una en curso y dos en cola", got.Get("n"))
	}

	rows := rowsOf(t, h, key)
	for _, number := range []int{1, 2, 3} {
		if state := exploreStage(t, rows[number]).State; state != jobCanceled {
			t.Errorf("fila #%d = %q, want canceled", number, state)
		}
	}
	select {
	case <-fake.observedC:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelar todo no cortó la corrida en curso")
	}
}

// S14: the per-POST ceiling refuses with a code and a count, and queues nothing.
func TestExploreOverFiftyIssuesIsRefused(t *testing.T) {
	st, key := indexedCorpus(t, 51)
	fake := newFakeStage()
	h := exploreHandler(t, st, fake.stage, nil)

	form := url.Values{"repo": {key}}
	for i := 1; i <= 51; i++ {
		form.Add("issue", fmt.Sprintf("%d", i))
	}
	got := redirectQuery(t, post(t, h, "/explore", form))
	if got.Get("estado") != "limite" {
		t.Errorf("estado = %q, want limite", got.Get("estado"))
	}
	if got.Get("n") != "51" {
		t.Errorf("n = %q, want 51 pedidas", got.Get("n"))
	}
	if fake.ran() != 0 {
		t.Fatalf("el runner corrió %d veces con 51 issues", fake.ran())
	}
	rows := rowsOf(t, h, key)
	if state := exploreStage(t, rows[1]).State; state != "" {
		t.Errorf("fila #1 = %q, want sin job: el pedido se rechazó entero", state)
	}
}

// S14: the same issue twice is one job, and a job already in the queue is not duplicated.
func TestExploreDoesNotQueueTheSameIssueTwice(t *testing.T) {
	st, key := indexedCorpus(t, 1)
	fake := newFakeStage()
	h := exploreHandler(t, st, fake.stage, nil)

	got := redirectQuery(t, post(t, h, "/explore", url.Values{"repo": {key}, "issue": {"1", "1"}}))
	if got.Get("n") != "1" {
		t.Errorf("n = %q, want 1 encolada", got.Get("n"))
	}
	waitFor(t, "la fila lista", func() bool {
		return exploreStage(t, rowsOf(t, h, key)[1]).State == jobReady
	})
	if fake.ran() != 1 {
		t.Errorf("el runner corrió %d veces, want 1", fake.ran())
	}

	// A second POST while the job runs does not queue it again.
	blocked := newFakeStage().held()
	defer blocked.release()
	h2 := exploreHandler(t, st, blocked.stage, nil)
	post(t, h2, "/explore", url.Values{"repo": {key}, "issue": {"1"}})
	waitFor(t, "la fila en curso", func() bool {
		return exploreStage(t, rowsOf(t, h2, key)[1]).State == jobRunning
	})
	if got := redirectQuery(t, post(t, h2, "/explore", url.Values{"repo": {key}, "issue": {"1"}})); got.Get("n") != "0" {
		t.Errorf("n = %q en el segundo pedido, want 0: ya está en curso", got.Get("n"))
	}
	if blocked.ran() != 1 {
		t.Errorf("el runner corrió %d veces, want 1: no se duplica lo que está en curso", blocked.ran())
	}
}

// S10: the running row carries the seconds the pipe has been silent, live.
func TestExploreShowsLiveSilence(t *testing.T) {
	st, key := indexedCorpus(t, 1)
	fake := newFakeStage().held()
	defer fake.release()
	fake.progress = []time.Duration{12 * time.Second}
	h := exploreHandler(t, st, fake.stage, nil)

	post(t, h, "/explore", url.Values{"repo": {key}, "issue": {"1"}})

	waitFor(t, "los segundos de silencio", func() bool {
		return exploreStage(t, rowsOf(t, h, key)[1]).Silence >= 12
	})
	stage := exploreStage(t, rowsOf(t, h, key)[1])
	if stage.State != jobRunning {
		t.Fatalf("fila = %q, want running mientras el silencio se informa", stage.State)
	}
	if stage.Silence < 12 {
		t.Errorf("Silence = %ds, want ≥ 12s", stage.Silence)
	}
}

// The ladder is data: today only explore can run, and the stages that do not exist say so
// instead of inventing a state (I2).
func TestRowsExposeTheLadder(t *testing.T) {
	st, key := indexedCorpus(t, 1)
	rows := rowsOf(t, exploreHandler(t, st, newFakeStage().stage, nil), key)

	stage := rows[1]
	if len(stage.Stages) != len(ladderStages) {
		t.Fatalf("la fila expone %d etapas, want %d", len(stage.Stages), len(ladderStages))
	}
	explore := exploreStage(t, stage)
	if explore.State != "" || explore.StateText != stateTextUntouched {
		t.Errorf("explore sin job = (%q, %q), want sin analizar", explore.State, explore.StateText)
	}
	for _, want := range []string{stageTriage, stageFit} {
		found := false
		for _, s := range stage.Stages {
			if s.Stage != want {
				continue
			}
			found = true
			if s.State != stageNone || s.StateText != stateTextNotApplicable {
				t.Errorf("%s = (%q, %q), want (%q, %q)", want, s.State, s.StateText, stageNone, stateTextNotApplicable)
			}
		}
		if !found {
			t.Errorf("la fila no expone la etapa %q", want)
		}
	}
}

// GET /explore/rows is routed and re-renders the same page data: the fragment exists for
// the browser to swap the <tbody> without a JSON contract.
func TestExploreRowsRouteIsWired(t *testing.T) {
	st, key := indexedCorpus(t, 1)
	h := exploreHandler(t, st, newFakeStage().stage, nil)

	rec := get(t, h, "/explore/rows?repo="+url.QueryEscape(key))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /explore/rows status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", ct)
	}

	// The route reads the corpus like the dashboard does, so an unknown repository is the
	// same honest 404 instead of an empty table.
	if rec := get(t, h, "/explore/rows?repo=no__existe"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /explore/rows de un repo desconocido status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// The real runner refuses before it can start an agent: no index, no pig. This covers the
// production stage's preflight without running pig, the network or a clone.
func TestStageRunnerRefusesBeforeStartingTheAgent(t *testing.T) {
	st, key := exploreCorpus(t, 1) // corpus without a code graph
	h := exploreHandler(t, st, nil, func(o *Options) {
		o.Explore = ExploreOptions{Model: "gemma4", Models: []string{"gemma4"}}
	})

	_, err := h.stageRunner(context.Background(), StageRequest{Repo: key, Issue: 99, Stage: stageExplore}, func(time.Duration) {})
	if err == nil || !strings.Contains(err.Error(), "gfi fetch") {
		t.Errorf("issue fuera del corpus: err = %v, want que indique gfi fetch", err)
	}

	_, err = h.stageRunner(context.Background(), StageRequest{Repo: key, Issue: 1, Stage: stageExplore}, func(time.Duration) {})
	if err == nil || !strings.Contains(err.Error(), "gfi index demo/repo") {
		t.Errorf("sin índice: err = %v, want que indique gfi index demo/repo", err)
	}
}
