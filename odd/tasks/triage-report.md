# Feature: el informe de triaje en la pantalla de issue (#11)

El bloque que muestra qué decidió P1 sobre una issue, con su probabilidad y sus preguntas, en la
pantalla que ya existe. Extiende `internal/web` siguiendo el molde del bloque de evidencia; no
abre artboard nuevo ni agrupa duplicados.

Rama `feat/11-triage-report` · worktree `/src/worktrees/goodfirstissue/feat-11-triage-report` ·
base `main` en `d96ce77`. Cuerpo de #11 re-escopeado antes de empezar (ver T0 y L6).

## Specs

S1. **El veredicto nunca aparece sin su probabilidad.** «Nunca un estado sin su probabilidad
(§7.7). Un `kind=bug` con 0,86 no es lo mismo que uno con 0,51.» El bloque muestra
`Result.Outcome` con `Result.Probability`. Un hecho determinista reporta 1 porque es certeza, no
probabilidad.

S2. **Las preguntas son las que se preguntaron, en el orden congelado.** No es una lista fija:
siete de substancia (`kind`, `self_contained`, `has_evidence`, `unambiguous`, `done_defined`,
`reports_defect`, `evidence_sufficient`) y, solo si el nodo de alineación corrió,
`only_for_requester` y `fits_direction`. «Un bug muestra siete, una feature nueve. Nadie ve una
pregunta que no se hizo.» No se afirma un número fijo en pantalla.

Corrección registrada en la implementación: las dos de alineación **no viven en `Answers`** sino en
`Analysis.Alignment`, que es un mapa aparte (`triage.Recall` devuelve los dos). Las filas se arman
del orden congelado consultando primero uno y después el otro; sin eso una feature nunca podría
mostrar nueve.

S3. **Cada respuesta viaja con su valor.** Una respuesta `choice` muestra la etiqueta elegida y
su `Confidence`; una `noul` muestra su valor. La pregunta que decidió el veredicto se distingue
de las que solo se midieron (`policy.go`: las de calidad no son compuertas).

S4. **Los cuatro modos de no-recomendar se distinguen en texto y en forma**, con el nombre real
del código: `DUDA` (`insufficient_evidence`, y se nombran las preguntas faltantes de
`Result.Missing`), `DERIVADA` (`second_round_pending`), `CONDICIONADA` (`alignment_unavailable` o
`ask_maintainer`, diciendo cuál) y `undecided` (`endpoint_failed` o `incomplete_answer`). «Cuatro
abstenciones, cuatro arreglos.» `endpoint_failed` dice que se puede volver a correr.

S5. **`MARCADA` muestra el motivo que la resolvió**, y los cinco del pre-gate se leen distinto:
`already_done`, `declined`, `already_working`, `closed`, `no_content`.

S6. **El `kind` declarado por el autor se contrasta con el leído.** Si `NormalizeTitle` encontró
una declaración y difiere de `Result.Kind` con probabilidad `>= 0,6`, se muestra: «El autor también
puede mentir.» Por debajo, no se presenta como contradicción.

Corrección registrada en la implementación: el corte cae sobre la **confianza de la respuesta del
`kind`**, no sobre `Result.Probability`. En una feature, `Result.Probability` es la probabilidad del
nodo de alineación, que mide otra cosa: usarla ponía dos números distintos sobre el mismo juicio
(la fila del `kind` arriba decía 0,86 y la contradicción citaba 0,68) y dejaba que un nodo muy seguro
levantara una contradicción sobre una lectura del `kind` de 0,40.

S7. **La etiqueta original aparece verbatim y marcada como dato del mantenedor**, nunca como
insumo de la decisión (I6). No se audita ni se juzga.

S8. **El bloque declara el pin de la respuesta** (I4, I14): guardado el · latencia · modelo ·
versión del prompt · commit. Un veredicto sin su pin se lee como fresco.

S9. **Sin triaje guardado, el bloque lo dice y nombra el comando** `gfi triage OWNER/REPO`. No
ofrece un botón que no existe y no queda vacío.

S10. **La tabla se lee a 360 px sin desbordar**, con el patrón de `.evid-files-wrap`. El CSS vive
en `design/app.css` y su copia idéntica en `internal/web/static/app.css`, porque `make
design-check` falla si difieren.

S11. **El bloque no inventa alcance.** No agrupa duplicados (#23, `post-demo`: «agrupar
duplicados: fuera del MVP por tiempo»), no expone `POST /triage`, y no toca `issue.html` — el
punto de inclusión ya existe en `issue.html:70`.

## Tasks

| ID | Specs | Qué | Ruta | Commit |
|---|---|---|---|---|
| T0 | — | Re-escopeo de #11: cuerpo nuevo y comentario con lo retirado y por qué | inline | issue #11, comentario 6889206172 |
| T1 | S8, S2 | `triage.Recall`: lectura pública del journal; `P1.lookup` delega en él. Test con clave armada a mano | inline | `08ca593` |
| T2 | S1, S2, S3, S4, S5, S6, S7 | `internal/web/triage_view.go`: `triageView`, `attachTriage`, `newTriageView` y las etiquetas de pregunta. Tests por regla | worker | `97df5f3` |
| T3 | S10, S11 | `templates/partials/triage.html` y el CSS en `design/app.css` con su espejo | worker | `97df5f3` |
| T4 | S9, S11 | Enganche: `Triage`/`HasTriage` en `issuePage` y la llamada en `h.issue`. Test de pantalla | worker | `97df5f3` |
| T5 | todas | `make verify` y la lectura real a 360 px | inline | `97df5f3` |

**Verificación**: RED por regla antes de implementar cada test, y `make verify` completo (build,
tests, `go vet`, `golangci-lint`, gate de `gofmt`, design-check). El check de 360 px se mira, no se
declara.

**Estrategia de entrega.** Dos slices, que son los dos commits: **A** `08ca593`, la lectura del
journal en `internal/triage` (~65 líneas de código); **B** `97df5f3`, el bloque en `internal/web`
(~1100 entre modelo, tests, parcial y CSS). B pasa el presupuesto de 400 líneas y se entrega en un
solo PR a propósito: un parcial sin modelo no renderiza y un modelo sin parcial no se ve, así que
partirlo dejaría una mitad que no se puede mirar. El riesgo de revisión se declara en el PR.

**Lo que encontró el check a 360 px**, y que ningún test cubría: la frase que explicaba la tabla
vivía dentro del `<caption>`, así que heredaba el ancho de la tabla y quedaba cortada detrás del
scroll. Se movió a un párrafo arriba y el `caption` quedó corto. Además `.tr-questions` bajó de
`26rem` a `21rem` de `min-width`: a 360 px la tabla ahora entra sin correr —la pregunta se parte en
dos líneas— y el envoltorio sigue corriendo por debajo de eso. Medido en el navegador: overflow de
página 0, tabla sin scroll a 390 px, y la fila del `decidió` en 49 px de alto.

**Ruta**: T2–T4 van a un solo worker: el modelo, el parcial y el enganche son una sola conducta —un bloque que se ve— y separarlos obligaría a tres escritores sobre el mismo archivo. T1 y T5 quedan inline.

## Log

L1. «Creamos un nuevo worktree para trabajar en la issue #11»

L2. «Entonces sugeris #41 anes que la 11?» · «primero 41 entonces»

L3. «pus y PR» · «Mergeamos la PR y seguimos con la #11»

L4. «Efecticamente, la issue es vieja, primero hay que re escopearla, re analizarla y re
evidenciarla, hay que actualizarla. de igual manera te doy las respuestas: 1. Recomendacion
Apropiada 2. Las agrupaciones quedsron fuera del mvp recopemdacion apropiada 3. No, lo havemos
directo, ya tenemos el diseño pasado a templates de golamg y la issue con su propia view, vamos a
extender lo que ya tenemos. Coincido en que la tarea es grande hsy que actualizsr su eje eso
requerira una orimera pasada para re scoppear la issue, luego un odd para ponerla en marcha»

L5. «si» — se publica el cuerpo re-escopeado de #11 y su comentario de registro.

L6. Revisión del entregable del worker: su reporte marcó dos lugares donde la spec y el código no
coincidían. S2 nombraba `Answers` para las preguntas de alineación, que en realidad viven en
`Analysis.Alignment`. S6 cortaba sobre `Result.Probability`, que en una feature es la probabilidad
del nodo de alineación: se corrigió a la confianza de la respuesta del `kind`, con un test que
falla si la probabilidad del nodo vuelve a levantar la contradicción. Las dos correcciones quedaron
en S2 y S6.

L7. Decisiones que fija L4, y quedan fuera de discusión salvo evidencia nueva: (a) preguntas y
abstenciones se alinean al código real, no al texto viejo; (b) el grupo de duplicados queda fuera
del MVP y lo absorbe #23 cuando exista; (c) no se maqueta artboard: se extiende la pantalla de
issue que ya está en templates.
