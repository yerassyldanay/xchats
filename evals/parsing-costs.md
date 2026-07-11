# Parsing costs & model picks (benchmark note)

Companion to `DECISIONS.md` → "Per input type". That file keeps only *relative*
cost drivers; the concrete prices and model choices live here, following the
`models.yaml` discipline: hand-checked and dated — **re-verify before any
implementation or budget decision**. Text rates are comparatively stable, but
**modality pricing (image / audio / video) varies by provider routing and
drifts faster** — check both the aggregator and the upstream source:
https://openrouter.ai/google/gemini-2.5-flash and
https://ai.google.dev/gemini-api/docs/pricing.

Prices checked: **2026-07-10** (OpenRouter API snapshot).

## Provider integration

**v1 integrates OpenRouter only** (`llm_provider: openrouter`, OpenAI-compatible
wire format — already what the backend speaks). No direct Google / Anthropic /
OpenAI SDK integration: every model below is just a **config-swappable id
behind the one OpenRouter seam**, so adding or switching models is a config
change, never an integration. The Google pricing link above is a
price-verification source only, not an integration target.

## Model picks (starting defaults, all via OpenRouter)

- **Pass 1 (per-file extraction)**: `llm_vision_model = google/gemini-2.5-flash`
  — one model covers images, native PDF, and audio now, video later, behind the
  same extractor seam. `google/gemini-2.5-flash-lite` is ~3× cheaper and takes
  the same inputs — worth an eval before switching; pick by extraction quality
  (Cyrillic price cards, OCR), not price.
- **Pass 2 (batch synthesis)**: the `llm_fast_model` tier; the winner among
  `openai/gpt-4o-mini` / `anthropic/claude-haiku-4.5` / `google/gemini-2.5-flash`
  is picked by the evals in this directory, not by opinion. Note:
  `gpt-4o-mini` has **no audio input** — it cannot serve as the single pass-1 model.
- **The brain (reply drafting)**: whichever model wins evals must support
  **prompt caching** through OpenRouter — DECISIONS §1's cost story depends on it.
  Structure prompts as [shared prefix][variable tail]; cached reads bill at
  roughly a tenth of the normal input rate.

## Unit prices (per 1M tokens, USD — snapshot)

| Model | input | output | audio input | cached read |
|---|---|---|---|---|
| google/gemini-2.5-flash | 0.30 | 2.50 | 1.00 | 0.03 |
| google/gemini-2.5-flash-lite | 0.10 | 0.40 | 0.30 | 0.01 |
| openai/gpt-4o-mini | 0.15 | 0.60 | — | 0.075 |

Media → token mechanics (Gemini): image ≈ 260–1,000 tokens (downscale first);
PDF page ≈ ~260; audio ≈ 32 tok/sec; video ≈ ~260 tok/sec. Each pass-1 call
adds ~1K context tokens (instruction + KB index) and ~200 output tokens.

## Per-file estimates (gemini-2.5-flash)

- text / url / excel / csv / docx: **$0** — deterministic code paths, no model call
- image: **~$0.001**
- 10-page **native** PDF (text layer): **~$0.002**
- 10-page **scanned** PDF (vision-OCR per page): **~$0.003–0.005**
- 3-min voice note: **~$0.007**
- video, **v1 scope** (audio-track transcript only): **~$0.005 per 2 min**
- video, **future** (sampled keyframes + full visual parse): **~$0.01+ per 2 min**
  — the only genuinely costly type, and NOT in v1 scope
- pass 2 (synthesis turn, ~8K in / 1K out): **~$0.005**

A realistic batch — 10 photos + a catalog PDF + a voice note + synthesis —
lands around **$0.025**. Conclusion recorded in DECISIONS: optimize extraction
for quality, not pennies; only video justifies cost-driven design choices.
