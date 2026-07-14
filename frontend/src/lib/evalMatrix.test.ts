import { describe, expect, it } from 'vitest'
import type { RunSummary, VExecution } from '../types'
import { buildComparisonMatrices, cellFor, costLabel, deriveLaunchStatus, groupRunsByLaunch, pct } from './evalMatrix'

// scenarioExec is the fixture builder — every field a real VExecution carries,
// defaulted to the "everything passed" shape so each test only overrides what it's
// actually exercising (mirrors evals/harness's own Go test fixture style).
function scenarioExec(over: {
  testID: string
  setup?: string
  experiment?: string
  model: string
  behaviorPass?: boolean
  contractPass?: boolean
  costBasis?: string
  estimateUSD?: number
  latencyMs?: number
}): VExecution {
  const behaviorPass = over.behaviorPass ?? true
  const contractPass = over.contractPass ?? true
  return {
    family: 'scenario',
    subject: { test_id: over.testID, scenario: over.setup },
    variant: { model: over.model, setup: over.setup, experiment: over.experiment },
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

function extractExec(over: { caseID: string; prompt: string; model: string; allChecksPass?: boolean; parseOK?: boolean }): VExecution {
  return {
    family: 'extract',
    subject: { case_id: over.caseID },
    variant: { model: over.model, setup: over.prompt, prompt: { name: over.prompt.split('@')[0], version: 1 } },
    output: { parse_ok: over.parseOK ?? true },
    scores: [],
    rollups: [{ key: 'all_checks_pass', label: 'All checks pass', pass: over.allChecksPass ?? true }],
    cost: { tokens_in: 500, tokens_out: 100, estimate_usd: 0.0005, basis: 'measured_split' },
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
    const groups = buildComparisonMatrices(execs, 'scenario')
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

    const groups = buildComparisonMatrices(execs, 'scenario')
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
    const groups = buildComparisonMatrices(execs, 'scenario')
    expect(groups).toHaveLength(2)

    const comparableGroup = groups.find((g) => !g.warning)!
    const warnedGroup = groups.find((g) => g.warning)!
    expect(comparableGroup.setups).toEqual(['lang-v2'])
    expect(warnedGroup.setups).toEqual(['lang-v3'])
    expect(warnedGroup.warning).toContain('lang-v3')
    expect(warnedGroup.warning).toContain('lang-bakeoff')
  })

  it('never pools across different experiments', () => {
    const execs: VExecution[] = [
      scenarioExec({ testID: 't1', setup: 'lang-v1', experiment: 'lang-bakeoff', model: 'm1' }),
      scenarioExec({ testID: 't1', setup: 'escalation-v1', experiment: 'escalation-bakeoff', model: 'm1' }),
    ]
    const groups = buildComparisonMatrices(execs, 'scenario')
    expect(groups.map((g) => g.experiment).sort()).toEqual(['escalation-bakeoff', 'lang-bakeoff'])
  })

  it('an unannotated scenario (no setup/experiment) falls back to the model-visible scenario name and is never dropped', () => {
    const execs: VExecution[] = [scenarioExec({ testID: 't1', model: 'm1' })] // no setup, no experiment
    execs[0].variant.setup = undefined
    execs[0].variant.experiment = undefined
    const groups = buildComparisonMatrices(execs, 'scenario')
    expect(groups).toHaveLength(1)
    expect(groups[0].experiment).toBe('')
    expect(groups[0].cells).toHaveLength(1)
  })
})

describe('buildComparisonMatrices — extract family', () => {
  it('uses all_checks_pass/parse_ok, never contract/behavior metrics', () => {
    const execs: VExecution[] = [
      extractExec({ caseID: 'c1', prompt: 'extract@v1', model: 'm1', allChecksPass: true }),
      extractExec({ caseID: 'c1', prompt: 'extract@v2', model: 'm1', allChecksPass: false, parseOK: true }),
    ]
    const groups = buildComparisonMatrices(execs, 'extract')
    // Different prompt refs => different item sets aren't the issue here (both cover
    // {c1}), so both v1 and v2 land in ONE comparable matrix, as two setup columns.
    expect(groups).toHaveLength(1)
    expect(groups[0].setups.sort()).toEqual(['extract@v1', 'extract@v2'])
    const v1 = cellFor(groups[0], 'extract@v1', 'm1')!
    expect(v1.allChecksPass).toBe(1)
    expect(v1.behaviorPass).toBeNull()
    expect(v1.contractPass).toBeNull()
    const v2 = cellFor(groups[0], 'extract@v2', 'm1')!
    expect(v2.allChecksPass).toBe(0)
    expect(v2.parsePass).toBe(1)
  })

  it('returns an empty list for a run with no executions of that family', () => {
    expect(buildComparisonMatrices([extractExec({ caseID: 'c1', prompt: 'extract@v1', model: 'm1' })], 'scenario')).toEqual([])
  })
})

describe('costLabel', () => {
  it('shows a dollar figure only for measured/borrowed bases, an explicit label otherwise', () => {
    expect(costLabel('measured_split', 0.00021)).toBe('$0.00021')
    expect(costLabel('cached_replay_borrowed', 0.00021)).toBe('$0.00021')
    expect(costLabel('cached_replay_unpriceable', 0)).not.toMatch(/^\$/)
    expect(costLabel('unknown_pricing', 0)).not.toMatch(/^\$/)
    expect(costLabel('', 0)).not.toMatch(/^\$/)
  })
})

describe('pct', () => {
  it('formats a pass/total pair, and reports н/д for null or zero total', () => {
    expect(pct(8, 10)).toBe('80% (8/10)')
    expect(pct(null, 10)).toBe('н/д')
    expect(pct(0, 0)).toBe('н/д')
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
