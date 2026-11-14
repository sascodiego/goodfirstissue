# web-split-by-screen

Partir `internal/web` por pantalla antes de la ola 1, sin cambiar comportamiento visible.
Issue #4 · rama `feat/split-web-by-screen` · base `main` @ 1a7d843.

## Specs

**S1.** `GET /` devuelve exactamente lo mismo que en `main`: las 150 filas del corpus, las mismas cifras y los mismos filtros. Se verifica con diff de bytes del HTML renderizado contra `main` sobre una matriz de URLs.

**S2.** Ninguna firma pública cambia. Texto de la issue: «Ninguna firma pública cambia: `NewHandler(store)` y `Routes()` siguen igual, porque `cmd/gfi` depende de ellas.» `cmd/` no se modifica.

**S3.** Texto de la issue: «`handler.go` → queda con `Handler`, `NewHandler`, estáticos y el registro de rutas.» En este plan el registro de rutas vive en `routes.go` (S8); `handler.go` queda con `Handler`, `NewHandler`, `embed` y estáticos. `dashboard.go` con el handler y el modelo de la pantalla salidos de `view.go`. `view.go` con lo compartido entre pantallas: `repoView`, filtros, helpers de fecha.

**S4.** Texto de la issue: «`templates/dashboard.html` → el `index.html` de hoy, renombrado.» Texto de la issue: «`templates/partials/` → carpeta nueva. `add-repo.html` con el formulario del rail, para que el alta de repositorio no tenga que editar `dashboard.html`.»

**S5.** Las plantillas se parsean en un set por pantalla: layout y parciales una vez, `Clone()` por pantalla. `NewHandler` devuelve error si una pantalla no define `"content"`. Motivo: el espacio de nombres de `html/template` es plano y la segunda pantalla que defina `"content"` reemplaza el cuerpo de la primera en silencio.

**S6.** Texto de la issue: «Los 17 tests actuales pasan; se mueven de archivo, no se debilitan.» Hoy son 14 funciones, no 17. Se mueven las 14 sin debilitar ninguna y se suman 3 que pinean S5 y S4: aislamiento de `"content"` entre pantallas, fallo de arranque cuando una pantalla no lo define, y el parcial del rail presente en `GET /`.

**S7.** `make verify` verde (build + test + lint + design-check). `app.css` y `app.js` no se tocan.

**S8.** `routes.go` es el único lugar donde se ve la superficie de la app: una línea por pantalla. Motivo: evita que cada pantalla de la ola 1 edite `handler.go`, que es el archivo con más carga del paquete.

## Tasks

| ID | Spec | Route | Commit |
| --- | --- | --- | --- |
| T1 | S8, S5 | comentar el plan y la evidencia en la issue #4 | — |
| T2 | — | documento de seguimiento y espejo en Engram | `docs(odd): track the web split by screen` |
| T3 | S2, S3 | separar `handler.go`, `routes.go`, `dashboard.go`, `view.go` | `refactor(web): split the dashboard out of view.go` |
| T4 | S4, S5 | `screens.go`, `dashboard.html`, `partials/add-repo.html` | `refactor(web): scope web templates per screen` |
| T5 | S6 | partir `handler_test.go` y sumar los 3 tests | `refactor(web): split web tests by screen` |
| T6 | S1, S7 | diff de bytes contra `main` + `make verify` | — |

## Log

**L1** (pedido original, verbatim): «Vamos a trabajar con la siguiente metodologia, vamos a leer una issue, analizarla planificarla y cuando yo te de mi concentimiento explicito recien ahi podes empezar a escribir codigo»

**L2** (verbatim): «Vamos a trabajar enun worktree nuevo» — worktree en `/src/worktrees/goodfirstissue/feat-split-web-by-screen`, rama `feat/split-web-by-screen` desde `main`. Baseline verificado: `go build ./...`, `go test ./internal/web/` (14 ok), `make design-check` ok.

**L3** (verbatim, respuesta a las tres decisiones): «1. Acepto tu propuesta / 2. Acepto tu propuesta. / 3. Ok» — se adopta `routes.go` separado (desvío respecto de la letra de #4) y set de plantillas por pantalla con `Clone()`.

**L4** (verbatim): «Acepto tu plan aunque se desvie. Comenta la issue con los motivos y la evidencia» — consentimiento explícito para escribir código y para comentar la issue.

**L5** (borde de rebanada): T3 y T4 se commitean juntos como una sola unidad revisable aunque el diffstat sume ~700 líneas, porque es una mudanza: `view.go` pierde 313 líneas y `dashboard.go` gana 289, y el neto es +77. Partir una mudanza en dos commits artificiales empeora la revisión, no la mejora. T5 va aparte porque sí cambia aserciones.

**Evidencia** (medida en el worktree): `handler.go` 71 líneas, `view.go` 417, `handler_test.go` 369 con 14 `func Test`, `templates/index.html` 162, `templates/layout.html` 14. Consumidores externos del paquete: solo `cmd/gfi/main.go:137` (`web.NewHandler`) y `:144` (`handler.Routes()`).
