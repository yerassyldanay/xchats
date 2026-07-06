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
│   └── xpayment-decisions-v1/
└── runs/
    ├── INDEX.md                       ← one line per run: what, when, headline numbers
    └── 2026-07-06_shop-compare/       ← one folder per run
        ├── SUMMARY.md                 ← the human report — read this first
        ├── CONTRACT.md                ← token/injection/media verdicts per answer
        └── results.json               ← promptfoo's raw data (not committed)
```

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

Five small Go programs (no dependency on the product's `backend/` code — the harness is
free-standing on purpose, since it exists to try out ideas the product hasn't built yet):

1. **`render`** — reads `data.yaml`, fills `frame.txt`'s slots with the generated
   KNOWLEDGE BASE / MEDIA / FACTS text, writes `generated/prompt.txt` +
   `generated/catalog.json` (every valid token, every valid media group) +
   `generated/promptfooconfig.yaml` (wires in `models.yaml` + the resolved
   `tests.yaml`). This is the only place scenario content becomes a prompt — nothing is
   ever hand-typed into a prompt file again.
2. **promptfoo runs** against the generated config, exactly as before — this part doesn't
   change, promptfoo is still the right tool for "call N models with M questions."
3. **`judge`** — reads promptfoo's raw answers and, per answer: parses the JSON (tolerant
   of a ```` ```json ```` wrapper) → checks every `{{token}}` used exists in the
   catalog (an unknown or malformed one gets the verdict **"BLOCKED — fail-closed"**,
   exactly what the real product would do) → substitutes the catalog's values into the
   text → checks the **substituted** text for leftover `{{`, invented digits, and correct
   media groups/refs. This is the part that didn't exist before and is the actual reason
   this playground exists — it tests the injection idea, not just "did the model try."
4. **`report`** — turns the judge's verdicts + promptfoo's own pass/fail into one
   `SUMMARY.md` per run: a stats table per scenario × model (model-behavior pass %,
   contract pass %, cost, latency), the answers that failed quoted in full, and one line
   appended to `runs/INDEX.md` so old runs stay easy to find and compare.
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
- Does the answer still hold up as the row count grows (10 products vs. 200)?

**The rule that ties it together: promptfoo grades whether the model behaved well. The
harness grades whether the contract — the actual template-and-injection idea — held up.**
A schema idea is not ready to build until both pass.

## Honest limits (read before trusting a green run as more than it is)

- **The harness is a playground twin of the real renderer, not the real renderer.** It
  proves the *design* can work. The product's own Go code that does this today is tested
  separately, in Go:
  [TestPostProcess_PriceRenderFailurePostsManualNote](../backend/internal/brain/prompt_test.go#L86).
  A green run here is evidence a design is worth building — it is not a replacement for
  testing the shipped pipeline once it exists.
- **A scenario can describe a response format the product doesn't have yet** (e.g.
  `attach_groups` — grouped media — is a proposal; the product today returns
  `asset_refs`, individual file refs; see
  [openrouter.go](../backend/internal/brain/llm/openrouter.go#L212)). That's the whole
  point of a playground — but it means a scenario's pass rate is never migration proof by
  itself. Say in `scenario.yaml` which real contract (if any) a scenario matches.
  `schema.sql` files are the same kind of thing: documentation of an idea, never executed,
  never a real database.

## One rule this reverses, on purpose

The first version of this eval was "zero program code, only text and config files" — the
right call for comparing 3 fixed prompts. It stopped being the right call once the goal
became *any number of imitated schemas*, because hand-editing prompts to match hand-edited
data is exactly the kind of repetitive, error-prone work code should do instead. The
harness is the one piece of program code in `evals/`, it is Go (matching the rest of this
repo), and it touches nothing outside `evals/`.
