import { describe, expect, it } from 'vitest'
import { CONFIG_FIELD_ACTIONS, kbActions } from './actions'
import type { ChangeType } from '@/composables/draftChanges'

const KINDS = ['topics', 'tariffs', 'products', 'delivery_zones', 'contacts', 'policies'] as const
const SINGLETONS = new Set(['contacts', 'policies'])

describe('kbActions — live page', () => {
  it.each(KINDS)('%s: offers Изменить + Удалить unless singleton', (kind) => {
    const keys = kbActions({ page: 'live', singleton: SINGLETONS.has(kind) }).map((a) => a.key)
    expect(keys).toContain('edit')
    if (SINGLETONS.has(kind)) {
      expect(keys).not.toContain('delete')
    } else {
      expect(keys).toContain('delete')
    }
  })
})

describe('kbActions — draft page', () => {
  it.each(['added', 'updated'] as ChangeType[])('%s: offers edit + publish + cancel', (changeType) => {
    const keys = kbActions({ page: 'draft', changeType }).map((a) => a.key)
    expect(keys).toEqual(['edit', 'publish', 'cancel'])
  })

  it('removed: offers only publish + cancel — nothing to edit', () => {
    const keys = kbActions({ page: 'draft', changeType: 'removed' }).map((a) => a.key)
    expect(keys).toEqual(['publish', 'cancel'])
  })

  it('cancel is always destructive/ghost, publish is always the primary action', () => {
    const actions = kbActions({ page: 'draft', changeType: 'updated' })
    expect(actions.find((a) => a.key === 'cancel')).toMatchObject({ variant: 'ghost', destructive: true })
    expect(actions.find((a) => a.key === 'publish')).toMatchObject({ variant: 'default' })
  })

  it('singleton has no bearing on the draft action set — draft never offers delete', () => {
    const withSingleton = kbActions({ page: 'draft', changeType: 'updated', singleton: true }).map((a) => a.key)
    const without = kbActions({ page: 'draft', changeType: 'updated', singleton: false }).map((a) => a.key)
    expect(withSingleton).toEqual(without)
    expect(withSingleton).not.toContain('delete')
  })
})

describe('CONFIG_FIELD_ACTIONS', () => {
  it('is exactly edit + cancel — no per-field publish', () => {
    expect(CONFIG_FIELD_ACTIONS.map((a) => a.key)).toEqual(['edit', 'cancel'])
  })
})
