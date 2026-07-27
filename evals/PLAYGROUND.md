# Eval Playground

## What this is

A place to imitate different versions of the product — different database columns,
different tables, different prompt wording, different response formats, different numbers
of rows — and see how well each version answers real customer questions, using real AI
models, before writing a single line of migration code.

Every imitated version lives in its own folder under `scenarios/`. Making a new version =
copy a folder, change what you want to test, run it. Nothing shared gets touched, so
experiments never step on each other.

## Why this exists (what came before, and why it wasn't enough)

The first version of this eval was 3 hand-written prompt files and one shared question
list. That answered "does the model behave" for one comparison, but it broke down for two
reasons a code review caught:

1. **Hand-written prompts drift from their own data.** Change one number in a fake
   product's price, and nothing forces the prompt file to update to match. We hit this
   exact class of bug three separate times in one afternoon — a check would quietly test
   the wrong thing and nobody would notice until the numbers looked "too clean" or "too
   broken" to be real.
2. **The old exam only ever looked at the model's raw answer.** It never tested the actual
   idea we care about — a placeholder token gets validated, then a real value gets
   substituted into it, and an unknown or broken token blocks the whole reply instead of
   shipping something wrong. That's the core mechanism. A grading tool that never runs it
   can't tell us if it works.

This playground fixes both: scenarios are built from one data file (never hand-duplicated),
and a small harness actually runs the validate → inject → check pipeline, not just "did the
model say something plausible."

## Structure

```
evals/
├── PLAYGROUND.md              ← this file
├── README.md                  ← run commands
├── models.yaml                ← the model list, in one place, used by every scenario
├── common/
│   ├── shop-questions.yaml    ← question bank shared by every "shop" scenario
│   └── xpayment-questions.yaml
├── scenarios/
│   ├── shop-current/                  ← one folder = one imitated product version
│   │   ├── scenario.yaml              ← meta: what this is, which files it uses
│   │   ├── schema.sql                 ← documentation only — never executed
│   │   ├── data.yaml                  ← the fake rows — the ONE source of truth
│   │   ├── frame.txt                  ← rules/persona wording, with fill-in slots
│   │   ├── tests.yaml                 ← which common questions + any scenario-only ones
│   │   └── generated/                 ← written by the harness, never hand-edited
│   │       ├── prompt.txt
│   │       ├── catalog.json
│   │       └── promptfooconfig.yaml
│   ├── shop-decisions-v1/
│   ├── xpayment-decisions-v1/
│   ├── shop-scale/                    ← data.yaml + frame.txt ONLY — no scenario.yaml of
│   │                                     its own; shared by the three below via `limits:`
│   └── shop-scale-{10,20,30}/         ← same pool, capped to N products each
└── runs/
    ├── INDEX.md                       ← deliberately retained evidence only
    └── 2026-07-06_shop-compare/       ← one local folder per run
        ├── SUMMARY.md                 ← the human report — read this first
        ├── CONTRACT.md                ← token/injection/media verdicts per answer
        └── results.json               ← promptfoo's raw data (not committed)
```

Generated scenario files and run directories are local by default. Keep source scenarios,
prompts, cases, and harness code in Git. Force-add only the reviewed manifest, snapshots,
judged output, and reports from a run that supports a durable decision, then list it
manually in `runs/INDEX.md`; generated viewer files and raw provider output remain
uncommitted.

**To imitate a new version:** copy a scenario folder, change its `data.yaml` (add a
column, remove a table, add 50 more products, whatever you're testing), change `frame.txt`
if the wording needs to change too, run it. A `scenario.yaml` can also point at another
scenario's `data.yaml` while supplying its own `frame.txt` — so you can test "same data,
different wording" without duplicating the data file.

## The three-part contract every scenario writes

- **Tables and columns** (`schema.sql`, documentation) + **the actual rows**
  (`data.yaml`, the real input) — including edge cases on purpose: a product with zero
  stock, a topic with no media, an unknown city the KB never mentions.
- **The exact placeholder catalog and media-group catalog**, generated FROM the rows —
  never hand-typed, so it can never drift from the data.
- **Customer messages and the expected behavior** (`tests.yaml`, usually mostly the
  shared `common/` bank plus a few scenario-specific ones).

## The harness — what it actually runs

Five small Go programs. Most scenarios are free-standing on purpose, since they exist to
try out ideas the product hasn't built yet — no dependency on the product's `backend/`
code. One pipeline is the exception: a scenario with `pipeline: schema_kb_v1` in its
`scenario.yaml` loads a schema-shaped fixture (`internal/kbfixture`, matching the
product's actual DB columns) and renders it through `backend/aiprompt` directly — the
exact prompt/catalog/response code the production backend will eventually call, not a
harness-side reimplementation of it. Both pipelines produce the same
`prompt.txt`/`catalog.json`/`promptfooconfig.yaml` outputs and go through the same
`judge`/`report` steps below.

1. **`render`** — reads `data.yaml`, fills `frame.txt`'s slots with the generated
   KNOWLEDGE BASE / MEDIA / FACTS text, writes `generated/prompt.txt` +
   `generated/catalog.json` (every valid token, every valid media group) +
   `generated/promptfooconfig.yaml` (wires in `models.yaml` + the resolved
   `tests.yaml`, including each test's `history:` turns rendered into the `{{history}}`
   var). This is the only place scenario content becomes a prompt — nothing is ever
   hand-typed into a prompt file again. `render` also FAILS (no prompt written) if a fact
   value contains a brace character, if the rendered prompt references a `{{token}}` this
   exact render's catalog doesn't resolve, or if a `%%SLOT%%` was left unfilled — the
   "prompt and catalog can never disagree" claim is an enforced check, not a convention
   (it caught a real `frame.txt`/`data.yaml` ref mismatch while building shop-scale).
2. **promptfoo runs** against the generated config, exactly as before — this part doesn't
   change, promptfoo is still the right tool for "call N models with M questions."
3. **`judge`** — reads promptfoo's raw answers and, per answer: parses the JSON (tolerant
   of a ```` ```json ```` wrapper) → checks every `{{token}}` used exists in the
   catalog (an unknown or malformed one gets the verdict **"BLOCKED — fail-closed"**,
   exactly what the real product would do; the same scan also covers `escalation_reason`,
   not just the customer-facing text) → substitutes the catalog's values into the text →
   checks the **substituted** text for ANY leftover brace character (not just `{{` — a
   single-brace typo like `{product.price}` never matches a token span in the first place,
   so it has to be caught this way instead), invented digits (any digit run, not just
   multi-digit — numbered-list markers like "1." / "1)" are allow-listed first), and correct
   media groups/refs. It also enforces the model's declared `reply_language` field matches
   what a test expects, not just a Kazakh-letter heuristic on the text. A test can also
   declare `outcomes:` — >=2 labeled alternative expectation blocks; the answer passes
   that gate if ANY one block's declared checks all hold (for genuinely ambiguous
   questions where two different behaviors are both right — see xph2 in
   `common/xpayment-history-questions.yaml`). This is the part
   that didn't exist before promptfoo alone and is the actual reason this playground
   exists — it tests the injection idea, not just "did the model try." It also computes an
   **estimated** cost per answer (see `models.yaml`'s pricing fields and README's "Known
   limits" — never presented as real spend).
4. **`report`** — turns the judge's verdicts + promptfoo's own pass/fail into one
   `SUMMARY.md` per run: a stats table per scenario × model (model-behavior pass %,
   contract pass %, an honestly-labeled cost estimate, latency, tokens, prompt/completion
   share), a scale-comparison table when 2+ `shop-scale-N` scenarios ran together, the
   answers that failed quoted in full. It does not modify the curated `runs/INDEX.md`.
5. **`run`** — ties the four steps together: render → promptfoo → judge → report, for
   one scenario or all of them, against whichever models you name.

## What gets measured every run

- Did the model choose the *correct* placeholder (not just *a* placeholder)?
- Does the model ever write a literal exact value itself, bypassing the token?
- Does an unknown or malformed token get blocked, the way the real product would block it?
- After the real value is substituted in, does the Kazakh/Russian reply stay clean (no
  stray words from the other language leaking in through an injected value)?
- Did the model choose the correct media group or file, and reject attaching something
  that doesn't exist?
- Does the answer still hold up as the row count grows? `shop-scale-{10,20,30}` share ONE
  30-product pool (`scenarios/shop-scale/data.yaml`) capped per scenario via `scenario.yaml`'s
  `limits: { product: N }` — run 2+ of them together and SUMMARY.md adds a size-series
  comparison table (model-behavior pass % and avg tokens at each size). Extending to 50 or
  200 is one more pool + one more 3-line scenario.yaml, not a new hand-written schema.
- Does a real chat history change the right answer? A test can attach `history:` turns
  (rendered into the prompt's own `{{history}}` slot, empty by default) — see
  `common/shop-questions.yaml`'s conversation-start/close/follow-up/misunderstanding cases.

**The rule that ties it together: promptfoo grades whether the model behaved well. The
harness grades whether the contract — the actual template-and-injection idea — held up.**
A schema idea is not ready to build until both pass.

## Honest limits (read before trusting a green run as more than it is)

- **The harness is a playground twin of the real renderer, not the real renderer.** It
  proves the *design* can work. The product's own Go code that does this today is tested
  separately, in Go:
  [TestPostProcess_PriceRenderFailurePostsManualNote](../backend/internal/brain/prompt_test.go#L160).
  A green run here is evidence a design is worth building — it is not a replacement for
  testing the shipped pipeline once it exists.
- **Every scenario describes a response format the product doesn't have yet.** All of
  them return the unified `media_files_to_send` shape (grouped media); the product today
  still returns `asset_refs`, individual file refs capped at 3 — see
  [openrouter.go](../backend/internal/brain/llm/openrouter.go#L212). That's the whole
  point of a playground — but it means a scenario's pass rate is never migration proof by
  itself, even for the `schema_kb_v1` scenarios that render through `backend/aiprompt`
  directly: that package is not wired into the production backend yet. `schema.sql`
  files are the same kind of thing: documentation of an idea, never executed, never a
  real database.

## One rule this reverses, on purpose

The first version of this eval was "zero program code, only text and config files" — the
right call for comparing 3 fixed prompts. It stopped being the right call once the goal
became *any number of imitated schemas*, because hand-editing prompts to match hand-edited
data is exactly the kind of repetitive, error-prone work code should do instead. The
harness is the one piece of program code in `evals/`, it is Go (matching the rest of this
repo), and it touches nothing outside `evals/`.
