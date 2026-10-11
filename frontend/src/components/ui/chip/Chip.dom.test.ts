import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import Chip from './Chip.vue'

describe('Chip', () => {
  it('exposes its toggle state through aria-pressed', () => {
    const on = mount(Chip, { props: { active: true }, slots: { default: 'Все' } })
    const off = mount(Chip, { slots: { default: 'Мои' } })
    expect(on.attributes('aria-pressed')).toBe('true')
    expect(off.attributes('aria-pressed')).toBe('false')
  })

  it('active chip is the solid ink fill, inactive is an outline', () => {
    const on = mount(Chip, { props: { active: true } })
    const off = mount(Chip)
    expect(on.classes()).toContain('bg-foreground')
    expect(off.classes()).not.toContain('bg-foreground')
  })

  it('forwards click to the parent', async () => {
    const wrapper = mount(Chip, { slots: { default: 'Все' } })
    await wrapper.trigger('click')
    expect(wrapper.emitted('click')).toHaveLength(1)
  })
})
