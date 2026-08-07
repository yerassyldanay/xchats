import { describe, expect, it } from 'vitest'
import ru from './locales/ru'
import en from './locales/en'
import kk from './locales/kk'
import { TEST_FIELD_KEYS } from '@/lib/evalFieldDocs'

// Plain-object comparison — no vue-i18n runtime needed. fallbackLocale: 'ru' means a
// missing EN key silently falls back instead of erroring, so this test is the only
// thing standing between "EN toggle" and "EN toggle that quietly still shows Russian
// for half the fields." Recurses through every locale message, not just the two lists
// below, so a stray extra/missing key anywhere in evalCatalog.* is caught too.
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

describe('ru/en locale parity', () => {
  it('has an identical key set in both locales', () => {
    const ruKeys = keySet(ru)
    const enKeys = keySet(en)
    const missingInEn = [...ruKeys].filter((k) => !enKeys.has(k))
    const missingInRu = [...enKeys].filter((k) => !ruKeys.has(k))
    expect(missingInEn, `keys present in ru but missing in en: ${missingInEn.join(', ')}`).toEqual([])
    expect(missingInRu, `keys present in en but missing in ru: ${missingInRu.join(', ')}`).toEqual([])
  })

  it('has a fields.<key> entry in both locales for every TEST_FIELD_KEYS entry', () => {
    for (const key of TEST_FIELD_KEYS) {
      expect(ru.evalCatalog.fields, `ru missing fields.${key}`).toHaveProperty(key)
      expect(en.evalCatalog.fields, `en missing fields.${key}`).toHaveProperty(key)
    }
  })

})

// kk.ts is deliberately partial (see its own doc comment) — these tests
// pin that shape down: only the landing.* namespace, and that namespace
// exactly matching ru/en's landing.* key set. A kk key added without its
// ru/en counterpart (or vice versa) fails here, not silently at render
// time via fallbackLocale.
describe('kk locale is landing-only and matches ru/en landing.* exactly', () => {
  it('contains only landing.* keys', () => {
    const kkKeys = [...keySet(kk)]
    const nonLanding = kkKeys.filter((k) => !k.startsWith('landing.'))
    expect(nonLanding, `kk has non-landing keys: ${nonLanding.join(', ')}`).toEqual([])
    expect(kkKeys.length).toBeGreaterThan(0)
  })

  it('has the same landing.* key set as ru and en', () => {
    const landingKeysOf = (locale: unknown) => new Set([...keySet(locale as { landing: unknown }).values()].filter((k) => k.startsWith('landing.')))
    const ruLanding = landingKeysOf(ru)
    const enLanding = landingKeysOf(en)
    const kkLanding = landingKeysOf(kk)

    const missingInKk = [...ruLanding].filter((k) => !kkLanding.has(k))
    const extraInKk = [...kkLanding].filter((k) => !ruLanding.has(k))
    expect(missingInKk, `landing keys present in ru but missing in kk: ${missingInKk.join(', ')}`).toEqual([])
    expect(extraInKk, `landing keys present in kk but missing in ru: ${extraInKk.join(', ')}`).toEqual([])

    // ru/en parity for landing.* is already covered by the top-level ru/en
    // test above (it walks the whole tree); this just double-checks the
    // three-way comparison agrees on the same set.
    expect([...ruLanding].sort()).toEqual([...enLanding].sort())
  })
})
