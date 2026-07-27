# shop-kb-v1 — schema-driven shop family (Russian KB, Kazakh-aware replies)

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
- `frame-ru.txt` — the only frame this family ever renders (prompt_ref
  `shop-kb@v4`). The KNOWLEDGE BASE is Russian-only, but the frame DOES
  route reply language by the CUSTOMER's own message (rule 7): a Kazakh
  message gets a fully Kazakh reply, recognized by words and grammar, not
  by alphabet — a customer typing Kazakh on a Russian keyboard (no
  ә/ғ/қ/ң/ө/ұ/ү/һ/і at all) still gets routed to Kazakh. `reply_language`
  is still enforced as one of `"ru"`/`"kk"` by `aiprompt.ValidateResponse`.

Unlike every other family in `evals/scenarios/`, this one's prompt is NOT
built by this harness's own `buildCatalog`/`buildPrompt` — `data-ru.yaml` is
loaded by `internal/kbfixture` and rendered through `backend/aiprompt`
directly, the same package the production backend will eventually call. See
`evals/PLAYGROUND.md`'s "What this is" section for how the two pipelines
relate.

## The three active size variants

| Scenario         | ai_products limit  | Question banks (all five, identical across sizes)                                                              |
| ----------------- | ------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `shop-kb-v1-10`   | 10                  | `kb-questions-ru`, `kb-history-ru`, `kb-delivery-ru`, `kb-messages-kk`, `kb-messages-kk-ambiguous` |
| `shop-kb-v1-50`   | 50                  | same five banks                                                                                                    |
| `shop-kb-v1-100`  | (none — full pool)  | same five banks                                                                                                    |

`scenarios/shop-kb-v1-{10,50,100}/tests.yaml` are byte-identical by design
(see that file's own header comment) — the catalog-size sweep controls for
every variable except product count, so the question set itself never
differs between sizes. `kb-messages-kk.yaml` (native Kazakh orthography) and
`kb-messages-kk-ambiguous.yaml` (Kazakh typed with only Russian-keyboard
letters, mixed-language clauses, mid-conversation language switches) are a
separate axis layered on top of the Russian-customer banks — both DRAFT,
machine-translated Kazakh pending native-speaker review (see each bank's own
header comment).

Earlier size families in this pool's history (`shop-kb-v1-30`,
`shop-kb-v1-scale-60`, `shop-kb-v1-scale-100`) are archived —
`archived: true` + `archived_reason` in each `scenario.yaml` explains what
superseded them; their historical runs are preserved unchanged.

## Regenerating and verifying (free)

```bash
cd evals/harness
go run ./cmd/genkbfixture        # rewrite data-ru.yaml if the generator changed
go test ./...                    # structural shape, strict round-trip, drift, fail-closed matrix,
                                  # frame guard test, lingua-go bank/canary agreement
go build -o harness . && cd ..
./harness/harness render -scenario scenarios/shop-kb-v1-10
./harness/harness render -scenario scenarios/shop-kb-v1-50
./harness/harness render -scenario scenarios/shop-kb-v1-100
```

## Running against real models (billed)

```bash
cd evals
./harness/harness run -scenario scenarios/shop-kb-v1-10,scenarios/shop-kb-v1-50,scenarios/shop-kb-v1-100 \
  -models openrouter:google/gemini-2.5-flash,openrouter:google/gemini-3.5-flash \
  -expect-calls <tests_per_scenario * models * 3>
./harness/harness rewrite-lang -scenario scenarios/shop-kb-v1-10 -run runs/<id>   # repeat per size
./harness/harness judge-llm -scenario scenarios/shop-kb-v1-10 -run runs/<id>      # repeat per size
```

`rewrite-lang` is optional and only ever bills for a row whose reply's
language doesn't match the customer's — see its own doc comment
(`evals/harness/rewrite_lang.go`) for the full mismatch/rewrite/re-judge
contract. Run it before `judge-llm` so judge-llm's semantic checks see the
final (possibly rewritten) text.
