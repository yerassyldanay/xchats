import { describe, expect, it } from 'vitest'
import { classifyChanges, totalCounts } from './draftChanges'
import type { DraftChangeSet, DraftView, TopicRow } from '@/types'

function topic(over: Partial<TopicRow> = {}): TopicRow {
  return {
    id: 'pricing', slug: 'pricing', title: 'Тарифы', body_md: 'Текст.',
    featured_image: null, illustration_images: [], explainer_videos: [], reference_documents: [],
    draft: true, updated_at: '2026-08-01T00:00:00Z',
    ...over,
  }
}

function emptyChanges(over: Partial<DraftChangeSet> = {}): DraftChangeSet {
  return {
    base_version: 1, updated_at: '2026-08-01T00:00:00Z', config: null,
    topics: [], tariffs: [], products: [], contacts: [], policies: [], tariff_info: [], zones: [], deletes: [],
    ...over,
  }
}

function emptyLive(over: Partial<DraftView> = {}): DraftView {
  return {
    config: {
      organization_id: 'org-1', persona: '', mission: '', guardrails: '', language_policy: '',
      reply_max_words: 120, draft: false, base_version: 0, updated_at: '',
    },
    topics: [], tariffs: [], products: [], contacts: [], policies: [], tariff_info: [], zones: [], materials: [], requests: [],
    ...over,
  }
}

describe('classifyChanges', () => {
  it('classifies a pending row with no live counterpart as added', () => {
    const changes = emptyChanges({ topics: [topic({ id: 'new-topic', slug: 'new-topic' })] })
    const groups = classifyChanges(changes, emptyLive())
    const g = groups.find((x) => x.kind === 'topics')
    expect(g?.entries).toHaveLength(1)
    expect(g?.entries[0].type).toBe('added')
    expect(g?.entries[0].liveRow).toBeUndefined()
  })

  it('classifies a pending row matching a live natural key as updated', () => {
    const changes = emptyChanges({ topics: [topic({ title: 'Новое название' })] })
    const live = emptyLive({ topics: [topic({ title: 'Старое название' })] })
    const groups = classifyChanges(changes, live)
    const g = groups.find((x) => x.kind === 'topics')
    expect(g?.entries[0].type).toBe('updated')
    expect((g?.entries[0].liveRow as TopicRow | undefined)?.title).toBe('Старое название')
    expect((g?.entries[0].draftRow as TopicRow | undefined)?.title).toBe('Новое название')
  })

  it('classifies a deletion entry as removed, carrying the PUBLISHED row', () => {
    const changes = emptyChanges({ deletes: [{ kind: 'topics', key: 'pricing' }] })
    const live = emptyLive({ topics: [topic()] })
    const groups = classifyChanges(changes, live)
    const g = groups.find((x) => x.kind === 'topics')
    expect(g?.entries).toHaveLength(1)
    expect(g?.entries[0].type).toBe('removed')
    expect(g?.entries[0].liveRow?.id).toBe('pricing')
    expect(g?.entries[0].draftRow).toBeUndefined()
  })

  it('a removed entry with no live counterpart still yields a card', () => {
    const changes = emptyChanges({ deletes: [{ kind: 'topics', key: 'vanished' }] })
    const groups = classifyChanges(changes, emptyLive())
    const g = groups.find((x) => x.kind === 'topics')
    expect(g?.entries).toHaveLength(1)
    expect(g?.entries[0].type).toBe('removed')
    expect(g?.entries[0].liveRow).toBeUndefined()
  })

  it('never turns an unchanged published row into an entry', () => {
    const live = emptyLive({ topics: [topic(), topic({ id: 'other', slug: 'other' })] })
    const groups = classifyChanges(emptyChanges(), live)
    expect(groups).toEqual([])
  })

  it('returns groups only for kinds with entries, in KB_ENTITY_ORDER', () => {
    const changes = emptyChanges({
      zones: [{ id: 'z1', ref: 'z1', name: 'Алматы', zone_level: 'city', parent_ref: '', delivery_available: true, delivery_cost: '', delivery_in_days: '', notes: '', sales_status: 'active', draft: true, updated_at: '' }],
      topics: [topic({ id: 'new', slug: 'new' })],
    })
    const groups = classifyChanges(changes, emptyLive())
    expect(groups.map((g) => g.kind)).toEqual(['topics', 'delivery_zones'])
  })

  it('reads delivery zones from the zones field, not a delivery_zones field', () => {
    const changes = emptyChanges({
      zones: [{ id: 'z1', ref: 'z1', name: 'Алматы', zone_level: 'city', parent_ref: '', delivery_available: true, delivery_cost: '', delivery_in_days: '', notes: '', sales_status: 'active', draft: true, updated_at: '' }],
    })
    const groups = classifyChanges(changes, emptyLive())
    const g = groups.find((x) => x.kind === 'delivery_zones')
    expect(g?.entries).toHaveLength(1)
    expect(g?.entries[0].key).toBe('z1')
  })

  it('yields no overview group when config is null', () => {
    const groups = classifyChanges(emptyChanges({ config: null }), emptyLive())
    expect(groups.find((g) => g.kind === 'config')).toBeUndefined()
  })

  it('two pending config fields yield exactly two entries, only the changed ones', () => {
    const changes = emptyChanges({ config: { persona: 'Дружелюбный', mission: 'Помогать клиентам' } })
    const groups = classifyChanges(changes, emptyLive())
    const g = groups.find((x) => x.kind === 'config')
    expect(g?.entries).toHaveLength(2)
    expect(g?.entries.map((e) => e.field).sort()).toEqual(['mission', 'persona'])
  })
})

describe('totalCounts', () => {
  it('sums added/updated/removed/total across every group', () => {
    const changes = emptyChanges({
      topics: [topic({ id: 'new', slug: 'new' }), topic({ title: 'edited' })],
      deletes: [{ kind: 'products', key: 'gone' }],
    })
    const live = emptyLive({
      topics: [topic()],
      products: [{
        id: 'gone', ref: 'gone', name: 'Товар', price: '', description: '', category: '',
        brand: '', advantages: '', disadvantages: '', best_for: '', not_for: '',
        availability_status: 'in_stock', availability_note: '', installation_terms: '', warranty_terms: '', additional_facts: [],
        sales_status: 'active', featured_image: null, gallery_images: [], demo_videos: [], certificate_documents: [], guarantee_documents: [],
        draft: false, updated_at: '',
      }],
    })
    const counts = totalCounts(classifyChanges(changes, live))
    expect(counts).toEqual({ added: 1, updated: 1, removed: 1, total: 3 })
  })

  it('is all zeros for no groups', () => {
    expect(totalCounts([])).toEqual({ added: 0, updated: 0, removed: 0, total: 0 })
  })
})
