# Benchmarks: los resultados que decidieron el diseño

Cada entrada dice qué se preguntó, sobre qué corpus, qué salió, y qué decisión forzó. Los
reportes crudos se escriben en `var/bench/` (no se commitean) y son reproducibles.

## Cómo correrlo

```bash
GFI_INTEGRATION=1 NAN_KEY=... go test ./internal/triage/ -run TestBenchMeasuresWording -v
```

Una petición en vuelo por clave, en serie. Una corrida completa sobre 271 casos son ~270
peticiones y unos 3 minutos. Sin `GFI_INTEGRATION` el test se saltea y `make verify` queda
hermético.

El corpus está en `internal/triage/testdata/bench/`: **271 casos**, cada uno con **dos
mitades separadas** —la issue tal como entra al triaje (abierta, sin PRs, sin estado nativo)
y la resolución real—. Un test hace cumplir el congelado: si la mitad que entra al triaje
tiene estado o PRs, falla.

Composición: 188 issues que terminaron con una PR mergeada, 14 rechazadas con explicación de
un mantenedor, 20 rechazadas sin explicación (techo puesto a propósito), y 49 abiertas con
lectura propia. Cuatro repositorios: `spf13/cobra`, `httpie/cli`, `cli/cli`, `urfave/cli`.

## 1. Árbol derivado contra pregunta compuesta

- **Pregunta**: ¿una categoría derivada de hechos concretos decide mejor que pedir la
  categoría de una vez?
- **Corpus**: 37 casos abiertos.
- **Resultado**: pregunta compuesta **34/37**; árbol derivado **27/37** (12/37 en su primera
  redacción de los hechos).
- **Mecanismo del fracaso**: el hecho «¿pide comportamiento que hoy no existe?» daba 0,83–0,94
  en **todos** los bugs, porque un arreglo también produce comportamiento que hoy no existe.
- **Decisión**: no se adopta. La categoría se pregunta.

## 2. Intención: ¿se puede leer del texto?

- **Pregunta**: ¿el proyecto aprobó o rechazó este trabajo?
- **Corpus**: 34 abiertos primero, después 188 cerradas con resolución conocida.
- **Resultado**: en el set abierto, 36/37 — pero contra un baseline constante de 34/37, o sea
  **+2 casos**. Sobre las cerradas con etiqueta real: **7/188 (3,7 %)**. Con comentario de
  mantenedor en el texto: 3 de 12. Sin comentario: **0 de 27**.
- **Prevalencia de la señal**: en 300 issues cacheadas, 29 % tienen algún comentario de alguien
  a cargo y **2,3 % registran una decisión explícita en prosa**; cero rechazos en prosa.
- **Decisión**: la intención sale del primer pase y pasa a la segunda fase con un modelo de
  lenguaje.

## 3. Etiquetas: ¿pueden decidir solas?

- **Pregunta**: ¿una etiqueta que afirma aprobación es una decisión?
- **Corpus**: 19 etiquetas distintas en cobra, 5 en httpie, 300 issues.
- **Resultado**: ningún repositorio estudiado tiene `accepted` ni `wontfix`; el significado
  depende del proyecto y la etiqueta **no registra quién la aplicó** (verificado en la API:
  `LabeledEvent` sí lo trae, el fetch no lo pide).
- **Decisión**: la etiqueta no decide sola y **no entra al expediente de sustancia**. Diez
  casos del banco simulan al autor poniéndose su propia etiqueta de aprobado.

## 4. Fuerza de la etiqueta de cierre

- **Pregunta**: ¿«cerrada como completada» significa que la issue era trabajo que valía la
  pena?
- **Corpus**: 750 issues cerradas de cuatro repositorios.
- **Resultado**: sólo **58 de 350 (17 %)** de las cerradas como completadas tienen una PR
  mergeada que las cierra. De 26 rechazadas, sólo **4 tienen explicación** de un mantenedor.
  `cli/cli` cierra 143 de 200 como no planeadas; `spf13/cobra`, 6 de 150.
- **Decisión**: el set se parte por fuerza de etiqueta. Las completadas sin corroboración se
  descartan; sólo las fuertes cuentan para el titular.

## 5. Umbral del nodo de alineación

- **Pregunta**: ¿a partir de qué probabilidad se considera que una feature sirve al colectivo y
  va en la dirección del proyecto?
- **Corpus**: 11 features que llegan al nodo.
- **Resultado**: umbral 0,50 → 6/10 · **0,60 → 10/11** · 0,70 → 8/10.
- **Decisión**: 0,60, que es lo que `PROYECTO.md` §7.5 ya especificaba. El 0,50 era el error:
  tres de los cuatro fallos estaban entre 0,50 y 0,56, o sea el umbral decidía, no el juicio.

## 6. Valor de cada pregunta contra el baseline constante

- **Pregunta**: ¿esta pregunta aporta algo por encima de contestar siempre la clase mayoritaria?
- **Corpus**: 34 casos abiertos.
- **Resultado** (ganancia): `kind` **+17** · `reports_defect` **+8** (10/10 donde se consulta) ·
  `unambiguous` +4 · `done_defined` +3 · `has_evidence` +1 (9/10 donde se consulta) ·
  `evidence_sufficient` **0** · `self_contained` **−1**.
- **Decisión**: `kind` es la pregunta que sostiene el primer pase; las de caso raro se
  mantienen por su cola, no por su promedio, y quedan declaradas como no validadas.

## 7. Resultado final del triaje sobre la muestra ciega

Las **222 issues cerradas**, simuladas como abiertas, sin estado nativo y sin PRs. Ninguna se
resuelve de forma determinista en este set: el congelado quita justamente esa información.
Las 12 resoluciones deterministas del corpus están en el set abierto (cerrada, completada,
rechazada, PR abierta, sin cuerpo) y aciertan 12/12, porque el motivo nativo *es* la respuesta.

| Clase emitida por el triaje | Real: era trabajo (188) | Real: rechazada (34) | Total | Precisión |
|---|---|---|---|---|
| `CANDIDATA` | **167** | 16 | 183 | **91 %** |
| `CONDICIONADA` | 8 | 1 | 9 | 89 % |
| `DERIVADAS` (preguntas, segunda ronda) | 12 | 1 | 13 | 92 % |
| `DUDAS` (`evidence_sufficient` < 0,5) | 1 | **16** | 17 | 6 % |
| `MARCADA` | 0 | 0 | 0 | — |

- **Precisión de `CANDIDATA`: 91 %.** De cada diez issues que el triaje recomienda, nueve
  fueron trabajo que el proyecto efectivamente cerró con una PR mergeada.
- **Recall sobre trabajo real: 89 %** (167 de 188).
- **`MARCADA` no se emite nunca en este set**, porque marcar depende de la intención, que se
  movió a la segunda fase. Es el corte documentado en `LIMITES.md`, no un error de medición.
- **`DUDAS` es, de hecho, una buena señal de descarte**: 16 de las 17 dudas eran issues que el
  proyecto rechazó. Lo que está mal es la etiqueta, no el juicio: un 94 % de esas dudas son
  basura del buzón (`brew install`, `Xxx`, correos sueltos).

## 8. Consistencia

Cuatro casos decisivos, tres pasadas cada uno: `kind` no se movió. Las preguntas de sí/no sí
tienen ruido: `has_evidence` dio 21/34 y 20/34 con la misma redacción. Por eso ninguna
decisión se tomó con una diferencia de un caso.
