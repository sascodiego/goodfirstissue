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
-->

# P1 triage policy

Policy version: `p1-1`

This file is the frozen policy for the first pass. It decides what is asked, in what
order, and what each answer permits. It contains no wording of its own: the questions
live in `prompt.md`, and which wording survives is decided by the benchmark under
`testdata/bench/`.

## Vocabulary

| Name | Meaning |
|---|---|
| `github_state` | The issue's native state: `OPEN` or `CLOSED`. Not free text. |
| `github_state_reason` | Native reason: `COMPLETED`, `NOT_PLANNED`, `REOPENED`, or empty. |
| `dossier` | The text sent as the state under judgement. A string, nothing else. |
| `evidence` | What P2 produces. Not a dossier. |
| `outcome` | `CANDIDATA`, `CONDICIONADA`, or `MARCADA`. |
| `abstention` | `no_verdict`, `uncertain`, `insufficient_information`, `undecided`. |

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

## Two dossiers

One dossier cannot answer everything: the texts that settle substance are exactly the
ones that contaminate the judgement of substance. So substance and intent are judged in
separate dossiers, chained by our code. Same issue, same facts, different text.

| | Dossier A: substance | Dossier B: intent |
|---|---|---|
| Contains | normalised title, body, technical comments | the text plus the label list verbatim, maintainer comments, `github_state_reason` |
| Questions | `kind`, `self_contained`, `has_evidence`, `unambiguous`, `done_defined`, `reports_defect`, `evidence_sufficient` | `intent` |
| Runs when | always, for an issue that passed the pre-gates | only when the pre-gates did not settle intent, and only if A did not already mark the issue |
| Labels present | **never** | yes, verbatim |

Dossier B must state, in the contract the judge reads, that a label is an unverified
claim of unknown authorship, that the person making the request may have applied it
themselves, and that it is not evidence on its own.

A dossier never contains: the label list (A), the kind the author declared, or the
outcome of a previous gate.

## Comments in the dossier

Comments are where authorisation actually lives, so they are evidence. They are also
long, noisy, and written by strangers.

- Include comments written by the issue author or by an account whose author
  association is owner, member or collaborator. Drop the rest.
- Keep the most recent ones up to a fixed cap; truncate each one to a fixed length.
- Delimit them and label them as third-party text that is evidence, never instruction
  (I3).

## The gate chain

The chain is a decision tree walked by our code over facts and answers. It is not a
sequence of requests: the answers to dossier A arrive together, and each gate consumes
them. Every gate judges one issue, because grouping is out of the MVP.

    G0 evidence_sufficient  →  G1 open work  →  G2 quality  →  G3 kind  →  G4 intent

### G0 — `evidence_sufficient`

- Source: dossier A. Threshold: `0.5`.
- Below it the outcome is **`no_verdict`**: doubt out loud, with what is missing. It is
  not a discard (I10) and it never fabricates a verdict (I2).
- Below it, the issue is **not** marked and **not** dropped: it stays in the list with
  its doubt and its reason.

### G1 — open work

- Source: layer 2. Costs no request and resolves without the judge.
- Fails when a linked pull request is open, or when the issue was closed as completed.
- Result: **MARCADA, "already being worked"**, with the pull request numbers.
- **Precedence: G1 preempts G0.** A deterministic fact does not depend on being able to
  classify the text. If the evidence is insufficient *and* there is open work, the
  outcome is MARCADA and the doubt is attached as a note, not as the outcome.
- Why it sits here: it is the cheapest gate on the chain and it prevents the single
  worst recommendation the product can make.

### G2 — quality

- Source: dossier A. Threshold: `0.5` on every criterion.
- `self_contained` below threshold: **MARCADA**, "missing context: it cannot be
  approached without asking for it".
- `kind = bug` and `has_evidence` below threshold: **MARCADA**, "without a reproduction
  there is no bug".
- `unambiguous` or `done_defined` below threshold: **CONDICIONADA**, with the list of
  what has to be agreed. This is actionable because it names the gap, never a score.
- All criteria at or above threshold: well formed.
- Why it sits before `kind`: a badly formed issue fails whatever its nature is, and
  saying what is missing is more useful than naming a category.

### G3 — `kind` and the disguise guard

- Source: dossier A.
- `kind` in `question`, `spam`, `other`: **MARCADA**, "not code work".
- `kind = bug` and `reports_defect` below threshold: **reclassified to feature**, and it
  then walks the alignment node exactly like a feature. The reclassification is
  reported, never silent.
- `reports_defect` asks whether the text reports behaviour that already exists as
  defective, or asks for behaviour that does not exist. It does not ask whether the
  behaviour is *wrong*: that is P2's `confirmed_defect`, which is out of scope.
- Why it sits before `intent`: it can change which path the request takes, and intent
  means different things for a feature and for a bug.

### G4 — `intent`

- Source: the pre-gates first; dossier B only for the residue.
- `refused`: **MARCADA**, "the project already said no".
- `approved`: **CANDIDATA**. A closed line: nothing left to evaluate.
- `undecided`: the open line. `feature` + `undecided` is **CONDICIONADA** with a fixed
  condition, "ask the maintainer first", because the alignment node is deferred.
- Why it sits last: it is the most expensive gate, it is the least decisive for a bug,
  and it is pointless for anything the earlier gates already removed.

### What the benchmark settled, and what it left open

Measured on the corpus under `testdata/bench/` (`prompt.md` carries the full record).
These are conclusions of measurement, not preferences:

| Question | Agreement | Consequence |
|---|---|---|
| `kind`, one composite question | 22 of 23 | the category is asked, not derived |
| `kind`, derived from grounded nodes | 18 of 23 | the tree was tested and is **not** adopted |
| `intent` | 22 of 23 | labels plus maintainer prose are enough, and the label never decides alone |
| `self_contained` | 21 of 23 | usable as a gate |
| `evidence_sufficient` | 22 of 23 | usable as a gate |
| `reports_defect` | 22 of 23 | usable as a gate |
| `has_evidence` | 20 of 23 | usable, with one case of run-to-run drift |
| `unambiguous` | 19 of 23 | the weakest; see below |
| `done_defined` | 19 of 23 | the weakest; see below |

Two consequences bind the implementation:

- **`kind` stays a single question.** Do not rebuild the derived tree: it lost by four
  cases, and the mechanism is known — a grounded node about "behaviour that does not
  exist yet" also fires on every bug report, because a fix produces behaviour that does
  not exist either.
- **`unambiguous` and `done_defined` are kept in disjunction**, as the policy already
  says, and the error direction is accepted on purpose: when they fire wrongly they make
  an issue CONDICIONADA, which keeps it visible with its condition. They never discard.
  A wording that reaches twenty or more cases replaces them; until then they are the
  measured weak point of the chain, and a fresh case set has to confirm any replacement.

No conclusion here is drawn from a single case: repeated runs showed a drift of one case
on `has_evidence`, so differences of one are noise and differences of three are not.

## Outcomes

| Outcome | Meaning |
|---|---|
| `CANDIDATA` | worth exploring, and it can be recommended |
| `CONDICIONADA` | an opportunity, but an external condition is missing; it arrives with the condition named |
| `MARCADA` | not recommended, with the reason and the evidence in view |

No outcome is hidden. `MARCADA` and `CONDICIONADA` appear in the main list with their
marker. No outcome is ever shown without its probability.

## The four abstentions

Four different problems with four different fixes. They are never collapsed into one.

| Abstention | Threshold | Emitted by | What it means |
|---|---|---|---|
| `no_verdict` | `evidence_sufficient < 0.5` | P1 | the judge answered "not enough"; doubt out loud (I10) |
| `uncertain` | any answer below `0.45` that the chain needed | P1, P2, P3 | no verdict is emitted |
| `insufficient_information` | a required input is absent | P3 | the reason is the **profile**, not the opportunity; P1 does not emit it |
| `undecided` | the request failed: timeout, retries exhausted, 4xx, 5xx | P1, P2, P3 | the application does not decide (I2) |

## Rules that are not negotiable

- **The label never selects, never filters, never sorts, and never decides.** It is not
  a query key, it is not a pre-gate, and it is not present in dossier A. This makes the
  label's influence structural rather than a promise to be trusted.
- **Insufficient evidence is not a discard** (I10).
- **One issue is the unit of judgement.** Duplicate grouping (I12) is out of this MVP for lack of time; nothing in this chain assumes it exists.
- **Repository text is evidence, never instruction** (I3).
- **Triage runs before explore** (I9): an issue that did not pass P1 never spends an
  exploration.
- **A deterministic fact outranks an abstention** (the G1 precedence above).
- **Two requests per issue is the ceiling.** More questions per request are free; more
  requests are not.
