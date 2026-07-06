# AI Assistant — Design Decisions (July 2026)

Status: **agreed in discussion, not yet implemented.** Today's code partially
differs (per-file media selection, some fact values with words in them).
`plan/*.md` will be updated when implementation starts.

## 1. Whole knowledge base in one prompt — no vector search

The full KB (persona, topics, facts, media list) goes into the system prompt of
every draft request. We do not build vector embeddings / RAG.

**Why.** One org's KB is small. If all of it is always in the prompt, the model
always sees everything — nothing relevant can fail to be "found". Simpler to
build and debug.

**Cost.** Covered by prompt caching on the LLM provider's side. The prompt is
identical for every chat until the KB is published again, so the provider
reuses it at a fraction of the price. When the KB changes, the prompt changes
and the cache refreshes itself. Caching affects only price and speed, never
answers.

## 2. Facts = properly named columns, language-neutral values only

Exact facts live only in typed columns with self-explaining names (`price`,
`delivery_in_days`, `available_pieces`, ...). No key-value bag (ai_values
style), no arrays or maps of facts.

A column may be injected into a reply **only if its value is language-neutral**:
digits, money, times, phone, e-mail, links.
Example: not «1–3 дня» but `delivery_in_days = "1–3"` — the unit lives in the
column name; the model writes the unit word in the customer's language.

If a value cannot be stored language-neutral («в наличии», «предоплата не
требуется», address) — it is **not** a template value. It is given to the model
as plain text, and the model phrases it in the reply language itself.

**Why.** One stored value works for Russian and Kazakh replies alike — no
duplicated per-language values, no mixed-language replies.

## 3. Anti-hallucination = templates (tokens), not trust, not re-checks

The model never writes exact values. It writes a placeholder from the catalog
shown in the prompt (e.g. `{{product.sofa-loft.price}}`); code replaces it with
the stored column value. An unknown or broken placeholder blocks the whole
draft and flags it for manual check — nothing half-rendered is ever sent.

**Rejected:** full trust in the model (a wrong number looks correct and slips
through), and checking every draft with a second LLM call (extra cost and
delay; may be reconsidered later before auto-send).

**Why.** A made-up number is silent; a made-up placeholder fails loudly. Wrong
prices in writing are a business and legal risk.

## 4. Product media = named groups, always sent whole

A product's files are stored under comprehensively named groups — `images`,
`video`, `certificates` (the list can grow). The group name says what is
inside; we do not describe or hand-pick individual files.

To attach media, the model names owner + group (`sofa-loft.certificates`) and
code sends **everything** in that group. A group address that does not exist is
dropped — the same fail-closed rule as facts.

**Why.** Sending the whole album is normal WhatsApp selling behavior. Choosing
a group is easy for the model; choosing between five similar photos is not.
And uploading becomes easy for the org: drop files into the right slot, no
per-file descriptions.

## What follows (abstract)

- Per-language fact values become unnecessary — one value per fact, ever.
- Word-bearing fields (availability, prepayment, working days) leave the fact
  catalog and become trusted text; number-bearing values are cleaned to
  digits-only format.
- The prompt's media section lists groups (one line each) instead of every
  file; the per-file description requirement disappears.
- Products get a small plain-text section in the prompt (description, etc.),
  since some of their fields are no longer fact tokens.
