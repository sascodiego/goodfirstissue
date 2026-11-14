# Feature: nan HTTP client (issue #5)

Cliente HTTP contra la API de NaN: `/v1/systemone` con una petición en vuelo (I1) y
`/v1/chat/completions` en paralelo. Paquete nuevo, sin UI, sin modelos propios.

Rama `feat/nan-client` · worktree `/src/worktrees/goodfirstissue/feat-nan-client` · issue #5.

## Specs

S1. Dos goroutines llamando a `SystemOne` a la vez: el servidor de prueba **nunca ve dos en vuelo**.

S2. `Chat`: el servidor ve concurrencia > 1. Prueba que no se serializó de más.

S3. `401 invalid_api_key` se mapea a un error tipado propio, y **no se reintenta**.

S4. Timeout del cliente ≥ 50 s por config; el server de prueba lo respeta con un `ctx` cancelable.

S5. `5xx` y errores de red: reintento con backoff y un máximo; agotado, error tipado.

S6. Sin modelos: el paquete no decide *qué* modelo usar, lo recibe. La lista constitucional es de `internal/config` (I13).

S7. La cola vive en el cliente, no en cada llamador.

S8. Solo `internal/nan/**` (paquete nuevo). Nada de UI.

S9. Reintento ante `429` tras 1 s, máximo 5 intentos (I1, `PROYECTO.md` §10, `docs/api.md`).

S10. Si Clef falla, la app no decide (I2): el error tipado lleva `Kind`, `Status`, `Body` y `Attempts`, para que el llamador pueda emitir `undecided` con el motivo.

## Tasks

| ID | Tarea | Specs | Ruta | Commit |
|---|---|---|---|---|
| T1 | `Options`, `New` con validación (key, URLs, piso de 50 s, `MaxRetries ≥ 0`) y tipo `Error` con mapeo de status/red | S4, S6, S10 | inline | C1 |
| T2 | Tipos de `SystemOne`: `Noul`/`Choice`/`Score`, encode del request y decode del sobre real (§17.6) | S1 | inline | C1 |
| T3 | Cola 1-en-vuelo con `ctx`; test de concurrencia máxima observada == 1 | S1, S7 | inline | C2 |
| T4 | Reintentos: 401 sin reintento, 429 a 1 s, 5xx y red con backoff, agotado tipado | S3, S5, S9, S10 | inline | C2 |
| T5 | `Chat` paralelo + decode de `choices[].message.content` | S2, S6 | inline | C3 |
| T5b | `Chat` no reintenta 401 ni serializa; tope de concurrencia lo pone el llamador | S2, S6 | inline | C3 |
| T6 | Timeout y `ctx` respetados de punta a punta contra un server de prueba cancelable | S4 | inline | C3 |
| T7 | Doc del paquete + `PROYECTO.md` §17.3 + `make verify` verde | S8 | inline | C3 |

Riesgo: **ítem 4 (concurrencia)** — la cola de I1 y la prueba de que nunca hay dos en vuelo.
Verificación independiente además de los tests, y revisión nativa con lentes `resilience` y `reliability`.

## Log

L1. «Vamos a trabajar con la siguiente metodologia, vamos a leer una issue, analizarla planificarla y cuando yo te de mi concentimiento explicito recien ahi podes empezar a escribir codigo. vas a trabajar en un worktree nuevo»

L2. «Vamos a trabajar en la 5»

L3. «Decision 1. Bien, vamos por esa y necesito que modifiques o comentes la issue explicando por que tomaste esa decision.

Decision 2. No lo agregamos»

L4. Decisión (nombre del paquete): `internal/nan`, un paquete con split en archivos `systemone.go` y `chat.go`. Evidencia: `PROYECTO.md:1322` describe `internal/clef/` como «cliente, cola 1-en-vuelo, registro de decisiones», pero el registro de decisiones ya vive en `internal/store/decisions.go` (`Pass`, journal, mutex): la línea está desactualizada en uno de sus tres ítems, así que es un árbol propuesto y no un contrato. Ningún invariante nombra paquetes. Se descartó `nan` + `clef` separados: la cola son ~30 líneas (§17.1).

L5. Decisión (configuración del chat): no se agrega sección `chat` en esta entrega. Este PR entrega el contrato (`Options`), no el cableado, y hoy no existe consumidor que construya un cliente. Deuda con dueño: #8 agrega `chat.{url,timeout,max_retries}` en el commit que la consume. `Options.Timeout` documenta que el timeout es por endpoint y lo elige el llamador (`clef.timeout` no acota el chat).

L6. Evidencia: `gh issue comment 5` → https://github.com/sascodiego/goodfirstissue/issues/5#issuecomment-6880899976

L7. Evidencia (reintentos, I1): `PROYECTO.md` §10, fila I1: «Timeout ≥ 50 s, reintento ante `429` tras 1 s, máximo 5 intentos». El 429 entra en el alcance de T4 aunque el criterio de aceptación de la issue solo nombre `5xx` y red.
