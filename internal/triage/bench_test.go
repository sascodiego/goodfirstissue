package triage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// The benchmark is the answer to the one criticism the design could not survive: the
// wording was chosen by opinion. These cases are hand-labelled before any measurement,
// so that the wording is judged against a verdict and not against itself.

const benchDir = "testdata/bench"

type benchSource struct {
	Repository string `json:"repository"`
	Number     int    `json:"number"`
	URL        string `json:"url"`
}

type benchComment struct {
	Author            string `json:"author"`
	Body              string `json:"body"`
	AuthorAssociation string `json:"author_association"`
}

type benchLinkedPR struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	Merged bool   `json:"merged"`
}

type benchIssue struct {
	Title             string          `json:"title"`
	Body              string          `json:"body"`
	Comments          []benchComment  `json:"comments"`
	Labels            []string        `json:"labels"`
	GitHubState       string          `json:"github_state"`
	GitHubStateReason string          `json:"github_state_reason"`
	LinkedPRs         []benchLinkedPR `json:"linked_prs"`
}

// Expected is the verdict a human assigned before the wording existed. It is data, not
// policy: the gate chain lives in the policy and is implemented elsewhere.
type Expected struct {
	Kind               string `json:"kind"`
	Intent             string `json:"intent"`
	SelfContained      bool   `json:"self_contained"`
	HasEvidence        bool   `json:"has_evidence"`
	Unambiguous        bool   `json:"unambiguous"`
	DoneDefined        bool   `json:"done_defined"`
	ReportsDefect      bool   `json:"reports_defect"`
	EvidenceSufficient bool   `json:"evidence_sufficient"`
	Outcome            string `json:"outcome"`
}

type benchDuplicate struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type benchCase struct {
	ID            string           `json:"id"`
	Provenance    string           `json:"provenance"`
	Trap          string           `json:"trap"`
	Source        *benchSource     `json:"source"`
	Issue         benchIssue       `json:"issue"`
	Duplicates    []benchDuplicate `json:"duplicates"`
	Expected      Expected         `json:"expected"`
	ExpectPreGate string           `json:"expect_pre_gate"`
}

type coverage struct {
	Trap string `json:"trap"`
	Min  int    `json:"min"`
	Why  string `json:"why"`
}

type provenanceFloor struct {
	Real      int `json:"real"`
	Synthetic int `json:"synthetic"`
}

type manifest struct {
	Version            string          `json:"version"`
	PolicyVersion      string          `json:"policy_version"`
	Purpose            string          `json:"purpose"`
	RequiredCoverage   []coverage      `json:"required_coverage"`
	RequiredProvenance provenanceFloor `json:"required_provenance"`
	Cases              []string        `json:"cases"`
}

func loadBench(t *testing.T) ([]benchCase, manifest) {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(benchDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	paths, err := filepath.Glob(filepath.Join(benchDir, "cases", "*.json"))
	if err != nil {
		t.Fatalf("glob cases: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no cases found: the benchmark would measure nothing")
	}

	cases := make([]benchCase, 0, len(paths))
	for _, path := range paths {
		blob, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var c benchCase
		if err := json.Unmarshal(blob, &c); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		cases = append(cases, c)
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].ID < cases[j].ID })
	return cases, m
}

func (c benchCase) issue() Issue {
	issue := Issue{
		Number:            c.Source.NumberOrZero(),
		Title:             c.Issue.Title,
		Body:              c.Issue.Body,
		Labels:            c.Issue.Labels,
		GitHubState:       c.Issue.GitHubState,
		GitHubStateReason: c.Issue.GitHubStateReason,
	}
	if c.Source != nil {
		issue.Repository = c.Source.Repository
	}
	for _, comment := range c.Issue.Comments {
		issue.Comments = append(issue.Comments, Comment(comment))
	}
	for _, pr := range c.Issue.LinkedPRs {
		issue.LinkedPRs = append(issue.LinkedPRs, LinkedPR(pr))
	}
	return issue
}

func (s *benchSource) NumberOrZero() int {
	if s == nil {
		return 0
	}
	return s.Number
}

// Every case has to be classifiable by the schema, or the measurement runs on garbage.
func TestBenchCasesAreComplete(t *testing.T) {
	cases, _ := loadBench(t)

	kinds := map[string]bool{"bug": true, "feature": true, "question": true, "spam": true, "other": true}
	intents := map[string]bool{"approved": true, "undecided": true, "refused": true}
	outcomes := map[string]bool{"CANDIDATA": true, "CONDICIONADA": true, "MARCADA": true, "no_verdict": true}
	seen := map[string]bool{}

	for _, c := range cases {
		if seen[c.ID] {
			t.Errorf("duplicate case id %q", c.ID)
		}
		seen[c.ID] = true

		if c.Issue.Title == "" {
			t.Errorf("%s: empty title", c.ID)
		}
		if !kinds[c.Expected.Kind] {
			t.Errorf("%s: expected kind %q is not one of the five", c.ID, c.Expected.Kind)
		}
		if !intents[c.Expected.Intent] {
			t.Errorf("%s: expected intent %q is not one of the three", c.ID, c.Expected.Intent)
		}
		if !outcomes[c.Expected.Outcome] {
			t.Errorf("%s: expected outcome %q is not one of the four", c.ID, c.Expected.Outcome)
		}
		switch c.Provenance {
		case "real":
			if c.Source == nil || c.Source.Repository == "" {
				t.Errorf("%s: a real case must name its source", c.ID)
			}
		case "synthetic":
			if c.Source != nil {
				t.Errorf("%s: a synthetic case must not claim a source", c.ID)
			}
		default:
			t.Errorf("%s: provenance %q must be real or synthetic", c.ID, c.Provenance)
		}
	}
}

// A trap that is listed but has no case is worse than no trap at all: it looks covered.
func TestBenchManifestCoverage(t *testing.T) {
	cases, m := loadBench(t)

	onDisk := make([]string, 0, len(cases))
	for _, c := range cases {
		onDisk = append(onDisk, c.ID)
	}
	if want, got := m.Cases, onDisk; len(want) != len(got) {
		t.Fatalf("manifest lists %d cases, %d on disk", len(want), len(got))
	} else {
		for i := range want {
			if want[i] != got[i] {
				t.Fatalf("manifest case %d is %q, disk has %q", i, want[i], got[i])
			}
		}
	}

	byTrap := map[string]int{}
	byProvenance := map[string]int{}
	for _, c := range cases {
		byTrap[c.Trap]++
		byProvenance[c.Provenance]++
	}
	for _, required := range m.RequiredCoverage {
		if byTrap[required.Trap] < required.Min {
			t.Errorf("trap %q has %d cases, the manifest requires %d: %s",
				required.Trap, byTrap[required.Trap], required.Min, required.Why)
		}
	}
	if byProvenance["real"] < m.RequiredProvenance.Real {
		t.Errorf("real cases: %d, the manifest requires %d", byProvenance["real"], m.RequiredProvenance.Real)
	}
	if byProvenance["synthetic"] < m.RequiredProvenance.Synthetic {
		t.Errorf("synthetic cases: %d, the manifest requires %d", byProvenance["synthetic"], m.RequiredProvenance.Synthetic)
	}
}

// The native pre-gates are the cheapest part of the chain and the only part that can be
// wrong with no model involved at all, so the corpus pins them against real data.
func TestBenchPreGatesMatchTheLabelledCases(t *testing.T) {
	cases, _ := loadBench(t)

	for _, c := range cases {
		got := Prescreen(c.issue())
		if got.Reason != c.ExpectPreGate {
			t.Errorf("%s: pre-screen = %q, want %q", c.ID, got.Reason, c.ExpectPreGate)
		}
		if got.Skip != (c.ExpectPreGate != "") {
			t.Errorf("%s: skip = %v with reason %q", c.ID, got.Skip, c.ExpectPreGate)
		}
	}
}

// The cases the model cannot be trusted with are also the ones a single run cannot
// settle: the same dossier three times must give the same answer.
func TestBenchConsistencySubsetExists(t *testing.T) {
	cases, _ := loadBench(t)
	seen := map[string]bool{}
	for _, c := range cases {
		seen[c.ID] = true
	}
	for _, id := range consistencySubset {
		if !seen[id] {
			t.Errorf("consistency subset names %q, which is not in the corpus", id)
		}
	}
}

// DerivedKind is the candidate tree: grounded nodes combined by our code, never by the
// model. Whether it beats the single classification question is what the benchmark
// measures; this test only pins the rule.
func TestDerivedKindFollowsTheTree(t *testing.T) {
	tests := []struct {
		name string
		noul map[string]float64
		want string
	}{
		{"asking for information", map[string]float64{"asks_for_information": 0.9, "asks_for_new_behaviour": 0.1}, "question"},
		{"asking for new behaviour", map[string]float64{"asks_for_information": 0.2, "asks_for_new_behaviour": 0.8}, "feature"},
		{"existing behaviour called defective", map[string]float64{"asks_for_new_behaviour": 0.1, "describes_existing_behaviour": 0.9, "reports_defect": 0.85}, "bug"},
		{"existing behaviour, no defect claim", map[string]float64{"asks_for_new_behaviour": 0.1, "describes_existing_behaviour": 0.8, "reports_defect": 0.2}, "feature"},
		{"nothing grounded", map[string]float64{"asks_for_information": 0.4, "asks_for_new_behaviour": 0.4, "describes_existing_behaviour": 0.3}, "other"},
		{"a question that also asks for a change", map[string]float64{"asks_for_information": 0.8, "asks_for_new_behaviour": 0.7}, "feature"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DerivedKind(tt.noul); got != tt.want {
				t.Errorf("DerivedKind(%v) = %q, want %q", tt.noul, got, tt.want)
			}
		})
	}
}
