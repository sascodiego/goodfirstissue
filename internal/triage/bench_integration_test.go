package triage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

// This file is the experiment, not the product: the question bank below is a candidate,
// and the winner is decided by the numbers this test prints. Nothing in the shipped
// binary calls into this file.
//
//	GFI_INTEGRATION=1 NAN_KEY=... go test ./internal/triage/ -run Bench -v
//
// The run is strictly serial: one request in flight per key, as the API requires.
// Results are written under var/bench/, which is not committed; the summary table goes
// in the issue.

// The cases a single run cannot settle: the same dossier, three times, must agree.
var consistencySubset = []string{
	"httpie-800",
	"httpie-1525",
	"cobra-2211",
	"syn-self-labeled",
}

const (
	clefModel       = "clef"
	clefTimeout     = 60 * time.Second
	clefMaxAttempts = 5
	clefRetryDelay  = time.Second
	consistencyRuns = 3
)

// answer is what comes back: a probability for a yes/no, a label plus probabilities for
// a choice.
type answer struct {
	Type          string
	Noul          float64
	Choice        string
	Confidence    float64
	Probability   float64
	Probabilities map[string]float64
}

type clefClient struct {
	url  string
	key  string
	http *http.Client
}

func newClefClient() clefClient {
	url := os.Getenv("CLEF_URL")
	if url == "" {
		url = "https://api.nan.builders/v1/systemone"
	}
	return clefClient{url: url, key: os.Getenv("NAN_KEY"), http: &http.Client{Timeout: clefTimeout}}
}

// ask sends the questions the product sends, spelled by the production type. The request
// body is not rebuilt here: a second shape of the wire format is a second contract.
func (c clefClient) ask(ctx context.Context, dossier string, questions map[string]nan.Question) (map[string]answer, error) {
	payload, err := json.Marshal(nan.SystemOneRequest{
		Model:     clefModel,
		State:     dossier,
		Questions: questions,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= clefMaxAttempts; attempt++ {
		raw, retry, err := c.post(ctx, payload)
		if err == nil {
			return decodeAnswers(raw)
		}
		lastErr = err
		if !retry {
			break
		}
		time.Sleep(clefRetryDelay)
	}
	return nil, lastErr
}

func (c clefClient) post(ctx context.Context, payload []byte) ([]byte, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return nil, false, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("call: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, false, fmt.Errorf("read body: %w", err)
	}
	switch {
	case res.StatusCode == http.StatusTooManyRequests:
		return nil, true, fmt.Errorf("429: one request in flight per key")
	case res.StatusCode != http.StatusOK:
		return nil, false, fmt.Errorf("status %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, false, nil
}

func decodeAnswers(raw []byte) (map[string]answer, error) {
	var payload nan.SystemOneResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode answers: %w", err)
	}
	out := make(map[string]answer, len(payload.Answers))
	for id, a := range payload.Answers {
		out[id] = answer{
			Type:          string(a.Type),
			Noul:          a.Noul,
			Choice:        a.Choice,
			Confidence:    a.Confidence,
			Probability:   a.Probabilities[a.Choice],
			Probabilities: a.Probabilities,
		}
	}
	return out, nil
}

func noulMap(answers map[string]answer) map[string]float64 {
	out := make(map[string]float64, len(answers))
	for id, a := range answers {
		if a.Type == "noul" {
			out[id] = a.Noul
		}
	}
	return out
}

type benchRun struct {
	Alignment   map[string]float64 `json:"alignment,omitempty"`
	TreeOutcome string             `json:"tree_outcome"`
	Closed      bool               `json:"closed_label"`
	FlatOutcome string             `json:"flat_outcome"`
	Case        string             `json:"case"`
	Trap        string             `json:"trap"`
	Expect      Expected           `json:"expect"`
	Has         map[string]float64 `json:"substance"`
	Kind        string             `json:"kind_flat"`
	KindProb    map[string]float64 `json:"kind_probabilities"`
	KindConf    float64            `json:"kind_confidence"`
	Intent      string             `json:"intent"`
	IntentProb  map[string]float64 `json:"intent_probabilities"`
	IntentConf  float64            `json:"intent_confidence"`
}

func TestBenchMeasuresWording(t *testing.T) {
	if os.Getenv("GFI_INTEGRATION") == "" {
		t.Skip("set GFI_INTEGRATION=1 to reach the decision endpoint")
	}
	if os.Getenv("NAN_KEY") == "" {
		t.Skip("set NAN_KEY to reach the decision endpoint")
	}

	cases, m := loadBench(t)
	client := newClefClient()
	briefs := loadBriefs(t)
	ctx := t.Context()

	closedByTrap := map[string][]string{}
	intentByTrap := map[string]int{}
	treeByTrap := map[string]int{}
	var (
		closed       int
		strong       int
		weak         int
		strongIntent int
		weakIntent   int
		strongTree   int
		weakTree     int
		preGated     []string
		runs         []benchRun
		flatHits     int
		intentHits   int
		abstentions  int
		scored       int
	)
	questionHits := map[string]int{}
	questionSeen := map[string]int{}

	for _, c := range cases {
		issue := c.issue()

		if got := Prescreen(issue); got.Reason != c.ExpectPreGate {
			t.Errorf("%s: pre-screen = %q, want %q", c.ID, got.Reason, c.ExpectPreGate)
		}
		if pre := Prescreen(issue); pre.Skip {
			// It never reaches a question: measuring it would measure a request the product
			// never sends.
			preGated = append(preGated, c.ID+":"+pre.Reason)
			t.Logf("%-24s resuelto sin preguntar (%s)", c.ID, pre.Reason)
			continue
		}

		started := time.Now()
		substance, err := client.ask(ctx, SubstanceDossier(issue), Questions())
		if err != nil {
			// One bad case must not cost the whole report: the benchmark is the evidence.
			t.Errorf("%s: substance request: %v", c.ID, err)
			continue
		}
		answersAt := time.Since(started)

		var alignment map[string]float64
		if substance["kind"].Choice == "feature" {
			// The tree only walks here: a feature nobody approved, whose value to the
			// project is the open question.
			project := c.Project
			if project == "" {
				project = c.issue().Repository
			}
			project = strings.ReplaceAll(project, "__", "/")
			brief, ok := briefs[project]
			if !ok {
				t.Fatalf("%s: no project brief for %q", c.ID, project)
			}
			got, err := client.ask(ctx, AlignmentDossier(issue, brief), AlignmentQuestions())
			if err != nil {
				t.Errorf("%s: alignment request: %v", c.ID, err)
			}
			alignment = noulMap(got)
		}

		noul := noulMap(substance)
		run := benchRun{
			Case:       c.ID,
			Trap:       c.Trap,
			Expect:     c.Expected,
			Has:        noul,
			Kind:       substance["kind"].Choice,
			KindProb:   substance["kind"].Probabilities,
			KindConf:   substance["kind"].Confidence,
			Alignment:  alignment,
			IntentProb: map[string]float64{},
		}
		run.Closed = c.ClosedLabel
		run.TreeOutcome = TreeOutcome(c.issue(), substance, alignment)
		run.FlatOutcome = FlatOutcome(c.issue(), substance)
		runs = append(runs, run)

		if c.ClosedLabel {
			closed++
			closedByTrap[c.Trap] = append(closedByTrap[c.Trap], c.ID)

			if run.TreeOutcome == c.Expected.Outcome {
				treeByTrap[c.Trap]++
			} else {
				t.Logf("FALLA %-22s %-28s esperaba %-12s árbol=%-12s :: %s",
					c.Trap, c.ID, c.Expected.Outcome, run.TreeOutcome, c.Issue.Title)
			}
			if c.Truth.LabelStrength == "strong" {
				strong++
				if run.Intent == c.Expected.Intent {
					strongIntent++
				}
				if run.TreeOutcome == c.Expected.Outcome {
					strongTree++
				}
			} else {
				weak++
				if run.Intent == c.Expected.Intent {
					weakIntent++
				}
				if run.TreeOutcome == c.Expected.Outcome {
					weakTree++
				}
			}
			t.Logf("%-24s cerrada como %-12s fuerza=%-6s árbol=%-12s esp=%s",
				c.ID, c.Truth.StateReason, c.Truth.LabelStrength, run.TreeOutcome, c.Expected.Outcome)
			continue
		}

		scored++
		if run.Kind == c.Expected.Kind {
			flatHits++
		}
		if run.Intent == c.Expected.Intent {
			intentHits++
		}
		if noul["evidence_sufficient"] < 0.5 {
			abstentions++
		}
		for id, want := range map[string]bool{
			"self_contained":      c.Expected.SelfContained,
			"has_evidence":        c.Expected.HasEvidence,
			"unambiguous":         c.Expected.Unambiguous,
			"done_defined":        c.Expected.DoneDefined,
			"reports_defect":      c.Expected.ReportsDefect,
			"evidence_sufficient": c.Expected.EvidenceSufficient,
		} {
			questionSeen[id]++
			if (noul[id] >= 0.5) == want {
				questionHits[id]++
			}
		}

		t.Logf("%-24s kind %-8s flat=%-8s(p=%.2f) evsuf=%.2f latency=%s",
			c.ID, c.Expected.Kind, run.Kind, substance["kind"].Probability,
			noul["evidence_sufficient"], answersAt.Round(time.Millisecond))
	}

	treeHits, flatOutcomeHits, aligned, alignedHits := 0, 0, 0, 0
	for _, run := range runs {
		if run.Closed {
			continue
		}
		if run.TreeOutcome == run.Expect.Outcome {
			treeHits++
		}
		if run.FlatOutcome == run.Expect.Outcome {
			flatOutcomeHits++
		}
		if run.Alignment != nil && run.Expect.OnlyForRequester != nil {
			aligned++
			okSelf := (run.Alignment["only_for_requester"] >= 0.5) == *run.Expect.OnlyForRequester
			okDir := run.Expect.FitsDirection == nil
			if !okDir {
				okDir = (run.Alignment["fits_direction"] >= 0.5) == *run.Expect.FitsDirection
			}
			if okSelf && okDir {
				alignedHits++
			}
		}
	}
	for _, trap := range []string{"closed_completed_merged_pr", "closed_refused_with_reason", "closed_refused_without_reason"} {
		n := len(closedByTrap[trap])
		if n == 0 {
			continue
		}
		t.Logf("%-30s n=%3d · árbol %3d/%3d · intent %3d/%3d", trap, n, treeByTrap[trap], n, intentByTrap[trap], n)
	}
	t.Logf("cerradas con etiqueta real: %d (fuertes %d · débiles %d)", closed, strong, weak)
	t.Logf("  intent  fuerte %d/%d · débil %d/%d", strongIntent, strong, weakIntent, weak)
	t.Logf("  desenlace árbol fuerte %d/%d · débil %d/%d", strongTree, strong, weakTree, weak)
	t.Logf("desenlace plano: %d/%d", flatOutcomeHits, len(runs))
	t.Logf("desenlace árbol: %d/%d", treeHits, len(runs))
	t.Logf("nodo de alineación: %d/%d", alignedHits, aligned)
	t.Logf("")
	t.Logf("resueltos sin preguntar: %d (%s)", len(preGated), strings.Join(preGated, ", "))
	t.Logf("medidos:        %d", scored)
	t.Logf("flat kind:      %d/%d", flatHits, scored)
	t.Logf("intent:         %d/%d", intentHits, scored)
	t.Logf("abstentions:    %d/%d", abstentions, scored)
	for _, id := range []string{"self_contained", "has_evidence", "unambiguous", "done_defined", "reports_defect", "evidence_sufficient"} {
		t.Logf("  %-20s %d/%d", id, questionHits[id], questionSeen[id])
	}

	reportConsistency(t, ctx, client, cases, runs)
	writeBenchReport(t, m, runs)
}

// A single run says nothing about whether the wording is stable. The subset that decides
// the product is asked again, and any disagreement is reported: a question whose answer
// moves between identical inputs is not a question, it is noise.
func reportConsistency(t *testing.T, ctx context.Context, client clefClient, cases []benchCase, first []benchRun) {
	t.Helper()

	byID := map[string]benchRun{}
	for _, run := range first {
		byID[run.Case] = run
	}
	for _, id := range consistencySubset {
		var issue Issue
		for _, c := range cases {
			if c.ID == id {
				issue = c.issue()
			}
		}
		base, ok := byID[id]
		if !ok {
			t.Errorf("%s: missing from the first pass", id)
			continue
		}
		for pass := 2; pass <= consistencyRuns; pass++ {
			again, err := client.ask(ctx, SubstanceDossier(issue), Questions())
			if err != nil {
				t.Errorf("%s: consistency request: %v", id, err)
				continue
			}

			if again["kind"].Choice != base.Kind {
				t.Errorf("%s pass %d: kind moved %q -> %q", id, pass, base.Kind, again["kind"].Choice)
			}
			t.Logf("consistency %-24s pass %d: kind=%s", id, pass, again["kind"].Choice)
		}
	}
}

func writeBenchReport(t *testing.T, m manifest, runs []benchRun) {
	t.Helper()

	dir := filepath.Join("..", "..", "var", "bench")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("create report dir: %v", err)
	}
	path := filepath.Join(dir, "report-"+strconv.FormatInt(time.Now().Unix(), 10)+".json")
	blob, err := json.MarshalIndent(map[string]any{
		"manifest": m.Version,
		"policy":   m.PolicyVersion,
		"at":       time.Now().UTC().Format(time.RFC3339),
		"runs":     runs,
	}, "", "  ")
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if err := os.WriteFile(path, append(blob, '\n'), 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}
	t.Logf("report written to %s", path)
}

// loadBriefs reads the project direction written for each repository under test. It is
// real material taken from the clone: what the README declares and what the last twenty
// commits reveal.
func loadBriefs(t *testing.T) map[string]string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(benchDir, "project-brief", "*.md"))
	if err != nil {
		t.Fatalf("glob briefs: %v", err)
	}
	briefs := make(map[string]string, len(paths))
	for _, path := range paths {
		blob, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		name := strings.TrimSuffix(filepath.Base(path), ".md")
		briefs[strings.ReplaceAll(name, "__", "/")] = string(blob)
	}
	if len(briefs) == 0 {
		t.Fatal("no project briefs: the alignment node would judge blind")
	}
	return briefs
}

// prescreen resolves the native and structural facts, and nothing else.
func prescreen(issue Issue) string {
	return Prescreen(issue).Reason
}

// FlatOutcome is the chain as frozen in criteria.md: independent gates in a fixed order.
func FlatOutcome(issue Issue, substance map[string]answer) string {
	if pre := prescreen(issue); pre != "" {
		return "MARCADA"
	}
	at := func(id string) float64 { return substance[id].Noul }
	kind := substance["kind"].Choice

	if at("evidence_sufficient") < 0.5 {
		return "DUDA"
	}
	if kind == "question" || kind == "spam" || kind == "other" {
		return "MARCADA"
	}
	if at("self_contained") < 0.5 {
		return "MARCADA"
	}
	if kind == "bug" && at("has_evidence") < 0.5 {
		return "MARCADA"
	}
	if kind == "bug" && at("reports_defect") < 0.5 {
		kind = "feature"
	}
	_ = kind
	if at("done_defined") < 0.5 || at("unambiguous") < 0.5 {
		return "CONDICIONADA"
	}
	return "CANDIDATA"
}

// TreeOutcome is the chain frozen in criteria.md, and it must stay in step with it: the
// document is what a reader trusts, this is what runs.
//
//	pre-gates → kind → bug: candidate · documentation/tests: candidate · spam: marked
//	                 · question: deferred to the second round
//	                 · feature: alignment (0.6) → candidate / conditioned / marked
func TreeOutcome(issue Issue, substance map[string]answer, alignment map[string]float64) string {
	if pre := prescreen(issue); pre != "" {
		return "MARCADA"
	}
	at := func(id string) float64 { return substance[id].Noul }
	kind := substance["kind"].Choice

	if at("evidence_sufficient") < 0.5 {
		return "DUDA"
	}
	switch kind {
	case "spam":
		return "MARCADA"
	case "question":
		// A question that turns into work is decided in the second round, with the
		// comments. Until then it is not a rejection.
		return "SEGUNDA_RONDA"
	case "bug", "other":
		// Documentation, tests and refactors are real work, and a bug is a deviation from
		// what the project already wants. Neither needs the alignment node.
		return "CANDIDATA"
	}

	// Feature: the only case that has to answer whether it belongs to the project.
	if alignment == nil {
		return "CONDICIONADA"
	}
	const threshold = 0.6
	if alignment["only_for_requester"] >= threshold {
		return "MARCADA"
	}
	if alignment["fits_direction"] >= threshold {
		return "CANDIDATA"
	}
	return "CONDICIONADA"
}
