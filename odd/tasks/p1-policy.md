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

S10. La cadena de producción es **la que produjo el informe medido**, no una composición nueva:
pre-compuertas → `evidence_sufficient` → `kind` → feature: nodo de alineación con umbral 0,6. Las
preguntas de §7.3 y el guardarraíl de §7.6 se preguntan y se reportan, y **no** deciden: el informe
declaró que no están validadas y aplicarlas movía 29 issues fuera de `CANDIDATA` sin respaldo.

S11. El nodo recibe la dirección del proyecto: lo que declara en su README y lo que viene
construyendo en sus últimos 20 commits, armado desde el clon. Un clon ilegible degrada el desenlace
a `alignment_unavailable`; no lo borra y no finge que el proyecto no fija posición.

S12. La ronda de `question` no se implementa, pero su costura sí: un tipo con un método y un campo
de configuración, instalado en nil y probado con un doble, para que agregarla después no toque
preguntas, expedientes, journal, banco ni política.

## Tasks

| ID | Tarea | Specs | Ruta | Commit |
|---|---|---|---|---|
| T1 | Preguntas congeladas en producción, `Version()` y guarda contra `prompt.md` | S1, S2 | inline | `271324e` |
| T2 | Pre-compuertas, tabla de decisión y desenlaces, con un test por fila | S3, S4, S5, S9 | inline | `2fcbf4d` (superado por T2′) |
| T3 | Pipeline: caché del journal, cliente inyectable, I2 | S7, S8 | inline | `4f669cd` |
| T4 | Recablear el banco al código de producción y borrar sus copias | S1, S2 | worker | `fdb5544` |
| T5 | `PROYECTO.md` §7.2–§7.7 y notas de `criteria.md`/`LIMITES.md` | S6 | inline | `0fe0e3f` (superado por T5′) |
| T2′ | Reescribir la cadena a la que produjo el informe medido | S3, S4, S10 | inline | `02113fb` |
| T6 | Nodo de alineación: `CloneBrief`, `repo.RecentSubjects`, segunda petición | S10, S11 | inline | `02113fb` |
| T7 | Costura `SecondRound`, instalada en nil y probada con un doble | S12 | inline | `02113fb` |
| T4b | El banco alimenta el brief congelado y vuelve a medir el nodo | S1, S10 | inline | `d7deaaa` |
| T5′ | `PROYECTO.md` §2.1.1/§2.2/§7.3/§7.5/§7.6/§7.7 y presupuesto | S6, S10 | inline | `1a9a991` |

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

L8. Límite de slice: T1 y T2 fueron unidades separadas, pero T2′, T6 y T7 comparten un solo
comportamiento —la cadena que se envía, con su nodo y su costura— y no se pueden separar sin dejar
el árbol sin compilar. Van en un commit (`02113fb`) con el corte registrado acá.

L9. Corrección grande del owner sobre mi lectura, verbatim: «Debería habernos dado los mismos
números que antes de migrar, no hubo cambio de decisión posterior a la última corrida del
benchmark». Tenía razón. La primera corrida dio `CANDIDATA` 115 y `CONDICIONADA` 75 contra 183 y 9
del informe. La descomposición no dejó residuo: **+26** por el nodo de alineación ausente y **+29**
por las compuertas de §7.3 que yo había agregado. Las respuestas del modelo no se movieron
(`kind` 34/37 → 35/37).

L10. Causa raíz, dicha sin adornos: la cadena que produjo `INFORME-TRIAJE.md` es
`pre-compuertas → evidence_sufficient → kind → (feature: alineación 0,6)`, sin §7.3, sin §7.6 y sin
`intent`. Yo compuse una tercera cadena mezclando la prosa de §7.7 con esas secciones, y **esa
cadena no la midió nadie**. El propio informe (§5) ya había declarado esas compuertas no validadas
y una de ellas rindiendo «peor que contestar siempre lo mismo».

L11. Decisión del owner sobre el presupuesto, verbatim: «No importa la cantidad de requests
secuenciales que le tengamos que pasar a clef, si hacer un árbol de decisiones aporta es una buena
decisión. Aumentar uso de clef es aceptable, acordarse que el core de la hackathon es clef, cuanto
más provecho le saquemos mejor». Consecuencia: el «techo de dos peticiones por issue» de
`criteria.md` deja de ser una regla y pasa a ser presupuesto medido (~0,55 s por petición, E3 = 30
issues < 120 s).

L12. Decisión del owner sobre la ronda de `question`, verbatim: «debe ser fácil de agregar la ronda
sin tener que modificar medio proyecto. La idea es que si tenemos una ventana de tiempo lo
agregamos ya con el proyecto funcionando para la demo». Se resuelve con la costura `SecondRound`
(S12): un tipo con un método y un campo de configuración, probado con un doble e instalado en nil.

L13. Decisión del owner sobre las etiquetas: no se sobrescriben. Se había sellado «se relabelea» y
resultó un problema de integridad de medición —relabelar al propio criterio convierte esos casos en
un espejo y apaga su capacidad de detectar una regresión—, así que se reportan dos números: acuerdo
con el juicio humano y distribución de emisión contra la resolución real del proyecto.

L14. El resultado que cierra la discusión: con el nodo instalado, la corrida reproduce el informe
anterior. `CANDIDATA` 183 (antes 183), `CONDICIONADA` 10 (9), `DERIVADA` 11 (13), `DUDA` 18 (17),
`MARCADA` 0 (0); precisión de `CANDIDATA` 92 % (91 %); recall 89 % (89 %); rechazos detectados 0/34
(0/34). Las diferencias son ruido de modelo ya documentado en el informe, no cambios de decisión.

L15. Las 37 etiquetas a mano no se tocaron: coinciden en 16 y las 21 restantes tienen causa medida,
ninguna es bug de política. Ocho son `question` que la etiqueta marcaba y la decisión sellada deriva
(`LIMITES.md` §3); cuatro dependían de las compuertas de §7.3 o del `intent` que salió de P1; y
nueve son el nodo de alineación juzgando distinto que la etiqueta humana, que es medición del modelo
y no de la cadena.

L16. Corrección de reporte encontrada al cerrar: el resumen contaba el bloque ambiguo (issues
cerradas sin PR mergeada que las respalde) en el denominador del acierto, y como ese bloque no
tiene etiqueta que la política pueda acertar, lo hundía a 0. Se separó: el acierto se mide sobre la
muestra ciega con resolución real, y el bloque ambiguo se reporta por distribución, que es lo único
que puede decir.

L17. Corrida de cierre, con el nodo instalado y el banco cableado al pipe de producción
(`GFI_INTEGRATION=1`, 279 issues, 203 s, ~0,7 s por issue porque 76 consultaron el nodo).

| Medición | `INFORME-TRIATJE.md` | Corrida de cierre |
|---|---|---|
| Muestra ciega (222 con resolución real) | 75 % de acierto global | **168/222** |
| `CANDIDATA` emitidas | 183 | **183** |
| `CONDICIONADA` | 9 | **10** |
| `DERIVADA` | 13 | **11** |
| `DUDA` | 17 | **18** |
| `MARCADA` | 0 | **0** |
| Precisión de `CANDIDATA` | 91 % | **92 %** |
| Recall sobre trabajo real | 89 % | **89 %** |
| Rechazos detectados | 0/34 | **0/34** |
| `kind` | 34/37 | **35/37** |
| `self_contained` | — | 37/37 |
| `has_evidence` | — | 27/37 |
| `unambiguous` | — | 29/37 |
| `done_defined` | — | 26/37 |
| `reports_defect` | — | 36/37 |
| `evidence_sufficient` | — | 36/37 |
| Bloque ambiguo (20 sin PR mergeada) | 11 CANDIDATA · 8 derivadas · 1 duda | **10 · 8 · 1 · 1 CONDICIONADA** |

Las tres corridas posteriores al arreglo dieron la misma distribución, y el subconjunto de
consistencia (4 casos × 3 pasadas) no movió ni una respuesta. Las diferencias con el informe son
ruido de modelo ya documentado por #14, no cambios de decisión: la cadena, los umbrales y las
preguntas son las mismas.

Lo que queda medido como perdido, y hay que decirlo en la demo: **el primer pase no detecta
rechazos** (0/34) porque `intent` salió de P1 (`LIMITES.md` §2), y **ocho preguntas quedan
derivadas** porque la segunda ronda no está instalada. Las dos cosas tienen su corte escrito y su
costura lista.
