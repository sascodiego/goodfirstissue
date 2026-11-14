package web

import "net/http"

// Routes arma el mux. Las rutas usan los patrones con método de Go 1.22.
//
// Este archivo es el único lugar donde se ve la superficie de la app: una
// pantalla nueva agrega una línea acá y nada más. Y como ServeMux entra en
// panic si dos patrones conflictúan, el conflicto aparece en un solo lugar
// conocido, al construir el mux, y no según el orden en que cada pantalla se
// enganchó sola.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.dashboard)
	mux.HandleFunc("POST /repos", h.addRepo)
	mux.Handle("GET /static/", http.StripPrefix("/static/", h.static))
	return mux
}
