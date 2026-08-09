# URL Import: Fetch / Puppeteer MCP into `kbd_draft`

[`DECISIONS.md`](DECISIONS.md) is authoritative. This document is an
implementation plan for a capability that does not exist yet, and it proposes
one amendment to that record (§10). It is design intent, not a description of
current code.

## 1. Why

Knowledge reaches `kbd_draft` three ways today: the browser editor
(`/playground/draft/*`), the MCP connector's `kb_*` tools
(`backend/internal/mcpserver`), and direct browser media upload
(`POST /kb/materials`). None of them takes a URL.

[`mcp.md`](mcp.md) §8 names the gap outright — its product-URL flow assumes
*the MCP host* opens the page, and it closes with "Server-side URL fetching is
not part of this initial MCP contract." [`playground.md`](playground.md) and
`DECISIONS.md`'s pass-1 extractor table both describe a URL extractor as
intended v1 work (`source_type='url'`, `source_ref` = the URL,
`extracted_text` = the guarded readable snapshot), and
`kbstore.CreateMaterial` (`materials.go:83`) even carries the `url` branch —
but nothing calls it, and the `internal/playground` package its comments point
at no longer exists. The lane is designed, half-wired, and unbuilt.

This plan builds it. A URL goes in; the page is rendered to Markdown (through
an external Fetch/Puppeteer MCP server when one is configured, through a
pure-Go static renderer otherwise); referenced images are downloaded into
`kbd_materials`; an LLM maps the Markdown onto the canonical KB schemas; the
result is upserted into `kbd_draft` through the existing `kbstore.MCPUpsert*`
functions. The safety boundary is unchanged:

```text
URL import → kbd_draft → human review → live ai_*
```

### Decisions taken

| Question | Decision |
|---|---|
| Endpoint | `POST /xchats/api/v1/kb/import-url`. The one `/kb/*` route that stages into `kbd_draft` instead of writing live — documented as such at the route |
| MCP transport | A minimal Streamable-HTTP JSON-RPC client against a configured URL, plus a compose sidecar; **and** a pure-Go static renderer as the zero-infrastructure default |
| `material_id` format | Bare UUIDs, like every other material. No `mat_` prefix — it appears nowhere in this repo, and adding one would touch the migration, `schema_contract.json`, every media validator, the MCP tool schemas and the frontend types |
| Sync vs async | `202 Accepted`, with the URL's own `kbd_materials` row as the durable job record, and an SSE refresh on completion |

## 2. Shape

```text
POST /kb/import-url {url, target_type}
   └─ validate URL shape; create kbd_materials row
      (source_type='url', source_ref=url, status='extracting')  ──►  202 {material_id}
      │
      └─ goroutine (kbimport.Service.run) — holds no database write lock
         1. urlfetch.Renderer.Render(url)      → markdown, title, image candidates
         2. LLM extraction (markdown + schema) → JSON patch using img.N handles
         3. deterministic validation           → one retry with feedback on failure
         4. download the selected images       → CreateUploadMaterial → blob.Put
                                                 → CompleteMaterialUpload
         5. kbstore.MCPUpsert{Product,Tariff,Topic}(…, MCPProvenance{MaterialIDs})
         6. UpdateMaterialExtraction(job row: markdown + result JSON)
            hub.Broadcast("kb.row.changed")
```

Steps 1–4 are network I/O and must stay outside the draft transaction.
`internal/dbx` opens SQLite with `MaxOpenConns(1)` and `_txlock=immediate`
(see `kbstore/draft.go:387-406`), so a draft write holds the process's single
write lock for its whole duration — a fetch inside a `writeDraftBlobVersioned`
closure would stall every other writer in the application.

## 3. New packages

### `backend/internal/httpsafe`

The SSRF-hardened client from `mcpauth/cimd.go:116-152`, lifted into shared,
exported form. Both the renderer and the image downloader need it.

```go
func Client(allowPrivateHosts bool, timeout time.Duration) *http.Client
func IsPublicIP(ip net.IP) bool
```

Copy that implementation as-is: a `net.Dialer.Control` hook vetting the
*resolved* IP on the initial connection and on every redirect (so a hostname
that rebinds mid-fetch is refused before bytes flow), a five-redirect cap, and
https-only redirects. Gated by the existing `config.KBAllowPrivateFetch`
(`config.go:191`, already wired at `cmd/xchats/main.go:726`), which tests flip
to `true` to reach an `httptest.Server`.

`mcpauth.safeHTTPClient` stays where it is — its own comment argues that
duplication across trust boundaries is deliberate. Collapsing it is optional
follow-up, not a prerequisite.

The submitted URL is shape-checked first with
`publicendpoint.publicHTTPS` (`internal/publicendpoint/endpoint.go:82`), which
already rejects userinfo, query/fragment, `localhost` and loopback literals.

### `backend/internal/urlfetch`

The renderer seam: one interface, three implementations, no database access.

```go
type ImageCandidate struct {
	URL string // absolute
	Alt string
}

type Rendered struct {
	RequestedURL string
	FinalURL     string // after redirects
	Title        string
	Markdown     string
	Images       []ImageCandidate
	RendererName string // "static" | "mcp" — recorded in the job's extraction metadata
}

type Renderer interface {
	Render(ctx context.Context, rawURL string) (Rendered, error)
	Name() string
}
```

- **`staticRenderer`** (default) — `httpsafe.Client` GET, body bounded by
  `io.LimitReader(body, maxPageBytes+1)` with an explicit over-size check, the
  same shape as `FetchCIMD` (`cimd.go:58-64`). Parse with
  `golang.org/x/net/html`, drop `script`/`style`/`nav`/`header`/`footer`/
  `aside`, emit Markdown, resolve every `<img src>`/`srcset` against
  `FinalURL`. `golang.org/x/net` is already in `go.mod` as an indirect
  dependency — promote it to direct rather than taking on a third-party
  converter; this module is kept deliberately lean and ships
  `THIRD_PARTY_LICENSES.txt`.

- **`mcpRenderer`** — a small JSON-RPC 2.0 client over Streamable HTTP.
  Three methods are enough: `initialize`, `tools/list` (once, to confirm the
  configured tool exists), and `tools/call`; it reads the text parts out of
  the result's `content[]`. Hand-rolled on purpose — an MCP SDK is a large
  dependency for three method names, and this module has no MCP client today.
  Tool name and argument mapping are configuration, so one client serves a
  fetch server and a Puppeteer server alike.

- **`chainRenderer`** — `renderer: auto` tries MCP when a URL is configured
  and falls back to static on any error (logged, and reflected in
  `RendererName`). `renderer: static` / `renderer: mcp` pin one.

- An exported `Fake` for `kbimport`'s tests.

### `backend/internal/kbimport`

The orchestrator. Depends on `kbstore.Store`, `blob.Store`, the LLM registry,
`urlfetch.Renderer`, config and a logger — never on `internal/dbx`, which
`dbtest/architecture_test.go` forbids outside the persistence packages.

```go
type Request struct {
	URL        string
	TargetType string // product | tariff | topic | auto
}

type Service struct { /* KB, Blob, Renderer, LLMs, Params, Cfg, Hub, Log */ }

func (s *Service) Start(ctx context.Context, orgID, userID uuid.UUID, in Request) (uuid.UUID, error)
func (s *Service) Status(ctx context.Context, orgID, materialID uuid.UUID) (Status, error)
func (s *Service) SweepStale(ctx context.Context, olderThan time.Duration, limit int) (int, error)
```

Files: `service.go` (lifecycle), `pipeline.go` (the six steps), `schema.go`
(the extraction JSON Schema), `extract.go` (prompt render, parse, validate,
one retry), `media.go` (image download to material), `apply.go` (patch to
`MCPUpsert*`).

## 4. The job record — no new table

`kbd_materials` already carries everything a job needs, and
[`architecture.md`](architecture.md) names the idiom: the row *is* the durable
work item, exactly as `worker.SweepTelegramMedia` (`worker.go:156-160`) treats
an undownloaded attachment.

| Field | Use |
|---|---|
| `source_type` | `'url'` |
| `source_ref` | the submitted URL |
| `status` | `extracting` → `ready` → `built` on success; `failed` / `needs_human` on error |
| `extracted_text` | the rendered Markdown — which is what `DECISIONS.md` already specifies for a url material |
| `extraction` | `{"method","renderer","images_found","images_saved","targets":[…],"skipped":[…],"error":""}` |

`CreateMaterial` (`materials.go:75`) handles the `url` branch but starts at
`status='pending'` and offers no move to `extracting`. Two small additions to
`kbstore/materials.go`:

```go
// StartMaterialExtraction flips a pending url material to 'extracting'.
func (s *Store) StartMaterialExtraction(ctx context.Context, id uuid.UUID) error

// StaleExtractingMaterials returns url materials stuck in 'extracting' past a
// deadline — the restart-recovery input, mirroring store.PendingTelegramMedia.
func (s *Store) StaleExtractingMaterials(ctx context.Context, olderThan time.Duration, limit int) ([]Material, error)
```

The success path finishes through the existing `UpdateMaterialExtraction`
(`materials.go:145`) and then `MarkMaterialsBuilt` (`materials.go:172`) —
`built` is precisely `playground.md`'s "synthesis consumed the evidence".

**No migration, and no `schema_contract.json` change.** Neither `status` nor
`processing_status` carries a CHECK constraint in
`migrations/sqlite/0004_knowledge_base.up.sql` (only the `json_valid` checks
on the JSON columns). This is the strongest reason to route the job through
the material row instead of a new table: a new table would need a migration
*and* a contract update, or `dbtest/contract_test.go` fails.

Restart recovery: `SweepStale` marks anything stuck in `extracting` past
`job_timeout_seconds` as `failed` with a visible reason, started from
`cmd/xchats/main.go` alongside `w.StartTelegramMediaSweeper`. Re-running is
the operator's call — a half-finished import may already have written draft
rows, so silent re-execution would be wrong.

## 5. LLM extraction contract

`llm.ChatClient` (`backend/llm/llm.go:52`) is text-in/text-out;
`llmprovider.OpenAICompatible` sends no `tools`, `tool_choice` or
`response_format`, and says so in its own comment. Structure is therefore
enforced prompt-side and parser-side, exactly as the live response path
already does it.

The model to copy is `response.Engine.Generate`
(`backend/response/engine.go:87-159`): render the schema into the prompt,
complete, classify, retry once with feedback, extract the JSON, validate.
Reuse the call shape at `engine.go:182-196` and wrap it with
`llmprovider.StartTrace` (`tracing.go:66`) for Langfuse parity.

Prompt input: the rendered Markdown (truncated to a token budget), the page
title, `target_type`, the organization's current `IdentityIndex`
(`kbstore/mcp_read.go:57`) so the model reuses an existing `ref`/`slug`
instead of inventing a duplicate, and the numbered image candidates.

Required output:

```json
{
  "target_type": "product",
  "products": [{
    "ref": "coffee-machine-x200",
    "name": "Кофемашина X200",
    "price": "180 000 ₸",
    "description": "…",
    "category": "Кофемашины",
    "in_stock": true,
    "sales_status": "active",
    "featured_image": "img.1",
    "gallery_images": ["img.2", "img.3"]
  }],
  "tariffs": [],
  "topics": [],
  "skipped_reason": ""
}
```

Field names mirror `kbstore.ProductChanges` / `TariffChanges` / `TopicChanges`
(`mcp_write.go:269/355/196`) one for one, so the mapping is mechanical.

**Handle discipline**, per `playground.md`: images are referenced only as
`img.N`, resolved through a request-scoped sidecar map. A raw URL, a UUID, or
an unknown handle in a media field is rejected — the rule that document's
deterministic validation already states.

Validation before any draft write rejects: unknown top-level or per-entity
properties; a `ref`/`slug` containing a dot; `sales_status` outside
`active|inactive`; `pricing_type` outside `fixed|percentage|tiered|hybrid`; a
missing `in_stock` on a new product or `pricing_type` on a new tariff
(`mcp_write.go:316/398` enforce these regardless, but failing early produces a
better retry message); any media value that is not a known `img.N`; and a
`body_md` that trips the topic gate (§6).

On failure: **one** retry with the validation errors appended, mirroring
`aiprompt.RetryFeedback` (`retry.go:68`) and gated by the existing
`LLM_DRAFT_RETRY` setting. A second failure writes nothing and records the
errors on the job row.

`target_type: "auto"` asks the model to choose among `product | tariff |
topic` and to say why in `skipped_reason` when it declines. The backend does
not guess from the URL.

The schema lives in `kbimport/schema.go`. The builders in
`mcpserver/tools.go:107-166` (`obj`, `str`, `enumStr`, `materialIDs`,
`changesObject`, `upsertCommon`) are the right shapes but are unexported and
bound to the MCP tool contract; extracting them into a shared package is
worthwhile follow-up, not a prerequisite.

## 6. The topic gate

`gateTopicBody` (`kbstore/kbstore.go:479`) runs at draft-write time for topics
and rejects a body containing `{{` or a literal currency amount:

```go
var rawCurrencyRE = regexp.MustCompile(`(?:[0-9][0-9 \x{00a0}.,]*\s*(?:₸|₽|€|£|тг|тенге|руб)|[$€£]\s*[0-9])`)
```

Markdown scraped from a commercial page almost always contains a price, so a
naive topic import fails with `*GateError` most of the time. Three parts:

1. The extraction prompt states the rule directly — amounts belong in typed
   columns (`price`, `fee`, `delivery_cost`), never in `body_md`.
2. `kbimport` pre-checks the body with the same rule before calling
   `MCPUpsertTopic`, through a thin exported `kbstore.GateTopicBody(slug,
   bodyMD) []string` delegating to the existing unexported function. A hit
   triggers the one retry with the gate reasons as feedback.
3. If it still fails, drop **only** the topic, keep the products and tariffs,
   record it under `extraction.skipped[]`, and surface it in the response.
   This is `playground.md`'s stated behaviour: an invalid control value drops
   its own entry, and independent valid entries still merge. Partial success,
   never silent.

## 7. Image pipeline

Ordering is render → LLM → download only the selected handles → upsert. A
product page can carry forty images; downloading all of them before knowing
which matter wastes bandwidth and disk, and `blob.Store` has no streaming read
(`blob.go:27`), so every file is fully buffered. The model chooses from alt
text and filename, which is the only signal available either way.

Per selected image, reusing `handleKBUploadMaterial`'s sequence verbatim
(`httpapi/kb_media.go:189-212`):

1. Reject non-`http(s)` and `data:` URIs; dedupe by absolute URL.
2. GET through `httpsafe.Client`, bounded by `mcpUploadMaxBytes`
   (`50 << 20`, `mcp_upload.go:25`) and by `max_image_bytes`.
3. `mimeSanityCheck(declared, data)` — currently unexported in `httpapi`
   (`mcp_upload.go:164`). It is the stored-XSS guard and is not optional;
   move it to a shared home or duplicate it.
4. Reject `kbstore.KindOfMime(mime) == ""` and anything that is not
   `image/*` — a material no validator can classify is permanently
   un-attachable (`kb_media.go:172-179`).
5. `kb.CreateUploadMaterial(ctx, orgID, kbstore.UploadMaterialInput{Filename,
   MimeType, SizeBytes, SHA256Checksum: sumHex, CustomerVisibility: "visible"})`
6. `sha256.Sum256(data)` → `blob.Put(materialID.String()+"-"+sumHex[:16],
   data, blob.Meta{…})` — the content-hash key convention both existing upload
   paths share, so two payloads can never collide on one on-disk key.
7. `kb.CompleteMaterialUpload(ctx, materialID, "disk", storageKey, size, sumHex)`

A single failed image is dropped with a reason and never aborts the batch,
matching the widget's per-file error behaviour in `mcp.md` §6. Caps:
`max_images` (default 12) and `max_image_bytes` (default 10 MiB).

**Attachment is done by the upsert, not by `MCPAttachMedia`.**
`ProductChanges` already carries `FeaturedImage **uuid.UUID` and
`GalleryImages *[]uuid.UUID` (`mcp_write.go:269-275`), and `MCPUpsertProduct`
runs `validateMediaRefs` over them (`mcp_write.go:332`). This matters because
`featured_image` is deliberately **not** an attachment target
(`mcp_media.go:31-34`), so `MCPAttachMedia` would reject it. One upsert call
sets both.

## 8. Provenance

`recordProvenance` (`kbstore/mcp_media.go:509`) inserts a new url material row
for every call carrying `SourceURL`. One URL yielding one product and three
tariffs would leave four near-identical url rows in the Материалы tab.

Avoid it. The job material row created in step 0 *is* the URL provenance
record — it already holds `source_ref` (the URL) and `extracted_text` (the
Markdown). Pass `MCPProvenance{MaterialIDs: <the image materials that record
used>}` and leave `SourceURL` empty. Each image then gets tagged with
`extraction_metadata.mcp_target`, and the URL is recorded exactly once.

Record the per-record targets in the job row's own `extraction` JSON
(`{"targets":[{"type":"product","key":"coffee-machine-x200"}]}`). That also
sidesteps `recordProvenance`'s shallow merge, where `mcp_target` is
last-writer-wins for a material referenced by several records
(`mcp_media.go:464-492`).

Note that url-marker materials are written `customer_visibility='invisible'`
(`mcp_media.go:522`), which `validateMediaRef` refuses to attach to any KB
field (`mcp_media.go:201`). Provenance rows and attachable images are disjoint
populations by design, so a job row can never leak into a customer reply.

## 9. Touchpoints

### New

| Path | What |
|---|---|
| `backend/internal/httpsafe/httpsafe.go` (+ test) | Exported SSRF-hardened client |
| `backend/internal/urlfetch/{urlfetch,static,mcp,chain}.go` (+ tests, `testdata/*.html`) | Renderer seam and its implementations |
| `backend/internal/kbimport/{service,pipeline,schema,extract,media,apply}.go` (+ tests) | Orchestrator |
| `backend/internal/httpapi/kb_import.go` (+ test) | `POST /kb/import-url`, `GET /kb/import-url/:material_id` |
| `frontend/src/components/kb/UrlImportCard.vue` (+ `.dom.test.ts`) | The UI |

### Modified

| Path | Change |
|---|---|
| `backend/internal/httpapi/server.go:413-427` | Register both routes on the existing `kb` group; add a nil-tolerant `kbImport *kbimport.Service` to `Server` and `Deps` (503 when unset), following the `mcpAuth`/`tunnel` precedent |
| `backend/internal/kbstore/materials.go` | `StartMaterialExtraction`, `StaleExtractingMaterials` |
| `backend/internal/kbstore/kbstore.go` | Exported `GateTopicBody` wrapper |
| `backend/internal/httpapi/mcp_upload.go:164` | Export or relocate `mimeSanityCheck` for `kbimport` |
| `backend/internal/config/config.go` | A `KBImportConfig` section mirroring `MCPConfig` (`config.go:105-126`) — every field needs both `yaml:` and `env:` tags, slices need `envSeparator:","` — plus its `defaults()` entry |
| `config.yaml`, `deploy/config.docker.yaml` | The `kb_import:` block |
| `deploy/docker-compose.yaml` | An optional `fetch-mcp` sidecar, commented out by default |
| `backend/cmd/xchats/main.go` | Build the renderer and `kbimport.Service`, pass into `httpapi.Deps`, start the stale-job sweeper |
| `backend/go.mod` / `go.sum` | Promote `golang.org/x/net` to direct, then `make notices` |
| `frontend/src/components/kb/DraftKnowledgeBase.vue:101-104` | `<UrlImportCard />` beside `<McpConnectCard />` |
| `frontend/src/components/kb/DraftKnowledgeBase.dom.test.ts:156-160` | The "a card renders no input" assertion counts inputs across the whole page and will now fail — scope it to the change-list container |
| `frontend/src/types.ts`, `frontend/src/i18n/locales/ru.ts` | Request/status types and `kb.import.*` strings |
| `mcp.md` §8, `DECISIONS.md`, `CHANGELOG.md` | See §10 |

### Schema impact

None. No migration and no `schema_contract.json` change: `kbd_materials` and
`kbd_draft` are used exactly as their existing contracts describe.

### HTTP surface

`POST /xchats/api/v1/kb/import-url`, session- and organization-authenticated
through the standard `s.kbWrite(c)` preamble (`kb_live.go:23`). No `If-Match`:
the import is asynchronous and takes no version token.

```jsonc
// request
{ "url": "https://shop.example/coffee-machine", "target_type": "auto" }
// 202 Accepted
{ "material_id": "…uuid…", "status": "extracting" }
```

`GET /xchats/api/v1/kb/import-url/:material_id`:

```jsonc
{
  "material_id": "…", "url": "…", "status": "built",
  "renderer": "mcp", "markdown_chars": 4821,
  "images_found": 9, "images_saved": 3,
  "targets": [{ "type": "product", "key": "coffee-machine-x200", "created": true }],
  "skipped": [{ "type": "topic", "reason": "literal amount \"180 000 ₸\" in body_md" }],
  "error": ""
}
```

Errors route through `s.kbFail(c, err)` (`playground.go:17`): `*GateError` and
`*ErrMediaReference` to 422, `ErrStale` to 409 `DRAFT_STALE`. Add
`*ErrDuplicateConflict` and `*ErrAmbiguousMatch` to 409 so an identity clash
is not reported as a 500. Bodies use the standard `ok`/`accepted`/`fail`
envelope (`server.go:530-553`).

On completion the pipeline broadcasts `kb.row.changed`; `lib/sse.ts:42`
already binds that event and `stores/playground.ts:386` already
debounce-refreshes on it, so every open draft page updates itself with no new
frontend plumbing.

### Frontend

`UrlImportCard.vue` sits beside `<McpConnectCard />` in the `shrink-0` strip
at `DraftKnowledgeBase.vue:101-104` — above the `loading` / `isEmpty` /
content branches, so it still renders on an empty draft, which is exactly when
someone imports a URL. `McpConnectCard.vue` is the precedent: a self-contained
card with its own API calls and local busy/error refs, bypassing the
playground store.

This does not violate that page's review-only rule
(`DraftKnowledgeBase.vue:2-6`). Like `McpConnectCard`, it is an ingestion
affordance, not an entity form.

Behaviour: URL input and target-type select, submit to
`api.post('/kb/import-url', …)`, then poll `GET /kb/import-url/:id` every
2.5 s — the `AddAccountDialog.vue:99-146` pattern, including
`onBeforeUnmount(stopPolling)` and a terminal-404 guard. On a terminal status
stop polling, render the targets/skipped summary, and call `pg.load()`.

The job also surfaces for free in the Материалы tab:
`KnowledgeBase.vue:234-237` already renders `source_type` and
`processing_status` badges for every material row.

## 10. Amendments to the design record

Two documents contradict this plan and must be updated in the same change,
rather than left disagreeing with the code:

- `DECISIONS.md`'s pass-1 extractor table says URL extraction uses "no
  headless browser". Puppeteer is a headless browser. It stays optional and
  off by default — the Go renderer is the default and the only one that works
  with no extra infrastructure — but the entry needs to say so.
- `mcp.md` §8's closing line ("Server-side URL fetching is not part of this
  initial MCP contract") becomes false. It should point here instead.

## 11. Implementation order

Each stage compiles and passes its own tests.

1. **`internal/httpsafe`** — extract and test the client. Assert `127.0.0.1`
   is refused with `allowPrivate=false` and reachable with `true`.
2. **`internal/urlfetch`, static only** — `Renderer`, `Rendered`,
   `staticRenderer`, `Fake`. Table tests over `testdata/*.html` served by
   `httptest.Server`; assert absolute image URLs, chrome stripping, and the
   body-size cap.
3. **kbstore additions** — `StartMaterialExtraction`,
   `StaleExtractingMaterials`, `GateTopicBody`, tested through
   `dbtest.NewKB(t)` and the `newTestKB` helper (`kbstore_test.go:20`).
4. **`internal/kbimport`, extraction only** — schema, prompt, parse, validate,
   one retry. Pure unit tests against canned model output; no database, no
   network. Cover every rejection: raw URL in a media field, unknown handle,
   dotted `ref`, bad enum, missing `in_stock`, topic gate hit.
5. **`internal/kbimport`, media and apply** — image download and the
   `MCPUpsert*` calls, wired with `dbtest.NewKB` plus
   `blob.NewDisk(t.TempDir())`, the `Fake` renderer and a stub `ChatClient`.
   Assert material rows and checksums, the resulting `kbd_draft` contents,
   provenance tagging, and partial success when the topic is gated out.
6. **HTTP layer** — both routes, `Deps` wiring, the SSE broadcast, error
   mapping, and an `httpapi` integration test that posts a URL end to end with
   a stub renderer and stub LLM.
7. **Composition root and config** — `KBImportConfig`, `main.go` wiring, the
   stale-job sweeper, and the `config.yaml` / `deploy/config.docker.yaml`
   blocks.
8. **MCP renderer** — the JSON-RPC client and `chainRenderer`, tested against
   an `httptest.Server` speaking JSON-RPC 2.0, plus a fallback-on-error case.
   Add the commented `fetch-mcp` sidecar to `deploy/docker-compose.yaml`.
9. **Frontend** — `UrlImportCard.vue`, types, i18n, its `.dom.test.ts`, and
   the fix to `DraftKnowledgeBase.dom.test.ts`'s input assertion.
10. **Docs** — §10's amendments and `CHANGELOG.md`; run `make notices` for the
    `x/net` promotion.

## 12. Verification

Offline and deterministic, the way the suite already runs:

```bash
make test-backend     # cd backend && go test -race -count=1 ./...
make lint-backend
make test-frontend    # typecheck + vitest + build
```

Two tests are the ones a mistake here would trip, so check them explicitly:
`dbtest/architecture_test.go` (no `dbx`, `database/sql` or `modernc.org/sqlite`
outside the persistence packages) and `dbtest/contract_test.go` (schema
unchanged).

Live:

```bash
make migrate && make seed && make dev-backend   # :8080
make dev-frontend                               # :5173
```

1. Sign in, open `/playground`, import a real product page. The card shows
   "extracting", then a summary; the draft populates; the images appear on the
   product card's media strip.
2. Confirm the URL row under `/knowledge-base` → Файлы, with `source_type=url`
   and the rendered Markdown as its extracted text.
3. Publish from the draft; confirm the product lands in `ai_products` with its
   `featured_image` / `gallery_images` resolving.
4. SSRF: submit `http://127.0.0.1:8080/` and expect a validation error while
   `kb_allow_private_fetch` is false.
5. Topic gate: import a page with prices as `target_type: topic`; expect the
   topic skipped with a visible reason while products still land.
6. MCP path: run a fetch MCP server, set `kb_import.mcp_url`, re-import a
   JavaScript-heavy page, confirm `renderer: "mcp"` in the status payload, and
   confirm a graceful fallback to `static` when the sidecar is stopped.

## 13. Risks

- **Prompt injection.** The fetched page is untrusted text driving an LLM
  whose output drives draft writes. The mitigations are structural and already
  in place: the model never sees material UUIDs or storage keys, media is
  referenced only through `img.N` handles, every value is deterministically
  validated, and the draft still requires human approval before reaching
  `ai_*`. State this explicitly in the package doc comment.
- **Blob memory.** Every image is buffered whole (`blob.Store` has no
  streaming read). At `max_images: 12` × `max_image_bytes: 10 MiB` the worst
  case is roughly 120 MiB per concurrent import; a per-process semaphore on
  concurrent imports is worth considering.
- **Extraction quality is unmeasured.** `evals/` covers the response path,
  nothing covers KB extraction. A scenario set under `evals/scenarios/` is the
  honest way to tune the prompt; this plan ships without one.
- **`recordProvenance`'s `mcp_target` is last-writer-wins** for a material
  referenced by several records. §8 sidesteps it, but the limitation remains
  for MCP callers.
- **`internal/queue` is deliberately unused here.** It has no retries and no
  durability (`queue.go:105` drops a message on handler error), so a goroutine
  plus the material row plus the sweeper is strictly better. If imports ever
  need capped retries, `internal/automation`'s dispatch-job pattern is the
  model — and that would then require a migration.
