# Feature: congelar la redacción de P1 (issue #14)

Contrato de la primera pasada: qué preguntas se le hacen al juez, en qué orden deciden las
compuertas, y qué evidencia entra al expediente. El laboratorio que mide la redacción antes
de congelarla. Sin UI y sin política implementada: eso es #9.

Rama `feat/p1-bench` · worktree `/src/worktrees/goodfirstissue/feat-p1-bench` · issue #14.

## Specs

S1. `criteria.md` congela el vocabulario (`github_state`, `github_state_reason`, dossier,
`evidence`), las tres capas y la regla determinista-primero: si un `if` lo resuelve, no
gasta una pregunta.

S2. La etiqueta nunca es determinista —su significado depende del repositorio y no registra
quién la aplicó—, nunca decide sola, y **no aparece en el expediente de sustancia**. En el
expediente de intención entra verbatim, marcada como afirmación de autoría desconocida.

S3. La cadena es `G0 evidence_sufficient → G1 trabajo abierto → G2 calidad → G3 kind → G4
intent`, con el umbral de 0,5 en cada criterio, y **G1 precede a G0**: un hecho determinista
no depende de poder clasificar el texto.

S4. Las cuatro abstenciones llevan umbral y etapa que las emite: `no_verdict`
(`evidence_sufficient` < 0,5, P1), `uncertain` (p < 0,45), `undecided` (fallo del endpoint,
I2) e `insufficient_information`, que es de P3 por falta de perfil y P1 no emite.

S5. P1 no responde «qué te falta»: eso es P2 (requisitos en tres baldes, lagunas) y P3.

S6. Dos expedientes: A de sustancia, sin etiquetas y sin la declaración del autor en el
título; B de intención, con etiquetas verbatim, `state_reason` y comentarios de mantenedor.
Las pre-compuertas nativas corren antes de cualquier juicio.

S7. El texto de terceros viaja delimitado y rotulado como evidencia, nunca instrucciones
(I3), y los recortes son visibles.

S8. El banco tiene 23 casos etiquetados a mano con procedencia `real` o `synthetic`, y el
manifiesto declara la cobertura exigida: una trampa listada sin caso hace fallar un test.

S9. La medición reporta acierto por pregunta, abstención, consistencia con tres pasadas y
guarda esperado contra obtenido. `make verify` queda hermético sin `GFI_INTEGRATION` y
`NAN_KEY`.

S10. La redacción congelada en `prompt.md` es la que ganó la medición, y una guarda falla si
el contrato y el código que lo envía divergen.

S11. Ningún archivo del contrato nombra modelos, endpoints ni código.

## Tasks

| ID | Tarea | Specs | Ruta | Commit |
|---|---|---|---|---|
| T1 | Vocabulario, capas, regla determinista-primero, etiqueta, dos expedientes, cadena de compuertas, abstenciones, presupuesto | S1-S5 | inline | `eb7ea20` |
| T2 | Banco: 23 casos y manifiesto con cobertura y procedencia | S8 | inline | `c0a5959` |
| T3 | Pre-compuertas, normalización del título y expedientes A y B, con tests offline | S6, S7 | worker | `0af43c4` |
| T4 | Medición: banco de preguntas, cliente serial, comparación, consistencia, reporte | S9 | inline | `9137d5e` |
| T5 | Redacción congelada, tabla de lo que el banco decidió y guarda contra la deriva | S10, S11 | inline | `a51e613` |

Riesgo: **ítem 6** — la redacción es el producto y ningún test la cubría; el banco y la
guarda del contrato son la mitigación.

## Log

L1. Pedido original, verbatim:

> Vamos a trabajar con la siguiente metodologia, vamos a leer una issue, analizarla planificarla y
>   cuando yo te de mi concentimiento explicito recien ahi podes empezar a escribir codigo. vas a trabajar
>  en un worktree nuevo. la issue es la 14

L2. La issue pedía solo dos `.md`. El owner amplió el alcance: laboratorio primero, redacción
al final, y «más casos, son baratos». Se registró como desviación sellada.

L3. Decisión del owner sobre precedencia: **G1 gana a G0**. Contradice el orden literal de la
cadena de #14 y queda escrito en `criteria.md` con su motivo.

L4. Decisión del owner sobre la etiqueta: es información para el juez, no un filtro. Se
resolvió con dos expedientes: la etiqueta no puede contaminar la sustancia porque no la ve,
y aporta en la intención porque ahí sí está.

L5. Premisa del owner puesta a prueba: «un árbol de decisiones da mejor criterio que una sola
solicitud de clasificación». Medición: 22/23 la clasificación plana, 12/23 el árbol en su
primera redacción y 18/23 reescrito. **No se adopta.** Mecanismo: el hecho «pide
comportamiento que hoy no existe» daba 0,83–0,94 en todos los bugs, porque un arreglo también
produce comportamiento que hoy no existe.

L6. `unambiguous` y `done_defined` son el punto débil medido (19/23 cada una); una segunda
redacción las empeoró (13/23 y 16/23). Se congelan en su mejor versión, se mantienen en
disyunción y el error cae del lado seguro: CONDICIONADA de más, nunca descarte.

L7. Ruido observado: `has_evidence` dio 21/23 y 20/23 con la misma redacción. Las decisiones
se tomaron con umbral de 3 casos, no de 1. La segunda pasada se escribió mirando la primera:
su mejora incluye ajuste al set, y cualquier redacción futura se mide sobre casos nuevos.

L8. Agrupación de duplicados **fuera del MVP** por decisión del owner: decidir si dos textos
reportan el mismo problema es trabajo de un modelo de lenguaje sobre cada par, y no hay
tiempo. Documentado en `PROYECTO.md` §2.3 y §7.1, y en `criteria.md`. La unidad pasa a ser
la issue sola; I12 no aplica en el MVP.

L9. `intent` reescrita: opciones `approved` / `refused` / `undecided`, todas en positivo y
con el criterio de qué cuenta como evidencia; se quitó de la pregunta el caso nativo que ya
resuelve la pre-compuerta, y se sumó el nodo `maintainer_spoke` para separar «nadie habló» de
«alguien habló sin decidir». Sobre 34 casos medidos acierta 33/34.

L10. Casos inválidos corregidos: el disfraz de feature ya no confiesa que la capacidad no
existe; el caso de pregunta se reemplazó por un reporte de crash que pregunta si el
comportamiento es intencional; y los casos con estado nativo cerrado, cuerpo vacío o PR
abierta dejaron de medirse con el modelo, porque la pre-compuerta los resuelve antes: ahora
el banco los reporta como «resueltos sin preguntar».

L11. El banco creció a 46 casos, 35 reales de los dos repositorios del corpus. Con datos
reales el acierto de punta a punta es 34/46 (74 %), no el 96 % que sugería el set sintético:
`done_defined` y `unambiguous` son responsables de seis de las doce fallas.

L12. Diagnóstico del owner, aceptado: la pregunta de intención no estaba mal redactada,
estaba **mal alimentada**. Se le preguntaba por la posición del proyecto y se le pasaba la
issue entera. `IntentDossier` ahora lleva sólo lo que puede responderla: estado nativo,
etiquetas verbatim, una línea con lo que se pide, y los comentarios de cuentas con
asociación de responsable — más la definición de qué significan OWNER, MEMBER y
COLLABORATOR, que antes nunca se entregaba.

L13. El árbol que propuso el owner quedó implementado como recorrido de nuestro código:
aprobada → candidata (fin); rechazada → marcada (fin); sin posición → si es bug, sigue; si
es feature, nodo de alineación con la dirección real del proyecto (README + últimos 20
commits del clon, en `testdata/bench/project-brief/`).

L14. Medición del árbol: `intent` 36/37 contra un baseline constante de 34/37 (atrapa los
dos rechazos en prosa y comete un falso rechazo); el nodo de alineación acierta 6/10, con
dos casos exactamente en el umbral de 0,50 (cada eje por separado acierta 8/10, la
conjunción castiga); y el desenlace árbol 22/37 contra plano 24/37, comparación que **no
vale** todavía porque las etiquetas de desenlace se escribieron bajo la política plana.

L15. Bug de medición propio, registrado: el bloque de reporte reutilizó la variable del
conteo de categoría y publicó un número equivocado en una corrida. Corregido; los números
de arriba son de la corrida posterior.

L16. Banco cerrado con etiqueta real: 350 issues cerradas de los dos repos, de las cuales
se usan 39. El hallazgo que obliga a desconfiar de la premisa: **sólo el 17 % de las
cerradas como completadas (58 de 350) tiene una PR mergeada que las cierra**, y **sólo 4
de 26 rechazadas tienen explicación de un mantenedor**. La etiqueta del cierre dice qué
decidió el proyecto, no si la issue era buena. Por eso el set se parte por fuerza de
etiqueta: 24 fuertes y 15 débiles, y sólo las fuertes cuentan para el titular.

L17. `intent` contra etiqueta real: **2/24 en las fuertes** (22 veces «sin posición», 2
veces «aprobada»), 1/15 en las débiles. Desglose por causa: con comentario de mantenedor
acierta 3 de 12; **sin comentario, 0 de 27**. La decisión del proyecto casi nunca está en
el texto que vemos, y el acierto del 36/37 en el set abierto era el prior de «sin
posición». Conclusión: `intent` no puede ser pregunta del modelo; la capa nativa
(`state_reason`) es la única fuente confiable.

L18. Umbral del nodo de alineación medido: **0,50 → 6/10 · 0,60 → 9/10 · 0,70 → 8/10**.
Se adopta 0,60, que es lo que `PROYECTO.md` §7.5 ya decía. El fallo de `cobra#2232` expone
un defecto de redacción del nodo: preguntar «¿es una situación específica de quien lo
pide?» hace que el modelo se ancle en la herramienta del que pide, cuando la pregunta era
si otros usuarios del proyecto se benefician.

L19. Bug de reporte propio, segundo: las cerradas se anotaban dos veces en la lista de
corridas y el denominador del desenlace salía en 115 en vez de 76. Corregido; los números
citados salen de recalcular sobre el reporte guardado, no de esa salida.

L20. Semántica de `intent`, confirmada por el owner: responde una sola cosa, **¿el proyecto ya
decidió?** con tres valores — aprobado, rechazado, sin posición. Una etiqueta o un comentario
que afirme aprobación o rechazo es una decisión tomada. **`sin posición` no es nunca un estado
final**: cae al nodo siguiente del árbol.

L21. Defecto de alimentación encontrado y corregido: el expediente de intención se quedaba con
los **últimos tres** comentarios de mantenedor, y la aprobación suele ser la primera respuesta.
Ahora los comentarios con lenguaje de decisión tienen prioridad sobre los más recientes, y el
tope sube a seis. La corrida siguiente dirá si la causa principal era ésta o la ausencia de
material.

L22. Defecto de alimentación que **no** podemos corregir en el triaje: la etiqueta no registra
quién la aplicó, porque el fetch no trae el actor de `LabeledEvent`. Para honrar la regla «si
la etiqueta afirma aprobación, hay decisión», el fetch de E1 tiene que traer ese actor; sin él,
una etiqueta que se puso el propio autor vale lo mismo que una del mantenedor.

L23. Decisión del owner sobre el bug sin reproducción: **MARCADA**, con el motivo y marcada para
revisión posterior del modelo en la etapa de exploración. No es terminal: que no se pueda
reproducir no significa que no se pueda encontrar en el código, y esa ambigüedad se resuelve
más adelante con evidencia, no en P1.

L24. Dato nuevo del tracker: `state_reason` ahora incluye `DUPLICATE` (7 casos en `cli/cli`).
Es una señal nativa de duplicado que no estaba contemplada; queda fuera del banco hasta que se
decida qué significa para el MVP sin agrupación.
