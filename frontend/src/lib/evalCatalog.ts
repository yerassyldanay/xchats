import type { CatalogExtractCase, CatalogFact, CatalogMediaExpect, CatalogScenario, CatalogTestCase } from '../types'
import type { TestFieldKey } from './evalFieldDocs'

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

export interface MediaExpectAnnotation {
  key: 'any_of' | 'all_of'
  name: string
  found: boolean // false = not in this scenario's media_tokens — a broken reference
}

// annotateMediaExpect audits a test's declared media expectation against the
// scenario's real media_tokens — the raw `media` field is displayed as-is (via
// formatYamlValue) regardless; this only adds the "does this token actually exist"
// annotation. A forbid-only expectation has nothing to audit (no tokens named), so it
// deliberately returns [] rather than treating that as broken.
export function annotateMediaExpect(media: CatalogMediaExpect | undefined, mediaTokens: string[] | undefined): MediaExpectAnnotation[] {
  if (!media) return []
  const known = new Set(mediaTokens ?? [])
  const out: MediaExpectAnnotation[] = []
  for (const name of media.any_of ?? []) out.push({ key: 'any_of', name, found: known.has(name) })
  for (const name of media.all_of ?? []) out.push({ key: 'all_of', name, found: known.has(name) })
  return out
}

// notCheckedRequirements lists, by schema key name, which knobs a scenario test does
// NOT declare — mirrors judge.go's own gating EXACTLY (requiresSatisfied/media/
// escalate/language/must_not_contain/must_contain_any/forbid_tokens all default to
// "trivially satisfied" when the test simply doesn't declare that check), so this list
// is never a guess about what's ungraded — it's the direct complement of what judge.go
// actually skips. stock_check/llm_checks are deliberately excluded: they're the
// separate opt-in judge-llm dimension, never part of the plain judge gating this list
// mirrors. No separate branch is needed for media.forbid: `!test.media` already treats
// ANY declared media block (forbid-only or any_of-based) as "checked," matching
// judge.go's own gating on `tc.Media != nil`.
export function notCheckedRequirements(test: CatalogTestCase): TestFieldKey[] {
  const items: TestFieldKey[] = []
  if (!test.requires || test.requires.length === 0) items.push('requires')
  if (test.language !== 'kk' && test.language !== 'ru') items.push('language')
  if (test.escalate === undefined) items.push('escalate')
  if (!test.media) items.push('media')
  if (!test.must_not_contain || test.must_not_contain.length === 0) items.push('must_not_contain')
  if (!test.must_contain_any || test.must_contain_any.length === 0) items.push('must_contain_any')
  if (!test.forbid_tokens || test.forbid_tokens.length === 0) items.push('forbid_tokens')
  if (!test.outcomes || test.outcomes.length === 0) items.push('outcomes')
  return items
}

// notCheckedExtractRequirements mirrors extract_checks.go's own opt-in gating.
// no_invented_numbers is DELIBERATELY excluded here — it always runs (fail-closed
// default) regardless of whether allowed_numbers is declared or empty, so it's never
// "not checked"; it always renders as an active requirement (see the test-detail view).
// Returns schema key names, not labels: the component renders each through
// evalCatalog.extract.<key> so the same wording serves this list and the
// section heading above it, in whatever locale is active.
export type ExtractFieldKey =
  | 'fields'
  | 'text_contains_all'
  | 'identify_contains_all'
  | 'identify_contains_any'
  | 'required_numbers'
  | 'forbid_currency'
export function notCheckedExtractRequirements(c: CatalogExtractCase): ExtractFieldKey[] {
  const items: ExtractFieldKey[] = []
  if (!c.fields || Object.keys(c.fields).length === 0) items.push('fields')
  if (!c.text_contains_all || c.text_contains_all.length === 0) items.push('text_contains_all')
  if (!c.identify_contains_all || c.identify_contains_all.length === 0) items.push('identify_contains_all')
  if (!c.identify_contains_any || c.identify_contains_any.length === 0) items.push('identify_contains_any')
  if (!c.required_numbers || c.required_numbers.length === 0) items.push('required_numbers')
  if (!c.forbid_currency) items.push('forbid_currency')
  return items
}

export interface ScenarioExperimentGroup {
  experiment: string // '' = "no experiment declared" (rendered via evals.noExperiment)
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

export interface ArchivedPartition {
  active: CatalogScenario[]
  archived: CatalogScenario[]
}

// partitionScenariosByArchived splits scenarios so the tree can hide archived ones
// behind a toggle by default — the Go harness already excludes archived scenarios from
// every run path (run.go/launch.go), this brings the sidebar in line. Preserves
// first-appearance order within each bucket (same discipline as
// groupScenariosByExperiment). A missing `archived` field (stale v2 catalog.json) is
// treated as active, never archived.
export function partitionScenariosByArchived(scenarios: CatalogScenario[]): ArchivedPartition {
  const active: CatalogScenario[] = []
  const archived: CatalogScenario[] = []
  for (const s of scenarios) {
    ;(s.archived === true ? archived : active).push(s)
  }
  return { active, archived }
}

function formatYamlScalarOrFlow(v: unknown): string {
  if (Array.isArray(v)) return `[${v.map(formatYamlScalarOrFlow).join(', ')}]`
  if (v === null || v === undefined) return 'null'
  return String(v)
}

function formatYamlObjectListItem(obj: Record<string, unknown>, pad: string): string {
  const entries = Object.entries(obj)
  if (entries.length === 0) return `${pad}- {}`
  const [firstKey, firstVal] = entries[0]
  const lines = [`${pad}- ${firstKey}: ${formatYamlScalarOrFlow(firstVal)}`]
  for (const [k, v] of entries.slice(1)) lines.push(`${pad}  ${k}: ${formatYamlScalarOrFlow(v)}`)
  return lines.join('\n')
}

// formatYamlValue renders a resolved test/scenario field as YAML-ish text for a raw
// display <pre> block — display only (unquoted scalars, no escaping), never re-parsed.
// Insertion order is preserved throughout; nothing here ever sorts.
export function formatYamlValue(value: unknown, indent = 0): string {
  const pad = '  '.repeat(indent)
  if (value === null || value === undefined) return `${pad}null`
  if (Array.isArray(value)) {
    if (value.length === 0) return `${pad}[]`
    if (value.every((v) => Array.isArray(v))) {
      return (value as unknown[][]).map((v) => `${pad}- [${v.map(formatYamlScalarOrFlow).join(', ')}]`).join('\n')
    }
    if (value.every((v) => v === null || typeof v !== 'object')) {
      return (value as unknown[]).map((v) => `${pad}- ${formatYamlScalarOrFlow(v)}`).join('\n')
    }
    return (value as Record<string, unknown>[]).map((v) => formatYamlObjectListItem(v, pad)).join('\n')
  }
  if (typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
    if (entries.length === 0) return `${pad}{}`
    return entries.map(([k, v]) => `${pad}${k}: ${formatYamlScalarOrFlow(v)}`).join('\n')
  }
  return `${pad}${value}`
}

// scenarioNavLabel is the sidebar's file-path label for a scenario — the actual
// tests.yaml path, not the (possibly duplicated) setup/name string. A scenario
// borrowing another's tests.yaml (ScenarioConfig.Tests: "../other/tests.yaml") is shown
// as "borrower → owner/tests.yaml" rather than silently under the borrower's own name.
export function scenarioNavLabel(s: Pick<CatalogScenario, 'name' | 'tests_path'>): string {
  if (!s.tests_path) return `${s.name}/tests.yaml` // stale v2 catalog.json fallback
  const rel = s.tests_path.startsWith('scenarios/') ? s.tests_path.slice('scenarios/'.length) : s.tests_path
  if (rel === `${s.name}/tests.yaml` || rel.startsWith(`${s.name}/`)) return rel
  return `${s.name} → ${rel}`
}
