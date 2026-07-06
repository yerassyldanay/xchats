# Exam results — old schema vs. new schema (DECISIONS.md)

## What this measures — and what it does NOT prove

This exam answers one narrow question: **can a model be steered, by prompt wording alone,
into using the right fact tokens and media groups?** That's it. Read the numbers below with
these limits in mind — a code review of this exam caught all of them:

- **This is not proof the new schema or database is safe.** `schemas/decisions-v1.sql` is
  documentation only — nothing executes it, nothing enforces it. No real database has ever
  rejected a bad value here. See that file for what enforcement (CHECK constraints, etc.)
  *would* need to look like once this becomes a real migration.
- **The new prompts test a contract the product does not have yet.** They ask the model to
  return `attach_groups` (whole media groups). The real product today returns and expects
  `asset_refs` (individual file refs) — see [openrouter.go](../../backend/internal/brain/llm/openrouter.go#L212)
  and [draft.go](../../backend/internal/brain/domain/draft.go#L8). Grouped media is a
  proposal, not shipped code.
- **This does not test the fail-closed rendering pipeline.** The real safety property — an
  unresolved or malformed `{{...}}` token blocks the whole draft rather than shipping a
  half-rendered price — is real and already tested, but in Go, not here:
  [TestPostProcess_PriceRenderFailurePostsManualNote](../../backend/internal/brain/prompt_test.go#L86).
  This exam only ever looks at the model's raw JSON, before that step would run.

So: treat every number below as "how promptable is this design," not "this design is
proven safe to ship." That distinction is the whole point of writing it down here.

## The run

Eval ID: `eval-qhv-2026-07-06T11:18:30` — open the live table at http://localhost:15500
(run `npx promptfoo view` from `evals/` if it's not already running), or open
`results/results.html` directly as a plain file.

**81 checks total (15 questions x 3 models; xpayment questions run on 1 prompt only): 66 passed, 15 failed, 0 errors.**

## Score by schema

| prompt | passed | rate |
|---|---|---|
| shop, OLD schema (today's product) | 27 / 36 | 75% |
| shop, NEW schema (DECISIONS.md proposal) | 32 / 36 | 89% |
| xpayment, NEW schema | 7 / 9 | 78% |

The new-schema prompt answers more of these specific questions correctly than the
old-schema prompt, on the exact same questions and models. That's a real signal the new
prompt structure is easier for a model to follow — not proof the underlying schema change
is safe to build (see the limits above).

## Score by model

| model | passed |
|---|---|
| openai/gpt-4o-mini | 24 / 27 |
| anthropic/claude-haiku-4.5 | 21 / 27 |
| google/gemini-2.5-flash | 21 / 27 |

No model stood out as clearly better or worse overall.

## What actually failed (all 15, genuine — not tool bugs)

- **Kazakh questions answered in Russian** — the single biggest pattern. Happened on
  the price question and the payment-limit question, on both schemas and multiple
  models. One bright spot: the delivery question ("cost + time, Kazakh") was answered
  in Russian 3/3 times on the OLD schema, but correctly in Kazakh 3/3 times on the
  NEW schema — a real, visible difference between the two prompts.
- **Refund requests not escalated** — the NEW schema's shop prompt escalated a refund
  request only 1/3 times, worse than the OLD schema's 3/3. This is a real regression
  to look at, not a universal win — the new prompt's wording may need a firmer rule
  about money-back requests.
- **Off-KB questions answered instead of escalated** — 3 cases across both schemas
  where the model tried to help with delivery-to-Astana or crypto-payment questions
  that are not in the knowledge base, instead of saying "let me check and get back
  to you."

## Checks tightened after a code review (2026-07-06)

A review found four of the checks were looser than they looked, which could make the
pass rate above look better than it should:
- Test 3 (delivery, Kazakh) accepted *any* `policy.main.*` token, not specifically the
  delivery-cost and delivery-time/duration tokens the question actually asks for.
- Test 4 (stock count) accepted *any* `product.coffee-machine.*` token — including the
  price token, which doesn't answer "how many in stock."
- Tests 6 & 7 (media) asked an LLM judge's opinion instead of reading the actual
  `attach_groups` / `asset_refs` field in the model's JSON.

All four now check the exact token or field expected, parsed from the real JSON. On this
run the pass/fail outcome for every single case was verified unchanged after tightening —
not because the checks don't matter, but because on these particular questions the models
never happened to exploit the old checks' looseness (confirmed by reading the raw model
output for each one, not assumed). The tighter checks are still the right ones to keep:
they will catch a wrong-token or wrong-media case if one occurs on a future run.

Also fixed: the xpayment prompt had one exact fact ("подключение занимает один рабочий
день") sitting in prose instead of a typed FACTS token — inconsistent with our own rule
that no exact/committed number belongs in prose. Moved to
`{{policy.main.connection_days}}`.

## Bugs found and fixed earlier (all in the exam's own grading code, not in xchats)

1. A required field was missing from passing checks, which made most of them
   register as broken instead of passed.
2. A rubric instruction used `{{...}}` as shorthand for "a placeholder token" — the
   grading tool tried to read that as its own syntax and choked.
3. Several checks that look for an exact placeholder (e.g. the price token) were
   silently reading a scrambled version of themselves instead of the real text —
   some always failed, some always trivially passed. Rewritten so this can't happen.
4. One check meant to catch invented numbers was reading the model's entire raw
   answer (including harmless numbers like a 0.95 confidence score) instead of just
   the customer-facing reply text, when that reply arrived wrapped in a code block.

All four were caught by noticing results that were suspiciously perfect or
suspiciously broken, then checking the actual model answer by hand — not assumed.
Every number above is from the final, corrected, tightened run.
