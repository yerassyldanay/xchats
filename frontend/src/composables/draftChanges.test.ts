import { describe, expect, it } from 'vitest'
import { classifyChanges, totalCounts } from './draftChanges'
import { KB_ENTITY_ORDER } from '@/components/kb/kbEntities'
import { emptyChangeSet, emptyLiveView, tariffRow, topicRow } from '@/test/fixtures'
import type { TariffRow, TopicRow } from '@/types'

describe('classifyChanges', () => {
  it('classifies a pending row with no live counterpart as added', () => {
    const changes = emptyChangeSet({ topics: [topicRow({ id: 'new-topic', slug: 'new-topic' })] })
    const groups = classifyChanges(changes, emptyLiveView())
    const group = groups.find((g) => g.kind === 'topics')!
    expect(group.entries).toHaveLength(1)
    expect(group.entries[0].type).toBe('added')
    expect(group.entries[0].liveRow).toBeUndefined()
  })

  it('classifies a pending row matching a live natural key as updated', () => {
    const changes = emptyChangeSet({ topics: [topicRow({ id: 'x', slug: 'x', title: 'New title' })] })
    const live = emptyLiveView({ topics: [topicRow({ id: 'x', slug: 'x', title: 'Old title' })] })
    const groups = classifyChanges(changes, live)
    const entry = groups.find((g) => g.kind === 'topics')!.entries[0]
    expect(entry.type).toBe('updated')
    expect((entry.liveRow as TopicRow | undefined)?.title).toBe('Old title')
    expect((entry.draftRow as TopicRow | undefined)?.title).toBe('New title')
  })

  it('classifies a deletion entry as removed, carrying the PUBLISHED row', () => {
    const live = emptyLiveView({ tariffs: [tariffRow({ id: 'r1', ref: 'r1', name: 'Published name' })] })
    const changes = emptyChangeSet({ deletes: [{ kind: 'tariffs', key: 'r1' }] })
    const groups = classifyChanges(changes, live)
    const entry = groups.find((g) => g.kind === 'tariffs')!.entries[0]
    expect(entry.type).toBe('removed')
    expect(entry.draftRow).toBeUndefined()
    expect((entry.liveRow as TariffRow | undefined)?.name).toBe('Published name')
  })

  it('never lets an unchanged published row become an entry', () => {
    const live = emptyLiveView({ topics: [topicRow({ id: 'untouched' })] })
    const groups = classifyChanges(emptyChangeSet(), live)
    expect(groups).toHaveLength(0)
  })

  it('returns groups only for kinds with entries, in KB_ENTITY_ORDER', () => {
    const changes = emptyChangeSet({
      tariffs: [tariffRow({ id: 't1' })],
      config: { persona: 'P' },
    })
    const groups = classifyChanges(changes, emptyLiveView())
    expect(groups.map((g) => g.kind)).toEqual(['config', 'tariffs'])
    const orderIndex = (k: string) => KB_ENTITY_ORDER.indexOf(k as (typeof KB_ENTITY_ORDER)[number])
    expect(orderIndex('config')).toBeLessThan(orderIndex('tariffs'))
  })

  it('yields no overview group when config has no pending patch', () => {
    const changes = emptyChangeSet({ topics: [topicRow()], config: null })
    const groups = classifyChanges(changes, emptyLiveView())
    expect(groups.some((g) => g.kind === 'config')).toBe(false)
  })

  it('yields exactly two entries for two pending config fields', () => {
    const changes = emptyChangeSet({ config: { persona: 'P', mission: 'M' } })
    const groups = classifyChanges(changes, emptyLiveView())
    const group = groups.find((g) => g.kind === 'config')!
    expect(group.entries).toHaveLength(2)
    expect(group.entries.map((e) => e.field).sort()).toEqual(['mission', 'persona'])
    expect(group.entries.every((e) => e.type === 'updated')).toBe(true)
  })

  it('a removed entry with no live counterpart still yields a card', () => {
    const changes = emptyChangeSet({ deletes: [{ kind: 'topics', key: 'vanished' }] })
    const groups = classifyChanges(changes, emptyLiveView())
    const entry = groups.find((g) => g.kind === 'topics')!.entries[0]
    expect(entry.type).toBe('removed')
    expect(entry.liveRow).toBeUndefined()
  })

  it('returns no groups for a null change set', () => {
    expect(classifyChanges(null, emptyLiveView())).toEqual([])
  })

  it('returns no groups for a null live baseline (no live counterpart to diff against)', () => {
    const changes = emptyChangeSet({ topics: [topicRow({ id: 'a' })] })
    const groups = classifyChanges(changes, null)
    expect(groups.find((g) => g.kind === 'topics')!.entries[0].type).toBe('added')
  })
})

describe('totalCounts', () => {
  it('sums counts across every group', () => {
    const changes = emptyChangeSet({
      topics: [topicRow({ id: 'a' })],
      tariffs: [tariffRow({ id: 'b' })],
      deletes: [{ kind: 'products', key: 'c' }],
    })
    const groups = classifyChanges(changes, emptyLiveView())
    const totals = totalCounts(groups)
    expect(totals.added).toBe(2)
    expect(totals.removed).toBe(1)
    expect(totals.updated).toBe(0)
    expect(totals.total).toBe(3)
  })

  it('is all-zero for no groups', () => {
    expect(totalCounts([])).toEqual({ added: 0, updated: 0, removed: 0, total: 0 })
  })
})
