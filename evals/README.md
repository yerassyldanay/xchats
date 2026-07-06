# Eval Playground

Read `PLAYGROUND.md` first — it explains what this is and why it's shaped this way. This
file is just the commands.

## One-time setup

```bash
export OPENROUTER_API_KEY=sk-or-...   # same value as LLM_API_KEY in the repo's .env
node --version                        # needs >=20.20 or >=22.22 (nvm handles this — see repo root .bashrc)
cd evals/harness && go build -o harness . && cd ..
```

## Run a scenario (or all of them)

```bash
cd evals
./harness/harness run -scenario scenarios/shop-current
./harness/harness run -scenario scenarios/shop-current,scenarios/shop-decisions-v1
./harness/harness run -all
```

Each run: renders every named scenario's prompt from its `data.yaml`, calls the models in
`models.yaml` via promptfoo, grades every answer (fail-closed token check, injection, media,
escalation, language), and writes `runs/<timestamp>/SUMMARY.md` — read that first. Real
model calls cost real money; unchanged answers are cached by promptfoo on repeat runs, so
tweaking one scenario and re-running only pays for what changed. Add `-no-cache` to force
everything fresh.

## Try an idea without spending anything

```bash
./harness/harness render -scenario scenarios/shop-current
```

Writes `scenarios/shop-current/generated/{prompt.txt,catalog.json,promptfooconfig.yaml}` —
inspect the actual prompt your data produces, or run `npx promptfoo@latest validate -c
scenarios/shop-current/generated/promptfooconfig.yaml` to check it's well-formed. No API
calls, no cost.

## Make a new scenario

Copy the closest existing scenario folder, then edit:
- `data.yaml` — the rows. This is the only place a fact value or a media file is ever
  named; the prompt and the grading catalog are both generated from it, so they can never
  disagree with each other.
- `frame.txt` — the rules/persona wording, with `%%KNOWLEDGE_BASE%%` / `%%MEDIA%%` /
  `%%FACTS%%` / `%%DESCRIPTIONS%%` / `%%MEDIA_FIELD%%` slots the renderer fills in.
- `scenario.yaml` — points at the two files above plus `tests.yaml`, and says which
  response contract (`asset_refs` or `attach_groups`) this version's frame expects back.
- `tests.yaml` — usually just `include: [common/shop-questions.yaml]`; add scenario-only
  questions under `tests:` if this version needs one a shared bank doesn't have.

Then `render` it (free) before you `run` it (costs money).

## Reading a run's output

- **`SUMMARY.md`** — per scenario × model: model-behavior pass rate (did the model do the
  right thing), contract pass rate (did token resolution/injection/media validation hold
  up), cost, latency, tokens. Failing answers are quoted in full underneath.
- **`CONTRACT.md`** — one block per answer: parsed or not, which tokens resolved, whether
  the draft would be BLOCKED (an unknown/malformed token — the real product's fail-closed
  behavior), the actual injected customer-facing text, and every check's pass/fail.
- **`runs/INDEX.md`** — one line per run, so past attempts stay easy to find and compare.

## Known limits

- **Cost shows as `n/a`, not $0.** promptfoo has no pricing table for generic
  `openrouter:` provider IDs, so it always reports $0 — that means "unmeasured", not
  "free". Tokens are real; for real spend, check OpenRouter's own dashboard.
- **The harness is a playground twin of the real renderer, not the real renderer.** It
  proves the *design* can work; the product's own Go code that does this today is tested
  separately: [TestPostProcess_PriceRenderFailurePostsManualNote](../backend/internal/brain/prompt_test.go#L86).
- **A scenario's response contract may not be the shipping one.** `attach_groups`
  (grouped media) is a DECISIONS.md proposal; the product today returns `asset_refs`
  (individual file refs) — see [openrouter.go](../backend/internal/brain/llm/openrouter.go#L212).
  Check `scenario.yaml`'s `contract:` field before treating a pass rate as migration proof.
- **`schema.sql` files are documentation, not a database.** Nothing runs or enforces them.
- **No LLM-judge layer.** Every check in `judge.go` is deterministic (token/media/escalate/
  a Kazakh-letter heuristic). A few real questions (e.g. "does this read as a natural
  next step") have no automated check — read the injected text in `CONTRACT.md` by eye.

## The one earlier eval this replaced

Before the playground, `evals/` was 3 hand-written prompt files (`schemas/current.sql` +
`schemas/decisions-v1.sql` + a flat `promptfooconfig.yaml`). That comparison's numbers are
preserved in `results/REPORT.md` as a historical record — it is not re-run or updated by
anything here.
