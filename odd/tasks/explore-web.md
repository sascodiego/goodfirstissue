# explore-web

Encender el botón «Analizar N seleccionadas»: cola en Go, estado por etapa y bloque de evidencia.
Issue #12 · rama `feat/12-explore-web` · base `main` @ 4b8684e.

## Specs

**S1.** Texto de la issue: «`internal/web/explore.go` — `POST /explore` sobre las issues seleccionadas.» Texto de la issue: «`templates/partials/evidence.html` — el bloque de evidencia.»

**S2.** Decisión sellada del usuario: «La cola debe resistir a la navegacion, lo mejor es que se maneje la cola y el control del parlelismo del lado de go y la web renderice su estado». La cola vive en el proceso del servidor, indexada por `(repo, issue, pass)`; navegar no la toca.

**S3.** Decisión sellada del usuario: «el paralelismo corre igual con tope de paralelas 3 configurable». Campo `explore.workers`, default `3`, validado en `1..3`; un worker es un proceso `pig` con su propio `--cwd`.

**S4.** Texto de la issue: «Seleccionar 3 issues y analizar: las tres quedan en cola y la UI refleja en curso / listo / error por issue.»

**S5.** Texto de la issue: «Lo ya analizado no se repite: la clave del journal `(issue, pass, modelo, versión del prompt, commit)` decide (I14), y se avisa cuándo se produjo.» Texto de la issue: «Una issue ya analizada vuelve en milisegundos y lo dice ("reutilizado, de hace X").»

**S6.** Texto de la issue: «El bloque de evidencia muestra archivos con `path:line` y su relación: `observed` solo si vino de `relatedFiles`/`entryPoints` del índice; si no, `probable`.» Decisión sellada del usuario (**A**, «lo mas sencillo»): manda la regla implementada —`PROYECTO.md` §8.2: `observed` exige respaldo en la traza de la corrida— y la UI degrada a `probable` toda relación que el registro ya lista como violación. La línea vieja de `AGENTS.md` se corrige para que no contradiga §8.2.

**S7.** Texto de la issue: «Sin `NAN_KEY`: error claro, no un fallo silencioso.» No se encola nada; el motivo se muestra por issue.

**S8.** Texto de la issue: «Tests sin red ni pig real: el runner se inyecta.»

**S9.** Decisión sellada del usuario: «Tambien debe haber una forma de cancelar lo que esta en curso y vaciar la cola.» Cancelar mata el proceso hijo por el `ctx` que ya existe; vaciar descarta lo pendiente; un job cancelado no escribe en el journal.

**S10.** Texto de la issue: «**Vigilancia por silencio del pipe, no por reloj**: 20 s sin líneas = "esperando al modelo" con los segundos; 90 s = se corta.» El estado `en curso` muestra los segundos de silencio en vivo.

**S11.** Decisión sellada del usuario: «no tengo tiempo para ver el tema de como mostrar en realtime». El estado se renderiza en Go; la tabla se actualiza con un parcial y `<meta refresh>` en `<noscript>`.

**S12.** Aviso de la issue: «**Colisiona con**: `[triaje] informe por issue` y `[encaje] veredicto final` en la página de issue.» La cola es por `pass` con una sola etapa cableada hoy (`explore`); la escalera nombra las etapas que faltan («llega con #11», «necesita perfil») y avisa que hoy no hay compuerta de P1 (I9).

**S13.** Decisión de plan (I4): índice ausente o con un `commit_sha` distinto del corpus ⇒ no se encola y se explica qué correr. Un resultado nunca se pinnea a un árbol que no leyó.

**S14.** Decisión de plan: tope de 50 issues por POST, con estimación visible (`≈ ⌈N/3⌉ × 1m49s`); encolar algo que ya está en cola o en curso no lo duplica.

**S15.** `make verify` verde (build + test + lint + design-check). `app.css`/`app.js` se editan en `design/` y se copian, porque `design-check` falla si difieren.

## Tasks

| ID | Spec | Route | Commit |
| --- | --- | --- | --- |
| T1 | — | documento de seguimiento y espejo en Engram | `docs(odd): track the explore queue from the dashboard` |
| T2 | S5, S8 | sesión única de análisis en `internal/explore`, compartida con el CLI | `47e0240 refactor(explore): share one analysis session with the CLI` |
| T3 | S2, S3, S7, S9, S10, S13, S14 | cola, rutas, wiring en `internal/web/explore.go` | `aa2b1a8 feat(config)` + `28594e9 feat(web): queue explore runs with cancel and drain` |
| T4 | S1, S4, S6, S11, S12 | parcial de evidencia, escalera por fila, parcial de filas | `488e539 feat(web): show the evidence block and live stage state` |
| T5 | S15 | docs y `make verify` | ver L9 |

## Log


**L1** (pedido original, verbatim): «Vamos a trabajar en un nuevo worktree en la issue #12»

**L2** (verbatim): «Bien, una cosa importante con respecto a la cola y la concurrencia, maximo 2 o 3 analisis en concurrencia. La cola debe resistir a la navegacion, lo mejor es que se maneje la cola y el control del parlelismo del lado de go y la web renderice su estado asi no tenemos que ocmplejizar con monitorear el estado y bla bla, es un mvp para una hackaton en la que estamos a contra reloj o sea necesito que funione, que sea vistoso, pero no se si tengo tiempo para ver el tema de como mostrar en realtime de lo que esta pasando en go. Si se puede implementar o reutilizar algo que ya exista mejor»

**L3** (verbatim): «Lo que pasa es que hay diferentes fases aca. Primero esta el triage preliminar(esta implementandose en otro agente, hoy l otenes en los benchmarks) a todas las issues o a las seleccionadas (no esta definido eso) que pasa si hay 1000 issues abiertas?[...] Dicho esto el paralelismo corre igual con tope de paralelas 3 configurable. Tambien debe haber una forma de cancelar lo que esta en curso y vaciar la cola.»

**L4** (verbatim): «El tema es que entiendo que el pipe es importante. Porque eso depende la ux: Tenemos varios pasos para cada issue y eso debe tenerse en cuenta. es importante conocer el pipe para definir como hacemos para resolverlo en la ui.»

**L5** (verbatim): «1. Lo mas sencillo, estamos a contrareloj. 2. Sellado» — S6 queda en **A** (manda §8.2, la UI degrada violaciones) y el plan queda sellado con el alcance de S1–S15.

**L7** El commit de T2 quedó en 601 líneas cambiadas, por encima de la unidad de ~400 que pide el workflow: la sesión compartida (`session.go` + sus 5 tests), el callback nuevo de `pig` y el CLI reescrito no se separan sin dejar un commit intermedio que no compila o que duplica el camino de código. Se registra acá en vez de fingir que fue una unidad. T3 se partió en dos commits (config y cola) y T4 en uno, para que el revisor tenga cortes.

**L8** T3 se delegó a un worker (`gentle-ai-worker`) por presupuesto de contexto del orquestador, con el handoff por referencia a este documento. Evidencia que devolvió: RED observado en `internal/config` (`Workers undefined`) y en `internal/web` (`undefined: StageRequest`), cuatro mutaciones temporales que rompen y luego se revierten, `go test -count=5` y `-race -count=2` verdes, `golangci-lint` en 0. El orquestador revisó el código, corrigió un `%w` sobre un error nil en el mensaje de `stalled`, y escribió T4.

**L9** Worktree `/src/worktrees/goodfirstissue/feat-12-explore-web`, rama `feat/12-explore-web` desde `main`. Baseline en el worktree: `go build ./...` ok, `go test ./internal/...` 11 paquetes ok, `make design-check` ok. Corpus disponible sin red: `var/store/spf13__cobra` con el análisis de la #1272 cuya clave coincide con el `Version()` actual (`2c5927dda0f1`), y `var/store/*/index.json` para los dos repos. `NAN_KEY` está en `.env`, no en el entorno del servidor: la verificación en vivo lo exporta.
