package nan_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

// assertJSONEqual compares two JSON documents regardless of key order or whitespace.
func assertJSONEqual(t *testing.T, got []byte, want string) {
	t.Helper()
	var gotAny, wantAny any
	if err := json.Unmarshal(got, &gotAny); err != nil {
		t.Fatalf("the client sent invalid JSON: %v\n%s", err, got)
	}
	if err := json.Unmarshal([]byte(want), &wantAny); err != nil {
		t.Fatalf("the expectation is invalid JSON: %v", err)
	}
	if !reflect.DeepEqual(gotAny, wantAny) {
		t.Errorf("request body mismatch\n got: %s\nwant: %s", got, want)
	}
}

// The wire shape is the contract with the API: auth header, content type, and the exact
// envelope docs/api.md documents.
func TestSystemOneSendsTheDocumentedRequest(t *testing.T) {
	var gotBody []byte
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+testKey {
			t.Errorf("Authorization = %q, want the bearer key", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"answers":{}}`))
	})

	_, err := c.SystemOne(context.Background(), nan.SystemOneRequest{
		Model: "clef",
		State: "a ticket",
		Questions: map[string]nan.Question{
			"urgent": nan.NoulQuestion("Is it urgent?"),
			"team": nan.ChoiceQuestion("Which team?", map[string]string{
				"billing":   "Payments and refunds",
				"technical": "Outages and errors",
			}),
			"severity": nan.ScoreQuestion("How severe?", []string{"None", "Minor", "Major"}),
		},
	})
	if err != nil {
		t.Fatalf("SystemOne() error = %v", err)
	}

	assertJSONEqual(t, gotBody, `{
		"model": "clef",
		"state": "a ticket",
		"questions": {
			"urgent":   {"type": "noul", "instructions": "Is it urgent?"},
			"team":     {"type": "choice", "instructions": "Which team?",
			             "criteria": {"billing": "Payments and refunds", "technical": "Outages and errors"}},
			"severity": {"type": "score", "instructions": "How severe?",
			             "criteria": ["None", "Minor", "Major"]}
		}
	}`)
}

// The envelope measured against the real API (§17.6), including the detail that a score's
// legend and probabilities are keyed by level index as strings.
func TestSystemOneDecodesTheDocumentedResponse(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"id": "abc",
			"object": "structured_decision",
			"model": "clef",
			"answers": {
				"urgent": {"type": "noul", "noul": 0.94, "confidence": 0.94,
				           "probabilities": {"yes": 0.94, "no": 0.06}},
				"team": {"type": "choice", "choice": "technical", "confidence": 0.93,
				         "probabilities": {"billing": 0.04, "technical": 0.93, "sales": 0.03}},
				"severity": {"type": "score", "score": 2.39, "confidence": 0.55,
				             "legend": {"0": "No impact", "1": "Minor", "2": "Major"},
				             "probabilities": {"0": 0.05, "1": 0.51, "2": 0.44}}
			},
			"usage": {"input_tokens": 459, "output_tokens": 0}
		}`))
	})

	got, err := c.SystemOne(context.Background(), nan.SystemOneRequest{
		Model: "clef", State: "x", Questions: sampleQuestions,
	})
	if err != nil {
		t.Fatalf("SystemOne() error = %v", err)
	}

	if got.ID != "abc" || got.Object != "structured_decision" || got.Model != "clef" {
		t.Errorf("envelope = %q/%q/%q, want abc/structured_decision/clef", got.ID, got.Object, got.Model)
	}
	if got.Usage.InputTokens != 459 || got.Usage.OutputTokens != 0 {
		t.Errorf("usage = %+v, want 459 input and 0 output", got.Usage)
	}

	noul := got.Answers["urgent"]
	if noul.Type != nan.Noul || noul.Noul != 0.94 {
		t.Errorf("urgent = %+v, want a noul answer of 0.94", noul)
	}
	if noul.Probabilities["no"] != 0.06 {
		t.Errorf("urgent probabilities = %v, want 0.06 for no", noul.Probabilities)
	}

	choice := got.Answers["team"]
	if choice.Type != nan.Choice || choice.Choice != "technical" {
		t.Errorf("team = %+v, want the technical choice", choice)
	}
	if choice.Confidence != 0.93 {
		t.Errorf("team confidence = %v, want 0.93", choice.Confidence)
	}

	score := got.Answers["severity"]
	if score.Type != nan.Score || score.Score != 2.39 {
		t.Errorf("severity = %+v, want a score answer of 2.39", score)
	}
	if score.Legend["1"] != "Minor" {
		t.Errorf("legend = %v, want the level labels keyed as strings", score.Legend)
	}
	if score.Probabilities["2"] != 0.44 {
		t.Errorf("probabilities = %v, want levels keyed as strings", score.Probabilities)
	}
}
