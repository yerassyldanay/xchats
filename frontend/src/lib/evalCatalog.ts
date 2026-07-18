import type { CatalogExtractCase, CatalogFact, CatalogMediaExpect, CatalogScenario, CatalogTestCase } from '../types'

// Pure display logic for the requirements catalog (/evals/catalog) — NO Vue imports,
// same discipline as evalMatrix.ts, and deliberately a SEPARATE module: this page is a
// read-only requirements review, not a results/performance view, and must never grow a
// dependency on runs.json/executions.json or any pass/fail data (see the catalog
// plan's explicit scope cut).

export interface ResolvedToken {
  token: string
  value: string | null // null = not found in Facts — the page's whole purpose is surfacing this
}
// RequireGroup is one AND-step of TestCase.Requires — "alternatives" holds every OR
// option for that step (satisfied by using ANY ONE of them). Never flatten AND-of-OR
// into a single list; that changes what the requirement actually means.
export interface RequireGroup {
  alternatives: ResolvedToken[]
}

// requires entries are BARE dotted refs ("product.coffee-machine.price"), as authored
// in tests.yaml — but CatalogFact.token is the literal "{{...}}" form the model must
// actually emit (evals/harness's factToken/factTokenLiteral) and Facts is keyed on
// that literal form. Wrap before lookup, and keep the wrapped literal for display too —
// that IS the requirement's real shape.
export function resolveRequires(groups: string[][] | undefined, facts: CatalogFact[]): RequireGroup[] {
  const byToken = new Map(facts.map((f) => [f.token, f.value]))
  return (groups ?? []).map((alts) => ({
    alternatives: alts.map((dotted) => {
      const token = `{{${dotted}}}`
      return { token, value: byToken.get(token) ?? null }
    }),
  }))
}

export interface MediaExpectationEntry {
  name: string
  found: boolean // false = not in this scenario's media_refs/media_groups — a broken reference
}
export interface MediaExpectation {
  groups: MediaExpectationEntry[]
  refs: MediaExpectationEntry[]
  // forbid: true means the test requires an EMPTY media array — mutually exclusive with
  // groups/refs being non-empty (render.go rejects the combination at the source).
  forbid: boolean
  // exclusive: true means groups/refs above is not just "attach at least one of these"
  // but "attach at least one of these, and nothing else" — a modifier on the same list,
  // never populated without a non-empty groups/refs (render.go enforces this).
  exclusive: boolean
}

export function resolveMediaExpectation(
  media: CatalogMediaExpect | undefined,
  mediaRefs: string[] | undefined,
  mediaGroups: string[] | undefined,
): MediaExpectation | null {
  if (!media) return null
  const refSet = new Set(mediaRefs ?? [])
  const groupSet = new Set(mediaGroups ?? [])
  return {
    groups: (media.any_of_groups ?? []).map((g) => ({ name: g, found: groupSet.has(g) })),
    refs: (media.any_of_refs ?? []).map((r) => ({ name: r, found: refSet.has(r) })),
    forbid: media.forbid ?? false,
    exclusive: media.exclusive ?? false,
  }
}

// notCheckedRequirements lists, in Russian, which knobs a scenario test does NOT
// declare — mirrors judge.go's own gating EXACTLY (requiresSatisfied/media/escalate/
// language/must_not_contain/must_contain_any all default to "trivially satisfied" when the test simply
// doesn't declare that check), so this list is never a guess about what's ungraded —
// it's the direct complement of what judge.go actually skips. No separate branch is
// needed for media.forbid: `!test.media` already treats ANY declared media block
// (forbid-only or any_of-based) as "checked," matching judge.go's own gating on
// `tc.Media != nil`.
export function notCheckedRequirements(test: CatalogTestCase): string[] {
  const items: string[] = []
  if (!test.requires || test.requires.length === 0) items.push('Обязательные факты')
  if (test.language !== 'kk' && test.language !== 'ru') items.push('Язык ответа')
  if (test.escalate === undefined) items.push('Эскалация')
  if (!test.media) items.push('Медиа')
  if (!test.must_not_contain || test.must_not_contain.length === 0) items.push('Запрещённые фразы')
  if (!test.must_contain_any || test.must_contain_any.length === 0) items.push('Ожидаемые фразы (любая из)')
  return items
}

// notCheckedExtractRequirements mirrors extract_checks.go's own opt-in gating.
// no_invented_numbers is DELIBERATELY excluded here — it always runs (fail-closed
// default) regardless of whether allowed_numbers is declared or empty, so it's never
// "not checked"; it always renders as an active requirement (see the test-detail view).
export function notCheckedExtractRequirements(c: CatalogExtractCase): string[] {
  const items: string[] = []
  if (!c.fields || Object.keys(c.fields).length === 0) items.push('Классификация')
  if (!c.text_contains_all || c.text_contains_all.length === 0) items.push('Обязательные фразы')
  if (!c.identify_contains_all || c.identify_contains_all.length === 0) items.push('Тема/описание (обязательно)')
  if (!c.identify_contains_any || c.identify_contains_any.length === 0) items.push('Тема/описание (любое из)')
  if (!c.required_numbers || c.required_numbers.length === 0) items.push('Обязательные числа')
  if (!c.forbid_currency) items.push('Запрет валюты')
  return items
}

export interface ScenarioExperimentGroup {
  experiment: string // '' = "Без эксперимента"
  scenarios: CatalogScenario[]
}

// groupScenariosByExperiment buckets scenarios for the catalog tree, preserving
// first-appearance order (same discipline as evalMatrix.ts's groupTestCases) — never
// alphabetized, so a bake-off's scenarios stay grouped the way the repo defines them.
export function groupScenariosByExperiment(scenarios: CatalogScenario[]): ScenarioExperimentGroup[] {
  const order: string[] = []
  const byExperiment = new Map<string, CatalogScenario[]>()
  for (const s of scenarios) {
    const key = s.experiment ?? ''
    if (!byExperiment.has(key)) {
      byExperiment.set(key, [])
      order.push(key)
    }
    byExperiment.get(key)!.push(s)
  }
  return order.map((experiment) => ({ experiment, scenarios: byExperiment.get(experiment) ?? [] }))
}
