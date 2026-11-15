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
	mux.HandleFunc("GET /issue/{repo}/{number}", h.issue)
	mux.HandleFunc("POST /repos", h.addRepo)
	mux.HandleFunc("POST /repos/{repo}/refresh", h.refreshRepo)
	mux.HandleFunc("GET /profile", h.profile)
	mux.HandleFunc("POST /profile", h.extractProfile)
	mux.HandleFunc("POST /profile/document", h.saveProfileDocument)
	mux.HandleFunc("POST /explore", h.exploreSelected)
	mux.HandleFunc("POST /explore/cancel", h.exploreCancel)
	mux.HandleFunc("POST /explore/clear", h.exploreClear)
	mux.HandleFunc("POST /explore/cancel-all", h.exploreCancelAll)
	mux.HandleFunc("GET /explore/rows", h.exploreRows)
	mux.HandleFunc("POST /fit", h.fitRun)
	mux.HandleFunc("POST /index", h.indexRepo)
	mux.Handle("GET /static/", http.StripPrefix("/static/", h.static))
	return mux
}
