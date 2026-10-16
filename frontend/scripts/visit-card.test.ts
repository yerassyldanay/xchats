import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { landingDictionaries, renderVisitCard } from './lib/visitCard.ts'

// frontend/visit-card/index.html is generated (see scripts/lib/visitCard.ts).
// This is the drift guard: edit landing.css, the landing.* copy in the locale
// files, or scripts/visit-card.template.html, and this fails until
// `npm run build:visit-card` is run and the result committed.
describe('visit-card', () => {
  it('index.html is up to date with the landing page it is generated from', () => {
    const committed = readFileSync(join(dirname(fileURLToPath(import.meta.url)), '..', 'visit-card', 'index.html'), 'utf8')
    expect(committed).toBe(renderVisitCard())
  })

  it('ships all three languages, each with the full landing copy', () => {
    const dicts = landingDictionaries()
    const keys = Object.keys(dicts.en).sort()
    expect(keys.length).toBeGreaterThan(150)
    expect(Object.keys(dicts.ru).sort()).toEqual(keys)
    expect(Object.keys(dicts.kk).sort()).toEqual(keys)
  })

  it('describes the app and does not sell the disabled Meta channels as available', () => {
    const html = renderVisitCard()
    expect(html).toContain('data-i18n="overview.title"')
    expect(html).toContain('data-i18n="features.draft.title"')
    expect(landingDictionaries().en['platforms.igBadge']).toBe('Coming soon')
    expect(landingDictionaries().en['platforms.mgBadge']).toBe('Coming soon')
  })
})
