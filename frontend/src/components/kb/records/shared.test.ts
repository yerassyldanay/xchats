import { describe, expect, it } from 'vitest'
import { changedFields, KB_MEDIA_FIELDS, kindOfMime, materialContentURL, mediaFieldKind, mediaIds, stateForChange } from './shared'

describe('stateForChange', () => {
  it('maps added -> new', () => {
    expect(stateForChange('added')).toBe('new')
  })
  it('maps updated -> changed', () => {
    expect(stateForChange('updated')).toBe('changed')
  })
  it('maps removed -> to_delete', () => {
    expect(stateForChange('removed')).toBe('to_delete')
  })
})

describe('changedFields', () => {
  it('returns nothing when either side is missing (new record, or no draft edit)', () => {
    expect(changedFields(undefined, { a: '1' }, ['a'])).toEqual([])
    expect(changedFields({ a: '1' }, undefined, ['a'])).toEqual([])
  })
  it('lists only the keys whose value actually differs', () => {
    const draftRow = { name: 'New name', price: '100', category: 'same' }
    const liveRow = { name: 'Old name', price: '100', category: 'same' }
    expect(changedFields(draftRow, liveRow, ['name', 'price', 'category'])).toEqual(['name'])
  })
})

describe('mediaIds', () => {
  it('is [] for null/undefined', () => {
    expect(mediaIds(null)).toEqual([])
    expect(mediaIds(undefined)).toEqual([])
  })
  it('wraps a single attached id', () => {
    expect(mediaIds('abc-123')).toEqual(['abc-123'])
  })
  it('passes an array through as-is', () => {
    expect(mediaIds(['a', 'b', 'c'])).toEqual(['a', 'b', 'c'])
    expect(mediaIds([])).toEqual([])
  })
})

describe('kindOfMime', () => {
  it('classifies the four recognised prefixes', () => {
    expect(kindOfMime('image/png')).toBe('image')
    expect(kindOfMime('video/mp4')).toBe('video')
    expect(kindOfMime('audio/mpeg')).toBe('audio')
    expect(kindOfMime('application/pdf')).toBe('document')
    expect(kindOfMime('text/plain')).toBe('document')
  })
  it('is empty for anything unrecognised', () => {
    expect(kindOfMime('')).toBe('')
    expect(kindOfMime('font/woff2')).toBe('')
  })
})

describe('materialContentURL', () => {
  it('points at the session-authenticated content endpoint', () => {
    expect(materialContentURL('abc-123')).toBe('/xchats/api/v1/kb/materials/abc-123/content')
  })
})

describe('mediaFieldKind', () => {
  it('resolves every field in the KB_MEDIA_FIELDS registry to its declared kind', () => {
    for (const specs of Object.values(KB_MEDIA_FIELDS)) {
      for (const spec of specs) {
        expect(mediaFieldKind(spec.field)).toBe(spec.kind)
      }
    }
  })
  it('agrees on featured_image across every type that carries it', () => {
    expect(mediaFieldKind('featured_image')).toBe('image')
  })
  it('agrees on explainer_videos, shared by topics and tariffs', () => {
    expect(mediaFieldKind('explainer_videos')).toBe('video')
  })
  it('is empty for an unrecognized field', () => {
    expect(mediaFieldKind('')).toBe('')
    expect(mediaFieldKind('not_a_real_field')).toBe('')
  })
})
