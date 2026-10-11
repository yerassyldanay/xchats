import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SegmentedToggle from './SegmentedToggle.vue'

const options = [
  { value: 'customer', label: 'Клиент' },
  { value: 'ai', label: 'ИИ-помощник' },
]

describe('SegmentedToggle', () => {
  it('marks only the selected option as pressed', () => {
    const wrapper = mount(SegmentedToggle, { props: { modelValue: 'ai', options } })
    const buttons = wrapper.findAll('button')
    expect(buttons.map((b) => b.attributes('aria-pressed'))).toEqual(['false', 'true'])
  })

  it('emits update:modelValue with the clicked option value', async () => {
    const wrapper = mount(SegmentedToggle, { props: { modelValue: 'ai', options } })
    await wrapper.findAll('button')[0]!.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['customer'])
  })
})
