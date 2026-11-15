# Feature: la etiqueta se ignora y la valoración es nuestra (reemplaza #35)

La etiqueta de dificultad deja de ser un objeto de estudio. No se valida si miente: se ignora.
El producto emite su propia valoración de si una issue es una buena primera contribución, y la
emite Clef sobre la evidencia del código, no sobre la opinión de un mantenedor.

Rama `feat/35-label-honest` · worktree `/src/worktrees/goodfirstissue/feat-35-label-honest` ·
nació como #35 y #35 se cierra: la rama conserva el nombre por historia, no por alcance.

## Specs

S1. **La etiqueta se ignora; no se audita.** El producto no emite ningún juicio sobre si la
etiqueta dice la verdad. El valor lo emite el sistema: «La etiqueta se ignora no se valida si
miente o no, el valor lo damos nosotros diciendo si es una good first issue, no me interesa si
tiene esa etiqueta o no. No validamos nada no perdemos tiempo en esto.»

S2. **La etiqueta no entra a ningún expediente ni a ninguna tarea de modelo.** Ni al expediente
de sustancia, ni al de intención, ni al de alineación, ni al `PromptFor` de explore. Se sigue
persistiendo **verbatim** en el corpus, porque el corpus conserva lo que el mantenedor escribió;
se guarda y no se juzga.

S3. **La etiqueta nunca selecciona ni filtra.** El único filtro sobre una issue sigue siendo
`state: OPEN`. Esta feature no cambia el fetch ni el corpus.

S4. **El pitch se reformula.** «El pitch se reforma, no "Validamos una label: good first issue"
nosotros le "Buscamos una good first issue"».

S5. **La valoración propia es determinista y vive en el `state` de P3.** Un bloque de
coherencia compara hechos que P1 declaró (`triage.Analysis.Answers`) contra hechos que P2 midió
(`explore.Evidence`) y escribe las contradicciones como texto. Cero peticiones, cero tokens:
mismo input, mismo output. **Emite hallazgos, no veredicto** — si dijera «no apta», Clef
confirmaría un juicio ya tomado y la decisión dejaría de ser suya (Reto 4, §3).

S6. **Cambiar un insumo del modelo cambia la pregunta que un resultado guardado responde**
(I14). Todo fingerprint que cubría el insumo cambiado se mueve: `triage.PolicyVersion` sube y
su declaración en `criteria.md` con él, y `explore.Version()` cubre el constructor de la tarea,
no solo el prompt y la skill.

S7. **#35 se cierra.** El fact-check de la etiqueta y cualquier pase de auditoría por modelo
salen del MVP y quedan en `LIMITES.md` como límite post-MVP.

## Tasks

| ID | Specs | Qué | Ruta | Commit |
|---|---|---|---|---|
| T1 | S2, S6 | Tests en rojo: ningún expediente de P1 puede llevar la etiqueta; `PolicyVersion` sube y `criteria.md` la declara | inline | 1 |
| T2 | S2, S6 | Quitar `labelSection` del expediente de intención y la etiqueta de `explore.PromptFor`; `explore.Version()` cubre el constructor | inline | 1 |
| T3 | S1, S4, S7 | `PROYECTO.md`: tesis, §1.3, §4.2, §7.4, §8.3, §11, §12, §13, §15, §16, §17.7 | inline | 2 |
| T4 | S5 | `PROYECTO.md` §9.1: el bloque de coherencia dentro del `state` de P3, con sus reglas | inline | 2 |
| T5 | S7 | `LIMITES.md`: el fact-check de la etiqueta como corte post-MVP; borrar `PassLabelHonest` del contrato | inline | 2 |
| T6 | S7 | Cerrar #35 con el motivo | inline | — |

## Log

L1. «Vamos a crear un wortree para trabajar en la issue #35»

L2. «Yo creo que le estamos dando demasiadas vueltas a esa etiqueta de good first issue, se que
es el nombre de nuestro proyecto pero no me interesa lo que otro considere good first issue, si
una issue tiene esa etiqueta o no la voy a evaluar igual que a cualquier otra issue, le voy a
hacer el mismo triagge la voy a explorar y la voy a contrarrestar ocn el cv. Es decir que esa
label para mi es basura. En todo caso podemos poner entre el 2 y el 3 una auditoria a la issue
nuestra, a todas las issues tengan etiqueta o no. no me interesa si la etiqueta miente, dice la
verdad para nosotros es como si no existiera, nosotros valoramos si es una good first issue
nadie mas, no quiero perder ni un token validando lo que algun pelotudo se le dio por poner.
Para el MVP considero que no nos da el tiempo de agregar esta auditoria pero permitime evaluarlo,
es decir hace una valoracion porque si usamos PiG como en el cv process y el explore y lanzamos
una corrida de un audit de algun agente y eso no es demasiado tiempo para la demo lo podemos
agregar. Si el problema es el tiempo que lleva hacer todo el pipe (triagge + explore + audit +
encaje) lo dejamos como opcional o lo ponemos como omitido para la hackaton porque en realidad la
hackaton se centra en cleff y el audit este es llm yo lo dejaria como limite post mvp»

L3. Decisión del humano sobre las tres preguntas del plan: «1. Si · 2. Cerramos. · 3. Si»

L4. Reformulación del pitch, verbatim: «La etiqueta se ignora no se valida si miente o no, el
valor lo damos nosotros diciendo si es una good first issue, no me interesa si tiene esa etiqueta
o no. No validamos nada no perdemos tiempo en esto. El pitch se reforma, no "Validamos una label:
good first issue" nosotros le "Buscamos una good first issue"»

L5. «sellado con tu recomendacion» — se sella A→D y entra el bloque de coherencia determinista
en el alcance de P3.

L6. Valoración registrada, con evidencia: el tiempo de corrida no es el costo (la demo es
precomputada, §15: `PRE-COMPUTADO 3 repos, 30 issues triajeadas, candidatas exploradas`; los
pasos en vivo son ~2 s y solo Clef). El costo es de horas: §2.1 cierra en 16 h contra ≈15,5 h
disponibles y ya está 0,5 h arriba, así que ningún pase nuevo por modelo entra en el MVP. Un
audit sin herramientas corre en ~5 s y precomputar 30 issues son ~2,5 min; lo caro es
construirlo bien. Y la razón de fondo: re-juzgar con un segundo modelo hechos que P1 y P2 ya
produjeron agrega una opinión, no información, y si esa opinión alimenta a Clef, Clef confirma.
