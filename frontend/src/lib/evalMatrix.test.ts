import { describe, expect, it } from 'vitest'
import ru from '../i18n/locales/ru'
import type { RunSummary, VContractRow, VExecution } from '../types'
import {
  buildComparisonMatrices,
  caseLevelRequirements,
  cellFor,
  costLabel,
  deriveLaunchStatus,
  execIsPass,
  filterLaunches,
  formatDuration,
  formatStartedAt,
  groupRunsByLaunch,
  groupTestCases,
  launchListRow,
  passBucket,
  pct,
  recommendedSetup,
  setupFrames,
  shortModelName,
  type LaunchListRow,
} from './evalMatrix'

// t resolves against the REAL ru catalog rather than echoing keys back, so the
// assertions below still pin the exact Russian shipped in the UI. Built here from
// the plain message object instead of i18n/index.ts's vue-i18n instance: this test
// runs in the `unit` project (environment: 'node'), where that module's
// localStorage read at import time would throw.
function t(key: string, named: Record<string, unknown> = {}): string {
  const value = key.split('.').reduce<unknown>((node, part) => (node as Record<string, unknown>)?.[part], ru)
  if (typeof value !== 'string') throw new Error(`no ru message for ${key}`)
  return value.replace(/\{(\w+)\}/g, (whole, name: string) => (name in named ? String(named[name]) : whole))
}

// scenarioExec is the fixture builder — every field a real VExecution carries,
// defaulted to the "everything passed" shape so each test only overrides what it's
// actually exercising (mirrors evals/harness's own Go test fixture style). `scenario`
// defaults to `setup` (most fixtures don't care about the distinction); pass it
// explicitly to test the setup-vs-scenario-name fallback chain itself.
function scenarioExec(over: {
  testID: string
  setup?: string
  scenario?: string
  experiment?: string
  model: string
  message?: string
  behaviorPass?: boolean
  contractPass?: boolean
  costBasis?: string
  estimateUSD?: number
  latencyMs?: number
  promptRef?: { name: string; version: number; sha256?: string }
}): VExecution {
  const behaviorPass = over.behaviorPass ?? true
  const contractPass = over.contractPass ?? true
  return {
    family: 'scenario',
    subject: { test_id: over.testID, scenario: over.scenario ?? over.setup, message: over.message },
    variant: { model: over.model, setup: over.setup, experiment: over.experiment, prompt: over.promptRef },
    output: { parse_ok: true },
    scores: [],
    rollups: [
      { key: 'model_behavior_pass', label: 'Model-behavior pass', pass: behaviorPass },
      { key: 'contract_pass', label: 'Contract pass', pass: contractPass },
    ],
    cost: { tokens_in: 100, tokens_out: 50, estimate_usd: over.estimateUSD ?? 0.0002, basis: over.costBasis ?? 'measured_split' },
    latency_ms: over.latencyMs ?? 900,
  }
}

describe('buildComparisonMatrices — scenario family', () => {
  it('aggregates pass rate, cost, and latency per (setup, model) cell', () => {
    const execs: VExecution[] = [
      scenarioExec({ testID: 't1', setup: 'lang-v1', experiment: 'lang-bakeoff', model: 'm1', behaviorPass: true, latencyMs: 800 }),
      scenarioExec({ testID: 't2', setup: 'lang-v1', experiment: 'lang-bakeoff', model: 'm1', behaviorPass: false, latencyMs: 1000 }),
      scenarioExec({ testID: 't1', setup: 'lang-v1', experiment: 'lang-bakeoff', model: 'm2', behaviorPass: true }),
      scenarioExec({ testID: 't2', setup: 'lang-v1', experiment: 'lang-bakeoff', model: 'm2', behaviorPass: true }),
    ]
    const groups = buildComparisonMatrices(execs, 'scenario', t)
    expect(groups).toHaveLength(1)
    const g = groups[0]
    expect(g.experiment).toBe('lang-bakeoff')
    expect(g.setups).toEqual(['lang-v1'])
    expect(g.models).toEqual(['m1', 'm2'])

    const m1 = cellFor(g, 'lang-v1', 'm1')!
    expect(m1.n).toBe(2)
    expect(m1.behaviorPass).toBe(1)
    expect(m1.avgLatencyMs).toBe(900) // (800+1000)/2

    const m2 = cellFor(g, 'lang-v1', 'm2')!
    expect(m2.behaviorPass).toBe(2)

    // Scenario family: behaviorPass/contractPass populated, allChecksPass/parsePass
    // null (review amendment 6 — never a fake shared metric).
    expect(m1.contractPass).not.toBeNull()
    expect(m1.allChecksPass).toBeNull()
    expect(m1.parsePass).toBeNull()
  })

  it('the routed-strategy story: two prompt_refs sharing ONE setup pool into ONE column', () => {
    // Mirrors evals/scenarios/lang-canary-v4-kk + v4-ru: two DIFFERENT scenario
    // dirs, two DIFFERENT prompt refs, but the SAME setup value — the matrix must
    // show one "lang-v4-routed" column with pooled results, not two.
    const execs: VExecution[] = [
      scenarioExec({ testID: 'kk1', setup: 'lang-v4-routed', experiment: 'lang-bakeoff', model: 'm1' }),
      scenarioExec({ testID: 'ru1', setup: 'lang-v4-routed', experiment: 'lang-bakeoff', model: 'm1' }),
    ]
    execs[0].variant.prompt = { name: 'lang-kk', version: 4 }
    execs[1].variant.prompt = { name: 'lang-ru', version: 4 }

    const groups = buildComparisonMatrices(execs, 'scenario', t)
    expect(groups).toHaveLength(1)
    expect(groups[0].setups).toEqual(['lang-v4-routed'])
    expect(cellFor(groups[0], 'lang-v4-routed', 'm1')!.n).toBe(2)
  })

  it('the comparability guard: a setup with a DIFFERENT test-id set is split into its own warned matrix, never silently pooled', () => {
    // V1-V3 run a 6-test bank; V4-kk/V4-ru each run a 3-test half of it — the
    // README's own documented shape. Simulate the mismatch directly: lang-v2 covers
    // {t1,t2}, lang-v3 covers only {t1} — genuinely different test sets.
    const execs: VExecution[] = [
      scenarioExec({ testID: 't1', setup: 'lang-v2', experiment: 'lang-bakeoff', model: 'm1' }),
      scenarioExec({ testID: 't2', setup: 'lang-v2', experiment: 'lang-bakeoff', model: 'm1' }),
      scenarioExec({ testID: 't1', setup: 'lang-v3', experiment: 'lang-bakeoff', model: 'm1' }),
    ]
    const groups = buildComparisonMatrices(execs, 'scenario', t)
    expect(groups).toHaveLength(2)

    const comparableGroup = groups.find((g) => !g.warning)!
    const warnedGroup = groups.find((g) => g.warning)!
    expect(comparableGroup.setups).toEqual(['lang-v2'])
    expect(warnedGroup.setups).toEqual(['lang-v3'])
    expect(warnedGroup.warning).toEqual({ setup: 'lang-v3', experiment: 'lang-bakeoff' })
  })

  it('never pools across different experiments', () => {
    const execs: VExecution[] = [
      scenarioExec({ testID: 't1', setup: 'lang-v1', experiment: 'lang-bakeoff', model: 'm1' }),
      scenarioExec({ testID: 't1', setup: 'escalation-v1', experiment: 'escalation-bakeoff', model: 'm1' }),
    ]
    const groups = buildComparisonMatrices(execs, 'scenario', t)
    expect(groups.map((g) => g.experiment).sort()).toEqual(['escalation-bakeoff', 'lang-bakeoff'])
  })

  it('an unannotated scenario (no setup/experiment) falls back to the scenario name and is never dropped', () => {
    const execs: VExecution[] = [scenarioExec({ testID: 't1', model: 'm1', scenario: 'shop-current' })]
    const groups = buildComparisonMatrices(execs, 'scenario', t)
    expect(groups).toHaveLength(1)
    expect(groups[0].experiment).toBe('')
    expect(groups[0].setups).toEqual(['shop-current'])
  })

  // Regression test for the bug caught while planning the launch-detail redesign:
  // a run from BEFORE scenario.yaml's setup/prompt_ref annotations existed has
  // variant.setup entirely empty, and the matrix used to fall back straight to
  // variant.model — producing a nonsensical models-as-columns matrix instead of
  // grouping by scenario. Two scenarios × two models must produce scenario-named
  // COLUMNS with both models as ROWS, never the reverse.
  it('legacy run (no setup annotation at all) groups by scenario name, never falls back to model names as columns', () => {
    const execs: VExecution[] = [
      scenarioExec({ testID: 't1', model: 'm1', scenario: 'shop-current' }),
      scenarioExec({ testID: 't1', model: 'm2', scenario: 'shop-current' }),
      scenarioExec({ testID: 't2', model: 'm1', scenario: 'shop-decisions-v1' }),
      scenarioExec({ testID: 't2', model: 'm2', scenario: 'shop-decisions-v1' }),
    ]
    // Both scenarios cover disjoint test-id sets ({t1} vs {t2}), so the
    // comparability guard legitimately splits them into two single-column
    // matrices — the point being that EACH column is scenario-named, not model-named.
    const groups = buildComparisonMatrices(execs, 'scenario', t)
    const allSetups = groups.flatMap((g) => g.setups)
    expect(allSetups.sort()).toEqual(['shop-current', 'shop-decisions-v1'].sort())
    expect(allSetups).not.toContain('m1')
    expect(allSetups).not.toContain('m2')
    for (const g of groups) {
      expect(g.models.sort()).toEqual(['m1', 'm2'])
    }
  })
})

describe('recommendedSetup', () => {
  it('picks the setup with the strictly highest pooled pass rate', () => {
    const execs: VExecution[] = [
      scenarioExec({ testID: 't1', setup: 'v1', model: 'm1', behaviorPass: false }),
      scenarioExec({ testID: 't1', setup: 'v2', model: 'm1', behaviorPass: true }),
    ]
    const [group] = buildComparisonMatrices(execs, 'scenario', t)
    expect(recommendedSetup(group)).toBe('v2')
  })

  it('returns null on a tie — never guesses a winner', () => {
    const execs: VExecution[] = [
      scenarioExec({ testID: 't1', setup: 'v1', model: 'm1', behaviorPass: true }),
      scenarioExec({ testID: 't1', setup: 'v2', model: 'm1', behaviorPass: true }),
    ]
    const [group] = buildComparisonMatrices(execs, 'scenario', t)
    expect(recommendedSetup(group)).toBeNull()
  })

  it('a group with zero gradeable data returns null rather than an arbitrary first setup', () => {
    expect(recommendedSetup({ experiment: '', family: 'scenario', setups: ['v1'], models: [], cells: [] })).toBeNull()
  })
})

describe('setupFrames', () => {
  it('the routed-strategy story: one setup, two distinct (prompt ref, scenario) frames', () => {
    const execs: VExecution[] = [
      scenarioExec({ testID: 'kk1', setup: 'lang-v4-routed', scenario: 'lang-canary-v4-kk', model: 'm1', promptRef: { name: 'lang-kk', version: 4, sha256: 'aaa' } }),
      scenarioExec({ testID: 'ru1', setup: 'lang-v4-routed', scenario: 'lang-canary-v4-ru', model: 'm1', promptRef: { name: 'lang-ru', version: 4, sha256: 'bbb' } }),
    ]
    const frames = setupFrames(execs, 'scenario', 'lang-v4-routed')
    expect(frames).toHaveLength(2)
    expect(frames.map((f) => f.ref).sort()).toEqual(['lang-kk@v4', 'lang-ru@v4'])
    expect(frames.find((f) => f.ref === 'lang-kk@v4')?.scenario).toBe('lang-canary-v4-kk')
    expect(frames.find((f) => f.ref === 'lang-kk@v4')?.sha256).toBe('aaa')
  })

  it('de-duplicates repeated (ref, scenario) pairs across models/repeats', () => {
    const execs: VExecution[] = [
      scenarioExec({ testID: 't1', setup: 'v1', scenario: 's1', model: 'm1', promptRef: { name: 'p', version: 1 } }),
      scenarioExec({ testID: 't1', setup: 'v1', scenario: 's1', model: 'm2', promptRef: { name: 'p', version: 1 } }),
    ]
    expect(setupFrames(execs, 'scenario', 'v1')).toHaveLength(1)
  })

  it('only looks within the requested setup', () => {
    const execs: VExecution[] = [
      scenarioExec({ testID: 't1', setup: 'v1', scenario: 's1', model: 'm1', promptRef: { name: 'p', version: 1 } }),
      scenarioExec({ testID: 't1', setup: 'v2', scenario: 's2', model: 'm1', promptRef: { name: 'q', version: 1 } }),
    ]
    expect(setupFrames(execs, 'scenario', 'v1')).toHaveLength(1)
  })
})

describe('execIsPass', () => {
  it('reads model_behavior_pass for scenario executions', () => {
    expect(execIsPass(scenarioExec({ testID: 't1', model: 'm1', behaviorPass: true }))).toBe(true)
    expect(execIsPass(scenarioExec({ testID: 't1', model: 'm1', behaviorPass: false }))).toBe(false)
  })
})

describe('groupTestCases', () => {
  const execs: VExecution[] = [
    scenarioExec({ testID: 't1', setup: 'v1', model: 'm1', message: 'Hello', behaviorPass: true }),
    scenarioExec({ testID: 't1', setup: 'v1', model: 'm2', message: 'Hello', behaviorPass: false }),
    scenarioExec({ testID: 't2', setup: 'v2', model: 'm1', message: 'Bye', behaviorPass: true }),
  ]

  it('groups by test id, preserving first-appearance order, with a per-group pass count', () => {
    const groups = groupTestCases(execs, {})
    expect(groups.map((g) => g.id)).toEqual(['t1', 't2'])
    expect(groups[0].message).toBe('Hello')
    expect(groups[0].passCount).toBe(1)
    expect(groups[0].total).toBe(2)
  })

  it('filters by setup before grouping', () => {
    expect(groupTestCases(execs, { setup: 'v2' }).map((g) => g.id)).toEqual(['t2'])
  })

  it('filters by model before grouping', () => {
    const groups = groupTestCases(execs, { model: 'm2' })
    expect(groups).toHaveLength(1)
    expect(groups[0].execs).toHaveLength(1)
  })

  it('filters by pass/fail status', () => {
    expect(groupTestCases(execs, { status: 'fail' })[0].execs).toHaveLength(1)
    expect(groupTestCases(execs, { status: 'fail' })[0].execs[0].variant.model).toBe('m2')
  })

  it('failuresFirst sort moves any group with a failure ahead of all-pass groups, stably', () => {
    // t1 has a failure (m2), t2 is all-pass — failuresFirst must put t1 first even
    // though t1 already appears first in insertion order here (weak check); use a
    // reversed-order fixture to actually prove the sort, not just insertion order.
    const reordered: VExecution[] = [
      scenarioExec({ testID: 't2', setup: 'v2', model: 'm1', message: 'Bye', behaviorPass: true }),
      scenarioExec({ testID: 't1', setup: 'v1', model: 'm1', message: 'Hello', behaviorPass: true }),
      scenarioExec({ testID: 't1', setup: 'v1', model: 'm2', message: 'Hello', behaviorPass: false }),
    ]
    const groups = groupTestCases(reordered, { sort: 'failuresFirst' })
    expect(groups.map((g) => g.id)).toEqual(['t1', 't2'])
  })
})

describe('caseLevelRequirements', () => {
  const requirementRow: VContractRow = { key: 'requires', label: 'Обязательные факты', kind: 'requirement', expected: 'a', actual: 'a', pass: true }
  const safetyRow: VContractRow = { key: 'valid_json', label: 'Валидный JSON', kind: 'safety', expected: 'x', actual: 'x', pass: true }

  it('reads requirement rows from the FIRST execution only, dropping safety rows', () => {
    const e1: VExecution = { ...scenarioExec({ testID: 't1', setup: 'v1', model: 'm1' }), contract: [requirementRow, safetyRow] }
    // A second execution with DIFFERENT contract rows proves the group-level summary
    // never merges across executions — Expected is defined by the test itself, so
    // sampling the first execution is enough (and per-model differences, if any ever
    // appeared, must never leak into this case-level summary).
    const e2: VExecution = { ...scenarioExec({ testID: 't1', setup: 'v1', model: 'm2' }), contract: [{ ...requirementRow, actual: 'b', pass: false }] }
    const group = { id: 't1', message: 'Hello', total: 2, passCount: 1, execs: [e1, e2] }

    const rows = caseLevelRequirements(group)
    expect(rows.map((r) => r.key)).toEqual(['requires'])
    expect(rows[0].actual).toBe('a') // from e1, not e2
  })

  it('returns empty when the first execution has no contract data (legacy run, no snapshot)', () => {
    const group = { id: 't1', message: 'Hello', total: 1, passCount: 1, execs: [scenarioExec({ testID: 't1', setup: 'v1', model: 'm1' })] }
    expect(caseLevelRequirements(group)).toEqual([])
  })

  it('returns empty for a group with no executions', () => {
    expect(caseLevelRequirements({ id: 't1', message: '', total: 0, passCount: 0, execs: [] })).toEqual([])
  })
})

describe('costLabel', () => {
  it('shows a dollar figure only for measured/borrowed bases, an explicit label otherwise', () => {
    expect(costLabel('measured_split', 0.00021, t)).toBe('$0.00021')
    expect(costLabel('cached_replay_borrowed', 0.00021, t)).toBe('$0.00021')
    expect(costLabel('cached_replay_unpriceable', 0, t)).not.toMatch(/^\$/)
    expect(costLabel('unknown_pricing', 0, t)).not.toMatch(/^\$/)
    expect(costLabel('', 0, t)).not.toMatch(/^\$/)
  })
})

describe('pct', () => {
  it('formats a pass/total pair, and reports н/д for null or zero total', () => {
    expect(pct(8, 10, t)).toBe('80% (8/10)')
    expect(pct(null, 10, t)).toBe('н/д')
    expect(pct(0, 0, t)).toBe('н/д')
  })
})

describe('deriveLaunchStatus', () => {
  it('is a pure presentational fallback for a launch with no LaunchManifest', () => {
    expect(deriveLaunchStatus(true)).toBe('complete')
    expect(deriveLaunchStatus(false)).toBe('unknown')
  })
})

function summary(over: Partial<RunSummary> & { run_id: string }): RunSummary {
  return {
    launch_id: over.launch_id,
    has_manifest: true,
    family: 'scenario',
    models: [],
    prompts: [],
    scenario_total: 0,
    scenario_behavior_pass: 0,
    scenario_contract_pass: 0,
    extract_total: 0,
    extract_checks_pass: 0,
    has_index_html: false,
    ...over,
  }
}

describe('groupRunsByLaunch', () => {
  it('groups a real `harness launch` pair under their shared launch_id', () => {
    const runs = [
      summary({ run_id: '2026-07-14_10-00-01-aaaa', launch_id: '2026-07-14_10-00-00-launch' }),
      summary({ run_id: '2026-07-14_10-00-02-bbbb', launch_id: '2026-07-14_10-00-00-launch' }),
    ]
    const groups = groupRunsByLaunch(runs)
    expect(groups).toHaveLength(1)
    expect(groups[0].launchID).toBe('2026-07-14_10-00-00-launch')
    expect(groups[0].runs).toHaveLength(2)
  })

  it('a run with no launch_id (never went through `harness launch`) falls back to its own run_id — a singleton launch, never dropped', () => {
    const groups = groupRunsByLaunch([summary({ run_id: '2026-07-14_09-00-00-solo', launch_id: undefined })])
    expect(groups).toHaveLength(1)
    expect(groups[0].launchID).toBe('2026-07-14_09-00-00-solo')
  })

  it('sorts newest first by launch id', () => {
    const groups = groupRunsByLaunch([
      summary({ run_id: 'a', launch_id: '2026-01-01_00-00-00-old' }),
      summary({ run_id: 'b', launch_id: '2026-06-01_00-00-00-new' }),
    ])
    expect(groups.map((g) => g.launchID)).toEqual(['2026-06-01_00-00-00-new', '2026-01-01_00-00-00-old'])
  })
})

describe('passBucket', () => {
  it('buckets at the documented 80%/40% boundaries, inclusive on the low end of each band', () => {
    expect(passBucket(8, 10)).toBe('green') // exactly 80%
    expect(passBucket(79, 100)).toBe('amber') // just under 80%
    expect(passBucket(4, 10)).toBe('amber') // exactly 40%
    expect(passBucket(39, 100)).toBe('red') // just under 40%
    expect(passBucket(0, 10)).toBe('red')
  })
  it('total=0 is its own bucket, never red — "nothing to report," not "failing"', () => {
    expect(passBucket(0, 0)).toBe('none')
  })
})

describe('shortModelName', () => {
  it('keeps only the segment after the last "/"', () => {
    expect(shortModelName('google/gemini-2.5-flash')).toBe('gemini-2.5-flash')
    expect(shortModelName('demo/model-alpha')).toBe('model-alpha')
  })
  it('preserves a trailing " [label]" disambiguator (providerModelKey format)', () => {
    expect(shortModelName('openrouter:google/gemini-2.5-flash [reasoning-on]')).toBe('gemini-2.5-flash [reasoning-on]')
  })
  it('a bare id with no "/" passes through unchanged', () => {
    expect(shortModelName('gpt-4o')).toBe('gpt-4o')
  })
})

describe('formatStartedAt', () => {
  it('renders an RFC3339 UTC timestamp as the reference design\'s exact shape', () => {
    expect(formatStartedAt('2026-07-14T15:53:00Z', t)).toBe('14 июля 2026, 15:53')
  })
  it('returns empty for missing or unparseable input, never a fabricated date', () => {
    expect(formatStartedAt(undefined, t)).toBe('')
    expect(formatStartedAt('', t)).toBe('')
    expect(formatStartedAt('not-a-date', t)).toBe('')
  })
})

describe('formatDuration', () => {
  it('matches the reference design\'s worked examples exactly', () => {
    expect(formatDuration(40 * 1000, t)).toBe('40с')
    expect(formatDuration((18 * 60 + 42) * 1000, t)).toBe('18м 42с')
    expect(formatDuration((24 * 60 + 18) * 1000, t)).toBe('24м 18с')
  })
  it('drops seconds once the span reaches an hour', () => {
    expect(formatDuration((60 * 60 + 5 * 60) * 1000, t)).toBe('1ч 05м')
  })
})

function demoRun(over: Partial<RunSummary> & { run_id: string }): RunSummary {
  return summary(over)
}

describe('launchListRow', () => {
  it('sums pass/total across every member run — the same math the original passSummary used', () => {
    const row = launchListRow({
      launchID: 'L1',
      runs: [
        demoRun({ run_id: 'chat-a', family: 'scenario', models: ['m1'], scenario_total: 12, scenario_behavior_pass: 11 }),
        demoRun({ run_id: 'chat-b', family: 'scenario', models: ['m2'], scenario_total: 4, scenario_behavior_pass: 3 }),
      ],
    })
    expect(row.pass).toBe(14)
    expect(row.total).toBe(16)
    expect(row.bucket).toBe('green') // 14/16 = 87.5%
    expect(row.families).toEqual(['scenario'])
    expect(row.models.sort()).toEqual(['m1', 'm2'])
    expect(row.shortId).toBe('L1') // no "-" in this fixture id, falls back to the whole id
  })

  it('shortId is the last hyphen-delimited segment of a real timestamp id', () => {
    const row = launchListRow({ launchID: '2026-07-14_15-53-39-2cbb', runs: [demoRun({ run_id: 'r' })] })
    expect(row.shortId).toBe('2cbb')
  })

  it('duration spans the EARLIEST start to the LATEST finish across members — concurrent, not summed', () => {
    const row = launchListRow({
      launchID: 'L1',
      runs: [
        demoRun({ run_id: 'chat', started_at: '2026-07-14T10:00:00Z', finished_at: '2026-07-14T10:05:00Z' }),
        demoRun({ run_id: 'chat-b', started_at: '2026-07-14T10:01:00Z', finished_at: '2026-07-14T10:07:00Z' }),
      ],
    })
    // earliest start 10:00:00, latest finish 10:07:00 -> 7 minutes, NOT 5+6=11.
    expect(row.durationMs).toBe(7 * 60 * 1000)
  })

  it('duration is null (never a partial guess) when any member is missing a timestamp', () => {
    const row = launchListRow({
      launchID: 'L1',
      runs: [
        demoRun({ run_id: 'chat', started_at: '2026-07-14T10:00:00Z', finished_at: '2026-07-14T10:05:00Z' }),
        demoRun({ run_id: 'chat-b', started_at: '2026-07-14T10:01:00Z' }), // still running — no finished_at
      ],
    })
    expect(row.durationMs).toBeNull()
  })
})

describe('filterLaunches', () => {
  const rows: LaunchListRow[] = [
    { id: '2026-07-14_a', shortId: 'a', startedAt: '', families: ['scenario'], models: ['m1'], pass: 8, total: 10, bucket: 'green', durationMs: null },
    { id: '2026-07-10_b', shortId: 'b', startedAt: '', families: ['scenario'], models: ['m2'], pass: 1, total: 20, bucket: 'red', durationMs: null },
    { id: '2026-07-10_c', shortId: 'c', startedAt: '', families: ['scenario'], models: ['m1', 'm2'], pass: 4, total: 20, bucket: 'amber', durationMs: null },
  ]
  it('narrows by launch-id substring', () => {
    expect(filterLaunches(rows, { query: '07-14' }).map((r) => r.id)).toEqual(['2026-07-14_a'])
  })
  it('narrows by model membership', () => {
    expect(filterLaunches(rows, { model: 'm1' }).map((r) => r.id).sort()).toEqual(['2026-07-10_c', '2026-07-14_a'])
  })
  it('narrows by pass-rate bucket', () => {
    expect(filterLaunches(rows, { bucket: 'red' }).map((r) => r.id)).toEqual(['2026-07-10_b'])
  })
  it('combines dimensions with AND', () => {
    expect(filterLaunches(rows, { family: 'scenario', bucket: 'amber' }).map((r) => r.id)).toEqual(['2026-07-10_c'])
  })
  it('an empty filter returns everything unchanged', () => {
    expect(filterLaunches(rows, {})).toHaveLength(3)
  })
})
