import { describe, expect, it } from 'vitest'
import { changedFields, mediaCount, recordState } from './shared'

describe('recordState', () => {
  it('is always published in live mode, regardless of the counterpart flag', () => {
    expect(recordState('live', true)).toBe('published')
    expect(recordState('live', false)).toBe('published')
  })
  it('is new in draft mode with no live counterpart', () => {
    expect(recordState('draft', false)).toBe('new')
  })
  it('is changed in draft mode with a live counterpart', () => {
    expect(recordState('draft', true)).toBe('changed')
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

describe('mediaCount', () => {
  it('is 0 for null/undefined', () => {
    expect(mediaCount(null)).toBe(0)
    expect(mediaCount(undefined)).toBe(0)
  })
  it('is 1 for a single attached id', () => {
    expect(mediaCount('abc-123')).toBe(1)
  })
  it('is the array length for a multi-value media field', () => {
    expect(mediaCount(['a', 'b', 'c'])).toBe(3)
    expect(mediaCount([])).toBe(0)
  })
})
