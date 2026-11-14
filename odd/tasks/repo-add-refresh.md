# Feature: alta de repositorio y refresco reales (issue #7)

Que el rail deje de mentir: el alta funciona y «Refrescar» vuelve a consultar GitHub en
vez de recargar la misma URL. Todo el fetch entra por una interfaz inyectable, así que
ningún test toca la red.

Rama `feat/repo-add-refresh` · worktree `/src/worktrees/goodfirstissue/feat-repo-add-refresh` · base `main` @ `51654ef` · issue #7.

## Specs

**S1.** Texto de la issue: «Que el rail deje de mentir en dos lugares: **"Agregar repositorio"** está apagado y **"Refrescar"** es un link a la misma URL que no re-consulta nada. Hoy el único camino es `gfi fetch` por consola.» Los dos controles hacen lo que dicen.

**S2.** Texto de la issue: «`internal/web/repos.go` — `POST /repos` y `POST /repos/{repo}/refresh`.» Las dos rutas se registran en `routes.go`.

**S3.** Texto de la issue: «`internal/web/templates/partials/add-repo.html` — el formulario del rail.»

**S4.** Texto de la issue: «`internal/ghdata` — una interfaz mínima de fetch, para poder inyectar un doble en los tests.» La interfaz es un consumidor del paquete, no un método nuevo del cliente.

**S5.** Texto de la issue: «El campo acepta `owner/nombre` **o** la URL completa. El parseo se queda con `nameWithOwner` **de la respuesta de GitHub**, no con lo que escribió el usuario: es el trap ya medido de `httpie/httpie` → `httpie/cli`, y el alias se guarda en `var/store/aliases.json`.»

**S6.** Texto de la issue: «Nombre inválido o repo inexistente: 400 con mensaje claro, no 500.»

**S7.** Texto de la issue: «Alta por `owner/nombre` y por URL completa; las dos terminan en el mismo `key` del store.»

**S8.** Texto de la issue: «Renombrado en GitHub: se guarda el alias y el rail muestra el nombre canónico.»

**S9.** Texto de la issue: «Entrada inválida → 400 con el motivo en el cuerpo.»

**S10.** Texto de la issue: «"Refrescar" actualiza `FetchedAt` y el commit pinneado, y lo muestra.»

**S11.** Texto de la issue: «Durante el fetch, la UI no queda en silencio: el botón se apaga y hay estado en curso.»

**S12.** Texto de la issue: «Tests con un fetcher falso: **ninguno toca red**.»

**S13.** Texto de la issue: «**Nota**: el fetch real tarda ~11 s por repo. No bloquear el request de la página; el POST sí espera, y por eso el botón se apaga.»

**S14.** Texto de la issue: «**Colisiona con**: el resto de issues de web **solo** en `internal/web/routes.go`, donde agrega una línea.»

## Decisiones que el plan fijó antes de escribir

- **La interfaz es del consumidor.** `web.Fetcher` declara `Fetch(ctx, owner, name, ghdata.Options) (*ghdata.Snapshot, error)`, que es exactamente la firma de `*ghdata.Client`: no hace falta adaptador, y el doble de test es de cinco líneas.
- **`NewHandler` recibe sus dependencias, no las busca.** `web.Options{Store, Fetcher, GitHub, Timeout}`. Sin fetcher, los POST responden 503 y la pantalla sigue funcionando: el binario arranca sin GitHub.
- **400 y no 200 con aviso.** El criterio lo pide literal, así que la respuesta de error es `text/plain` con el motivo: se puede leer en el cuerpo y se puede afirmar en un test.
- **Post/Redirect/Get.** El éxito responde 303 a `/?repo=<key>` con `&estado=alta|refresco`, que la pantalla traduce a una frase fija. Un `Refresh` del navegador no re-dispara el fetch. El `estado` viaja como código de una lista blanca, nunca como texto libre del pedido.
- **Defensa de origen en los POST.** Es el primer endpoint que cambia estado y el binario escucha en `0.0.0.0:8080` por defecto: un `POST` cross-site contra `localhost` escribiría en el store. Se rechaza con 403 cuando el pedido se declara cross-site (`Sec-Fetch-Site: cross-site`) o cuando `Origin` no coincide con `Host`. Sin dependencias y sin token.
- **El nombre canónico manda.** El alias y la clave salen de `snap.Repository.NameWithOwner`; lo que escribió el usuario solo sirve para pedir y para registrar el alias.
- **Repo sin issues abiertas no es un error.** `ghdata.ErrNoIssues` trae snapshot válido: se guarda igual, con cero issues, igual que hace `gfi fetch`.
- **Repo inexistente se distingue de un fallo de red.** `ghdata` gana un centinela `ErrRepoNotFound` clasificado en el decode de la respuesta de GraphQL (`NOT_FOUND`), porque el criterio pide 400 para «no existe» y 500 solo para lo que de verdad falló.

## Tasks

| ID | Tarea | Specs | Ruta | Commit |
|---|---|---|---|---|
| T1 | `ghdata`: test en rojo de `ErrRepoNotFound` y del NOT_FOUND de GraphQL | S6 | inline | `868018c` |
| T2 | `web.Fetcher` y `web.Options`: `NewHandler` deja de tener firma pelada; doble de test | S4, S12 | inline | `868018c` |
| T3 | `repos_test.go` en rojo: parseo de las dos formas, 400 con motivo, alta al mismo `key` | S5, S6, S7, S9 | inline | `868018c` |
| T4 | `repos.go`: parseo y `POST /repos` | S2, S5, S7, S9 | inline | `868018c` |
| T5 | `POST /repos/{repo}/refresh`: mueve `FetchedAt` y el commit | S10 | inline | `3fa09c1` |
| T6 | Alias y nombre canónico (`httpie/cli`) | S5, S8 | inline | `868018c` |
| T7 | Plantillas: formulario activo y estado en curso (alta) · «Refrescar» como POST | S1, S3, S11, S13 | inline | `868018c`, `3fa09c1` |
| T8 | `routes.go`, defensa de origen, CSS aditivo en los dos `app.css` | S2, S14 | inline | `868018c`, `3fa09c1` |
| T9 | `make verify` y mirada a mano con el binario real y un `gh` falso (sin red) | S12 | inline | este commit |

## Log

**L1** — Pedido del usuario, verbatim: «Trabajaremos en la 6 en un nuevo worktree». Después de comprobar que la #6 ya está implementada y activa en `feat/issue-screen` (rama de otro agente), el usuario corrige, verbatim: «Ok , esa no porque esta siendo trabajada por el otro agente». Y elige esta, verbatim: «Si la 7 no tiene dependecia con la 6 vamos con esa».

**L2** — Condición del sello verificada antes de escribir: la dependencia declarada de #7 es `[web] dividir internal/web por pantalla`, que ya está en `main` (`410eb05`, PR #16). No lee nada de la pantalla de issue. El único roce con #6 es textual: las dos agregan una línea en `internal/web/routes.go`. `git merge-tree main feat/issue-screen` sale sin conflictos.

**L3** — Evidencia de exploración, antes de escribir: `partials/add-repo.html` ya existe con el formulario **apagado** y el texto «El alta desde la interfaz todavía no está conectada»; «Refrescar» es `<a class="btn" href="{{.Active.Href}}">`, o sea la misma URL; `NewHandler(st)` no recibe configuración, así que la capa web hoy no puede construir un cliente de GitHub ni conocer `github.timeout`; `splitTarget` vive en `package main` y no se puede importar, y no acepta URLs; `store.SaveAlias`/`SaveRepo`/`SaveSnapshot` y `ghdata.Client.Fetch` ya hacen todo lo que hace `gfi fetch` (`cmd/gfi/main.go:215-275`), que es la secuencia a reusar; `decode` hoy devuelve el mensaje de GraphQL como texto y no distingue «no existe» de un fallo; `DefaultAddr` es `0.0.0.0:8080`.

**L4** — Rojo observado antes de cada verde: `undefined: ErrRepoNotFound` (T1, el paquete no compilaba); `404 page not found` en las seis variantes del alta (T3/T4, la ruta no existía); `the add form does not post to /repos`, `the add form has no named field`, `the add form is still disabled`, `Refrescar does not post to the refresh route`, `Refrescar is still a link to the same URL` y `the page does not confirm the add` (T7); y el 403 del origen cuando el POST cross-site todavía pasaba (T8).

**L5** — El refresco **no tuvo rojo propio** y lo digo acá porque importa: se escribió junto con el alta porque los dos son `fetchAndStore` con distinta entrada, y el test llegó después. Para que conste que esos tests pueden fallar, corrí dos controles de mutación y los revertí: (a) un «Refrescar» que redirige sin consultar hace fallar `TestRefreshMovesThePinAndTheFetchTime` con `CommitSHA = "111111111111", want the pin moved` y `fetcher calls = [], want exactly one`; (b) guardar la clave pedida y no el alias canónico hace fallar `TestAddRepoKeepsTheCanonicalNameAndTheAlias` y `TestRefreshFollowsARenameAndKeepsOneEntry`.

**L6** — Evidencia de cierre. `make verify` exit 0 (build, 10 paquetes, vet, `golangci-lint` 0 issues, design-check) y los dos `app.css`/`app.js` idénticos. Mirada a mano con el binario real, store vacío, `PATH` apuntando a un `gh` falso que devuelve un JSON fijo: **red cero**. `POST /repos` con `https://github.com/httpie/httpie` → 303 a `/?repo=httpie__cli&estado=alta`; la pantalla dice «Se agregó httpie/cli.» y el rail muestra `corpus 2026-11-14 · pin deadbeefcafe`, que es el nombre canónico y el commit del fixtures, no los pedidos. Alta por `httpie/cli` → mismo `key`, y el registro sigue con **una** entrada. Entradas inválidas → 400 con el motivo (`"cobra" no es owner/nombre ni el enlace de un repositorio`, `el enlace tiene que ser de GitHub, no de "gitlab.com"`). Refresco de algo fuera del corpus → 404 sin tocar al fetcher. Refresco real → 303 con `estado=refresco` y «Refrescado httpie/cli.». `Sec-Fetch-Site: cross-site` → 403. `?estado=<script>alert(1)</script>` no llega a la página. El store queda con `repos.json` de una entrada y `aliases.json` con `httpie/httpie → httpie/cli`, y el `gh` falso recibió exactamente dos llamadas: la del alta y la del refresco. Un `gfi` viejo ocupaba el :8099 (no era de esta sesión), así que la prueba corrió en :8111 sin matar nada ajeno.

**L7** — Rebanadas: `868018c` son 687 líneas y `3fa09c1` 311. La primera pasa el objetivo de ~400 porque el alta, su validación, el centinela de `ghdata`, el cableado de dependencias y el formulario son **un mismo comportamiento**: partirlos dejaba commits que no se pueden ejecutar. Lo que protege al revisor es el PR, que es una sola issue.

**L8** — Un test existente cambió de contrato, y es correcto que cambie: `TestDashboardRendersAddRepoPartial` afirmaba el formulario apagado y el texto «`gfi fetch owner/nombre`». Ahora afirma el formulario habilitado, su `action="/repos"` y el campo con nombre, así que un parcial enganchado pero apagado —el defecto que la issue reporta— vuelve a fallar.

**L9** — Nits relevados y no accionados, con motivo:

- El «se apaga el botón» de `app.js` no tiene test automático: hace falta un navegador. El servidor no depende de eso (el POST funciona sin JS y el `hidden` del aviso es del cliente), así que se verificó leyendo y con `node --check`, no con un test.
- `SaveAlias` hace read-modify-write **sin lock** (`Write` no toma lock; `SaveRepo` sí). Dos altas simultáneas podrían perder un alias. Con un humano apretando el botón no pasa, y arreglarlo es una decisión sobre el store, no sobre esta pantalla.
- `DefaultAddr` sigue siendo `0.0.0.0:8080`. Enlazar solo loopback por defecto es una decisión de producto; la defensa de origen es la que correspondía a este endpoint. Queda anotado para decidirlo, no resuelto acá.
- El CLI conserva su propio `splitTarget`, que no acepta URLs. Unificarlo con `parseRepoInput` es deuda: la issue pide el formulario, no tocar el comando.
- Si algún día hay sesión o auth, la defensa de origen se reemplaza por un token: hoy no hay nada que un token proteja que el origen no cubra.

**L10** — La verificación independiente encontró **un defecto bloqueante que los tests no veían**, y tenía razón:

- **El refresco de un repositorio renombrado dejaba la entrada vieja en el registro.** `SaveRepo` reemplaza por `Key`; con el nombre nuevo la clave cambia, así que la entrada vieja se **agregaba al lado** y el rail listaba el mismo repositorio dos veces, una con un pin que ya no existía. Peor: el test llamado `TestRefreshFollowsARenameAndKeepsOneEntry` afirmaba el alias y la redirección y **nunca miraba el registro**, así que pasaba con el duplicado puesto. Rojo observado al agregar la afirmación que faltaba: `registry has 2 entries (...), want exactly one`, con la entrada vieja en `111111111111`. Arreglado con `store.RemoveRepo` y el borrado dentro de `fetchAndStore` cuando la clave canónica difiere de la pedida: se guarda la entrada nueva **y después** se borra la vieja, así el rail nunca ve cero repositorios. `RemoveRepo` tiene su propio test (clave ajena: no es error; clave vacía: sí), porque el llamador lo ejecuta en cada fetch que GitHub contesta con otro nombre.

Disposición de los hallazgos advisory:

- **DNS rebinding (real, no cerrado).** Rebindear el dominio a `127.0.0.1` hace que `Origin`, `Host` y `Sec-Fetch-Site` coincidan, porque el atacante controla los tres. La defensa por cabeceras no puede cubrirlo. La mitigación verdadera es enlazar loopback por defecto, que es la decisión de producto anotada en L9. Chrome la mitiga con Private Network Access; Firefox no. Esto **no** es un endpoint para exponer fuera de la máquina.
- **Colisión de claves (`a__b`/`c` vs `a`/`b__c` → `a__b__c`).** Preexistente en `store.RepoName`, no la introduce esta pantalla, y cambiarla rompe los stores ya escritos. Deuda registrada, no arreglada acá.
- **`https://github.com:443/owner/name` lo rechazaba.** Se corrigió: el chequeo de autoridad usa `u.Hostname()` y el puerto deja de importar. Test agregado.
- **Mayúsculas en el pedido.** El verificador lo dio como dos claves para un repo; no es así y quedó pinneado: la clave sale de `nameWithOwner`, que GitHub devuelve canónico, así que `SPF13/Cobra` y `spf13/cobra` guardan la misma. Lo que se pide se pide tal cual y no se reescribe el caso.
- **S11 no tenía ningún test.** Ahora se afirma el contrato que el cliente necesita (`data-busy` en los dos formularios, el aviso `class="busy"` y `form[data-busy]` en `app.js`), con control de mutación: borrar el atributo hace fallar el test. El comportamiento **en el navegador** sigue sin test automático, y eso se queda dicho (L9).
- El resto de lo verificado quedó confirmado sin cambios: orden de escrituras y su reporte (snapshot → alias → registro, error a la vista, nunca medio registrado), lista blanca de `estado`, ausencia de redirección abierta, y que el test de entrada inválida prueba que no hubo request.
