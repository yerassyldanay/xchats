import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'
import ru from './locales/ru'
import en from './locales/en'
import kk from './locales/kk'
import { TEST_FIELD_KEYS } from '@/lib/evalFieldDocs'

// Plain-object comparison — no vue-i18n runtime needed. fallbackLocale: 'ru' means a
// missing EN or KK key silently falls back instead of erroring, so this test is the
// only thing standing between "three-language app" and "three-language app that
// quietly still shows Russian for half the screens." Recurses through every locale
// message, not just the lists below, so a stray extra/missing key anywhere is caught.
function keySet(obj: unknown, prefix = ''): Set<string> {
  const keys = new Set<string>()
  if (obj === null || typeof obj !== 'object') return keys
  for (const [k, v] of Object.entries(obj as Record<string, unknown>)) {
    const path = prefix ? `${prefix}.${k}` : k
    if (v !== null && typeof v === 'object') {
      for (const nested of keySet(v, path)) keys.add(nested)
    } else {
      keys.add(path)
    }
  }
  return keys
}

// ru is the source of truth: strings are written there first, then translated.
// Both other locales are compared against it rather than against each other, so a
// failure names exactly which file is behind.
const LOCALES: [string, unknown][] = [
  ['en', en],
  ['kk', kk],
]

describe('locale parity', () => {
  const ruKeys = keySet(ru)

  it('has a non-trivial ru catalog to compare against', () => {
    expect(ruKeys.size).toBeGreaterThan(500)
  })

  for (const [name, messages] of LOCALES) {
    it(`${name} has an identical key set to ru`, () => {
      const keys = keySet(messages)
      const missing = [...ruKeys].filter((k) => !keys.has(k))
      const extra = [...keys].filter((k) => !ruKeys.has(k))
      expect(missing, `keys present in ru but missing in ${name}: ${missing.join(', ')}`).toEqual([])
      expect(extra, `keys present in ${name} but missing in ru: ${extra.join(', ')}`).toEqual([])
    })

    // An empty string is legitimate where it is empty in ru too: landing.hero
    // splits one headline into prefix/highlight/suffix, and which of those slots
    // a language needs depends on its word order (kk fills the suffix, ru/en
    // leave it blank). What is never legitimate is blanking out a slot ru fills
    // — that is a translation someone skipped.
    it(`${name} fills every string ru fills`, () => {
      const ruStrings = new Map(collectStrings(ru))
      const blank = collectStrings(messages)
        .filter(([k, v]) => v.trim() === '' && (ruStrings.get(k) ?? '').trim() !== '')
        .map(([k]) => k)
      expect(blank, `untranslated (blank) keys in ${name}: ${blank.join(', ')}`).toEqual([])
    })
  }

  it('has a fields.<key> entry in every locale for every TEST_FIELD_KEYS entry', () => {
    for (const key of TEST_FIELD_KEYS) {
      expect(ru.evalCatalog.fields, `ru missing fields.${key}`).toHaveProperty(key)
      expect(en.evalCatalog.fields, `en missing fields.${key}`).toHaveProperty(key)
      expect(kk.evalCatalog.fields, `kk missing fields.${key}`).toHaveProperty(key)
    }
  })
})

function collectStrings(obj: unknown, prefix = ''): [string, string][] {
  const out: [string, string][] = []
  if (obj === null || typeof obj !== 'object') return out
  for (const [k, v] of Object.entries(obj as Record<string, unknown>)) {
    const path = prefix ? `${prefix}.${k}` : k
    if (typeof v === 'string') out.push([path, v])
    else if (v !== null && typeof v === 'object') out.push(...collectStrings(v, path))
  }
  return out
}

// A locale that is still 1:1 Russian is a translation that never happened. These
// spot-checks pin down a handful of high-traffic strings per locale — enough that
// copying ru.ts to en.ts/kk.ts to "fix" the parity test above fails here instead.
describe('locales are actually translated, not copies of ru', () => {
  it('en differs from ru on the chrome every screen shows', () => {
    expect(en.nav.inbox).not.toBe(ru.nav.inbox)
    expect(en.common.close).not.toBe(ru.common.close)
    expect(en.login.title).not.toBe(ru.login.title)
    expect(en.kb.page.title).not.toBe(ru.kb.page.title)
  })

  it('kk differs from ru on the chrome every screen shows', () => {
    expect(kk.nav.channels).not.toBe(ru.nav.channels)
    expect(kk.common.close).not.toBe(ru.common.close)
    expect(kk.login.title).not.toBe(ru.login.title)
    expect(kk.kb.page.title).not.toBe(ru.kb.page.title)
  })

  // kk uses the Kazakh-specific letters ә/ғ/қ/ң/ө/ұ/ү/һ/і; a block of Russian
  // pasted into kk.ts would carry none of them.
  it('kk uses Kazakh-specific letters throughout, not just in a few keys', () => {
    const kazakhOnly = /[әғқңөұүһі]/
    const strings = collectStrings(kk).filter(([, v]) => /\p{Script=Cyrillic}/u.test(v))
    const withKazakhLetters = strings.filter(([, v]) => kazakhOnly.test(v))
    // Not every Cyrillic string can contain one (short words like "Модель" have
    // no Kazakh-only letter), so this is a proportion, not a per-string rule.
    expect(withKazakhLetters.length / strings.length).toBeGreaterThan(0.5)
  })
})

// Key parity says the same NAMES exist everywhere; it says nothing about whether
// the values behind them survive vue-i18n's message compiler. Two things routinely
// break only at render time, on one locale, on a screen nobody opened yet:
//
//   - a special character the compiler claims. A bare "@" starts a linked-message
//     reference and throws "Invalid linked format" — so a literal handle must be
//     written {'@'}BotFather. Same for an unbalanced brace or a stray "|".
//   - a lost interpolation. A translator who drops {count} from one locale leaves
//     a sentence that silently renders without its number.
//
// So compile and render every message in every locale here, where a failure names
// the exact key, rather than finding out from a blank spot in production.
describe('every message compiles and renders in every locale', () => {
  const LOCALE_MESSAGES: Record<string, unknown> = { ru, en, kk }
  const i18n = createI18n({
    legacy: false,
    locale: 'ru',
    fallbackLocale: 'ru',
    messages: { ru, en, kk },
    missingWarn: false,
    fallbackWarn: false,
  })

  // Every named value any message interpolates. Values are placeholders — this
  // exercises the compiler, it does not assert wording.
  const NAMED = {
    n: 3, count: 3, pct: 42, at: 'X', d: 'X', id: 'X', list: 'X', total: 5, scenario: 1, extract: 1,
    from: 1, to: 2, avg: '$1', priced: 1, setup: 'S', experiment: 'E', axis: 'A', in: 1, out: 2,
    pass: 1, h: 1, m: 2, s: 3, name: 'N', seconds: 5, offset: '+05:00', when: 'X', interval: 1,
    jitter: 2, variables: 'v', used: 1, max: 2, sent: 1,
  }

  for (const locale of Object.keys(LOCALE_MESSAGES)) {
    it(`${locale} renders every key without a compile error`, () => {
      i18n.global.locale.value = locale as 'ru' | 'en' | 'kk'
      const failures: string[] = []
      for (const [key, source] of collectStrings(LOCALE_MESSAGES[locale])) {
        try {
          // A message with "|" branches is a plural set and must be called with a count.
          const rendered = source.includes(' | ')
            ? i18n.global.t(key, 3, { named: NAMED })
            : i18n.global.t(key, NAMED)
          if (rendered === key) failures.push(`${key} (did not resolve)`)
        } catch (e) {
          failures.push(`${key} (${e instanceof Error ? e.message.split('\n')[0] : String(e)})`)
        }
      }
      expect(failures, `messages that failed to render in ${locale}:\n${failures.join('\n')}`).toEqual([])
    })
  }
})

describe('interpolations survive translation', () => {
  // The DISTINCT named values a message interpolates. Distinct, not a raw count,
  // because a plural message repeats its placeholder once per branch and the
  // branch count is a property of the language: ru needs four (zero/one/few/many),
  // kk needs one — a Kazakh noun after a numeral stays in the singular.
  //
  // Only real {placeholder} names match — the {'{{'} literal-brace escape used by
  // the campaign-template hint is not \w, so it is never picked up here.
  const placeholdersIn = (message: string) =>
    [...new Set([...message.matchAll(/\{(\w+)\}/g)].map((match) => match[1]))].sort()

  for (const [name, messages] of LOCALES) {
    it(`${name} keeps the same named values as ru`, () => {
      const translated = new Map(collectStrings(messages))
      const mismatched = collectStrings(ru)
        .map(([key, source]) => {
          const other = translated.get(key)
          if (other === undefined) return null
          const expected = placeholdersIn(source)
          const actual = placeholdersIn(other)
          return expected.join(',') === actual.join(',')
            ? null
            : `${key}: ru has {${expected.join('}, {')}}, ${name} has {${actual.join('}, {')}}`
        })
        .filter((row): row is string => row !== null)
      expect(mismatched, `interpolation mismatches in ${name}:\n${mismatched.join('\n')}`).toEqual([])
    })
  }
})
