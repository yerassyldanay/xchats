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

Run directories are local and gitignored by default. If a run becomes durable project
evidence, review its manifest, snapshots, judged output, and reports, then deliberately
force-add only those reviewed evidence files and add one line to `runs/INDEX.md`. Never
force-add the entire run directory: raw provider results, viewer exports, processed
inputs, and HTML remain reproducible local artifacts and must not be committed.

Before spending anything, `run` prints the resolved `(tests x models) = calls` count. Narrow
which models run with `-models google/gemini-2.5-flash,openai/gpt-4o-mini`. Omitting `-models`
no longer means "every provider" — it resolves to whichever providers are marked
`default: true` in `models.yaml` (today: gpt-4o-mini, gemini-2.5-flash-lite), so a bare `run`
can't accidentally fan out to every configured model. Pass `-models all` to explicitly run
every provider in the file (`-models-file` overrides the models.yaml path itself). Pass
`-expect-calls N` to hard-fail before any call if the resolved count doesn't match N — a
deliberate confirmation gate for a run you want to cost-check first:

```bash
./harness/harness run -scenario scenarios/shop-current -models google/gemini-2.5-flash -expect-calls 29
```

Running two or more `shop-scale-N` scenarios together adds a "Scale comparison" table to
`SUMMARY.md` — model-behavior pass % and avg tokens per answer at each catalog size, so
"does quality hold up as the product list grows" is answerable from one place.

## Formalized sample sizes

Two stages, two sample sizes — both uncached, both counted by the `-expect-calls` gate
before anything is spent:

- **Screening** (comparing frame/prompt variants, e.g. the language bake-off's V1-V4 or
  an escalation-wording V1/V2): 3 uncached repetitions per (test, model) pair.
  ```bash
  ./harness/harness run -scenario scenarios/lang-canary-v1 -repeats 3 -no-cache -expect-calls 84
  ```
- **Survivor stage** (the 1-2 variants screening didn't eliminate): 15 unique Kazakh
  intents x 5 repetitions = 75 outputs per prompt+model pair, at production temperature.
  `-repeats N` (`N>1`) hard-requires `-no-cache` — promptfoo's own repeat/cache
  interaction isn't something this harness trusts blindly (see `run.go`'s
  `validateRepeats`), and a repeat that silently replayed a cached answer would
  deflate exactly the variance a confidence interval depends on. **The 15-intent Kazakh
  bank itself does not exist yet** — every Kazakh scenario in this repo is flagged DRAFT,
  needing native-speaker review before a billed run, and that review is out of scope for
  the harness changes that added `-repeats`; authoring it is separate, later work.
  Alongside the production-temperature run, a **temperature-0 diagnostic** (same 75
  intents, `temperature: 0` — does the answer even change run to run when nothing else
  does?) is informational only: run it as its own separate `harness run` invocation
  against a sibling models file (see `models-diagnostic-t0.yaml`), into its own run dir —
  never merged into the production-temperature numbers.

`SUMMARY.md`'s per-model row reports a 95% Wilson interval on the **pooled** result (all
N outputs for that one prompt+model pair together) — never a per-intent interval; 5
repetitions per intent is enough to notice one systematically-broken intent, not enough
for its own confidence interval. Rows are always one-per-model: a specific prompt+model
combination is the experimental unit throughout, and results are never pooled across
different models.

## Finalist workflow (language quality, once screening narrows to 1-2 variants)

`judge.go`'s `looksKazakh` (a 2-letter presence heuristic) is fine for cheap early
screening, but it grades the same kind of text `detectLang` uses to *route* — a
circularity that isn't trustworthy for picking a real finalist. At the finalist stage,
replace it with a blinded human read, and track three signals as genuinely separate
numbers, never collapsed into one pass/fail:

1. **Routing accuracy** — did `detectLang` pick the frame a human would have. Needs no
   live review: a test's own `language:` field already **is** a human's judgment of the
   correct frame, recorded at authoring time. Written automatically by `blind-export`
   (below) as `ROUTING_ACCURACY.md`, or compute it standalone any time via
   `computeRoutingAccuracy` (`blind_types.go`).
2. **Declared `reply_language`** — the model's own self-reported field.
3. **Blinded prose-language label** — a human, shown ONLY the customer's message and the
   model's reply text (no model id, no prompt variant, no declared language), labels the
   reply `kk` / `ru` / `mixed` / `unclear`.

```bash
./harness/harness blind-export -run runs/<finalist-run-id> -out runs/<finalist-run-id>/review
# -> review/review.csv               (hand this to a reviewer; blank `label` column)
# -> review/mapping.DO-NOT-SHARE-WITH-REVIEWER.json   (withhold — keeps the review blind)
# -> review/ROUTING_ACCURACY.md      (signal 1, ready immediately, no review needed)

# a reviewer fills in review.csv's `label` column with kk / ru / mixed / unclear, blind
# to which model or prompt variant produced each row, then:
./harness/harness blind-report -review runs/<finalist-run-id>/review/review.csv \
  -mapping runs/<finalist-run-id>/review/mapping.DO-NOT-SHARE-WITH-REVIEWER.json
# -> review/BLIND_REPORT.md          (signals 2 and 3, plus how often they agree)
```

`blind-export` only exports `ContractPass` rows (nothing else is a meaningful
language-quality judgment) and refuses to overwrite an existing export without `-force`,
so re-running it can't silently desync a reviewer's in-progress labels. `blind-report`
rejects a `review.csv`/mapping pair that don't genuinely match (a content hash, not just
the opaque row ids, which aren't unique across different exports of the same size).

**The held-out canary**: `canary-holdout/tests.yaml` — sealed, outside `scenarios/` so
`-all` can never reach it, empty until a finalist is actually chosen (see the file's own
header for why real content isn't in it yet: native-speaker review is separate, later
work). Never touched during screening or prompt iteration; opened only once, after a
finalist is picked.

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
  `common/shop-questions.yaml`'s "16. follow-up with history" for the pattern. When two
  different behaviors are BOTH acceptable (e.g. an ambiguous pronoun: answer for the
  last-named tariff OR ask which one is meant), declare `outcomes:` — a list of >=2
  labeled alternative expectation blocks, each with the same knobs a test has
  (requires/escalate/language/media/must_not_contain/must_contain_any); the answer passes
  the gate if ANY one block fully holds, while top-level checks stay universal. See
  `common/xpayment-history-questions.yaml`'s xph2 for the pattern.

Language rule note: since the Phase 2.3 combo variants, every frame's mixed-language rule
is "reply in the DOMINANT language — the one the question itself is asked in" (it used to
be a blanket "mixed → Russian"). `detectLang` (langdetect.go) resolves mixed messages the
same way (last question clause, then clause majority, tie → ru), so the routed variants'
kk/ru test splits and the frames' own rule can never disagree —
`TestDetectLang_AgreesWithRoutedCanarySplits` enforces it.

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
- **`runs/INDEX.md`** — one line per deliberately retained evidence run; ordinary local
  attempts are discovered through the generated viewer export instead.
- **`index.html`** — one self-contained page per run covering BOTH families: for scenario
  runs, the same model × pass-rate table as SUMMARY.md plus a collapsible per-verdict
  detail (scores, injected text, evidence); for extraction runs, each case's captured
  input image next to every model/prompt variant's parsed fields, checks, and raw
  output. Written automatically at the end of `run`, `extract`, and `report` (best-effort
  — a broken viewer never fails the underlying eval); regenerate by hand with `harness
  html -run <dir>`. Gitignored (regenerate rather than diff in review). Works on runs
  from before this existed too, degrading gracefully — no manifest section, and a
  captured-input image shows as "input not captured" rather than a reconstructed guess.
- **`executions.json` / `runs.json`** — the same data as `index.html`, as dedicated
  schema-versioned JSON instead of a rendered page: the eval viewer's (see below) one
  data source. Written alongside `index.html` by the same commands; gitignored the
  same way.

## Comparing prompts and models (the eval viewer)

The fastest way to see "which prompt and model wins" is the Vue page baked into the
product frontend, not `SUMMARY.md`/`index.html` directly:

```bash
make up                          # from the repo root — builds + starts postgres/backend/frontend
cd evals && ./harness/harness export -all   # regenerate executions.json + runs.json once
```

Then open `http://localhost:8081/evals` (log in first) — a launches list, each with a
decision matrix (model × prompt/setup, pass rate + contract rate + cost + latency per
cell) and a drill-down into every individual test: the customer message and history, the
exact prompt version used, the model's raw reply next to the post-injection
customer-facing text, and every check. Both eval families (WhatsApp responses and media
extraction) show up as tabs within one launch.

How the data gets there: `harness html` (auto-run at the end of `run`/`extract`/`report`,
best-effort) and `harness export` (fatal-on-error — the one command a **fresh clone**
needs before the viewer has anything to show, since the derived `executions.json`/
`runs.json` are gitignored, same status as `index.html`) both write the SAME dedicated,
schema-versioned JSON next to each run's `SUMMARY.md`. `frontend/nginx.conf` serves
`evals/runs/` read-only at `/evals-data/` — mounted only in
`docker-compose.override.yaml` (**local dev only**: this is raw model output, prompts,
and KB material with no auth in front of it; the base compose file never mounts it, so
an internet-facing deploy of the same image simply 404s there).

### Grouping multiple families into one launch

```bash
./harness/harness launch -all -expect-calls 616
```

Mints one `launch_id`, writes `runs/launches/<id>.json` **before any billed call**
(status, planned families, the COMBINED scenario+extract pre-flight count —
`-expect-calls` gates the total, same discipline as `run`'s own gate), then runs both
families under that id so the viewer groups them as one launch instead of two unrelated
runs. A launch where one family fails and the other completes reports `partial`, not a
flat success/failure — read `runs/launches/<id>.json` to see which. Running `run`/
`extract` directly (not through `launch`) is still fully supported; each is simply its
own singleton launch unless you pass its own `-launch <id>` flag.

### Comparison metadata (`setup` / `prompt_ref` / `experiment`)

Chat scenarios can optionally declare, in `scenario.yaml`:

```yaml
setup: lang-v4-routed        # the comparison COLUMN — a strategy, not necessarily one file
prompt_ref: lang-kk@v4       # the ACTUAL frame used (ParsePromptSpec syntax: name@vN)
experiment: lang-bakeoff     # the comparison GROUP — only same-experiment setups pool into one matrix
```

`setup` and `prompt_ref` can differ on purpose: the V4 language-routed strategy is ONE
setup (`lang-v4-routed`) realized by TWO scenario dirs (`lang-canary-v4-kk` /
`lang-canary-v4-ru`), each with its own `prompt_ref` — the viewer's matrix shows one
column for the strategy, while the drill-down for any individual execution still shows
exactly which frame it actually used. All three fields are optional; an unannotated
scenario (e.g. `shop-*`) falls back to its own name for both `setup` and `prompt_ref`,
and to an empty `experiment` (never auto-compared against anything — the safer default).
See `lang-canary-v1..v4-*` and `escalation-canary-v1/v2` for worked examples.

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

The asset directory intentionally contains only the five images referenced by these
cases and `xpayment_caledar_ads_kz.mp3`. The MP3 is reserved for a future audio or
transcription evaluation and is not used by the current image-only extraction harness.
Full-size video fixtures are excluded until an active video case justifies a small,
deliberately committed sample.

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
  separately: [TestPostProcess_PriceRenderFailurePostsManualNote](../backend/internal/brain/prompt_test.go#L160).
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
- **A retained run is inspectable, not re-judgeable from a fresh clone.** Selected
  evidence bundles keep their manifest, snapshots, judged output, and human reports so
  the decision remains reviewable. Raw `*.results.json`, processed inputs, viewer JSON,
  and HTML are gitignored because they are large or reproducible. A manifest records the
  raw output hashes, but re-judging still requires regenerating or restoring those raw
  provider results.
