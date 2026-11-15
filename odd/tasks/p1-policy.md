# Feature: P1 con rúbrica congelada (issue #9)

La primera pasada en producción: pre-compuertas deterministas, una petición a Clef con las
preguntas congeladas, y la tabla de decisión. El banco de pruebas mide **este** código, no una
copia suya: una sola fuente de verdad para preguntas, expedientes y tabla.

Rama `feat/9-p1-policy` · worktree `/src/worktrees/goodfirstissue/feat-9-p1-policy` · issue #9.

## Specs

S1. Las preguntas disponibles para producción son **exactamente** las que usa el banco. No hay
copias: el banco consume el código de producción, y una guarda falla si `prompt.md` y las
preguntas de producción divergen en una sola palabra.

S2. La tabla de decisión y todo lo demás viven una sola vez, en producción. El banco no declara
su propia tabla ni su propio cliente HTTP.

S3. La cadena, en este orden: pre-compuertas deterministas → una petición con el expediente A →
`evidence_sufficient` < 0,5 es **DUDA** y no descarta (I10) → calidad de §7.3 →
`kind`: `spam` MARCADA, `other`/docs/tests CANDIDATA, `bug` CANDIDATA con el guardarraíl de
§7.6, `feature` CONDICIONADA → fallo del endpoint `undecided` (I2).

S4. `kind=question` sale **DERIVADA**: la segunda ronda está especificada y no implementada
(`LIMITES.md` §3). Es un desenlace propio y terminal, no un descarte.

S5. La etiqueta no entra en el expediente de sustancia, así que no puede mover `kind` ni la
calidad. El test es estructural, no de comparación (I6).

S6. Lo que se midió en la PR del benchmark manda sobre los documentos anteriores a esa
medición: `intent` sale de P1 (`LIMITES.md` §2), el nodo de alineación queda diferido (§7.5),
y `PROYECTO.md` §7.2–§7.7 se actualiza a esa realidad.

S7. La caché reutiliza el journal con la pasada `triage`, y la clave incluye modelo, versión de
prompt y commit (I14). Una petición en vuelo la garantiza el cliente (I1), no este paquete.

S8. Tests con respuestas de Clef fijas: la política no toca la red. `make verify` sigue
hermético, sin `NAN_KEY` y sin `GFI_INTEGRATION`.

S9. `DUPLICATE` no tiene política decidida (`LIMITES.md` §5): se resuelve como `closed`, sin
inventar un motivo propio.

## Tasks

| ID | Tarea | Specs | Ruta | Commit |
|---|---|---|---|---|
| T1 | Preguntas congeladas en producción, `Version()` y guarda contra `prompt.md` | S1, S2 | inline | — |
| T2 | Pre-compuertas, tabla de decisión y desenlaces, con un test por fila | S3, S4, S5, S9 | inline | — |
| T3 | Pipeline: caché del journal, cliente inyectable, I2 | S7, S8 | inline | — |
| T4 | Recablear el banco al código de producción y borrar sus copias | S1, S2 | inline | — |
| T5 | `PROYECTO.md` §7.2–§7.7 y notas de `criteria.md`/`LIMITES.md` | S6 | inline | — |

Riesgo: **ítem 3** — `PROYECTO.md` y las etiquetas del banco son contrato que otros leen — e
**ítem 6** — la tabla de decisión es el producto; mitigado porque desde T4 el banco corre el
mismo código que se envía.

## Log

L1. Pedido original, verbatim:

> Vamos a trabajar en un nuevo worktree en la issue #9

L2. Aclaración de alcance del owner: agrupar duplicados queda fuera del MVP y ya estaba
decidido antes (`PROYECTO.md` §2.3, §7.1, `LIMITES.md` §1). No se implementa ni se prueba acá.

L3. Corrección del owner a mi lectura: el triaje preliminar, los benchmarks y la evaluación
determinista **ya existen**; lo que falta es llevarlo a producción y cablearlo.

L4. Requisito central del owner, verbatim: «el benchmark se haga sobre la realidad, es decir
las preguntas disponibles para producción sean exactamente las que se usen en los benchmark,
con exactamente digo exactamente, o sea que no se reescriban sino que se use el mismo pipe de
producción en los benchmarks para que no haya divergencias nunca. Lo mismo con la tabla de
decisiones y todo lo demás.» → S1, S2.

L5. Decisión del owner sobre `kind=question`: sale como `DERIVADA`, desenlace propio (opción a).

L6. Decisión del owner sobre precedencia documental: «Lo que se trabajo en la PR de los
benchmarks es lo que debe pesar mas que los documentos anteriores ahi se midieron cosas y se
decidio sobre las medidas.» → S6, y habilita reescribir §7.2–§7.7 de `PROYECTO.md`.

L7. Medición previa al cableado: de los 291 casos del banco, 49 llegan a preguntas y 14
esperan un desenlace que la política sellada ya no produce (7 `question` MARCADA, 1 `question`
DUDA, 3 `other` MARCADA, 3 `syn-refused-*` MARCADA por `intent`). No se relabelean por
criterio propio: se corre el banco cableado a producción y cada diferencia se clasifica como
bug o como decisión sellada, con su causa asentada acá.
