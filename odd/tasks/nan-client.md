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
| T1 | `Options`, `New` con validación (key, URLs, piso de 50 s, `MaxAttempts ≥ 0`) y tipo `Error` con mapeo de status/red | S4, S6, S10 | inline | `edd4b61` |
| T2 | Tipos de `SystemOne`: `Noul`/`Choice`/`Score`, encode del request y decode del sobre real (§17.6) | S1 | inline | `edd4b61` |
| T3 | Cola 1-en-vuelo con `ctx`; test de concurrencia máxima observada == 1 | S1, S7 | inline | `90f0af9` |
| T4 | Reintentos: 401 sin reintento, 429 a 1 s, 5xx y red con backoff, agotado tipado | S3, S5, S9, S10 | inline | `edd4b61` |
| T5 | `Chat` paralelo + decode de `choices[].message.content` | S2, S6 | inline | `cc383ed` |
| T6 | Timeout y `ctx` respetados de punta a punta contra un server de prueba cancelable | S4 | inline | `edd4b61` |
| T7 | Doc del paquete, `PROYECTO.md` §17.3 y `make verify` verde | S8 | inline | cierre |

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

L8. Evidencia (TDD en rojo): `go test ./internal/nan/` → «no non-test Go files» (RED de T1). Negativos de control ejecutados y observados: con `SystemOne` sin la cola, `TestSystemOneNeverHasTwoRequestsInFlight` falla con «the server saw 2 requests in flight, want 1»; con `Chat` reteniendo la cola, `TestChatRunsConcurrently` falla con «the server saw 1 requests in flight, want more than 1». Restaurado el código, ambos pasan. Esas corridas son la prueba de que los tests muerden.

L9. Punto abierto (off-by-one de I1, no resuelto por código): I1 dice «máximo 5 intentos» y `config.clef.max_retries` vale 5, que con la lectura natural del campo son 1 + 5 = 6 intentos. El paquete expone `Options.MaxAttempts` (intentos totales, default 5 = la letra de I1) para no violar el invariante. Queda pendiente decidir si el cableado de #9 traduce `max_retries: 5` a 6 intentos (y se corrige I1) o si el campo significa 5 intentos totales (y se ajusta el ejemplo de configuración). No se tocó `internal/config` (decisión L5).

L10. Nota de rebanadas: C1 quedó en 1255 líneas porque el cliente, el endpoint de decisión, el motor de reintentos y sus tests no compilan por separado sin stubs. C2 (cola, 189) y C3 (chat, 270) sí quedaron en tamaño revisable, y C2 aísla el ítem de riesgo 4 para que la revisión nativa lo pueda tomar solo. Si la revisión pide cortar C1, el corte posible es «tipos y validación» / «transporte y reintentos» / «endpoint y sobre de respuesta», con tres estados intermedios que hoy no existen.

L11. Evidencia (verificación independiente, `gentle-ai-verify`, commit `593669c` como candidato): `make verify` exit 0; `go test -race -count=1 ./internal/nan/` limpio; S1–S10 cubiertos con test que los pinnea. Hallazgos cerrados en `fix(nan)`:

- **Piso de 50 s esquivale** inyectando un `http.Client{Timeout: 5s}`: `New` solo validaba `Options.Timeout`. Ahora valida también el timeout del transporte inyectado (cero sigue permitido: ahí el único límite es el `ctx` del llamador). Test en rojo observado antes del fix: `error is not *nan.Error: <nil>`.
- **Exhaustion de 429 sin test**: el techo de 5 intentos y las cuatro esperas de 1 s solo estaban pinneados en el camino 5xx, por analogía. Ahora hay test propio. Honestidad: este no tuvo RED, cubre comportamiento que ya existía.
- **`queue_test.go` podía colgarse**: `<-sleeping` sin cota; una regresión que hiciera exitoso el primer intento colgaba el test hasta el timeout global de 10 m. Ahora falla con mensaje.

Nits relevados y no accionados, con motivo: `release` no va en `defer` (un panic en `attempt` mata el proceso igual); `KindTimeout` del propio cliente es reintentable, así que el peor caso de una llamada es 5 × 50 s + esperas — queda para revisar cuando #9 cablee `clef.timeout`, anotado junto al off-by-one de L9.
