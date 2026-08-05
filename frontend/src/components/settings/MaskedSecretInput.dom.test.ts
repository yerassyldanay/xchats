import { describe, expect, it } from 'vitest'
import { mountKb } from '@/test/mount'
import MaskedSecretInput from './MaskedSecretInput.vue'

describe('MaskedSecretInput', () => {
  it('renders a password input by default and reveals it on toggle', async () => {
    const wrapper = mountKb(MaskedSecretInput, { props: { modelValue: '' } })
    const input = wrapper.get('input')
    expect(input.attributes('type')).toBe('password')

    await wrapper.get('button').trigger('click')
    expect(input.attributes('type')).toBe('text')

    await wrapper.get('button').trigger('click')
    expect(input.attributes('type')).toBe('password')
  })

  it('emits update:modelValue as the user types', async () => {
    const wrapper = mountKb(MaskedSecretInput, { props: { modelValue: '' } })
    await wrapper.get('input').setValue('sk-abc123')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['sk-abc123'])
  })

  it('disables both the input and the toggle when disabled', () => {
    const wrapper = mountKb(MaskedSecretInput, { props: { modelValue: '', disabled: true } })
    expect(wrapper.get('input').attributes('disabled')).toBeDefined()
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
  })
})
