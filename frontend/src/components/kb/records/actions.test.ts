import { describe, expect, it } from 'vitest'
import { kbActions } from './actions'
import { ENTITY_META, KB_ENTITY_ORDER } from '@/components/kb/kbEntities'

describe('kbActions', () => {
  it('draft + added -> edit, publish, cancel', () => {
    expect(kbActions({ page: 'draft', changeType: 'added' }).map((a) => a.key)).toEqual(['edit', 'publish', 'cancel'])
  })

  it('draft + updated -> edit, publish, cancel', () => {
    expect(kbActions({ page: 'draft', changeType: 'updated' }).map((a) => a.key)).toEqual(['edit', 'publish', 'cancel'])
  })

  it('draft + removed -> publish, cancel only — nothing to edit', () => {
    expect(kbActions({ page: 'draft', changeType: 'removed' }).map((a) => a.key)).toEqual(['publish', 'cancel'])
  })

  it('cancel is always the destructive/ghost action on draft', () => {
    for (const changeType of ['added', 'updated', 'removed'] as const) {
      const cancel = kbActions({ page: 'draft', changeType }).find((a) => a.key === 'cancel')!
      expect(cancel.variant).toBe('ghost')
      expect(cancel.destructive).toBe(true)
    }
  })

  it('publish is always the default-variant action on draft', () => {
    for (const changeType of ['added', 'updated', 'removed'] as const) {
      const publish = kbActions({ page: 'draft', changeType }).find((a) => a.key === 'publish')!
      expect(publish.variant).toBe('default')
    }
  })

  it('live edit is always the outline-variant action', () => {
    expect(kbActions({ page: 'live' }).find((a) => a.key === 'edit')!.variant).toBe('outline')
    expect(kbActions({ page: 'live', singleton: true }).find((a) => a.key === 'edit')!.variant).toBe('outline')
  })

  it('live delete is always the destructive/ghost action when present', () => {
    const del = kbActions({ page: 'live' }).find((a) => a.key === 'delete')!
    expect(del.variant).toBe('ghost')
    expect(del.destructive).toBe(true)
  })

  // Table-driven over every one of the seven KB entity kinds, exercising
  // each kind's own ENTITY_META.singleton flag — the real per-kind variable
  // kbActions responds to on the live page (contacts/policies/config are
  // singletons; the other four are not).
  it.each(KB_ENTITY_ORDER)('live: %s gets edit+delete iff it is not a singleton', (kind) => {
    const meta = ENTITY_META[kind]
    const actions = kbActions({ page: 'live', singleton: meta.singleton })
    if (meta.singleton) {
      expect(actions.map((a) => a.key)).toEqual(['edit'])
    } else {
      expect(actions.map((a) => a.key)).toEqual(['edit', 'delete'])
    }
  })
})
