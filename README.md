# GoodFirstIssue

> **La etiqueta miente. La verificamos contra el código.**

Una aplicación que decide dos cosas sobre una issue de un proyecto Open Source:

1. **¿La etiqueta de dificultad dice la verdad?** `good first issue` hace promesas implícitas —es un trabajo, está especificada, es auto-contenida— y muchas veces no las cumple.
2. **¿Esta persona concreta cubre los requisitos técnicos que la issue exige?** Comparando requisitos contra capacidades, no contra una escala inventada de dificultad.

Construida para el **#2 Hackathon NaN · Reto 4**, donde la regla es que **Clef** —un modelo de decisión, no de texto— tome la decisión central.

## El problema que ataca

| Lo que dice la etiqueta | Lo que encuentra el análisis |
|---|---|
| `good first issue` | exige conocer el sistema de migraciones y no hay tests en ese paquete |
| un reporte de una línea | es el duplicado de otro con la reproducción completa |
| «Bug: falta el modo oscuro» | es un pedido de funcionalidad con el formato de un bug |
| una issue abierta y disponible | tiene una PR abierta desde marzo |

Ninguna de esas cuatro se detecta mirando etiquetas o lenguajes. Todas se detectan leyendo el código y el hilo.

Y por eso la etiqueta nunca filtra: el corpus son **todas** las issues abiertas. Si filtráramos por etiqueta solo podríamos auditar las que ya la tienen, que es exactamente donde el problema no se ve.

## Cómo decide

Tres pasadas, con un embudo de costo: el recurso caro es el modelo generativo y no se gasta en lo que no lo merece.

| Pasada | Insumo | Motor | Coste |
|---|---|---|---|
| **P1 · agrupar y triajear** | texto de la issue, etiquetas, comentarios | **solo Clef** | ~45 s y **0 tokens** para 20 issues |
| **P2 · explore** | + herramientas de código | **agente**: el modelo decide qué mirar y Go ejecuta | caro, solo candidatas |
| **P3 · encaje** | + perfil del desarrollador | **solo Clef** | ~1,5 s por parte |

Tres estados de salida, nunca dos: **CANDIDATA**, **CONDICIONADA** (falta una condición externa) y **MARCADA** (no se recomienda, con el motivo y la evidencia a la vista). Ninguna se oculta.

Y dos formas de no decidir, que son parte del producto:

- `undecided` — Clef no respondió. La app **no** emite veredicto por defecto.
- **duda en voz alta** — la evidencia no alcanza. Se dice, no se descarta.

El grafo completo de decisiones está en [`diagrams/decision-tree.png`](diagrams/decision-tree.png).

## Estado actual

Esqueleto funcionando, producto en construcción. Sin decoración:

| | |
|---|---|
| ✅ Servidor con plantillas y estáticos embebidos | binario estático de ~8 MB, sin dependencias externas |
| ✅ Cero módulos de terceros | `go list -m all` devuelve solo este módulo |
| ✅ Tool calling verificado | `/v1/chat/completions` devuelve `finish_reason: "tool_calls"` en los cinco modelos permitidos |
| ✅ Preflight de prerequisitos | `./scripts/preflight.sh` |
| ✅ Árbol de decisiones y diseño del flujo | [`PROYECTO.md`](PROYECTO.md) |
| ✅ Cola de análisis desde la pantalla | el botón encola de verdad: hasta tres `pig` en paralelo, cancelar por issue y vaciar la cola |
| ✅ Evidencia en la pantalla de issue | archivos con `path:line` y su relación, con el pin del análisis a la vista |
| ⏳ Triaje preliminar y encaje | ver la ruta en `PROYECTO.md` §2.1.2 |

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
