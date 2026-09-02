import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import Combobox from './Combobox.vue'

// TODO.md Settings/AI Providers phase: a searchable model combobox that
// still accepts a fully custom model id — these pin both halves of that
// contract directly against the component, independent of where it's used
// (AiEngineTab.vue, ProviderCredentialCard.vue).
describe('Combobox', () => {
  it('typing a value not in options still updates modelValue (custom model ids stay possible)', async () => {
    const wrapper = mount(Combobox, {
      props: { modelValue: '', options: ['gpt-4o', 'gpt-4o-mini'] },
    })
    await wrapper.find('input').setValue('my-custom-finetune')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['my-custom-finetune'])
  })

  it('filters the dropdown to options matching the typed text', async () => {
    const wrapper = mount(Combobox, {
      props: { modelValue: '', options: ['gpt-4o', 'gpt-4o-mini', 'o1', 'o3-mini'] },
    })
    await wrapper.find('input').trigger('focus')
    expect(wrapper.findAll('button')).toHaveLength(4)

    await wrapper.setProps({ modelValue: 'mini' })
    expect(wrapper.findAll('button').map((b) => b.text())).toEqual(['gpt-4o-mini', 'o3-mini'])
  })

  it('clicking an option selects it and closes the dropdown', async () => {
    const wrapper = mount(Combobox, {
      props: { modelValue: '', options: ['gpt-4o', 'gpt-4o-mini'] },
    })
    await wrapper.find('input').trigger('focus')
    await wrapper.findAll('button')[0].trigger('click')

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['gpt-4o'])
    await wrapper.setProps({ modelValue: 'gpt-4o' })
    expect(wrapper.findAll('button')).toHaveLength(0)
  })

  it('allows keyboard navigation with ArrowDown/ArrowUp and selection with Enter', async () => {
    const wrapper = mount(Combobox, {
      props: { modelValue: '', options: ['gpt-4o', 'gpt-4o-mini', 'o1'] },
    })
    const input = wrapper.find('input')
    await input.trigger('focus')
    await input.trigger('keydown', { key: 'ArrowDown' })
    await input.trigger('keydown', { key: 'ArrowDown' })
    await input.trigger('keydown', { key: 'Enter' })

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['gpt-4o-mini'])
  })

  it('forwards passthrough attributes (e.g. data-testid) to the actual input, not the wrapper', () => {
    const wrapper = mount(Combobox, {
      props: { modelValue: '', options: [] },
      attrs: { 'data-testid': 'model-combobox' },
    })
    expect(wrapper.find('input[data-testid="model-combobox"]').exists()).toBe(true)
  })
})
