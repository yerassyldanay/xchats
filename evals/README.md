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
./harness/harness run -scenario scenarios/shop-scale-10,scenarios/shop-scale-20,scenarios/shop-scale-30
./harness/harness run -all
```

Each run: renders every named scenario's prompt from its `data.yaml`, calls the models in
`models.yaml` via promptfoo, grades every answer (fail-closed token check, injection, media,
escalation, language), and writes `runs/<timestamp>/SUMMARY.md` — read that first. Real
model calls cost real money; unchanged answers are cached by promptfoo on repeat runs, so
tweaking one scenario and re-running only pays for what changed. Add `-no-cache` to force
everything fresh.

Before spending anything, `run` prints the resolved `(tests x models) = calls` count. Narrow
which models run with `-models google/gemini-2.5-flash,openai/gpt-4o-mini` (default: every
provider in `models.yaml`; `-models-file` overrides the models.yaml path itself). Pass
`-expect-calls N` to hard-fail before any call if the resolved count doesn't match N — a
deliberate confirmation gate for a run you want to cost-check first:

```bash
./harness/harness run -scenario scenarios/shop-current -models google/gemini-2.5-flash -expect-calls 19
```

Running two or more `shop-scale-N` scenarios together adds a "Scale comparison" table to
`SUMMARY.md` — model-behavior pass % and avg tokens per answer at each catalog size, so
"does quality hold up as the product list grows" is answerable from one place.

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
  Optionally caps a fact table's row count with `limits: { <table>: N }` (see
  `scenarios/shop-scale-10/scenario.yaml`) — lets several scenarios share ONE larger
  `data.yaml` while each imitating a different catalog size.
- `tests.yaml` — usually just `include: [common/shop-questions.yaml]`; add scenario-only
  questions under `tests:` if this version needs one a shared bank doesn't have. A test can
  set `history: [{role: client, text: ...}, {role: assistant, text: ...}]` to simulate a
  multi-turn conversation instead of a single fresh message — see
  `common/shop-questions.yaml`'s "16. follow-up with history" for the pattern.

Then `render` it (free) before you `run` it (costs money). `render` also now FAILS if the
rendered prompt references a `{{token}}` not in this exact render's catalog, or if a
`%%SLOT%%` was left unfilled — the same guarantee the injection check applies to model
answers, applied to the prompt itself before any model ever sees it.

## Reading a run's output

- **`SUMMARY.md`** — per scenario × model: model-behavior pass rate (did the model do the
  right thing), contract pass rate (did token resolution/injection/media validation hold
  up), an **estimated** cost (see below), latency, tokens, and prompt-vs-completion token
  share. Failing answers are quoted in full underneath. Running 2+ `shop-scale-N`
  scenarios together also adds a scale-comparison table.
- **`CONTRACT.md`** — one block per answer: parsed or not, which tokens resolved, whether
  the draft would be BLOCKED (an unknown/malformed token — the real product's fail-closed
  behavior), whether injection came out brace-clean, the actual injected customer-facing
  text, cost basis for that one answer, and every check's pass/fail.
- **`runs/INDEX.md`** — one line per run, so past attempts stay easy to find and compare.
- **`index.html`** — one self-contained page per run covering BOTH families: for scenario
  runs, the same model × pass-rate table as SUMMARY.md plus a collapsible per-verdict
  detail (scores, injected text, evidence); for extraction runs, each case's captured
  input image next to every model/prompt variant's parsed fields, checks, and raw
  output. Written automatically at the end of `run`, `extract`, and `report` (best-effort
  — a broken viewer never fails the underlying eval); regenerate by hand with `harness
  html -run <dir>`. Gitignored (regenerate rather than diff in review). Works on runs
  from before this existed too, degrading gracefully — no manifest section, and a
  captured-input image shows as "input not captured" rather than a reconstructed guess.

## Extraction eval (Eval 1: file -> extracted information)

Separate from the scenarios above — this tests the playground's pass-1 extraction step
in isolation, with real files and real OpenRouter vision calls (no promptfoo).

```bash
cd evals/harness && go build -o harness . && cd ..
cp .env.example .env        # fill in OPENROUTER_API_KEY
./harness/harness extract -all
./harness/harness extract -case screenshot -models google/gemini-2.5-flash
./harness/harness extract -record -case infographic -models google/gemini-2.5-flash
```

Each case in `extract/cases.yaml` names one real file under `assets/` and the exact
requirements a correct extraction must satisfy (written by looking directly at the file —
ground truth, not guesses). The model must answer with one fixed JSON shape (see
`extract_types.go`'s `ExtractionResult`) — every check is deterministic string/number
matching, same philosophy as `judge.go`. Output is `runs/<timestamp>/EXTRACT.md` plus the
raw per-(case,model,prompt) JSON under `runs/<timestamp>/extract_outputs/`. `-record`
freezes a fully-passing output to `extract/fixtures/<case>.json` (plus a
`<case>.provenance.json` sidecar naming the model/prompt/run that produced it), meant to
feed a later, separate eval (extracted information -> `ai_*` draft schema) without
re-calling vision models.

The prompt under test lives in `prompts/extract/v1.txt`, not in Go — pass `-prompt
extract@v1` (the default) or a comma-separated list (e.g. `-prompt
extract@v1,extract@v2`) to compare prompt versions in one run; cut a new
`prompts/extract/v2.txt` rather than editing `v1.txt` in place, since existing runs'
results are tied to `v1`'s exact hash.

Cost: a few tenths of a cent per case per model (see `parsing-costs.md`).

## Known limits

- **Cost is an ESTIMATE, never real spend.** `models.yaml` hand-maintains
  `input_per_mtok`/`output_per_mtok` per model (with `pricing_source` + `pricing_checked_at`
  so a report can say when a price was last verified) — promptfoo itself has no pricing
  table for generic `openrouter:` provider IDs. A model with no price entry reports
  **"unknown pricing"**, never a guessed number. A cached row (promptfoo replays a prior
  answer, common on repeat runs) reports zero prompt/completion tokens from the API, so its
  cost is estimated by **borrowing** the token split from a fresh row in the same run for
  the same (model, test) if one exists — otherwise it's **"unpriceable"**, not free. Every
  cost cell says which of these applied; always cross-check real spend on OpenRouter's own
  dashboard.
- **Latency under ~50ms is a cache artifact, not real API latency** — SUMMARY.md flags this
  inline rather than presenting it as a real number.
- **The harness is a playground twin of the real renderer, not the real renderer.** It
  proves the *design* can work; the product's own Go code that does this today is tested
  separately: [TestPostProcess_PriceRenderFailurePostsManualNote](../backend/internal/brain/prompt_test.go#L86).
- **A scenario's response contract may not be the shipping one.** `attach_groups`
  (grouped media) is a DECISIONS.md proposal; the product today returns `asset_refs`
  (individual file refs) — see [openrouter.go](../backend/internal/brain/llm/openrouter.go#L212).
  Check `scenario.yaml`'s `contract:` field before treating a pass rate as migration proof.
- **`schema.sql` files are documentation, not a database.** Nothing runs or enforces them.
- **Media is validated by NAME, not by file existence.** `data.yaml`'s media filenames
  (e.g. `cm-1.jpg`) are fictional — judge.go checks a model attached a group/ref that
  EXISTS IN THE CATALOG, not that the file itself is real. That's the right level for a
  text-only prompt-design eval; testing real image/video files only matters once you're
  testing vision or the shipped pipeline, not the prompt design.
- **No LLM-judge layer.** Every check in `judge.go` is deterministic (token/media/escalate/
  a Kazakh-letter heuristic/reply_language field). A few real questions (e.g. "does this
  read as a natural next step") have no automated check — read the injected text in
  `CONTRACT.md` by eye.
- **A committed run is inspectable, not re-judgeable from a fresh clone.** Each run's
  `manifest.json` and `snapshots/` (scenario.yaml, prompt.txt, catalog.json,
  resolved_tests.json, promptfooconfig.yaml, models.yaml, extract cases — all small text)
  are committed, so you can see exactly what a run graded against. But the raw
  `*.results.json` (promptfoo's answers) and, for extraction, the processed input images
  under `inputs/`, are gitignored — large and reproducible by re-running, not durable
  history. `manifest.json` records their sha256 so you can tell if a local copy still
  matches, but a fresh clone alone can't re-run `judge`/`report` end to end without
  re-generating those first.

## The one earlier eval this replaced

Before the playground, `evals/` was 3 hand-written prompt files (`schemas/current.sql` +
`schemas/decisions-v1.sql` + a flat `promptfooconfig.yaml`). That comparison's numbers are
preserved in `results/REPORT.md` as a historical record — it is not re-run or updated by
anything here.
