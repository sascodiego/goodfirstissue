# Límites y cortes por tiempo

Un solo lugar para lo que el sistema **no** hace, por qué, y qué costaría cerrarlo. Está
pensado como fuente para la presentación: cada entrada dice qué se cortó, qué se pierde
medido, y cuál sería el arreglo.

Fecha de los cortes: hackathon, noviembre 2026. Rama `feat/p1-bench`.

## Cortes por tiempo

### 1. Agrupación de duplicados: fuera del MVP

Decidir si dos textos describen el mismo problema es trabajo de un modelo de lenguaje sobre
cada par de candidatas, y no había tiempo. La unidad de evaluación es **la issue sola**.

- **Qué se pierde**: un reporte escueto que duplica a otro completo se juzga por su propio
  texto y puede quedar marcado como falto de contexto. Tampoco se puede avisar «esto ya está
  siendo trabajado» por la vía del grupo.
- **Qué se conserva**: la señal determinista de trabajo abierto por issue (PR enlazada
  abierta), que resuelve ese caso sin agrupar.
- **Dónde está escrito**: `PROYECTO.md` §2.3 y §7.1, `internal/triage/criteria.md`.

### 2. La intención del proyecto sale del primer pase

Saber si el proyecto ya aprobó o rechazó el trabajo no se decide en el triaje: pasa a la
segunda fase, donde un modelo de lenguaje lee la issue, toda su metadata y todos los
comentarios con la instrucción de priorizar la voz del owner e ignorar la del creador.

- **Qué se pierde, medido**: el primer pase **no detecta rechazos**. De 34 issues que los
  proyectos rechazaron, marca 0. Dieciséis de ellas salen `CANDIDATA`, o sea el 8,7 % de las
  candidatas que emite son trabajo que alguien ya rechazó.
- **Por qué se aceptó igual**: intentar la lectura de intención en el primer pase acertaba
  **3,7 %** sobre 188 issues con resolución conocida. No era una mejora recortable, era ruido.
- **El arreglo**: la segunda fase con LLM, ya especificada, sin implementar.

### 3. Segunda ronda para preguntas: especificada, no implementada

Un pedido clasificado como `question` no se resuelve en el primer pase: se deriva. Con los
comentarios se decide si la respuesta se transforma en trabajo (documentar un comportamiento
no documentado es una tarea).

- **Qué se pierde**: 12 de las 188 issues que el proyecto terminó con una PR mergeada eran
  preguntas, y quedan derivadas en vez de candidatas.
- **Cómo se ve en el producto**: la issue sale `DERIVADA`, un desenlace propio del primer pase:
  ni candidata ni descartada. Es lo que muestra la tabla de `PROYECTO.md` §7.7.
- **El arreglo**: la ronda dos, que es una petición más por issue.

### 4. Autoría de las etiquetas: verificada en la API, no incorporada

`timelineItems(itemTypes:[LABELED_EVENT])` devuelve el actor de cada etiqueta, así que un
`accepted` puesto por quien pide la issue se distingue de uno puesto por un mantenedor. Hoy
el fetch no lo trae.

- **Qué se pierde**: la etiqueta no puede decidir sola, porque no se sabe quién la aplicó.
  Diez casos del banco simulan el abuso (auto-etiqueta) para que la medición no lo olvide.
- **El arreglo**: agregar `LABELED_EVENT` y `actor{login}` al fetch de `internal/ghdata`.

### 5. `state_reason = DUPLICATE`: detectado, sin política

GitHub incorporó el motivo nativo `DUPLICATE` (7 casos en 200 cerradas de `cli/cli`). Es una
señal de duplicado que el MVP, sin agrupación, no tiene decidido cómo tratar. Queda fuera del
banco hasta que se decida.

### 6. Sin pantalla

El triaje no se ve en la aplicación todavía (issue #11). Los desenlaces existen como datos y
como banco de pruebas, no como interfaz.

### 7. Veredicto por partes: fuera del MVP

El veredicto de encaje es **uno por issue**. Las partes que produce el explore siguen existiendo
y se muestran, pero agrupan la evidencia: dicen qué áreas toca la issue y qué archivos y
requisitos cuelgan de cada una. No se puntúan por separado.

- **Qué se pierde**: una issue de tres cambios independientes se juzga por la **unión** de sus
  requisitos. Alguien que podría hacer la parte 1 sin problema puede recibir `needs_experience`
  porque las partes 2 y 3 piden cosas que no sabe. El veredicto se vuelve más conservador cuanto
  más grande la issue.
- **Qué se conserva**: la evidencia sigue completa en el `state` —todos los archivos, todos los
  requisitos, las lagunas— y las partes siguen visibles agrupando esa evidencia. Así el veredicto
  es explicable: se puede leer qué lo empujó y por dónde. Las issues de un solo cambio, que son la
  mayoría del corpus, no se ven afectadas.
- **Qué se agrava**: la recomendación de por dónde empezar deja de ser el mecanismo de evaluación
  y vuelve a ser lo que era, un consejo. La política ya no puede decir «empezá por la 2 de 3».
- **Dónde está escrito**: `PROYECTO.md` §2.1.1, §8.2, §9 y §9.1.

### 8. Diferidos por alcance, no por tiempo

`confirmed_defect` de P2 y `learning_value` de P3: cortados en §2.1.1 y no tocados acá.

**El nodo de alineación salió de esta lista**: se implementó y se midió en la PR del benchmark, y
desde entonces es la segunda petición de P1 para las features (`PROYECTO.md` §7.5). Los briefs con
los que se midió quedaron en `internal/triage/testdata/bench/project-brief/`, y producción los arma
desde el clon.

### 8. El fact-check de la etiqueta: fuera del MVP

**El producto no audita la etiqueta de dificultad.** No emite juicio sobre si dice la verdad: la
ignora. El veredicto de si una issue es una buena primera contribución lo emite P3 sobre la
evidencia de P2 (`PROYECTO.md` §1, §9), y la etiqueta no entra en ningún expediente ni en ningún
prompt.

Eso cierra una línea de trabajo entera, y el motivo es doble:

- **De producto.** Validar la opinión de un mantenedor no es asunto del sistema, y validarla cuesta
  tokens que el MVP no tiene. La etiqueta se guarda verbatim en el corpus porque es un dato, y no
  se juzga.
- **De tiempo.** §2.1 cierra en 16 h contra ≈15,5 h de foco y ya está 0,5 h arriba. Un pase más por
  modelo no entra: `internal/label` con su prompt, criterios, `Version()`, journal, un test por
  criterio y un banco con veredicto de referencia humano antes de correr son 2–3 h.

**Lo que sí quedó adentro, y es lo que reemplaza esa función**: el bloque determinista de coherencia
P1↔P2 dentro del `state` de P3 (`PROYECTO.md` §9.1.1). Compara hechos ya calculados, cuesta cero
peticiones, y emite hallazgos —nunca un veredicto— para que Clef siga siendo el que decide.

**Lo que se pierde y queda como límite post-MVP**: una métrica explícita de «cuánto miente el
ecosistema de etiquetas». `BENCHMARKS.md` §3 la reporta como hallazgo del estudio de corpus, no como
salida del producto.

## Límites técnicos medidos

No son cortes: son cosas que probamos y no funcionan como suponíamos.

| Límite | Evidencia |
|---|---|
| `kind` es la única pregunta con señal fuerte | 34/37 y **+17 casos** sobre contestar siempre la clase mayoritaria |
| `evidence_sufficient` no aporta | ganancia **0** sobre el baseline; lo único que atrapaba era el cuerpo vacío, que ahora resuelve la pre-compuerta |
| `self_contained` es peor que decir siempre «sí» | 32/34 contra un baseline de 33/34 |
| `has_evidence` sólo sirve donde se consulta | 24/34 en bruto, pero **9/10** en los casos donde la política la usa (bugs) |
| `done_defined` y `unambiguous` son el punto flojo | 25/34 y 30/34; una segunda redacción las empeoró. Se usan en disyunción y el error cae del lado seguro (CONDICIONADA) |
| El árbol derivado de hechos pierde contra la pregunta compuesta | 27/37 contra 34/37 |
| El banco no es neutral | la mitad de los casos son sintéticos o derivados; el bloque de rechazadas mudas es basura del buzón de `cli/cli` (`brew install`, `Xxx`, correos), no una prueba de clasificación |
| La etiqueta «completada» es débil | sólo el 17 % de las cerradas como completadas tiene una PR mergeada que las cierra. El bloque sin corroboración (20 casos) mide ambigüedad, no acierto: 11 salen candidatas y ocho parecen trabajo real resuelto por otra vía |
| Los ids del corpus colisionaban | `httpie/cli` y `cli/cli` compartían el prefijo `c-cli-`, así que un caso podía pisar a otro. Corregido a `owner-name-número`, con aserción de unicidad |
| Ruido entre corridas idénticas | `has_evidence` dio 21/34 y 20/34 con la misma redacción: diferencias de un caso no son señal |

## Dos errores de medición propios, registrados

Para que la presentación no venda más de lo que hay:

1. El reporte reutilizó una variable del conteo de categoría y publicó `flat kind 23/37`
   cuando el valor real era `34/37`.
2. Las issues cerradas se anotaban dos veces en la lista de corridas y el denominador del
   desenlace salía en 115 en vez de 76.

Los dos están corregidos y los números de `BENCHMARKS.md` salen de recalcular sobre el
reporte guardado, no de esas salidas.
