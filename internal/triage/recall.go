// Package triage — the read side of the journal.
//
// A screen shows what the pass decided; it does not decide anything itself. That is why the
// reading lives here and not in the caller: the key is the question (I4, I14), and a reader that
// built its own key would eventually ask a different question than the one the writer answered.
package triage

import (
	"encoding/json"
	"strings"

	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// keyFor is the journal identity of this exact question. It is the only place that assembles one,
// so the pass that writes a row and the screen that reads it cannot disagree about what they hold.
func keyFor(number int, model, commitSHA string) store.Key {
	return store.Key{
		Issue:         number,
		Pass:          store.PassTriage,
		Model:         strings.TrimSpace(model),
		PromptVersion: Version(),
		CommitSHA:     strings.TrimSpace(commitSHA),
	}
}

// Recall returns the verdict the journal holds for this exact question, together with the stored
// decision that carries when it was produced and what it cost, because a verdict without its pin
// reads as fresh.
//
// A miss is not a failure: another model, another prompt, another commit or another commit-less
// journal answers a different question, and a damaged entry is damage rather than an answer. The
// caller shows that there is nothing stored instead of presenting an empty outcome as a decision.
func Recall(journal *store.Decisions, number int, model, commitSHA string) (Analysis, store.Decision, bool) {
	if journal == nil {
		return Analysis{}, store.Decision{}, false
	}
	decision, ok := journal.Get(keyFor(number, model, commitSHA))
	if !ok {
		return Analysis{}, store.Decision{}, false
	}

	var entry stored
	if err := json.Unmarshal(decision.Verdict, &entry); err != nil {
		return Analysis{}, store.Decision{}, false
	}
	if entry.Result.Outcome == "" {
		// A stored envelope without a verdict is damage, not an answer: showing it would mean
		// presenting an empty outcome as a decision.
		return Analysis{}, store.Decision{}, false
	}

	return Analysis{
		Result:    entry.Result,
		Answers:   entry.Answers,
		Alignment: entry.Alignment,
		Cached:    true,
		At:        decision.At,
	}, decision, true
}
