import { describe, expect, it } from 'vitest'
import type { CatalogExtractCase, CatalogFact, CatalogScenario, CatalogTestCase } from '../types'
import {
  groupScenariosByExperiment,
  notCheckedExtractRequirements,
  notCheckedRequirements,
  resolveMediaExpectation,
  resolveRequires,
} from './evalCatalog'

const facts: CatalogFact[] = [
  { token: '{{product.coffee-machine.price}}', value: '129 900 ₸' },
  { token: '{{policy.main.delivery_cost}}', value: '1 500 ₸' },
]

describe('resolveRequires', () => {
  it('resolves a single-alternative AND group to its real value, keyed on the literal {{token}} form', () => {
    const groups = resolveRequires([['product.coffee-machine.price']], facts)
    expect(groups).toHaveLength(1)
    expect(groups[0].alternatives).toEqual([{ token: '{{product.coffee-machine.price}}', value: '129 900 ₸' }])
  })

  it('keeps AND-of-OR structure intact — never flattens', () => {
    // "must state delivery cost AND (either schema's field name for duration)"
    const groups = resolveRequires(
      [['policy.main.delivery_cost'], ['policy.main.delivery_time', 'policy.main.delivery_in_days']],
      facts,
    )
    expect(groups).toHaveLength(2) // two AND steps, not flattened into one list of 3
    expect(groups[1].alternatives).toHaveLength(2) // the OR step keeps both alternatives
    expect(groups[1].alternatives.map((a) => a.token)).toEqual(['{{policy.main.delivery_time}}', '{{policy.main.delivery_in_days}}'])
  })

  it('surfaces a missing/unresolvable token as value=null, never hidden', () => {
    const groups = resolveRequires([['product.coffee-machine.cost']], facts) // typo'd token, not in facts
    expect(groups[0].alternatives[0]).toEqual({ token: '{{product.coffee-machine.cost}}', value: null })
  })

  it('returns empty for an undeclared requires', () => {
    expect(resolveRequires(undefined, facts)).toEqual([])
  })
})

describe('resolveMediaExpectation', () => {
  it('flags a ref that exists in the catalog as found', () => {
    const m = resolveMediaExpectation({ any_of_refs: ['coffee-photo-1'] }, ['coffee-photo-1', 'coffee-photo-2'], [])
    expect(m?.refs).toEqual([{ name: 'coffee-photo-1', found: true }])
  })

  it('flags an unknown ref as NOT found — the page must surface this, never hide it', () => {
    const m = resolveMediaExpectation({ any_of_refs: ['coffee-photo-999'] }, ['coffee-photo-1'], [])
    expect(m?.refs).toEqual([{ name: 'coffee-photo-999', found: false }])
  })

  it('returns null when the test declares no media expectation at all', () => {
    expect(resolveMediaExpectation(undefined, ['coffee-photo-1'], [])).toBeNull()
  })
})

function baseTest(over: Partial<CatalogTestCase> = {}): CatalogTestCase {
  return { id: 't1', message: 'hi', source: 'tests.yaml', ...over }
}

describe('notCheckedRequirements', () => {
  it('lists every knob a bare test omits', () => {
    expect(notCheckedRequirements(baseTest())).toEqual([
      'Обязательные факты',
      'Язык ответа',
      'Эскалация',
      'Медиа',
      'Запрещённые фразы',
    ])
  })

  it('drops a knob once it is declared', () => {
    const items = notCheckedRequirements(baseTest({ requires: [['x']], language: 'ru' }))
    expect(items).not.toContain('Обязательные факты')
    expect(items).not.toContain('Язык ответа')
    expect(items).toContain('Эскалация')
  })

  it('escalate:false is an ACTIVE requirement, not "not checked" — must not appear here', () => {
    const items = notCheckedRequirements(baseTest({ escalate: false }))
    expect(items).not.toContain('Эскалация')
  })
})

function baseExtractCase(over: Partial<CatalogExtractCase> = {}): CatalogExtractCase {
  return { id: 'c1', image: 'catalog/inputs/c1.jpg', source: 'extract/cases.yaml', ...over }
}

describe('notCheckedExtractRequirements', () => {
  it('lists every knob a bare case omits, EXCLUDING no_invented_numbers (always active)', () => {
    const items = notCheckedExtractRequirements(baseExtractCase())
    expect(items).toEqual([
      'Классификация',
      'Обязательные фразы',
      'Тема/описание (обязательно)',
      'Тема/описание (любое из)',
      'Обязательные числа',
      'Запрет валюты',
    ])
  })

  it('drops a knob once declared', () => {
    const items = notCheckedExtractRequirements(baseExtractCase({ fields: { content_kind: 'infographic' } }))
    expect(items).not.toContain('Классификация')
  })
})

describe('groupScenariosByExperiment', () => {
  function scenario(name: string, experiment?: string): CatalogScenario {
    return { name, contract: 'asset_refs', facts_source: 'data.yaml', facts: [], tests: [], experiment }
  }

  it('groups by experiment, preserving first-appearance order', () => {
    const groups = groupScenariosByExperiment([
      scenario('lang-v1', 'lang-bakeoff'),
      scenario('esc-v1', 'escalation-bakeoff'),
      scenario('lang-v2', 'lang-bakeoff'),
      scenario('legacy'), // no experiment
    ])
    expect(groups.map((g) => g.experiment)).toEqual(['lang-bakeoff', 'escalation-bakeoff', ''])
    expect(groups[0].scenarios.map((s) => s.name)).toEqual(['lang-v1', 'lang-v2'])
    expect(groups[2].scenarios.map((s) => s.name)).toEqual(['legacy'])
  })
})
