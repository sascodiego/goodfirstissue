package web

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/config"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
	"github.com/sascodiego/nan-hackathon-clef/internal/triage"
)

// seedVerdict leaves in the journal the verdict a pass produced, under the exact identity the
// screen is going to look it up with. It is the same key triage.keyFor assembles: if the two
// drift apart the screen stops seeing what the pass stored, and the coherence test is the one
// that has to catch that, not a demo.
func seedVerdict(t *testing.T, st *store.Store, number int, result triage.Result) {
	t.Helper()

	journal, err := st.Decisions("spf13__cobra")
	if err != nil {
		t.Fatalf("journal: %v", err)
	}

	body, err := json.Marshal(struct {
		Result triage.Result `json:"result"`
	}{Result: result})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	err = journal.Put(store.Decision{
		Key: store.Key{
			Issue:         number,
			Pass:          store.PassTriage,
			Model:         config.DecisionModel,
			PromptVersion: triage.Version(),
			CommitSHA:     testCommit,
		},
		At:        time.Date(2026, 11, 14, 7, 5, 0, 0, time.UTC),
		LatencyMs: 900,
		Verdict:   body,
	})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
}

// The screen shows the verdict the journal holds. Before this, the corpus could be triaged whole
// and the dashboard still said «sin triajar» on every row: the issue screen was reading the
// journal and the dashboard was not, which is one record and two stories.
func TestTheRowShowsTheVerdictTheJournalHolds(t *testing.T) {
	st := corpus(t)
	seedVerdict(t, st, 2568, triage.Result{Outcome: triage.Candidate, Probability: 0.82})

	// state=all: este test mira los estados de fila, no cual es el default de la pantalla.
	page := body(t, newHandler(t, st), "/?repo=spf13__cobra&state=all")

	if !strings.Contains(page, ">candidata<") {
		t.Error("la fila no muestra el veredicto que el journal tiene guardado")
	}
	// Three issues in the fixture and one verdict stored: the other two are honestly untriaged.
	if got := strings.Count(page, ">sin triajar<"); got != 2 {
		t.Errorf("quedan %d filas sin triajar, se triajeó una de tres: quiero 2", got)
	}
}

// A verdict that does not recommend keeps its own word. Rendering four different outcomes with
// the same label would be one label, not four outcomes.
func TestEachVerdictKeepsItsOwnWord(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   triage.Result
		want string
	}{
		{"candidate", triage.Result{Outcome: triage.Candidate, Probability: 0.82}, ">candidata<"},
		{"conditional", triage.Result{Outcome: triage.Conditional, Reason: triage.ReasonAskMaintainer, Probability: 0.7}, ">condicionada<"},
		{"derived", triage.Result{Outcome: triage.Derived, Reason: triage.ReasonSecondRoundPending, Probability: 0.9}, ">derivada<"},
		{"marked", triage.Result{Outcome: triage.Marked, Reason: triage.ReasonNotCodeWork, Probability: 1}, ">marcada<"},
		{"undecided", triage.Result{Outcome: triage.Undecided, Reason: triage.ReasonEndpointFailed}, ">sin veredicto<"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := corpus(t)
			seedVerdict(t, st, 2568, tc.in)

			page := body(t, newHandler(t, st), "/?repo=spf13__cobra")
			if !strings.Contains(page, tc.want) {
				t.Errorf("la fila no dice %s", tc.want)
			}
		})
	}
}

// The word the demo already says stays the word for it: an issue taken by a pull request reads
// «tomada», whether the fact came from the snapshot or from a pre-gate that saw the same PR.
func TestAlreadyWorkingStillReadsAsTaken(t *testing.T) {
	st := corpus(t)
	seedVerdict(t, st, 2568, triage.Result{Outcome: triage.Marked, Reason: triage.ReasonAlreadyWorking, Probability: 1})

	data, err := buildPageWith(st, url.Values{"repo": {"spf13__cobra"}}, nil)
	if err != nil {
		t.Fatalf("buildPageWith() error = %v", err)
	}

	for _, row := range data.Rows {
		if row.Number != 2568 {
			continue
		}
		if row.StateText != "tomada" {
			t.Errorf("la fila tomada dice %q y la demo dice «tomada»", row.StateText)
		}
	}
}

// The funnel counts what the journal holds, and only that. A count the screen cannot derive from
// the record is a number the screen invented.
func TestTheFunnelCountsTheJournal(t *testing.T) {
	st := corpus(t)
	seedVerdict(t, st, 2568, triage.Result{Outcome: triage.Candidate, Probability: 0.82})
	seedVerdict(t, st, 1391, triage.Result{Outcome: triage.Marked, Reason: triage.ReasonAlreadyWorking, Probability: 1})
	seedVerdict(t, st, 1995, triage.Result{Outcome: triage.Derived, Reason: triage.ReasonSecondRoundPending, Probability: 0.9})

	data, err := buildPageWith(st, url.Values{"repo": {"spf13__cobra"}}, nil)
	if err != nil {
		t.Fatalf("buildPageWith() error = %v", err)
	}

	if data.Stats.Candidate != 1 || data.Stats.Marked != 1 || data.Stats.Derived != 1 {
		t.Errorf("el embudo cuenta candidatas=%d marcadas=%d derivadas=%d, guardadas 1/1/1",
			data.Stats.Candidate, data.Stats.Marked, data.Stats.Derived)
	}
	if data.Stats.Untriaged != 0 {
		t.Errorf("el embudo dice %d sin triajar sobre un corpus triajeado entero", data.Stats.Untriaged)
	}
	if got, want := funnelTotal(data.Stats), data.Total; got != want {
		t.Errorf("los cajones del embudo suman %d y el corpus tiene %d: hay una issue en cero cajones o en dos", got, want)
	}
}

// funnelTotal suma los cajones del embudo. Si no da el total del corpus, alguna issue se contó dos
// veces o no se contó ninguna, y el número que muestra la cabecera deja de significar algo.
func funnelTotal(s statsView) int {
	return s.Marked + s.Derived + s.Conditional + s.Candidate + s.Doubt + s.Undecided + s.Untriaged
}

// One record, one story. The pass writes with the key triage.keyFor assembles and the screen reads
// with triage.Recall; this runs the real first pass and then asks the screen, so a drift between
// the two keys —the one thing that would make the screen quietly blind— fails here instead of on a
// stage. No network: the judge is a recorded answer.
func TestWhatThePassWritesIsWhatTheScreenReads(t *testing.T) {
	st := corpus(t)
	seedTriage(t, st)

	data, err := buildPageWith(st, url.Values{"repo": {"spf13__cobra"}}, nil)
	if err != nil {
		t.Fatalf("buildPageWith() error = %v", err)
	}

	// The real pass answered kind=bug with enough evidence, so the chain recommends it.
	var found bool
	for _, row := range data.Rows {
		if row.Number != 2568 {
			continue
		}
		found = true
		if row.StateText != "candidata" {
			t.Errorf("la pasada guardó candidata y la fila dice %q", row.StateText)
		}
	}
	if !found {
		t.Fatal("la issue que la pasada triajeó no está en la página")
	}

	if data.Stats.Candidate != 1 {
		t.Errorf("el embudo cuenta %d candidatas y la pasada dejó una", data.Stats.Candidate)
	}
	if got, want := funnelTotal(data.Stats), data.Total; got != want {
		t.Errorf("el embudo suma %d sobre un corpus de %d", got, want)
	}
}

// Without the record the screen goes back to saying it does not know, which is what it said
// before and what it has to keep saying: a screen that guesses is worse than one that admits.
func TestWithoutTheJournalTheFunnelSaysItDoesNotKnow(t *testing.T) {
	data, err := buildPageWith(corpus(t), url.Values{"repo": {"spf13__cobra"}}, nil)
	if err != nil {
		t.Fatalf("buildPageWith() error = %v", err)
	}

	if data.Stats.Untriaged != 3 {
		t.Errorf("sin journal el embudo dice %d sin triajar, hay 3 issues", data.Stats.Untriaged)
	}
	if data.Stats.Candidate+data.Stats.Marked+data.Stats.Derived+data.Stats.Conditional+data.Stats.Doubt+data.Stats.Undecided != 0 {
		t.Error("sin journal el embudo afirma veredictos que nadie produjo")
	}
}

// The funnel reaches the page. A model that counts and a template that does not print it is half
// the work, and the half that the demo does not see.
func TestTheFunnelReachesThePage(t *testing.T) {
	st := corpus(t)
	seedVerdict(t, st, 2568, triage.Result{Outcome: triage.Candidate, Probability: 0.82})

	page := body(t, newHandler(t, st), "/?repo=spf13__cobra")
	for _, want := range []string{"candidatas", "marcadas", "derivadas"} {
		if !strings.Contains(page, want) {
			t.Errorf("la cabecera no muestra la columna %q", want)
		}
	}
}

// El corpus abre en lo que el triaje recomendo: quien entra quiere ver las candidatas, no las
// ciento cincuenta filas con su estado. Pero el default no puede vaciar la pantalla, asi que
// necesita que haya al menos una, y `state=all` sigue devolviendo todo.
func TestTheCorpusOpensOnTheCandidates(t *testing.T) {
	st := corpus(t)
	seedVerdict(t, st, 2568, triage.Result{Outcome: triage.Candidate, Probability: 0.82})

	// Las filas se distinguen por su clase: el embudo tambien imprime «sin triajar» como
	// etiqueta de columna, asi que buscar el texto contaria la cabecera.
	opening := body(t, newHandler(t, st), "/?repo=spf13__cobra")
	if !strings.Contains(opening, `class="st candidate"`) {
		t.Error("la pantalla no abrio en las candidatas")
	}
	if strings.Contains(opening, `class="st pending"`) {
		t.Error("la pantalla abrio en el corpus entero y no en lo que el triaje recomendo")
	}

	all := body(t, newHandler(t, st), "/?repo=spf13__cobra&state=all")
	if !strings.Contains(all, `class="st pending"`) {
		t.Error("state=all no devuelve el corpus entero")
	}

	// Sin triaje el default no se aplica: mostrar cero filas no es un default, es un error.
	empty := body(t, newHandler(t, corpus(t)), "/?repo=spf13__cobra")
	if !strings.Contains(empty, `class="st pending"`) {
		t.Error("un corpus sin triajear quedo con la lista vacia")
	}
}
