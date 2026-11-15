# Cómo usa Clef el primer pase

Un solo lugar para lo que se pregunta en la demo: **en qué pasos se usa Clef, qué se le manda,
cuántas veces, qué decide cada respuesta y qué no se le pregunta**. Todo lo que sigue describe
código que corre, no diseño: cada paso nombra el archivo y la función que lo ejecuta.

Rama `feat/9-p1-policy` · issues #5 (cliente), #9 (política), #14 (redacción y medición).

## 1. Qué es Clef en este producto

Clef es el modelo de decisión, y es el core de la hackathon: **todas las decisiones del producto
pasan por él**. No extrae ni redacta: juzga un texto contra preguntas con umbral, y devuelve
probabilidades, no un veredicto. La aplicación decide después, con una tabla.

Cuatro propiedades del transporte, y ninguna es un detalle:

| Propiedad | Valor | Dónde |
|---|---|---|
| Modelo | `clef`, fijo por constitución, no configurable (I13) | `internal/config` |
| Endpoint | `POST /v1/systemone`, un `state` por petición | `internal/nan/systemone.go` |
| Concurrencia | **una petición en vuelo por clave** (I1); la cola vive en el cliente | `internal/nan` |
| Tiempo | timeout mínimo 50 s, por defecto 60 s; reintentos con backoff y espera de 1 s ante 429 | `internal/nan/nan.go` |

El modelo no se elige por pantalla ni por configuración: `config.json` elige modelos para
`explore` y `profile_extract`, y **las decisiones son siempre `clef`**.

## 2. La cadena exacta, paso por paso

Se evalúa en este orden, y el orden no es conmutativo.

| # | Paso | Quién decide | Coste | Qué produce |
|---|---|---|---|---|
| 0 | Pre-compuertas: cerrada como completada · cerrada como no planeada · cerrada por otro motivo · PR abierta enlazada · cuerpo vacío | Go, con un `if` | **0 peticiones** | `MARCADA` con el motivo que la resolvió, probabilidad 1: es un hecho, no un juicio |
| 1 | Petición **1**: expediente A con las 7 preguntas congeladas | Clef | **1 petición** | `kind` y las 6 preguntas de sí/no con su probabilidad |
| 2 | `evidence_sufficient` < 0,5 | Go, sobre la respuesta | 0 | **`DUDA`**: no se descarta, se muestra con lo que falta (I10) |
| 3 | `kind`: `spam` · `question` · `bug` · `other` | Go | 0 | `spam` → `MARCADA`; `question` → **`DERIVADA`**; `bug` y `other` → `CANDIDATA` |
| 4 | Petición **2**, **solo si `kind=feature`**: expediente C con `only_for_requester` y `fits_direction` | Clef | **1 petición** | con umbral 0,6: `MARCADA` (sirve solo a quien pide) · `CANDIDATA` (va con el proyecto) · `CONDICIONADA` (preguntá al mantenedor) |
| 5 | El nodo no se pudo consultar | Go | 0 | `CONDICIONADA` con `alignment_unavailable`: no se finge que el proyecto no fija posición |
| 6 | La petición 1 falló, o la respuesta omite una pregunta que la cadena necesita | Go | 0 | **`undecided`**: no es un veredicto sobre la issue (I2) |

Dos cosas que la tabla dice sin decirlas:

- **Una issue cuesta una petición; una feature, dos.** No hay una tercera.
- **Clef nunca decide solo.** Devuelve probabilidades; la tabla de arriba es la que decide, y vive
  en Go, con un test por fila (`internal/triage/policy.go`, `Decide`).

## 3. Qué se le manda: los dos expedientes

El endpoint recibe **un `state` por petición**, así que dos juicios con material distinto son dos
peticiones. Los expedientes los arma `internal/triage/dossier.go`.

**Expediente A — sustancia** (`SubstanceDossier`), para la petición 1:

- título **sin** los prefijos que declaran categoría (`bug:`, `[FEATURE]`, `RFC:`…), porque el
  encuadre del autor contamina al juez (medido, §7.2.1);
- cuerpo de la issue;
- hasta **3 comentarios**, los más recientes, de quien pide o de una cuenta `OWNER`, `MEMBER` o
  `COLLABORATOR`, cortados a 600 caracteres;
- **no** lleva etiquetas, y **no** lleva la categoría declarada por el autor.

**Expediente C — alineación** (`AlignmentDossier`), solo para features:

- lo que se pide, en una línea, y el cuerpo;
- la **dirección del proyecto**: lo que declara en su README (hasta 2000 caracteres) y lo que viene
  construyendo en sus **últimos 20 commits** (`CloneBrief`, `repo.RecentSubjects`);
- el bloque va delimitado y rotulado como texto de terceros: **evidencia, nunca instrucciones**
  (I3).

Los dos tienen techo (4000 caracteres) y **todo recorte se marca**: un expediente que corta sin
avisar le miente al juez sobre lo que leyó.

## 4. Qué se le pregunta, y qué no

**Se pregunta** — redacción congelada en `internal/triage/prompt.md`, enviada por
`internal/triage/questions.go`, y una guarda falla si las dos divergen:

| Petición | Preguntas |
|---|---|
| 1 · sustancia | `kind` (elección: `bug` · `feature` · `question` · `spam` · `other`) · `self_contained` · `has_evidence` · `unambiguous` · `done_defined` · `reports_defect` · `evidence_sufficient` |
| 2 · alineación | `only_for_requester` · `fits_direction` |

**No se pregunta, y cada ausencia tiene su corte escrito:**

| No se pregunta | Por qué | Dónde |
|---|---|---|
| `intent` (¿el proyecto aprobó o rechazó?) | medido 3,7 % sobre 188 issues con resolución conocida; pasa a la segunda fase | `LIMITES.md` §2 |
| Segunda ronda de `question` | especificada y no instalada; hoy la pregunta queda `DERIVADA` | `LIMITES.md` §3 |
| Agrupación de duplicados | decidir si dos textos son el mismo problema es trabajo por par, y no hay tiempo | `PROYECTO.md` §7.1 |

Además, cuatro preguntas **se preguntan y se miden pero no deciden**: `self_contained`,
`has_evidence`, `done_defined` y `unambiguous`. El informe de la corrida concluyó que no están
validadas y que una de ellas rinde peor que contestar siempre lo mismo; aplicarlas movía 29 issues
fuera de `CANDIDATA` sin respaldo. Viajan en la misma petición, así que cuestan cero, y quedan en el
informe como medición.

## 5. Cuánto cuesta

Medido en la corrida de cierre, con el banco completo: **279 issues, 203 s**, de las cuales **76
consultaron el nodo** (features). Eso da ~0,7 s por issue, con peticiones de ~0,55 s.

La compuerta E3 pide 30 issues en menos de 120 s, y entra con margen. **La restricción es el tiempo,
no la cantidad de peticiones**: por eso una feature puede pagar la segunda, y por eso las preguntas
que no deciden viajan igual dentro de la misma petición.

**Caché.** Reutiliza el journal del repositorio con la pasada `triage`. La clave incluye issue,
pasada, **modelo**, **versión del prompt** y **commit** (`internal/store/decisions.go`), y la
versión es `sha256(prompt.md ‖ criteria.md)[:12]`. Un veredicto producido por otro contrato, otro
modelo u otro commit **falla la búsqueda**: es otra pregunta (I4, I14). El resultado dice si vino de
la caché, porque nunca se presenta como fresco.

## 6. Dónde está cada pieza

| Pieza | Archivo · función |
|---|---|
| Pre-compuertas deterministas | `internal/triage/dossier.go` · `Prescreen` |
| Expedientes A y C | `internal/triage/dossier.go` · `SubstanceDossier`, `AlignmentDossier` |
| Preguntas y versión del contrato | `internal/triage/questions.go` · `Questions`, `AlignmentQuestions`, `Version` |
| Tabla de decisión | `internal/triage/policy.go` · `Decide` |
| Pipeline (una petición, o dos) | `internal/triage/p1.go` · `Run`, `align` |
| Brief del proyecto desde el clon | `internal/triage/brief.go` · `CloneBrief` + `internal/repo` · `RecentSubjects` |
| Cliente y cola de una en vuelo | `internal/nan` · `Client.SystemOne` |
| Journal de decisiones | `internal/store/decisions.go` · `Decisions`, `Key` |
| Costura de la segunda ronda | `internal/triage/p1.go` · `SecondRound` |

## 7. Lo que hay que decir en la demo, con su número

Lo que el producto **sí** hace, medido sobre 222 issues cerradas con resolución real:

| Afirmación | Número |
|---|---|
| El embudo descarta sin gastar modelo | 12 de 12 resueltas con reglas deterministas |
| Lo que recomienda, lo recomienda bien | precisión de `CANDIDATA` **92 %** sobre 183 emisiones |
| Cubre el trabajo real | recall **89 %** de 188 issues que terminaron en una PR mergeada |
| La clasificación del pedido es sólida | `kind` 35/37 |
| La duda es honesta | 18 dudas, 16 de ellas rechazadas de verdad por su proyecto |
| La alineación separa features | el nodo se consultó 76 veces y sostiene la diferencia entre `CANDIDATA` y `CONDICIONADA` |

Lo que **no** se puede afirmar, y hay que decirlo con el mismo énfasis:

- **El primer pase no detecta rechazos: 0 de 34.** No es una regresión: es el corte de
  `LIMITES.md` §2. Consecuencia medida: **15 de las 183 candidatas (8,2 %)** son trabajo que el
  proyecto ya había rechazado. La segunda fase, con la voz del owner, es la que lo resuelve.
- **Once de las 222 cerradas con resolución real quedan `DERIVADA`** (27 sobre las 279 medidas): son
  preguntas cuyo destino se decide en una ronda que todavía no está instalada. No se descartan: se
  derivan, que es un desenlace propio.
- **Las cuatro preguntas de calidad no están validadas.** Se miden (`has_evidence` 27/37,
  `done_defined` 26/37 son las más flojas) y no deciden.
- **El bloque ambiguo no mide acierto**: 20 issues cerradas como completadas sin una PR mergeada que
  las respalde terminan en 10 `CANDIDATA`, 8 derivadas, 1 duda y 1 condicionada. Es ambigüedad del
  corpus, no del triaje.

## 8. Reproducir la medición

```bash
GFI_INTEGRATION=1 NAN_KEY=... go test ./internal/triage/ -run TestBenchMeasuresWording -v
```

El banco corre **el mismo código que el producto** —las mismas preguntas, los mismos expedientes, la
misma tabla—, y escribe el detalle por caso en `var/bench/report-*.json`, que no se commitea. Sin
`GFI_INTEGRATION` y sin `NAN_KEY` el test se saltea y `make verify` queda hermético.
