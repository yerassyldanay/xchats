# Eval vocabulary

Short glossary of the terms used across `evals/` (scenarios, harness, catalog page).

## Test organization

- **Scenario** (test family) — a folder `scenarios/<name>/` with `scenario.yaml`: one prompt + one KB + one test list, runnable as a unit.
- **Test case** — one entry in a `tests:` list; the atomic graded unit.
- **Common bank** — a shared test file in `common/*.yaml`, pulled into scenarios via **include**.
- **Experiment** — a `scenario.yaml` label grouping scenarios into a bake-off.
- **Setup** — a scenario's human display label.
- **Pipeline** — the resolution path: legacy `fact_tables`, or `schema_kb_v1` (the shop-kb family).
- **Scale variant** — same KB, different `limits` (e.g. `shop-kb-v1-10/-50/-100`).
- **Canary** — a small fast-signal suite. **Holdout** — the sealed `canary-holdout/`, deliberately unreachable by `-all`.
- **Archived** — a scenario kept for history (`archived: true` + `archived_reason`), no longer active.
- **Extract case** — the image → structured-extraction eval family (`extract/cases.yaml`).

## Knowledge & prompt

- **KB / fixture** — a scenario's knowledge base (`data.yaml` / `kb-fixture.yaml`): products, policies, delivery zones, contacts.
- **Fact** — one token→value pair. **Token (placeholder)** — `{{table.ref.field}}`; the model cites facts by token, never by value.
- **Injection** — code substituting token → value after the model replies; turns **raw reply_text** into **injected text**.
- **Catalog (model-facing)** — the value-free list of tokens a render allows. **Requirements catalog** — `runs/catalog.json`, the audit artifact behind `/evals/catalog`.
- **Frame** — the prompt template (`frame.txt`). **Contract** — the required reply JSON: `reply_text`, `reply_language`, `escalate`, `media_files_to_send`.
- **BLOCKED** — fail-closed state: an unknown `{{…}}` in a reply; production refuses to send it.
- **Escalation** — the model handing the dialog to a human instead of answering.

## Checks

Declared per test — undeclared = vacuously true ("not checked"):

- **requires** — token check, AND-of-OR: the literal `{{delivery.almaty.delivery_cost}}` must appear in the **raw** reply.
- **forbid_tokens** — negative token check on the **raw** reply; a trailing dot bans a family (`"delivery."` = every zone).
- **escalate** — field check: the contract's `escalate` bool must equal the declared value exactly.
- **language** — field + heuristic check: `reply_language` must be the declared `ru|kk`; Kazakh-letter count on the text.
- **must_not_contain / must_contain_any** — substring checks, case-insensitive, on the **injected** text.
- **media** — membership check on `media_files_to_send`: `any_of` (≥1) / `all_of` (all) / `forbid` (none) / `exclusive` (nothing else).
- **outcomes** — OR-gate: ≥2 labeled blocks of the same knobs; the test passes the gate when any one block fully holds.
- **stock_check / llm_checks** — semantic checks judged by a pinned LLM; `judge-llm` command only, billed, reported as a separate dimension.

Universal — always run, never declared:

- Contract group: **parse_ok** (JSON extracts), **contract_fields** (typed fields present), **no_unknown_tokens** (unknown `{{…}}` ⇒ BLOCKED), **no_leftover_braces**, **no_reasoning_leak**, **no_control_chars**, **finish_reason_ok** (no truncation). `schema_kb_v1` adds **exact_value_literal** (wrote a fact's value as literal text instead of its token) and **media_resolve_ok** (media still resolves against live materials).
- Behavior group: **no_invented_digits** (digit runs outside tokens / the customer's message / trusted description text), **no_unit_issues** (post-injection artifacts like «1 500 ₸ ₸»), **no_unknown_media**, **media_count** (≤ 2 attachments).

Aggregates: **ContractPass** (the pipeline would accept the reply) · **ModelBehaviorPass** (the model behaved honestly) · the LLM dimension is reported separately, never folded in. **Verdict** — the per-test record all checks fold into.

## Where checks look (template vs final text)

- **Raw reply_text** (template, as the model wrote it) — token discipline: `requires`, `forbid_tokens`, `no_unknown_tokens`, `exact_value_literal`. Proves grounding in the KB; immune to KB value edits.
- **Injected text** (what the customer would receive) — wording: `must_not_contain` / `must_contain_any`, `no_unit_issues`, the language heuristic. Catches phrases and errors that only materialize after substitution.
- **Parsed contract JSON** — structure: `parse_ok`, `contract_fields`, `escalate`, media checks.

## Harness stages & artifacts

- **Render** (free) → prompts + `resolved_tests.json` · **Run** (billed) → provider outputs · **Judge** (free, deterministic) / **judge-llm** (billed, semantic) · **Export** → `runs.json`, `executions.json`, `catalog.json` (`generated_at` = freshness signal).
- **Run snapshot vs repository state** — past results vs what the repo defines now; the catalog page shows only repository state, the Запуски page shows snapshots.
