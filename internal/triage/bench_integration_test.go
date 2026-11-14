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

// question is one typed entry of the request. The wording here is the independent
// variable of the experiment; the frozen copy lands in prompt.md once a winner exists.
type question struct {
	ID           string
	Type         string
	Instructions string
	Criteria     map[string]string
}

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

var kindCriteria = map[string]string{
	"bug":      "It reports behaviour that already exists and says that behaviour is wrong or broken.",
	"feature":  "It asks for behaviour that does not exist yet.",
	"question": "It asks for information, an explanation, or how to do something, and does not ask for a change.",
	"spam":     "It contains no real request and no usable information.",
	"other":    "It describes work that fits none of the above, such as documentation, tests, or project process.",
}

var intentCriteria = map[string]string{
	"approved":  "Someone responsible for the project asked for this work, said to go ahead with it, or merged work that does it. A label claiming acceptance counts only if it is consistent with what those people wrote.",
	"refused":   "Someone responsible for the project said it will not be done, or asked for it to be closed. A label claiming rejection counts only if it is consistent with what those people wrote.",
	"undecided": "The material records no position from anyone responsible for the project: the request simply sits there, or the only voice in favour is the person asking.",
}

// substanceQuestions is the experiment. The first seven are the analytic questions the
// design already had; the four after them are grounded nodes of the candidate tree, and
// they exist to test whether a category derived from concrete facts beats a category the
// model is asked to name in one step.
var substanceQuestions = []question{
	{ID: "kind", Type: "choice", Instructions: "What is this issue asking for?", Criteria: kindCriteria},
	{
		ID:   "self_contained",
		Type: "noul",
		Instructions: "Does this text state what is being asked clearly enough that someone who has not " +
			"spoken to the author could start, without material that is not here: a recording, a " +
			"screenshot, or a private conversation?",
	},
	{
		ID:   "has_evidence",
		Type: "noul",
		Instructions: "Does the issue contain the material someone would need to reproduce or verify the claim: " +
			"steps, a code sample, a log, a version, an environment, or a concrete use case?",
	},
	{
		ID:   "unambiguous",
		Type: "noul",
		Instructions: "Does the expected result admit exactly one reading, so that two people would agree on " +
			"what a correct outcome looks like?",
	},
	{
		ID:   "done_defined",
		Type: "noul",
		Instructions: "Does the issue define what would make it finished, so that a maintainer could accept or " +
			"reject the result without another conversation?",
	},
	{
		ID:   "reports_defect",
		Type: "noul",
		Instructions: "Does the text report behaviour that already exists as defective, rather than asking for " +
			"behaviour that does not exist yet?",
	},
	{
		ID:   "evidence_sufficient",
		Type: "noul",
		Instructions: "Is the text enough to decide what kind of request this is, regardless of how much work " +
			"the request would be or how thin or unwelcome it looks?",
	},
	{
		ID:   "asks_for_information",
		Type: "noul",
		Instructions: "Is the main purpose of the text to get an answer or an explanation, rather than to get " +
			"something changed?",
	},
	{
		ID:   "asks_for_new_behaviour",
		Type: "noul",
		Instructions: "Does the text ask the software to do something it does not do, where the thing asked " +
			"for is a capability the software was never meant to have, rather than a correction of the " +
			"behaviour that is failing?",
	},
	{
		ID:   "describes_existing_behaviour",
		Type: "noul",
		Instructions: "Does the text describe behaviour that already exists, whether or not it calls that " +
			"behaviour a defect?",
	},
	{
		ID:   "maintainer_spoke",
		Type: "noul",
		Instructions: "Does the material contain at least one statement written by someone responsible for " +
			"the project, as opposed to statements written only by people asking for it?",
	},
	{
		ID:   "has_reproduction",
		Type: "noul",
		Instructions: "Does the text give concrete steps, code, or output that someone else could follow to see " +
			"the reported behaviour for themselves?",
	},
}

// alignmentQuestions are dossier C. A feature can be clear, approved as a topic, and
// still be worth nothing to anyone but the person asking.
var alignmentQuestions = []question{
	{
		ID:   "only_for_requester",
		Type: "noul",
		Instructions: "Is this request driven by a situation specific to the person asking, such that the " +
			"project's other users would not get anything out of it?",
	},
	{
		ID:   "fits_direction",
		Type: "noul",
		Instructions: "Judged against what the project's documentation and recent history show it is trying to " +
			"be, would this request move it further in that direction, or away from it?",
	},
}

var intentQuestions = []question{
	{ID: "intent", Type: "choice", Instructions: "What position does the project record about this work?", Criteria: intentCriteria},
	{ID: "maintainer_spoke", Type: "noul", Instructions: "Does the material contain at least one statement written by someone responsible for the project, as opposed to statements written only by people asking for it?"},
}

// DerivedKind is the candidate tree: the model extracts grounded facts, our code walks
// the branches. Whether it beats asking for the category directly is the measurement.
func DerivedKind(noul map[string]float64) string {
	const half = 0.5
	if noul["asks_for_information"] >= half && noul["asks_for_new_behaviour"] < half {
		return "question"
	}
	if noul["asks_for_new_behaviour"] >= half {
		return "feature"
	}
	if noul["describes_existing_behaviour"] >= half {
		if noul["reports_defect"] >= half {
			return "bug"
		}
		return "feature"
	}
	return "other"
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

func (c clefClient) ask(ctx context.Context, dossier string, questions []question) (map[string]answer, error) {
	body := map[string]any{"model": clefModel, "state": dossier, "questions": map[string]any{}}
	entries := body["questions"].(map[string]any)
	for _, q := range questions {
		entry := map[string]any{"type": q.Type, "instructions": q.Instructions}
		if q.Criteria != nil {
			entry["criteria"] = q.Criteria
		}
		entries[q.ID] = entry
	}
	payload, err := json.Marshal(body)
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
	var payload struct {
		Answers map[string]struct {
			Type          string             `json:"type"`
			Noul          *float64           `json:"noul"`
			Choice        string             `json:"choice"`
			Confidence    float64            `json:"confidence"`
			Probabilities map[string]float64 `json:"probabilities"`
		} `json:"answers"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode answers: %w", err)
	}
	out := make(map[string]answer, len(payload.Answers))
	for id, a := range payload.Answers {
		decoded := answer{Type: a.Type, Choice: a.Choice, Confidence: a.Confidence}
		if a.Noul != nil {
			decoded.Noul = *a.Noul
		}
		decoded.Probability = a.Probabilities[a.Choice]
		decoded.Probabilities = a.Probabilities
		out[id] = decoded
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
		derivedHits  int
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
		substance, err := client.ask(ctx, SubstanceDossier(issue), substanceQuestions)
		if err != nil {
			t.Fatalf("%s: substance request: %v", c.ID, err)
		}
		answersAt := time.Since(started)

		intent, err := client.ask(ctx, IntentDossier(issue), intentQuestions)
		if err != nil {
			t.Fatalf("%s: intent request: %v", c.ID, err)
		}

		var alignment map[string]float64
		if run0 := substance["kind"]; run0.Choice == "feature" && intent["intent"].Choice == "undecided" {
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
			got, err := client.ask(ctx, AlignmentDossier(issue, brief), alignmentQuestions)
			if err != nil {
				t.Fatalf("%s: alignment request: %v", c.ID, err)
			}
			alignment = noulMap(got)
		}

		noul := noulMap(substance)
		derived := DerivedKind(noul)
		run := benchRun{
			Case:       c.ID,
			Trap:       c.Trap,
			Expect:     c.Expected,
			Has:        noul,
			Kind:       substance["kind"].Choice,
			KindProb:   substance["kind"].Probabilities,
			KindConf:   substance["kind"].Confidence,
			Alignment:  alignment,
			Intent:     intent["intent"].Choice,
			IntentProb: intent["intent"].Probabilities,
			IntentConf: intent["intent"].Confidence,
		}
		run.Closed = c.ClosedLabel
		run.TreeOutcome = TreeOutcome(c.issue(), substance, intent["intent"].Choice, alignment)
		run.FlatOutcome = FlatOutcome(c.issue(), substance, intent["intent"].Choice)
		runs = append(runs, run)

		if c.ClosedLabel {
			closed++
			if c.Closure.LabelStrength == "strong" {
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
			t.Logf("%-24s cerrada como %-12s fuerza=%-6s intent %-10s got=%-10s(p=%.2f) árbol=%-12s esp=%s",
				c.ID, c.Closure.StateReason, c.Closure.LabelStrength, c.Expected.Intent,
				run.Intent, intent["intent"].Probability, run.TreeOutcome, c.Expected.Outcome)
			continue
		}

		scored++
		if run.Kind == c.Expected.Kind {
			flatHits++
		}
		if derived == c.Expected.Kind {
			derivedHits++
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

		t.Logf("%-24s kind %-8s flat=%-8s(p=%.2f) derived=%-8s intent %-10s got=%-10s(p=%.2f) evsuf=%.2f latency=%s",
			c.ID, c.Expected.Kind, run.Kind, substance["kind"].Probability,
			derived, c.Expected.Intent, run.Intent, intent["intent"].Probability,
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
	t.Logf("derived kind:   %d/%d", derivedHits, scored)
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
			again, err := client.ask(ctx, SubstanceDossier(issue), substanceQuestions)
			if err != nil {
				t.Fatalf("%s: consistency request: %v", id, err)
			}
			againIntent, err := client.ask(ctx, IntentDossier(issue), intentQuestions)
			if err != nil {
				t.Fatalf("%s: consistency intent request: %v", id, err)
			}
			if again["kind"].Choice != base.Kind {
				t.Errorf("%s pass %d: kind moved %q -> %q", id, pass, base.Kind, again["kind"].Choice)
			}
			if againIntent["intent"].Choice != base.Intent {
				t.Errorf("%s pass %d: intent moved %q -> %q", id, pass, base.Intent, againIntent["intent"].Choice)
			}
			t.Logf("consistency %-24s pass %d: kind=%s intent=%s", id, pass, again["kind"].Choice, againIntent["intent"].Choice)
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
func FlatOutcome(issue Issue, substance map[string]answer, intent string) string {
	if pre := prescreen(issue); pre != "" {
		return "MARCADA"
	}
	at := func(id string) float64 { return substance[id].Noul }
	kind := substance["kind"].Choice

	if at("evidence_sufficient") < 0.5 {
		return "no_verdict"
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
	if at("done_defined") < 0.5 || at("unambiguous") < 0.5 {
		return "CONDICIONADA"
	}
	if kind == "feature" && intent == "undecided" {
		return "CONDICIONADA"
	}
	if intent == "refused" {
		return "MARCADA"
	}
	return "CANDIDATA"
}

// TreeOutcome is the tree the owner proposed: approval ends the walk, refusal ends it,
// and only a feature nobody decided on reaches the alignment node, where the question is
// whether the request serves the project or only the person asking.
func TreeOutcome(issue Issue, substance map[string]answer, intent string, alignment map[string]float64) string {
	if pre := prescreen(issue); pre != "" {
		return "MARCADA"
	}
	at := func(id string) float64 { return substance[id].Noul }
	kind := substance["kind"].Choice

	if intent == "approved" {
		return "CANDIDATA"
	}
	if intent == "refused" {
		return "MARCADA"
	}
	if at("evidence_sufficient") < 0.5 {
		return "no_verdict"
	}
	if kind == "question" || kind == "spam" || kind == "other" {
		return "MARCADA"
	}
	if kind == "bug" {
		if at("has_evidence") < 0.5 {
			return "MARCADA"
		}
		return "CANDIDATA"
	}
	// Feature with no recorded position.
	if alignment == nil {
		return "CONDICIONADA"
	}
	if alignment["only_for_requester"] >= 0.5 {
		return "MARCADA"
	}
	if alignment["fits_direction"] >= 0.5 {
		return "CANDIDATA"
	}
	return "CONDICIONADA"
}
