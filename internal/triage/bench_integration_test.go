package triage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

// This file is the experiment, not the product: the case bank below is hand-labelled, and the
// winner is decided by the numbers this test prints. Nothing in the shipped binary calls into
// this file.
//
//	GFI_INTEGRATION=1 NAN_KEY=... go test ./internal/triage/ -run Bench -v
//
// The run goes through the production pipeline — the same questions, the same request, the
// same policy — because a second copy of the chain would be a copy that could disagree with
// the product (S2). The client keeps one request in flight per key, as the API requires.
// Results are written under var/bench/, which is not committed; the summary table goes in the
// issue.

// The cases a single run cannot settle: the same dossier, three times, must agree.
var consistencySubset = []string{
	"httpie-800",
	"httpie-1525",
	"cobra-2211",
	"syn-self-labeled",
}

const (
	// benchModel is the decision model. It is spelled here because the benchmark is not the
	// product: the product reads it from the constitution (I13), the experiment names it.
	benchModel       = "clef"
	benchTimeout     = 60 * time.Second
	benchMaxAttempts = 5
	consistencyRuns  = 3
)

// benchRun is one case as the report keeps it: what the judge said, what the policy decided,
// and the label it is measured against.
type benchRun struct {
	Answers  map[string]nan.Answer `json:"answers,omitempty"`
	Case     string                `json:"case"`
	Trap     string                `json:"trap"`
	Expect   Expected              `json:"expect"`
	Kind     string                `json:"kind"`
	KindProb map[string]float64    `json:"kind_probabilities,omitempty"`
	Outcome  string                `json:"outcome"`
	Reason   string                `json:"reason,omitempty"`
	Missing  []string              `json:"missing,omitempty"`
	KindConf float64               `json:"kind_confidence"`
	Closed   bool                  `json:"closed_label"`
}

// why renders the policy's explanation for a verdict. A mark or a doubt carries a reason, a
// condition carries the list of what would settle it, and the report shows whichever exists:
// a CONDICIONADA printed as "sin razón" hides the only thing the user can act on.
func why(res Result) string {
	switch {
	case res.Reason != "" && len(res.Missing) > 0:
		return res.Reason + " falta=" + strings.Join(res.Missing, ",")
	case res.Reason != "":
		return res.Reason
	case len(res.Missing) > 0:
		return "falta=" + strings.Join(res.Missing, ",")
	}
	return "sin explicación"
}

func TestBenchMeasuresWording(t *testing.T) {
	if os.Getenv("GFI_INTEGRATION") == "" {
		t.Skip("set GFI_INTEGRATION=1 to reach the decision endpoint")
	}
	if os.Getenv("NAN_KEY") == "" {
		t.Skip("set NAN_KEY to reach the decision endpoint")
	}

	cases, m := loadBench(t)
	client, err := nan.New(nan.Options{
		APIKey:       os.Getenv("NAN_KEY"),
		Timeout:      benchTimeout,
		SystemOneURL: os.Getenv("CLEF_URL"), // empty takes the documented default
		MaxAttempts:  benchMaxAttempts,
	})
	if err != nil {
		t.Fatalf("build the decision client: %v", err)
	}
	p1, err := New(Options{
		Judge:     ClientJudge{Client: client, Model: benchModel},
		Model:     benchModel,
		CommitSHA: resolveCommit(t),
		// Nil disables the journal: a cached verdict would hide the request this test counts.
		Journal: nil,
	})
	if err != nil {
		t.Fatalf("build the first pass: %v", err)
	}
	ctx := t.Context()

	closedByTrap := map[string][]string{}
	outcomeByTrap := map[string]int{}
	byReason := map[string]int{}
	var (
		closed      int
		strong      int
		weak        int
		strongHits  int
		weakHits    int
		preGated    []string
		runs        []benchRun
		kindHits    int
		abstentions int
		scored      int
		outcomeHits int
	)
	questionHits := map[string]int{}
	questionSeen := map[string]int{}

	for _, c := range cases {
		issue := c.issue()

		if got := Prescreen(issue); got.Reason != c.ExpectPreGate {
			t.Errorf("%s: pre-screen = %q, want %q", c.ID, got.Reason, c.ExpectPreGate)
		}
		if pre := Prescreen(issue); pre.Skip {
			// The production pass still runs, because that is what proves the pre-gate resolves
			// the issue before a question is asked. It never reaches a question, so measuring it
			// would measure a request the product never sends.
			analysis, err := p1.Run(ctx, issue)
			if err != nil {
				t.Errorf("%s: pre-gated run: %v", c.ID, err)
				continue
			}
			res := analysis.Result
			if res.Outcome != Marked {
				t.Errorf("%s: pre-gated outcome = %q, want %q", c.ID, res.Outcome, Marked)
			}
			if res.Reason == "" {
				t.Errorf("%s: a pre-gated verdict must name the fact that resolved it", c.ID)
			}
			if len(analysis.Answers) != 0 {
				t.Errorf("%s: a pre-gated verdict spent a request: %d answers", c.ID, len(analysis.Answers))
			}
			preGated = append(preGated, c.ID+":"+res.Reason)
			byReason[res.Reason]++
			t.Logf("%-24s resuelto sin preguntar (%s)", c.ID, res.Reason)
			continue
		}

		started := time.Now()
		analysis, err := p1.Run(ctx, issue)
		if err != nil {
			// One bad case must not cost the whole report: the benchmark is the evidence.
			t.Errorf("%s: triage request: %v", c.ID, err)
			continue
		}
		answersAt := time.Since(started)

		// The production answers and the production verdict are what gets scored. Nothing is
		// recomputed here: a second chain would be a second contract.
		kind := analysis.Answers[QKind].Choice
		noul := noulValues(analysis.Answers)
		res := analysis.Result
		run := benchRun{
			Case:     c.ID,
			Trap:     c.Trap,
			Expect:   c.Expected,
			Closed:   c.ClosedLabel,
			Answers:  analysis.Answers,
			Kind:     kind,
			KindProb: analysis.Answers[QKind].Probabilities,
			KindConf: analysis.Answers[QKind].Confidence,
			Outcome:  string(res.Outcome),
			Reason:   res.Reason,
			Missing:  SortedMissing(res),
		}
		runs = append(runs, run)

		if run.Outcome == c.Expected.Outcome {
			outcomeHits++
		}
		reason := why(res)
		byReason[reason]++

		if c.ClosedLabel {
			closed++
			closedByTrap[c.Trap] = append(closedByTrap[c.Trap], c.ID)

			if run.Outcome == c.Expected.Outcome {
				outcomeByTrap[c.Trap]++
			} else {
				t.Logf("FALLA %-22s %-28s esperaba %-12s obtuvo %-12s %-40s :: %s",
					c.Trap, c.ID, c.Expected.Outcome, run.Outcome, reason, c.Issue.Title)
			}
			if c.Truth.LabelStrength == "strong" {
				strong++
				if run.Outcome == c.Expected.Outcome {
					strongHits++
				}
			} else {
				weak++
				if run.Outcome == c.Expected.Outcome {
					weakHits++
				}
			}
			t.Logf("%-24s cerrada como %-12s fuerza=%-6s obtuvo=%-12s esp=%s",
				c.ID, c.Truth.StateReason, c.Truth.LabelStrength, run.Outcome, c.Expected.Outcome)
			continue
		}

		scored++
		if kind == c.Expected.Kind {
			kindHits++
		}
		if noul[QEvidenceSufficient] < threshold {
			abstentions++
		}
		for id, want := range map[string]bool{
			QSelfContained:      c.Expected.SelfContained,
			QHasEvidence:        c.Expected.HasEvidence,
			QUnambiguous:        c.Expected.Unambiguous,
			QDoneDefined:        c.Expected.DoneDefined,
			QReportsDefect:      c.Expected.ReportsDefect,
			QEvidenceSufficient: c.Expected.EvidenceSufficient,
		} {
			questionSeen[id]++
			if (noul[id] >= threshold) == want {
				questionHits[id]++
			}
		}

		t.Logf("%-24s kind %-8s obtuvo=%-8s(p=%.2f) evsuf=%.2f desenlace=%-12s %-32s latency=%s",
			c.ID, c.Expected.Kind, kind, analysis.Answers[QKind].Confidence,
			noul[QEvidenceSufficient], run.Outcome, reason, answersAt.Round(time.Millisecond))
	}

	for _, trap := range []string{"closed_completed_merged_pr", "closed_refused_with_reason", "closed_refused_without_reason"} {
		n := len(closedByTrap[trap])
		if n == 0 {
			continue
		}
		t.Logf("%-30s n=%3d · obtuvo %3d/%3d", trap, n, outcomeByTrap[trap], n)
	}
	t.Logf("cerradas con etiqueta real: %d (fuertes %d · débiles %d)", closed, strong, weak)
	t.Logf("  obtuvo fuerte %d/%d · débil %d/%d", strongHits, strong, weakHits, weak)
	t.Logf("desenlace: %d/%d", outcomeHits, len(runs))
	for _, key := range sortedReasons(byReason) {
		t.Logf("  %-28s %d", key, byReason[key])
	}
	t.Logf("")
	t.Logf("resueltos sin preguntar: %d (%s)", len(preGated), strings.Join(preGated, ", "))
	t.Logf("medidos:        %d", scored)
	t.Logf("kind:           %d/%d", kindHits, scored)
	t.Logf("abstentions:    %d/%d", abstentions, scored)
	for _, id := range []string{QSelfContained, QHasEvidence, QUnambiguous, QDoneDefined, QReportsDefect, QEvidenceSufficient} {
		t.Logf("  %-20s %d/%d", id, questionHits[id], questionSeen[id])
	}

	reportConsistency(t, ctx, p1, cases, runs)
	writeBenchReport(t, m, runs)
}

// noulValues keeps the yes/no answers by question id: that is the shape the hand labels are
// scored against.
func noulValues(answers map[string]nan.Answer) map[string]float64 {
	out := make(map[string]float64, len(answers))
	for id, a := range answers {
		if a.Type == nan.Noul {
			out[id] = a.Noul
		}
	}
	return out
}

// sortedReasons puts the reason breakdown in a stable order, so two runs of the report can be
// compared line by line.
func sortedReasons(byReason map[string]int) []string {
	out := make([]string, 0, len(byReason))
	for reason := range byReason {
		out = append(out, reason)
	}
	sort.Strings(out)
	return out
}

// resolveCommit pins the verdict to the tree it was produced from (I4). The journal is nil in
// this test, so a failure is not fatal: an unpinned verdict is still a measurement, and the log
// says why it is unpinned.
func resolveCommit(t *testing.T) string {
	t.Helper()

	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Logf("git rev-parse HEAD failed (%v): the verdict will not be pinned to a commit", err)
		return ""
	}
	return strings.TrimSpace(string(out))
}

// A single run says nothing about whether the wording is stable. The subset that decides the
// product is asked again through the same first pass, and any disagreement is reported: a
// question whose answer moves between identical inputs is not a question, it is noise.
func reportConsistency(t *testing.T, ctx context.Context, p1 *P1, cases []benchCase, first []benchRun) {
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
			again, err := p1.Run(ctx, issue)
			if err != nil {
				t.Errorf("%s: consistency request: %v", id, err)
				continue
			}

			res := again.Result
			if moved := movedQuestions(base.Answers, again.Answers); len(moved) > 0 {
				t.Errorf("%s pass %d: answers moved: %s", id, pass, strings.Join(moved, ", "))
			}
			if base.Outcome != string(res.Outcome) {
				t.Errorf("%s pass %d: outcome moved %q -> %q", id, pass, base.Outcome, res.Outcome)
			}
			t.Logf("consistency %-24s pass %d: kind=%s desenlace=%s",
				id, pass, again.Answers[QKind].Choice, res.Outcome)
		}
	}
}

// movedQuestions names the questions whose answer differs between two identical runs. A noul
// answer moves when its probability crosses the cut the policy uses — a probability that drifts
// without crossing it decides the same thing, and the policy never sees the decimal. A choice
// answer moves when it picks another label.
func movedQuestions(base, again map[string]nan.Answer) []string {
	var moved []string
	for id, a := range base {
		b, ok := again[id]
		if !ok {
			moved = append(moved, id+" (missing)")
			continue
		}
		switch a.Type {
		case nan.Noul:
			if (a.Noul >= threshold) != (b.Noul >= threshold) {
				moved = append(moved, fmt.Sprintf("%s: %.2f -> %.2f", id, a.Noul, b.Noul))
			}
		case nan.Choice:
			if a.Choice != b.Choice {
				moved = append(moved, fmt.Sprintf("%s: %q -> %q", id, a.Choice, b.Choice))
			}
		}
	}
	sort.Strings(moved)
	return moved
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
