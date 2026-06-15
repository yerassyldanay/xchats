# TODO — Implement the AI Assistant Suggestion (real brain)

## Goal

Replace the hardcoded `Stub` drafter with a **real, KB-grounded brain** so that pressing
"Подсказать ответ" produces a genuine reply suggestion (text + optional media) drawn from a
Knowledge Base. The KB content and media files are **invented here** (a small demo business) — no
external data needed.

Scope decisions (locked):
- **One option** per suggestion to start (the UI already renders 1–N cards).
- **Text + media**: the brain may attach catalog media by ref; we ship the media files and serve them.
- **No chat profile**: send only the last ~15 messages; the model infers from them.
- **Embedded KB** (in-memory snapshot at boot) — no `ai_*` DB tables yet; swap to DB later behind the
  same `ContentSource` seam.
- **Stub fallback**: if `LLM_API_KEY` is unset, keep the Stub so the app still runs.

## Current state (already built — reuse, don't rebuild)

- `internal/assistant/assistant.go` — the `Drafter` interface (`Draft(ctx, Input) ([]Option, error)`)
  + `Input`, `Option`, `Media`, and `NewStub` (embeds sample media → blob store, keyed by ref).
- `internal/worker/worker.go` `handleAIDraft` — calls `Drafter.Draft`, writes via `WriteDraftSet`,
  broadcasts `ai_draft.created`.
- `internal/httpapi/drafts.go` — `handleSuggest` / `handleListDrafts` / `handleApprove` (approve
  sends text + `media_ids` via `sendParts`).
- `internal/config/config.go` — two-file config (yaml + env), `caarlos0/env` tags.
- Brain to port: `plan/examples/repos/xpayment-crm/internal/{domain,usecase/assistant,infrastructure/llm}`.

## The invented Knowledge Base (demo business: "Demo Shop")

A small online shop assistant. Put this content in the seed file (step 4). Prices/numbers/contacts
are **value tokens**, never digits in topic bodies.

- **Persona**: friendly, concrete sales assistant for an online shop; explains simply; no hard-selling.
- **Guardrails**: never invent prices; if asked about refunds/legal/an angry customer → escalate;
  always offer a next step.
- **Language policy**: reply in the customer's language; KK+RU mix → Russian.
- **Topics** (`slug`, lang, body with tokens):
  - `pricing` (ru): three plans — Базовый `{{price.basic}}`, Стандарт `{{price.standard}}`,
    Премиум `{{price.premium}}`; delivery `{{delivery.days}}`.
  - `delivery` (ru): delivery time `{{delivery.days}}`, free over `{{price.free_delivery_min}}`.
  - `how_to_order` (ru): steps to order via WhatsApp.
  - `contacts` (ru): WhatsApp `{{contact.whatsapp}}`, e-mail `{{contact.email}}`, address
    `{{contact.address}}`.
- **Value book** (token → value, with lang; `*` = language-neutral):
  - `price.basic` = "9 900 ₸", `price.standard` = "19 900 ₸", `price.premium` = "39 900 ₸"
  - `price.free_delivery_min` = "30 000 ₸", `delivery.days` (ru) = "1–3 дня"
  - `contact.whatsapp` (*) = "+7 700 123 45 67", `contact.email` (*) = "hello@demoshop.kz"
  - `contact.address` (*) = "Алматы, ул. Абая, 10"
  - (The ported `PriceBook` uses `price.<key>`/`limit.<key>` for tariffs and `ns.key` placeholders for
    the rest — map the above onto Tariffs + Placeholders, or extend it; see step 2 note.)
- **Media catalog** (asset `ref` | kind | topic | description) — files shipped in step 5:
  - `pricing_card` | image | pricing | "Карточка с тремя тарифами и ценами (RU). Для вопросов о цене."
  - `catalog_pdf` | document | pricing | "PDF-каталог товаров с ценами. Когда просят подробности."
  - `intro_audio` | audio | how_to_order | "Голосовое: как оформить заказ за минуту."

## Implementation steps

### 1. Port the brain core into `backend/internal/brain/`
- [ ] `internal/brain/domain/` ← copy `domain/{content,draft,message,catalog}.go` (module path
  `github.com/yessaliyev/xpayment-crm` → `github.com/yerassyldanay/xchats/backend`). Drop the unused
  `ChatID` type from `message.go` (keep `Message`, `Role`).
- [ ] `internal/brain/prompt.go` (package `brain`) ← `usecase/assistant/{prompt.go + the postProcess
  pipeline from brain.go + errors}`. Add a `Prompt{System,User}` type. **Modify `BuildUser`** to omit
  the PROFILE block when no profile is given (we pass none). Export `PostProcess(raw, snap, log)`.
- [ ] `internal/brain/llm/openrouter.go` (package `llm`) ← copy; repoint `assistant.Prompt` →
  `brain.Prompt` and the domain import. Keep the forced `emit_draft` tool + defensive JSON parse.

### 2. Knowledge-base value tokens
- [ ] Reuse the ported `PriceBook.Render` (`price.*`/`limit.*` → tariffs; `ns.key` → bilingual
  placeholders). Map the demo value book onto `Tariffs` (basic/standard/premium with a price; limits
  optional) + `Placeholders` (delivery, contacts). If a non-tariff numeric token is needed, add it as
  a placeholder. (Keeps the port faithful; the doc-level `ai_values` rename is a later DB concern.)

### 3. LLM config
- [ ] Add to `internal/config/config.go`: `LLMProvider` (`env:"LLM_PROVIDER"`), `LLMAPIKey`
  (`env:"LLM_API_KEY"`), `LLMBaseURL`, `LLMFastModel`, `LLMMaxTokens`, `LLMTemperature` + defaults
  (provider `openrouter`, model `openai/gpt-4o-mini`, maxTokens 1024, temp 0.3) + a
  `LLMResolvedBaseURL()` helper (openrouter/openai/gemini base URLs; `LLMBaseURL` overrides).
- [ ] Document the keys in `.env.example` / `config.example.yaml`.

### 4. Embedded seed snapshot
- [ ] `internal/brain/seed.go` — `func SeedSnapshot() *domain.Snapshot` returning the "Demo Shop" KB
  above (Config + PriceBook + Topics + Assets). Asset URLs = `/xchats/api/v1/media/<ref>`.

### 5. KB media files
- [ ] Add `internal/brain/kb-media/` with the three assets (`pricing_card.png`, `catalog_pdf.pdf`,
  `intro_audio.*`) — generate simple placeholder files (a rendered pricing PNG, reuse the existing
  `sample-doc.pdf` shape, a short audio). `//go:embed kb-media/*`.
- [ ] A loader (mirror `NewStub`): on boot, `blob.Put(ref, bytes, Meta{...})` for each asset so
  approve→`sendParts` can serve/send them by ref.

### 6. RealDrafter behind `assistant.Drafter`
- [ ] `internal/assistant/real.go` — `RealDrafter{store, llm, snap, log}` implementing
  `Draft(ctx, Input) ([]Option, error)`:
  - parse `Input.ChatID`; `store.MessagesForChat(ctx, chatID, time.Time{}, 15)` → window.
  - current = latest `Direction=="in"` message (fallback `Input.LastInboundText`); window = the rest,
    mapped to `domain.Message` (in→customer, out→agent).
  - `brain.BuildSystem(snap)` + `brain.BuildUser(nil, window, current)` → `llm.Draft` → on error,
    return one escalation option (holding reply); else `brain.PostProcess` → map `domain.Draft` to one
    `Option` (Text, Confidence, Escalate, Reason, **Media** from `ResolveAssets`).
- [ ] `NewReal(store, llmClient, snap, log)`.

### 7. Wire it up
- [ ] `cmd/xchats/main.go`: if `cfg.LLMAPIKey != ""` build `llm.New(cfg.LLMResolvedBaseURL(), key,
  fastModel, "", maxTokens, temp)` + load KB media into blob + `assistant.NewReal(st, lc,
  brain.SeedSnapshot(), log)`; else keep `assistant.NewStub`. Pass the chosen `Drafter` to both the
  worker and the HTTP server (as today). Log which drafter is active.

### 8. Build, test, verify
- [ ] `go build ./...` and `go vet ./...` in `backend/`.
- [ ] Port `usecase/assistant/brain_test.go` → `internal/brain` as a parity check (escalation stops
  pipeline, refs resolved/dropped, value render failure → manual note). Use a fake `llm` Drafter.
- [ ] Manual e2e: set `LLM_API_KEY` (+ provider/model), run backend+frontend, open a chat with an
  inbound like "Сколько стоит?", press "Подсказать ответ" → a grounded card appears with the real
  price injected (`{{price.*}}` → "… ₸") and, when relevant, an attached catalog asset.
- [ ] Off-KB question (e.g. "Вы возвращаете деньги?") → the option correctly escalates.
- [ ] Unset `LLM_API_KEY` → app still boots on the Stub.

## Out of scope (separate)
- The realtime multi-user "generating" signal (see `~/.claude/plans/1-we-must-add-cuddly-diffie.md`).
- `ai_*` KB tables + authoring CMS (the snapshot stays embedded for now).
- The `ai_suggestions` jsonb storage refactor (current per-option `ai_drafts` storage is untouched).

---

## Status — implemented (this branch)

Steps 1–7 done; step 8 done except the two checks that need a live `LLM_API_KEY`.

- [x] **1. Brain ported** → `backend/internal/brain/`: `domain/{content,draft,message,catalog}.go`
  (module path rewritten; `ChatID` dropped from `message.go`), `prompt.go` (package `brain`: `Prompt`
  type, `BuildSystem`, `BuildUser` **with the PROFILE block omitted when none**, exported `PostProcess`,
  the consts + `ErrNoPublishedConfig`), `llm/openrouter.go` (package `llm`, repointed to `brain.Prompt`
  + `brain/domain`, forced `emit_draft` tool + defensive parse).
- [x] **2. Value tokens** → reused the ported `PriceBook.Render`; demo book mapped onto `Tariffs`
  (basic/standard/premium/free_delivery_min) + `Placeholders` (delivery/contacts).
- [x] **3. LLM config** → `config.go` gains `LLMProvider/LLMAPIKey/LLMBaseURL/LLMFastModel/
  LLMMaxTokens/LLMTemperature` + defaults (openrouter, `openai/gpt-4o-mini`, 1024, 0.3) +
  `LLMResolvedBaseURL()`. Documented in `.env.example` / `config.example.yaml`.
- [x] **4. Embedded seed** → `brain/seed.go` `SeedSnapshot()` = the "Demo Shop" KB (config + price book
  + 4 topics + 3 assets); asset URLs `/xchats/api/v1/media/<ref>`.
- [x] **5. KB media** → `brain/kb-media/{pricing_card.png,catalog_pdf.pdf,intro_audio.wav}` (placeholder
  bytes) `//go:embed`-ed; `brain.LoadMedia(blob.Store)` mirrors `NewStub` (ref == blob id).
- [x] **6. RealDrafter** → `internal/assistant/real.go`: reads the last 15 msgs via
  `store.MessagesForChat`, builds the prompt (no profile), calls the LLM, `PostProcess`es, returns ONE
  option (text + resolved media); LLM error → one escalation option (holding reply). `NewReal(...)`.
- [x] **7. Wired** → `main.go buildDrafter`: `LLM_API_KEY` set → load KB media + `assistant.NewReal`
  (logs `mode=real`); else `NewStub` (logs `mode=stub`). Same `Drafter` passed to worker + HTTP server.
- [x] **8. Build/test** → `go build ./...` + `go vet ./...` clean; parity tests pass —
  `internal/brain/prompt_test.go` (escalation stops, refs resolved/dropped, price-render failure →
  manual note, seed tokens resolve) + `internal/assistant/real_test.go` (grounded option, LLM-error
  escalation, bad chat id) with a fake LLM + fake window store. **No live LLM used.**
- [ ] **8. Manual e2e with a real key** — pending: set `LLM_API_KEY`, ask "Сколько стоит?" → grounded
  card with injected price + catalog asset; off-KB "Вы возвращаете деньги?" → escalates; unset key →
  boots on the stub. (Can't run here — no key in this environment.)

> Note on the port source: the `xpayment-crm` submodule is unreachable from this environment (SSH/proxy
> blocked), so the brain was ported from a **direct HTTPS clone** of the same repo — the literal
> copy + module-path rewrite the todo describes, not a reconstruction.
