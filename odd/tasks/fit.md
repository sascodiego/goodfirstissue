# Feature: el encaje — esta persona contra esta issue (#13)

El paso final: P3 lee la issue, lo que el explore encontró, la ficha técnica del proyecto y el CV
aprobado, y emite un nivel de encaje. Una petición por issue, y lo que se muestra es un nivel, no un
decimal.

Rama `feat/13-fit` · worktree `/src/worktrees/goodfirstissue/feat-13-fit` · base `main` en `f7ce2c2`.
Cuerpo de #13 re-escopeado antes de empezar (T0): el texto viejo pedía evaluación por partes con
`smallest_slice`, y `PROYECTO.md` §16 la tiene fuera del MVP.

## Specs

S1. **Una petición, un veredicto por issue.** «una  peticion por issie, se deja claro que es por
limite del mvp.» La evaluación por parte se cayó por reloj (#29), no por diseño: `PROYECTO.md` §16
la registra fuera del MVP y `:156` fija que se juzga por la **unión** de los requisitos. Las áreas
de la issue **se muestran** —agrupan la evidencia y dicen qué toca— pero no se evalúan por
separado. Esto se dice así en el documento, no se presenta como una decisión permanente.

S2. **El veredicto es un nivel, nunca un decimal.** `§9.3` (`PROYECTO.md:1206-1213`) mapea el
`choice` de cinco opciones a lo que se muestra: `insufficient_evidence` → `insufficient_information`;
**`p < 0,45` → `uncertain`**; `ready` con `p >= 0,70` → `recommended`; `ready_with_support` con
`p >= 0,50` → `recommended_with_support`; los `needs_*` con su probabilidad; Clef caído →
`undecided`. La probabilidad **acompaña** al nivel; no lo reemplaza. P3 no usa preguntas `score`:
un `2,39` con `confidence` 0,55 finge una precisión que no existe.

S3. **El `state` de P3 lleva las seis secciones de §9.1, y marca qué puede pesar.** Cada requisito
del explore viaja con marcador: `knowledge` es `[evaluable]`, `project` y `process` son
`[contexto]`. Solo lo evaluable participa del veredicto; el resto está ahí para que el modelo
entienda los requisitos, no para ser portón («la aptitud es técnica y solo técnica», §13; «ser nuevo
no puede ser un criterio de aceptación», §1.2).

S4. **§9.1 se actualiza al perfil que ya existe** (decisión A del mantenedor). La plantilla del
`state` pasa a decir lo que `internal/profile` emite: niveles `declarado` · `demostrado` · `inferido`
(`profile.go:48-56`), sin campo `fuente:`, y las secciones reales del documento. Entra como
`PROYECTO.md` 2.25, y `:1625` («nivel, fuente y evidencia») se corrige con ella.

S5. **`## REPOSITORIO` son tres hechos y el stack.** `Name`, `Primary language` y `Tests` salen del
corpus y del explore, más la sección `Stack técnico` del retrato del proyecto (`#47`). El retrato
entero **no** entra: su propia mitad «proyecto» declara en el código que nunca es compuerta de
encaje (`internal/projectbrief/sections.go:27-31`).

S6. **La clave del journal incluye el perfil.** La pregunta de P3 no es `(issue, pasada, modelo,
versión, commit)`: es eso **más el CV**. `PromptVersion` del `store.Key` pasa a ser
`Version()` + la huella del perfil aprobado, así otro CV es otra pregunta y no se presenta como
fresca (I14). No se toca el contrato de `store.Key`.

S7. **Sin perfil no hay veredicto.** `insufficient_evidence` → `insufficient_information`, y el
bloque dice qué falta. Un corte de red da `undecided` visible y la app sigue viva (I2). Nunca un
veredicto por defecto.

S8. **Las cinco opciones de la pregunta son puntos de una sola recta**: el costo de cerrar la
brecha técnica, de cero a infinito (§9.2). Las descripciones están escritas en el diseño de #13 y no
se re-redactan.

S9. **La etiqueta de la issue no cambia el veredicto** (I6). Test explícito: mismo CV, misma issue,
etiqueta distinta, mismo nivel.

S10. **El riesgo no es criterio de encaje**: es atributo de la issue (§9.3). `project` y `process`
son costo de onboarding, nunca portón.

S11. **El bloque en la pantalla de issue** muestra el nivel, el desglose de los requisitos
evaluables con la evidencia que los respalda, las lagunas, y el pin de la respuesta (modelo, versión
del prompt con la huella del perfil, commit, cuándo). Sin perfil guardado dice qué falta.

S12. **Cada fila de §9.3 se escribe con su test en rojo antes de implementarse.** La política es una
función pura: sin cliente, sin store, sin reloj.

## Tasks

| ID | Specs | Qué | Ruta | Commit |
|---|---|---|---|---|
| T0 | — | Re-escopeo de #13: cuerpo nuevo con las cinco contradicciones resueltas y las tres decisiones | inline | pendiente |
| T1 | S2, S12 | `internal/fit/policy.go`: `Decide` puro con las siete filas de §9.3, y sus tests | worker | `6684669` |
| T2 | S8, S2 | `internal/fit/questions.go`: el `choice` de cinco opciones y la versión del contrato | worker | `6684669` |
| T3 | S3, S4, S5 | `internal/fit/state.go`: constructor del `state` con marcadores, tres hechos + stack técnico, y el perfil aprobado | worker | `6684669` |
| T4 | S1, S6, S7 | `internal/fit/fit.go`: `Options`, `Judge` inyectable, journal con `PassFit` y la huella del perfil en la clave | worker | `6684669` |
| T5 | S11 | El bloque: `internal/web/fit_view.go`, `templates/partials/fit.html`, 3 líneas en `issue.go` | worker | `6684669` |
| T6 | todas | e2e con el corpus real: el mismo CV contra dos issues da niveles distintos; sin CV, `insufficient_information` | inline | pendiente |
| T7 | todas | `make verify`, RDD y PR | inline | pendiente |

**Verificación**: RED por fila de §9.3 antes de implementar; `make verify` completo; y el e2e mirado
en la app con el corpus de `var/store`.

**Estrategia de entrega.** Tres slices: **A** `6684669`, el paquete `internal/fit` (1924 líneas con tests y el documento); **B** el verbo y el bloque, que dependen de A; **C** `PROYECTO` 2.25 y el e2e. A sola no cambia nada visible —es una política con sus tests— y B no compila sin A, así que van encadenados: PR de A contra `main`, y B contra la rama de A. A pasa el presupuesto de 400 líneas y se declara en el PR.

**Ruta**: T1–T4 son un paquete y una conducta —la pasada— y van a un worker; T5 es otro worker
porque depende del modelo anterior y toca la pantalla. T0, T6 y T7 quedan inline.

## Log

L1. «Dale, si, main avanzo mucjo, hubieron varios PRs y paso tiempo desde que se creo 13 con varias
decisiones en el camino asi qie si»

L2. «una  peticion por issie, se deja claro que es por limite del mvp.»

L3. «2. acepto tu recomendacion» — §9.1 se actualiza al perfil real que entregó #8, como `PROYECTO`
2.25.

L4. «3. aceoto tu recomendacion» — `## REPOSITORIO`: los tres campos factuales de §9.1 desde el
corpus y el explore, más la sección `Stack técnico` del retrato; el retrato entero no entra.

L5. «sellamos. Hacemos un odd comoleti»

L6. Evidencia del re-escopeo, con `archivo:línea`: `internal/fit/` no existe; `fit.html:5` es un
`define` vacío; `issue.go` no proyecta nada para fit; `store.PassFit` (`decisions.go:25`) no tiene
usuario de producción; `explore.Evidence` no tiene `parts` (`explore.go:63-76`); no existe ningún
marcador `[evaluable]`; y `smallest_slice` no aparece ni en `PROYECTO.md` ni en `internal/`.
