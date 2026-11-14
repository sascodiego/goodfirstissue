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
| T1 | `markdown_test.go` en rojo: `<script>`, `<img onerror>`, `javascript:` y enlaces http/https | S3 | inline | `c1` |
| T2 | `markdown.go`: escapado total primero, después el subconjunto | S3 | inline | `c1` |
| T3 | `issue_test.go` en rojo: 200 con las piezas de la 1272, 404 × 2, href del título, «Abrir en GitHub», los tres bloques | S1, S2, S4, S5, S6, S8 | inline | `c2` |
| T4 | `issue.go`: modelo y handler sobre el rail de `view.go` | S2, S4, S5, S7 | inline | `c2` |
| T5 | `templates/issue.html`, stubs `partials/{triage,evidence,fit}.html`, una línea en `routes.go` y `screenIssue` en `screens.go` | S2, S8 | inline | `c2` |
| T6 | Dashboard: `href` del título a la pantalla, sin tocar nada más | S1, S6, S9 | inline | `c2` |
| T7 | CSS nuevo, si hace falta, en `design/app.css` y `internal/web/static/app.css` (idénticos) | S2 | inline | `c2` |
| T8 | `make verify` y mirada a mano de las cuatro rutas contra `var/store` | S4, S10 | inline | cierre |
| T9 | README, si documenta rutas | S9 | inline | cierre |

## Log

**L1** — Pedido del usuario, verbatim: «Creamos un nuevo worktree para trabajar en #6».

**L2** — El usuario condiciona el sello a comprobar la sincronía con el origen, verbatim: «Por las dudas antes fijate que main este sincronizada con el origen si esto es asi esta sellado». Verificado: `main` y `origin/main` en `410eb05`, 0 adelante, 0 atrás. Plan sellado con las tres recomendaciones: rama `feat/issue-screen`; los tres stubs como parciales `templates/partials/{triage,evidence,fit}.html`; sin maqueta previa en `design/issue/`.

**L3** — Evidencia de exploración, antes de escribir: la dependencia #4 está cerrada y `routes.go`/`screens.go`/`view.go` ya existen; `screens_test.go` ya usa `issue` como segunda pantalla en su fixture; `main` no tiene `internal/triage` ni `internal/profile`, así que #6 no depende de ninguna ola; el corpus local `var/store/spf13__cobra` trae la 1272 con 4 comentarios, 3 etiquetas y 6 PRs; `design/app.css` y `internal/web/static/app.css` están idénticos y `make design-check` falla el build si dejan de estarlo.
