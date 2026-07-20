# Playground and Knowledge Authoring

[`DECISIONS.md`](../DECISIONS.md) is authoritative. The Playground is the
authoring UI for pending knowledge. It may propose and accumulate changes, but
only the explicit draft-to-live approval transaction writes `ai_*`.

## End-to-end lifecycle

```text
1. submit any mix of text, URL, instruction, and files
2. ingest one kbd_materials row per input; persist file bytes first
3. pass 1: parse every material independently and in parallel
4. after all submitted materials are terminal, select the parsed evidence
5. pass 2: synthesize one KB-shaped candidate patch plus workflow requests
6. validate schema, natural keys, exact values, and request-scoped handles
7. atomically merge valid complete entries into the one organization kbd_draft
8. operator reviews field diffs, provenance, visibility, and open requests
9. approve all or selected entries into ai_*; clear those entries and reload brain
```

There is no per-job mini-draft, intermediate approval, generic attachment
assignment, or second manual file-linking step. Consecutive submissions build on
the accumulated draft. The safety boundary is `kbd_draft → ai_*`.

## Ingestion and durable materials

Every operator input becomes its own `kbd_materials` row:

- pasted text or instruction keeps the original in `source_text`;
- a URL keeps the URL in `source_ref` and later stores a guarded readable
  snapshot in `extracted_text`;
- a file row owns its filename, MIME type, checksum, storage adapter locator,
  extraction output, status, and visibility.

File upload succeeds only after bytes and the material row are durable. Parsing
starts afterward. A parse retry updates the same material row and never
re-uploads or duplicates it. No other file registry is introduced.

Statuses follow this lifecycle:

```text
uploaded → extracting → parsed | needs_human | failed
                           ↓
                         built
```

`built` means synthesis consumed the evidence. It does not make the material or
blob disposable: live and draft media references keep it durable. Materials
answered through a request return to `parsed` and join the next normal synthesis
turn.

## Pass 1: understand one material at a time

Each material is parsed in isolation. Its request receives the operator message
and a compact model-safe index of current draft/live natural refs, slugs, and
allowed media columns. It never receives database IDs or storage locators.

Pass 1 emits a common evidence shape on the material row: normalized extracted
text, detected facts with provenance, a “relates to” hint, and, where relevant,
visual/audio summary plus a visibility suggestion. It describes evidence and
does not choose KB tables or produce live/draft entities.

| Input | V1 extraction | Visible fallback |
|---|---|---|
| text | passthrough/normalize | none |
| URL | SSRF-safe HTTP(S) fetch to readable text; no recursive crawl or headless browser | ask for pasted text or screenshot |
| image | downscale; one vision call; keep OCR and visual summary separate | ask the operator to describe it |
| PDF | native text first, OCR scanned pages, page provenance; roughly ten-page direct cap, chunk large catalogs then merge by natural key | ask which pages matter or for key text |
| DOCX | read text in code; legacy `.doc` is unsupported | ask for key text |
| Excel/CSV | read sheets in code into structured text | none |
| audio | transcribe up to roughly five minutes, preserve timestamps, summarize corrections/temporal intent before synthesis | ask for a description |
| video | transcript-first from the audio track; optional sampled keyframes later, never every frame | ask for a description |

All model calls use the single configured OpenAI-compatible aggregator. Model
selection is configuration, and relative/absolute parsing cost assumptions live
in [`evals/parsing-costs.md`](../evals/parsing-costs.md) rather than this
contract.

## Pass 2: synthesize the batch into the real draft schema

Pass 2 is one text-only model call after every submitted material is terminal.
It receives:

- the operator instruction;
- the content of every parsed material selected for this turn;
- a short handle for each material, with attachable `upload.*` handles available
  only to files currently eligible for customer visibility;
- non-attachable `evidence.*` handles for invisible evidence/provenance;
- model-safe views of the full accumulated draft and full live KB, preserving
  business values and semantic media fields but replacing all internal material
  references with request-scoped handles;
- names/reasons for skipped materials so missing evidence cannot be silent.

A handle exists only for that call. It is resolved by a backend sidecar map and
has no meaning in storage or in another request. Pass 2 must not assume one file
equals one KB entity; several uploads may fill different purpose-named columns
on one product, or one file may intentionally be reused by several rows.

The output is a candidate patch in the exact mapped KB shape plus requests. The
example is trimmed for readability; an actual create/update includes every
business column:

```json
{
  "draft_patch": {
    "products": [{
      "ref": "drill-zt40h",
      "name": "ZT-40H magnetic drill",
      "price": "",
      "featured_image": "upload.1",
      "images": ["upload.2"]
    }]
  },
  "requests": []
}
```

Creates and updates are complete business rows; removals are `deletes[]`
markers. There is no separate media operation. The candidate is side-effect-free
while the model runs and is neither stored knowledge nor shown as approved.

## Visibility and attachability

Every file has `auto | invisible | visible` visibility, defaulting to `auto`.
The operator always outranks the model:

- explicit `invisible`: evidence may inform synthesis, but the backend never
  creates an attachable handle for it;
- explicit `visible`: the builder may place its `upload.*` handle in a compatible
  concrete media column;
- `auto`: extraction suggests evidence-only or customer-sendable; an attachable
  handle is created only for a customer-sendable suggestion, and approving a row
  containing it confirms that decision.

An invisible file still contributes its extracted content through an
`evidence.*` provenance handle. Evidence handles are never accepted in media
columns. A file becomes sendable at runtime only when its internal material ID
is in a purpose-named media column on an approved live row and all visibility,
ownership, storage, and MIME checks still pass.

The review UI states the result plainly, for example “used as evidence; never
sent to customers” or `products.sofa-loft.featured_image`. Changing visibility
does not alter live knowledge until approval.

## Deterministic validation before draft write

For each proposed complete entry, backend code verifies:

1. The model-facing section maps to one known `ai_*` table and every field is in
   its closed schema.
2. Required fields and natural refs are present, well-formed, and dot-free.
3. Exact values fit their purpose-named columns; exact facts are not accepted in
   a generic data bag.
4. Every handle exists in the request sidecar and belongs to this organization.
5. Every media handle resolves to a file with a storage locator, compatible MIME
   type, customer-sendable visibility, and permission for that exact column.
6. Evidence, expired, invented, cross-org, non-file, path, URL, and UUID-shaped
   media values are rejected.
7. Duplicate natural refs are deterministically merged or produce a
   `resolve_duplicate` request.

An invalid control value drops its complete entry, plus dependent entries, and
the chat reports the reason. Independent valid entries may still merge. If the
whole output is malformed, the backend re-asks once with deterministic
validation errors; a second failure writes nothing and reports a visible error.

## Atomic draft merge

Validated handles are translated to `kbd_materials.id` values before storage.
One compare-and-swap transaction merges complete entries into the organization's
delta-only `kbd_draft` by natural key and advances `base_version`. A stale
version returns `409 Conflict`. Retrying the same candidate is idempotent.

When an entry already exists in the draft, the new synthesis starts from that
pending entry, not the older live row:

- empty field plus new value: fill it and record provenance;
- same value: keep it and add provenance;
- conflicting exact value: retain a safe pending value and create
  `confirm_value`/`conflict`; never overwrite silently;
- changed trusted prose: update it and expose the field-level diff;
- result identical to live: remove it from the delta to avoid false change
  badges.

The chat narrates each merge. UI state and diffs are derived from draft versus
live, while backend provenance associates source materials with natural
table/ref/field targets. “Reject this upload” is a bulk operation over that
provenance, not a separate mini-draft or approval status.

## Requests and failures

`kbd_requests` is workflow sidecar state, never part of the KB or a customer
prompt. Allowed request types are `confirm_value`, `describe_file`,
`choose_media_column`, `resolve_duplicate`, and `conflict`. Targets use table +
natural ref + field, or a backend material ID. Editing/removing/resolving a
target auto-resolves its request so stale questions cannot survive.

Mixed intent or an unclear instruction is asked directly in chat; it is not a
stored request. Only open requests attached to selected approval entries block
that approval.

Per-material failure does not abort a batch:

- transient extraction errors receive two or three retries;
- permanent or exhausted failures become `needs_human` with a `describe_file`
  request; timed-out extraction is force-failed into the same visible path;
- pass 2 runs over parsed materials and is told each skipped name/reason;
- if all files fail, synthesis is skipped unless submitted text alone is enough;
- the chat accounts for every input (for example, “processed 8 of 10; 2 need
  attention”).

Every submitted material therefore ends either as consumed draft evidence or in
a visible request/error state. There is no silent third bucket.

## Review and approval

Review shows **Новый**, **Изменён**, and **К удалению** states derived from the
current live comparison, field-level diffs, source provenance, file fate, and
blocking requests. The operator can edit, reject, or approve the whole draft or
selected entities.

Approval locks the draft and revalidates selected complete rows, required
fields, every media reference, current file existence/visibility/ownership, and
only the open requests targeting those rows. In one transaction it upserts by
natural key, applies delete markers, removes the approved entries from the blob,
and appends `ai_audit_log`. After commit the backend reloads the live prompt
prefix. V1 deliberately has no KB version history or rollback.
