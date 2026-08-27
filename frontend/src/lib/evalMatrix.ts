import type { RunSummary, VContractRow, VExecution } from '../types'

// Pure decision logic for the eval comparison UI — NO Vue imports, so it's testable
// in plain Vitest and components stay thin views over it. Mirrors the family-specific
// metric discipline of evals/harness/html.go's formatVCost/aggregateScenarioModelStats,
// re-derived here client-side from the SAME executions.json the Go side already emits
// (never a second source of truth for the underlying pass/fail facts — only the
// pivot/grouping shape is computed here).

export type Family = 'scenario' | 'extract'

export interface MatrixCell {
  setup: string
  model: string
  n: number
  // null means "not applicable to this family" (review amendment 6: never a fake
  // shared metric) — e.g. contractPass is always null for the extract family.
  behaviorPass: number | null
  contractPass: number | null
  allChecksPass: number | null
  parsePass: number | null
  costLabel: string
  avgLatencyMs: number | null
}

// Translate is the narrow slice of vue-i18n's t() the presentational helpers
// below need. Passed in rather than imported so this module stays pure and
// unit-testable in the node project (no localStorage, no app singleton), and
// so a caller always renders in the locale that is active right now.
export type Translate = (key: string, named?: Record<string, unknown>) => string

export interface MatrixGroup {
  experiment: string
  family: Family
  setups: string[]
  models: string[]
  cells: MatrixCell[]
  // Set only for a group split off by the comparability guard (review amendment 3) —
  // its setup(s) disagree with the rest of the experiment on which tests/cases they
  // cover, so pooling them into one table would silently misrepresent the comparison.
  // Carries the two operands, not a sentence: the component renders it through
  // evals.matrix.incomparable so the warning follows the UI locale.
  warning?: { setup: string; experiment: string }
}

function itemID(e: VExecution): string {
  return e.subject.test_id || e.subject.case_id || ''
}

// setupKey is the ONE fallback chain for "which comparison column does this
// execution belong to" — used everywhere a setup needs deriving, so the matrix and
// the test-case filters can never disagree about it. Regression fix: a run from
// before scenario.yaml's setup/prompt_ref annotations existed (or extraction, which
// has no `setup` concept at all — see viewmodel.go's extractExecutionFromResult,
// which sets Setup to the prompt ref string) used to fall back straight to
// variant.model, producing a nonsensical models-as-columns matrix for legacy scenario
// runs. Falling back to subject.scenario first (the scenario name — always present
// for the chat family, always meaningful as a column) fixes that; model stays the
// last resort for anything with neither (shouldn't happen for real data, but the
// chain must still terminate in something rather than an empty string).
function setupKey(e: VExecution): string {
  return e.variant.setup || e.subject.scenario || e.variant.model
}

function idSet(execs: VExecution[]): Set<string> {
  return new Set(execs.map(itemID).filter(Boolean))
}

function setsEqual(a: Set<string>, b: Set<string>): boolean {
  if (a.size !== b.size) return false
  for (const x of a) if (!b.has(x)) return false
  return true
}

function firstAppearanceOrder(values: string[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const v of values) {
    if (v && !seen.has(v)) {
      seen.add(v)
      out.push(v)
    }
  }
  return out
}

// costLabel mirrors evals/harness/html.go's formatVCost — a dollar figure is only
// ever shown alongside what it's based on; every other basis reads as an explicit
// non-number label, never a bare "$0" that could be misread as free or measured.
export function costLabel(basis: string, estimateUSD: number, t: Translate): string {
  switch (basis) {
    case 'measured_split':
    case 'cached_replay_borrowed':
      return `$${estimateUSD.toFixed(5)}`
    case 'cached_replay_unpriceable':
      return t('evals.cost.cachedUnpriceable')
    case 'unknown_pricing':
      return t('evals.cost.unknownPricing')
    default:
      return t('evals.na')
  }
}

function buildCell(setup: string, model: string, execs: VExecution[], t: Translate): MatrixCell {
  let behaviorPass = 0
  let contractPass = 0
  let allChecksPass = 0
  let parsePass = 0
  let isScenario = false
  let isExtract = false
  let costSum = 0
  let pricedN = 0
  let anyUnpriced = false
  let latencySum = 0
  let latencyN = 0

  for (const e of execs) {
    if (e.family === 'scenario') isScenario = true
    if (e.family === 'extract') isExtract = true
    if (e.output.parse_ok) parsePass++
    for (const r of e.rollups) {
      if (r.key === 'model_behavior_pass' && r.pass) behaviorPass++
      if (r.key === 'contract_pass' && r.pass) contractPass++
      if (r.key === 'all_checks_pass' && r.pass) allChecksPass++
    }
    if (e.cost.basis === 'measured_split' || e.cost.basis === 'cached_replay_borrowed') {
      costSum += e.cost.estimate_usd
      pricedN++
    } else {
      anyUnpriced = true
    }
    if (e.latency_ms) {
      latencySum += e.latency_ms
      latencyN++
    }
  }

  const n = execs.length
  let cost = t('evals.na')
  if (pricedN > 0) {
    const avg = `~$${(costSum / pricedN).toFixed(5)}`
    cost = anyUnpriced
      ? t('evals.cost.avgPartial', { avg, priced: pricedN, total: n })
      : t('evals.cost.avg', { avg })
  } else if (n > 0) {
    cost = t('evals.cost.unknownPricing')
  }

  return {
    setup,
    model,
    n,
    behaviorPass: isScenario ? behaviorPass : null,
    contractPass: isScenario ? contractPass : null,
    allChecksPass: isExtract ? allChecksPass : null,
    parsePass: isExtract ? parsePass : null,
    costLabel: cost,
    avgLatencyMs: latencyN > 0 ? Math.round(latencySum / latencyN) : null,
  }
}

function buildOneMatrix(
  experiment: string,
  family: Family,
  setups: string[],
  bySetup: Map<string, VExecution[]>,
  t: Translate,
): MatrixGroup {
  const allExecs = setups.flatMap((s) => bySetup.get(s) || [])
  const models = firstAppearanceOrder(allExecs.map((e) => e.variant.model))
  const cells: MatrixCell[] = []
  for (const setup of setups) {
    for (const model of models) {
      const execs = (bySetup.get(setup) || []).filter((e) => e.variant.model === model)
      if (execs.length === 0) continue
      cells.push(buildCell(setup, model, execs, t))
    }
  }
  return { experiment, family, setups, models, cells }
}

// buildComparisonMatrices is the eval viewer's core aggregation: group executions by
// experiment (empty string = "no experiment declared"), then within each experiment
// group split off any setup whose covered test/case-id set disagrees with the
// group's reference set into its OWN separate matrix with a warning — the
// comparability guard (review amendment 3) that keeps a matrix from silently
// averaging pass rates across columns that never ran the same questions.
export function buildComparisonMatrices(executions: VExecution[], family: Family, t: Translate): MatrixGroup[] {
  const inFamily = executions.filter((e) => e.family === family)
  if (inFamily.length === 0) return []

  const byExperiment = new Map<string, VExecution[]>()
  for (const e of inFamily) {
    const key = e.variant.experiment || ''
    if (!byExperiment.has(key)) byExperiment.set(key, [])
    byExperiment.get(key)!.push(e)
  }

  const groups: MatrixGroup[] = []
  for (const [experiment, execs] of byExperiment) {
    const setups = firstAppearanceOrder(execs.map(setupKey))
    const bySetup = new Map<string, VExecution[]>()
    for (const s of setups) bySetup.set(s, execs.filter((e) => setupKey(e) === s))

    // Reference set: the item-id set covered by the FIRST setup in this experiment —
    // arbitrary but deterministic; every other setup in the group is compared to it.
    const reference = setups.length > 0 ? idSet(bySetup.get(setups[0])!) : new Set<string>()
    const comparable: string[] = []
    const mismatched: string[] = []
    for (const s of setups) {
      if (setsEqual(idSet(bySetup.get(s)!), reference)) comparable.push(s)
      else mismatched.push(s)
    }

    if (comparable.length > 0) {
      groups.push(buildOneMatrix(experiment, family, comparable, bySetup, t))
    }
    for (const s of mismatched) {
      const g = buildOneMatrix(experiment, family, [s], bySetup, t)
      g.warning = { setup: s, experiment }
      groups.push(g)
    }
  }
  return groups
}

export function cellFor(group: MatrixGroup, setup: string, model: string): MatrixCell | undefined {
  return group.cells.find((c) => c.setup === setup && c.model === model)
}

export function pct(pass: number | null, total: number, t: Translate): string {
  if (pass === null || total === 0) return t('evals.na')
  return `${Math.round((pass / total) * 100)}% (${pass}/${total})`
}

// recommendedSetup picks the "★ Лучший результат" column: the setup with the
// strictly highest pooled family-metric pass rate (behaviorPass for scenario,
// allChecksPass for extract — same metric the matrix cell itself leads with,
// never a different one). Pools every model's n/pass for that setup together
// (this is a "which strategy wins overall" badge, not a per-model claim). Returns
// null on a tie for the max, or when nothing in the group has gradeable data —
// a badge is only trustworthy when there is one unambiguous winner, never guessed.
export function recommendedSetup(group: MatrixGroup): string | null {
  const rates = group.setups.map((setup) => {
    let pass = 0
    let total = 0
    for (const c of group.cells) {
      if (c.setup !== setup) continue
      const p = group.family === 'scenario' ? c.behaviorPass : c.allChecksPass
      if (p !== null) {
        pass += p
        total += c.n
      }
    }
    return { setup, total, rate: total > 0 ? pass / total : -1 }
  })
  const withData = rates.filter((r) => r.total > 0)
  if (withData.length === 0) return null
  const maxRate = Math.max(...withData.map((r) => r.rate))
  const winners = withData.filter((r) => r.rate === maxRate)
  return winners.length === 1 ? winners[0].setup : null
}

export interface SetupFrame {
  ref: string // "name@vN", '' when the execution never set a prompt ref (legacy/unannotated)
  sha256?: string
  scenario: string // chat family only; '' for extract (no scenario concept — see viewmodel.go)
}

// setupFrames lists the distinct (prompt ref, scenario) pairs behind one setup —
// the strategy dialog's "frames included" list and the prompt-viewer's targets.
// A routed strategy (e.g. lang-v4-routed = lang-canary-v4-kk + lang-canary-v4-ru)
// returns TWO frames here even though it's ONE matrix column, by design.
export function setupFrames(executions: VExecution[], family: Family, setup: string): SetupFrame[] {
  const seen = new Set<string>()
  const out: SetupFrame[] = []
  for (const e of executions) {
    if (e.family !== family || setupKey(e) !== setup) continue
    const ref = e.variant.prompt?.name ? `${e.variant.prompt.name}@v${e.variant.prompt.version}` : ''
    const scenario = e.subject.scenario || ''
    const key = ref + '|' + scenario
    if (seen.has(key)) continue
    seen.add(key)
    out.push({ ref, sha256: e.variant.prompt?.sha256, scenario })
  }
  return out
}

// execIsPass is the ONE per-execution pass/fail read — the family-appropriate
// rollup, same keys buildCell already sums. Exported so components can render a
// per-model pass/fail chip without re-deriving the rollup-key mapping themselves.
export function execIsPass(e: VExecution): boolean {
  const key = e.family === 'scenario' ? 'model_behavior_pass' : 'all_checks_pass'
  return e.rollups.find((r) => r.key === key)?.pass ?? false
}

export interface TestCaseGroup {
  id: string
  message: string
  execs: VExecution[]
  passCount: number
  total: number
}

// caseLevelRequirements reads the Requirements panel's "what does this test/case
// expect" rows once per group, from the FIRST execution — Expected is defined by the
// test/case itself (evals/harness/viewmodel.go's scenarioRequirementRows/
// extractRequirementRows), never by which model or prompt answered it, so every
// execution in one group carries an identical Expected column and sampling the first
// is never lossy. Only "requirement" rows (never "safety") — those vary per model
// (e.g. valid_json can differ call to call) and belong in the per-execution table, not
// a case-level summary.
export function caseLevelRequirements(group: TestCaseGroup): VContractRow[] {
  return (group.execs[0]?.contract ?? []).filter((r) => r.kind === 'requirement')
}

export interface TestCaseFilter {
  setup?: string // '' = any
  model?: string // '' = any
  status?: 'pass' | 'fail' | '' // '' = any
  sort?: 'default' | 'failuresFirst'
}

// groupTestCases is the launch detail page's evidence list: every execution bucketed
// by its test/case id (preserving first-appearance order — the same order the
// scenario/case bank itself defines), each group carrying its own passCount/total so
// a card can show "3/4 моделей прошли" without a component re-deriving it. Filtering
// happens BEFORE grouping (a setup/model/status filter narrows which executions are
// even considered), matching the matrix's own cell-click-to-filter behavior.
export function groupTestCases(executions: VExecution[], filter: TestCaseFilter): TestCaseGroup[] {
  let execs = executions
  if (filter.setup) execs = execs.filter((e) => setupKey(e) === filter.setup)
  if (filter.model) execs = execs.filter((e) => e.variant.model === filter.model)
  if (filter.status === 'pass') execs = execs.filter((e) => execIsPass(e))
  else if (filter.status === 'fail') execs = execs.filter((e) => !execIsPass(e))

  const byID = new Map<string, TestCaseGroup>()
  const order: string[] = []
  for (const e of execs) {
    const id = e.subject.test_id || e.subject.case_id || ''
    if (!byID.has(id)) {
      byID.set(id, { id, message: e.subject.message || e.subject.case_id || id, execs: [], passCount: 0, total: 0 })
      order.push(id)
    }
    const g = byID.get(id)!
    g.execs.push(e)
    g.total++
    if (execIsPass(e)) g.passCount++
  }

  const groups = order.map((id) => byID.get(id)!)
  if (filter.sort === 'failuresFirst') {
    // Stable partition (Array.prototype.sort is stable per spec): any-failure groups
    // first, in their original relative order, then all-pass groups.
    return [...groups].sort((a, b) => Number(a.passCount === a.total) - Number(b.passCount === b.total))
  }
  return groups
}

// deriveLaunchStatus is the display-time fallback for a launch with no
// runs/launches/<id>.json (a run started directly via `run`/`extract`, never through
// `harness launch` — the common case: a solo run is its own singleton launch). Purely
// presentational — "complete" if the launch has at least one member run recorded,
// "unknown" if somehow neither exists (shouldn't happen for a launch id that appears
// in runs.json at all, but never silently invents a status either way).
export function deriveLaunchStatus(hasAnyMember: boolean): 'complete' | 'unknown' {
  return hasAnyMember ? 'complete' : 'unknown'
}

export interface LaunchGroup {
  launchID: string
  runs: RunSummary[]
}

// groupRunsByLaunch is the launches-list page's one aggregation: every run.json row
// grouped by launch_id — which, per index.go's buildRunsIndexRow fallback, is ALWAYS
// populated (a run started outside `harness launch` falls back to its own run_id, so
// it shows up as a singleton launch, never a special "no launch" case here). Newest
// first — launch/run ids are timestamp-prefixed, so a plain string sort orders them
// correctly without parsing a date.
export function groupRunsByLaunch(runs: RunSummary[]): LaunchGroup[] {
  const byLaunch = new Map<string, RunSummary[]>()
  for (const r of runs) {
    const key = r.launch_id || r.run_id
    if (!byLaunch.has(key)) byLaunch.set(key, [])
    byLaunch.get(key)!.push(r)
  }
  const groups = Array.from(byLaunch, ([launchID, runs]) => ({ launchID, runs }))
  groups.sort((a, b) => b.launchID.localeCompare(a.launchID))
  return groups
}

// --- /evals dashboard row logic. This page is a directory: find a launch, see
// what was tested, see if it passed, and open it. No cross-launch aggregate lives
// here; everything below is per-row.

export type PassBucket = 'green' | 'amber' | 'red' | 'none'

// Thresholds named, not inlined, so the boundary is a single place to read/change —
// and to test the >= vs > distinction precisely (0.80 must read green, 0.79 amber).
export const PASS_BUCKET_GREEN_MIN = 0.8
export const PASS_BUCKET_AMBER_MIN = 0.4

// passBucket buckets a pass rate into the row's status-dot/progress-bar color.
// total=0 is its OWN bucket ('none', rendered neutral gray) — a launch with zero
// gradeable results is not "failing," it's "nothing to report," same "not_run vs
// fail" discipline as evals/harness/viewmodel.go's ScoreStatus.
export function passBucket(pass: number, total: number): PassBucket {
  if (total <= 0) return 'none'
  const rate = pass / total
  if (rate >= PASS_BUCKET_GREEN_MIN) return 'green'
  if (rate >= PASS_BUCKET_AMBER_MIN) return 'amber'
  return 'red'
}

// shortModelName strips everything up to the last "/" (provider prefix), keeping a
// trailing " [label]" disambiguator (providerModelKey's format, judge.go) intact —
// "openrouter:google/gemini-2.5-flash [reasoning-on]" -> "gemini-2.5-flash
// [reasoning-on]". The full id is still the right thing for a title/tooltip; this is
// display-only.
export function shortModelName(id: string): string {
  const m = id.match(/^(.*?)(\s\[.+\])$/)
  const base = m ? m[1] : id
  const suffix = m ? m[2] : ''
  const segments = base.split('/')
  return segments[segments.length - 1] + suffix
}

// formatStartedAt renders an RFC3339 UTC timestamp (every timestamp this app reads
// comes from evals/harness's time.Now().UTC() calls) as "14 июля 2026, 15:53" /
// "14 July 2026, 15:53" / "14 шілде 2026, 15:53" — assembled from the catalog's
// month list rather than Intl.DateTimeFormat so the exact shape is deterministic
// across environments/locale-data versions, not just "close enough."
export function formatStartedAt(iso: string | undefined, t: Translate): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return ''
  const day = d.getUTCDate()
  const month = t(`evals.months.${d.getUTCMonth()}`)
  const year = d.getUTCFullYear()
  const hh = String(d.getUTCHours()).padStart(2, '0')
  const mm = String(d.getUTCMinutes()).padStart(2, '0')
  return `${day} ${month} ${year}, ${hh}:${mm}`
}

// formatDuration renders a millisecond span as "38с" / "18м 42с" / "1ч 05м" — seconds
// dropped once the span reaches an hour, matching the reference design's examples.
// Unit letters come from the catalog (evals.duration.*), so kk/en read as "38 с" /
// "38s" rather than a Cyrillic suffix on an English page.
export function formatDuration(ms: number, t: Translate): string {
  const totalSeconds = Math.max(0, Math.round(ms / 1000))
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  if (hours > 0) return t('evals.duration.hm', { h: hours, m: String(minutes).padStart(2, '0') })
  if (minutes > 0) return t('evals.duration.ms', { m: minutes, s: String(seconds).padStart(2, '0') })
  return t('evals.duration.s', { s: seconds })
}

export interface LaunchListRow {
  id: string
  shortId: string
  startedAt: string // '' if no member run reports one
  families: string[] // raw family keys ("scenario"/"extract"/"mixed"/"unknown") — components own the display label
  models: string[]
  pass: number
  total: number
  bucket: PassBucket
  durationMs: number | null // null when any member run is missing started_at/finished_at — never a partial guess
}

// launchListRow computes one launch-directory row. Pass/total sums the same way
// EvalRuns.vue's original passSummary did (never a
// second, different aggregation for the same fact). Duration is the outer span
// across every member run (earliest start to latest finish), not a sum of member
// durations — a launch's two members ran concurrently, not back to back.
export function launchListRow(group: LaunchGroup): LaunchListRow {
  const shortId = group.launchID.split('-').pop() || group.launchID
  let pass = 0
  let total = 0
  const families = new Set<string>()
  const models = new Set<string>()
  let minStarted: number | null = null
  let maxFinished: number | null = null
  let earliestStartedAt = ''
  let missingTimestamp = false

  for (const r of group.runs) {
    pass += r.scenario_behavior_pass + r.extract_checks_pass
    total += r.scenario_total + r.extract_total
    families.add(r.family)
    for (const m of r.models) models.add(m)

    if (r.started_at) {
      const t = Date.parse(r.started_at)
      if (!isNaN(t) && (minStarted === null || t < minStarted)) {
        minStarted = t
        earliestStartedAt = r.started_at
      }
    } else {
      missingTimestamp = true
    }
    if (r.finished_at) {
      const t = Date.parse(r.finished_at)
      if (!isNaN(t) && (maxFinished === null || t > maxFinished)) maxFinished = t
    } else {
      missingTimestamp = true
    }
  }

  return {
    id: group.launchID,
    shortId,
    startedAt: earliestStartedAt,
    families: Array.from(families),
    models: Array.from(models),
    pass,
    total,
    bucket: passBucket(pass, total),
    durationMs: !missingTimestamp && minStarted !== null && maxFinished !== null ? maxFinished - minStarted : null,
  }
}

export interface LaunchFilter {
  query?: string
  family?: string // '' = any
  model?: string // '' = any
  bucket?: PassBucket | '' // '' = any
}

// filterLaunches applies the dashboard's search box + three selects — AND across
// dimensions, exact match within a dimension (family/model membership, bucket
// equality), substring on the launch id for the free-text search.
export function filterLaunches(rows: LaunchListRow[], filter: LaunchFilter): LaunchListRow[] {
  const query = (filter.query || '').trim().toLowerCase()
  return rows.filter((r) => {
    if (query && !r.id.toLowerCase().includes(query)) return false
    if (filter.family && !r.families.includes(filter.family)) return false
    if (filter.model && !r.models.includes(filter.model)) return false
    if (filter.bucket && r.bucket !== filter.bucket) return false
    return true
  })
}
