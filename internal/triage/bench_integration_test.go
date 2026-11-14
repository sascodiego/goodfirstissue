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
	"syn-bug-prefix-feature",
	"syn-question-as-change",
	"syn-self-labeled",
	"httpie-800",
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
	Type        string
	Noul        float64
	Choice      string
	Confidence  float64
	Probability float64
}

var kindCriteria = map[string]string{
	"bug":      "It reports behaviour that already exists and says that behaviour is wrong or broken.",
	"feature":  "It asks for behaviour that does not exist yet.",
	"question": "It asks for information, an explanation, or how to do something, and does not ask for a change.",
	"spam":     "It contains no real request and no usable information.",
	"other":    "It describes work that fits none of the above, such as documentation, tests, or project process.",
}

var intentCriteria = map[string]string{
	"authorized": "Someone responsible for the project approved this work, said to go ahead, or merged related work.",
	"declined":   "Someone responsible said the project will not do this, or the project closed the request with the native reason not planned.",
	"neutral":    "Nothing in the material settles it either way: no approval and no refusal.",
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
		Instructions: "Can this issue be understood and started from the text alone, without asking for " +
			"context that is not here: a chat, a video, a screenshot, or a private discussion?",
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
		Instructions: "Is there enough in this text to classify the issue with grounds, as opposed to guessing " +
			"out of caution or politeness?",
	},
	{
		ID:   "asks_for_information",
		Type: "noul",
		Instructions: "Is the main purpose of the text to get an answer or an explanation, rather than to get " +
			"something changed?",
	},
	{
		ID:           "asks_for_new_behaviour",
		Type:         "noul",
		Instructions: "Does the text ask for behaviour that it also indicates does not exist today?",
	},
	{
		ID:   "describes_existing_behaviour",
		Type: "noul",
		Instructions: "Does the text describe behaviour that already exists, whether or not it calls that " +
			"behaviour a defect?",
	},
	{
		ID:   "has_reproduction",
		Type: "noul",
		Instructions: "Does the text give concrete steps, code, or output that someone else could follow to see " +
			"the reported behaviour for themselves?",
	},
}

var intentQuestions = []question{
	{ID: "intent", Type: "choice", Instructions: "What does the project's material say about doing this work?", Criteria: intentCriteria},
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
	Case   string             `json:"case"`
	Trap   string             `json:"trap"`
	Expect Expected           `json:"expect"`
	Has    map[string]float64 `json:"substance"`
	Kind   string             `json:"kind_flat"`
	Intent string             `json:"intent"`
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
	ctx := t.Context()

	var (
		runs        []benchRun
		flatHits    int
		derivedHits int
		intentHits  int
		abstentions int
		scored      int
	)
	questionHits := map[string]int{}
	questionSeen := map[string]int{}

	for _, c := range cases {
		issue := c.issue()

		if got := Prescreen(issue).Reason; got != c.ExpectPreGate {
			t.Errorf("%s: pre-screen = %q, want %q", c.ID, got, c.ExpectPreGate)
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

		noul := noulMap(substance)
		derived := DerivedKind(noul)
		run := benchRun{
			Case:   c.ID,
			Trap:   c.Trap,
			Expect: c.Expected,
			Has:    noul,
			Kind:   substance["kind"].Choice,
			Intent: intent["intent"].Choice,
		}
		runs = append(runs, run)

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

	t.Logf("")
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
