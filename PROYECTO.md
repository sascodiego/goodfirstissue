# PROYECTO.md — GoodFirstIssue

**Versión:** 2.22
**Fecha:** 2026-11-13
**Estado:** Documento rector del MVP para el #2 Hackathon NaN (Reto 4 — tema libre)
**Reemplaza:** el documento de definición de producto y arquitectura v1.0, en todo lo relativo al **alcance del MVP** (sus secciones 13 y 18). La visión de producto, el análisis del problema y los principios de diseño del original siguen vigentes.

**Historial**
- **2.22** — **la etiqueta de dificultad se ignora; no se audita** (§1). Sale `label_honest` de P2 (§8.3), y la etiqueta sale de todo expediente y de todo prompt: se guarda verbatim en el corpus y no se juzga (I6, §4.2). El pitch pasa de «validamos una label» a «buscamos una good first issue». La valoración propia entra como bloque determinista de coherencia P1↔P2 dentro del `state` de P3 (§9.1.1): emite hallazgos, no veredicto. `triage.PolicyVersion` sube a `p1-2` y `explore.Version()` pasa a cubrir el constructor de la tarea (I14).
- **2.21** — el paso de «¿ya fue atendida antes?» más la agrupación de duplicadas se ubica **después** del triaje y **antes** del explore: el triaje juzga la issue sola y deja candidatas. Del paso, el MVP solo conserva la señal determinista de trabajo abierto por issue (§7.1).
- **2.20** — la agrupación de duplicados **sale del MVP por tiempo** (`LIMITES.md` §1). **I12 se reformula**:
  la unidad de evaluación es la issue sola. No se reescribe la entrada 2.1: registra lo que se decidió entonces.
- **2.19** — el proyecto pasa a repositorio propio (`sascodiego/goodfirstissue`), no un fork. Sin remote hacia el repositorio oficial del hackathon: la ruta para abrir ahí una issue o un PR no existe. El historial completo y el material original se conservan; la procedencia queda declarada en `NOTICE.md` (§18).
- **2.18** — persistencia de análisis (§17.9): registro de repositorios (`gfi repos`) y journal de decisiones append-only con bloqueo `flock`. El caché se indexa por `(issue, pasada, modelo, versión del prompt, commit)` (I14): la segunda corrida de la misma issue pasó de **1m49s a 0,023s**. Medido: SQLite puro Go cuesta 26 módulos y ~5 MB de binario; DuckDB 72 módulos, 34 MB y cgo, así que queda descartado.
- **2.17** — persistencia y clonado (§17.3): `internal/store` con envoltura de esquema y escritura atómica, `internal/repo` con clonado shallow idempotente, `gfi fetch` y `gfi index`. Corpus real de dos lenguajes. El nombre canónico de GitHub resuelve renombrados (§5.2).
- **2.16** — sistema de configuración (§17.7 reescrita): JSON con defaults, validación que rechaza modelos fuera del allowlist y herramientas de escritura, duraciones legibles, y `config.example.json` generado desde el código con test de deriva. Los tres wrappers implementados (§17.3).
- **2.15** — doctrina y contrato fusionados en un solo `prompt.md` (§6.3). Comparación medida: +8% de tiempo, +2 archivos, call_paths de 4 a 8, y encuentra `rpad` que antes se perdía. La regla de no leer `.git/` funciona.
- **2.14** — aclarado el mecanismo de skill (§6.1): `--no-skills` no bloquea `--skill`, pero el agente no lee la skill por su cuenta (medido: 0 lecturas). La doctrina va en el system prompt; la skill queda re-legible.
- **2.13** — corregido el diagnóstico de sobre-exploración: las 24 llamadas son investigación dirigida (§6.3). Portabilidad: la config la genera la app, PiG nunca lee `~/.pig` (§6.8). Concurrencia separada por motor: Clef serial, chat paralelo hasta 4 verificadas (§6.9). Vigilancia de ociosidad por silencio del pipe (§6.10).
- **2.12** — invocación verificada end-to-end (§6.1): allowlist con la herramienta MCP, `--no-extensions -e builtin:mcp`, skill + contrato por `--append-system-prompt`. `codegraph_explore` por MCP confirmado, portabilidad por `PIG_HOME` resuelta, costos medidos (79 s, 14 turnos).
- **2.11** — el explore es un **agente delegado** verificado (§6 reescrito): PiG headless contra NaN, allowlist `read,ls,grep`, JSONL parseable, árbol limpio. `codegraph` entra por MCP, no por shell. `observed` se valida contra la traza.
- **2.10** — corregido el rol de `codegraph`: no es un pipeline determinista en Go sino una **herramienta del agente de explore** (§6 reescrito). Tool calling nativo verificado en los cinco modelos del allowlist. El único filtro sobre una issue es `state: OPEN`; la etiqueta nunca selecciona (§4.1).
- **2.9** — integración con GitHub y `codegraph` verificados con mediciones (§5.1, §6.4). Cuatro trampas del indexador medidas. *Superado por 2.10: el corpus no se filtra por etiqueta y no hay compuerta de solapamiento.*
- **2.8** — ruta de ejecución sugerida (§2.1.2): corpus primero, con compuerta del sábado a las 14:00 y regla del video.
- **2.7** — evaluación de tiempo real: el plan baja de 23 h a 16 h de Tier 1 contra ~15,5 h de foco disponible. Se registra lo cortado con su costo (§2.1.1). El CV queda en texto pegado; PDF y URL diferidos.
- **2.6** — el CV entra en tres formas (texto, PDF, URL); se registra como requisito abierto sin diseñar. Reabre la decisión previa que descartaba la extracción de PDF/DOCX.
- **2.5** — un modelo por acción en `config.json`; las acciones de decisión quedan fijas en `clef`. Se nombra `profile_extract`, el paso que faltaba para convertir el CV en capacidades (§17.8).
- **2.4** — lista constitucional de modelos: solo cinco flash más `clef`; la lista es constante en código y el arranque falla cerrado (I13).
- **2.3** — primera medición contra la API real: la higiene del `state` (normalizar el título) queda como ley; el benchmark incorpora casos adversariales; se documenta el contrato real de Clef.
- **2.2** — se fija el stack concreto y los prerequisitos ejecutables (cero módulos de terceros).
- **2.1** — la agrupación de duplicados entra como primer trabajo de P1: la unidad de evaluación deja de ser la issue y pasa a ser el grupo; nueva invariante I12; se incorpora la marca «ya está siendo trabajada». **Revertida en 2.20 y 2.21.**
- **2.0** — flujo reestructurado en tres pasadas con embudo de costo; el triaje pasa a ser la primera etapa y corre sin LLM; se incorporan calidad de la issue, líneas de decisión cerradas y abiertas, nodo de alineación y guardarraíl de feature disfrazada de bug.
- **1.4** — los antecedentes se limitan a literatura académica.
- **1.3** — la aptitud queda restringida a requisitos técnicos; la promesa 4 de la etiqueta queda en disputa.
- **1.2** — triaje como árbol de compuertas de un eje; desaparece la escala de dificultad y el riesgo como criterio de aptitud.
- **1.1** — E2 reutiliza `codegraph` en lugar de construir un indexador.

**Regla de lectura:** ante cualquier contradicción entre este documento y el original, manda este.

---

## 1. Qué se construye

Una aplicación que **decide si una issue es una buena primera contribución** —con la evidencia del código a la vista— y si una persona concreta cubre los requisitos técnicos que esa issue exige.

La afirmación central, en una línea:

> **La etiqueta no decide: nosotros decidimos.** Que alguien haya escrito `good first issue` es su opinión sobre la dificultad; el veredicto sale de lo que la issue exige del código, verificado contra el árbol.

**La etiqueta no se audita: se ignora.** El producto no emite juicio sobre si la etiqueta dice la verdad. La etiqueta no entra en ningún expediente, no llega a ningún modelo y no mueve ninguna decisión (I6). Se guarda verbatim en el corpus, porque el corpus registra lo que el mantenedor escribió y nada más. El producto *es* el juicio que el mantenedor no da.

Los criterios con los que juzgamos, separados por si están en disputa:

| Criterio | Pregunta | Estado |
|---|---|---|
| Es un trabajo, no una consulta | ¿pide un cambio en el código, o es una pregunta, una discusión, un duplicado? | firme |
| Está especificada | ¿define qué comportamiento se espera y qué significaría estar terminada? | firme |
| Es auto-contenida | ¿es un solo cambio coherente, o son varios independientes? | firme |
| No exige conocimiento profundo del proyecto ni del proceso | ¿cuánto contexto del proyecto y del proceso hace falta? | **en disputa** |

El cuarto queda fuera del veredicto y se registra como atributo `onboarding_cost`. Ver §1.2.

El producto tiene una sola pantalla protagonista: una issue, un perfil, un veredicto con probabilidades y la evidencia que lo sustenta.

**No existe una escala de dificultad.** No hay «Principiante / Intermedio / Experto». Hay una lista de **requisitos concretos** que la issue exige y una lista de **capacidades concretas** que la persona tiene. El veredicto compara esas dos listas. Una escala única obliga a ubicar a todo el mundo en un punto de una recta imaginaria, y ahí es donde el sistema empieza a mentir: alguien puede ser experto en Go y principiante en Open Source al mismo tiempo.

### 1.1. Cumplimiento de la regla del Reto 4

> «Clef tiene que tomar la decisión central de tu app. Si quitas Clef, tu app deja de funcionar o pierde su razón de ser.»

Si se quita Clef, el embudo no arranca: la primera etapa —la que filtra un repositorio entero antes de gastar un solo token de modelo generativo— desaparece, y también la última. El producto queda mudo, no degradado.

Corolario operativo: la decisión **no puede ser la agregación de puntajes producidos por el propio pipeline**. Ver §3.

### 1.2. La aptitud es técnica y solo técnica

**Principio, no preferencia.** Los requisitos se separan en tres baldes (§8.2) y **solo `knowledge` participa del veredicto de encaje**. `project` y `process` se reportan como **costo de onboarding**: se muestran, informan el orden de las recomendaciones, y no aprueban ni rechazan a nadie.

El argumento, en la versión más fuerte:

> Si tener la capacidad técnica alcanza para resolver la issue, ser nuevo no puede ser un criterio de aceptación. El conocimiento del proyecto y del proceso se cierra **haciendo el trabajo** — eso es lo que significa onboarding. Tratarlo como brecha de capacidad castiga a la persona exactamente por ser nueva, y este producto dice servirle. Además, la decisión de mergear es del owner: nosotros decidimos aptitud, no autorización.

Este principio tiene una consecuencia incómoda y deliberada: un sistema así **no puede decir «no estás listo» a alguien que sabe programar**. Puede decir «esto exige saber X y tu perfil no muestra X», que es información técnica, no un juicio sobre la trayectoria de nadie.

**El criterio 4, en disputa.** Se sostiene la posición contraria: que «buena primera contribución» **no habla de una persona sino de la issue**, y que «buen punto de entrada» implica por definición bajo prerrequisito de contexto, porque si no la recomendación no significa nada. Esa es una afirmación *sobre la issue*, verificable sin tocar la aptitud de nadie.

No se resuelve redactando mejor: se resuelve midiendo (§12). Mientras tanto, el criterio 4 queda fuera del veredicto y se registra como atributo `onboarding_cost`.

### 1.3. Antecedentes y diferenciación

**La tesis no es nueva, y no la reclamamos como propia.** Está en la literatura académica sobre incorporación de nuevos contribuyentes a proyectos Open Source:

| Trabajo | Qué establece |
|---|---|
| Tan et al., *A first look at good first issues on GitHub* — [FSE 2020](https://doi.org/10.1145/3368089.3409746) | los GFI existen, y muchos recién llegados fracasan igual |
| *Characterizing and Predicting Good First Issues* — [ESEM 2021](https://www.eecs.yorku.ca/~wangsong/papers/esem21b.pdf) | se pueden predecir etiquetas GFI automáticamente |
| Alderliesten & Zaidman, *An Initial Exploration of the GFI Label* — [CHASE 2021](https://doi.org/10.1109/chase52884.2021.00023) | la etiqueta sirve para onboarding, pero su efecto cambia con el tiempo |
| Horiguchi et al., *Onboarding to OSS with Good First Issues* — [SANER 2021](https://doi.org/10.1109/saner50967.2021.00054) | análisis de GFI y barreras de entrada |
| **RecGFI**, *Recommending good first issues* — [ICSE 2022](https://doi.org/10.1145/3510003.3510196) | recomendación de GFI a partir del contenido de la issue |
| **Xiao et al.**, *Personalized First Issue Recommender* — [FSE 2023](https://arxiv.org/abs/2308.09038) | «mismatches between generic GFIs and the diverse background of newcomers» — **la tesis de este producto, ya publicada** |

Lo que este MVP agrega sobre esos trabajos:

1. **Evidencia anclada a un commit concreto y verificada contra el árbol.** Los trabajos previos derivan características del texto de la issue, de sus metadatos y de la historia del repositorio. Acá la evidencia es una referencia a archivo y línea que se puede abrir, con la regla de que una relación no observada no puede afirmarse (§8.2).
2. **Probabilidad y abstención explícita.** El patrón habitual en este espacio es emitir un puntaje o un porcentaje sin calibrar. Acá se emite una probabilidad con umbral definido y existe una salida propia para «no alcanza la evidencia» (§7.7).
3. **Un modelo de decisión como capa de decisión**, no un LLM generativo: reproducible, barato, rápido y auditable (§3.1).
4. **El veredicto lo emite la evidencia, no el mantenedor.** La etiqueta de dificultad no entra en el cálculo: el sistema decide por su cuenta si la issue es una buena primera contribución (§1).

Consecuencia para el pitch: no vender novedad conceptual. Vender calidad de decisión y evidencia verificable.

---

## 2. Flujo: tres pasadas y un embudo de costo

El sistema no es una secuencia de etapas: es un **embudo**. El recurso caro es el modelo generativo, y el diseño consiste en no gastarlo en lo que no lo merece.

| Pasada | Insumo | Motor | Coste por unidad |
|---|---|---|---|
| **P1 · Triaje** | texto de la issue, metadatos, etiquetas, comentarios | **solo Clef** | ~1,5 s · **0 tokens de LLM** · 1 request |
| **P2 · Explore** | + índice de código | LLM + `codegraph` | caro |
| **P3 · Encaje** | + perfil del desarrollador | **solo Clef** | ~1,5 s por issue |

Consecuencia estructural: **P1 no necesita modelo generativo**. Un repositorio con 30 issues abiertas se triajea en ~45 s, en serie, con cero tokens de LLM. El primer filtro de toda la aplicación es Clef y nadie más.

### 2.1. Etapas y presupuesto

| Etapa | Qué hace | Horas |
|---|---|---|
| **E0** | Corpus: 1–2 repositorios, clonados e indexados, ~20 issues con su etiqueta original | 1,5 |
| **E1** | Datos y clon: fetch GraphQL, caché en disco, SHA fijo, inventario | 1,5 |
| **E2** | Herramientas del agente: catálogo, ejecutor, loop de tool calling (§6) | 1 |
| **E3** | Triaje P1: compuertas y calidad sobre la issue sola, sin nodo de alineación (§7) | 2,5 |
| **E4** | Explore P2: prompt del agente y esquema de salida, sin `confirmed_defect` (§8) | 1,5 |
| **E5** | Encaje P3 + `profile_extract`, sin `learning_value` (§9) | 2 |
| — | UI: embudo y veredicto | 3,5 |
| — | Cola serializada y degradación | 1 |
| — | Demo, README, video de respaldo, benchmark de 6 casos | 1,5 |
| | **Total** | **16** |

Presupuesto realista medido el viernes a las 22:56: 42,1 h de reloj − 14 de sueño − 3 de comidas y pausas = **≈15,5 h de foco**. El plan queda **0,5 h por encima**: es la primera vez que está al alcance, y sin colchón cómodo.

Cualquier trabajo fuera de esta tabla se paga quitando algo de esta tabla.

### 2.1.1. Lo que salió del alcance para entrar

Registrado con su costo, para que volver a meterlo sea una decisión y no un olvido.

| Corte | Ahorra | Qué cuesta |
|---|---|---|
| ~~Nodo de alineación (§7.5)~~ | — | **entra al MVP**: se implementó y se midió en la PR del benchmark, y es la rama que separa una feature que el proyecto quiere de una que no. Ver §7.5 |
| Tercera pantalla de detalle | 1 h | la evidencia se muestra inline en el veredicto; el contenido es el mismo |
| `confirmed_defect` (P2) | 0,5 h | el guardarraíl queda solo con `reports_defect`; medido, funciona con el título limpio (§7.2.1) |
| `learning_value` (P3) | 0,5 h | se pierde el eje de crecimiento; no cambia ningún veredicto |
| Veredicto por parte en P3 | 0,5 h | una issue de tres cambios independientes se juzga por la **unión** de sus requisitos: quien podría hacer la parte 1 recibe `needs_experience` por la 2 y la 3. El veredicto se vuelve más conservador cuanto más grande la issue (`LIMITES.md` §7) |
| Tercer repositorio | 0,5 h | menos variedad en el embudo, no menos historia |
| Benchmark de 10 → 6 casos | 0,5 h | dos disfraces adversariales en vez de tres |

**Orden de retorno si sobra tiempo:** primero la segunda ronda de `question` (la costura ya existe, ver §7.7); después la tercera pantalla; al final el tercer repositorio y los casos extra.

**Nada de esta lista se implementa sin que el Tier 1 esté cerrado y verificado.**

### 2.1.2. Ruta de ejecución sugerida

El orden importa tanto como el contenido: hay trabajo con incógnitas **externas** que no controlamos y trabajo con incógnitas **internas** que podemos postergar. Lo externo va primero.

| # | Unidad | Horas | Acumulado | Terminada cuando |
|---|---|---|---|---|
| 1 | **Corpus congelado** (E0+E1+E2) | 4,0 | 4,0 | `var/store/<repo>/snapshot.json` con las issues abiertas e índice construido — **hecho**: 150 issues y 41/178 archivos indexados en los dos repos |
| 2 | **P1 · triaje** (E3) | 2,5 | 6,5 | las 20 issues clasificadas, cero llamadas al modelo de chat, < 120 s, y un test por cada fila de §7.7 |
| 3 | **UI · embudo** (parte de UI) | 2,0 | 8,5 | el embudo muestra los números reales del corpus, no fixtures |
| 4 | **P2 · explore** (E4) | 1,5 | 10,0 | cada candidata tiene `EvidencePackage` válido, con partes que particionan la evidencia |
| 5 | **`profile_extract` + P3 · encaje** (E5) | 2,0 | 12,0 | 5 veredictos con perfiles distintos, uno por issue |
| 6 | **UI · veredicto, cola y degradación** | 2,5 | 14,5 | la ruta `undecided` es demostrable cortando la red |
| 7 | **Benchmark, README y video de respaldo** | 1,5 | 16,0 | 6 casos con 2 disfraces, README con cómo correrlo, video de 90 s grabado |

#### Por qué el corpus primero, y no la maqueta

1. **Las incógnitas externas no se negocian.** El fetch de GitHub, si `codegraph` soporta el lenguaje del repo, si el SHA se resuelve, si las issues traen los comentarios: nada de eso lo decidimos nosotros. Los retrasos grandes vienen de ahí, y descubrirlos el sábado a las 20:00 cuesta el proyecto. La UI es incógnita interna: se puede hacer de noche y rápido.
2. **El embudo es la demo.** Sus números son el argumento. Diseñarlo antes de saber qué forma tiene un dato real es diseñarlo dos veces.
3. **El corpus es el fixture de todo lo demás.** Los tests de P1, P2 y P3 se escriben contra él. Sin corpus se testea contra datos inventados, que siempre pasan.
4. **Produce un artefacto durable.** Un corpus congelado en SHA fijo sirve el domingo aunque el código cambie.

**Contraargumento, y por qué no gana:** empezar por la UI da una victoria visible temprano y elimina el escenario «no hay nada que mostrar». Es legítimo, pero el embudo es la unidad 3, a 8 h del inicio — no al final. El riesgo que evita es menor que el que crea.

#### Compuerta del sábado a las 14:00

> Si a las 14:00 del sábado el corpus y el triaje no están terminados, **se aplican los cortes de §2.1.1 en ese momento**, no el domingo.

Es el único punto de control duro del fin de semana, y está puesto donde todavía queda tiempo para reaccionar.

#### Regla del video

> El video de respaldo se graba **antes de la última hora**, cuando el flujo funciona aunque falte prolijidad.

Es lo único que convierte un desastre técnico de último momento en un pitch decente, y cuesta 15 minutos de las 16 h.

### 2.2. Grafo de decisiones

```
repo ─► [E1 datos + clon] ─► [E2 indexar]
                                  │
   ┌──────────────────────────────▼──────────────────────────────────┐
   │  P1 · TRIAJEAR — Clef solo, 0 tokens LLM, 0 código              │
   │  la issue sola: kind · intent · self_contained · has_evidence · │
   │  unambiguous · done_defined · reports_defect ·                  │
   │  evidence_sufficient                                            │
   └───────┬──────────────────────────┬──────────────────────┬───────┘
           │                          │                      │
   evidence_sufficient<0,5      calidad < 0,5           kind=trabajo
           │                          │                      │
           ▼                          ▼                      ▼
     D U D A  E N            MARCADA, no candidata    ┌──────────────┐
     V O Z  A L T A          con lo que le falta      │    intent    │
     (no se descarta)                                 └──┬────┬───┬──┘
                                                        │    │   │
                                          authorized ───┘    │   └── declined
                                          (cerrada)      neutral       (cerrada)
                                                     (abierta)            │
                                                            │             ▼
                                                            ▼           fuera
                                     ┌──────────────────────────────┐
                                     │  feature + neutral           │
                                     │  bug     + neutral → sigue   │
                                     └──────────────┬───────────────┘
                                                    ▼
                                     ┌──────────────────────────────┐
                                     │  NODO ALINEACIÓN (solo feature)│
                                     │  declarado + revelado         │
                                     │  1 request, 1 pregunta        │
                                     └──┬──────────┬──────────┬─────┘
                                        │          │          │
                                  in_line    out_scope   insufficient
                                        │     divergent        │
                                        ▼          │           ▼
                                   CANDIDATA       ▼      CONDICIONADA
                                             MARCADA, no   «preguntá
                                             recomendada,   primero»
                                             con la cita
                                        └──────────┴──────────┘
                                                    ▼
   ┌────────────────────────────────────────────────▼───────────────┐
   │  PASO DIFERIDO — fuera del MVP (§7.1)                          │
   │  ¿ya fue atendida antes? · agrupar duplicadas                  │
   │  el MVP conserva: open_prs determinista                        │
   └────────────────────────────────┬───────────────────────────────┘
                                    ▼
   ┌────────────────────────────────────────────────▼───────────────┐
   │  P2 · EXPLORE — LLM + codegraph (solo candidatas)              │
   │  evidencia · requisitos en 3 baldes · lagunas · partes         │
   │  gates: confirmed_defect (bugs) · guarda 16k                     │
   └────────────────────────────────┬───────────────────────────────┘
                                    ▼
   ┌────────────────────────────────▼───────────────────────────────┐
   │  P3 · ENCAJE — Clef solo, 1 request por issue                  │
   │  fit · learning_value                                          │
   └────────────────────────────────┬───────────────────────────────┘
                                    ▼
              veredicto por issue + «con este CV, entra; con el otro, no»
              (la CONDICIONADA llega hasta acá, en la lista principal,
               encabezada por su condición)
```

Tres estados de salida, no dos:

| Estado | Significado |
|---|---|
| **CANDIDATA** | vale la pena explorar y puede recomendarse |
| **CONDICIONADA** | es una oportunidad, pero falta una condición externa (el sí del mantenedor, una aclaración) |
| **MARCADA** | no se recomienda, con el motivo y la evidencia a la vista |

Ninguna se oculta. `MARCADA` y `CONDICIONADA` aparecen en la lista principal con su distintivo (§15).

**Nota de alcance:** el nodo `intent` está **fuera del primer pase**: se midió mal alimentado y pasa a la segunda fase (§7.4, `LIMITES.md` §2). El nodo de alineación **sí está dentro** (§7.5): se midió y separa la feature que el proyecto quiere de la que no. `confirmed_defect` sigue diferido a P2. Lo que decide el primer pase es la tabla de §7.7.

### 2.3. Fuera de alcance

Agrupación de duplicados y chequeo de candidatas ya atendidas (diferidos: ver §7.1) · construir un indexador propio · workers asíncronos · object storage · clonación en vivo durante la demo · extracción de DOCX · GitHub App · webhooks · análisis incremental · modelo de costes · rúbricas numéricas · marketplace · reputación · ejecución de tests del repositorio · generación de PR. La extracción de PDF y la descarga por URL **sí** están en alcance, con el cómo aún abierto (§17.8.1).

---

## 3. La inversión arquitectónica

El error a evitar, y la razón por la que este documento existe:

```
DESCARTADO
  evidencia → LLM → rúbricas (números 0..1) → Clef "combina" los números → decisión
  Problema: con scores y políticas declaradas, la decisión es una suma ponderada con
            umbrales. Treinta líneas de código la reemplazan. Clef queda decorativo
            y el Reto 4 falla.

ADOPTADO
  evidencia → state en texto, estructurado por las rúbricas
            → Clef LEE y JUZGA → decisión + probabilidades
            → las rúbricas validan y explican la decisión a posteriori
```

Fundamento técnico: **Clef no agrega números.** Recibe `state` (texto, ~16 000 tokens) y hasta 64 preguntas tipadas `noul` / `choice` / `score`. No tiene noción de una escala 0..1 propia del llamador.

Consecuencia de diseño: **las rúbricas no calculan, describen.** Son la estructura del `state` y la forma esperada de la respuesta. El juicio lo hace Clef sobre el texto.

### 3.1. Por qué Clef y no un prompt de chat

| | Reglas deterministas | Un prompt de chat | Clef |
|---|---|---|---|
| Reproducible ante la misma entrada | sí | **no** | sí |
| Devuelve una probabilidad utilizable | no | no, devuelve prosa | **sí** |
| Puede abstenerse de forma medible | no | solo retóricamente | **sí, categoría propia** |
| Latencia | 0 | 3–8 s | **0,1–2 s** |
| Coste por decisión | 0 | alto | centavos |
| Sirve para triajear 30 issues | — | no | **sí: ~45 s, 0 tokens** |

La tabla se demuestra, no se afirma: §12 la convierte en un benchmark de 10 casos.

---

## 4. E0 — Corpus pre-computado

El trabajo caro ocurre **antes** de la demo. La demo solo ejecuta decisiones.

Criterios de selección de repositorios:

- Públicos, clonables sin credenciales.
- Menos de ~50 MB de árbol de trabajo (`diskUsage` lo da la misma consulta GraphQL).
- Presencia de tests.
- Entre los repos, al menos 2 lenguajes distintos.

### 4.1. El único filtro sobre una issue es `state: OPEN`

La etiqueta **nunca** selecciona. No se busca por etiqueta, no se compara contra una lista y no se normaliza para decidir si una issue entra al corpus.

Si se filtrara por etiqueta, solo podríamos evaluar issues que ya la tienen, y el producto perdería justo lo que tiene que decir: «esta issue no tiene ninguna etiqueta y aun así es la mejor para este CV» y «esta está etiquetada para principiantes y es una trampa». El corpus se define por **estado**, no por promesa.

Las etiquetas se traen **verbatim** y se guardan como atributo declarado. Tienen un solo destino: entrar en el `state` del triaje (§7), donde Clef juzga si describen el trabajo. Ahí son materia de juicio, no criterio de admisión.

Lo que califica una issue es el análisis: requisitos concretos contra capacidades concretas (§8, §9).

### 4.2. La etiqueta está escrita de todas las formas posibles

Medido el 2026-11-13 sobre ocho repositorios:

| Repositorio | Etiqueta real |
|---|---|
| `spf13/cobra` | `good-first-issue` |
| `urfave/cli` | `good first issue` |
| `sindresorhus/got` | `good for beginner`, `✭ help wanted ✭` |
| `pallets/click`, `psf/requests`, `encode/httpx` | **ninguna** |

Cuatro escrituras para la misma idea y tres repositorios donde no existe. No cambia el criterio de selección —no se filtra por etiqueta— y tampoco cambia el juicio: la etiqueta no entra en ningún expediente (§1). Se guarda verbatim al lado de la issue porque es un dato del corpus, no una señal del producto.

Y mide el tamaño del problema que el producto ataca: si la etiqueta ni siquiera se escribe igual entre proyectos, no puede ser el criterio de nada. Ese es el motivo por el que el veredicto es nuestro.

Por repositorio se persisten: URL, rama, SHA, inventario y **todas** las issues abiertas con su texto original intacto y la etiqueta cruda al lado.

### 4.3. Selección propuesta (T1, a confirmar)

| Repositorio | Lenguaje | Árbol | Abiertas |
|---|---|---|---|
| `spf13/cobra` | Go | 1,2 MB | 262 |
| `httpie/httpie` | Python | 4,1 MB | 192 |

---

## 5. E1 — Datos y clon

### 5.1. Límites de GitHub, verificados

`gh` **no tiene límite propio**: usa el token OAuth del usuario y hereda el suyo. Medido el 2026-11-13:

| Recurso | Límite | Uso en este proyecto |
|---|---|---|
| REST (`core`) | 5 000 / hora | ninguno |
| GraphQL | 5 000 puntos / hora | **1 consulta por repositorio** |
| Search | **30 / minuto** | **no se usa** |

Una sola consulta GraphQL trae N issues con cuerpo, etiquetas, autor, estado, `stateReason` y los primeros comentarios. **Triajear un repositorio cuesta un request, no treinta.**

#### Medición del fetch real (2026-11-13)

Consulta con alias: metadatos del repo + 5 issues etiquetadas de `spf13/cobra`, con cuerpo, etiquetas, autor, `assignees`, 10 comentarios y los eventos de referencia cruzada.

```
latencia            1,05 s
payload             23 KB
puntos GraphQL      0  (contador sin movimiento)
árbol del clon      1,2 MB · 66 archivos · clon en 0,71 s
```

La misma consulta devuelve, sin pedir nada extra:

- `diskUsage` en KB → el criterio de tamaño.
- `defaultBranchRef.target.oid` → el `commit_sha` de I4.

#### Detección de PR abierta en la misma consulta

`timelineItems(itemTypes: [CROSS_REFERENCED_EVENT, CONNECTED_EVENT])` devuelve `PullRequest` con `number`, `state`, `url` y `merged`. Es lo que alimenta `cluster_has_open_work` (§7.1) sin una segunda llamada. Verificado:

```
#1391  PR #1984:OPEN                                  → ya tiene trabajo abierto
#1272  #1632:OPEN #2373:OPEN #2426:OPEN #2431:OPEN …   → seis PRs vinculadas
#225   #2511:OPEN  más cuatro historicas MERGED/CLOSED
```

El caso #1391 es el que justifica la etiqueta G1 del grafo: se recomendaría como «primera issue» una que tiene una PR abierta desde hace meses.

### 5.2. Caché local y funcionamiento offline

```
fetch (1 consulta GraphQL) → caché en disco → triaje offline → se descartan los cuerpos
                                                                    │
                     al abrir una issue: se re-consulta solo esa, por actualizaciones
```

- Clave de caché: `(repo, número de issue, updated_at)`.
- **El nombre canónico manda.** `httpie/httpie` es `httpie/cli` para GitHub; el `nameWithOwner` de la respuesta es la clave del store, y la traducción se guarda en `var/store/repos.json`. Sin eso, un renombrado parte un repositorio en dos.
- **La clave en disco es `owner__repo`** (`spf13__cobra`), y el nombre canónico con barra es otra cosa: la primera es lo que nombra el store, el directorio del clon y `Issue.Repository`; la segunda sirve para hablar con GitHub y para mostrar. `store.RepoName` y `config.RepoDir` tienen que coincidir en esa forma, y convertir de una a la otra se hace en `store.RepoKey`, en un solo lugar. La confusión no falla ruidosamente: apunta a un directorio que no existe y el consumidor degrada en silencio, por eso hay un test que ata los dos productores.
- Cada snapshot se guarda envuelto en `{schema, saved_at, data}`. Un archivo de otro esquema se rechaza en vez de decodificarse mal (I4).
- El triaje **nunca** toca la red (I11).
- La caché sobrevive al reinicio y hace la demo hermética: sin red, sin `429`, sin esperas.

### 5.3. Hechos que extrae una regla, sin Clef

Estos son deterministas y se computan, no se juzgan:

```
state              open | closed
state_reason       completed | not_planned | null
maintainer_commented  ¿comentó alguien con permiso de escritura sobre el hilo?
labels[]           texto libre, verbatim, sin normalizar y sin interpretar
open_prs[]         PRs vinculadas por evento de referencia cruzada, con estado
```

Nada más se clasifica por adelantado. Qué tipo de issue es, si la etiqueta dice la verdad y si alcanza la evidencia son preguntas de Clef (§7), no de una regla. `open_prs` es el insumo de `cluster_has_open_work` (§7.1), la única señal del paso diferido que el MVP conserva, y sale de la misma consulta GraphQL.

Lo que la regla **no** puede hacer es interpretar un conjunto libre de etiquetas y comentarios. Ahí decide Clef (§7.4).

### 5.4. Clon

Validar URL → clonar en directorio temporal bajo control de la aplicación → resolver rama → registrar `commit_sha` → `codegraph init -y` (§6) → inventario → índice `.codegraph/` asociado al `commit_sha` (I4).

**No se ejecuta código del repositorio.** Ni `make`, ni `npm install`, ni scripts de setup. Se excluyen `node_modules`, `vendor`, `.git`, binarios y archivos > 1 MB. Límite duro de tiempo y tamaño; al excederse, la etapa falla y la issue queda `undecided`.

Inventario mínimo:

```json
{
  "commit_sha": "…",
  "primary_language": "go",
  "languages": { "go": 0.82, "shell": 0.11, "other": 0.07 },
  "manifests": ["go.mod"],
  "has_tests": true,
  "test_command": "go test ./...",
  "entrypoints": ["cmd/server/main.go"],
  "docs": ["README.md", "CONTRIBUTING.md"],
  "conventions": ["CONTRIBUTING.md#L42 cita: «toda migración requiere entrada en CHANGELOG.md»"]
}
```

---

## 6. E2 — El agente de explore

**Decisión: el explore es un agente delegado.** Go invoca `pig` en modo headless y parsea su salida; no ejecuta el loop de herramientas ni implementa guardarraíles de lectura.

Verificado el 2026-11-13 con el binario real (`go install github.com/MichaelKinsy/PiG/cmd/pig@latest`: 83 MB, 2m 6s de compilación).

Por qué delegado y no propio: los guardarraíles de un agente read-only sobre un repositorio ajeno —symlinks que escapan, `.git` escribible, archivos gigantes, comandos destructivos— son trabajo real y peligroso de improvisar. Un agente sin enjaular leyendo código de un tercero es el escenario que I3 prohíbe. PiG ya los resuelve y los prueba.

### 6.1. La invocación

```bash
pig -p --mode json \
    --provider nan --model nan/deepseek-v4-flash \
    --tools read,ls,grep,mcp__codegraph__codegraph_explore \
    --no-extensions -e builtin:mcp \
    --append-system-prompt "<doctrina del explorador>" \
    --append-system-prompt "<contrato de salida, §8.2>" \
    --skill internal/explore/skills/explore/SKILL.md \
    --no-session --no-skills -nc -np --no-approve \
    "<issue + etiquetas verbatim>"
# cwd = el clon
```

| flag | para qué |
|---|---|
| `-p` / `--print` | headless: procesa y sale |
| `--mode json` | **JSONL**, un evento por línea |
| `--tools read,ls,grep,mcp__codegraph__codegraph_explore` | la allowlist completa, incluida la herramienta MCP |
| `--no-extensions -e builtin:mcp` | hermético **excepto** MCP (ver la trampa abajo) |
| `--append-system-prompt` ×2 | la doctrina de exploración y el contrato de salida. **Siempre presentes** |
| `--skill <ruta>` | la misma doctrina como skill re-legible (ver abajo) |
| `--no-session --no-skills -nc -np --no-approve` | sin sesiones, sin descubrimiento de skills, sin leer el `AGENTS.md` del repo ajeno |
| `--provider nan --model nan/<id>` | el endpoint de NaN |

#### La doctrina va en el system prompt, no solo en la skill

`--no-skills` **no** bloquea nuestra skill: los paths explícitos de `--skill` cargan igual, y la skill aparece en la sección `<skills>` del system prompt con nombre, descripción y ruta. Sin `--skill`, esa sección no existe: cero contaminación del repo o del usuario.

Pero se midió que **no alcanza**: con la doctrina solo como skill, el agente **no leyó el `SKILL.md`** —cero lecturas— y exploró igual durante 44 s confiando en el contrato. La skill es una invitación; el system prompt es una obligación.

> **Decisión:** la doctrina va en `--append-system-prompt` porque tiene que estar, y `--skill` se pasa igual para que el agente pueda releerla si se desvía. Nunca se depende de la skill para el comportamiento.

**La trampa que costó una hora:** `--no-extensions` apaga también la **extensión integrada de MCP**. Sin `-e builtin:mcp`, `pig mcp list` dice `connected` —porque ese comando abre su propia conexión— pero la sesión envía solo 3 herramientas al modelo y el agente nunca ve `codegraph_explore`. La verificación es una línea de `PIG_DEBUG_TOOLS=1`: tiene que decir `sending 4 tools`, no `3`.

Y las herramientas `direct` **tienen que estar nombradas en `--tools`**: la doc dice que el primer turno espera hasta 10 s solo por las declaradas en la petición. Si se omiten, no llegan.

### 6.2. NaN como provider

`~/.pig/agent/models.json`. Cualquier endpoint OpenAI-compatible entra sin escribir código:

```json
{ "providers": { "nan": {
    "name": "NaN",
    "baseUrl": "https://api.nan.builders/v1",
    "apiKey": "$NAN_KEY",
    "api": "openai-completions",
    "models": [{ "id": "glm5.3-flash" }, { "id": "deepseek-v4-flash" }]
} } }
```

`pig --list-models` lista los cinco modelos del allowlist con 128K de contexto. Cambiar `explore` en `config.json` es cambiar un string.

### 6.3. Medición de dos corridas reales

Consulta trivial («¿en qué archivo se define `AddCommand`?»), `glm5.3-flash`:

```
latencia            4,82 s
herramientas        2 llamadas (grep 14 ms)
respuesta           command.go:1342
árbol de git        limpio
```

Issue real completa (#1272 de `cobra`, comandos ocultos en el ancho de uso), `deepseek-v4-flash`, con skill y contrato:

```
latencia            79,31 s
turnos del modelo   14
herramientas        24 (12 grep · 10 read · 1 ls · 1 codegraph_explore)
salida              1,36 MB de JSONL
resultado           12 archivos, 12 observed, 4 call_paths, JSON válido
árbol de git        limpio — no escribió nada
```

Los símbolos que encontró son los correctos: `AddCommand:1342`, `RemoveCommand:1414`, `commandsMaxNameLen:227`, `NamePadding:583`.

**El costo lo domina el modelo, no el indexador:** 14 turnos × ~5,5 s = 79 s, contra 365 ms de la única consulta al grafo. De ahí que el explore se pre-computa en E0.

**Las 24 llamadas no son vagabundeo.** Revisadas una por una, son una investigación dirigida:

```
grep  commandsMaxUseLen|commandsMaxCommandPathLen|commandsMaxNameLen
grep  NamePadding|UsagePadding|CommandPathPadding
grep  Padding|commandsMax            (solo *_test.go)
grep  IsAvailableCommand|rpad|Additional Commands
grep  "Hidden: true"                 (en command_test.go)
grep  updateCommandMaxLen|maxLen|MaxLen
read  command.go (tramos) · command_test.go · CONTRIBUTING.md · plantillas de uso
```

Eso es exactamente el código implicado más su cobertura y las convenciones de contribución, que son las que alimentan el balde `process`. Ruido real: dos llamadas — leyó `.git/HEAD` y buscó en `site/`.

> **Regla:** el tope de llamadas es una **red de seguridad contra bucles patológicos**, no un corrector de sobre-exploración. Valor generoso: 40 llamadas y 180 s por issue. Si se agota, `complete: false` y decide P3 (I10).

#### Contrato fusionado: la comparación

Misma issue, mismo modelo, mismas condiciones. Solo cambia el prompt: antes eran dos archivos solapados (doctrina + contrato corto), ahora es uno solo con doctrina, contrato y presupuesto de llamadas.

| | antes | fusionado |
|---|---|---|
| latencia | 79,3 s | 85,6 s |
| turnos | 14 | 14 |
| llamadas de herramienta | 24 | 26 |
| archivos `observed` | 12 | **14** |
| `call_paths` | 4 | **8** |
| requisitos `process` | 2 | **4** |
| leyó `.git/` | **sí** | **no** |
| JSON válido | sí | sí |
| árbol limpio | sí | sí |

Cuesta 8% más de tiempo y devuelve bastante más evidencia. El hallazgo que la primera corrida se perdió: **`cobra.go:174 rpad`**, el consumidor final de `NamePadding` en la plantilla de uso. Sin él, la red de implicancia quedaba cortada a mitad de camino.

Los 14 archivos son todos señal: `command.go` (los tres `*Padding`, `commandsMax*`, `defaultUsageFunc`, `defaultUsageTemplate`, `IsAvailableCommand`), `completions.go` (los dos comandos que llaman a `AddCommand`) y `cobra.go` (`rpad`). Y el balde `process` trae cuatro hechos concretos del `CONTRIBUTING.md` —correr `go test ./...`, gofmt y golangci-lint, CLA, flujo fork/branch— que es exactamente el costo de onboarding.

La skill se leyó **cero veces** otra vez: confirma que el system prompt es el que manda.

**Solo se parsean dos tipos de evento:** `tool_execution_end` (la traza) y `message_end` con `role=assistant` (la respuesta). Los 1,36 MB son casi todos `message_update` del streaming y no se guardan.

### 6.4. `codegraph` entra por MCP, no por shell

El problema que parecía irresoluble: para usar `codegraph` hay que ejecutar algo, y `bash` rompe el read-only. Se resuelve sin shell:

```
$ codegraph install --print-config codex
command = "codegraph"
args    = ["serve", "--mcp"]
```

`codegraph` es un servidor **MCP por stdio**, y PiG habla MCP. `--tools` conserva las herramientas MCP salvo que se excluyan con `mcp__*`.

> **Verificado.** `pig mcp add codegraph -- codegraph serve --mcp`, con `"exposure": "direct"` en `mcp.json`, y el modelo invoca `mcp__codegraph__codegraph_explore` con `error=false`. El agente tiene el grafo de código sin una sola herramienta de escritura.

`pig mcp add` no acepta `--exposure` por línea de comandos: se escribe en `~/.pig/agent/mcp.json`. El valor por defecto es `codemode`, que exigiría un tool de ejecución y rompería el read-only.

### 6.5. Lo que Go hace

- invocar el proceso con timeout y matar el árbol
- parsear el JSONL: `tool_execution_end` da `{toolName, result, isError, durationMs}`; `message_end` con `role=assistant` da la respuesta
- acotar bytes y presupuesto de reloj
- convertir un fallo del proceso en `complete: false`, nunca en descarte (I10)

Verificado desde Go antes de esta decisión: `exec.CommandContext` con 120 ms de timeout mata el hijo y devuelve código de salida de señal.

**La traza permite verificar las afirmaciones.** Como PiG emite cada ejecución de herramienta con su resultado, podemos contrastar los archivos que el modelo dice haber visto contra lo que las herramientas devolvieron de verdad. Eso hace verificable la regla de `observed` de §8.2, en vez de confiar en la palabra del modelo.

**Se pierde:** granularidad del presupuesto. Con PiG el tope es tiempo de reloj y proceso, no cuenta de llamadas.

### 6.6. Trampas que siguen siendo nuestras

El índice lo construimos nosotros, así que `.codegraph/` sigue apareciendo como `?? .codegraph/` en el clon ajeno (3,8–5,8 MB): va a `.git/info/exclude`, nunca a un `.gitignore` nuestro.

Y dos comportamientos de `codegraph` que ahora son **instrucciones del prompt**, no lógica de Go:

- no vacío ≠ evidencia, vacío ≠ prueba (una consulta sin relación devolvió 0 entryPoints en `cobra` y 1 en `httpie`);
- lo que la issue nombra se pide por nombre con `codegraph_node`, no por similitud: de cinco textos apuntando a `AddCommand`, dos lo devolvieron.

### 6.7. Lo que el agente no hace

No ve el CV, no emite el veredicto, no escribe. La aptitud es de P3 (I6), y la separación es lo que impide que el explore arregle el resultado de la etapa siguiente.

---

### 6.8. Portabilidad: la configuración viaja con la app

PiG no necesita instalación más allá del binario. Toda su configuración es archivo + variable de entorno:

```
$ mkdir -p /tmp/pigportable/agent
$ cp models.json mcp.json /tmp/pigportable/agent/
$ PIG_HOME=/tmp/pigportable  NAN_KEY=...  pig --list-models
provider  model              context
nan       deepseek-v4-flash  128K
nan       glm5.3-flash       128K
```

`PIG_HOME` mueve el árbol de configuración completo. La clave entra por `$NAN_KEY` referenciada desde `models.json`, así que **no hace falta `auth.json`** ni el entorno de Pi del usuario.

**Decisión: la configuración es nuestra, no de PiG.** La app genera el árbol completo en `var/pig/` y lo impone por `PIG_HOME`; nunca lee ni hereda `~/.pig`.

| Pieza | Origen |
|---|---|
| `agent/models.json` | **generado**: provider `nan` fijo (es la hackathon de NaN), `baseUrl` fijo, `api: openai-completions`, modelos = el allowlist de `config.json`, `apiKey: "$NAN_KEY"` |
| `agent/mcp.json` | **generado**: `codegraph serve --mcp` con `"exposure": "direct"` |
| `NAN_KEY` | del `.env` nuestro, inyectada en el entorno del hijo. **No se escribe a disco.** |
| cualquier otra cosa | no existe: `--no-session --no-skills -nc -np --no-approve --no-extensions -e builtin:mcp` |

Se **regenera en cada corrida**, así que un archivo viejo o una configuración externa no pueden contaminar nada. Lo único externo que queda es el binario `pig`, y por eso se fija versión en §17.4. `--pig-home` existe solo como escape para depurar.

Modos disponibles: `-p` para una corrida y salir (lo que usamos), y `--mode rpc`, que habla JSON Lines por stdin hasta cerrarlo y permite varias órdenes en una sesión viva. Si en algún momento hace falta conversar con el agente en vez de dispararlo, `rpc` ya está.

### 6.9. Concurrencia: dos límites distintos

La clave tiene **dos reglas que no son la misma**.

| Etapa | Motor | Límite | Evidencia |
|---|---|---|---|
| P1, P3 | Clef (`/v1/systemone`) | **una en vuelo**, cola serializada (I1) | `docs/api.md`: una petición a la vez por persona |
| P2 | chat (`/v1/chat/completions`) | **paraleliza** | 4 peticiones simultáneas → 4 × `200` en 2,2–2,5 s |

El «una a la vez» del documento es de Clef. Confundirlos habría serializado el explore sin motivo, o paralelizado P1 y comido `429` en la demo.

**Diseño:** un pool de `explore` con `N` workers, cada uno con su propio proceso `pig`, su propio `--cwd` y el **mismo índice `.codegraph` en modo lectura** (SQLite en WAL aguanta lectores concurrentes). Las peticiones a Clef van por su cola de un solo carril y nunca salen de un worker de explore.

El embudo ya acota el paralelismo: se explora solo el subconjunto que pasó P1, no las 262 issues.

**Costo por worker:** `pig` son **92 MB de RSS** medidos; además cada worker levanta un proceso `codegraph serve --mcp`, que corre sobre Node. El RSS de ese proceso **no está aislado todavía** —hay más procesos Node en la máquina, incluido el propio harness— y por eso `N` arranca en 3 y no se sube sin medirlo.

### 6.10. Ociosidad: vigilar el silencio, no los timestamps

El JSONL **no sirve para detectar inactividad**: solo un tipo de evento lleva `timestamp`. Intentar medir la ociosidad por ahí no funciona.

Lo que sí funciona es el **silencio del pipe**. PiG escribe un evento por línea apenas ocurre algo: si no llega ninguna línea, no está pasando nada.

- Se lee la salida del hijo línea por línea; cada línea reinicia un temporizador.
- Sin líneas durante `idleWarn` (20 s) → se registra `explore.idle` y la UI muestra «esperando al modelo» con los segundos transcurridos. Un turno largo legítimo entra acá; no se mata.
- Sin líneas durante `idleKill` (90 s) → se mata el árbol de procesos y el paquete sale `complete: false`, que P3 resuelve con lo que haya (I10). Nunca se descarta.
- Se reintenta **una sola vez** y solo si la corrida murió antes de la primera llamada de herramienta: ahí el reintento no repite trabajo.
- **Se mide siempre:** `ms` hasta el primer evento, `ms` del hueco más largo y `ms` total van al `trace` de §8.2. Un agente ocioso se detecta mirando datos, no adivinando.

---

## 7. E3 — Triaje (P1)

**Clef solo. Sin LLM. Sin código.** Un solo trabajo: **triajear la issue sola**. Una petición por unidad, en serie por la cola (I1). Agrupar duplicados y resolver si una candidata ya fue atendida antes van **después** del triaje (§7.1).

La issue se juzga a sí misma **antes** de explorar el código, por una razón de producto y no técnica: *una issue mal armada le hace perder tiempo al contribuyente*. Explorarla primero sería gastar el recurso más caro en el caso que ya sabemos que no sirve.

### 7.1. Agrupación: fuera del MVP por tiempo

> **Diferida: fuera del MVP.** El triaje deja **candidatas**. Sobre esa lista falta el paso
> que hoy no existe: saber si una candidata **ya fue atendida antes** —aprobada, trabajada,
> mergeada o denegada— y **juntar las que están duplicadas**, porque el grupo sabe más que
> cualquiera de sus partes. Decidir si dos textos describen el mismo problema es trabajo de
> un modelo de lenguaje sobre cada par de candidatas, y no entra en el tiempo del hackathon.
> La unidad de evaluación del MVP es **la issue sola**: cada candidata se explora y se encaja
> por separado. Del chequeo histórico se conserva **una** señal determinista, la PR abierta
> enlazada (`open_prs`, §5.3). El costo está asumido: un reporte escueto que duplica a otro
> completo se juzga por su propio texto y puede quedar marcado como falto de contexto. Lo que
> sigue en esta sección queda como diseño registrado, no como comportamiento del producto.

Este paso va **después** del triaje (§7.2–§7.7) y **antes** del explore (§8): no es parte de P1.

Varias issues reportan el mismo problema. Tratarlas por separado produce tres errores concretos:

1. **Falso negativo de calidad.** Un reporte de una línea que duplica a otro con reproducción completa se marcaría «mal armada». No lo está: es el duplicado de algo mejor. Sin agrupar, se descarta una oportunidad real.
2. **Evidencia desperdiciada.** La reproducción está en una, el entorno en otra, el comentario del mantenedor en una tercera. El grupo sabe más que cualquiera de sus partes.
3. **El peor consejo posible:** recomendar una issue que **ya tiene una PR abierta**. Eso no se perdona en una demo.

Por eso el paso va **después** del triaje y **antes** de explorar: la evaluación (§8, §9) se hace sobre el grupo, no sobre las partes. El error 1 no lo evita este paso —la calidad ya se juzgó por issue en P1— y el MVP lo paga a cambio de no agrupar; los errores 2 y 3 sí los resuelve.

#### Hermanas deterministas (coste cero)

Para cada candidata se buscan hasta 8 hermanas potenciales por señal, todo sobre la caché local:

| Señal | De dónde sale |
|---|---|
| Referencia explícita | `#123`, «duplicate of», «same as», «related to» en cuerpo o comentarios |
| Cross-reference | eventos del timeline de GitHub, ya presentes en el fetch de §5.2 |
| Términos distintivos compartidos | identificadores entre backticks, mensajes de error citados, palabras del título de más de 4 caracteres |
| Archivos o símbolos compartidos | `foo.go`, `pkg.Bar` mencionados en ambas |
| Proximidad temporal y etiqueta | ±90 días, misma familia de labels |

La señal no decide: **ordena candidatas**. La decisión la toma Clef.

#### Decisión (1 request por issue)

| id | tipo | criterios |
|---|---|---|
| `duplicate_of` | `choice` | un id por candidata + `none` |
| `cluster_has_open_work` | `noul` | ¿alguna del grupo tiene una PR abierta, o fue cerrada como completada? |

`duplicate_of` es el uso canónico de `choice`: `criteria` es un diccionario `id: descripción`, y acá las descripciones son los títulos de las candidatas. **Hasta 8 opciones + `none`, ni una más**: con treinta, el juez reparte la probabilidad y deja de discriminar.

Las decisiones por pares se cierran con union-find. Un grupo es un conjunto conexo de duplicados.

#### Consecuencias

- Cuando exista, **la unidad de evaluación es el grupo**, no la issue. El `state` lleva el cuerpo completo del miembro más rico, los títulos de los demás y `report_count`.
- Las issues duplicadas no se descartan: se **absorben** (`absorbed`) y quedan apuntando al grupo. Siguen visibles.
- La recomendación es del grupo, con el reporte más completo como puerta de entrada.
- `cluster_has_open_work ≥ 0,5` → **MARCADA: ya está siendo trabajada.** Además de Clef hay señal determinista: `state` cerrado con `state_reason=completed`, o una PR abierta enlazada.
- Los comentarios de mantenedores en las hermanas son evidencia para el nodo de alineación (§7.5).

Coste: ~45 s más sobre el triaje de 30 issues. Cero tokens de modelo generativo.

### 7.2. Las siete preguntas

| # | id | tipo | qué juzga |
|---|---|---|---|
| 1 | `kind` | `choice` | `bug` · `feature` · `question` · `spam` · `other` |
| 2 | `self_contained` | `noul` | ¿se entiende sin depender de un chat, un video o contexto externo? |
| 3 | `has_evidence` | `noul` | ¿trae el material para reproducir o verificar (traza, versión, ejemplo, caso de uso)? |
| 4 | `unambiguous` | `noul` | ¿el resultado esperado admite una sola lectura? |
| 5 | `done_defined` | `noul` | ¿define qué significaría estar terminada? |
| 6 | `reports_defect` | `noul` | ¿reporta un comportamiento **que ya existe** como defectuoso, o pide uno que no existe? |
| 7 | `evidence_sufficient` | `noul` | ¿alcanza lo disponible para clasificarla con fundamento? |

Estas siete van en **una** petición, sobre el expediente de sustancia. `intent` ya no está entre
ellas: leer la posición del proyecto en el primer pase acertaba 3,7 % sobre 188 issues con
resolución conocida, y pasa a la segunda fase con el expediente de intención (`LIMITES.md` §2,
§7.4). La redacción congelada vive en `internal/triage/prompt.md`, y el código que la envía en
`internal/triage/questions.go`: son el mismo contrato, y una guarda falla si divergen.

`kind` es un `choice` legítimo: son categorías mutuamente excluyentes de un mismo eje — *la naturaleza del pedido*. En una issue de texto libre la persona puede mezclar contenido, pero el pedido dominante es uno.

**Riesgo conocido:** las preguntas 5 y 6 (y 3 con 4) pueden solaparse. No se resuelve redactando: lo mide la matriz de confusión de §12. Si dos criterios se confunden, se fusionan o se reescriben.

#### 7.2.1. Higiene del `state`: el encuadre del autor contamina al juez

Medido contra la API real, siete sondas (2026-11-13):

| Caso | Título | `kind` | `reports_defect` |
|---|---|---|---|
| bug honesto | con prefijo `Bug:` | bug 0,93 | 0,99 |
| bug honesto | limpio | bug 0,94 | 0,98 |
| feature honesta | limpio | feature 0,95 | 0,13 |
| pregunta honesta | — | question 0,94 | 0,02 |
| **feature disfrazada de bug** | **con `Bug:`** | **bug 0,86–0,91 ❌** | **0,62–0,91 ❌** |
| **feature disfrazada de bug** | **limpio** | **feature 0,76–0,80 ✅** | **0,11–0,21 ✅** |

El factor dominante **no** es la redacción de la pregunta: es el **prefijo del título**. Cambiar sola la instrucción de `reports_defect` bajó 0,87 → 0,62. Quitar el prefijo la llevó a 0,21, y combinando las dos a 0,11.

> **Ley:** el encuadre del autor contamina al juez. El estado se normaliza y la afirmación del autor se registra **aparte**, para poder detectar cuando la declaración contradice la sustancia.

```
title_normalized = título sin prefijos de declaración
                   "bug:" "[BUG]" "feature:" "feat:" "[FEATURE]" "enhancement:"
                   "question:" "help:" "proposal:" "RFC:"
declared_kind    = la categoría que el autor declaró, como hecho (o null)
```

- A Clef se le pasa `title_normalized`. Nunca el original.
- `declared_kind` no es una señal: es un **atributo reportado**, igual que la etiqueta de dificultad.
- Cuando `declared_kind` ≠ `kind` con p ≥ 0,6, la UI lo muestra: **el autor también puede mentir**, y es el mismo detector de mentiras de §1 aplicado al título en vez de a la etiqueta.

Coste: un `TrimPrefix` sobre una lista cerrada. Los dos controles muestran que limpiar **no** degrada los casos honestos (0,93 → 0,94).

Siete sondas no son una calibración: son la razón por la que el benchmark de §12 incluye disfraces deliberados.

### 7.3. Calidad de la issue: dos formas de fallar

| Falla | Culpable | Tratamiento |
|---|---|---|
| La issue está mal armada | **la issue** | se excluye, **diciendo qué le falta** |
| Clef no puede clasificarla | **nosotros** | se mantiene, con la duda explícita (I10) |

**Las cuatro preguntas se preguntan y se miden; no deciden.** Esta es la corrección que salió de
la corrida del benchmark: `self_contained`, `has_evidence`, `done_defined` y `unambiguous` se
preguntan en la misma petición —cuestan cero— y se reportan pregunta por pregunta, pero **no mueven
el desenlace**. Dos razones medidas:

- El informe de la corrida concluye que no están validadas, y que una de ellas rinde **peor que
  contestar siempre lo mismo**. Aplicarlas era prometer una precisión que no se midió.
- Aplicarlas mueve 29 issues de `CANDIDATA` a `CONDICIONADA` sin que ninguna medición lo respalde.
  Una compuerta así no protege al usuario: lo aleja de issues que sí eran trabajo.

La única pregunta de este bloque que decide es **`evidence_sufficient`**, y decide en la dirección
segura: por debajo de 0,5 la issue no se descarta, se muestra como **DUDA** (I10). Cuando alguna
vuelva a decidir, será porque una medición nueva lo respalde, no porque el diseño lo suponía.

Esa distinción es la que ordena "dos formas de fallar": de la issue mal armada se ocupa la
segunda fase, con la lista de lo que le falta; de la que no podemos clasificar nos ocupamos
nosotros, y por eso se muestra como duda.

### 7.4. `intent`: fuera del primer pase

**Decisión medida, no preferencia.** Preguntar por la posición del proyecto en el primer pase
acertaba **3,7 %** sobre 188 issues con resolución conocida, y el diagnóstico fue que la pregunta
no estaba mal redactada sino mal alimentada: se le pasaba la issue entera. Queda para la segunda
fase, con el expediente de intención (estado nativo y comentarios de cuentas responsables), ya
especificada y sin implementar (`LIMITES.md` §2).

**Qué cuesta.** El primer pase **no detecta rechazos en prosa**: de 34 issues que sus proyectos
rechazaron, marca 0. Dieciséis salen `CANDIDATA`, o sea el 8,7 % de las candidatas que emite es
trabajo que alguien ya rechazó. Lo que sí se conserva es lo determinista y gratis: `state`,
`stateReason` y la PR abierta enlazada, que se resuelven antes de cualquier pregunta (§7.7).

La líneas que siguen describen el nodo cuando llegue; **no son parte de P1**:

```
intent = authorized   → LÍNEA CERRADA. Candidata. Nada que evaluar.
intent = declined     → LÍNEA CERRADA. Fuera. Nada que evaluar.
intent = neutral      → LÍNEA ABIERTA.  ← acá hay algo que ganar
```

**Las etiquetas son texto libre** — `accepted`, `✅ accepted`, `status/accepted`, `triage:accepted`, `wontfix`, `not planned` — y no se pueden enumerar. Lo que sí es determinista y gratis: `state`, `stateReason` (`not_planned` es un valor nativo de GitHub) y si un mantenedor comentó (§5.3).

```
regla  →  hechos: {state, state_reason, maintainer_commented}
Clef   →  interpreta: ¿eso significa authorized, neutral o declined?
```

La regla no decide: informa. La interpretación del conjunto libre la hace Clef, en la misma petición. Es medible con los 10 casos; si falla, se ajusta la redacción de los criterios, no el mecanismo.

### 7.5. Nodo de alineación (solo `feature`) — *en el MVP*

Cuando el pedido es una feature, se evalúa **si va con el proyecto o es una desviación**. Un bug no
pasa por acá (§7.6), ni un pedido de documentación: ninguno de los dos tiene nada que preguntarle a
la dirección del proyecto.

**Estado: implementado.** Es la segunda petición de P1, y solo la pagan las features. El expediente
es el C, y el brief del proyecto se arma desde el clon —lo que el proyecto dice que es, en su
README, y lo que viene construyendo, en sus últimos 20 commits—, así que no cuesta ninguna petición
extra fuera de la del juicio. Cuando el clon no se puede leer, el desenlace lo dice
(`alignment_unavailable`) en vez de fingir que el proyecto no fija posición.

La pauta no es una opinión: sale de dos fuentes.

| Fuente | Coste | Qué aporta |
|---|---|---|
| **Declarado**: README (qué es y qué no es), CONTRIBUTING, ROADMAP, ADRs, plantillas de issue | 0 — está en el clon | intención explícita |
| **Revelado**: `git log --oneline -200` y los merges | **0 — el clon ya está** | dirección real: qué han estado construyendo y qué han estado quitando |

La segunda es la que más rinde: **la dirección de un proyecto se lee gratis en su historial de commits**. Comparar *declarado* contra *revelado* detecta el caso más filoso — la feature que el proyecto viene desmantelando desde hace seis meses.

Dos preguntas `noul`, en una petición, con umbral **0,6** medido:

| id | qué juzga |
|---|---|
| `only_for_requester` | ¿la petición responde a una situación de quien la pide, tal que a los demás usuarios no les aporte nada? |
| `fits_direction` | ¿la petición mueve al proyecto en la dirección que su documentación y su historia muestran, o lo aleja? |

```
only_for_requester ≥ 0,60                    → MARCADA: sirve solo a quien la pide
fits_direction     ≥ 0,60                    → CANDIDATA
resto                                        → CONDICIONADA: «preguntá al mantenedor
                                                antes de empezar»
```

**Lo que el nodo no hace, y hay que decirlo:** no distingue «el proyecto va en contra» de «el
proyecto no fija posición»; los dos caen en `CONDICIONADA`. Es lo que se midió con estas dos
preguntas, y separarlos exigiría una tercera que todavía no está medida. Tampoco descarta con cita:
negar una feature sigue siendo autoridad del owner, y por eso los dos casos piden hablar antes de
escribir código.

### 7.6. Guardarraíl: la feature disfrazada de bug

Una issue puede pedir una capacidad nueva con el formato de un reporte de error. Es el abuso más común y el más caro, porque entra por la puerta de lo que está autorizado por defecto.

Dos niveles, y no son la misma pregunta:

| Nivel | Pregunta | Insumo | Coste |
|---|---|---|---|
| **P1** (guardarraíl barato) | `reports_defect`: ¿reporta un comportamiento **existente** como defectuoso, o pide uno que no existe? | solo el texto | 0 tokens |
| **P2** (verificación definitiva) | `confirmed_defect`: ¿el código y la documentación confirman que el comportamiento reportado **no es intencional**? | evidencia | caro |

Política de P1: **`reports_defect` se mide y se reporta, no recategoriza.** Reclasificar un bug a
feature movería la issue al nodo de alineación con un umbral que no se validó; el dato queda en el
informe y la verificación definitiva sigue siendo `confirmed_defect` de P2, que es la que mira el
código. Lo que P1 sí conserva del guardarraíl es que un `bug` es CANDIDATA sin pasar por alineación:
el costo de una reclasificación equivocada es una exploración, el de una feature sin dueño es un
veredicto que nadie pidió.

Política de P2, solo para `kind=bug`:

```
confirmed_defect ≥ 0,60  → el bug es real → sigue
confirmed_defect < 0,50  → MARCADA: «el comportamiento reportado es el documentado;
                            esto parece un pedido de cambio, no un defecto»
```

El guardarraíl barato evita gastar explore; el definitivo evita recomendar una contribución que el mantenedor va a rechazar.

### 7.7. Política de P1

La cadena se evalúa en este orden, y el orden no es conmutativo.

| # | condición | resultado |
|---|---|---|
| 0 | pre-compuerta determinista: cerrada como completada · cerrada como no planeada · cerrada por otro motivo · PR abierta enlazada · cuerpo vacío | MARCADA, **con el motivo que la resolvió** |
| 1 | `evidence_sufficient` < 0,5 | **DUDA EN VOZ ALTA** — no se descarta (I10) |
| 2 | `kind=spam` | MARCADA: no es trabajo de código |
| 3 | `kind=question` | **DERIVADA**: la segunda ronda está especificada y no instalada (`LIMITES.md` §3) |
| 4 | `kind=bug` | CANDIDATA: un bug es una desviación de lo que el proyecto ya quiere, sin nodo que consultar |
| 5 | `kind=other` (documentación, tests, proceso) | CANDIDATA: es exactamente el trabajo de una primera contribución |
| 6 | `kind=feature`: `only_for_requester` ≥ 0,6 | MARCADA: sirve solo a quien la pide (§7.5) |
| 7 | `kind=feature`: `fits_direction` ≥ 0,6 | CANDIDATA (§7.5) |
| 8 | `kind=feature`: el resto | CONDICIONADA: «preguntá al mantenedor antes de empezar» (§7.5) |
| 9 | `kind=feature` sin poder leer la dirección del proyecto | CONDICIONADA: `alignment_unavailable` |
| 10 | la respuesta omite una pregunta que la cadena necesita, o trae un `kind` fuera de los cinco | `undecided` (I2): no es un veredicto ni una duda sobre la issue |
| 11 | resto | CANDIDATA |

Cuatro consecuencias que el código respeta y que conviene leer juntas:

- **La pre-compuerta gana a la duda** (fila 0 antes que fila 1): un hecho determinista no depende de
  poder clasificar el texto.
- **`MARCADA` no es terminal.** Lleva su motivo y su evidencia, y una ronda posterior con mejor
  material puede revertirla. **`DERIVADA` tampoco**: es una pregunta que el primer pase no resuelve.
- **Una respuesta incompleta no es una duda sobre la issue** (fila 10). Leerla como «falta una
  condición» inventaría una condición que el servicio nunca dijo.
- **Las preguntas de calidad de §7.3 no están en la tabla.** Se preguntan y se reportan; aplicar las
  que no se validaron movía 29 issues fuera de `CANDIDATA` sin respaldo medido.

La tabla vive en `internal/triage/policy.go` y tiene un test por fila. El banco de
`internal/triage/testdata/bench/` corre **ese mismo código** —las mismas preguntas, los mismos
expedientes, la misma política—, así que la medición y el producto no pueden divergir.

**Cómo se usa Clef, paso por paso** —qué se le manda en cada petición, cuántas cuesta una issue,
qué decide cada respuesta y qué queda fuera—: `CLEF.md`.

**Coste.** Una issue cuesta una petición; una feature, dos. Con ~0,55 s por petición medidos, el
presupuesto de E3 (30 issues en menos de 120 s) se cumple con margen: el número de peticiones no es
la restricción, el tiempo total sí.

**Extensible sin tocar el proyecto.** `question` queda en `DERIVADA` porque la segunda ronda no está
instalada, y la costura ya existe: un tipo con un método y un campo de configuración. Agregarla no
toca preguntas, expedientes, journal, banco ni esta tabla.

La UI nunca muestra un estado sin su probabilidad. Los desenlaces de abstención se muestran
distinto: `DUDA` por `evidence_sufficient` < 0,5 (Clef respondió «no alcanza»), `uncertain` (p <
0,45), `insufficient_information` (falta perfil, es de P3) y `undecided` (el servicio no respondió,
I2). Son cuatro problemas con cuatro arreglos distintos.

---

## 8. E4 — Explore (P2)

**El agente de §6. Solo para candidatas.**

El modelo recibe la issue y un catálogo de herramientas, y **decide él qué mirar**: busca con `codegraph_context`, ancla lo que la issue nombra con `codegraph_node`, lee archivos con `read_file`, barre con `grep`. Go ejecuta y devuelve observaciones; el modelo itera hasta que tiene lo que necesita o se queda sin presupuesto.

Lo que se le pide: **qué partes del código involucra la issue, cuál es su red de implicancia, si el cambio es crítico, cuánto arrastra** y qué requisitos técnicos exige. No juzga aptitud: eso es de P3.

Que el modelo elija qué mirar es lo que separa esto de un analizador estático. Y es la razón por la que `codegraph` no es código nuestro de decisión: es una herramienta más en la caja.

### 8.1. Qué recibe y qué no

Recibe:

- Título y cuerpo completos de la issue, y la etiqueta **verbatim** — es una de las cosas que tiene que evaluar.
- Hasta 10 comentarios, los más recientes.
- Inventario del repositorio (§5.4) y ruta del clon.
- El catálogo de herramientas de §6.2.

**No recibe** el CV, ni el veredicto de P1, ni nada de P3. La separación es lo que impide que el explore arregle el resultado de la etapa siguiente.

**Presupuesto:** máximo de llamadas, de tiempo y de tokens (§6.3). Cuando se agota, el paquete sale con `complete: false` y P3 decide con lo que hay: es una entrada más a `insufficient_evidence`, no un fallo de la app.

### 8.2. Salida obligatoria

JSON validado contra esquema. Si no valida, la etapa falla; no se repara a mano.

```json
{
  "intent": "una o dos frases sobre qué comportamiento se espera",
  "files": [
    {
      "path": "internal/migrations/runner.go",
      "reason": "define el orden de aplicación; la issue pide cambiar el orden de rollback",
      "relation": "observed | probable | unverified",
      "evidence": "líneas 44-78 definen applyOrder; no hay tests en este paquete"
    }
  ],
  "requirements": {
    "knowledge": ["ciclo de migraciones en bases relacionales"],
    "project": ["el orden de aplicación está en internal/migrations/runner.go:44"],
    "process": ["PR con tests; CI corre go test ./..."]
  },
  "parts": [
    { "id": "p1", "title": "…", "files": [], "requirements": { "knowledge": [] }, "depends_on": [] }
  ],
  "call_paths": [["AddCommand", "Commands", "UsageString"]],
  "blast_radius": { "files": 5, "symbols": 9, "public_api": true },
  "unknowns": ["qué debe pasar con migraciones ya aplicadas"],
  "conventions": [{ "source": "CONTRIBUTING.md:42", "text": "…" }],
  "missing_information": ["criterio de aceptación ausente"],
  "trace": [{ "tool": "codegraph_context", "ms": 365 }],
  "complete": true
}
```

Reglas duras:

- `relation: "observed"` exige que **la traza de §6.5 lo respalde**: el archivo tiene que aparecer en el resultado de alguna herramienta de esta corrida. Se contrasta contra los eventos `tool_execution_end`, no contra la palabra del modelo. Si no figura, el máximo es `probable`. Es la defensa contra rutas alucinadas, y no depende solo del índice: `grep` cubre los archivos que el índice no indexó (41 de 66 en `cobra`, §6.4).
- **Prohibido** cualquier campo de dificultad, complejidad, riesgo o recomendación. Eso es de P3.
- `requirements` son **hechos sobre el trabajo**, no juicios sobre personas. Tres baldes separados porque se consiguen distinto: `knowledge` (lo que hay que saber hacer), `project` (lo que hay que saber de este proyecto), `process` (cómo se contribuye acá). Un experto en Go en su primera PR falla en baldes distintos que un veterano del proyecto, y el veredicto tiene que poder decirlo.
- Las `parts` **particionan la evidencia**: cada archivo va a una parte, o se marca `shared`. Una parte que no es mergeable sola no es una parte: es un paso. Si la issue es un solo cambio, hay una parte.
- Las `parts` **se conservan y se usan**: el explore las sigue emitiendo exactamente igual, y entran al `state` de P3 (§9.1) y al informe como **agrupación de la evidencia**. Lo que no existe en el MVP es un **veredicto por parte**: P3 emite uno por issue (§9, `LIMITES.md` §7).
- Archivos fuera del repositorio, rutas absolutas o symlinks que escapan: se descartan.

### 8.3. Preguntas de P2

| id | tipo | cuándo | criterios |
|---|---|---|---|
| `confirmed_defect` | `noul` | solo `kind=bug` — **diferido, §2.1.1** | ¿el código y la documentación confirman que el comportamiento reportado no es intencional? |

**No hay una pregunta que audite la etiqueta de dificultad.** El producto no opina sobre si la etiqueta dice la verdad: no la lee. El veredicto de si la issue es una buena primera contribución es de P3 (§9), sobre la evidencia de P2, y la coherencia entre lo que P1 declaró y lo que P2 midió se reporta de forma determinista en el `state` de P3 (§9.1).

### 8.4. Guarda de 16 000 tokens — **nodo abierto**

> **Estado: abierto, por decisión explícita.** Se resuelve cuando el árbol de decisiones esté cerrado. Se registran las propuestas, no la decisión.

**Propuesta T1 — Comprimir** (determinista, automático):

- comentarios: los 10 más recientes, y ni uno más
- evidencia: máximo 12 entradas de archivo
- cuerpos de código: 2 × 40 líneas
- extractos de documentación: máximo 3 × 5 líneas

**Propuesta T2 — Partir**: si aún no entra y hay N partes, cada parte se evalúa en **su propia petición**, con un `state` que contiene solo su evidencia. No es un parche: la partición ocurre **antes** del veredicto —cada parte se trata como una issue— y la unidad de evaluación sigue siendo una sola issue por petición.

**Propuesta T3 — `scoped_out`**: si una sola parte excede el presupuesto, no se evalúa y se informa:

> «Esta parte toca 23 archivos de 9 módulos y no se puede resumir dentro del presupuesto de evidencia.»

**Propuesta de medición**: estimador conservador `tokens ≈ chars/3`, tope duro 12 000 (no 16 000, porque el estimador se equivoca), calculado **antes** de la petición y registrado en el log de decisión.

**Regla que no se negocia**: **nunca truncar por el final.** Truncar por el final garantiza perder justo el dato que discrimina.

---

## 9. E5 — Encaje (P3)

**Clef solo. Una petición por issue, un veredicto por issue.** La issue es la unidad de evaluación: las `parts` que produce el explore (§8.2) entran al `state` y al informe como **agrupación de la evidencia** —qué áreas toca la issue, y qué archivos y requisitos cuelgan de cada una—, no como unidades de veredicto. El titular responde una sola pregunta, si esta persona puede tomar esta issue, y no elige por dónde empezar. El costo está declarado en §2.1.1 y en `LIMITES.md` §7.

### 9.1. Plantilla del `state`

Texto plano con encabezados. Sin JSON ni sintaxis que Clef tenga que parsear. Las áreas agrupan la evidencia; el veredicto es uno solo para la issue. `depends_on` sigue describiendo el orden entre áreas y no cambia el veredicto.

```
## ISSUE
Title: <título>
Body:
<cuerpo completo>

## REPOSITORIO
Name: owner/project
Primary language: go
Tests: yes (go test ./...)

## ÁREAS DE LA ISSUE
La issue toca 2 áreas. Agrupan la evidencia; no se puntúan por separado.

### Área 1 — <título de la parte>
Files:
- internal/migrations/runner.go [observed] — define el orden de aplicación.

Requirements:
- knowledge: ciclo de migraciones en bases relacionales          [evaluable]
- project: el orden de aplicación vive en internal/migrations/runner.go:44   [contexto]

### Área 2 — <título de la parte>
Files:
- internal/migrations/rollback_test.go [observed] — cubre solo el caso simple.

Requirements:
- knowledge: leer tests de un paquete ajeno                        [evaluable]

## EVIDENCIA DE TODA LA ISSUE
Requirements:
- process: PR con tests; CI corre go test ./...                    [contexto]

Conventions:
- CONTRIBUTING.md:42 — «toda migración requiere entrada en CHANGELOG.md»

## LAGUNAS
- La issue no especifica qué debe pasar con migraciones ya aplicadas.

## DESARROLLADOR
Skills:
- Go — nivel: profesional — fuente: CV (2 años, backend de pagos) — evidencia: «…»
- PostgreSQL — nivel: declarado — fuente: CV (mencionado, sin experiencia asociada)
- React — nivel: demostrado — fuente: portfolio

Availability: 6 h/semana
Preferences: prefiere tareas acompañadas; quiere aprender Rust
```

La etiqueta original **no entra**: no es un dato del juicio, y el veredicto es el nuestro (§1). El expediente de intención tampoco la lleva: no llega a ningún modelo.

### 9.1.1. El bloque de coherencia — nuestra valoración, sin gastar un token

P3 necesita poder decir algo más que «los requisitos son estos». Necesita poder decir **«esto no cierra»**, y decirlo con hechos, no con una opinión nueva. Para eso se calcula un bloque determinista que compara lo que P1 declaró contra lo que P2 midió, y lo escribe dentro del `state`:

```
## NUESTRA VALORACIÓN (calculada, sin modelo)
Hallazgos de coherencia: 2
- El triaje declaró la issue auto-contenida; la exploración encontró trabajo
  en 3 módulos sin relación entre sí (blast_radius.files = 14, call_paths disjuntos).
- El triaje declaró criterio de terminado; la exploración no encontró ningún
  test ni comportamiento verificable (tests = 0).
```

Las reglas, cada una una comparación entre dos hechos ya calculados:

| Declarado en P1 (`triage.Analysis.Answers`) | Medido en P2 (`explore.Evidence`) | Hallazgo |
|---|---|---|
| `self_contained` alto | `blast_radius.files` por encima del umbral y `call_paths` disjuntos | son varios cambios, no uno |
| `has_evidence` alto | `files` vacío | no hay qué tocar |
| `done_defined` alto | `tests` vacío y sin criterio de aceptación en el cuerpo | no se puede saber si terminó |
| `kind = bug` | `blast_radius.public_api` | el «arreglo» cambia un contrato |
| `evidence_sufficient` alto | `missing_information` no vacío | faltaba justo lo que hacía falta |

Tres propiedades que el bloque **tiene** que sostener:

1. **Cero peticiones, cero tokens.** Compara hechos que ya están en memoria. Mismo input, mismo output, y se prueba con datos fijos.
2. **Emite hallazgos, no veredicto.** No dice «no apta»: dice «estas dos cosas no cierran», y Clef decide qué hacer con eso. Si emitiera un veredicto, Clef confirmaría un juicio ya tomado y la decisión dejaría de ser suya — el modo de falla del Reto 4 (§3).
3. **Nunca lee la etiqueta**, porque no está en ningún expediente (§1, I6).

### 9.2. Preguntas

| id | tipo | criterios |
|---|---|---|
| `fit` | `choice` | `ready` · `ready_with_support` · `needs_preparation` · `needs_experience` · `insufficient_evidence` |
| `learning_value` | `score` | `None` · `Some` · `Significant` · `Major` — **diferido, §2.1.1** |

Descripciones de `fit`, escritas como puntos de una sola recta — **el costo de cerrar la brecha técnica**, de cero a infinito:

```
"ready":               "The profile covers every technical requirement the issue demands. No gap to close."
"ready_with_support":  "A technical gap exists but can be closed while doing the work, with a maintainer answering questions."
"needs_preparation":   "A technical gap must be closed before starting: study or practice outside this issue."
"needs_experience":    "A technical gap cannot be closed within the horizon of this contribution; it requires months of prior experience in the specific domain."
"insufficient_evidence":"The profile does not describe the technical capabilities this issue requires, so no comparison is possible."
```

Ninguna menciona principiantes, juniors, niveles, años, ni si la persona contribuyó antes a este proyecto o a cualquier otro. **Ser nuevo no es un criterio de aceptación** (§1.2).

`learning_value` es un eje independiente: se puede estar `ready` y aprender poco, o `needs_preparation` y aprender mucho. Ordena resultados; no aprueba ni rechaza. **Si el reloj aprieta, es lo primero que se cae.**

Se descartaron `gap_severity` y `support_needed` porque medían el mismo eje que `fit`: dos preguntas sobre una recta son solapamiento por construcción.

### 9.3. Política de P3

| condición | veredicto mostrado |
|---|---|
| `fit` = `insufficient_evidence` | `insufficient_information` — el motivo es el **perfil**, no la oportunidad |
| p(`fit`) < 0,45 | `uncertain` — no se emite veredicto |
| `fit` = `ready` y p ≥ 0,70 | `recommended` |
| `fit` = `ready_with_support` y p ≥ 0,50 | `recommended_with_support` |
| `needs_preparation` / `needs_experience` | se muestra `fit` con su probabilidad |
| Clef falló | `undecided` (I2) |

El riesgo no aparece porque no pertenece a ninguna tabla: es un atributo de la issue. Tampoco aparecen `project` ni `process`: se muestran como costo de onboarding y no como compuerta.

`learning_value` se muestra como nivel, nunca como porcentaje ni como decimal sin calibrar.

---

## 10. Invariantes técnicos

Incumplir cualquiera invalida la demo.

| id | Invariante |
|---|---|
| **I1** | **Una sola petición a Clef en vuelo por clave**, en todo el proceso. Cola + mutex, no reintentos. Timeout ≥ 50 s, reintento ante `429` tras 1 s, máximo 5 reintentos (6 intentos). |
| **I2** | **Si Clef falla, la app no decide.** Timeout, `429` agotado, `4xx`/`5xx`: `undecided` con el motivo. Nunca un veredicto por defecto. |
| **I3** | **Contenido de repositorio = evidencia, no instrucciones.** Se delimita, se etiqueta como no confiable y no se interpola en posición de sistema. |
| **I4** | **Todo resultado está pinneado** a `commit_sha` + versión de analizador + versión de prompt + versión de política. La clave de caché incluye los cuatro. |
| **I5** | **Ningún número cruza de P2 a P3.** Si P2 devuelve una puntuación, es un defecto, no una optimización. |
| **I6** | **La etiqueta original no entra en ningún expediente ni en ningún prompt.** No se audita: se ignora. El veredicto de si la issue es una buena primera contribución es nuestro (§1). |
| **I7** | **Replay y live son el mismo camino de código.** El modo replay cambia el transporte, no la decisión. |
| **I8** | **El indexador es una herramienta del agente, no código de decisión.** `codegraph` primario; `grep` y `read_file` cubren lo que el índice no ve. Índice pinneado al `commit_sha`. |
| **I9** | **Triaje antes de explore.** Nunca se gasta un explore en una issue que no pasó la compuerta de P1. |
| **I10** | **Nunca se descarta por evidencia insuficiente.** Se duda en voz alta. Una issue mal armada se excluye diciendo qué le falta; una issue que no se puede clasificar se mantiene con la duda. |
| **I11** | **El triaje corre sobre la caché local.** La red se usa solo en el fetch de §5.2. |
| **I12** | **La unidad de evaluación del MVP es la issue sola.** La agrupación de duplicados queda fuera del MVP por tiempo (`LIMITES.md` §1): comparar pares con un modelo no entra en el reloj. Cuando exista, los duplicados se absorberán en un grupo y no se descartarán. |
| **I13** | **Solo los modelos de la lista constitucional.** `clef` más cinco flash. Un id fuera de ella impide el arranque; la lista es constante en código y `NAN_CHAT_MODEL` solo elige dentro de ella. |
| **I14** | **Un resultado guardado declara la pregunta que responde.** El caché se indexa por `(issue, pasada, modelo, versión del prompt, commit)`. Si cambia cualquiera de los cinco, es otra pregunta y se recalcula. Nunca se reutiliza un veredicto ajeno ni se presenta como fresco. |

Notas: la documentación de la API indica `404` para «sin acceso», pero el servicio devuelve **`401 invalid_api_key`** — el manejo de errores cubre `401`. La concurrencia máxima del modelo de chat no está documentada: se comienza en 2 y se mide.

---

## 11. Contratos de datos

```json
// RepositoryAnalysis
{ "repository_id", "url", "provider", "branch", "commit_sha",
  "inventory", "analyzer_version", "index_status" }

// IssueSnapshot   (caché de §5.2)
{ "repository_id", "issue_number", "updated_at", "state", "state_reason",
  "labels", "author", "body", "comments_preview", "maintainer_commented" }

// IssueCluster    (salida de la agrupación, §7.1)
{ "repository_id", "cluster_id", "members[]", "canonical_issue_number",
  "report_count", "cluster_has_open_work", "linked_prs[]" }

// IssueTriage     (salida de P1)
{ "repository_id", "commit_sha", "issue_number", "kind", "intent",
  "title_normalized", "declared_kind",
  "self_contained", "has_evidence", "unambiguous", "done_defined",
  "reports_defect", "evidence_sufficient", "alignment",
  "outcome", "outcome_reason", "answers", "policy_version", "prompt_version" }

// EvidencePackage (salida de P2)
{ "repository_id", "commit_sha", "issue_number", "intent", "files[]",
  "requirements": { "knowledge": [], "project": [], "process": [] },
  "parts[]", "unknowns", "conventions", "missing_information",
  "confirmed_defect", "explore_version" }

// FitDecision     (salida de P3, una por issue)
{ "decision_id", "repository_id", "commit_sha", "issue_number",
  "developer_id", "answers", "policy_version", "prompt_version",
  "outcome", "outcome_reason", "created_at" }
```

Ningún contrato incluye una puntuación agregada: el producto no produce un `score` único.

---

## 12. Verificación y benchmark

10 casos reales del corpus, con veredicto de referencia asignado a mano antes de correr nada.

**Los 10 casos incluyen al menos 3 disfraces deliberados:** una feature con prefijo `Bug:`, un bug con prefijo `Feature:`, y una pregunta redactada como pedido de cambio. Sin esa categoría, el benchmark mide sobre casos honestos y **certifica una compuerta que falla exactamente donde importa** — es lo que pasó en la primera medición: cinco aciertos limpios y el único caso adversarial mal.

| Columna | Qué es |
|---|---|
| Reglas | etiqueta + heurística de cantidad de archivos |
| LLM directo | un prompt de chat con la misma evidencia |
| Clef | el sistema de este documento |

Se mide: acierto del veredicto, consistencia (misma entrada 3 veces), casos donde cada sistema se abstiene, y casos donde cada uno falla. La tabla se publica en el README. **No se asume que Clef gana**; se reporta lo que dé.

Chequeos por etapa, ejecutables:

- **E1:** el SHA registrado coincide con `git rev-parse HEAD`; el fetch de 30 issues consume **1** request.
- **E2:** para cada issue del corpus, al menos un archivo realmente implicado aparece en `entryPoints` o `relatedFiles`; se mide el tiempo por consulta.
- **E3:** las 30 issues del corpus triajeadas en menos de 120 s, con cero llamadas al modelo de chat.
- **Diferida (§7.1) — Agrupación:** sobre el corpus, con los duplicados marcados a mano, medir precisión y recall de los grupos. Un falso positivo fusiona dos oportunidades distintas; un falso negativo recomienda trabajo ya hecho. Se reportan los dos. Método registrado, no chequeo del MVP.
- **E4:** toda ruta en `files` existe en ese SHA; ningún campo numérico en el JSON; ninguna parte sin archivos.
- **E5:** misma entrada dos veces → misma decisión; fallo simulado → `undecided`.
- **I1:** 10 llamadas disparadas en paralelo → cero `429` y todas respondidas.
- **Suficiencia (valida el diseño del `state`):** recortar la evidencia a la mitad debe **bajar `evidence_sufficient`** y producir `no_verdict`. Si el veredicto no se mueve al quitar la mitad de la evidencia, el `state` está lleno de datos que no discriminan.
- **Solapamiento de criterios:** con los 10 casos etiquetados, matriz de confusión de `fit` y de las cuatro preguntas de calidad de P1. Si dos criterios intercambian casos más de una vez, se solapan y hay que reescribirlos. **Esto no se le pregunta a Clef:** él juzga el caso, no el cuestionario.
- **El criterio 4 en disputa (§1.2):** correr la valoración propia en dos variantes — (a) criterios 1-3, (b) los cuatro — sobre los mismos 10 casos, y comparar contra el juicio humano. Convierte una discusión de valores en un dato, con una variante de prompt sobre el mismo benchmark.

---

## 13. Decisiones tomadas y descartadas

| Decisión | Estado | Motivo |
|---|---|---|
| Rúbricas como estructura del `state`, no como aritmética | **adoptada** | con scores numéricos, Clef queda reemplazable por 30 líneas (§3) |
| Normalizar el título antes de preguntar (`title_normalized`) | **adoptada** | medido: el prefijo `Bug:` hace que Clef clasifique una feature como bug con 0,91; limpiarlo la clasifica bien con 0,76–0,80 y **no** degrada los casos honestos (0,93 → 0,94) |
| `declared_kind` como atributo reportado, no como señal | **adoptada** | el prefijo del autor es una afirmación, no evidencia: mismo patrón que la etiqueta de dificultad |
| La etiqueta de dificultad no entra en ningún expediente ni en ningún prompt | **adoptada** | es la opinión de un tercero sobre la dificultad y el veredicto es nuestro (§1); se guarda verbatim en el corpus y no se juzga |
| Bloque de coherencia determinista P1↔P2 dentro del `state` de P3 | **adoptada** | da la valoración propia sin gastar un token y sin que Clef confirme un juicio ya tomado (§9.1.1) |
| El benchmark incluye disfraces deliberados | **adoptada** | sin casos adversariales el benchmark certifica una compuerta que falla donde importa |
| **Agrupar duplicados antes de triajear** | **adoptada** | un duplicado escueto parece una issue mal armada; sin agrupar se descarta una oportunidad real, y se puede recomendar algo que ya tiene PR abierta. **Descartada en 2.21:** el paso va después del triaje y queda fuera del MVP |
| Triaje antes de explore | **adoptada** | no se gasta el recurso más caro en una issue que no es trabajo (I9) |
| Triaje sin LLM, solo Clef | **adoptada** | el insumo es texto y preguntas tipadas: ~45 s y 0 tokens para 30 issues |
| Calidad de la issue evaluada en P1 | **adoptada** | una issue mal armada le hace perder tiempo al contribuyente |
| Descomponer en compuertas de un eje en lugar de un `choice` multi-eje | **adoptada** | cinco criterios heterogéneos en una pregunta se solapan por construcción |
| Líneas cerradas (`authorized`/`declined`) y línea abierta (`neutral`) | **adoptada** | si el mantenedor ya decidió, no hay nada que evaluar |
| Nodo de alineación solo para `feature + neutral` | **adoptada** | un bug es una desviación de lo ya querido; la alineación no aplica |
| Pauta de alineación = declarado (docs) + revelado (`git log`) | **adoptada** | el historial de commits da la dirección real a coste cero |
| Guardarraíl de feature disfrazada de bug en dos niveles | **adoptada** | `reports_defect` en P1 evita gastar explore; `confirmed_defect` en P2 evita recomendar un rechazo seguro |
| Una sola consulta GraphQL por repo + caché en disco | **adoptada** | 1 request en vez de 30; la demo queda hermética |
| Sin escala de dificultad; requisitos en tres baldes | **adoptada** | una recta única obliga a ubicar a la persona en un punto que no existe |
| La aptitud es técnica y solo técnica | **adoptada** | incluir el contexto del proyecto castiga a la persona por ser nueva (§1.2) |
| Stack: Go, binario único, `net/http` + `html/template` | **adoptada** | un solo lenguaje, sin `node_modules`, sin microservicios |
| Lista constitucional de modelos: `clef` + cinco flash | **adoptada** | los modelos grandes consumen la suscripción; la lista es constante en código y falla cerrada (I13) |
| Un modelo por acción, elegido en `config.json` | **adoptada** | el usuario decide qué modelo usa cada acción generativa; las de decisión quedan fijas en `clef` para no romper el Reto 4 |
| El criterio 4 («no exige conocimiento profundo») | **en disputa** | §1.2; se dirime con el benchmark de §12 |
| Guarda de 16k | **abierta** | §8.4, con propuestas registradas |
| Una issue multi-cambio se evalúa por partes (`part_id` en `FitDecision`) | **descartada** | cortada por tiempo (§2.1.1): el veredicto por unión de requisitos castiga a quien podría hacer una parte y no las otras. Las `parts` se conservan como agrupación de evidencia y el veredicto es uno por issue (§9, `LIMITES.md` §7) |
| Construir un indexador propio | **descartada** | peor recall y 3 h de trabajo no diferenciado frente a `codegraph` |
| Auditar la etiqueta de dificultad (`label_honest`) | **descartada** | valida la opinión de otro sobre la dificultad, que no es asunto del producto, y el MVP no tiene horas para un pase más por modelo (`LIMITES.md` §8) |
| `gap_severity` y `support_needed` | **descartadas** | medían el mismo eje que `fit` |
| Preguntarle a Clef si dos `criteria` se solapan | **descartada** | Clef juzga el caso, no el cuestionario |
| Extracción de DOCX | **descartada** | el MVP acepta texto, PDF y URL (§17.8.1); DOCX queda fuera |
| PDF y URL como entrada del CV | **diferida** | el MVP usa texto pegado; entra si el reloj sobra (§17.8.1) |
| Workers asíncronos, object storage, DB relacional | **descartada** | con 30 issues pre-computadas no hace falta |

---

## 14. Decisiones pendientes

| Tema | Bloquea | Propuesta |
|---|---|---|
| **Guarda de 16k** | E4 | las tres propuestas de §8.4 |
| Repositorios del corpus | E0 | criterios de §4 |
| Formato de persistencia | E1 | archivos JSON por `(repo, sha, issue)`; revisar solo si duele |
| Concurrencia máxima del modelo de chat | E4 | comenzar en 2, medir |
| Umbrales de §7.7 y §9.3 | E3/E5 | ajustar solo contra el benchmark de §12 |
| El criterio 4 («no exige conocimiento profundo») | cierre del producto | experimento de §12 |
| Formas de entrada del CV (PDF, URL) | post-demo | **diferidas por tiempo**: el MVP usa texto pegado (§17.8.1) |
| Nodo de alineación, tercera pantalla, `confirmed_defect`, `learning_value` | post-demo | cortados para entrar en el presupuesto; orden de retorno en §2.1.1 |

---

## 15. Pantallas y demo

| | Qué se ve | El momento de la demo |
|---|---|---|
| **S1 · Repo** | El embudo: 47 abiertas → 5 no son trabajo → 12 features sin el sí del mantenedor → 30 candidatas → 1 condicionada. Cada una con su distintivo; ninguna oculta. | «mirá lo que filtra sola, sin modelo de lenguaje» |
| **S2 · Issue** | Evidencia clickeable, requisitos en tres baldes, lagunas, partes, y nuestra valoración de la issue | «requiere conocer el sistema de migraciones, y la issue no lo dice» |
| **S3 · Veredicto** | Requisitos vs perfil, con probabilidades, y las áreas de la issue como evidencia | «con este CV, entra; con el otro CV, no» |

S2 y S3 pueden colapsar en una sola pantalla si el reloj aprieta (§2.1.1): el contenido es el mismo, cambia dónde se muestra.

Guion:

```
PRE-COMPUTADO  3 repos, 30 issues triajeadas, candidatas exploradas.
EN VIVO (~2 s por paso)
  1. El embudo de S1 completo, con los números.
  2. Una issue con PR abierta, marcada como ya tomada.
  3. Una issue MARCADA y por qué (con la cita del proyecto).
  4. Una issue que no recomendamos para arrancar, con la evidencia y los hallazgos de coherencia.
  5. Pegar un CV → veredicto. Pegar otro CV → otro veredicto. Misma evidencia.
  6. Cortar la red → la app muestra `undecided`. «No decidí» es parte del producto.
CIERRE         La tabla de §12 en pantalla: Clef contra reglas y contra LLM directo.
```

La pantalla que gana el jurado no es la del veredicto: es **S1**. El embudo se explica solo.

---

## 16. Definición de terminado

- [ ] 3 repositorios en SHA fijo, indexados, 30 issues con su etiqueta original preservada por separado.
- [ ] Las 30 issues tienen `IssueTriage` válido; triaje completo en menos de 120 s sin modelo de chat.
- [ ] Al menos un caso demostrable de issue MARCADA por `cluster_has_open_work`.
- [ ] Al menos un caso demostrable de feature reclasificada por `reports_defect`.
- [ ] Al menos un caso demostrable de `alignment` = `out_of_scope` o `divergent`, con la cita del proyecto.
- [ ] Al menos un caso demostrable de una issue que no recomendamos para arrancar, con la evidencia clickeable y los hallazgos de coherencia a la vista.
- [ ] Las candidatas tienen `EvidencePackage` válido, con partes que particionan la evidencia.
- [ ] Al menos 5 decisiones de encaje con perfiles distintos.
- [ ] Los chequeos de §12 pasan.
- [ ] La ruta `undecided` (I2) y la ruta `DUDA EN VOZ ALTA` (I10) son demostrables.
- [ ] El modo replay ejecuta el mismo camino de código que el live (I7).
- [ ] Tabla de benchmark de 10 casos en el README.
- [ ] Video de respaldo de 90 s, grabado antes de la última hora.
- [ ] README con: qué hace, cómo correrlo, qué decisiones tomó y por qué.

---

## 17. Stack y prerequisitos

### 17.1. El principio: cero módulos de terceros de Go

`go.mod` sin un solo `require`. Todo lo que la aplicación necesita está en la biblioteca estándar. La consecuencia no es purismo: `go build` funciona sin red, el binario es único, no hay cadena de suministro que auditar y el despliegue es copiar un archivo.

**La regla es un freno, no un veto.** Su intención es no construir un Tesla para llevar el supermercado: cuando aparece la tentación de sumar una dependencia, el trabajo se detiene y se pide autorización. No prohíbe los **binarios externos**: `git`, `gh`, `codegraph` y `pig` no son módulos, se invocan por `os/exec`, se declaran en §17.4 y el preflight los verifica. Un binario ausente degrada la app (§6), no la rompe en silencio.

| Necesidad | Solución | Paquete |
|---|---|---|
| Servidor HTTP y rutas | `http.ServeMux` con patrones `GET /repos/{id}` y `r.PathValue` (Go 1.22+) | `net/http` |
| Vistas HTML | `html/template` sobre plantillas embebidas | `html/template`, `embed` |
| CSS y JS | `embed.FS` + `http.FileServerFS` | `embed` |
| JSON de entrada y salida | `encoding/json` con `DisallowUnknownFields` | `encoding/json` |
| Validación de esquema de P2 | decodificador estricto + chequeos explícitos (~60 líneas) | — |
| Actualizaciones en vivo del embudo | `http.Flusher` + `text/event-stream` | `net/http` |
| Cliente de Clef | `http.Client` con timeout de 60 s | `net/http` |
| Cola de una petición en vuelo (I1) | goroutine + channel + `sync.Mutex` | `sync`, `context` |
| Subprocesos `git`, `gh`, `codegraph` y `pig` | `os/exec` con `context` y timeout | `os/exec` |
| Persistencia | archivos JSON con escritura atómica (`tmp` + `rename`) | `os` |
| Configuración | variables de entorno + lectura de `.env` (~20 líneas) | `os` |
| Logs y trazas de decisión | `log/slog` | `log/slog` |
| Tests | `testing`, `httptest`, golden files | `testing` |
| IDs | `crypto/rand` | `crypto/rand` |

### 17.2. Lo que no se usa, y por qué

| Descartado | Por qué |
|---|---|
| router de terceros (chi, gin, echo) | el mux estándar tiene patrones por método y parámetros desde Go 1.22 |
| `templ` u otro generador de plantillas | `html/template` alcanza para tres pantallas; cero codegen |
| React, Vue, Svelte | el estado cabe en HTML renderizado en servidor + SSE; el build de frontend costaría horas de las 24 |
| Tailwind, npm, cualquier `node_modules` | CSS escrito a mano; el control de versiones no necesita árbol de dependencias |
| SQLite (`modernc` u otro driver) | 3 repos × 30 issues caben en archivos JSON; un driver suma decenas de MB y no resuelve nada que duela |
| ORM | no hay base de datos |
| librería de JSON Schema | un decodificador estricto con chequeos explícitos es más corto y **lee mejor el error** |
| cliente HTTP de terceros | `net/http` con `context` y timeout alcanza |
| biblioteca de reintentos | la cola de I1 es explícita y auditable; una dependencia la esconde |
| extracción de PDF **en Go** | no hay camino sin dependencia y §17.1 prohíbe módulos de terceros; el cómo queda abierto (§17.8.1) |

### 17.3. Árbol de directorios propuesto

```
cmd/gfi/main.go          punto de entrada: servidor, `config example`, `config check`
internal/config/         configuración única, defaults, validación, `config.example.json`
internal/ghdata/         ✅ wrapper de `gh api graphql`: snapshot pinneado + PRs vinculadas
internal/indexer/        ✅ wrapper de `codegraph init` + `.git/info/exclude`
internal/pig/            ✅ wrapper del agente: PIG_HOME propio, JSONL, watchdog de silencio
internal/explore/        ✅ contrato de salida embebido, parseo estricto, validación contra la traza
internal/nan/            ✅ cliente HTTP: /v1/systemone con cola 1-en-vuelo, /v1/chat/completions en paralelo
internal/profile/        ✅ CV pegado → documento con evidencia y supuestos explícitos, delegado a PiG, caché por pregunta
internal/triage/         P1: preguntas, política §7.7 — agrupación y estado histórico diferidos (§7.1)
internal/fit/            P3: estado por issue, preguntas, política §9.3
internal/store/          ✅ JSON atómico, envoltura de esquema, snapshots, registro de repos, journal de decisiones
internal/repo/           ✅ clonado shallow idempotente y SHA de HEAD
internal/web/            ✅ handlers, estáticos y plantillas embebidos
diagrams/                árbol de decisiones (excalidraw + png)
fixtures/                corpus congelado: snapshots en SHA fijo, respuestas de Clef
scripts/                 preflight.sh
```

### 17.4. Prerequisitos

Verificables con `./scripts/preflight.sh`. Estado al 2026-11-13:

```
PASS  Go >= 1.22        1.27.3
PASS  codegraph >= 1.6  1.6.2
PASS  git >= 2.30       2.53.0
PASS  gh autenticado    2.46.0
PASS  jq                jq-1.8.1
PASS  NAN_KEY           25 caracteres, verificada contra la API
PASS  disco libre       22 GB (mínimo 2)
PASS  api.nan.builders  alcanzable (HTTP 401)
```

| Prerequisito | Para qué | Bloqueante |
|---|---|---|
| Go ≥ 1.22 | compilar; los patrones de ruta necesitan 1.22 | sí |
| `codegraph` ≥ 1.6 | índice del clon; se consume por MCP (`serve --mcp`) | no |
| `pig` (PiG) ≥ 0.7 | el agente de explore (§6). `go install github.com/MichaelKinsy/PiG/cmd/pig@latest`, 83 MB, ~2 min | **sí** — sin él no hay P2 |
| git ≥ 2.30 | clon y SHA | sí |
| `gh` autenticado | **solo** construir el corpus; el runtime lee la caché (I11) | no |
| `NAN_KEY` | toda llamada a Clef y al modelo de chat | **sí** |
| modelo de chat de NaN | P2; lista obtenida con `GET /v1/models` (§17.6) | sí, elegir uno |
| ≥ 2 GB libres | clones e índices | sí |
| red a `api.nan.builders` y `github.com` | desarrollo; **no** en la demo (I7) | sí |

**Ningún bloqueante abierto.** `NAN_KEY` está cargada y verificada contra la API real.

### 17.5. Comandos

```
./scripts/preflight.sh              # verifica prerequisitos
make build                          # go build -o bin/gfi ./cmd/gfi
make test                           # go test ./...
make lint                           # go vet ./... && golangci-lint run && gofmt -l . (falla si hay sucio)
./bin/gfi                                   # 0.0.0.0:7117, configurable en server.addr
make verify                         # build + test + lint, de una
```

El `Makefile` existe solo para no repetir tres líneas largas. No hay orquestador.

### 17.6. Hechos verificados contra la API real

Medido el 2026-11-13 con la clave real, no leído de la documentación.

**Forma de la respuesta.** El sobre es `{id, object: "structured_decision", model, answers, usage}`. Cada respuesta en `answers.<id>` trae `type`, `confidence` y `probabilities`:

| tipo | campos |
|---|---|
| `noul` | `noul` (0..1), `probabilities: {yes, no}` |
| `choice` | `choice` (el id ganador), `probabilities: {<id>: …}`, `confidence` |
| `score` | `score` (media ponderada de niveles), `legend: {"0": "…"}`, `probabilities: {"0": …}` con **claves como cadenas**, `confidence` |

Tres hechos que condicionan la UI y la política:

1. **`confidence` es la probabilidad del ganador**, no una medida independiente. Para `choice` y `noul` es exactamente `max(probabilities)`; no aporta información extra.
2. **`score` es la media ponderada de los niveles**, y su `confidence` es la probabilidad del nivel más probable. Un `score` de 2,39 con `confidence` 0,55 finge una precisión que no existe — razón por la cual §9.3 muestra niveles y nunca el decimal.
3. **Coste real de una petición de triaje:** `input_tokens` 459–674, `output_tokens` 0. Treinta issues ≈ 20 000 tokens de entrada en total.

**Latencia medida:** 0,50 s a 1,71 s por petición. Coherente con la documentación; el timeout de 60 s se queda.

**Modelos disponibles** (`GET /v1/models`, 18 ids). **Solo los marcados se pueden usar** (regla constitucional, `AGENTS.md`):

```
PERMITIDOS     clef
               deepseek-v4-flash · glm5.3-flash · mimo-v2.6-flash · gemma4 · qwen3.6
PROHIBIDOS     qwen3.8-max · qwen3.8-flash · glm5.3 · deepseek-v4-pro
               kimi-k3.1-pro · minimax-h3 · hy4
NO SE USAN     kokoro (TTS) · whisper (STT) · qwen3-embedding · rerank
               flux-2-klein · qwen-image-2.1 (imagen)
```

La lista vive como **constante en código**, no como variable de entorno: `NAN_CHAT_MODEL` elige cuál de las cinco, nunca amplía la lista. Un id fuera de ella impide el arranque (I13), en lugar de fallar a mitad de un análisis.

Para P2 se elige una de las cinco flash con salida JSON estricta. **Pendiente: comparar `glm5.3-flash`, `deepseek-v4-flash` y `mimo-v2.6-flash` sobre un fixture de evidencia real.** Con la lista acotada a modelos flash, esa comparación importa más, no menos: es la única variación posible.

### 17.7. Configuración: un modelo por acción

**Un archivo.** `config.json` en la raíz, con `config.example.json` versionado. JSON porque es el mismo formato que todo lo demás: no hay un formato de configuración nuevo que aprender ni una dependencia para leerlo.

**Dos clases de acción.** Las de **decisión** usan `clef` y no se configuran — el Reto 4 depende de eso. Las **generativas** se configuran por separado, cada una con su entrada.

| Acción | Qué hace | Modelo |
|---|---|---|
| `triage` | P1: compuertas y calidad sobre la issue sola | `clef` — fijo |
| `alignment` | nodo de alineación (§7.5) | `clef` — fijo |
| `confirmed_defect` | P2: verificación del bug (§8.3) | `clef` — fijo |
| `fit` | P3: encaje (§9.2) | `clef` — fijo |
| `profile_extract` | CV en texto → capacidades | **configurable** |
| `explore` | P2: evidencia a partir del código (§8) | **configurable** |

**Formato: JSON, no YAML.** YAML costaría un `require` en `go.mod`, que es lo que §17.1 prohíbe, y escribir un parser propio sería el Tesla para llevar el supermercado. JSON además permite `DisallowUnknownFields`: un typo en la configuración **falla fuerte** en vez de caer en un default silencioso.

**Un archivo, y nada más.** `config.json` en la raíz; `config.example.json` versionado y **generado desde el código** con `make config-example`. Un test falla si el archivo commiteado difiere de lo que produce el binario, así que la documentación no puede envejecer.

Un archivo parcial solo nombra lo que cambia; el resto toma su default. Las secciones, una por wrapper:

| Sección | Qué configura |
|---|---|
| `server` | `addr`: dirección de escucha. Por defecto `0.0.0.0:7117`, que acepta conexiones de la red (el binario corre en un VPS y se mira desde afuera); `127.0.0.1:7117` la cierra a esta máquina. `7117` y no `8080` porque `8080` es el default de media docena de servidores de desarrollo |
| `repos.clone_dir` | dónde se clonan los repositorios (`var/repos`) |
| `store.dir` | corpus congelado y resultados (`var/store`) |
| `github` | `max_issues`, `page_size`, `timeout` — **sin filtro por etiqueta** |
| `indexer` | binario, `timeout` |
| `explore` | binario, modelo, `PIG_HOME`, allowlist de herramientas, `idle_warn`, `idle_kill`, `budget`, `workers` (1–3, la cola de la pantalla) |
| `profile` | la extracción del CV: binario, `PIG_HOME` propio, `idle_warn`, `idle_kill`, `budget`. Sin herramientas y sin MCP: el trabajo es texto entra, texto sale |
| `clef` | URL, `timeout`, `max_retries` |
| `models` | `explore` y `profile_extract`; las acciones de decisión son constantes de código |

Las duraciones se escriben como `"20s"` o `"1m30s"`, o como número de segundos.

**La validación no es cosmética.** Rechaza, entre otras cosas:

- un modelo fuera del allowlist constitucional (I13), y **`clef` como modelo de exploración**: decide en otro endpoint y no puede conducir al agente;
- cualquier herramienta de `explore.tools` que pueda escribir (`bash`, `edit`, `write`, `multiedit`, `shell`) o un comodín `*`: la exploración es read-only y la configuración no puede cambiar eso;
- `idle_kill` menor o igual a `idle_warn`, o un `budget` menor que `idle_kill`;
- una clave desconocida, o contenido después del objeto.

**Precedencia:** flag de línea de comandos → `config.json` → valor por defecto. Sin `config.json`, la aplicación arranca con los valores por defecto.

**La lista de modelos vive en un solo lugar.** `config.AllowedChatModels` es la fuente; `pig.DefaultConfig` recibe la lista como parámetro en vez de tener su propia copia, que se desincronizaría a la primera edición.

**Validación en el arranque, no en el uso:** cada `models.<accion>` se contrasta contra la constante de I13. Un id fuera de la lista impide arrancar y el error **nombra la acción y los ids válidos**:

```
config.json: models.explore = "glm5.3" no es un modelo permitido.
  Para explore, elegí uno de: deepseek-v4-flash, glm5.3-flash, mimo-v2.6-flash, gemma4, qwen3.6
```

Un archivo de configuración equivocado se descubre al arrancar, no veinte llamadas después.

### 17.8. Hueco detectado: ¿de dónde salen las capacidades del CV?

Al nombrar las acciones apareció algo que el flujo no tenía: **el paso que convierte el CV en texto en una lista de capacidades** con nivel, fuente y evidencia (§9.1). El perfil aparece en la plantilla del `state` de P3, pero nadie lo producía.

Queda como acción `profile_extract`, con modelo configurable:

```
CV pegado como texto  →  profile_extract (modelo configurable)  →  perfil editable
                                                                   │
                                          usuario corrige  ──────┘
                                                                   ▼
                                                        P3 · ENCAJE (clef)
```

Reglas: la extracción **no inventa** capacidades ausentes en el texto, distingue declarado de demostrado (§1.2 del documento original) y marca `fuente` y `evidencia` en cada una. El perfil siempre es editable antes de encajar — la decisión final nunca depende de una extracción que el usuario no vio.

Coste: 0,5 h, ya contabilizadas en E5.

#### 17.8.1. Formas de entrada del CV — decidido: solo texto pegado

**Decisión:** para el MVP el CV entra **copiado como texto**. PDF y URL quedan diferidos y sólo se adaptan si el reloj sobra.

El contrato sigue siendo el mismo y por eso el diferimiento es barato: las tres formas convergen en un `string` antes de `profile_extract`. Cuando se agreguen, no cambia ni una línea de la acción.

Lo diferido, con sus tensiones registradas:

1. **PDF contra «cero módulos de terceros» (§17.1).** La biblioteca estándar de Go no extrae texto de un PDF: implicaría una dependencia, un binario externo o pedirle el texto al usuario.
2. **URL: contenido externo no confiable.** Regiría I3: límites de tamaño y tiempo, sin redirecciones hacia destinos internos, contenido como evidencia y nunca como instrucción.
3. **Nombre y `Content-Type` no son de fiar** para decidir qué es el contenido.

DOCX **no** está en el alcance, ni siquiera condicionado al tiempo.

---

### 17.9. Persistencia y caché de análisis

Una corrida de explore cuesta **1m49s** y consume cuota de modelo. Un triaje cuesta ~1,5 s de Clef. **Nada de eso se repite.** Si una issue ya fue analizada, se reutiliza su resultado; el trabajo ya hecho no se vuelve a pagar.

**Por qué no hay base de datos.** El requisito es conservar la lista de repositorios y las issues ya analizadas. Con 300 issues en 1,4 MB, un motor relacional no agrega nada: la consulta es un map en memoria y el motor es una capa entre nosotros y un problema que no tenemos. Medido, para decidirlo con números y no con intuición:

| Opción | Módulos | `go.sum` | Binario | cgo |
|---|---|---|---|---|
| JSON + journal (elegido) | 1 | — | 8,6 MB | no |
| SQLite (`modernc.org/sqlite`) | 26 | 50 líneas | ~14 MB | no |
| DuckDB (`marcboeker/go-duckdb`) | 72 | — | 34 MB | **sí** |

DuckDB no compila con `CGO_ENABLED=0` (`undefined: Conn`). Adoptarlo obliga a abandonar el binario estático verificado con `ldd`. SQLite es viable; queda como salida si los datos crecen más allá de lo que entra en memoria, porque `store` es una interfaz y cambiar la implementación no toca ningún call site.

**Qué hay en disco.**

```
var/store/
  repos.json              registro: nombre canónico, commit, issues, cuándo se hizo fetch
  aliases.json            renombrados de GitHub (httpie/httpie → httpie/cli)
  spf13__cobra/
    snapshot.json         issues abiertas, pineadas al commit
    index.json            pin del índice de codegraph
    decisions.jsonl       journal append-only: una línea por análisis terminado
```

**El journal.** Cada `Put` agrega una línea y hace `fsync`: un resultado que se informó no se pierde en un reinicio. Al cargar, el journal se proyecta a un map en memoria, así que consultar es una búsqueda en RAM. Un `flock` protege la escritura entre procesos: el servidor y un comando de CLI pueden agregar a la vez sin intercalar líneas. Una línea final cortada por un crash se descarta —esa línea nunca se reconoció—; una línea corrupta en el medio falla en voz alta. `Compact` reescribe el archivo cuando acumula más del doble de líneas que análisis vivos.

**La clave del caché (I14).** No es la issue. Es la pregunta completa:

```
(issue, pasada, modelo, versión del prompt, commit)
```

La versión del prompt es `sha256(prompt.md + SKILL.md)[:12]`, calculada en el binario. Cambiar una coma del contrato **invalida solo**, sin que nadie se acuerde de bumpear nada. Un veredicto de otro prompt es un veredicto sobre otra pregunta. Lo mismo con el modelo y con el commit: un grafo de otro commit es otro programa.

**Medido.**

```
1ª corrida  spf13/cobra#1272   1m49.386s  (29 llamadas a herramientas, 0 violaciones)
2ª corrida  spf13/cobra#1272          0,023s   ← 4.756× más rápido
```

La corrida cacheada imprime qué reutilizó, cuándo se produjo y bajo qué versiones, y sugiere `-force`. Una reutilización nunca se presenta como un análisis fresco.

## 18. Reglas de trabajo

- El desarrollo ocurre en `sascodiego/goodfirstissue`, un repositorio propio. **No hay remote hacia `helmcode/nan-hackathon-clef`**: el repositorio oficial del hackathon no es alcanzable desde este clon. La procedencia del material copiado está en `NOTICE.md`.
- Cada issue se trabaja en su rama `feat/<slug>` o `fix/<slug>` desde `main`, y entra por pull request contra `main` del propio repo.
- Entorno verificado: Go 1.27.3, `codegraph` 1.6.2, `gh` 2.46.0 autenticado. Una clave de NaN por persona: una petición a Clef en vuelo por clave.
- Documentación, mensajes de interfaz e identificadores: inglés en el código, español en los documentos.
- Un test en rojo antes de cada regla de decisión. La política de §7.7 y §9.3 se prueba con casos fijos, no con llamadas a la red.
- Cada commit es una unidad revisable (~400 líneas), con su test y su actualización de README.
