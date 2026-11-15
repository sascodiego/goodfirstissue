# GoodFirstIssue

> **La etiqueta no decide: decide Clef.**

Una aplicación que responde la única pregunta que importa cuando alguien quiere hacer su
primer aporte a un proyecto open source:

**¿esta persona concreta puede resolver esta issue concreta?**

No clasifica issues por dificultad ni audita la etiqueta `good first issue`: la **ignora**. Una
etiqueta es la conjetura de un mantenedor hecha con la información que tenía, y muchas veces ya
está agotada. GoodFirstIssue lee la issue, lee el código y después cruza ese juicio con el perfil
real de quien se ofrece.

Construida para el **#2 Hackathon NaN · Reto 4**, donde la regla es que **Clef** —un modelo de
decisión, no de texto— tome la decisión central. Y la toma en tres momentos distintos.

## El problema que ataca

| Lo que dice la etiqueta | Lo que encuentra el análisis |
|---|---|
| `good first issue` | exige conocer el sistema de migraciones y no hay tests en ese paquete |
| un reporte de una línea | es el duplicado de otro con la reproducción completa |
| «Bug: falta el modo oscuro» | es un pedido de funcionalidad con el formato de un bug |
| una issue abierta y disponible | tiene una PR abierta desde marzo |

Ninguna de esas cuatro se detecta mirando etiquetas o lenguajes. Todas se detectan leyendo el
código y el hilo.

En `spf13/cobra` hay exactamente **4 issues** con etiqueta de dificultad. Las **4** ya tienen una
PR abierta. La etiqueta no mentía: estaba agotada. Por eso el corpus son **todas** las issues
abiertas —en `cobra`, 262, de las que se traen 150— y no un filtro por etiqueta, que solo podría
mirar donde el problema no se ve.

## Cómo decide

Tres pasadas, con un embudo de costo: el recurso caro es el modelo generativo y no se gasta en lo que no lo merece.

| Pasada | Insumo | Motor | Coste |
|---|---|---|---|
| **P1 · triaje** | texto de la issue, estado nativo, PR vinculada, comentarios | **solo Clef**: 7 preguntas en una petición, más 2 de alineación solo para features | `cobra`: **150 issues, 100 peticiones, 53 s**. **66 de 150** las resuelven las pre-compuertas sin consultar al modelo |
| **P2 · explore** | + grafo de código | **agente de lectura** (`pig` + modelo flash): decide qué mirar y devuelve archivos con `path:line`. **No emite veredictos** | el caro; solo candidatas |
| **P3 · encaje** | + perfil aprobado del desarrollador | **solo Clef**: 1 pregunta con 5 opciones | `cobra`: 15 encajes en 2 peticiones, 1,7 s |

Cuatro estados de salida, nunca dos: **CANDIDATA**, **CONDICIONADA** (falta una condición externa), **DERIVADA** (la pregunta necesita los comentarios, y esa ronda está especificada y no instalada) y **MARCADA** (no se recomienda, con el motivo y la evidencia a la vista). Ninguna se oculta.

Y dos formas de no decidir, que son parte del producto:

- **sin veredicto** — Clef no respondió, o el juicio no llegó a su propio umbral. La app **no**
  emite un veredicto por defecto. Con `ready` en 0,543 devuelve *sin veredicto*, no «sí»: el piso
  es 0,45 y el corte de `ready` es 0,70, y son números distintos.
- **duda en voz alta** — la evidencia no alcanza. Se dice, no se descarta.

El detalle de cada request —qué se le manda a Clef, las preguntas con sus opciones, por qué cada
una y cómo se usa su resultado— está en [`PRESENTACION.md`](PRESENTACION.md).

## Medición

Un producto que decide tiene que poder mostrar contra qué se midió. Todo lo de acá sale de
correr el producto; el método completo está en [`BENCHMARKS.md`](BENCHMARKS.md).

**Banco ciego de 291 casos** — 188 issues que terminaron con una PR mergeada, 34 rechazadas con
explicación del mantenedor, el resto casos de borde.

| | resultado |
|---|---|
| Precisión de `CANDIDATA` | **91 %** — de cada diez que recomienda, nueve eran trabajo que el proyecto aceptó |
| Recall sobre trabajo real | **89 %** — de 188 issues que sí eran trabajo, encontró 167 |
| `DUDA` (`evidence_sufficient` < 0,5) | 17 casos, de los cuales **16 eran rechazos**: **6 %** de precisión |

Ese último número va a contramano y por eso está: cuando el producto duda, casi siempre tiene
razón en dudar. **La etiqueta del mantenedor estaba mal; el juicio estaba bien.**

**Y los atajos deterministas se midieron antes de descartarlos**, porque si un `if` alcanzaba no
había producto: un árbol de reglas derivado del texto daba **0,83–0,94 en las dos issues
gemelas** —la que se acepta y la que se rechaza—, o sea no distinguía. La intención del proyecto
se leía del texto en **7 de 188 casos (3,7 %)**. Lo determinista ya está explotado hasta su
techo: las pre-compuertas resuelven **66 de 150** issues de `cobra` sin gastar una petición.

**Sobre corpus reales**: `cobra` 150 issues en 100 peticiones y 53 s, y la segunda corrida
**0 peticiones y 36 ms** porque cada veredicto declara la pregunta que responde. `gentle-ai` 60
issues en 62 peticiones. El encaje: 15 veredictos en 2 peticiones.

## Estado actual

Esqueleto funcionando, producto en construcción. Sin decoración:

| | |
|---|---|
| ✅ Servidor con plantillas y estáticos embebidos | binario estático de 9,7 MB, sin dependencias externas |
| ✅ Cero módulos de terceros | `go list -m all` devuelve solo este módulo |
| ✅ Tool calling verificado | `/v1/chat/completions` devuelve `finish_reason: "tool_calls"` en los cinco modelos permitidos |
| ✅ Preflight de prerequisitos | `./scripts/preflight.sh` |
| ✅ Árbol de decisiones y diseño del flujo | [`PROYECTO.md`](PROYECTO.md) |
| ✅ Cola de análisis desde la pantalla | el botón encola de verdad: hasta tres `pig` en paralelo, cancelar por issue y vaciar la cola |
| ✅ Evidencia en la pantalla de issue | archivos con `path:line` y su relación, con el pin del análisis a la vista |
| ✅ Perfil del desarrollador | el CV se extrae con PiG, se auditan las citas contra el texto pegado y el documento se edita antes de juzgar |
| ✅ Triaje preliminar | `gfi triage` corre las siete preguntas sobre el corpus: 150 issues en 53 s y 100 peticiones, de las cuales 66 las resolvieron las pre-compuertas sin consultar al modelo |
| ✅ Informe de triaje en la pantalla de issue | el veredicto con su probabilidad, las preguntas que se hicieron con su respuesta, y el motivo cuando no hay recomendación |
| ✅ Retrato del proyecto | `gfi brief` describe un repositorio clonado en seis secciones, cada afirmación con su `archivo:línea`, y alimenta el dossier de alineación sin cambiarle la firma |
| ✅ Embudo en pantalla | el dashboard lee el journal y muestra las 150 issues con el veredicto que está guardado: 69 marcadas, 31 derivadas, 11 condicionadas, 39 candidatas |
| ✅ Encaje en pantalla | el botón **Encajar candidatas** corre la tercera pasada sobre el corpus y el bloque de la issue muestra nivel, probabilidad y pin. Antes solo se podía desde la consola |
| ✅ Índice desde la pantalla | el botón **Indexar** clona y construye el grafo de código: un repositorio recién agregado ya no queda trabado esperando una terminal |
| ✅ El veredicto primero | la pantalla de issue abre con una tira de veredicto y el cuerpo de la issue debajo; los informes completos, después |

## Correr

Requisitos y su verificación: `./scripts/preflight.sh`. Necesitás Go ≥ 1.22, `git`, `codegraph` y una clave de NaN en `.env` (la plantilla es `env.example`).

```bash
cp env.example .env            # completá NAN_KEY
cp config.example.json config.json
./scripts/preflight.sh         # tiene que dar 0 bloqueantes

make build                     # binario estático en bin/gfi
./bin/gfi                     # http://localhost:7117, y desde la red en 0.0.0.0:7117
make verify                    # build + test + lint + design-check
```

El servidor lee `config.json` si está, y arranca con los defaults si no: escucha en
`0.0.0.0:7117`, con `-addr` y `server.addr` para cambiarlo.

### Describir un repositorio

El nodo de alineación necesita saber qué es el proyecto y hacia dónde va, y esa ficha no salía de
ningún lado. `gfi brief` la produce a partir del clon, una vez por repositorio:

```bash
gfi brief spf13/cobra > brief.md     # seis secciones, cada afirmación con archivo:línea
```

- **Pineado al commit.** Si el clon se mueve, el retrato se recalcula; nunca se sirve uno viejo
  como fresco. `-force` lo pide de nuevo sin esperar a que el commit cambie.
- **Una corrida por repositorio, no por issue.** La clave del caché es el repositorio, el commit,
  el modelo y la versión del prompt.
- **Cada sección declara su naturaleza.** `Stack técnico` es lo único que se cruza con el CV;
  el resto es conocimiento del proyecto, y como tal informa el costo de onboarding y nunca es un
  portón de encaje (`PROYECTO.md` §1.2, §13).
- **Cada cita se audita contra el clon**: si el archivo no está, o no llega a esa línea, la
  afirmación se reporta como no sostenida en vez de borrarse del documento.
- **Sin inventar.** Una sección que el repositorio no sostiene dice `insufficient_information`
  (I10). El modelo se elige en `models.project_brief`, dentro del allowlist de I13.
- **Es lo que lee el nodo de alineación.** `gfi triage` usa el retrato como dirección del proyecto
  para las features; si no se puede producir, lee el brief de respaldo (README + últimos 20
  commits, coste cero) y lo avisa en stderr. Una corrida de agente por repositorio y commit, la
  paga la primera feature y el resto del lote la reusa.
- **Necesita `NAN_KEY`** en el entorno, como cualquier otra corrida del agente, y el repositorio
  tiene que estar clonado (`gfi index owner/repo` lo clona e indexa). El documento va a stdout y
  los hechos a stderr, así que `gfi brief owner/repo > brief.md` escribe solo el retrato.

### Analizar desde la pantalla

El botón «Analizar N seleccionadas» encola de verdad. La cola vive en el servidor, así que
navegar no la corta: volvés y sigue donde estaba.

- **Paralelismo:** `explore.workers` en `config.json`, de 1 a 3 (por defecto 3). Cada worker
  es un proceso `pig` con su propio `codegraph`: subirlo más no entra en una laptop.
- **Un análisis cuesta ~1m49s** (medido). La pantalla dice cuánto falta con ese número:
  3 en cola a 3 en paralelo son ~1m49s; a 1 en paralelo, ~5m27s.
- **Cancelar por fila** corta el proceso del agente; **vaciar la cola** descarta lo que espera
  y deja correr lo que ya está en curso; **cancelar todo** hace las dos cosas. Lo cancelado no
  escribe en el journal: una corrida cortada no responde ninguna pregunta.
- **Tope por pedido:** 50 issues. Con 1.000 seleccionadas, la app lo dice en vez de encolar
  diez horas de trabajo.
- **Lo ya analizado no se paga dos veces.** La clave es `(issue, pass, modelo, versión del
  prompt, commit)`; si coincide, la fila dice «reutilizado, de hace X» en milisegundos.
- **Sin `NAN_KEY`, sin índice, o con el índice en otro commit que el corpus**, no se encola y
  cada fila dice qué correr para arreglarlo.
- El triaje preliminar es la compuerta de P1 y todavía no está conectado: hoy se analiza lo
  que selecciones. La cola ya es por etapa, así que el triaje entra como una etapa más.
- **Cada transición deja una línea en stderr** (JSON opcional con `slog`, por defecto texto):
  `pedido de análisis` con lo pedido y lo encolado, `análisis en curso`, `análisis listo` con
  `ms` y `tool_calls`, `análisis reutilizado` con cuándo se produjo, `análisis fallido` con
  `exit` y el motivo, y `WARN` para cada rechazo. Es lo que queda cuando la pestaña ya se
  cerró.

Si lo vas a dejar público, que sea detrás del proxy y no desnudo: la app no tiene
autenticación y el alta de repositorios escribe en el store y gasta el rate limit del token
de GitHub. Las pantallas de lectura no necesitan nada; los dos `POST` sí merecen que el
proxy pida credenciales.

## Decisiones que tomó, y por qué

Están todas en [`PROYECTO.md`](PROYECTO.md) §13, con las descartadas incluidas. Las cinco que explican el resto:

- **Clef juzga, no agrega números.** Las rúbricas describen el estado; la decisión es de Clef. Agregar puntajes con umbrales habría dejado a Clef reemplazable por treinta líneas de código, y el Reto 4 exige lo contrario.
- **La aptitud es técnica y solo técnica.** El conocimiento del proyecto y del proceso se muestra como costo de onboarding, nunca como criterio de rechazo: incluirlo castigaría a la persona por ser nueva, que es a quien el producto dice servir.
- **No hay escala de dificultad.** Hay requisitos concretos en tres baldes y capacidades concretas. Una recta única obliga a ubicar a alguien «experto en Go y principiante en Open Source» en un solo punto, y no existe.
- **La evidencia se verifica contra el árbol.** Un archivo solo se marca `observed` si figura en el índice del commit analizado. Nada de rutas que el modelo recordó.
- **El encuadre del autor contamina al juez.** Medido: un título que empieza con `Bug:` hace que Clef clasifique una funcionalidad como error con 0,91 de confianza. Se normaliza el título y la declaración del autor se registra aparte.

## Documentación

| Documento | Contenido |
|---|---|
| [`PROYECTO.md`](PROYECTO.md) | Documento rector: flujo, grafo de decisiones, invariantes, contratos, presupuesto de tiempo |
| [`AGENTS.md`](AGENTS.md) | Reglas duras para agentes: modelo permitido, invariantes, trampas conocidas |
| [`design/README.md`](design/README.md) | Cómo se maqueta la interfaz y cómo llega a Go templates |
| [`diagrams/`](diagrams/) | Árbol de decisiones |

## Material del hackathon

El README original del hackathon se conserva **sin modificar** en [`README.hackathon.md`](README.hackathon.md).

| | |
|---|---|
| [docs/api.md](docs/api.md) | Referencia del endpoint de Clef en NaN |
| [docs/retos/04-tema-libre.md](docs/retos/04-tema-libre.md) | La regla del reto que jugamos |
| [docs/retos/](docs/retos/) | Los cuatro retos |
| [examples/](examples/) | Clientes mínimos en curl, Python y Node.js |

Este repositorio **no es un fork**: el material de arriba es una copia del repositorio del hackathon, que **no declara licencia**. Qué se copió, de dónde y bajo qué condiciones está en [`NOTICE.md`](NOTICE.md).

## Límites y mediciones

- [`INFORME-TRIAJE.md`](INFORME-TRIAJE.md) — el resultado de pasar la muestra ciega por el
  triaje real, con la matriz por clase emitida.
- [`LIMITES.md`](LIMITES.md) — qué no hace el sistema, por qué, y qué costaría cerrarlo.
- [`BENCHMARKS.md`](BENCHMARKS.md) — los resultados que decidieron el diseño, con el corpus y
  el comando para reproducirlos.
