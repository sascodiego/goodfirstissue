package web

// El bloque de evidencia de la pantalla de issue.
//
// No decide nada: muestra lo que el explore ya produjo sobre el árbol pineado, con la
// pregunta que lo produjo a la vista. Una evidencia sin su pin se lee como fresca, y una
// «observed» sin respaldo se lee como un hecho medido; las dos cosas serían mentira (I4).

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/explore"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// evidenceView es el bloque completo: intención, archivos, caminos, tests, requisitos y
// dudas, más el pin que dice cuándo y bajo qué pregunta se produjo.
type evidenceView struct {
	Analyzed string // fecha del análisis que se está mostrando
	Latency  string // cuánto tardó esa corrida
	Model    string
	Prompt   string // versión del contrato, la que hace del análisis otra pregunta si cambia
	Commit   string
	Intent   string

	Files    []evidenceFile
	Observed int // cuántos archivos quedaron en observed después del descarte

	CallPaths [][]string
	Tests     []string
	Knowledge []string
	Project   []string
	Process   []string
	Unknowns  []string
	Missing   []string

	// Violations son las rutas que el registro ya marcó como «observed» sin respaldo en la
	// traza. Se muestran como probable y se dice por qué: la regla de §8.2 es que observed
	// exige que una herramienta lo haya devuelto en esta corrida.
	Violations []string
	HasFiles   bool
	// Partial avisa cuando la corrida no terminó completa: el registro existe y es lo que
	// hay, pero no se presenta como un resultado terminado (I10).
	Partial string
}

// evidenceFile is one file the issue touches, with the relation it may honestly claim.
type evidenceFile struct {
	Path     string
	Symbol   string
	Line     int
	Reason   string
	Relation string
	Note     string
	// Downgraded marks a relation the trace did not back: the file is shown as probable.
	Downgraded bool
}

// attachEvidence busca el análisis guardado de esta issue y lo deja listo para la plantilla.
// Sin modelo configurado, sin store o sin análisis previo no hace nada: el bloque muestra
// su estado vacío, que es la verdad, en vez de un error.
func (h *Handler) attachEvidence(page *issuePage) {
	if h.store == nil || h.explore.Model == "" || page.CommitSHA == "" || page.Active.Key == "" {
		return
	}

	page.CanAnalyze = h.queue != nil && h.getenv != nil && h.getenv("NAN_KEY") != ""

	// La clave es la pregunta completa, igual que en la cola y en el CLI: si el modelo, el
	// prompt o el commit cambiaron, lo guardado responde otra cosa y no se muestra.
	key := store.Key{
		Issue:         page.Issue.Number,
		Pass:          store.PassExplore,
		Model:         h.explore.Model,
		PromptVersion: explore.Version(),
		CommitSHA:     page.CommitSHA,
	}
	decisions, err := h.store.Decisions(page.Active.Key)
	if err != nil {
		slog.Error("leer el journal de la issue", "key", page.Active.Key, "err", err)
		return
	}
	decision, ok := decisions.Get(key)
	if !ok {
		return
	}

	var record explore.Record
	if err := json.Unmarshal(decision.Verdict, &record); err != nil || record.Evidence == nil {
		// Un registro que no se puede leer no es evidencia: se dice que no hay, y el botón
		// de analizar queda disponible para rehacerlo.
		return
	}

	page.Evidence = newEvidenceView(&record, decision)
	page.HasEvidence = true
}

// newEvidenceView projects a stored record into what the screen can honestly say.
func newEvidenceView(record *explore.Record, decision store.Decision) evidenceView {
	ev := record.Evidence
	view := evidenceView{
		Analyzed:  formatDate(decision.At),
		Latency:   latencyText(time.Duration(decision.LatencyMs) * time.Millisecond),
		Model:     record.Model,
		Prompt:    decision.PromptVersion,
		Commit:    shortSHA(record.CommitSHA),
		Intent:    ev.Intent,
		CallPaths: ev.CallPaths,
		Tests:     ev.Tests,
		Knowledge: ev.Requirements.Knowledge,
		Project:   ev.Requirements.Project,
		Process:   ev.Requirements.Process,
		Unknowns:  ev.Unknowns,
		Missing:   ev.MissingInformation,
	}

	// Las violaciones llegan con la ruta: la fila que las nombra es la que baja a probable.
	unsupported := make(map[string]bool, len(record.Violations))
	for _, v := range record.Violations {
		unsupported[v.Path] = true
		view.Violations = append(view.Violations, v.Path)
	}

	for _, f := range ev.Files {
		file := evidenceFile{
			Path:     f.Path,
			Symbol:   f.Symbol,
			Line:     f.Line,
			Reason:   f.Reason,
			Relation: f.Relation,
		}
		if f.Relation == explore.RelationObserved && unsupported[f.Path] {
			file.Relation = explore.RelationProbable
			file.Downgraded = true
			file.Note = "la traza de la corrida no lo respalda: se muestra como probable"
		}
		if file.Relation == explore.RelationObserved {
			view.Observed++
		}
		view.Files = append(view.Files, file)
	}
	view.HasFiles = len(view.Files) > 0

	switch {
	case record.Metrics.Stalled:
		view.Partial = "La corrida se cortó por silencio del agente antes de terminar: lo que sigue es lo que alcanzó a producir."
	case record.Metrics.TimedOut:
		view.Partial = "La corrida agotó su presupuesto de tiempo: lo que sigue es lo que alcanzó a producir."
	case !record.Metrics.Complete:
		view.Partial = "La corrida no terminó completa: lo que sigue es lo que alcanzó a producir."
	}

	return view
}

// latencyText es la duración de una corrida como la lee una persona, y «—» cuando el
// registro no la guardó.
func latencyText(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	return d.Round(time.Millisecond).String()
}
