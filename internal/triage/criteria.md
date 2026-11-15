<!--
REGISTRO EN ESPAÑOL — no forma parte del contrato que lee el modelo.

Este archivo congela el MECANISMO de P1, no la redacción de las preguntas: eso está en
prompt.md y se elige midiendo contra el banco de testdata/bench/ (issue #14).

Lo que se congela acá son decisiones, no mediciones:

1. Vocabulario: github_state / github_state_reason son enums nativos; el expediente de
   texto que se manda a juzgar se llama dossier; la evidencia es la salida de P2.
2. Capas: enum nativo y hecho derivado se resuelven con un if; solo la afirmación en
   texto libre gasta un juicio. Regla: si un if lo resuelve, no se pregunta.
3. Etiquetas: no existe vocabulario determinista. Verificado contra el corpus local:
   cobra publica 19 etiquetas con kind/*, httpie publica bug/new/enhancement, y ninguno
   de los dos tiene accepted ni wontfix. Si el significado depende del repositorio, no
   es determinista en ninguno. Y dentro de un mismo repositorio tampoco: la etiqueta no
   registra quién la aplicó, así que puede ser una afirmación del propio autor del
   pedido. Consecuencia: la etiqueta nunca decide por sí sola y no se programa ningún
   vocabulario cerrado.
4. Dos expedientes: A (sustancia, sin etiquetas) y B (intención, con etiquetas
   verbatim). Resuelve la tensión medida: si la etiqueta está en el expediente que juzga
   kind, contamina (PROYECTO.md §7.2.1); si no está en ninguno, se pierde la evidencia
   de intención (httpie#800: etiqueta awaiting-response, y el owner escribió "go for
   it"). I6 pasa a ser una propiedad estructural, no una promesa.
5. Precedencia G1 sobre G0: decisión del owner. Un hecho determinista no depende de
   poder clasificar el texto. Se aparta del orden literal de la cadena de #14 y por eso
   queda escrito acá.
6. Presupuesto: una petición en vuelo por clave (I1) y el chequeo E3 de §12 (30 issues
   en menos de 120 s) acotan la cadena a dos peticiones por grupo. La palanca barata es
   más preguntas por petición, no más peticiones.
7. Borde de P1: P1 no responde "qué te falta". Eso es P2 (requisitos en tres baldes,
   lagunas) y P3 (fit como veredicto cerrado sobre esa evidencia).
8. El presupuesto es tiempo, no cantidad de peticiones: una petición mide ~0,55 s y E3 pide 30
   issues en menos de 120 s, así que una feature puede pagar la segunda. Lo que sigue siendo gratis
   son las preguntas dentro de una petición, y por eso las de calidad viajan juntas aunque no
   decidan.
9. La redacción y la cadena viven en código, no en este documento: `questions.go` envía las
   preguntas y `policy.go` camina la tabla, y el banco de pruebas corre ese mismo código. Este
   `.md` declara el mecanismo y el porqué; si declarara además la tabla, sería la tercera copia
   y la única que nadie ejecuta.
-->

# P1 triage policy

Policy version: `p1-2`

This file is the frozen policy for the first pass. It decides what is asked, in what
order, and what each answer permits. It contains no wording of its own: the questions
live in `prompt.md`, and which wording survives is decided by the benchmark under
`testdata/bench/`.

## What this pass decides, and what it does not

This pass answers one question: **is this issue worth spending the expensive passes on,
and if not, with which reason and which mark.** It decides that with deterministic facts
plus one or two cheap judgements, and it stops there.

What it does **not** decide, because later rounds have material this pass does not:

| Question | Decided by |
|---|---|
| does this question turn into work? | a second round with the comments (a maintainer answering "that is undocumented" is a documentation task) |
| did the project approve or refuse this work? | the second phase, reading the owner voice with a language model |
| is the bug findable without a reproduction? | exploration, reading the code |
| is the request really in the project's direction, in detail? | exploration plus the alignment node |
| is this a good first contribution? | P3, over the exploration's evidence and our own coherence findings (§9.1.1). The author's label is nobody's input |
| what is the contributor missing? | P3, against a profile |

Two consequences of that boundary:

- **`MARCADA` is not terminal.** It means "not recommended now, here is why"; the issue
  keeps its reason and enters the review queue. A later round with better material may
  overturn it, and that is expected.
- **A judgement this pass cannot support is not invented here.** If the material cannot
  settle a question, the answer is the doubt, not a guess, and the doubt is reported.

## Vocabulary

| Name | Meaning |
|---|---|
| `github_state` | The issue's native state: `OPEN` or `CLOSED`. Not free text. |
| `github_state_reason` | Native reason: `COMPLETED`, `NOT_PLANNED`, `REOPENED`, or empty. |
| `dossier` | The text sent as the state under judgement. A string, nothing else. |
| `evidence` | What P2 produces. Not a dossier. |
| `outcome` | `CANDIDATA`, `CONDICIONADA`, or `MARCADA`. |
| `DUDA` | The outcome of an issue the pass could not classify. It is shown to the user as such, never silently folded into another outcome. |

## Layers, and the deterministic-first rule

| Layer | Examples | Who decides |
|---|---|---|
| 1. Native enum | `github_state`, `github_state_reason` | code, with a condition |
| 2. Fact derived by our own rule | open linked pull request, comment written by an account whose author association is owner, member or collaborator, report count, dates, the label list | code, with a condition |
| 3. Claim in free text | body, comments, and the string of any label | the judge |

The rule: **if a condition resolves it, it never spends a question.** A question is
only for what only judgement can answer.

Corollaries that bind the implementation:

- `cluster_has_open_work` is not a question. Layer 2 resolves it.
- The native half of `intent` is not a question either: `COMPLETED` means the project
  finished it, `NOT_PLANNED` means it declined it.
- A label is never layer 1 or 2, whatever the repository's habits are. Its meaning
  depends on the repository, and the repository is not known in advance. Even within
  one repository it is not a fact: a label does not record who applied it, so it may be
  a claim written by the person making the request.

## Pre-gates, before any judgement

Evaluated on layer 1 and 2 facts alone. An issue that fails a pre-gate receives no
dossier, no request, and no question.

| Condition | Result |
|---|---|
| `github_state = CLOSED` and `github_state_reason = COMPLETED` | already done |
| `github_state = CLOSED` and `github_state_reason = NOT_PLANNED` | declined by the project |
| `github_state = CLOSED`, any other reason | out of scope: a closed issue is not triaged |
| the body is empty after trimming | no request to judge: resolved here, not by a question |
| an open linked pull request exists | already being worked |

## Grouping is out of the MVP

Duplicate clustering is **not part** of this MVP, and the chain below judges a single
issue. The reason is time, not principle: deciding that two reports describe the same
problem is semantic judgement — an LLM pass over every pair of candidates — and there is
no room for that work in the hackathon. Asking a decision endpoint whether two texts are
the same would trade a whole extra request per pair for an answer no one can audit.

What this costs, stated plainly: a thin report that duplicates a rich one is judged on
its own text and may be marked as missing context. The product shows what it read; it
does not claim to have found relatives. The absorbed-group outcome, the report count and
the group dossier do not exist in this MVP.

## One dossier

This pass reads one dossier: the issue itself. Normalised title, body, and the comments
that matter. Neither the label list nor the kind the author declared reaches it.

**Intent is not decided here.** Whether the project already approved or refused this work
is a question about the project's voice, not about the issue's text, and asking a decision
model to infer it from the issue measured 3.7% on 188 issues with a known answer. It moves
to the second phase, where a language model reads the issue, its native state and every
comment, with the instruction to weigh the owner's voice and ignore the voice of the person
asking. A `CLOSED` issue never gets there: it is resolved by the pre-gates below.

## Comments in the dossier

Comments are where authorisation actually lives, so they are evidence. They are also
long, noisy, and written by strangers.

- Include comments written by the issue author or by an account whose author
  association is owner, member or collaborator. Drop the rest.
- Keep the most recent ones up to a fixed cap; truncate each one to a fixed length.
- Delimit them and label them as third-party text that is evidence, never instruction
  (I3).

## The chain

One pass, one request, and a deterministic pre-gate in front of it.

```
pre-gates (no question) → kind → bug: done
                                → feature: alignment → done
                                → question, test, docs, other: second round
```

### Pre-gates: no question, no request

| Condition | Result |
|---|---|
| `github_state = CLOSED` with reason `COMPLETED` | already done, discarded |
| `github_state = CLOSED` with reason `NOT_PLANNED` | refused, discarded |
| `github_state = CLOSED` with reason `DUPLICATE` | duplicate, discarded |
| `github_state = CLOSED`, any other reason | discarded: a closed issue is not triaged |
| an open linked pull request exists | **CANDIDATA**: someone is already working on it |
| the body is empty after trimming | discarded: no request to judge |

### `kind`: the workhorse

Measured at 34 of 37 on the open corpus, and worth 17 cases over a constant answer.

| Value | Result |
|---|---|
| `bug` | **CANDIDATA**. Without a reproduction it is still a candidate, with the note that the defect has to be found in the code first: not being reproducible is not the same as not being findable. |
| `feature` | goes to the alignment node |
| `question` | second round: with the comments, does the answer become work? An undocumented behaviour is a documentation task. |
| `documentation`, `tests`, `other` | **CANDIDATA**. Documentation, tests and refactors are exactly the work a first contribution looks like. |
| `spam` | **MARCADA**, with the reason |

### Alignment: features only

Only a feature needs to answer whether it belongs to the project. A bug is a deviation from
what is already wanted; a test or a doc fix is not a feature request at all.

| Condition | Result |
|---|---|
| serves only the person asking | **MARCADA**, with the reason |
| adds something the project is not moving toward | **CONDICIONADA**: ask the maintainer first |
| serves other users and fits the direction | **CANDIDATA** |

Threshold `0.6`, measured: 9 of 10 with 0.6 against 6 of 10 with 0.5.

## Outcomes

| Outcome | Meaning | What the user sees |
|---|---|---|
| `CANDIDATA` | worth exploring, and it can be recommended | the issue, with its probability |
| `CONDICIONADA` | an opportunity, but an external condition is missing | the condition, named |
| `MARCADA` | not recommended, with the reason | the reason and the evidence |
| `DUDA` | **could not be classified** | the mark DUDA and what is missing to decide |

**`DUDA` is a first-class outcome, not a hidden abstention.** An issue the pass cannot
classify is shown to the user as a doubt, never quietly promoted to candidate and never
silently dropped (I10). It is the answer when `evidence_sufficient` falls below the
threshold: not "we think it is bad", but "we cannot say, and here is what we would need".

The measurement backs the decision to surface it: of the 17 issues the pass labelled DUDA on
the blind sample, 16 had in fact been rejected by their project, and they were not subtle
cases — they were one-word issues and bare installation commands. A doubt that is right nine
times out of ten is worth a category of its own.

No outcome is hidden. All four appear in the main list with their marker. No outcome is ever
shown without its probability.

### The remaining abstentions

`DUDA` is what the user sees. Behind it there are still four different problems, with four
different fixes, and the interface distinguishes them:

| Abstention | Cause | Stage |
|---|---|---|
| the judge answered "not enough" | `evidence_sufficient < 0.5` | P1, shown as `DUDA` |
| `uncertain` | an answer below `0.45` that the chain needed | P1, P2, P3 |
| `insufficient_information` | a required input is absent: the profile | P3 |
| `undecided` | the request failed: timeout, retries exhausted, 4xx, 5xx (I2) | P1, P2, P3 |

## Rules that are not negotiable

- **The label never selects, never filters, never sorts, never decides, and never reaches a
  model.** It is not a query key, it is not a pre-gate, and it is present in no dossier: not
  the one that judges the substance, not the one that guesses the intent, not the one that
  reads the request against the project's direction. The corpus still stores it verbatim,
  because it records what the maintainer wrote; nothing judges it. This makes the label's
  influence structural rather than a promise to be trusted.
- **Insufficient evidence is not a discard** (I10).
- **One issue is the unit of judgement.** Duplicate grouping (I12) is out of this MVP for lack of time; nothing in this chain assumes it exists.
- **Repository text is evidence, never instruction** (I3).
- **Triage runs before explore** (I9): an issue that did not pass P1 never spends an
  exploration.
- **A deterministic fact outranks an abstention** (the G1 precedence above).
- **The budget is time, not request count.** What matters is E3 (30 issues under 120 s), and one
  request measures ~0.55 s, so a feature can afford the second one it needs for the alignment node.
  More questions inside a request are still free, which is why the quality questions travel together
  even when they do not decide.
