# Which model should we use?

Last reviewed: 2026-07-27. See "When to re-open this decision" below — this answer expires.

## The pick

**Use `google/gemini-3.5-flash`.** Best answers we have measured: **92–97% correct**,
depending on how many products are in the catalog. Fast: about 2 seconds per reply.
Not cheap: **$9–16 per 1,000 answers**, roughly 5x the next option. We accept that
price for the quality.

Cheaper fallback if volume grows too large: `google/gemini-2.5-flash` — $1.80–3.10
per 1,000 answers, 1.2s, but only 82–89% correct. You give up roughly 9 correct
answers in every 100.

## ⚠️ Read this before acting on the pick above

1. **The live model is not the recommended one.** Production's `LLM_FAST_MODEL`
   still defaults to `openai/gpt-4o-mini` ([config.go:95](../backend/internal/config/config.go#L95)),
   which scored **55%** in our tests — second-worst of everything we measured.
   Switching it means changing both the model id *and* its reasoning settings
   (see "Approach beats model choice" below) — not just editing one config line.
2. **The prompt behind these numbers is not live either.** These scores come from
   prompt v3, which lives in `backend/aiprompt`, used only by `evals/harness`.
   Production drafts replies through `internal/brain`
   ([real.go:82-84](../backend/internal/assistant/real.go#L82-L84)), a separate,
   older English-language prompt. The +12-point prompt gain described below does
   not reach customers today.

This document records what we *should* run. Neither fact above is fixed by writing
it down — someone still has to ship the change.

## Table A — models we are choosing between

Source: [2026-07-26_22-10-50-9064](runs/2026-07-26_22-10-50-9064/) and
[2026-07-26_22-30-10-bd13](runs/2026-07-26_22-30-10-bd13/), both on prompt v3.

**Same footing — all three ran the same 76 questions (10- and 50-product catalogs):**

| model | correct | typical speed | worst seen | per 1,000 answers |
|---|---|---|---|---|
| gemini-3.5-flash | **95%** (72/76) | 2.0s | 2.6s | $9.48 – $12.12 |
| deepseek-v3.2-exp | 91% (69/76) | 11s | **68s** | $1.63 – $2.21 |
| gemini-2.5-flash | 89% (68/76) | 1.2s | 2.3s | $1.84 – $2.39 |

**At the 100-product catalog, only the two Gemini models were run** — deepseek was
left out of that launch, so it has no 100-product score, and its 91% above should
not be compared directly to a 114-test Gemini number:

| model | correct at 100 products | per 1,000 answers |
|---|---|---|
| gemini-3.5-flash | **97%** (37/38) | $15.82 |
| gemini-2.5-flash | 82% (31/38) | $3.14 |

**On the size gap:** 3.5-flash led at every catalog size we tried — by 8, 7, and 15
points. But all these tests reuse the same **38 questions**, just asked again at
each catalog size, so 114 rows is not 114 independent tries. Read the lead as
consistent, not as a precise measurement. The honest summary: 3.5-flash wins, and
its margin grows as the catalog grows.

**Why the cost range moves so much:** a bigger catalog means a longer prompt.
Tokens per answer go 5.5k → 7.4k → 9.9k at 10 → 50 → 100 products, and the prompt
is about 99% of the bill. Cost tracks catalog size almost as strongly as it
tracks which model you pick.

**Deepseek is a trap, not a bargain.** Cheapest per answer, and quality not far
off 3.5-flash — but 11 seconds typical and 68 seconds worst case. A customer will
not wait a minute for a reply. Deepseek alone made run 9064 take 3m38s instead of
about 44s — 80% of all request time in that run.

## Table B — models we tested and rejected

Not candidates today — kept here so we don't re-test them by accident. These
numbers are from older runs the latest launches didn't repeat:

| model | correct | why rejected | measured in |
|---|---|---|---|
| gemini-2.5-flash-lite | 66% | too many mistakes | [b325](runs/2026-07-24_04-24-34-b325/) |
| gpt-4o-mini | 55% | too many mistakes — **and this is what production runs today** | [b325](runs/2026-07-24_04-24-34-b325/) |
| claude-haiku-4.5 | 74% | slow (4.7s) and expensive | [diagfix](runs/2026-07-23_10-23-53-diagfix/) |
| minimax-m2.5 | 68% | slow (8.6s) | [diagfix](runs/2026-07-23_10-23-53-diagfix/) |
| kimi-k2.5 | 77% | too expensive for the result; archived 2026-07-23 | [diagfix](runs/2026-07-23_10-23-53-diagfix/) |
| kimi-k2.6 | 52% | every reply cut off mid-thought; 60s | [diagfix](runs/2026-07-23_10-23-53-diagfix/) |

The bottom four ran on an **older prompt (v0)**, so their scores are not directly
comparable to Table A. They were rejected on speed, price, or broken output — not
on a narrow quality margin.

## Approach beats model choice

Three times, changing *how we ask* moved the numbers more than changing *who we ask*:

- **Prompt v2 → v3, same models, same questions**
  ([31c0](runs/2026-07-24_02-07-24-31c0/) → [0ab4](runs/2026-07-24_04-32-15-0ab4/)):
  deepseek 80% → 87%, gemini-2.5-flash 67% → **79%**. Twelve points, zero extra cost.
- **One settings line saved 3.5-flash from being archived.** On default settings
  it scored 65% at 9.2 seconds ([diagfix](runs/2026-07-23_10-23-53-diagfix/)) and
  we nearly dropped it. Setting `reasoning: effort: minimal, exclude: true`
  ([models.yaml:84-87](models.yaml#L84-L87)) took it to 95%+ at 2.0 seconds.
  **Our pick only works with that setting — shipping 3.5-flash without it ships
  the 65% version.**
- **A bigger catalog hurts the cheap model more than the expensive one.**
  gemini-2.5-flash goes 89% → 89% → 82% at 10/50/100 products; gemini-3.5-flash
  holds 97% → 92% → 97%.

One honesty note, not a model or prompt finding: run
[94b6](runs/2026-07-22_23-37-42-94b6/) first reported 3 correct out of 217. Same
replies, after fixing a bug in our own scoring code: 112, then 148. Check the test
harness before blaming the model for a bad score.

## What to trust less

- Costs are **estimates**, from prices we typed into [models.yaml](models.yaml) by
  hand, last checked 2026-07-11 (3.5-flash and deepseek: 2026-07-18). **Re-check
  the 3.5-flash price before relying on this doc for a budget decision** — it is
  now our pick and the expensive one.
- Some reruns replay a cached answer instead of asking the model again: $0 and
  12 milliseconds, not real. Run bd13 was 152 of 228 answers cached.
- Each catalog size is only 38 questions. Treat a gap smaller than about 8 points
  as a tie, not a real difference.
- Kazakh test messages are still a draft, pending review by a native speaker.

## When to re-open this decision

- a listed price moves more than 20%, or OpenRouter changes 3.5-flash's price;
- a new model family ships (Gemini 4.x, DeepSeek v4, GPT-5-mini class, etc.);
- the catalog grows past 100 products — the largest size we've tested, and where
  the cheap model already starts to struggle;
- prompt v3 is replaced — every number in this doc is tied to that exact prompt;
- 3.5-flash's reasoning setting stops working, or the API changes its defaults.

## Not covered here

- **How we test:** see [README.md](README.md) and [VOCABULARY.md](VOCABULARY.md).
- **Reading photos (image → JSON extraction):** a different job, tested once
  ([2026-07-14_22-54-51-c53b](runs/2026-07-14_22-54-51-c53b/), 10 cases per model —
  gemini-2.5-flash did best, 7/10). Too old and too small a sample to conclude
  anything from. Needs a fresh run before it belongs in this doc.
