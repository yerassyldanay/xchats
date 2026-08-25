import { createI18n } from 'vue-i18n'
import { watch } from 'vue'
import ru from './locales/ru'
import en from './locales/en'
import kk from './locales/kk'

// App-wide i18n foundation — vue-i18n over a one-off toggle because this is
// meant to grow beyond the catalog page. Composition API only (legacy:
// false) — no Options API $t() usage anywhere.
//
// All three locales (ru, en, kk) carry the FULL message catalog; locales.test.ts
// enforces that three-way parity so a key added to one and forgotten in another
// fails the suite instead of silently falling back at render time.
// fallbackLocale stays 'ru' as a last-resort net, not as a translation strategy.
// localStorage is guarded, not assumed: this module is imported by Pinia stores
// (for t() below), and those are unit-tested under vitest's `node` project where
// there is no localStorage at all. A missing store means "no saved preference",
// which is exactly the ru default — never a crash at import time.
function readStoredLocale(): string | null {
  try {
    return localStorage.getItem('locale')
  } catch {
    return null
  }
}
function writeStoredLocale(value: string): void {
  try {
    localStorage.setItem('locale', value)
  } catch {
    // Private-mode quota errors and non-browser environments both land here;
    // the locale still applies for this session, it just isn't remembered.
  }
}

const stored = readStoredLocale()
const initialLocale = stored === 'en' ? 'en' : stored === 'kk' ? 'kk' : 'ru' // garbage-safe: anything else falls back to ru

// Russian needs three plural forms where English needs two, and Kazakh needs
// none at all (a noun after a numeral stays in the singular: "5 іске қосу").
// Messages that pluralise are written with four choices — zero | one | few |
// many — and these rules pick among them; kk's messages carry a single choice,
// which vue-i18n returns unconditionally.
function ruPluralRule(choice: number, choicesLength: number): number {
  if (choice === 0) return 0
  const teen = choice % 100 > 10 && choice % 100 < 20
  const mod10 = choice % 10
  if (!teen && mod10 === 1) return 1
  if (!teen && mod10 >= 2 && mod10 <= 4) return 2
  return choicesLength < 4 ? 2 : 3
}

export const i18n = createI18n({
  legacy: false,
  locale: initialLocale,
  fallbackLocale: 'ru',
  messages: { ru, en, kk },
  pluralRules: { ru: ruPluralRule },
})

watch(i18n.global.locale, writeStoredLocale)

// t() for code that runs outside a component's setup() — Pinia stores, mostly.
// useI18n() is only legal inside setup, but the messages a store puts on state
// (API failure fallbacks) are rendered as UI and must follow the locale, so
// they resolve through the same instance at call time rather than at import
// time. Deliberately narrow: components should keep using useI18n().
export function t(key: string, named?: Record<string, unknown>): string {
  return named ? i18n.global.t(key, named) : i18n.global.t(key)
}
