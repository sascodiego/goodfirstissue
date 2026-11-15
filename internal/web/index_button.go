package web

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// indexRepo es el POST del índice.
//
// Construye el grafo de código del repositorio clonado, que es lo que falta cuando el corpus se
// trae pero todavía no se puede analizar: explore lee el índice, no el árbol. Igual que el
// encaje, corre el trabajo que ya existe —el clonador y el indexador— en este proceso, sin
// abrir un binario hijo: la pantalla no reimplementa nada, solo deja de exigir una terminal.
//
// Es lento por naturaleza (clonar y recorrer el árbol), así que el botón se deshabilita mientras
// corre y el límite de tiempo sale de la configuración del indexador.
func (h *Handler) indexRepo(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.PostFormValue("repo"))

	code := "indexado"
	if h.index == nil {
		code = "index-sin-config"
	} else if err := h.index(r.Context(), key); err != nil {
		slog.Error("indexar el repositorio", "repo", key, "err", err)
		code = "index-error"
	}

	target := "/?estado=" + code
	if key != "" {
		target += "&repo=" + url.QueryEscape(key)
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
