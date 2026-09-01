import { describe, expect, it } from 'vitest'
import { lineDiff } from './textDiff'

describe('lineDiff', () => {
  it('both empty returns no lines at all', () => {
    expect(lineDiff('', '')).toEqual([])
  })

  it('identical text is every line marked same', () => {
    expect(lineDiff('a\nb\nc', 'a\nb\nc')).toEqual([
      { type: 'same', text: 'a' },
      { type: 'same', text: 'b' },
      { type: 'same', text: 'c' },
    ])
  })

  it('empty before, non-empty after is every line added, nothing removed', () => {
    const out = lineDiff('', 'a\nb')
    expect(out.filter((l) => l.type === 'removed')).toHaveLength(0)
    expect(out).toEqual([
      { type: 'added', text: 'a' },
      { type: 'added', text: 'b' },
    ])
  })

  it('non-empty before, empty after is every line removed, nothing added', () => {
    const out = lineDiff('a\nb', '')
    expect(out.filter((l) => l.type === 'added')).toHaveLength(0)
    expect(out).toEqual([
      { type: 'removed', text: 'a' },
      { type: 'removed', text: 'b' },
    ])
  })

  it('one line changed in the middle keeps the unchanged lines on both sides', () => {
    expect(lineDiff('Line A\nLine B\nLine C', 'Line A\nLine X\nLine C')).toEqual([
      { type: 'same', text: 'Line A' },
      { type: 'removed', text: 'Line B' },
      { type: 'added', text: 'Line X' },
      { type: 'same', text: 'Line C' },
    ])
  })

  it('a line appended at the end is a trailing added entry only', () => {
    expect(lineDiff('a\nb', 'a\nb\nc')).toEqual([
      { type: 'same', text: 'a' },
      { type: 'same', text: 'b' },
      { type: 'added', text: 'c' },
    ])
  })

  it('a line removed from the middle, with no replacement, is a plain removal', () => {
    expect(lineDiff('a\nb\nc', 'a\nc')).toEqual([
      { type: 'same', text: 'a' },
      { type: 'removed', text: 'b' },
      { type: 'same', text: 'c' },
    ])
  })
})
