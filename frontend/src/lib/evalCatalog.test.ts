import { describe, expect, it } from 'vitest'
import type { CatalogExtractCase, CatalogFact, CatalogScenario, CatalogTestCase } from '../types'
import {
  annotateMediaExpect,
  formatYamlValue,
  groupScenariosByExperiment,
  notCheckedExtractRequirements,
  notCheckedRequirements,
  resolveRequires,
  scenarioNavLabel,
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

describe('annotateMediaExpect', () => {
  it('flags a token that exists in the scenario as found', () => {
    const a = annotateMediaExpect({ any_of: ['coffee-photo-1'] }, ['coffee-photo-1', 'coffee-photo-2'])
    expect(a).toEqual([{ key: 'any_of', name: 'coffee-photo-1', found: true }])
  })

  it('flags an unknown token as NOT found — a broken reference must be surfaced, never hidden', () => {
    const a = annotateMediaExpect({ any_of: ['coffee-photo-999'] }, ['coffee-photo-1'])
    expect(a).toEqual([{ key: 'any_of', name: 'coffee-photo-999', found: false }])
  })

  it('returns [] when the test declares no media expectation at all (undefined)', () => {
    expect(annotateMediaExpect(undefined, ['coffee-photo-1'])).toEqual([])
  })

  it('returns [] for a forbid-only expectation — nothing to audit', () => {
    expect(annotateMediaExpect({ forbid: true }, ['coffee-photo-1'])).toEqual([])
  })

  it('annotates all_of tokens with the all_of key', () => {
    const a = annotateMediaExpect({ all_of: ['a', 'b'] }, ['a'])
    expect(a).toEqual([
      { key: 'all_of', name: 'a', found: true },
      { key: 'all_of', name: 'b', found: false },
    ])
  })
})

function baseTest(over: Partial<CatalogTestCase> = {}): CatalogTestCase {
  return { id: 't1', message: 'hi', source: 'tests.yaml', ...over }
}

describe('notCheckedRequirements', () => {
  it('lists every knob a bare test omits, by schema key name', () => {
    expect(notCheckedRequirements(baseTest())).toEqual([
      'requires',
      'language',
      'escalate',
      'media',
      'must_not_contain',
      'must_contain_any',
      'forbid_tokens',
      'outcomes',
    ])
  })

  it('drops a knob once it is declared', () => {
    const items = notCheckedRequirements(baseTest({ requires: [['x']], language: 'ru' }))
    expect(items).not.toContain('requires')
    expect(items).not.toContain('language')
    expect(items).toContain('escalate')
  })

  it('a declared outcomes list counts as checked — must not appear here', () => {
    const items = notCheckedRequirements(
      baseTest({ outcomes: [{ label: 'a', escalate: false }, { label: 'b', must_contain_any: ['x'] }] }),
    )
    expect(items).not.toContain('outcomes')
  })

  it('escalate:false is an ACTIVE requirement, not "not checked" — must not appear here', () => {
    const items = notCheckedRequirements(baseTest({ escalate: false }))
    expect(items).not.toContain('escalate')
  })

  it('a forbid-only media block counts as declared — "media" must not appear here', () => {
    const items = notCheckedRequirements(baseTest({ media: { forbid: true } }))
    expect(items).not.toContain('media')
  })

  it('a declared forbid_tokens list counts as checked', () => {
    const items = notCheckedRequirements(baseTest({ forbid_tokens: ['product.x.'] }))
    expect(items).not.toContain('forbid_tokens')
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
    return { name, facts_source: 'data.yaml', facts: [], tests: [], experiment }
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

describe('formatYamlValue', () => {
  it('renders a scalar as-is, unquoted', () => {
    expect(formatYamlValue('hello')).toBe('hello')
    expect(formatYamlValue(42)).toBe('42')
  })

  it('renders false as the literal string "false", never dropped/blank', () => {
    expect(formatYamlValue(false)).toBe('false')
  })

  it('renders a string[] as dash-prefixed lines', () => {
    expect(formatYamlValue(['a', 'b'])).toBe('- a\n- b')
  })

  it('renders a string[][] as flow-style dash lines, never flattened', () => {
    expect(formatYamlValue([['a', 'b'], ['c']])).toBe('- [a, b]\n- [c]')
  })

  it('renders history (object array) with the first key inline and the rest indented', () => {
    const history = [
      { role: 'client', text: 'привет' },
      { role: 'assistant', text: 'здравствуйте' },
    ]
    expect(formatYamlValue(history)).toBe('- role: client\n  text: привет\n- role: assistant\n  text: здравствуйте')
  })

  it('renders a media map with flow-style arrays and plain scalars', () => {
    expect(formatYamlValue({ any_of: ['a', 'b'], exclusive: true })).toBe('any_of: [a, b]\nexclusive: true')
  })

  it('renders outcomes (object array with nested requires) via the same object-array rule', () => {
    const outcomes = [{ label: 'asks', forbid_tokens: ['delivery.'] }, { label: 'answers', escalate: false }]
    expect(formatYamlValue(outcomes)).toBe('- label: asks\n  forbid_tokens: [delivery.]\n- label: answers\n  escalate: false')
  })

  it('renders llm_checks (object array) with claim/expect', () => {
    const llmChecks = [{ claim: 'no date promised', expect: true }]
    expect(formatYamlValue(llmChecks)).toBe('- claim: no date promised\n  expect: true')
  })

  it('applies the indent parameter to every line', () => {
    expect(formatYamlValue(['a', 'b'], 1)).toBe('  - a\n  - b')
  })
})

describe('scenarioNavLabel', () => {
  it('falls back to name/tests.yaml when tests_path is absent (stale v2 catalog.json)', () => {
    expect(scenarioNavLabel({ name: 'combo-canary-v1-ru' })).toBe('combo-canary-v1-ru/tests.yaml')
  })

  it("shows the plain relative path for a scenario that owns its tests.yaml", () => {
    expect(scenarioNavLabel({ name: 'combo-canary-v1-ru', tests_path: 'scenarios/combo-canary-v1-ru/tests.yaml' })).toBe(
      'combo-canary-v1-ru/tests.yaml',
    )
  })

  it('shows a borrow arrow when the scenario points at another scenario\'s tests.yaml', () => {
    expect(scenarioNavLabel({ name: 'lang-canary-v2', tests_path: 'scenarios/lang-canary-v1/tests.yaml' })).toBe(
      'lang-canary-v2 → lang-canary-v1/tests.yaml',
    )
  })
})
