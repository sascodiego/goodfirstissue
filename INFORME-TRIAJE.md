# Informe del triaje inicial

Resultado de pasar la muestra ciega por el triaje real. Es el informe que sostiene la
presentación: qué se midió, qué salió, y qué se puede afirmar sin exagerar.

Fecha: hackathon, noviembre 2026 · rama `feat/p1-bench` · issue #14.

## Qué se midió

El corpus congelado tiene **271 casos** de cuatro repositorios (`spf13/cobra`, `httpie/cli`,
`cli/cli`, `urfave/cli`), y cada caso tiene **dos mitades separadas**:

- la **issue que entra al triaje**: abierta, sin estado nativo, sin PRs y con las etiquetas de
  aprobación reales retiradas. Diez casos simulan además que el propio autor se puso la
  etiqueta de aprobado;
- la **resolución real**: completada con una PR mergeada, o rechazada con o sin explicación.

Un test hace cumplir el congelado: si la mitad que entra al triaje trae estado o PRs, la
corrida falla.

## 1. Determinístico contra modelo

| Bloque | Casos | Resueltos con un `if` | Pasan al modelo |
|---|---|---|---|
| Issues abiertas del corpus | 49 | **12** (cerrada · completada · rechazada · PR abierta · sin cuerpo) | 37 |
| Cerradas simuladas como abiertas | 222 | **0** (el congelado quita estado y PRs) | 222 |

Las **12 resoluciones deterministas aciertan 12 de 12**, porque el motivo nativo *es* la
respuesta. En el set ciego **nada es determinístico por construcción**: es lo que hace que la
medición mida el modelo y no las reglas.

## 2. La muestra ciega, por clase emitida

222 issues cerradas, contra la resolución real:

| Clase emitida por el triaje | Era trabajo (188) | Fue rechazada (34) | Total | Precisión |
|---|---|---|---|---|
| **CANDIDATA** | **167** | 16 | 183 | **91 %** |
| CONDICIONADA | 8 | 1 | 9 | 89 % |
| DERIVADAS (preguntas, segunda ronda) | 12 | 1 | 13 | 92 % |
| **DUDA** | 1 | **16** | 17 | 6 % |
| MARCADA | 0 | 0 | 0 | — |

### Qué dicen estos números

- **De cada diez issues que el triaje recomienda, nueve fueron trabajo real**: el proyecto las
  cerró con una PR mergeada. Precisión de `CANDIDATA`: **91 %**.
- **Recall sobre trabajo real: 89 %** (167 de 188).
- **`MARCADA` no se emite nunca en esta muestra.** Marcar depende de la intención del proyecto,
  y la intención se movió a la segunda fase (`LIMITES.md`, corte 2). El costo está medido: 16
  de las 183 candidatas (8,7 %) son trabajo que el proyecto ya había rechazado.
- **`DUDA` es una buena señal.** 16 de las 17 dudas eran issues que su proyecto rechazó: 94 %.
  No por lucidez del modelo, sino porque son basura del buzón de `cli/cli` — `brew install`,
  `Xxx`, correos sueltos, issues de una palabra. El juicio funciona; por eso el triaje lo
  muestra como categoría propia y no lo esconde dentro de otra.

## 3. Los dos números que hay que decir sin adornos

1. **El primer pase no detecta rechazos.** Acierta 0 de 34. No es una regresión: es el corte
   decidido cuando se comprobó que intentar esa lectura en el primer pase acertaba 3,7 % sobre
   188 issues con resolución conocida. La segunda fase, con un modelo de lenguaje que lee toda
   la metadata y los comentarios priorizando la voz del owner, es la que lo resuelve.
2. **El primer pase sí decide bien lo que decide.** Cuando emite `CANDIDATA`, acierta nueve de
   cada diez; y no descarta nada sin motivo: lo que no puede clasificar lo deja en `DUDA`.

## 4. Qué se puede afirmar en la presentación

| Afirmar | Evidencia |
|---|---|
| El embudo descarta sin gastar modelo | 12 de 12 con seis reglas deterministas |
| La clasificación del pedido es sólida | `kind` 34/37, y **+17 casos** sobre contestar siempre la clase mayoritaria |
| Lo que recomienda, lo recomienda bien | precisión de `CANDIDATA` 91 % sobre 183 emisiones |
| La alineación de features funciona | **10 de 11** con umbral 0,6 |
| La duda es honesta y útil | 17 dudas, 16 de ellas rechazadas de verdad |
| El diseño se decidió midiendo | ocho benchmarks en `BENCHMARKS.md`, dos de ellos contra nuestras propias hipótesis |

## 5. Lo que no se puede afirmar

- No se puede decir que el triaje entienda la intención del proyecto: no lo hace, y por eso
  está separado.
- No se puede decir «acierto del 89 %» sin decir sobre qué: ese 89 % es **recall de las 188
  issues que eran trabajo real**, no acierto global. El acierto global sobre las 222 es 75 %,
  y baja justamente por lo que el corte dejó fuera.
- No se puede decir que las compuertas de caso raro funcionen: `self_contained`,
  `evidence_sufficient`, `done_defined` y `unambiguous` no están validadas, y una de ellas rinde
  peor que contestar siempre lo mismo.

## 6. Reproducir

```bash
GFI_INTEGRATION=1 NAN_KEY=... go test ./internal/triage/ -run TestBenchMeasuresWording -v
```

Una petición en vuelo por clave, en serie, ~3 minutos para 271 casos. Los reportes crudos se
escriben en `var/bench/`. Sin `GFI_INTEGRATION` el test se saltea y `make verify` queda
hermético.
