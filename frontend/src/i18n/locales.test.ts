import { describe, expect, it } from 'vitest'
import ru from './locales/ru'
import en from './locales/en'
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
