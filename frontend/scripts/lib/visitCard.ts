import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import en from '../../src/i18n/locales/en.ts'
import kk from '../../src/i18n/locales/kk.ts'
import ru from '../../src/i18n/locales/ru.ts'

// The Netlify visit-card (frontend/visit-card/index.html) is the SPA landing
// page's standalone twin: one static file with no build step on the host, so it
// can be published from the repo as-is. To keep the two from drifting apart it
// is GENERATED, never hand-edited:
//   - the stylesheet is src/components/landing/landing.css, verbatim;
//   - every string is the landing.* subtree of the app's own ru/en/kk catalogs,
//     switched client-side by the small script at the bottom of the template;
//   - the icons are the same lucide / brand glyphs the Vue components use.
// Only the markup (scripts/visit-card.template.html) is maintained by hand, and
// src/visit-card.test.ts fails whenever the committed file is out of date —
// regenerate with `npm run build:visit-card`.

const frontendRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
const read = (...p: string[]) => readFileSync(join(frontendRoot, ...p), 'utf8')

export type Dict = Record<string, string>

function flatten(o: Record<string, unknown>, prefix = '', out: Dict = {}): Dict {
  for (const [k, v] of Object.entries(o)) {
    if (v && typeof v === 'object') flatten(v as Record<string, unknown>, `${prefix}${k}.`, out)
    else out[`${prefix}${k}`] = String(v)
  }
  return out
}

/** The landing.* strings of each locale, flattened to "hero.title"-style keys. */
export function landingDictionaries(): Record<'ru' | 'en' | 'kk', Dict> {
  const pick = (m: unknown) => flatten((m as { landing: Record<string, unknown> }).landing)
  return { ru: pick(ru), en: pick(en), kk: pick(kk) }
}

const escapeHtml = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')

// --- icons ---------------------------------------------------------------

const kebab = (name: string) =>
  name
    .replace(/([a-z0-9])([A-Z])/g, '$1-$2')
    .replace(/([a-zA-Z])([0-9])/g, '$1-$2')
    .toLowerCase()

/** An inline SVG of a lucide icon, read from the same package the Vue app imports. */
function lucide(name: string, size: number, cls?: string): string {
  const src = read('node_modules', 'lucide-vue-next', 'dist', 'esm', 'icons', `${kebab(name)}.js`)
  const m = src.match(/createLucideIcon\("[^"]+",\s*(\[[\s\S]*?\])\s*\);/)
  if (!m) throw new Error(`visit-card: cannot read lucide icon ${name}`)
  const nodes = new Function(`return ${m[1]}`)() as [string, Record<string, string>][]
  const body = nodes
    .map(([tag, attrs]) => `<${tag} ${Object.entries(attrs).filter(([k]) => k !== 'key').map(([k, v]) => `${k}="${v}"`).join(' ')}/>`)
    .join('')
  return `<svg${cls ? ` class="${cls}"` : ''} xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${body}</svg>`
}

/** An inline SVG of one of the app's brand glyphs (src/components/icons/*.vue). */
function brand(name: 'whatsapp' | 'telegram' | 'instagram' | 'messenger', size: number, cls?: string): string {
  const file = `${name[0].toUpperCase()}${name.slice(1)}Icon.vue`
  const src = read('src', 'components', 'icons', file)
  const m = src.match(/<template>\s*<svg[\s\S]*?>\s*([\s\S]*?)\s*<\/svg>\s*<\/template>/)
  if (!m) throw new Error(`visit-card: cannot read brand icon ${file}`)
  const inner = m[1].replace(/\s+/g, ' ').replace(/\s*\/>/g, '/>').replace(/<path\s+d=/g, '<path d=')
  return `<svg${cls ? ` class="${cls}"` : ''} xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">${inner}</svg>`
}

// --- repeated blocks -------------------------------------------------------

const FEATURES: [key: string, icon: string][] = [
  ['inbox', 'Inbox'],
  ['customers', 'UsersRound'],
  ['followups', 'CalendarClock'],
  ['campaigns', 'Megaphone'],
  ['kb', 'Library'],
  ['draft', 'GitPullRequestDraft'],
  ['check', 'Bot'],
  ['channels', 'Radio'],
  ['settings', 'Settings'],
]

// One tab per app page — kept in step with ProductShowcase.vue's TABS.
export const SHOWCASE_TABS: [key: string, icon: string, img: string][] = [
  ['inbox', 'Inbox', 'inbox'],
  ['customers', 'UsersRound', 'customers'],
  ['followups', 'CalendarClock', 'followups'],
  ['campaigns', 'Megaphone', 'campaigns'],
  ['kb', 'Library', 'knowledge-base'],
  ['draft', 'GitPullRequestDraft', 'draft-staging'],
  ['simulator', 'Bot', 'simulator'],
  ['channels', 'Radio', 'channels'],
  ['settings', 'Settings', 'settings'],
]

const DOWNLOAD_BASE = 'https://github.com/yerassyldanay/xchats/releases/latest/download/'
const DESKTOP: [key: string, icon: string, installer: string, portable: string][] = [
  ['win', 'AppWindow', 'xchats-desktop-windows-amd64-installer.exe', 'xchats-desktop-windows-amd64.zip'],
  ['mac', 'Laptop', 'xchats-desktop-macos-universal.dmg', 'xchats-desktop-macos-universal.zip'],
  ['linux', 'Terminal', 'xchats-desktop-linux-amd64.deb', 'xchats-desktop-linux-amd64.tar.gz'],
]

const blocks: Record<string, () => string> = {
  features: () =>
    FEATURES.map(
      ([key, icon]) => `        <article class="landing-card">
          <div class="landing-card__icon">${lucide(icon, 23)}</div>
          <h3 class="landing-card__title" data-i18n="features.${key}.title"></h3>
          <p class="landing-card__desc" data-i18n="features.${key}.desc"></p>
        </article>`,
    ).join('\n'),

  showcaseTabs: () =>
    SHOWCASE_TABS.map(
      ([key, icon], i) => `        <button type="button" role="tab" class="landing-showcase__tab${i === 0 ? ' is-active' : ''}" id="showcase-tab-${key}" data-tab="${key}" aria-selected="${i === 0}" aria-controls="showcase-panel" tabindex="${i === 0 ? 0 : -1}">${lucide(icon, 17)} <span data-i18n="showcase.${key}Tab"></span></button>`,
    ).join('\n'),

  channels: () =>
    (
      [
        ['wa', 'whatsapp', false],
        ['tg', 'telegram', false],
        ['ig', 'instagram', true],
        ['mg', 'messenger', true],
      ] as const
    )
      .map(
        ([key, glyph, soon]) => `        <div class="landing-card${soon ? ' landing-card--soon' : ''}">
          <div class="landing-channel-card__icon landing-channel-card__icon--${key}">${brand(glyph, 28)}</div>
          <h3 class="landing-card__title" data-i18n="platforms.${key}Title"></h3>
          <p class="landing-card__desc" data-i18n="platforms.${key}Desc"></p>
          <span class="landing-pill landing-card__badge ${soon ? 'landing-pill--muted' : 'landing-pill--success'}" data-i18n="platforms.${key}Badge"></span>
        </div>`,
      )
      .join('\n'),

  desktop: () =>
    DESKTOP.map(
      ([key, icon, installer, portable]) => `        <div class="landing-link-card">
          <div class="landing-link-card__icon">${lucide(icon, 23)}</div>
          <div class="landing-link-card__title">
            <span data-i18n="desktop.${key}Title"></span>
            <code class="landing-desktop__exe" data-i18n="desktop.${key}InstallerExe"></code>
          </div>
          <p class="landing-link-card__desc" data-i18n="desktop.${key}Desc"></p>
          <a href="${DOWNLOAD_BASE}${installer}" target="_blank" rel="noreferrer" class="landing-btn landing-btn--primary landing-btn--sm landing-link-card__download"><span data-i18n="desktop.downloadCta"></span> ${lucide('ArrowRight', 17)}</a>
          <a href="${DOWNLOAD_BASE}${portable}" target="_blank" rel="noreferrer" class="landing-link-card__cta-secondary"><span data-i18n="desktop.portableCta"></span> <code class="landing-desktop__exe" data-i18n="desktop.${key}PortableExe"></code></a>
        </div>`,
    ).join('\n'),

  architecture: () =>
    (
      [
        ['binary', 'Cpu'],
        ['privacy', 'Lock'],
        ['model', 'Waypoints'],
      ] as const
    )
      .map(
        ([key, icon]) => `        <article class="landing-card">
          <div class="landing-card__icon">${lucide(icon, 23)}</div>
          <h3 class="landing-card__title" data-i18n="architecture.${key}Title"></h3>
          <p class="landing-card__desc" data-i18n="architecture.${key}Desc"></p>
        </article>`,
      )
      .join('\n'),

  steps: () =>
    [1, 2, 3, 4]
      .map(
        (n) => `        <div class="landing-step">
          <div class="landing-step__num">${n}</div>
          <div class="landing-step__title" data-i18n="how.step${n}Title"></div>
          <div class="landing-step__desc" data-i18n="how.step${n}Desc"></div>
        </div>`,
      )
      .join('\n'),

  flow: () =>
    [1, 2, 3]
      .map(
        (n) => `        <div class="landing-flow__row">
          <div class="landing-flow__num">${n}</div>
          <div>
            <div class="landing-flow__title" data-i18n="overview.step${n}Title"></div>
            <p class="landing-flow__desc" data-i18n="overview.step${n}Desc"></p>
          </div>
        </div>`,
      )
      .join('\n'),

  tools: () =>
    ['kb_product_upsert', 'kb_tariff_upsert', 'kb_delivery_zone_upsert']
      .map((t) => `          <div class="landing-tool-chip">${lucide('Wrench', 14)} ${t}</div>`)
      .join('\n'),
}

// --- rendering ----------------------------------------------------------------

export function renderVisitCard(): string {
  const dicts = landingDictionaries()
  const template = read('scripts', 'visit-card.template.html')
  const css = read('src', 'components', 'landing', 'landing.css')

  let html = template
    .replace('/*__LANDING_CSS__*/', () => css.trim())
    .replace(/\{\{block:(\w+)\}\}/g, (_, name: string) => {
      const block = blocks[name]
      if (!block) throw new Error(`visit-card: unknown block ${name}`)
      return block()
    })
    .replace(/\{\{i:(\w+)(?::(\d+))?(?::([\w -]+))?\}\}/g, (_, name: string, size?: string, cls?: string) => lucide(name, Number(size ?? 24), cls))
    .replace(/\{\{b:(\w+)(?::(\d+))?(?::([\w -]+))?\}\}/g, (_, name: string, size?: string, cls?: string) =>
      brand(name as Parameters<typeof brand>[0], Number(size ?? 24), cls),
    )

  // Default (English) copy goes into the markup so the page reads fine before —
  // and without — the script; the script then swaps in the visitor's language.
  const missing = new Set<string>()
  const text = (key: string) => {
    const v = dicts.en[key]
    if (v === undefined) missing.add(key)
    return v === undefined ? '' : escapeHtml(v)
  }
  html = html
    .replace(/(<([a-z0-9]+)\b[^>]*\sdata-i18n="([^"]+)"[^>]*>)(<\/\2>)/g, (_m, open: string, _tag: string, key: string, close: string) => open + text(key) + close)
    .replace(/data-i18n-alt="([^"]+)" alt=""/g, (_m, key: string) => `data-i18n-alt="${key}" alt="${text(key)}"`)
  if (missing.size) throw new Error(`visit-card: template uses keys missing from landing.*: ${[...missing].join(', ')}`)

  const json = JSON.stringify(dicts, null, 2).replace(/</g, '\\u003c')
  return html.replace('/*__TRANSLATIONS__*/{}', () => json).replace('/*__SHOWCASE__*/[]', () => JSON.stringify(SHOWCASE_TABS.map(([key, , img]) => [key, img])))
}
