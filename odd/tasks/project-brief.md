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
| T1 | vocabulario fijo de secciones con su naturaleza, `Parse` estricto y `Version()` | S1, S2, S5, S13 | inline | pending |
| T2 | `prompt.md` embebido, `Extract` con `pig` inyectado y `insufficient_information` por sección | S3, S9, S10, S14 | inline | pending |
| T3 | caché pineado a `commit_sha` y una corrida por repositorio | S4, S7 | inline | pending |
| T4 | auditoría de citas contra el clon, `Source` que alimenta `AlignmentDossier` | S2, S6 | inline | pending |
| T5 | `projectbrief` y `models.project_brief` en config, `gfi brief`, ayuda y README | S12, S15 | inline | pending |
| T6 | `make verify` y cierre del documento | S15 | inline | pending |

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
