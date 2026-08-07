import { describe, expect, it } from 'vitest'
import { changedFields, kindOfMime, materialContentURL, mediaIds, stateForChange } from './shared'

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
