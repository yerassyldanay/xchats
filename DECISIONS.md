# AI Assistant & Playground — Design Decisions (July 2026)

Status: **agreed in discussion, not yet implemented.** Where this file conflicts
with `plan/*.md` or today's code, **this file wins**. It is the single
authoritative decision record: it merges and **supersedes** `DECISION-BY-CLAUDE.md`
and `DECISION-BY-CODEX.md` (merged 2026-07-10).

---

## Part 1 — the idea, in plain words

1. **The whole KB goes into every prompt — no vector search / RAG.** One org's KB
   is small; if the model always sees everything, nothing relevant can fail to be
   "found". Cost is covered by the provider's prompt caching (the prompt only
   changes when the KB changes). Caching affects price and speed, never answers.
   If a KB ever outgrows the prompt, the fallback is **deterministic narrowing**
   (shortlist by category/owner in code) — not free-form vector retrieval
   (threshold: see Open questions).

2. **Exact facts live in typed columns, values language-neutral.** Column names
   explain themselves (`price`, `delivery_in_days`); values are digits, money,
   times, links — never words. One stored value serves Russian and Kazakh replies
   alike. A value that needs words («в наличии», address) is not a fact — it is
   trusted plain text the model phrases itself.

3. **The model never writes numbers — it writes placeholders.** A reply carries
   `{{product.sofa-loft.price}}`; code substitutes the stored value. An unknown
   placeholder blocks the whole draft for manual check. A made-up number is
   silent; a made-up placeholder fails loudly. Wrong prices in writing are a
   business and legal risk.

4. **Product media = owner + role, always sent whole.** Files are assigned to an
   owner and a role (`product.sofa-loft.gallery`, `.certificate`). To attach
   media the model names that reference and code sends **everything** in it —
   like a seller sending a whole album on WhatsApp. No per-file descriptions in
   the KB, no per-file picking. Unknown reference → dropped, fail-closed.

5. **The KB is built in the playground in two passes.** The operator drops any mix
   of material (text, URLs, images, PDFs, audio, video). Pass 1 reads each file
   separately; pass 2 turns everything into draft KB entries in the exact KB
   schema — written into the **one draft immediately** (no per-job mini-drafts,
   no intermediate approvals). A human reviews **once**, at draft → live.
   Nothing touches the live KB before Approve.

6. **Every uploaded file has a visibility: `auto | invisible | visible`.**
   Default `auto`: the system decides whether a file is only *evidence* (a
   screenshot, a voice note — its information enters the KB, the file never
   reaches a customer) or *customer-sendable media* (a product photo). The
   operator's explicit choice always wins over the model. A file becomes
   customer-visible only when assigned to an owner+role **and** approved.

---

## Part 2 — how it works (technical)

### Core principles

1. **Draft and live schemas are the same.** Draft entries mirror live rows —
   same business columns. Approval validates and upserts; it never translates.
2. **Raw files are stored before extraction.** Upload success is final for the
   bytes; a failed extraction never loses the file.
3. **Parse each material separately.** Every input gets its own row and status.
4. **Synthesize after extraction**, over the whole batch + current draft + live KB.
5. **The model proposes; code validates.** Every id, ref, column, and value is
   checked before anything is stored.
6. **Exact values are handled carefully.** Typed columns only, human-confirmed.
7. **The customer-facing assistant reads live rows only.** Never the draft,
   never staging.

### The flow, end to end

```text
1. operator submits inputs (files / url / text / instruction)
2. upload: one material row per input (text / url / file); file bytes → blob
   store + one media_files row per uploaded file; one extraction job per material
3. extraction (pass 1): parallel per file → parsed | needs_human | failed
4. synthesis (pass 2): parsed evidence + instruction + live KB + draft
   → KB-shaped draft patch + requests
5. validation: ids exist, refs resolve, columns exist, values well-formed
6. draft upsert: atomic merge into the org's draft blob (by natural keys)
7. review: operator edits / confirms / rejects on the accumulated draft
8. approve: gate → upsert into live tables → clear from draft → reload brain
```

### Two passes

**Pass 1 — per-file understanding (parallel, one request per file).** Each parse
gets context: the operator's message + a compact index of the draft and live KB
(refs, slugs, roles). Output per file, stored on its row: a short visual/audio
summary, extracted text (OCR / transcript / page text), detected facts with
provenance, a "relates to" hint, and a visibility suggestion. Pass 1
**describes, never decides** KB structure. Its summaries are working notes —
they never enter the live KB. Rejected: one bundled parse request for all files
(kills per-file isolation, retries, and provenance).

**Pass 2 — batch synthesis (one text-only call, no bytes).** Input: the
operator's instruction + **the content of every parsed file** + attachable ids
**only for customer-visible files** + the **full draft** + the **full live KB**
(so consecutive uploads build on each other and update-vs-create is real
diffing). Output: a **draft patch in the KB schema** plus requests. Pass 2 is
side-effect-free while running; the patch merges into the draft in one atomic
write. It must not assume one file = one entity: files 1–2 may be one product's
gallery, file 3 its spec plate, file 4 a tariff infographic.

Manifest in (trimmed example):

```json
{ "media_files": [
    { "id": "file_01", "visibility": "visible", "source_type": "image",
      "extraction_status": "parsed",
      "visual_summary": "Front photo of a magnetic drill.", "extracted_text": "" },
    { "id": "file_02", "visibility": "visible", "source_type": "image",
      "extraction_status": "parsed",
      "visual_summary": "Nameplate: model ZT-40H, 1450W.",
      "extracted_text": "ZT-40H, 1450W, 820 r/min",
      "detected_facts": [ { "field_hint": "power_watts", "value": "1450",
                            "provenance": "file_02" } ] },
    { "id": "source_03", "visibility": "invisible", "source_type": "image",
      "extraction_status": "parsed",
      "visual_summary": "Screenshot of a supplier pricing page.",
      "extracted_text": "ZT-40H wholesale 180 000 ₸" } ] }
```

Id namespaces carry the visibility rule: **visible files get attachable
`file_*` ids; invisible evidence gets `source_*` ids**, valid only in
provenance/`sources` fields — validation rejects a `source_*` anywhere else.

Draft patch out (trimmed example):

```json
{ "draft_patch": {
    "products": [ { "ref": "drill-zt40h", "name": "ZT-40H magnetic drill" } ],
    "media_files": [
      { "id": "file_01", "owner_type": "product", "owner_ref": "drill-zt40h",
        "role": "gallery", "sort_order": 1 },
      { "id": "file_02", "owner_type": "product", "owner_ref": "drill-zt40h",
        "role": "spec_plate", "sort_order": 2 } ] },
  "requests": [
    { "type": "confirm_value",
      "target": { "table": "products", "ref": "drill-zt40h", "field": "power_watts" },
      "suggested_value": "1450", "source": "file_02",
      "reason": "Read from the nameplate — needs human confirmation." } ] }
```

### Draft = same schema, stored as one jsonb blob per org

- The entire pending KB is **one jsonb document** (`kbd_draft`), one row per org:
  `{ config, topics[], products[], tariffs[], contacts[], policies[], media_files[], deletes[] }`
  — each entry mirrors a live-table row. The draft holds **deltas only**, not a
  copy of the KB. The brain never reads it.
- Jobs upsert into the draft **immediately** — rejected: per-job "mini drafts"
  with their own approval step (double review, stale snapshots, broken
  build-on-each-other). The safety boundary is draft → live, not job → draft.
- An update = a draft entry **shadowing** the live row by natural key
  (`products.ref`, `tariffs.ref`, `topics.slug`, singletons by org). Entity
  states are **derived, never stored**: no shadow = published · draft only =
  **Новый** · draft + live = **Изменён** · `deletes[]` marker = **К удалению**.
  A patch entry identical to the live row is dropped (no badge noise).
- Diffs are **computed** (draft entry vs live row, field by field — trivial
  because the schemas are identical), not stored. Rejected: per-row lifecycle
  columns (`change_type`, `review_status`, `base_live_version`,
  `changed_fields_json`) on draft/live tables — derived state cannot drift.
- A new upload touching an already-pending entity **overwrites its draft entry,
  building on it** (never resetting to live). Field-merge rule: empty field +
  new value → fill (via request if exact); same value → keep, add provenance;
  **different exact value → confirmation request**; different prose → update
  with a visible field-level diff. The second change is loud (chat + activity
  log), never silent.
- Merges are idempotent (natural keys) — re-running a pass never duplicates.
  One version counter on the blob catches concurrent writes (stale → 409).
- How the user sees changes: the **chat narrates** each turn; **badges + a
  field-level diff vs live** show the state now; **provenance** (source files
  per entry) explains why; the **audit log** records approves. "Reject this
  whole upload" = a bulk action over provenance, not a separate approval layer.
- Approve (whole draft or selected entities): run the gate → upsert into live
  tables on natural keys → apply `deletes[]` → remove from the blob → reload
  the brain. No versioning, no rollback in v1 (accepted trade-off).

### Media: one file table, fixed roles, derived references

- **One `media_files` table** holds every uploaded file: storage metadata
  (`storage_key`, `filename`, `mime_type`, `size`, `checksum`), extraction
  evidence (`extracted_text`, `visual_summary`, `transcript_text`,
  `extraction_json`, `extraction_status`), and the business assignment
  (`owner_type`, `owner_ref`, `role`, `sort_order`, `visibility`). No
  `product_media` / `topic_media` / group join tables.
- **Fixed role vocabulary** (extensible by us, never free-text):
  `gallery | certificate | pricing | instruction | spec_plate | document`.
  This closes the old "group vocabulary" question — a typo'd role is impossible.
- v1: one primary owner per file; reuse under several entities later =
  duplicate the metadata row over the same `storage_key`, or revisit.
- Assignment changes proposed by pass 2 ride the draft blob's `media_files[]`
  patch entries like every other entity and are **stamped onto the file row at
  approve**. (There is no separate `assets` concept — one name, one array.)
- The runtime prompt exposes media as a **derived catalog**, grouped
  `owner_type.owner_ref.role` with counts — e.g. `product.drill-zt40h.gallery: 3
  image(s)`. The model names a reference; code resolves it to rows and sends
  **everything** in it.

### Visibility: `auto | invisible | visible`

- Per-file, set at upload (default `auto`), editable in review. **The operator's
  choice always wins over the model**: user `invisible` → the model cannot make
  it visible; user `visible` → the model may attach it; `auto` → the model
  suggests.
- Pass 2 receives the **content of every parsed file**, but **attachable ids
  only for customer-visible files**. Invisible files feed knowledge; their
  provenance travels in extraction metadata (`detected_facts[].provenance`,
  source refs on entries), never as an attachable id — so attaching one is
  impossible, not merely forbidden.
- Enforcement (accepted trade-off, replaces the earlier two-table wall): the
  prompt-layer id exclusion above **plus** a strict loader — the brain's media
  catalog materializes only rows that are approved **and** visible **and**
  owner-assigned. Two independent layers; a leak requires both to fail plus a
  human approving the entry.
- The UI shows every file's fate plainly: «использован как источник, клиентам
  не отправляется» vs «альбом product.sofa-loft.gallery». Flipping a wrong call
  is one action; nothing is real until approve.

### Requests: a small sidecar table

> Note: this **reverses** an earlier "no stored question queue" decision — the
> review settled on a sidecar table as useful workflow state.

- `draft_requests` — workflow state, **never part of the KB schema**. Types:
  `confirm_value | describe_file | choose_owner | resolve_duplicate | conflict`.
- Anti-staleness rule: every request targets data **by natural key**
  (table+ref+field / file id). Editing or resolving the target **auto-resolves
  the request** — a request can never ask about a state that no longer exists.
- Approve is blocked only by open requests attached to the rows being approved;
  unrelated open requests never block.
- Ambiguity that needs conversation (mixed intent, unclear instruction) is
  asked **in the chat**, not stored; the operator's next message answers it.

### Errors: skip the failed, proceed with the rest

- Material statuses: `uploaded → extracting → parsed | needs_human | failed`,
  plus **`built`** once a synthesis pass has consumed the evidence (prevents
  re-feeding old materials every turn).
- Per-file failures never abort the batch: transient → 2–3 retries; then (and
  for permanent failures) → `needs_human` with a `describe_file` request.
  Retrying updates the **same** row, never duplicates. Extraction failure never
  deletes bytes.
- Pass 2 fires when every file is terminal and runs **only over the parsed** —
  it is told what was skipped (name + reason) so it can flag gaps instead of
  building a confidently incomplete draft.
- **Skipped is never silent**: the chat reply accounts for every file
  («обработано 8 из 10; 2 требуют внимания»). Every file ends in the draft or
  in a visible request — no third bucket.
- An answered request makes the file `parsed`; it **rejoins the next turn** —
  no special path (idempotent merge does the rest).
- Partially invalid patch: keep valid entries, drop broken ones **plus anything
  referencing them**, report what was dropped. Malformed output → one re-ask
  with the validation errors, then a visible chat error; nothing written.
- A parse stuck beyond a timeout is force-failed; if all files fail, pass 2 is
  skipped (unless the operator's text alone is enough).

### Per input type (pass-1 extractors)

| Type | How it is read | Fallback (a request) |
|---|---|---|
| text | passthrough; still evidence, not trusted truth | — |
| url | guarded best-effort fetch → readable text; SSRF-safe, http(s) only, **no recursive crawling in v1**, no headless browser | "paste the text or drop a screenshot" |
| image | downscale in code, one vision call per file with KB context; OCR + visual summary kept separate; product photos → identification/attachment, infographics/nameplates/price cards → facts | "describe what this is" |
| pdf | native text first (cheap); OCR for scanned pages; page-level provenance; page cap ~10 — **large catalogs are chunked, then merged by natural keys** | "which pages matter / paste the key points" |
| docx | text layer read **in code** — no LLM; legacy `.doc` → fallback | "paste the key points" |
| excel / csv | sheets read **in code** → text table — no LLM; the best-structured source for `products[]` | — |
| audio | transcription **ON** (cap ~5 min): transcript + language + summary; **summarize before synthesis** (filler, corrections); respect temporal intent («старая цена 20, новая 25» → only the new value proposed); timestamps kept in provenance | "describe what this is" |
| video | **phased**: v1 = audio-track **transcript-first** + store & assign to owner/role; sampled keyframes optional next; full visual understanding later. Never every frame. | "describe what this is" |

All extractors emit the same evidence shape, so pass 2 is type-blind; upgrading
one extractor never touches synthesis.

**Provider seam:** all model calls go through **one aggregator integration
(OpenRouter, OpenAI-compatible) from day one**; model ids are config, so models
can be added or swapped without touching code. No direct per-vendor SDKs in v1.
Starting defaults and prices: `evals/parsing-costs.md`.

**Cost drivers (relative only; numbers live in `evals/parsing-costs.md`):**
video ≫ audio ≈ scanned-pdf ≈ images ≫ native-pdf ≫ text / url / excel ≈ free.
Cost scales with media detail and duration; a typical mixed batch is cheap
enough that we optimize for extraction quality, not pennies.

### Anti-hallucination & validation

- **Every exact value** (price, count, limit, phone, dimension) reaches a typed
  column **only** through a confirmed `confirm_value` request — **no confidence
  thresholds**. Model confidence may be recorded as metadata; it never gates a
  write. `deletes[]` are confirmed the same way — removing knowledge is as
  protected as adding a price.
- Validation before draft write: every referenced file id exists (this org);
  every owner ref resolves in live, draft, or the same patch; every field
  exists in the target schema; values are well-formed; duplicate refs merge or
  raise `resolve_duplicate`. The model cannot invent tables or columns. Unknown
  anything → that entry is dropped, fail-closed.
- Validation at approve: schema-valid rows, required fields present, referenced
  media exist and are sendable, no open requests on the selected rows.
- Runtime (the brain): placeholders resolved from live typed columns — unknown
  placeholder blocks the reply; media resolved from the derived
  `owner.ref.role` catalog — unknown reference dropped or escalated. The LLM
  never has final authority over ids, paths, prices, counts, or phone numbers.

### Fact value shape

Language-neutral **text** values: «25 000 ₸», «от 5 000», «1–3». Symbols and
ranges allowed; units live in the column name (`delivery_in_days`); word-bearing
values are trusted prose instead. Considered and rejected: numeric
`price_amount` + `price_currency` split — cleaner typing, but cannot hold the
ranges and approximations small sellers actually use.

---

## Open questions

- **Multi-owner media reuse**: is one primary owner per file enough, or is a
  join table needed once real reuse appears?
- **Cleanup policy** for rejected / never-attached uploaded files.
- **Extraction models & data boundary**: which models run in production, and
  the PII / cross-border stance before enabling extraction on real customer data.
- **In-memory queue loses jobs on restart** → re-enqueue materials stuck in
  `uploaded`/`extracting` at startup.
- **KB-size threshold**: at what size does the whole-KB-in-prompt model stop
  working, and what deterministic shortlisting (by category/owner) kicks in then.
