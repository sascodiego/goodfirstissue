package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/sascodiego/nan-hackathon-clef/internal/ghdata"
	"github.com/sascodiego/nan-hackathon-clef/internal/store"
)

// máxima longitud de lo que se guarda de una entrada: un nombre de repositorio de GitHub
// no pasa de 100 caracteres, así que un campo enorme es o un error de tipeo o un intento de
// gastar memoria, y ninguna de las dos cosas merece un request a la API.
const maxRepoInput = 200

// add-repo es la ruta que el formulario del rail llama. El fetch real tarda unos once
// segundos, así que este es el único lugar de la app donde un pedido espera a GitHub; el
// resto de las pantallas se arma con lo que ya está en el store.
func (h *Handler) addRepo(w http.ResponseWriter, r *http.Request) {
	if rejectCrossSite(w, r) {
		return
	}
	if h.store == nil || h.fetcher == nil {
		http.Error(w, "el alta no está disponible: este binario arrancó sin store o sin GitHub",
			http.StatusServiceUnavailable)
		return
	}

	owner, name, err := parseRepoInput(r.PostFormValue("nuevo"))
	if err != nil {
		// La entrada es del usuario: 400 y el motivo a la vista, antes de gastar un request.
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	key, err := h.fetchAndStore(r.Context(), owner, name)
	if err != nil {
		h.failFetch(w, owner+"/"+name, err)
		return
	}
	redirectToRepo(w, r, key, "alta")
}

// refreshRepo vuelve a consultar GitHub para un repositorio que ya está en el corpus. Es
// lo que «Refrescar» hacía de mentira: antes era un enlace a la misma URL.
func (h *Handler) refreshRepo(w http.ResponseWriter, r *http.Request) {
	if rejectCrossSite(w, r) {
		return
	}
	key := r.PathValue("repo")
	if h.store == nil || h.fetcher == nil {
		http.Error(w, "el refresco no está disponible: este binario arrancó sin store o sin GitHub",
			http.StatusServiceUnavailable)
		return
	}

	entry, ok, err := h.store.Repo(key)
	if err != nil {
		slog.Error("leer el registro", "key", key, "err", err)
		http.Error(w, "no se pudo leer el corpus", http.StatusInternalServerError)
		return
	}
	if !ok {
		// El registro es la autoridad sobre qué existe en el corpus: refrescar algo que no
		// está sería un alta disfrazada, y el alta tiene su propio formulario.
		http.Error(w, "ese repositorio no está en el corpus; agregalo primero", http.StatusNotFound)
		return
	}

	owner, name, err := splitCanonical(entry.Name)
	if err != nil {
		// El nombre guardado es el canónico que devolvió GitHub, así que esto no puede pasar
		// sin que el store esté roto. Decirlo es mejor que refrescar cualquier cosa.
		slog.Error("nombre canónico ilegible en el registro", "key", key, "name", entry.Name, "err", err)
		http.Error(w, "el registro tiene un nombre que no se puede leer; volvé a traerlo con gfi fetch",
			http.StatusInternalServerError)
		return
	}

	canonical, err := h.fetchAndStore(r.Context(), owner, name)
	if err != nil {
		h.failFetch(w, entry.Name, err)
		return
	}
	// Un repositorio renombrado cambia de clave: la redirección sigue al nombre nuevo en vez
	// de dejar al usuario mirando una clave que ya no existe.
	redirectToRepo(w, r, canonical, "refresco")
}

// fetchAndStore trae un repositorio y lo registra. Devuelve la clave canónica del store.
//
// Repite, en el mismo orden, lo que hace `gfi fetch` en la consola: el snapshot, el alias y
// la entrada del registro. GitHub manda sobre el nombre —httpie/httpie responde
// httpie/cli—, porque guardar la clave con lo que escribió el usuario parte un repositorio
// en dos la primera vez que lo renombran.
func (h *Handler) fetchAndStore(ctx context.Context, owner, name string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	snap, err := h.fetcher.Fetch(ctx, owner, name, h.github)
	empty := errors.Is(err, ghdata.ErrNoIssues)
	if err != nil && !empty {
		return "", err
	}
	if snap == nil {
		return "", errors.New("la respuesta de GitHub vino vacía")
	}

	canonOwner, canonName := owner, name
	if co, cn, splitErr := splitCanonical(snap.Repository.NameWithOwner); splitErr == nil {
		canonOwner, canonName = co, cn
	}

	display := snap.Repository.NameWithOwner
	if display == "" {
		display = canonOwner + "/" + canonName
	}
	key := store.RepoName(canonOwner, canonName)

	if err := h.store.SaveSnapshot(key, snap); err != nil {
		return "", err
	}
	if err := h.store.SaveAlias(owner+"/"+name, canonOwner+"/"+canonName); err != nil {
		return "", err
	}
	if err := h.store.SaveRepo(store.RepoEntry{
		Name:      display,
		Key:       key,
		CommitSHA: snap.Repository.CommitSHA,
		TotalOpen: snap.TotalOpen,
		Issues:    len(snap.Issues),
	}); err != nil {
		return "", err
	}
	return key, nil
}

// failFetch separa el error del usuario del nuestro: un nombre que GitHub no conoce es un
// 400 con el motivo; cualquier otra cosa es un 500 y no se disfraza de «no existe».
func (h *Handler) failFetch(w http.ResponseWriter, target string, err error) {
	if errors.Is(err, ghdata.ErrRepoNotFound) {
		http.Error(w, fmt.Sprintf("GitHub no conoce el repositorio %s", target),
			http.StatusBadRequest)
		return
	}
	slog.Error("traer repositorio", "target", target, "err", err)
	http.Error(w, fmt.Sprintf("no se pudo traer %s: %v", target, err),
		http.StatusInternalServerError)
}

// redirectToRepo cierra el POST con un 303: la pantalla se vuelve a pedir con GET, así
// recargar en el navegador no dispara otro fetch.
func redirectToRepo(w http.ResponseWriter, r *http.Request, key, estado string) {
	target := "/?repo=" + url.QueryEscape(key) + "&estado=" + url.QueryEscape(estado)
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// rejectCrossSite corta el POST que viene de otra página, y devuelve true cuando ya
// respondió.
//
// El binario escucha en 0.0.0.0:8080 por defecto, así que «es local» no es un argumento: un
// sitio cualquiera abierto en el mismo navegador puede postear a localhost y escribir en el
// store. Se mira lo que el navegador dice del origen (Sec-Fetch-Site), y si no lo dice se
// compara Origin con Host. Es la defensa mínima y suficiente para un endpoint que no tiene
// sesión que robar: no hay token porque no hay nada que el token proteja de más.
func rejectCrossSite(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "este formulario solo acepta pedidos de la propia página",
			http.StatusForbidden)
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || !strings.EqualFold(u.Host, r.Host) {
		http.Error(w, "este formulario solo acepta pedidos de la propia página",
			http.StatusForbidden)
		return true
	}
	return false
}

// parseRepoInput acepta las dos formas que promete el formulario: `owner/nombre` o el
// enlace completo de GitHub. Devuelve las partes tal como las escribió el usuario; el nombre
// canónico lo decide la respuesta de GitHub, no esto.
func parseRepoInput(raw string) (owner, name string, err error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", errors.New("escribí owner/nombre o el enlace de GitHub")
	}
	if len(trimmed) > maxRepoInput {
		return "", "", fmt.Errorf("el nombre es demasiado largo (%d caracteres)", len(trimmed))
	}

	target := trimmed
	if strings.Contains(trimmed, "://") {
		u, parseErr := url.Parse(trimmed)
		if parseErr != nil {
			return "", "", fmt.Errorf("%q no es un enlace válido", trimmed)
		}
		if host := strings.ToLower(u.Host); host != "github.com" && host != "www.github.com" {
			return "", "", fmt.Errorf("el enlace tiene que ser de GitHub, no de %q", u.Host)
		}
		target = strings.Trim(u.Path, "/")
	}

	target = strings.TrimSuffix(target, ".git")
	parts := strings.Split(target, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("%q no es owner/nombre ni el enlace de un repositorio", trimmed)
	}
	owner, name = parts[0], parts[1]
	if !validRepoPart(owner) || !validRepoPart(name) {
		return "", "", fmt.Errorf("%q no es owner/nombre ni el enlace de un repositorio", trimmed)
	}
	return owner, name, nil
}

// splitCanonical parte un `owner/nombre` que ya viene de GitHub. Es el mismo parser sin las
// formas de entrada del formulario.
func splitCanonical(canonical string) (owner, name string, err error) {
	parts := strings.Split(strings.TrimSpace(canonical), "/")
	if len(parts) != 2 || !validRepoPart(parts[0]) || !validRepoPart(parts[1]) {
		return "", "", fmt.Errorf("%q no es owner/nombre", canonical)
	}
	return parts[0], parts[1], nil
}

// validRepoPart acepta lo que GitHub acepta: letras, dígitos, guion, guion bajo y punto.
// Rechazar acá es lo que evita mandarle a la API un nombre con espacios o con un `..`.
func validRepoPart(part string) bool {
	if part == "" || part == "." || part == ".." {
		return false
	}
	for _, r := range part {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}
