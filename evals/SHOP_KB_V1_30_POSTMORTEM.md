# `shop-kb-v1-30` evaluation: approach and lessons

This is the short reference for why the July 22–23 evaluation scores collapsed,
what the corrected results mean, and what must not be repeated.

## Approach used

- Scenario: 31 Russian shop questions against a WhatsApp reply-drafting assistant.
- Models: seven OpenRouter models in the July 22 run; three low-cost models in the
  July 23 run.
- Prompt: one large Russian prompt per question containing the rules, response schema,
  full 30-product knowledge base, all approved fact placeholders, all media tokens,
  and the customer's message. The latest prompt was about 44 KB and 8.4–8.8K input
  tokens; 98–99% of model tokens were prompt input.
- Knowledge base: approved prose plus volatile facts represented by placeholders such
  as `{{product.coffee-machine.price}}`. Code substitutes their real values only after
  validation. Media uses separate exact tokens such as
  `products.coffee-machine.gallery_images`.
- JSON schema: `reply_text`, `reply_language`, `media_files_to_send`, and `escalate`
  are operational fields. `confidence` and `escalation_reason` were originally treated
  too strictly; they are now optional diagnostics and must not affect the score.
- Evaluation dimensions are separate:
  1. final JSON extraction/parsing;
  2. operational contract validation;
  3. deterministic behavior checks in Go;
  4. optional LLM-as-judge behavior review.
- Raw provider output is retained unchanged in `*.results.json`; `*.judged.json`
  records the derived verdict. Re-judging reads the stored raw output again—it does
  not edit or regenerate the model answer.

## Scores

### July 22 original and corrected re-judge

Runs:

- `runs/2026-07-22_23-37-42-94b6`
- `runs/2026-07-22_23-37-42-94b6-rejudged`

The original report showed only 0–6% behavior for all models. This was mostly an
evaluator defect: the new validator parsed raw output without removing a single outer
Markdown JSON fence, while the old production-compatible path removed it. Therefore
147 of 217 answers failed before their content was evaluated.

After applying the same outer-fence handling and re-judging the original stored
responses:

- all seven models: behavior `112/217` (51.6%), contract `132/217` (60.8%);
- excluding two provider/configuration failures: behavior `112/155` (72.3%);
- best usable model: Gemini 2.5 Flash, behavior `24/31` (77%), contract `29/31` (94%);
- Gemini 3.5 Flash and Kimi K2.6 remained `0/31`: the former exposed reasoning instead
  of a final answer; the latter was truncated with `finish_reason=length`.

### July 23 latest live run

Run: `runs/2026-07-23_15-32-58-6a60`

- final JSON extracted: `93/93` (100%);
- operational contract: `79/93` (84.9%);
- deterministic behavior: `69/93` (74.2%);
- passed both contract and deterministic behavior: `61/93` (65.6%);
- LLM-as-judge: **not run**, so this run has no semantic judge score.

Per model, deterministic behavior was `23/31` (74%) for all three. Contract was
DeepSeek `27/31`, Gemini 2.5 Flash `28/31`, and Gemini 2.5 Flash Lite `24/31`.

## What failed and why

1. **Evaluator/parser asymmetry.** Valid JSON inside one Markdown fence was rejected.
   Parsing used different rules in different paths, creating false failures.
2. **Thinking output was mistaken for the answer.** Some provider configurations put
   reasoning in the answer channel or exhausted the output budget before final JSON.
   Reasoning must be logged separately; only the final answer may be parsed.
3. **The prompt was overloaded.** Every question included all 30 products, facts, and
   media. The model had to perform retrieval, policy decisions, exact token copying,
   media routing, escalation, and JSON formatting in one pass.
4. **Placeholder errors.** Models copied literal KB values, invented placeholder
   names, or placed media identifiers in `reply_text` instead of the media array.
5. **Media errors.** Models selected the wrong group, invented a nonexistent media
   token, attached media when none was allowed, or omitted required media.
6. **Escalation ambiguity.** Models confused “known to be unavailable” with “not
   present in the KB,” and disagreed on return, warranty, delivery, and missing-media
   cases.
7. **Some deterministic failures were evaluator artifacts.** The literal-value
   detector matched common repeated values such as “в наличии” against unrelated
   products. Such collisions can penalize a valid answer and must not be interpreted
   automatically as model failure.
8. **Diagnostics were used as gates.** `confidence` and `escalation_reason` added
   formatting failure modes without protecting the customer-facing operation.
9. **A 0/0 directory was treated as a run.** The harness created the final run
   directory before work completed, and run discovery scanned every directory,
   including interrupted attempts and the non-run `runs/catalog` asset directory.

## Rules for the next design

- Retrieve only the relevant product/topic facts and media before calling the model;
  do not send the full catalog for each message.
- Keep all exact values and media resolution in code where possible. Give the model
  short approved identifiers, then map them to storage tokens after validation.
- Use one shared final-answer extractor in production and evaluation. Preserve raw
  response and reasoning separately and never rewrite either during judging.
- Make escalation policy a small explicit decision table with examples for known
  absence, unknown fact, unsupported request, and off-catalog product.
- Score only operational fields. Log `confidence` and `escalation_reason`.
- Freeze evaluator fixtures before comparing models, add negative-control tests, and
  manually review a sample of deterministic failures for false positives.
- Report parsing, contract, deterministic behavior, and LLM judge separately; never
  average them into one unexplained score.
- Publish a run only after it has finished and contains evaluated executions.
  Interrupted work belongs under `runs/.incomplete/<run-id>` so raw responses remain
  available for debugging without producing a 0/0 result.
