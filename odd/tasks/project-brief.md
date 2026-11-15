# project-brief

La ficha técnica del proyecto: el insumo que el nodo de alineación consume y nadie produce.
Issue #28 · rama `feat/28-project-brief` · base `main` @ `fae8940`.

## Specs

**S1.** Texto de la issue: «Un documento por repositorio, en Markdown, con esqueleto fijo —misma
decisión que el perfil (#8), y por la misma razón: el consumidor es Clef y recibe un `string`—:»
«`## Qué es` / `## Stack técnico            ← cruza con el CV` / `## Arquitectura y componentes` /
`## Convenciones y cómo se construye   (build, test, lint, dónde vive cada cosa)` /
`## Dirección: qué acepta y qué no     ← lo que lee la alineación` /
`## Límites de lo observable           (qué no se puede saber sin leer todo el repo)`». El
`←` y el paréntesis son anotaciones de la issue, no texto del encabezado.

**S2.** Texto de la issue: «Cada afirmación con su evidencia (`archivo:línea`), igual que
`internal/explore`. Sin evidencia no es retrato, es opinión del modelo.» Una línea por
afirmación: `- <afirmación> — evidencia: <archivo>:<línea>`.

**S3.** Texto de la issue: «Un repositorio clonado produce el retrato con las seis secciones,
cada una con al menos una evidencia `archivo:línea`.»

**S4.** Texto de la issue: «**Pineado al commit**: si el commit cambia, el retrato se recalcula.
Nunca se presenta uno viejo como fresco (I14).»

**S5.** Texto de la issue: «El retrato declara explícitamente **qué secciones son técnicas y
cuáles son de proyecto**, para que ningún consumidor las mezcle.» Texto de la issue: «Lenguajes,
frameworks, herramientas, comandos de build y test | **conocimiento técnico** | entra al balde
`knowledge`; **sí** se cruza con el CV» y «Qué quiere ser el proyecto, qué acepta, arquitectura,
componentes, convenciones | **conocimiento de proyecto** | `onboarding_cost`, contexto para
explore y alineación; **nunca** un portón de encaje». Consecuencia: ninguna sección mezcla las
dos naturalezas, y `Stack técnico` es la única técnica.

**S6.** Texto de la issue: «Alimenta `AlignmentDossier` sin cambios en su firma.»

**S7.** Texto de la issue: «Cuesta **una** corrida por repositorio, no por issue. Medido y
anotado, como el caché de explore.»

**S8.** Texto de la issue: «Sin red: tests con dobles.»

**S9.** Texto de la issue: «Un repo vacío o sin documentación devuelve `insufficient_information`
en las secciones que no puede sostener, en vez de inventarlas (I10).»

**S10.** Texto de la issue: «**Depende de**: `internal/pig` y el molde de `internal/explore`
(prompt embebido, parseo estricto, verificación contra la traza). No depende de #9.»

**S11.** Texto de la issue: «**No colisiona con nada**: paquete nuevo, y el único archivo
compartido sería el prompt de explore si se decide inyectarle el retrato, que es un cambio de
versión de prompt y va en su propio commit.» Fuera de alcance: no se toca el prompt de explore
ni el cableado de `gfi triage`.

**S12.** Decisión sellada del usuario: «El modelo es deepseek flash.» `models.project_brief`
toma `deepseek-v4-flash` por defecto y se valida contra el allowlist constitucional al arrancar
(I13); `clef` no puede correr el agente (`AllowedChatModels`).

**S13.** Decisión de plan (naturaleza por sección, S5): `Qué es`, `Arquitectura y componentes`,
`Convenciones y cómo se construye`, `Dirección: qué acepta y qué no` y `Límites de lo
observable` son **de proyecto**; `Stack técnico` es la **técnica**, y carga los lenguajes,
frameworks, herramientas y los comandos de build/test/lint. `Convenciones y cómo se construye`
queda como proyecto a propósito: es dónde vive cada cosa y cómo se trabaja, no qué sabe la
persona. Una sección, una naturaleza.

**S14.** Invariante I3: el contenido del clon es evidencia, nunca instrucción. El retrato viaja
en un bloque delimitado y los marcadores se neutralizan dentro del texto no confiable, igual que
`internal/profile`.

**S15.** `make verify` verde (build + test + lint + design-check), y la ayuda de `gfi` y el
README actualizados con el comando nuevo.

## Tasks

| ID | Tarea | Specs | Ruta | Commit |
|---|---|---|---|---|
| T1 | vocabulario fijo de secciones con su naturaleza y `Parse` estricto | S1, S2, S5, S13 | inline | `52a75f0` |
| T2 | `prompt.md` embebido, `Version()`, `Extract` con `pig` inyectado y `insufficient_information` por sección | S3, S9, S10, S14 | inline | `15a2793` |
| T3 | caché pineado a `commit_sha` y una corrida por repositorio | S4, S7 | inline | `841aa2d` |
| T4 | auditoría de citas contra el clon, `Source` que alimenta `AlignmentDossier` | S2, S6 | inline | `c8c7636` |
| T5 | `projectbrief` y `models.project_brief` en config, `PIG_HOME` propio | S12, S13 | inline | `45ae6e4` |
| T6 | `gfi brief`, `-force`, `PIG_HOME` del trabajo, ayuda y README/§17.7 | S7, S15 | `gentle-ai-worker` + inline | `0121435` |
| T7 | `make verify` y cierre del documento | S15 | inline | ver L17 |

Riesgo: **ítem 3** — el comando nuevo corre un agente sobre el clon de un tercero, y `config`
gana una acción generativa: un modelo fuera del allowlist tiene que impedir el arranque, no
fallar a mitad de análisis (I13). Revisión con lentes `risk` y `reliability`.

## Log

L1. «Vamos a crear un nuevi worktree para trabajar en la issue 28»

L2. «El modelo es deepseek flash.» «plan sellado»

L3. «Hacemls ODD comoleto»

L4. Evidencia (molde leído, `path:line`):
- `internal/profile/profile.go:1-24` — el molde que la issue manda copiar: paquete que produce
  un documento Markdown con esqueleto fijo, sin JSON, porque el consumidor es Clef.
- `internal/profile/extract.go:29-31` — `RunFunc`: el seam de transporte por el que un test
  inyecta un doble y ningún test arranca un proceso.
- `internal/profile/extract.go:110-126` — `//go:embed prompt.md`, reglas en el system prompt,
  datos no confiables en el bloque delimitado.
- `internal/triage/brief.go:15-21` — `BriefSource`: la interfaz que el retrato tiene que
  satisfacer, ya cableada en `cmd/gfi/triage.go:105` con `triage.CloneBrief`.
- `internal/triage/dossier.go:149-159` — `AlignmentDossier(issue Issue, brief string)`: la firma
  que no se toca (S6).
- `internal/triage/bench_integration_test.go:64-83` — `benchBrief` alimenta la alineación con
  cuatro briefs congelados; es el test al que la issue se refiere.
- `internal/config/config.go:41-56` — allowlist constitucional y `DecisionModel` (I13).
- `internal/config/config.go:129-137` — precedente explícito: el modelo de una acción generativa
  vive en `models.<acción>`, no en el bloque del job.
- `internal/repo/repo.go:52-62` — `Cloner.Head`: de dónde sale el `commit_sha` del pinneo.
- `internal/store/store.go:54,108` — `Read`/`Write` del store: el molde del caché de
  `internal/profile/cache.go:11` (`sync.Mutex` de proceso, `cacheName`).

L5. Decisión (naturaleza de `Convenciones y cómo se construye`): proyecto, no técnica, aunque la
issue anote «build, test, lint» entre paréntesis. Los **comandos** de build/test son conocimiento
técnico (tabla de S5) y viajan en `Stack técnico`; la sección de convenciones describe dónde vive
cada cosa y cómo se trabaja. Alternativa descartada: marcarla técnica, que la vuelve una sección
de dos naturalezas y rompe S5 en el único lugar donde se puede romper —el consumidor que la lee—.
Consecuencia en el prompt: `Stack técnico` incluye una línea de comandos de build/test/lint
citada de un archivo real.

L6. Decisión (nombre): paquete `internal/projectbrief`, no `internal/brief` (ya hay
`triage.CloneBrief` y `triage/brief.go`, que es otro artefacto: README + subjects, gratis) ni
`internal/bootstrap` (el issue etiqueta la ola, no el artefacto).

L7. Decisión (declaración de naturaleza en el documento, S5): el retrato abre con dos líneas
fijas —`TÉCNICO: Stack técnico` y `PROYECTO: <las cinco>`— que `Parse` **valida** contra las
constantes del paquete. La declaración no se cree: el mapa sección→naturaleza es una constante en
código, y si el modelo la contradice el parseo falla. Alternativa descartada: marcar el
encabezado (`## Stack técnico [técnico]`), que rompe el vocabulario fijo de S1.

L8. Evidencia (TDD en rojo, una por tarea):
- T1: `go test ./internal/projectbrief/` → «undefined: Parse», «undefined: Sections»
  («no non-test Go files» antes de eso).
- T2: «undefined: projectbrief.Extractor», «undefined: projectbrief.Input».
- T3: «unknown field Store in struct literal of type projectbrief.Options».
- T4/T5: ver L14.

L9. Decisión (qué es «verificación» acá, contra S10): la evidencia se audita **contra el clon**, no
contra la traza. `internal/explore` verifica contra la traza porque allí la pregunta es si el
agente *miró*; acá la pregunta es si la cita *se puede comprobar*, y eso se contesta en el árbol:
el archivo existe, está dentro del clon, y llega a esa línea. Verificar el transcript daría un
aviso por cada afirmación derivada de un `grep` sobre el árbol, que es un falso positivo sobre un
retrato correcto. Lo verificado es más fuerte que lo transcripto: una cita que no existe en el
árbol se reporta, y ninguna cita que exista se puede inventar.

L10. Evidencia (el test encontró el fixture, no el producto): las pruebas que parchean el caché
para sembrar una entrada corrupta no sembraban nada, porque `store.Write` envuelve el documento
en su propio sobre (`{"schema":1,"saved_at":…,"data":{…}}`) y el parche buscaba `entries` en la
raíz. Se corrigió el helper del test (`patchCache`), no el código: el mutex de caché no tenía
defecto, y el test que no podía fallar ahora falla si el sobre cambia.

L11. Decisión (mensaje de `validateTools`): el helper pasa a recibir el nombre del campo, porque
ahora lo usan dos trabajos. El texto del rechazo dejó de nombrar una sola tarea («exploration is
read-only» → «the agent reads, it never writes»). Es un cambio de mensaje no pedido, mínimo y
necesario: con dos secciones válidas, el viejo afirmaba algo falso para una de ellas. Ningún test
ni documento pineaba el texto (verificado con `grep`); la regla de `PROYECTO.md` §17.7 no cambia.

L12. Decisión (`-force`): el retrato se cachea por commit, y un árbol puede cambiar sin que el
commit se mueva (un clon a medio actualizar, un archivo borrado a mano). `-force` responde la
misma pregunta por segunda vez; la respuesta nueva reemplaza la guardada.

L13. Evidencia (diferencial sobre el código compartido, antes del cambio): una copia de `fae8940`
en `mktemp -d` con `gfi config check` da `exit 0` con la configuración válida, `explore.tools grants
"bash", which can write: exploration is read-only` para una herramienta de escritura, y `unknown
field "project_brief"` para el bloque nuevo. Los tres se repiten contra el árbol nuevo en T6: lo
único que puede cambiar es el texto del rechazo (L11), documentado y sin test ni documento que lo
pinee.

L14. Evidencia (T6, delegado a `gentle-ai-worker` con superficies disjuntas y reanudado por el
padre): RED observado por el worker — «forced.Force undefined (type projectbrief.Input has no field
or method Force)» y «undefined: runBrief», los dos paquetes en `[build failed]`; GREEN con
`go test -count=1 -run 'TestExtractWithForce|TestRunBriefNeeds' ./internal/projectbrief/ ./cmd/...`
en `ok`. El test de `-force` además afirma lo que importa: la respuesta forzada reemplaza la
guardada y una lectura posterior sin `-force` devuelve el texto nuevo, con la corrida contada dos
veces y el costo medido de la corrida forzada.

L15. **Defecto que encontró la revisión del worker, y se cerró acá** (no estaba en el plan de T5 y
era real): `config.EnsureDirs` y `config.Resolve` no cubrían `project_brief.home`, y nadie escribía
`pig.WriteConfig` para ese `PIG_HOME`. `projectbrief.New` hacía `filepath.Abs`, pero el home
quedaba sin `agent/{models.json,mcp.json}`: `gfi brief` arrancaba un `pig` sin provider y fallaba
después de un minuto de reintentos, por la razón equivocada. Cerrado en tres piezas: el home entra
en `Resolve` y en `EnsureDirs` (con el test de directorios extendido), y `prepareBriefHome` en
`cmd/gfi/brief.go` verifica `NAN_KEY` **antes** de nada y genera la configuración del agente desde
`config.AllowedChatModels`. Tests nuevos: `TestPrepareBriefHomeWritesTheAgentConfiguration` (el
`models.json` generado lleva el allowlist y `NAN_KEY`) y `TestPrepareBriefHomeNeedsTheKey` (sin
clave no se escribe nada y el error la nombra).

L16. Evidencia (lint, `make verify` primera corrida): `make verify` salió **0 por culpa del pipe**
(`make verify | tail` devuelve el código de `tail`), y `golangci-lint` había fallado con dos
hallazgos reales: `fmt.Fprint` a stdout sin chequear (`errcheck`) — un retrato que llegó a medio
pipe no es un retrato, ahora se devuelve el error — y una ley de De Morgan en
`audit_test.go` (`staticcheck`). Corregidos; `golangci-lint run` → `0 issues`. Lección aplicada: la
terna build+test+lint se lee del log, no del código de salida del pipe.

L17. Evidencia (diferencial de T7, mismo `cwd` y misma configuración): `gfi config check` contra la
copia de `fae8940` y contra el árbol nuevo da salida **byte a byte idéntica** y `exit 0`. Las dos
diferencias encontradas y aceptadas: (a) el bloque `project_brief` es aditivo — la copia vieja lo
rechaza como `unknown field`, que es el comportamiento buscado de `DisallowUnknownFields`; (b) el
texto del rechazo de herramientas cambió por L11. `models.project_brief = "qwen3.8-max"` → error
que nombra el campo y el allowlist (I13).

L18. Evidencia (cierre, `make verify` segunda corrida): build OK, `go test ./...` verde (13
paquetes), `go vet` limpio, `golangci-lint run` → `0 issues`, `gofmt -l .` vacío, `design-check`
sin diferencias. `VERIFY_EXIT=0` leído del log, no del código de salida del pipe (L16).

L19. Estado del cierre: T1–T7 hechas; seis commits de trabajo y este de cierre. Fuera de alcance lo
que el issue deja abierto: inyectar el retrato en el prompt de explore (sería un commit de versión
de prompt aparte, S11) y cablear `gfi triage` al retrato en vez de a `CloneBrief` — hoy
`AlignmentDossier` se alimenta sin cambios de firma y con el adaptador probado, pero el cableado de
producción sigue usando el brief gratis de README + subjects, que cuesta cero corridas. Decidir eso
es una decisión de producto con costo por corrida, no una tarea de este issue.

L20. Límite declarado: el retrato no se corrió contra un repositorio real en este worktree, porque
`NAN_KEY` no está configurada. Lo que está verificado es el camino completo con dobles (S8): la
construcción del pedido, el parseo estricto, el pinneo, el caché, la auditoría contra el clon y el
dossier de alineación. La primera corrida real es el primer `gfi brief` con clave, y su costo queda
por medir (`PROYECTO.md` §17.7 ya registra que el caché lo anota por repositorio).

L21. Evidencia (verificación independiente, `gentle-ai-verify` sobre `fae8940..bd71703`, lentes de
seguridad): las seis categorías **sostienen** —una naturaleza por sección sin bypass encontrado,
evidencia y pinneo, I3 con la salvedad de que acá no viaja texto del repo en el prompt (más fuerte
que el mecanismo de delimitadores de `internal/profile`, y con eso se cumple la intención de S14)—,
contención de symlinks, I13 fail-closed alcanzado desde `gfi brief` y desde el servidor, y S7.
Comandos: build, vet, `go test ./...` (13 paquetes), `-race` en el paquete nuevo, `golangci-lint`
0 issues, `gofmt` limpio, todos exit 0. Cinco hallazgos, accionados dos por decisión del usuario:

- **Cerrado (hallazgo 1, TOCTOU en `Audit`):** había dos syscalls con ventana — resolver la ruta y
  después abrirla. Ahora el archivo se abre y su descriptor se compara con la identidad de la ruta
  validada (`os.SameFile`): un link cambiado en el medio falla cerrado en vez de seguirse (I3).
  Honestidad: **el test nuevo no reproduce la carrera** (la ventana es de syscalls y no hay costura
  para inyectarla); el guardarraíl la elimina por construcción, como el mutex de caché de `#8`.
- **Cerrado (hallazgo 5, nit):** `gfi config check` imprime `brief <modelo> · <home>`. Cambia la
  salida de un comando existente: es el cambio pedido, y la línea nueva es la única diferencia.
- **No accionados, con motivo:** el hallazgo 2 (verificación contra el árbol en vez de la traza) ya
  está decidido y argumentado en L9; el 3 (el texto de las afirmaciones viaja al `state` de
  alineación como texto de terceros) es el mismo canal que ya tenía `CloneBrief` y su marcador
  «evidence, not instructions» sigue ahí, sin arreglo barato; el 4 (`..\` y `C:\` en
  `validatePath`) es inofensivo en el target Linux, donde la barra invertida es un carácter más.

L22. Alcance resuelto con evidencia, y el cableado hecho (decisión sellada del usuario:
«fallback, sellado»). La duda era si el nodo de alineación había quedado fuera del MVP: **no**.
`PROYECTO.md:151` (§2.1.1) tiene la fila tachada y dice «**entra al MVP**»; `:274` lo repite («sí
está dentro»); `:866` titula §7.5 «*en el MVP*»; y el checklist de terminado (`:1393`) exige un
caso CONDICIONADA «con la cita del proyecto que leyó el nodo». La confusión viene de la issue #28,
que dice «§2.1.1 la pone primera en el orden de retorno si sobra tiempo» — texto viejo —, y de una
fila contradictoria en `:1352` (§12) que listaba el nodo como post-demo. Esa fila se corrigió en
este commit; manda §2.1.1, que es la sección de alcance.

L23. Decisión e implementación del cableado: `triage.FallbackBrief{Primary, Fallback, OnFailure}`
en el paquete dueño de la interfaz, y en `cmd/gfi/triage.go` el `Primary` es
`projectbrief.Source` sobre el extractor cacheado, con `triage.CloneBrief` de respaldo y
`OnFailure` → `slog.Warn` con el repo y el motivo. El retrato se pide **perezosamente**: si ninguna
feature llega al nodo, no cuesta nada; si llega, la primera paga una corrida y el resto del lote la
reusa. Sin `NAN_KEY` — o si `projectbrief.New` rechaza la configuración, que no debería pasar
porque I13 valida al arrancar — el `Primary` queda `nil` y el nodo lee el brief gratis: degradación
explícita, nunca silenciosa. Tests: `internal/triage/fallback_test.go` (seis casos, incluido «el
primario contesta vacío también es falla») y dos en `cmd/gfi/triage_test.go` que afirman la
decisión (con clave → `Primary` puesto y el home del agente configurado; sin clave → `Primary` nil,
`Fallback` es `CloneBrief` sobre el directorio de clones y **nada** se escribe en el home).
Costo documentado en `PROYECTO.md` §7.5 y `README.md`: §7.5 decía «no cuesta ninguna petición
extra» y eso dejó de ser cierto, así que ahí está el número y la condición.
