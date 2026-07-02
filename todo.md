# TODO — Knowledge Base & Playground

The architecture is **not** described here — it lives in the plan (this file only points and tracks).
- **Decisions:** [`plan/14-draft-staging-and-retrieval.md`](plan/14-draft-staging-and-retrieval.md)
  (draft tables, approve = gate→copy→embed, no tokens in topic bodies, ru-only v1, retrieval) and
  [`plan/13-kb-facts-and-grounding.md`](plan/13-kb-facts-and-grounding.md) (typed fact tables,
  anti-hallucination lanes).
- **Build plan:** [`plan/12-playground-build.md`](plan/12-playground-build.md) (layers L1–L5; read its
  banner — the `drafted_at` parts are superseded by 14).

## Tasks

- [ ] **Migration** — live KB tables (`ai_snapshots`, `ai_topics`, `ai_assets`, `ai_tariffs`,
      `ai_products`, `ai_contacts`) + their **draft twin tables** (same columns + delete-marker).
      No `drafted_at` column anywhere.
- [ ] **KBStore** — reads live only; playground CRUD writes draft tables only; no direct live writes.
- [ ] **Approve** — deterministic gate → copy approved rows to live (upsert on natural key; apply
      delete-markers) → refresh topic embeddings, in one step. Reject = delete the draft row.
- [ ] **Seed** — re-express the demo seed: topic bodies as pure prose (strip fact tokens/digits),
      facts into typed columns, **ru rows only**.
- [ ] **Brain prompt** — `[F]` facts catalog from live fact tables (single-language, cache-stable);
      topics `[D]` token-free.
- [ ] **Reply pipeline (v1)** — escalate gate → template render (fail closed) → number check →
      media validation → human review. Grounding judge: deferred (required before auto-send).
- [ ] **Retrieval** — pgvector over live topics only; facts always included exactly. (Embedding
      model/chunking: decide in a follow-up record.)
- [ ] **Playground UI** — draft rows from draft tables («Черновик» badge), per-row/bulk approve.
