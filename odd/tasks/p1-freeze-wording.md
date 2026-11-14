# Feature: congelar la redacción de P1 (issue #14)

Contrato de la primera pasada: qué preguntas se le hacen al juez, en qué orden deciden las
compuertas, y qué evidencia entra al expediente. El laboratorio que mide la redacción antes
de congelarla. Sin UI y sin política implementada: eso es #9.

Rama `feat/p1-bench` · worktree `/src/worktrees/goodfirstissue/feat-p1-bench` · issue #14.

## Specs

S1. `criteria.md` congela el vocabulario (`github_state`, `github_state_reason`, dossier,
`evidence`), las tres capas y la regla determinista-primero: si un `if` lo resuelve, no
gasta una pregunta.

S2. La etiqueta nunca es determinista —su significado depende del repositorio y no registra
quién la aplicó—, nunca decide sola, y **no aparece en el expediente de sustancia**. En el
expediente de intención entra verbatim, marcada como afirmación de autoría desconocida.

S3. La cadena es `G0 evidence_sufficient → G1 trabajo abierto → G2 calidad → G3 kind → G4
intent`, con el umbral de 0,5 en cada criterio, y **G1 precede a G0**: un hecho determinista
no depende de poder clasificar el texto.

S4. Las cuatro abstenciones llevan umbral y etapa que las emite: `no_verdict`
(`evidence_sufficient` < 0,5, P1), `uncertain` (p < 0,45), `undecided` (fallo del endpoint,
I2) e `insufficient_information`, que es de P3 por falta de perfil y P1 no emite.

S5. P1 no responde «qué te falta»: eso es P2 (requisitos en tres baldes, lagunas) y P3.

S6. Dos expedientes: A de sustancia, sin etiquetas y sin la declaración del autor en el
título; B de intención, con etiquetas verbatim, `state_reason` y comentarios de mantenedor.
Las pre-compuertas nativas corren antes de cualquier juicio.

S7. El texto de terceros viaja delimitado y rotulado como evidencia, nunca instrucciones
(I3), y los recortes son visibles.

S8. El banco tiene 23 casos etiquetados a mano con procedencia `real` o `synthetic`, y el
manifiesto declara la cobertura exigida: una trampa listada sin caso hace fallar un test.

S9. La medición reporta acierto por pregunta, abstención, consistencia con tres pasadas y
guarda esperado contra obtenido. `make verify` queda hermético sin `GFI_INTEGRATION` y
`NAN_KEY`.

S10. La redacción congelada en `prompt.md` es la que ganó la medición, y una guarda falla si
el contrato y el código que lo envía divergen.

S11. Ningún archivo del contrato nombra modelos, endpoints ni código.

## Tasks

| ID | Tarea | Specs | Ruta | Commit |
|---|---|---|---|---|
| T1 | Vocabulario, capas, regla determinista-primero, etiqueta, dos expedientes, cadena de compuertas, abstenciones, presupuesto | S1-S5 | inline | `eb7ea20` |
| T2 | Banco: 23 casos y manifiesto con cobertura y procedencia | S8 | inline | `c0a5959` |
| T3 | Pre-compuertas, normalización del título y expedientes A y B, con tests offline | S6, S7 | worker | `0af43c4` |
| T4 | Medición: banco de preguntas, cliente serial, comparación, consistencia, reporte | S9 | inline | `9137d5e` |
| T5 | Redacción congelada, tabla de lo que el banco decidió y guarda contra la deriva | S10, S11 | inline | `a51e613` |

Riesgo: **ítem 6** — la redacción es el producto y ningún test la cubría; el banco y la
guarda del contrato son la mitigación.

## Log

L1. Pedido original, verbatim:

> Vamos a trabajar con la siguiente metodologia, vamos a leer una issue, analizarla planificarla y
>   cuando yo te de mi concentimiento explicito recien ahi podes empezar a escribir codigo. vas a trabajar
>  en un worktree nuevo. la issue es la 14

L2. La issue pedía solo dos `.md`. El owner amplió el alcance: laboratorio primero, redacción
al final, y «más casos, son baratos». Se registró como desviación sellada.

L3. Decisión del owner sobre precedencia: **G1 gana a G0**. Contradice el orden literal de la
cadena de #14 y queda escrito en `criteria.md` con su motivo.

L4. Decisión del owner sobre la etiqueta: es información para el juez, no un filtro. Se
resolvió con dos expedientes: la etiqueta no puede contaminar la sustancia porque no la ve,
y aporta en la intención porque ahí sí está.

L5. Premisa del owner puesta a prueba: «un árbol de decisiones da mejor criterio que una sola
solicitud de clasificación». Medición: 22/23 la clasificación plana, 12/23 el árbol en su
primera redacción y 18/23 reescrito. **No se adopta.** Mecanismo: el hecho «pide
comportamiento que hoy no existe» daba 0,83–0,94 en todos los bugs, porque un arreglo también
produce comportamiento que hoy no existe.

L6. `unambiguous` y `done_defined` son el punto débil medido (19/23 cada una); una segunda
redacción las empeoró (13/23 y 16/23). Se congelan en su mejor versión, se mantienen en
disyunción y el error cae del lado seguro: CONDICIONADA de más, nunca descarte.

L7. Ruido observado: `has_evidence` dio 21/23 y 20/23 con la misma redacción. Las decisiones
se tomaron con umbral de 3 casos, no de 1. La segunda pasada se escribió mirando la primera:
su mejora incluye ajuste al set, y cualquier redacción futura se mide sobre casos nuevos.

L8. Agrupación de duplicados **fuera del MVP** por decisión del owner: decidir si dos textos
reportan el mismo problema es trabajo de un modelo de lenguaje sobre cada par, y no hay
tiempo. Documentado en `PROYECTO.md` §2.3 y §7.1, y en `criteria.md`. La unidad pasa a ser
la issue sola; I12 no aplica en el MVP.

L9. `intent` reescrita: opciones `approved` / `refused` / `undecided`, todas en positivo y
con el criterio de qué cuenta como evidencia; se quitó de la pregunta el caso nativo que ya
resuelve la pre-compuerta, y se sumó el nodo `maintainer_spoke` para separar «nadie habló» de
«alguien habló sin decidir». Sobre 34 casos medidos acierta 33/34.

L10. Casos inválidos corregidos: el disfraz de feature ya no confiesa que la capacidad no
existe; el caso de pregunta se reemplazó por un reporte de crash que pregunta si el
comportamiento es intencional; y los casos con estado nativo cerrado, cuerpo vacío o PR
abierta dejaron de medirse con el modelo, porque la pre-compuerta los resuelve antes: ahora
el banco los reporta como «resueltos sin preguntar».

L11. El banco creció a 46 casos, 35 reales de los dos repositorios del corpus. Con datos
reales el acierto de punta a punta es 34/46 (74 %), no el 96 % que sugería el set sintético:
`done_defined` y `unambiguous` son responsables de seis de las doce fallas.
