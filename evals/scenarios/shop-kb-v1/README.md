# shop-kb-v1 — schema-driven, Russian-only shop family

This directory holds the shared source data for the `shop-kb-v1-*` scenarios
(`pipeline: schema_kb_v1`). It has no `scenario.yaml` of its own — same
pattern as `scenarios/shop-scale/` — because it is a shared pool three
sibling scenario directories point at with different `limits.ai_products`
caps, not a runnable scenario by itself.

- `data-ru.yaml` — GENERATED. Run `cd evals/harness && go run
  ./cmd/genkbfixture` to regenerate; never hand-edit (see the file's own
  header and `internal/kbfixture/generate.go`). 100 products, ~9 topics, one
  contacts row, one policies row (`return_period_in_days` deliberately
  empty; `delivery_cost`/`delivery_in_days` blank and `outside_zones_note`
  set — see `ai_delivery_zones` below), four `ai_delivery_zones` rows (a
  country-level fallback, two cheaper/faster cities nested under it, and one
  explicit deny city), and one `kbd_materials` row per referenced file.
- `frame-ru.txt` — the only frame this family ever renders. No language-
  routing rule: every reply is Russian (`reply_language: "ru"`), enforced by
  `aiprompt.ValidateResponse`, not by frame wording alone.

Unlike every other family in `evals/scenarios/`, this one's prompt is NOT
built by this harness's own `buildCatalog`/`buildPrompt` — `data-ru.yaml` is
loaded by `internal/kbfixture` and rendered through `backend/aiprompt`
directly, the same package the production backend will eventually call. See
`evals/PLAYGROUND.md`'s "What this is" section for how the two pipelines
relate.

## The three size variants

| Scenario                | ai_products limit | Question bank                                                          |
| ------------------------ | ------------------ | ----------------------------------------------------------------------- |
| `shop-kb-v1-30`          | 30                 | core + one-shot history + delivery-zones banks + 1 boundary test        |
| `shop-kb-v1-scale-60`    | 60                 | core + delivery-zones banks + 2 deep-boundary tests                     |
| `shop-kb-v1-scale-100`   | (none — full pool) | core + delivery-zones banks + 2 deep-boundary tests                     |

Each deep-boundary test targets a specific product near that scenario's own
row-count boundary (e.g. the ~30th product for `shop-kb-v1-30`), so scaling
is actually exercised at the edge of the visible catalog, not just on the
same first few rows every size would trivially pass.

The five history cases (`common/kb-history-ru.yaml`) run only in the
30-product baseline: every case is one independent provider call containing
its complete history plus final message; the 60/100 variants stay focused on
catalog-size boundaries. The ten delivery-zones cases
(`common/kb-delivery-ru.yaml`) run in ALL THREE sizes — delivery/escalation
behavior is verified at every scale, not just the smallest one — and
`ai_delivery_zones` is unaffected by `limits.ai_products`, so the same four
zones are present regardless of scenario size.

## Prompt sizes (recorded 2026-07-22, free `render` only — no model calls)

Measured directly on `generated/prompt.txt` after `harness render`; word/char
counts are a size proxy, not a token count — actual token usage is model-
specific and is recorded per real call in a billed run's `SUMMARY.md`
(prompt-vs-completion token share), never estimated here.

| Scenario                | Fact tokens | Media entries | Tests | Prompt chars | Prompt words |
| ------------------------ | ----------- | -------------- | ----- | ------------- | ------------- |
| `shop-kb-v1-30`          | 76          | 30              | 31    | 44,303        | 3,675         |
| `shop-kb-v1-scale-60`    | 136         | 56              | 27    | 69,130        | 5,739         |
| `shop-kb-v1-scale-100`   | 216         | 91              | 27    | 104,181       | 8,488         |

Growth from 30 to 100 products is roughly linear (~2.6x the products, ~2.6x
the prompt size) — no unexpected super-linear blowup from the fact/media
catalog construction at this scale.

## Regenerating and verifying (free)

```bash
cd evals/harness
go run ./cmd/genkbfixture        # rewrite data-ru.yaml if the generator changed
go test ./...                    # structural shape, strict round-trip, drift, fail-closed matrix
go build -o harness . && cd ..
./harness/harness render -scenario scenarios/shop-kb-v1-30
./harness/harness render -scenario scenarios/shop-kb-v1-scale-60
./harness/harness render -scenario scenarios/shop-kb-v1-scale-100
```

No scenario in this family has been run against a real model yet — that is
the billed checkpoint the parent task hands to a human to run explicitly.
