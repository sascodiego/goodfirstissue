package web

// El bloque de encaje de la pantalla de issue.
//
// No decide nada: muestra el nivel que P3 ya decidió sobre esta issue contra el perfil aprobado,
// con el desglose de lo que pesó y el pin que dice bajo qué pregunta se produjo (I4, S11). La
// lectura del journal usa fit.Recall —la misma función que arma la clave al escribir— y nunca
// reconstruye la clave a mano: la pasada y la pantalla no pueden discrepar sobre qué es esto.

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/config"
	"github.com/sascodiego/nan-hackathon-clef/internal/explore"
	"github.com/sascodiego/nan-hackathon-clef/internal/fit"
	"github.com/sascodiego/nan-hackathon-clef/internal/profile"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// fitCommand es el verbo que produce el veredicto que falta. No hay POST /fit, así que la
// pantalla nombra el comando en vez de ofrecer un botón que no existe.
const fitCommand = "gfi fit "

// fitView es el bloque completo: el nivel, lo que se puede ver detrás de él, lo que sólo es costo
// de onboarding y el pin de la respuesta.
type fitView struct {
	// HasProfile dice si hay un perfil aprobado. Sin él no hay contra qué comparar y no existe
	// veredicto posible (S7).
	HasProfile bool
	// Command es el comando que produce el veredicto que falta.
	Command string

	// Level es el veredicto como se lee: un nivel, nunca un decimal.
	Level string
	// Class es la forma del nivel: cada modo de no recomendar se pinta distinto.
	Class string
	Why   string
	// Probability acompaña al nivel sólo donde §9.3 la pide: los needs_* y una lectura retenida.
	// Una recomendación no lleva decimal, porque el decimal ocuparía el lugar del veredicto (S2).
	Probability    string
	HasProbability bool
	// Withheld nombra la lectura que no se mostró como veredicto y con qué confianza quedó.
	Withheld string

	// Requirements son los requisitos evaluables: el único balde que cruza con el CV (S3).
	Requirements []string
	// Project y Process son costo de onboarding. Se muestran para entender el trabajo y se dice
	// que no pesaron: nunca un portón (§13, S10).
	Project []string
	Process []string
	// Evidence es la evidencia que respalda la lectura, la misma corrida de explore de la que
	// salieron los requisitos.
	Evidence  []fitEvidenceFile
	CallPaths [][]string
	Tests     []string
	// Gaps son las lagunas que la exploración nombró (I10).
	Gaps         []string
	HasBreakdown bool
	// Breakdown dice por qué no hay desglose cuando el veredicto existe y la exploración que lo
	// alimentó ya no está guardada.
	Breakdown string

	Analyzed string
	Latency  string
	Model    string
	Prompt   string
	Commit   string
}

// fitEvidenceFile is one file of the exploration the verdict was read against, with the relation
// it may honestly claim.
type fitEvidenceFile struct {
	Path     string
	Symbol   string
	Line     int
	Reason   string
	Relation string
}

// attachFit busca el perfil aprobado y el veredicto guardado de esta issue, y los deja listos para
// la plantilla. Sin store, sin corpus o sin perfil no hay veredicto posible, y el bloque lo dice
// en vez de mostrar un nivel vacío.
func (h *Handler) attachFit(page *issuePage) {
	view := fitView{Command: fitCommand + page.Active.Name}
	if h.store == nil || page.CommitSHA == "" || page.Active.Key == "" {
		page.Fit = view
		return
	}

	// El perfil es la pregunta (S6): lo que se guardó es el documento que la persona aprobó, y
	// sin documento no hay comparación que pedir ni veredicto que leer.
	document, ok, err := profile.Load(h.store)
	if err != nil {
		slog.Error("leer el perfil aprobado", "err", err)
	}
	if !ok {
		page.Fit = view
		return
	}
	view.HasProfile = true

	decisions, err := h.store.Decisions(page.Active.Key)
	if err != nil {
		slog.Error("leer el journal de la issue", "key", page.Active.Key, "err", err)
		page.Fit = view
		return
	}

	// La pregunta que el veredicto responde incluye la exploración que produjo su evidencia: otro
	// prompt de explore u otro agente es un estado que el juez nunca leyó (I14). El modelo de
	// decisión lo fija la constitución, no config.json (I13), y la huella del perfil viaja en la
	// misma versión del prompt: otro CV es otra pregunta (S6).
	exploration := h.fitExploration()
	if !exploration.Known() {
		// Este servidor no puede nombrar la pregunta que la fila guardada responde, así que no la
		// lee: mostrar el nivel de una exploración que no sabe nombrar sería presentar una respuesta
		// a otra pregunta como propia.
		slog.Error("no se puede leer el encaje guardado: este servidor no tiene modelo de explore configurado",
			"key", page.Active.Key, "issue", page.Issue.Number)
		page.Fit = view
		return
	}

	analysis, decision, ok := fit.Recall(decisions, page.Issue.Number, config.DecisionModel, page.CommitSHA, document, exploration)
	if !ok {
		page.Fit = view
		return
	}

	page.Fit = newFitView(analysis, decision, h.fitEvidence(page, exploration))
	page.HasFit = true
}

// fitExploration is the identity of the exploration this server reads: the explore contract's
// version and the agent model it is configured with. Both halves are what the journal key of an
// exploration is made of, so this names the row the evidence block shows and the row the fit key
// fingerprints.
func (h *Handler) fitExploration() fit.Exploration {
	return fit.Exploration{PromptVersion: explore.Version(), Model: h.explore.Model}
}

// fitEvidence reads the exploration the level was decided on. It is the same row the evidence
// block shows, read with the same identity the fit key fingerprints, so this screen can never pair
// a verdict with an exploration it was not read against.
//
// The verdict does not store what it read — it stores the level and the answer — so this is the
// only way to show the breakdown without inventing one. A missing or unreadable exploration is
// the empty result, and the block says the breakdown is no longer stored.
func (h *Handler) fitEvidence(page *issuePage, exploration fit.Exploration) *explore.Evidence {
	decisions, err := h.store.Decisions(page.Active.Key)
	if err != nil {
		slog.Error("leer el journal de la issue", "key", page.Active.Key, "err", err)
		return nil
	}

	decision, ok := decisions.Get(store.Key{
		Issue:         page.Issue.Number,
		Pass:          store.PassExplore,
		Model:         exploration.Model,
		PromptVersion: exploration.PromptVersion,
		CommitSHA:     page.CommitSHA,
	})
	if !ok {
		return nil
	}

	var record explore.Record
	if err := json.Unmarshal(decision.Verdict, &record); err != nil || record.Evidence == nil {
		// Un registro que no se puede leer no es evidencia: se dice que el desglose no está, en
		// vez de mostrar medio desglose.
		return nil
	}
	return record.Evidence
}

// newFitView projects a stored verdict into what the screen can honestly say.
func newFitView(analysis fit.Analysis, decision store.Decision, evidence *explore.Evidence) fitView {
	result := analysis.Result
	view := fitView{
		HasProfile: true,
		Level:      fitLevelText(result.Level),
		Class:      string(result.Level),
		Why:        fitWhy(result),
		Analyzed:   formatDate(decision.At),
		Latency:    latencyText(time.Duration(decision.LatencyMs) * time.Millisecond),
		Model:      decision.Model,
		Prompt:     decision.PromptVersion,
		Commit:     shortSHA(decision.CommitSHA),
	}

	// La probabilidad acompaña al nivel donde §9.3 la pide, y en ningún otro lugar: los needs_*
	// con la suya, y una lectura retenida con la que la dejó por debajo de su piso.
	switch result.Level {
	case fit.LevelNeedsPreparation, fit.LevelNeedsExperience, fit.LevelUncertain:
		view.Probability = probabilityText(result.Probability)
		view.HasProbability = true
	}
	if result.Level == fit.LevelUncertain {
		view.Withheld = withheldText(result)
	}

	if evidence == nil {
		view.Breakdown = "El nivel está guardado con su pin, pero el análisis de explore que lo " +
			"alimentó ya no está guardado para este commit: el desglose no se puede mostrar."
		return view
	}

	view.Requirements = evidence.Requirements.Knowledge
	view.Project = evidence.Requirements.Project
	view.Process = evidence.Requirements.Process
	view.CallPaths = evidence.CallPaths
	view.Tests = evidence.Tests
	view.Gaps = gapsOf(evidence.Unknowns, evidence.MissingInformation)
	for _, file := range evidence.Files {
		view.Evidence = append(view.Evidence, fitEvidenceFile{
			Path:     file.Path,
			Symbol:   file.Symbol,
			Line:     file.Line,
			Reason:   file.Reason,
			Relation: file.Relation,
		})
	}
	view.HasBreakdown = true
	return view
}

// fitLevelText nombra el nivel como se lee en esta pantalla. El nivel es un vocabulario cerrado y
// estas son sus palabras: un id del cable no se le muestra a nadie.
func fitLevelText(level fit.Level) string {
	switch level {
	case fit.LevelRecommended:
		return "recomendada"
	case fit.LevelRecommendedWithSupport:
		return "recomendada con acompañamiento"
	case fit.LevelNeedsPreparation:
		return "necesita preparación"
	case fit.LevelNeedsExperience:
		return "necesita experiencia"
	case fit.LevelUncertain:
		return "sin veredicto"
	case fit.LevelInsufficientInformation:
		return "información insuficiente"
	case fit.LevelUndecided:
		return "sin respuesta"
	}
	return string(level)
}

// fitWhy dice en una frase qué significa el nivel. Cada fila de §9.3 conserva la suya: dos formas
// distintas de no recomendar que se leen igual son una forma sola.
func fitWhy(result fit.Result) string {
	switch result.Level {
	case fit.LevelRecommended:
		return "El perfil cubre los requisitos técnicos que esta issue pide: no hay brecha que cerrar."
	case fit.LevelRecommendedWithSupport:
		return "Hay una brecha técnica, y se cierra mientras se hace el trabajo con el mantenedor respondiendo preguntas."
	case fit.LevelNeedsPreparation:
		return "Hay una brecha técnica que conviene cerrar antes de empezar: estudiar o practicar fuera de esta issue."
	case fit.LevelNeedsExperience:
		return "La brecha técnica no se cierra dentro del horizonte de esta contribución: pide experiencia previa en el dominio."
	case fit.LevelUncertain:
		return "El juicio no llegó a la confianza que su propia fila pide, así que no se muestra como veredicto (§9.3)."
	case fit.LevelInsufficientInformation:
		if result.Reason == fit.ReasonNoProfile {
			return "No hay un perfil aprobado, así que no hay contra qué comparar (S7)."
		}
		return "El perfil aprobado no describe las capacidades técnicas que esta issue requiere: no hay nada que comparar. Es un hecho sobre el perfil, no sobre la oportunidad."
	case fit.LevelUndecided:
		switch result.Reason {
		case fit.ReasonEndpointFailed:
			return "Clef no contestó, así que no hay veredicto que mostrar. Se puede volver a correr: una request caída no es un juicio sobre la issue (I2)."
		case fit.ReasonIncompleteAnswer:
			return "La respuesta no trae un encaje que se pueda leer, así que no hay veredicto ni probabilidad que mostrar."
		}
	}
	return "El nivel está guardado, pero su motivo no está entre los que esta pantalla sabe explicar."
}

// withheldText dice cuál fue la lectura que no se mostró y con qué confianza quedó: una lectura
// retenida sin nombre se lee como una respuesta que se perdió.
func withheldText(result fit.Result) string {
	reading := fitAnswerText(result.Answer)
	if reading == "" {
		return "El juicio no llegó a la confianza que su fila pide, así que no se muestra como veredicto (§9.3)."
	}
	return fmt.Sprintf("Se retuvo la lectura «%s» (%s): por debajo de la confianza que su fila pide, "+
		"el juicio no se muestra como veredicto (§9.3).", reading, probabilityText(result.Probability))
}

// fitAnswerText nombra las cinco respuestas del cable como las lee una persona. Los ids son el
// contrato del juez y no se traducen en ningún otro lugar.
func fitAnswerText(answer fit.Fit) string {
	switch answer {
	case fit.FitReady:
		return "cubre todo"
	case fit.FitReadyWithSupport:
		return "se cierra con apoyo"
	case fit.FitNeedsPreparation:
		return "hace falta preparación"
	case fit.FitNeedsExperience:
		return "hace falta experiencia"
	case fit.FitInsufficientEvidence:
		return "no hay con qué comparar"
	}
	return ""
}

// gapsOf joins both halves of the exploration in order, dropping a gap the two halves already
// reported: the same sentence twice reads as two different doubts (I10).
func gapsOf(lists ...[]string) []string {
	var (
		out  []string
		seen = map[string]bool{}
	)
	for _, list := range lists {
		for _, item := range list {
			line := strings.TrimSpace(item)
			if line == "" || seen[line] {
				continue
			}
			seen[line] = true
			out = append(out, line)
		}
	}
	return out
}
