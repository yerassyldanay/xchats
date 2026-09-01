import { describe, expect, it } from 'vitest'
import { mountKb } from '@/test/mount'
import FieldDiffNote from './FieldDiffNote.vue'

describe('FieldDiffNote — show gate', () => {
  it('renders nothing when show is false, however long the value', () => {
    const wrapper = mountKb(FieldDiffNote, { props: { show: false, was: 'x'.repeat(500), now: 'y'.repeat(500) } })
    expect(wrapper.text()).toBe('')
  })
})

describe('FieldDiffNote — short values (KB-07 baseline, unchanged)', () => {
  it('shows the plain "Was: [struck through]" caption for a short field', () => {
    const wrapper = mountKb(FieldDiffNote, { props: { show: true, was: 'Старая цена', now: 'Новая цена' } })
    expect(wrapper.text()).toContain('Было:')
    expect(wrapper.find('.line-through').text()).toBe('Старая цена')
    expect(wrapper.find('[data-testid="field-diff-details"]').exists()).toBe(false)
  })

  it('shows an em dash for an empty short "was"', () => {
    const wrapper = mountKb(FieldDiffNote, { props: { show: true, was: '', now: 'Новое значение' } })
    expect(wrapper.find('.line-through').text()).toBe('—')
  })
})

describe('FieldDiffNote — long values switch to a line-level diff (KB-07)', () => {
  it('swaps to a collapsed diff disclosure once the value exceeds the length threshold', () => {
    const wrapper = mountKb(FieldDiffNote, {
      props: { show: true, was: 'x'.repeat(150), now: 'y'.repeat(150) },
    })
    expect(wrapper.text()).not.toContain('Было:')
    expect(wrapper.find('[data-testid="field-diff-details"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Посмотреть изменения')
  })

  it('also swaps to the diff view for a short-per-line but multi-paragraph field', () => {
    const wrapper = mountKb(FieldDiffNote, {
      props: { show: true, was: 'Line one.\nLine two.\nLine three.', now: 'Line one.\nLine two, edited.\nLine three.' },
    })
    expect(wrapper.find('[data-testid="field-diff-details"]').exists()).toBe(true)
  })

  it('marks unchanged, removed, and added lines correctly, in order', () => {
    const wrapper = mountKb(FieldDiffNote, {
      props: { show: true, was: 'Line A\nLine B\nLine C', now: 'Line A\nLine X\nLine C' },
    })
    const removed = wrapper.findAll('[data-testid="diff-line-removed"]')
    const added = wrapper.findAll('[data-testid="diff-line-added"]')
    const same = wrapper.findAll('[data-testid="diff-line-same"]')

    expect(same).toHaveLength(2) // "Line A" and "Line C" are unchanged
    expect(removed).toHaveLength(1)
    expect(removed[0].text()).toContain('Line B')
    expect(added).toHaveLength(1)
    expect(added[0].text()).toContain('Line X')

    // Order matters for readability: unchanged, then the removed/added pair,
    // then unchanged again — not all removals bunched before all additions.
    const allLines = wrapper.findAll('[data-testid^="diff-line-"]')
    expect(allLines.map((l) => l.attributes('data-testid'))).toEqual([
      'diff-line-same',
      'diff-line-removed',
      'diff-line-added',
      'diff-line-same',
    ])
  })

  it('a wholly new (added) long field has no removed lines, just an empty "was"', () => {
    const wrapper = mountKb(FieldDiffNote, {
      props: { show: true, was: '', now: 'A brand new multi-line value.\nSecond line.\nThird line that pushes this well past the short-caption threshold.' },
    })
    expect(wrapper.find('[data-testid="field-diff-details"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-testid="diff-line-removed"]')).toHaveLength(0)
    expect(wrapper.findAll('[data-testid="diff-line-added"]').length).toBeGreaterThan(0)
  })
})
