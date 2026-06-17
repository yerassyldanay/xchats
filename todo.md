# Knowledge-Base UI — Конструктор (builder) + Редактор (editor)

## Context

Drafts in the inbox feel "the same / generic" not because the LLM is stubbed — it
isn't (backend logs `mode=real`; the DB holds varied, KB-grounded replies). They
feel generic because **~54% escalate** ("я передам вашей команде"): the live KB is
only the **seed snapshot (v1)**, so the model has nothing to ground on. The fix is
to **enrich the KB**, which needs the **Playground / Knowledge-Base UI — which does
not exist on the frontend yet** (the backend is built + integration-tested).

Two pages, from the designs ([plan/ui/ai-playground.png](plan/ui/ai-playground.png),
[plan/ui/ai-knowledge-base.png](plan/ui/ai-knowledge-base.png)):

| Page | Route | Nav label | What it does |
|---|---|---|---|
| **Конструктор базы знаний** | `/playground` | Конструктор | Chat-like builder: upload materials + talk to AI + answer "Запросы AI" popups |
| **Редактор базы знаний** | `/knowledge-base` | База знаний | Tabbed editor: Обзор · Темы · Медиа-ресурсы · Значения · Правки |

**Single KB — no versions.** There is exactly **one** knowledge base: you edit it and
the AI-assistant brain reads that same KB. **No version history, no rollback, no
multi-version list.** Internally we reuse the backend's existing "edit working copy →
apply" path (its publish path, **with version semantics hidden** — the version field
just stays put and is never shown). The editor/builder mutate the working copy; one
**"Сохранить в базу"** action writes it to the live KB and reloads the brain.

**Locked decisions (2026-06-17):**
- **Builder brain = Simple ingest (RuleSynthesizer, ship now).** No conversational LLM
  builder in v1. Page 2 = upload media/text → fold into KB topics/values via the existing
  deterministic synthesizer. **G2 is dropped for v1; Phase B is a no-op.**
- **Publish = explicit apply button.** Edits stage in the working copy; **"Сохранить в
  базу"** is the single action that writes to the live KB + reloads the brain. No
  instant-live mode.
- **Images = true background.** Enable `LLM_VISION_MODEL` so image uploads auto-caption
  with **no `describe_media` popups**. This is **required**, not optional.
- **docs-first** (spec before build), **match the images** for routes/labels,
  **single KB / no versioning**.

**One-button flow (resolve the "update store" mental model).** "Сохранить в базу" only
**publishes**; it is NOT a single call that ingests+synthesizes+publishes. The chain is:
composer attach/text → `POST /materials` (auto-caption via vision) → the builder folds
ready materials into draft rows (`POST /chat` with a default "ingest" instruction, fired
automatically when a material finishes extraction) → user clicks **"Сохранить в базу"** →
`POST /publish`. The UI must make this read as "drop stuff in, then one Save" even though
it's three endpoints under the hood.

---

## Backend readiness

**Ready — no work needed** (routes in [server.go:131-151](backend/internal/httpapi/server.go#L131),
handlers in [playground.go](backend/internal/httpapi/playground.go) +
[playground_ingest.go](backend/internal/httpapi/playground_ingest.go)):

- Draft lifecycle — `GET/POST/DELETE /playground/draft`; full `DraftView` returns
  `config{version,persona,mission,guardrails,language_policy,reply_max_words,updated_at}`,
  `topics[]`, `assets[]`, `values[]`, `materials[]`, `requests[]` — all with `review_state`,
  `provenance`, `updated_at` ([draft.go:14-73](backend/internal/kbstore/draft.go#L14)).
- Topics — `POST/DELETE /playground/draft/topics[/:slug]`.
- Assets (media + meta) — `POST` (multipart) / `PATCH /:ref` / `DELETE /:ref`.
- Values — `POST` / `DELETE /:token` `/playground/draft/values`.
- Config — `PATCH /playground/draft/config`.
- Review — `POST /playground/draft/review/:kind/:id {state:approved|rejected}`.
- Materials — `POST` (text/url/file) / `GET /playground/draft/materials`; non-text enqueues extraction.
- Builder chat — `POST /playground/chat {instruction}` → `{result, draft}`.
- Requests / popups — `GET /playground/requests`, `POST /playground/requests/:id/resolve` (`confirm_value`, `describe_media`).
- Apply to live KB — `POST /playground/publish` (relabel UI "Сохранить в базу"); writes the working copy to the single KB, reloads the brain, broadcasts `kb.published`. **Version semantics hidden; `rollback` and version listing intentionally unused.**
- Optimistic concurrency — `If-Match` (draft `updated_at`) → `DRAFT_STALE` 409.
- Media — `POST /media`, `GET /media/:id`. SSE — `kb.material.updated`, `kb.row.changed`, `kb.published`.
- Derivable client-side (no endpoint needed): "Последние изменения" (from row `updated_at`/`provenance`),
  "Готовность к публикации" (from `review_state` counts).

**Gaps — backend:**

- *(G1 — version-history listing) — **dropped.** No versioning per decision above.*
- *(G2 — LLM-backed builder synthesizer) — **dropped for v1.** Locked decision: ship with
  the deterministic `RuleSynthesizer` ([builder.go:233](backend/internal/playground/builder.go#L233);
  `main.go:110` keeps passing `nil`). It folds ready material text into a topic +
  regex-detects ₸ prices — functional, no conversation. The Конструктор UI must therefore
  present as "upload + ingest", NOT a chat with an LLM persona. Revisit G2 post-v1.*

---

## Phase A — Docs first: rewrite [plan/5-ui-pages.md](plan/5-ui-pages.md) §6

Replace the outdated single "AI Assistant — `/assistant` *(deferred)*" section with **two
page specs**, in the doc's existing format (ASCII sketch · visibility tiers · **"▸ Backed
by:"** endpoints per region · UI-stub call-outs · image prompt). Also update the intro
("v1 ships … pages") and the §2 NavRail note to mention the two added rail icons.

- [ ] **§6 — Конструктор базы знаний — `/playground`**
  - Header: title + "Соберите знания в диалоге с AI"; **Сохранить в базу** / **Отменить изменения**. ▸ `GET/POST/DELETE /playground/draft`, `POST /playground/publish` (apply, no version shown).
  - Chat thread: operator turns + **system "Добавлено в базу" confirmations** (NOT an LLM persona — RuleSynthesizer is deterministic), material-upload bubbles + thumbnails, a "Что добавилось" summary of the rows the ingest produced. ▸ `POST /playground/chat` (default ingest instruction). **No conversational quick-replies — the model doesn't talk back.**
  - Composer: text + attach (`Paperclip`) → materials. ▸ `POST /playground/draft/materials`; live `kb.material.updated`.
  - Right rail: "Обзор базы знаний" tiles, "Запросы AI" popups, "Последние изменения", readiness. ▸ `GET /playground/requests`, `POST /requests/:id/resolve`; counts from draft view.
- [ ] **§6b — Редактор базы знаний — `/knowledge-base`**
  - Header: title + "Управляйте темами…"; **Сохранить в базу** (no История, no версии).
  - Stat cards: Темы / Медиа-ресурсы / Значения / Правки. ▸ counts from draft view.
  - Tabs: **Обзор · Темы · Медиа-ресурсы · Значения · Правки** — each region lists its backing endpoint (topics/assets/values CRUD, review). *(No История tab.)*
  - Right rail: "Быстрый доступ", "Последние изменения", "Готовность к публикации". ▸ draft view + `kb.row.changed`.
  - **Review (Правки tab) is informational, not a hard gate** — "Сохранить в базу" publishes the working copy regardless of `review_state`; approve/reject just lets the user curate. (User did not request an approval gate.)
  - **Vision required (locked):** enable `LLM_VISION_MODEL` so images auto-caption; `describe_media` popups should be the rare fallback, not the norm.
- [ ] Verify §6 against both PNGs (ignoring the version/history UI in the mockups, which we're dropping): every kept element has a "▸ Backed by" endpoint or an explicit stub note; routes/labels match (`/playground`=Конструктор, `/knowledge-base`=База знаний). **User reviews the doc before any code.**

---

## Phase B — Backend — **NO-OP (G2 dropped)**

Locked: ship with `RuleSynthesizer`. No version/history work, no LLM synthesizer. The
existing apply path + deterministic ingest cover v1. **The only backend-adjacent change is
config**, handled in Phase C: enable `LLM_VISION_MODEL` in `.env` **and** the compose
backend block so image auto-captioning works (this is required, see Phase C).

---

## Phase C — Frontend build (Vue 3 + shadcn-vue, Russian UI)

- [ ] **Plumbing** — `postForm<T>(path, FormData)` in [client.ts](frontend/src/api/client.ts) (asset/material multipart with extra fields); add KB SSE events (`kb.material.updated`, `kb.row.changed`, `kb.published`) to [sse.ts](frontend/src/lib/sse.ts).
- [ ] **`stores/playground.ts`** — one method per `/playground/*` endpoint; shared `draft`/`materials`/`requests` state; `lastUpdatedAt` → `If-Match`; realtime refresh. Both pages share this store.
- [ ] **`views/Playground.vue`** (Конструктор) — chat + materials + requests, per §6.
- [ ] **`views/KnowledgeBase.vue`** (Редактор) — tabbed editor **Обзор/Темы/Медиа/Значения/Правки** (no История — versioning dropped), per §6b.
- [ ] **Route + nav** — add `/playground` + `/knowledge-base` to [router.ts](frontend/src/router.ts); two rail icons in [NavRail.vue](frontend/src/components/NavRail.vue) (`MessagesSquare` "Конструктор", `Library` "База знаний").
- [ ] **Vision (required)** — enable `LLM_VISION_MODEL` in [.env](.env) **and** add it to the compose backend block ([docker-compose.yaml:42-48](deploy/docker-compose.yaml#L42)); verify an image upload produces an auto-caption (no `describe_media` popup).

---

## Verification (closes the original problem)

1. `make test-frontend` (typecheck + build) + `make test-e2e` (playground integration) green.
2. Rebuild (`make up`), open `:8081`, hard-refresh, log in.
3. **Конструктор** → Open draft → add a text material + upload an image → resolve the `describe_media` popup → run a builder-chat instruction → confirm a `confirm_value` popup.
4. **Редактор** → Темы/Медиа/Значения populated → approve in **Правки** → **Сохранить в базу**; confirm `brain KB reloaded` in `make logs` + `kb.published` SSE.
5. **Inbox → Regenerate** a draft → it now reflects the richer KB with fewer escalations. ✔ original "same/generic" complaint resolved.

---

# (Archived) Frontend visual overhaul — shadcn-vue + Linear-style minimal

## Context

The xchats frontend (Vue 3 + Vite + TS + Tailwind 3.4) is **well-built code** but
reads as "cheap / school-project" because of a few *visual* choices, not architecture:
default Tailwind indigo `#4F46E5` + bright WhatsApp green, gradients everywhere
(logo, login panel, avatars), FontAwesome CDN icons, and oversized radii
(`rounded-2xl`/`3xl`) — the "glittery candy" look.

Goal: make it look like a premium SaaS tool (Linear/Front/Missive class) by adopting
**shadcn-vue** (Reka UI + Tailwind, accessible copy-paste components we own) as the
design system, themed in a **Linear-style minimal** direction: refined cool neutrals,
one confident accent used sparingly, tight radii, hairline borders over heavy shadows,
crisp dense type, **no gradients**. WhatsApp green is retained ONLY for message bubbles,
the WA glyph, and the "connected" status dot.

Stack stays on **Tailwind v3** (not v4) — shadcn-vue **v3 convention**: HSL CSS variables
mapped in `tailwind.config.js`, `cssVariables: true`, `baseColor: slate`, default style,
`tailwindcss-animate`.

**Critical version note (resolve before starting).** As of late 2025 shadcn-vue's
`@latest`/docs went **v4-only** (OKLCH colors, `@theme` CSS-first, `new-york` default
style, `tw-animate-css` instead of `tailwindcss-animate`, `data-slot`). This entire plan
is written against **v3**, which is the correct target for an existing v3 project. To
avoid the v4 conventions silently overriding the plan:
- **Pin the CLI to a v3-era `shadcn-vue`** (do NOT use `@latest`). Verify the resolved
  version targets v3 before running `init`.
- Follow **v3.shadcn-vue.com** for ALL component snippets / `add` flows — current docs are v4.
- **Inspect `init` output before committing.** If it scaffolds `@theme`/OKLCH tokens or
  pulls `tw-animate-css`, it went v4 — abort, re-pin, and redo. The plan expects HSL vars
  in `tailwind.config.js` + `tailwindcss-animate`.

The app must keep working and building at every step. Legacy tokens + the
`@layer components` bridge stay live until each consumer is migrated; removed only in
final cleanup. Commit after each screen so regressions are bisectable.

## Phase 1 — Setup (zero visual change)

- **Path alias `@/` → `src`** in BOTH (must match or typecheck breaks):
  - `frontend/vite.config.ts`: add `resolve.alias` via `fileURLToPath(new URL('./src', import.meta.url))` (keep `server.proxy` + `build`).
  - `frontend/tsconfig.json`: add `"baseUrl": "."` and `"paths": { "@/*": ["./src/*"] }`.
- **Deps** — runtime: `reka-ui`, `class-variance-authority`, `clsx`, `tailwind-merge`, `lucide-vue-next`, `@vueuse/core`. dev: `tailwindcss-animate`.
- **`cn()` util** → `frontend/src/lib/utils.ts` (clsx + twMerge).
- **`shadcn-vue init` (pinned v3 CLI, not `@latest`)** with `components.json`: framework
  `vite`, `style: default`, `tailwind.config: tailwind.config.js`, `css: src/style.css`,
  `baseColor: slate`, `cssVariables: true`. Confirm output is HSL + `tailwindcss-animate`
  (see Critical version note). **Merge, don't overwrite** its edits against existing
  `theme.extend` tokens, `boxShadow`, `fontFamily`, and the `@layer components` block +
  scrollbar CSS in `style.css` — review the diff and re-add anything dropped.
- Gate: `npm run typecheck && npm run build && npm run dev` all green, no visual change.

## Phase 2 — Theme tokens (Linear minimal)

- Merge HSL `:root` (light) + `.dark` blocks into `frontend/src/style.css`; set
  `darkMode: ['class']` and the `border/input/ring/background/foreground/primary/...`
  color mappings + `borderRadius: var(--radius)` in `tailwind.config.js`.
- Token intent (space-separated H S% L%): cool near-white `--background`, ink
  `--foreground 222 47% 11%`, `--primary 243 75% 59%` (reuse `#4F46E5` so accent doesn't
  shift), `--muted-foreground 215 16% 47%`, hairline `--border 220 16% 91%`,
  `--ring = --primary`, **`--radius: 0.5rem`** (tight). `.dark` = deep cool charcoal
  (`--background 224 30% 8%`), lifted primary `243 80% 67%`.
- Add shadcn tokens **alongside** legacy `brand/wa/ink/muted/panel/hair/rail` — inert
  until reskin references `bg-background`/`text-foreground`/`border-border`.
- **Dark-mode wiring (default light):** set `document.documentElement.classList` from a
  localStorage pref in `main.ts`/`App.vue`. No UI toggle required for v1, BUT **actually
  toggle `.dark` on `<html>` in devtools on every screen during Phase 4** — otherwise dark
  tokens rot silently until someone enables them. This is a verification obligation, not optional.
- Gate: build green, still no visual change.

## Phase 3 — Primitives (add + verify one at a time)

`shadcn-vue add <name>` (pinned v3 CLI, NOT `@latest`) → lands in `src/components/ui/<name>/`.
Minimum set: **Button, Input, Textarea, Dialog, DropdownMenu, Select, Badge, Avatar,
Skeleton, Tabs, Tooltip, Separator** (ScrollArea optional — existing scrollbar CSS already clean).

- Button replaces `.btn*`/`.icon-btn` (variants: default=primary, outline=ghost,
  secondary, ghost, destructive, `size=sm|icon`). **Decision (up front, not per-screen):
  the send button uses `default` (primary), NOT a green `wa` variant** — a green send
  button reintroduces the exact two-accent look we're removing. Green stays confined to
  message bubbles, the WA glyph, and the connected dot. Do not add a `wa` cva variant.
- **After adding EACH component run `npm run typecheck`** — generated `ui/` files can
  carry unused imports that fail `vue-tsc --noEmit` in `build` while `dev` passes. This is
  the single most likely CI break; prune unused symbols immediately.

## Phase 4 — Per-screen reskin (simplest first, commit each)

Swap legacy→semantic classes per screen: `bg-white/bg-panel`→`bg-background/bg-card`,
`text-ink`→`text-foreground`, `text-muted/text-slate-*`→`text-muted-foreground`,
`border-hair`→`border-border`, `bg-brand/text-brand/bg-brand-soft`→`bg-primary/text-primary/bg-accent`,
`ring-brand/10`→`ring-ring`; downshift `rounded-2xl/3xl`→`rounded-lg/xl`; kill gradients
(logo `from-indigo-500 to-violet-500`→solid `bg-primary`; login orbs/panel→flat). Swap
FontAwesome `<i>`→`lucide-vue-next` components inside each screen (spinners→`<Loader2 class="animate-spin"/>`).
**Preserve all Russian labels and emit/store contracts verbatim.**

Order:
1. **Login.vue** — validates Button/Input/icon/token pipeline on a low-risk screen.
2. **AddAccountDialog.vue + NewMessageDialog.vue** — to Reka Dialog + Select + Input/Textarea;
   keep `@close`/`@connected` emits via `@update:open` and AddAccount's QR-polling lifecycle.
3. **NavRail.vue** — DropdownMenu (user menu), Avatar, `WhatsappIcon`; keep `w-[68px]`.
4. **ChatList.vue** — Input search, Tabs/segmented filter, Select (account), Avatar, Badge
   (unread), FAB→Button `size=icon`; keep `w-[340px] border-r`.
5. **ChatThread.vue** — header Buttons + DropdownMenu, Avatar, bubbles (**keep `bg-wa`
   out-bubbles**, in-bubbles→`bg-card border-border`), `tick()` refactor.
6. **Composer.vue** — Textarea (keep `v-autosize`), send Button, icon buttons.
7. **AssistantPanel.vue** — Skeleton (shimmer), Badge (confidence), Textarea (draft,
   autosize), Button actions, `mediaMeta` lucide map; kill header gradient; keep `w-[340px] border-l`.
8. **Accounts.vue + InstancesMaintenance.vue** — Buttons, Badges, cards→`rounded-lg border-border`.

**Icons/status in logic** (`frontend/src/lib/format.ts`): **keep `format.ts` pure** —
refactor `tick()` and `connStatus()` to return a plain discriminant
(e.g. `'sent' | 'delivered' | 'read' | 'failed'`, `'connected' | 'disconnected' | ...`),
and map discriminant → lucide component + class / Badge variant **in the component**
(ChatThread, Accounts). Avoids coupling a formatting util to UI components. Do this once
their consumers are migrated. **`WhatsappIcon.vue`** (inline WA SVG) under
`src/components/icons/` since lucide has no brand glyph.

Preserve the 3-pane layout structure (`flex h-full`, `shrink-0`, `min-w-0`, fixed widths) —
reskin inside panes only.

## Phase 5 — Cleanup

Remove `@layer components` (`.btn*/.field/.card/.badge/.icon-btn`) and legacy tokens
(`brand*/ink/muted/panel/hair/rail`, unused `boxShadow`) from `style.css`/`tailwind.config.js`;
remove FontAwesome `<link>` from `index.html` (update `theme-color`). Re-grep to confirm
zero hits: `class="fa`, `bg-brand|border-hair|bg-panel|text-ink`, `gradient|from-|to-`.

## Verification

- After every step: `npm run typecheck` (fastest signal; catches alias + unused-import traps).
- After setup/theme/each screen: `npm run dev`, eyeball affected screen (no gradients, tight
  radii, hairline borders, one accent); toggle `.dark` on `<html>` in devtools.
- Before each commit + final: `npm run build` (the CI gate — green `dev` ≠ green `build`).
- Functional smoke (dev `/xchats` proxy → `localhost:8080`): login, chat list, open thread,
  send message, AssistantPanel suggest, Add-Account dialog (QR polling), account-filter
  Select, NavRail menu + logout — exercises every migrated component + preserved contracts.
- Final grep gates (expect zero hits) as listed in Phase 5.

## Critical files

- `frontend/vite.config.ts` + `frontend/tsconfig.json` — `@/`→`src` alias (must match).
- `frontend/src/lib/utils.ts` — new `cn()`.
- `frontend/tailwind.config.js` — shadcn color mappings, `darkMode`, radius; later strip legacy.
- `frontend/src/style.css` — HSL `:root`/`.dark` token set; later remove `@layer components`.
- `frontend/src/lib/format.ts` — `tick()`/`connStatus()` refactor to pure discriminants (mapped to icons/variants in components).
- `frontend/src/components/ChatThread.vue` — most complex consumer; canonical test of theming + icons + WA-green retention.
- `frontend/index.html` — remove FontAwesome CDN at the end.
