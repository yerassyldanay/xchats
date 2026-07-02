# 13 — KB Facts & Anti-Hallucination — Architecture Decision

> **Amended by [`14-draft-staging-and-retrieval.md`](14-draft-staging-and-retrieval.md):** Decision 6
> step 4 (prose grounding judge) is **deferred from v1**; Decision 8's embeddings rejection is relaxed —
> embeddings are allowed **for the Knowledge lane only** (facts still resolve by exact lookup); and topic
> bodies no longer carry fact tokens (tokens appear only in model replies). Everything else stands.

**Purpose:** capture the architecture we agreed on for storing exact facts and for keeping the
assistant from hallucinating. This is a decision record, not an implementation plan. The other
`plan/*.md` docs should be updated to reflect it (see "Docs to update" at the end).

---

## Problem

The assistant must state **exact** facts — prices, tariffs, delivery times, working hours,
addresses, phone numbers, limits — and must do so in **many languages** (Kazakh, Russian, and
more later). Two failure modes to eliminate:

- **Fabricated specifics** — a wrong or invented price/number.
- **Unsupported prose** — plausible-sounding claims not backed by the knowledge base.

## Core principle: two lanes

We treat the KB as **two different kinds of information**, handled differently:

| Lane | What it holds | Correctness need | How it's produced |
|---|---|---|---|
| **Facts** | prices, tariffs, times, hours, addresses, phones, limits | **exact, verbatim** | code substitutes the stored value |
| **Knowledge** | policies, descriptions, how-things-work | faithful paraphrase | the LLM writes prose, then it's verified |

The split *is* the anti-hallucination strategy: exact things are never authored by the model;
explanatory things are authored but checked.

---

## Decision 1 — Storage: uniform, typed, multilingual tables

- Every fact entity is its **own typed table** with **concrete columns** (e.g. `kb_tariffs`,
  `kb_products`). No generic key–value bag.
- **Language is a row, never a column.** One row per `(entity, language)`; adding a language
  means inserting rows, with **no schema change**. There are no `name_ru` / `name_kk` columns.
  A `*` language marks a language-neutral value (e.g. a phone number).
- All fact tables follow this **same shape**, and the existing prose table (`ai_topics`)
  already does too — so the whole KB is uniform.
- The old generic value store (`ai_values`) is **removed**. It's replaced by these typed tables.

Illustration (conceptual, not DDL) — one tariff, two languages = two rows:

```
kb_tariffs
  slug     lang   name   price            limit_text
  growth   ru     Рост   24 990 ₸/мес      до 2 000 платежей/мес
  growth   kk     Өсу    24 990 ₸/ай       айына 2 000 төлемге дейін
```

Values are stored **verbatim** (units included); code never reformats a number.

## Decision 2 — Reference model: tokens resolve to typed rows

- The model refers to a fact with a **token** `{{table.slug.field}}` — `table` selects the
  fact table, `slug` the row, `field` the column (e.g. `{{tariff.growth.price}}`).
- Code resolves the token against the typed table **for the reply's language** and substitutes
  the verbatim value. The model never emits the number itself.

## Decision 3 — Template layer (deterministic; the Facts lane)

- The LLM is forbidden to write a digit for a known fact; it must emit the token.
- After generation, code substitutes each token from the fact tables. **Fail closed:** an
  unresolved token (missing row/field/language) never ships — it becomes a
  manual-review/holding reply.
- Result: for facts, a fabricated value is **impossible by construction**.

## Decision 4 — Response-check layer (the safety net over both lanes)

Two checks run on the drafted reply:

- **Number check (deterministic).** Every currency-/unit-adjacent number in the reply must
  trace back to an injected source value; if a number appears from nowhere, escalate. This is a
  cheap, exact backstop that also catches any digit the model wrote inline against the rules.
- **Prose grounding judge (LLM).** A separate, cheap model checks that every non-numeric claim
  is supported by the injected knowledge (topics). Unsupported ⇒ escalate. The judge is
  **biased to escalate** — on doubt or error it defers to a human, never auto-approves.

Numbers are guarded deterministically; prose is guarded by the judge. We do **not** rely on the
LLM judge to catch numeric errors.

## Decision 5 — When each approach applies

- **Exact value** (price, limit, time, address, phone) → **template** (Decision 3).
- **Explanation / free text** → **prose + judge** (Decision 4).
- **Every reply** → **number check** as a backstop, regardless of lane.
- **Unknown / not in KB** → **escalate** (fail closed), never guess.

## Decision 6 — Pipeline order

For each drafted reply:

```
1. escalate gate        — if the model couldn't answer from the KB, stop → human
2. template render       — substitute fact tokens; unresolved ⇒ fail closed
3. number check          — untraceable number ⇒ escalate
4. prose grounding judge — unsupported claim ⇒ escalate
5. media validation      — keep only known media refs
6. human review          — the final gate (drafts are never auto-sent)
```

## Decision 7 — Language handling

- Values are injected for the **reply's language**. If a row is missing for that language,
  fall back to the **org default language**, then to the `*` (neutral) row; if still missing,
  **escalate**.
- Prompt injection shows the model, per fact, the **token + a human label (its meaning) + the
  value** for the reply language — so it can pick the right fact (e.g. which tariff fits the
  customer) while still being required to output the token, not the number.
- At publish time, a **completeness check** ensures every referenced entity has a row for each
  required language (or a `*` fallback), so the assistant never hits a missing-language hole.

## Decision 8 — Explicitly rejected / out of scope

- **No vector search / embeddings** for facts (nearest-neighbor can return the *wrong* tariff).
  Embeddings are not needed at all while the KB fits in the prompt; they'd only ever help the
  Knowledge lane, and only once size forces retrieval. Out of scope here.
- **No per-language columns** (`name_ru`, …) — language is always a row.
- **No generic `ai_values` bag** — replaced by typed tables.
- **No auto-send** — human review stays the final gate.

---

## Docs to update to reflect this decision

- `plan/9-database-schema.md` — replace `ai_values` with the typed fact tables
  (`kb_tariffs`, `kb_products`, uniform per-language shape); note "language is a row."
- `plan/8-ai-assistant.md` — (one doc; it absorbed the former `8.1`–`8.7`) the two lanes and the
  typed-table storage model; the FACTS catalog block (token + label + value) and the "never write a
  digit; emit `{{table.slug.field}}`" rule; the post-processing pipeline order (Decision 6) with the
  number check and grounding judge; the evals/guardrail metrics with fail-closed behavior.
- `plan/11-ai-design-overview.md` / `plan/12-playground-build.md` — the playground now curates
  typed `tariffs`/`products` (per language) instead of generic `values`.
