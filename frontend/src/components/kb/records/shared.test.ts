import { describe, expect, it } from 'vitest'
import { changedFields, mediaCount, stateForChange } from './shared'

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
