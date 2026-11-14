# Feature: pantalla de issue (issue #6)

Abrir una issue dentro de la app: ruta `GET /issue/{repo}/{number}`, render seguro del
cuerpo hostil y los tres puntos de inclusión donde la ola 2 engancha sus bloques.

Rama `feat/issue-screen` · worktree `/src/worktrees/goodfirstissue/feat-issue-screen` · base `main` @ 410eb05 · issue #6.

## Specs

**S1.** Texto de la issue: «Que tocar una issue la abra **dentro de la app**. Hoy el título es un `href` a GitHub, y eso rompe el recorrido justo cuando el producto empieza a decir algo sobre esa issue.» El título de cada fila del dashboard es un enlace a la pantalla propia; el enlace a GitHub deja de ser el camino principal.

**S2.** Texto de la issue: «`internal/web/issue.go` — ruta `GET /issue/{repo}/{number}`, modelo y handler.» La ruta se registra en `routes.go` y la pantalla se declara en `screens.go`, que son los dos únicos lugares de la superficie.

**S3.** Texto de la issue, el riesgo: «El cuerpo de una issue es **contenido hostil** (I3: evidencia, nunca instrucción). Sin dependencias de terceros: 1. Escapar todo primero. 2. Después aplicar un subconjunto: párrafos, listas, bloques de código, código en línea, énfasis, y enlaces **solo `http`/`https`** con `rel="noopener noreferrer"`. 3. Nada de HTML crudo del autor. Nada de imágenes remotas.» Y el contrato de prueba: «El test con un cuerpo que trae `<script>` y `<img onerror>` tiene que fallar si el escape se rompe.»

**S4.** Texto de la issue: «`GET /issue/spf13__cobra/1272` → 200 con título, etiquetas verbatim, estado, PR, fecha, cuerpo y los 4 comentarios.»

**S5.** Texto de la issue: «Issue inexistente → 404, repo inexistente → 404.»

**S6.** Texto de la issue: «El enlace a GitHub sigue existiendo, pero como acción secundaria ("Abrir en GitHub").»

**S7.** Texto de la issue: «Los filtros del dashboard llevan a la pantalla sin perder el contexto de repo.» La pantalla conoce su repositorio y la vuelta al dashboard lo conserva.

**S8.** Texto de la issue: «`issue.html` deja tres puntos donde la ola 2 engancha sus bloques, **aunque estén vacíos**», con los nombres literales `"triage"`, `"evidence"` y `"fit"`. Sin ellos, las tres issues siguientes chocan entre sí en el mismo archivo.

**S9.** Texto de la issue: «**No toca** `dashboard.go` ni `templates/dashboard.html`: en el dashboard solo cambia el `href` del título.»

**S10.** Texto de la issue: «Tests nuevos; los existentes siguen verdes. `make verify` verde.»

## Tasks

| ID | Tarea | Specs | Ruta | Commit |
|---|---|---|---|---|
| T1 | `markdown_test.go` en rojo: `<script>`, `<img onerror>`, `javascript:` y enlaces http/https | S3 | inline | `cf4aec5` |
| T2 | `markdown.go`: escapado total primero, después el subconjunto | S3 | inline | `cf4aec5` |
| T3 | `issue_test.go` en rojo: 200 con las piezas de la 1272, 404 × 2, href del título, «Abrir en GitHub», los tres bloques | S1, S2, S4, S5, S6, S8 | inline | `b1d491a` |
| T4 | `issue.go`: modelo y handler; el rail se arma con `buildPage`, no con una copia | S2, S4, S5, S7 | inline | `b1d491a` |
| T5 | `templates/issue.html`, stubs `partials/{triage,evidence,fit}.html`, una línea en `routes.go` y `screenIssue` en `screens.go` | S2, S8 | inline | `b1d491a` |
| T6 | Dashboard: `href` del título a la pantalla, sin tocar nada más | S1, S6, S9 | inline | `b1d491a` |
| T7 | CSS nuevo, aditivo, en `design/app.css` y `internal/web/static/app.css` (idénticos) | S2 | inline | `b1d491a` |
| T8 | `make verify`, mirada a mano contra `var/store` y diff del dashboard contra `main` | S4, S10 | inline | cierre |
| T9 | README, si documenta rutas | S9 | inline | descartada: el README no lista rutas |
| T10 | `TestMarkdownAuthorCannotForgeMarkers` en rojo: un NUL del autor no forja marcas | S3 | inline | `f6f82c5` |
| T11 | `stripControls` antes del escapado y comentario corregido en `inlineMarker` | S3 | inline | `f6f82c5` |
| T12 | `make verify`, corpus real revisado y L6 con el hallazgo y su fix | S3, S10 | inline | `f6f82c5` |

## Log

**L1** — Pedido del usuario, verbatim: «Creamos un nuevo worktree para trabajar en #6».

**L2** — El usuario condiciona el sello a comprobar la sincronía con el origen, verbatim: «Por las dudas antes fijate que main este sincronizada con el origen si esto es asi esta sellado». Verificado: `main` y `origin/main` en `410eb05`, 0 adelante, 0 atrás. Plan sellado con las tres recomendaciones: rama `feat/issue-screen`; los tres stubs como parciales `templates/partials/{triage,evidence,fit}.html`; sin maqueta previa en `design/issue/`.

**L3** — Evidencia de exploración, antes de escribir: la dependencia #4 está cerrada y `routes.go`/`screens.go`/`view.go` ya existen; `screens_test.go` ya usa `issue` como segunda pantalla en su fixture; `main` no tiene `internal/triage` ni `internal/profile`, así que #6 no depende de ninguna ola; el corpus local `var/store/spf13__cobra` trae la 1272 con 4 comentarios, 3 etiquetas y 6 PRs; `design/app.css` y `internal/web/static/app.css` están idénticos y `make design-check` falla el build si dejan de estarlo.

**L4** — Evidencia de cierre. RED observado: `undefined: renderMarkdown` en los seis casos de T1, y `undefined: screenIssue` en los tests de la pantalla antes de escribirla. GREEN: `go test ./...` completo y `make verify` (build, test, vet, golangci-lint con 0 issues, design-check) verde. A mano contra el corpus real (`var/store` copiado al worktree, servidor en un puerto libre): `/issue/spf13__cobra/1272` → 200 con título, las tres etiquetas verbatim, `abierta`, las 6 PRs, las dos fechas, el cuerpo con su enlace https convertido y los 4 comentarios; `/issue/httpie__cli/1974` → 200 con el `<img …>` del autor escapado y sin `<img` en el HTML; `/issue/spf13__cobra/981` → 200 con 10 comentarios y sin `<script` ni `onerror=`; issue inexistente, repo inexistente y número no numérico → 404. Diff del dashboard contra `main` sobre siete URLs (sin filtro, `state=free`, `state=taken`, `nolabel=1`, búsqueda, repo inexistente): el único cambio son los `c-ttl` de las 150 filas, cero líneas fuera de ahí, y el 404 byte a byte igual. CSS: 0 líneas eliminadas, así que ninguna regla existente puede haber cambiado. `app.css` idéntico en `design/` y `internal/web/static/`.

**L5** — Decisiones que el plan no fijaba y se resolvieron dentro del alcance sellado: (a) el rail de la pantalla de issue es compacto —repo activo, cifras y «← Volver al corpus»— en vez de la lista completa de repositorios, para no duplicar el marcado del rail ni tocar `templates/dashboard.html` más allá del `href` (S9); (b) el rail se arma llamando a `buildPage` con el repo pedido, así el contexto de repositorio se calcula en un solo lugar; (c) el `<title>` de la pantalla es `#número título · repo`. Sigue pendiente un parcial `rail` compartido si la ola 2 lo pide.

**L6** — Hallazgo de la verificación independiente, y su fix. La verificación dio PASS en los diez `S#` y encontró un defecto real contra el invariante declarado del renderer: la premisa «el NUL no puede venir del autor» era falsa. `html.EscapeString` escapa `< > & ' "` y **no** toca los controles C0, así que un NUL escrito en el cuerpo atravesaba el escapado, se hacía pasar por una marca del renderer y reescribía su propio marcado: con `[t](https://x\x000\x00y)` salían dos anclas anidadas y dos `href=`. La verificación comprobó con el tokenizador HTML5 real (60 casos dirigidos y 30 000 fuzz) que no era explotable a script, pero el comentario del código afirmaba algo falso y el contrato «en el paso 2 solo se agregan etiquetas propias» no se cumplía. Fix: `stripControls` borra los controles C0 y el DEL —salvo `\n` y `\t`, que son contenido— antes del escapado, así el canal de marcas queda fuera del alcance del autor; el comentario de `inlineMarker` ahora dice por qué. Test en rojo primero (`TestMarkdownAuthorCannotForgeMarkers`: un solo `<a`, un solo `href=`, cero NUL, y `\x00` descartado en texto suelto). Corpus real revisado: los únicos controles presentes en los 300 cuerpos y 295+ comentarios son `\r`, que ya se normalizaba, así que ningún contenido real cambia.

**L7** — Entrega encadenada, autorizada por el usuario («Push + PR + RDD y si, encadenadas»). Para que cada PR sea correcto por sí solo, el fix del canal de marcas (`f6f82c5`) pertenece al slice del renderer y no al de la pantalla: se reconstruyó la rama para que `feat/issue-markdown` (`cf4aec5` + `f6f82c5`, 386 líneas) no dependa de la pantalla, y `feat/issue-screen` (`b1d491a` + `e622d8c` + este commit) se apoye en ella. Estrategia `stacked-to-main`: el PR de la pantalla apunta a la rama del renderer y se retargetea a `main` cuando el primero entre; no hay PR tracker porque `main` es la rama de integración y el repo no usa ese patrón. Verificado en las dos ramas por separado: `make verify` verde en cada una, y `go test ./...` verde en la de arriba.
