# Frontend visual overhaul — shadcn-vue + Linear-style minimal

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
