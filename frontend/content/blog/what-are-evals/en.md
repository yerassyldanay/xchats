---
title: "What evals are, and why we build them"
description: "How we test an AI assistant's answers before any code changes: what an eval actually is, what it's made of, and one lesson that cost us a full day of debugging."
date: "2026-07-27"
cover: cover.png
category: evals
tags: ["evals", "testing", "engineering"]
---

An "eval" isn't a single check of "did the model answer correctly." It's a system:
a set of questions, an expected behavior for each one, and code that runs the
questions through a model and checks the answers against those expectations —
automatically, repeatedly, without a human reviewing every single run. We call this
place a "playground": it lets us imitate a different version of the product —
different database columns, different prompt wording, a different number of
products in the catalog — and see how a real model answers real questions, before
writing a single line of migration code.

## Why this was necessary

The first version of our evals was as simple as it gets: three hand-written prompt
files and one shared question list. That was enough to answer "does the model
behave reasonably" — but a code review found two reasons it wasn't enough.

**Hand-written prompts drift from the data they're built on.** Change one number in
a test product's price, and nothing forces the prompt to update to match. We hit
this exact class of bug three times in one afternoon: a check would quietly start
testing the wrong thing, and nobody noticed until the numbers looked "too clean" or
"too broken" to be real.

**The old exam only ever looked at the model's raw answer.** It never tested the
actual idea the whole thing exists for: the model names a placeholder token, code
validates it and substitutes in the real value, and an unknown or broken token is
supposed to block the entire reply — not ship something made up. That's the core
mechanism. A grading tool that never runs that mechanism can't tell us whether it
works.

The playground fixes both problems: scenarios are built from one data file, never
hand-duplicated, and a small harness actually runs the validate → inject → check
chain, instead of just asking "did the model say something plausible?"

## What one scenario is made of

Every imitated product version is one folder, and it always contains three things:

1. **Tables and columns** (documentation, never executed) plus **the actual rows**
   — real test data, including edge cases on purpose: a product with zero stock, a
   topic with no media, a city the knowledge base has never heard of.
2. **The placeholder and media-group catalog**, generated FROM those rows — never
   hand-typed, so it physically cannot drift from the data.
3. **Customer messages and the expected behavior** — usually the shared question
   bank plus a handful of scenario-specific cases.

## What the harness actually runs

The pipeline is four steps, always in the same order.

**Render** reads the data, fills the prompt template's slots, and writes the final
`prompt.txt` plus a full catalog of valid tokens. This is the only place scenario
content ever becomes a prompt — nothing is ever hand-typed into a prompt file
again. This step deliberately fails (writes no prompt) if a fact's value contains a
brace character, if the finished prompt references a token this exact render's
catalog doesn't resolve, or if a slot was left unfilled.

**Promptfoo** runs the generated config against the real models — this part
doesn't change, promptfoo is still the right tool for "call N models with M
questions."

**Judge** is the heart of the system. It parses the model's JSON reply (tolerant of
a code-fence wrapper), checks every token the model used against the catalog (an
unknown or malformed one gets the verdict "BLOCKED" — exactly what the real
product would do), substitutes the real values in place of the tokens, and only
THEN, in that substituted text, looks for anything left over: a stray brace, digits
the model invented on its own, the wrong media file attached. This is the part
promptfoo alone never had — and the whole reason this playground exists: it tests
the substitution idea itself, not just "did the model try."

**Report** turns the verdicts into one human-readable summary: a table of scenario
× model, an honestly-labeled cost estimate, latency, token counts.

## Contract and behavior — two separate scores, never averaged

For every answer we compute more than one score, kept deliberately separate:

- **Contract pass** — would the product's actual pipeline accept this reply at all
  (valid JSON, every required field present, not a single unknown token)?
- **Model-behavior pass** — did the reply behave honestly (no invented digit, no
  mixed-up media group, no language bleeding through after substitution)?
- Separately, optionally — an LLM-judge score for more semantic questions, like
  "does this reply actually match what's on the shelf."

These are different questions, and folding them into one averaged percentage would
throw away exactly the information the whole exercise exists to produce.

## The lesson that cost us a day of debugging

On July 22, one run across seven models reported a disaster: 0–6% correct answers,
across every single model at once. The first instinct was "the models broke."
The real cause was much more boring, and much more important: our own validator
parsed the model's raw reply differently than the product's own, already-proven
code did. The product's path strips an outer code-fence wrapper before parsing
JSON; the new eval validator didn't. Because of that, 147 of 217 answers failed
before their content was ever actually checked.

After applying the same fence-stripping logic and re-judging the exact same,
already-stored raw responses — without re-running the models, which is the whole
point of keeping the raw response stored separately from the verdict — the real
numbers were 112 of 217 (51.6%) on behavior, 132 of 217 (60.8%) on contract. The
best model scored 24 of 31 (77%).

We keep this incident in the documentation on purpose, not hidden as an
embarrassment: **before blaming the model for a bad score, check your own grading
tool first.** The bug wasn't in what we tested. It was in what we tested it with.

## What gets checked on every run

- Did the model pick the *correct* placeholder, not just *a* placeholder?
- Did the model ever write a value out literally, bypassing the token entirely?
- Does an unknown or broken token get blocked, the same way the real product would
  block it?
- Does the reply stay clean after substitution — no words from the other language
  leaking in through an injected value?
- Did the model pick the right media group, and refuse to attach something that
  doesn't exist?
- Does answer quality hold up as the catalog grows? The same question bank runs
  against catalogs of different sizes, and the report adds a table showing exactly
  how the score moves with size.

## Honestly, the limits

The harness is a playground twin of the product's real renderer, not the renderer
itself. A green run here means "this idea is worth building," not "the product's
code works" — the product's own code is tested separately, in Go. And, as our
other post on choosing a model admits just as plainly: an eval existing and passing
doesn't by itself mean the winning version has reached a real customer yet.
