package web

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/profile"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// Las palabras con las que la pantalla nombra sus estados. La clave rechazada se dice con
// las palabras de la API porque es el único error que la persona puede arreglar sola; la
// corrida incompleta se dice con lo que dijo el agente, que es evidencia y no un veredicto
// sobre el CV (S7).
const (
	invalidKeyTitle         = "401 invalid_api_key"
	agentFailTitle          = "El agente no completó la extracción."
	unreadableDocumentTitle = "El documento no se puede leer."
	nothingToSaveTitle      = "No hay nada para guardar."
	storeFailureTitle       = "No se pudo guardar el perfil."
)

// profilePage es lo único que ve templates/profile.html.
//
// El CV y el documento viajan como string y se renderizan con {{.Campo}}: son texto de
// afuera —el CV lo pegó una persona, el documento lo escribió un modelo— y template.HTML
// sería la única forma de convertirlos en marcado (S8).
type profilePage struct {
	Title string
	// Estado es la frase que confirma la última acción. Sale de una lista blanca de códigos,
	// nunca del texto que venga en la URL.
	Estado string
	// HasAgent es falso en un binario que arrancó sin agente usable. La pantalla se sirve
	// igual y lo dice; solo los POST responden que no está.
	HasAgent bool
	// Live mantiene el layout igual que en la issue: la cola vive en el dashboard, así que
	// esta pantalla nunca se refresca sola. El campo existe porque el layout lo consulta en
	// toda página, y un modelo sin él rompe el render.
	Live bool

	HasCV     bool
	CV        string
	CVDate    string
	OlderText string

	HasDocument bool
	// Edit es el valor del formulario de edición: el documento en efecto, o lo que la
	// persona acaba de escribir cuando el guardado no pasó la validación.
	Edit string

	Sections []profileSection
	HasItems bool

	Unsourced    []profileItem
	HasUnsourced bool

	// CVInput es la caja de pegado: vacía en un GET, el CV pegado cuando el último POST falló.
	CVInput string

	Insufficient       bool
	InsufficientReason string

	State       *profileState
	HasFeedback bool
}

// profileState es el bloque que dice con qué se encontró la última acción. Class elige cuál
// de los estados del producto se está mostrando, y son bloques distintos a propósito: una
// clave rechazada no se lee igual que una corrida que no terminó.
type profileState struct {
	Class  string
	Title  string
	Detail string
}

type profileSection struct {
	Name  string
	Items []profileItem
}

type profileItem struct {
	Section    string
	Name       string
	Level      string
	Evidence   string
	Assumption string
	Inferred   bool
	Sourced    bool
}

// profile sirve la pantalla del CV y del perfil que salió de él.
func (h *Handler) profile(w http.ResponseWriter, r *http.Request) {
	page, err := buildProfilePage(h.store)
	if err != nil {
		slog.Error("armar la pantalla del perfil", "err", err)
		http.Error(w, "no se pudo leer el perfil guardado", http.StatusInternalServerError)
		return
	}
	page.Estado = profileEstadoText(r.URL.Query().Get("estado"))
	page.HasAgent = h.extractor != nil
	h.renderProfile(w, page)
}

// extractProfile delega el CV al agente y, si contestó un documento, lo guarda como la
// revisión de ese CV y como el perfil en efecto.
func (h *Handler) extractProfile(w http.ResponseWriter, r *http.Request) {
	if rejectCrossSite(w, r) {
		return
	}
	if h.store == nil || h.extractor == nil {
		http.Error(w, "la extracción no está disponible: este binario arrancó sin store o sin NAN_KEY",
			http.StatusServiceUnavailable)
		return
	}

	cv := r.PostFormValue("cv")
	if strings.TrimSpace(cv) == "" {
		// La entrada es del usuario: 400 antes de gastar una corrida (S9).
		http.Error(w, "Escribí o pegá el CV antes de extraer.", http.StatusBadRequest)
		return
	}

	result, err := h.extractor.Extract(r.Context(), cv)
	if err != nil {
		h.failExtraction(w, cv, err)
		return
	}

	if result.Document.Insufficient {
		// Una duda no es un resultado: no crea revisión ni toca el documento en efecto. Se
		// dice, con su motivo, y el CV pegado queda a la vista para corregirlo (S5, I10).
		page, err := buildProfilePage(h.store)
		if err != nil {
			h.failProfilePage(w, "armar la pantalla del perfil", err)
			return
		}
		page.HasAgent = true
		page.CVInput = cv
		page.Insufficient = true
		page.InsufficientReason = result.Reason
		h.renderProfile(w, page)
		return
	}

	document := result.Document.Text
	revision := profile.Revision{
		CV:            cv,
		Document:      document,
		Model:         result.Model,
		PromptVersion: result.PromptVersion,
		At:            time.Now().UTC(),
	}
	if err := profile.SaveRevision(h.store, revision); err != nil {
		h.failProfilePage(w, "guardar la revisión del CV", err)
		return
	}
	if err := profile.Save(h.store, document); err != nil {
		h.failProfilePage(w, "guardar el perfil", err)
		return
	}

	// 303: recargar en el navegador vuelve a pedir la pantalla, no a repetir la corrida.
	http.Redirect(w, r, "/profile?estado=extraido", http.StatusSeeOther)
}

// saveProfileDocument guarda el perfil editado. El documento se valida con el mismo
// esqueleto con el que se lee, así que un texto que la pantalla no sabría mostrar no entra.
func (h *Handler) saveProfileDocument(w http.ResponseWriter, r *http.Request) {
	if rejectCrossSite(w, r) {
		return
	}
	if h.store == nil {
		http.Error(w, "la edición del perfil no está disponible: este binario arrancó sin store",
			http.StatusServiceUnavailable)
		return
	}

	edited := r.PostFormValue("documento")
	if strings.TrimSpace(edited) == "" {
		http.Error(w, "Escribí o pegá el documento antes de guardarlo.", http.StatusBadRequest)
		return
	}

	page, err := buildProfilePage(h.store)
	if err != nil {
		h.failProfilePage(w, "armar la pantalla del perfil", err)
		return
	}
	page.HasAgent = true
	// Lo editado vuelve al textarea en todos los caminos: nada de lo que la persona escribió
	// se pierde por un esqueleto roto.
	page.Edit = edited

	doc, err := profile.Parse(edited)
	if err != nil {
		page.State = &profileState{Class: "err", Title: unreadableDocumentTitle, Detail: err.Error()}
		h.renderProfile(w, page)
		return
	}
	if doc.Insufficient {
		page.State = &profileState{
			Class:  "err",
			Title:  nothingToSaveTitle,
			Detail: profile.Insufficient + ": el documento no tiene ningún ítem para guardar.",
		}
		h.renderProfile(w, page)
		return
	}

	if err := profile.Save(h.store, edited); err != nil {
		// El texto vuelve al formulario: llevarse la edición por un fallo del store sería el
		// peor final posible para la pantalla que existe para corregir el documento.
		slog.Error("guardar el perfil editado", "err", err)
		page.State = &profileState{Class: "err", Title: storeFailureTitle, Detail: err.Error()}
		h.renderProfile(w, page)
		return
	}
	http.Redirect(w, r, "/profile?estado=guardado", http.StatusSeeOther)
}

// failExtraction vuelve a la pantalla con el bloque del estado que corresponda. En los dos
// casos el CV pegado sigue en la caja y no se persiste nada: la corrida no produjo un
// resultado, así que no hay revisión que guardar (S7).
func (h *Handler) failExtraction(w http.ResponseWriter, cv string, err error) {
	page, perr := buildProfilePage(h.store)
	if perr != nil {
		h.failProfilePage(w, "armar la pantalla del perfil", perr)
		return
	}
	page.HasAgent = true
	page.CVInput = cv

	var failure *profile.Failure
	switch {
	case errors.As(err, &failure) && failure.Kind == profile.KindInvalidKey:
		page.State = &profileState{Class: "auth", Title: invalidKeyTitle, Detail: failure.Detail}
	case errors.As(err, &failure):
		// La corrida no terminó. No es un veredicto sobre el CV, y lo que dijo el agente es
		// la evidencia que hace falta para decidir si vale la pena reintentar.
		page.State = &profileState{Class: "err", Title: agentFailTitle, Detail: failure.Detail}
	default:
		// Una falla que no viene del agente —un pig que no está en el PATH— se dice igual,
		// con el mensaje tal cual, en vez de disfrazarla de respuesta del modelo.
		page.State = &profileState{Class: "err", Title: agentFailTitle, Detail: err.Error()}
	}
	slog.Error("extraer el CV", "err", err)
	h.renderProfile(w, page)
}

// failProfilePage responde un fallo nuestro, no del usuario: 500 y el motivo en el log.
func (h *Handler) failProfilePage(w http.ResponseWriter, what string, err error) {
	slog.Error(what, "err", err)
	http.Error(w, "no se pudo guardar ni leer el perfil: el store no está disponible",
		http.StatusInternalServerError)
}

// renderProfile ejecuta la pantalla con el layout y resuelve si hay algo que decir arriba.
func (h *Handler) renderProfile(w http.ResponseWriter, page profilePage) {
	page.HasFeedback = page.Estado != "" || page.State != nil || page.Insufficient || !page.HasAgent
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.render(w, screenProfile, page)
}

// buildProfilePage arma la pantalla con lo que hay en el store. Un store nil no es un error:
// es el binario recién compilado, y la pantalla lo dice en vez de fallar.
func buildProfilePage(st *store.Store) (profilePage, error) {
	page := profilePage{Title: "GoodFirstIssue · Perfil"}
	if st == nil {
		return page, nil
	}

	rev, ok, err := profile.CurrentRevision(st)
	if err != nil {
		return page, err
	}
	if ok {
		page.HasCV = true
		page.CV = rev.CV
		page.CVDate = formatDate(rev.At)
		all, err := profile.Revisions(st)
		if err != nil {
			return page, err
		}
		page.OlderText = olderCVs(len(all) - 1)
	}

	document, has, err := profile.Load(st)
	if err != nil {
		return page, err
	}
	if !has && ok {
		// Antes de la primera edición el documento en efecto es el que produjo el modelo.
		// Sin esta caída, una extracción recién hecha no se podría leer ni corregir.
		document, has = rev.Document, rev.Document != ""
	}
	if !has {
		return page, nil
	}
	page.HasDocument = true
	page.Edit = document
	page.showDocument(document)
	return page, nil
}

// showDocument llena la vista estructurada y los avisos de cita del documento dado.
//
// La auditoría corre antes de armar la vista para que cada ítem sepa si su cita está en el
// CV; los que no lo están se muestran aparte y no se borran (S10, I10).
func (p *profilePage) showDocument(document string) {
	doc, err := profile.Parse(document)
	if err != nil {
		// Un documento guardado que no se puede leer es un estado visible, no un 500: el
		// resto de la pantalla sirve, y el formulario de edición es justo lo que lo arregla.
		p.State = &profileState{Class: "err", Title: unreadableDocumentTitle, Detail: err.Error()}
		return
	}
	if doc.Insufficient {
		p.Insufficient = true
		p.InsufficientReason = "el documento no tiene ningún ítem que revisar"
		return
	}

	p.Unsourced = itemsOf(doc.Audit(p.CV))
	p.HasUnsourced = len(p.Unsourced) > 0

	for _, name := range profile.Sections {
		section := profileSection{Name: name}
		for _, item := range doc.Items {
			if item.Section == name {
				section.Items = append(section.Items, itemOf(item))
			}
		}
		// Una sección sin ítems no es un resultado vacío: es una sección que no se usa, y
		// mostrarla con una lista vacía sería inventar contenido.
		if len(section.Items) > 0 {
			p.Sections = append(p.Sections, section)
			p.HasItems = true
		}
	}
}

func itemOf(item profile.Item) profileItem {
	return profileItem{
		Section:    item.Section,
		Name:       item.Name,
		Level:      string(item.Level),
		Evidence:   item.Evidence,
		Assumption: item.Assumption,
		Inferred:   item.Level == profile.Inferred,
		Sourced:    item.Sourced,
	}
}

func itemsOf(items []profile.Item) []profileItem {
	out := make([]profileItem, 0, len(items))
	for _, item := range items {
		out = append(out, itemOf(item))
	}
	return out
}

// profileEstadoText traduce el código que deja el POST en la URL. Un valor que no está en
// la lista no se muestra: la pantalla no repite texto que vino de afuera.
func profileEstadoText(code string) string {
	switch code {
	case "extraido":
		return "Extraído. El CV quedó guardado."
	case "guardado":
		return "Guardado. El perfil en efecto es este documento."
	}
	return ""
}

// olderCVs nombra los CV que ya son historial. Se conservan enteros y no se pueden elegir ni
// restaurar en el MVP (S13), que es exactamente lo que dice la frase.
func olderCVs(n int) string {
	switch {
	case n <= 0:
		return ""
	case n == 1:
		return "1 CV anterior en historial, no disponible en este MVP"
	default:
		return fmt.Sprintf("%d CV anteriores en historial, no disponibles en este MVP", n)
	}
}
