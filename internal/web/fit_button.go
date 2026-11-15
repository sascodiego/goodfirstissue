package web

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// fitRun es el POST del encaje.
//
// La pantalla no decide: pide el mismo verbo que corre la consola, en un proceso hijo de este
// binario. Duplicar la selección de candidatas y el bucle de la tercera pasada dentro de
// internal/web habría creado un segundo camino que puede discrepar del que ya está probado, y
// el encaje es puro Clef: el viaje de ida y vuelta a la terminal era la única parte lenta.
//
// Sin runner configurado el botón no se dibuja; si igual llega el POST, la pantalla lo dice en
// vez de fingir que encajó algo.
func (h *Handler) fitRun(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.PostFormValue("repo"))

	code := "encaje"
	if h.fit == nil {
		code = "encaje-sin-clave"
	} else if err := h.fit(key); err != nil {
		slog.Error("encajar el corpus", "repo", key, "err", err)
		code = "encaje-error"
	}
	target := "/?estado=" + code
	if key != "" {
		target += "&repo=" + url.QueryEscape(key)
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
