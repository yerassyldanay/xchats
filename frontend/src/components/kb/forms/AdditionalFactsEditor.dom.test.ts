import { describe, expect, it } from 'vitest'
import { mountKb } from '@/test/mount'
import type { AdditionalFact } from '@/types'
import AdditionalFactsEditor from './AdditionalFactsEditor.vue'

function mountEditor(modelValue: AdditionalFact[]) {
  return mountKb(AdditionalFactsEditor, { props: { modelValue } })
}

function lastEmitted(wrapper: ReturnType<typeof mountEditor>): AdditionalFact[] {
  const events = wrapper.emitted('update:modelValue')
  if (!events) throw new Error('update:modelValue was never emitted')
  return events[events.length - 1][0] as AdditionalFact[]
}

describe('AdditionalFactsEditor — add/remove', () => {
  it('starts empty with no rows', () => {
    const wrapper = mountEditor([])
    expect(wrapper.findAll('[data-testid="additional-fact-row"]')).toHaveLength(0)
  })

  it('Add fact appends one empty string-typed row to the emitted array', async () => {
    const wrapper = mountEditor([{ ref: 'existing', value: 'x', instruction: 'i' }])
    await wrapper.find('[data-testid="additional-fact-add"]').trigger('click')
    expect(lastEmitted(wrapper)).toEqual([
      { ref: 'existing', value: 'x', instruction: 'i' },
      { ref: '', value: '', instruction: '' },
    ])
  })

  it('removing a row emits the array with just that row dropped, others untouched', async () => {
    const wrapper = mountEditor([
      { ref: 'a', value: 1, instruction: 'first' },
      { ref: 'b', value: 2, instruction: 'second' },
      { ref: 'c', value: 3, instruction: 'third' },
    ])
    await wrapper.findAll('[data-testid="additional-fact-remove"]')[1].trigger('click')
    expect(lastEmitted(wrapper)).toEqual([
      { ref: 'a', value: 1, instruction: 'first' },
      { ref: 'c', value: 3, instruction: 'third' },
    ])
  })

  it('removing the only row emits an empty array (explicit clear, not omission)', async () => {
    const wrapper = mountEditor([{ ref: 'a', value: true, instruction: 'i' }])
    await wrapper.find('[data-testid="additional-fact-remove"]').trigger('click')
    expect(lastEmitted(wrapper)).toEqual([])
  })
})

describe('AdditionalFactsEditor — editing ref/value/instruction', () => {
  it('typing in the ref field emits the row with just ref changed', async () => {
    const wrapper = mountEditor([{ ref: '', value: '', instruction: '' }])
    await wrapper.find('[data-testid="additional-fact-ref"]').setValue('limit_on_devices')
    expect(lastEmitted(wrapper)).toEqual([{ ref: 'limit_on_devices', value: '', instruction: '' }])
  })

  it('typing in the instruction field emits the row with just instruction changed', async () => {
    const wrapper = mountEditor([{ ref: 'has_wifi', value: true, instruction: '' }])
    await wrapper.find('[data-testid="additional-fact-instruction"]').setValue('Поддерживает ли товар Wi-Fi.')
    expect(lastEmitted(wrapper)).toEqual([{ ref: 'has_wifi', value: true, instruction: 'Поддерживает ли товар Wi-Fi.' }])
  })

  it('a string-typed row edits value as a JS string', async () => {
    const wrapper = mountEditor([{ ref: 'model_code', value: '', instruction: '' }])
    await wrapper.find('[data-testid="additional-fact-value"]').setValue('DLM-500X')
    const emitted = lastEmitted(wrapper)
    expect(emitted[0].value).toBe('DLM-500X')
    expect(typeof emitted[0].value).toBe('string')
  })
})

describe('AdditionalFactsEditor — value type preserved and switchable', () => {
  it('renders a checkbox for a boolean-valued fact and toggling emits a JS boolean', async () => {
    const wrapper = mountEditor([{ ref: 'has_wifi', value: false, instruction: 'i' }])
    const checkbox = wrapper.find('[data-testid="additional-fact-value"]')
    expect(checkbox.attributes('type')).toBe('checkbox')
    await checkbox.setValue(true)
    const emitted = lastEmitted(wrapper)
    expect(emitted[0].value).toBe(true)
    expect(typeof emitted[0].value).toBe('boolean')
  })

  it('renders a number input for a number-valued fact and typing emits a JS number', async () => {
    const wrapper = mountEditor([{ ref: 'limit_on_devices', value: 5, instruction: 'i' }])
    const input = wrapper.find('[data-testid="additional-fact-value"]')
    expect(input.attributes('type')).toBe('number')
    await input.setValue('12')
    const emitted = lastEmitted(wrapper)
    expect(emitted[0].value).toBe(12)
    expect(typeof emitted[0].value).toBe('number')
  })

  it('switching type to number re-seeds value to 0, not a coerced string', async () => {
    const wrapper = mountEditor([{ ref: 'model_code', value: 'DLM-500X', instruction: 'i' }])
    await wrapper.find('[data-testid="additional-fact-type"]').setValue('number')
    expect(lastEmitted(wrapper)).toEqual([{ ref: 'model_code', value: 0, instruction: 'i' }])
  })

  it('switching type to boolean re-seeds value to false', async () => {
    const wrapper = mountEditor([{ ref: 'trial_in_days', value: 3, instruction: 'i' }])
    await wrapper.find('[data-testid="additional-fact-type"]').setValue('boolean')
    expect(lastEmitted(wrapper)).toEqual([{ ref: 'trial_in_days', value: false, instruction: 'i' }])
  })

  it('switching type to string re-seeds value to an empty string', async () => {
    const wrapper = mountEditor([{ ref: 'has_wifi', value: true, instruction: 'i' }])
    await wrapper.find('[data-testid="additional-fact-type"]').setValue('string')
    expect(lastEmitted(wrapper)).toEqual([{ ref: 'has_wifi', value: '', instruction: 'i' }])
  })
})

describe('AdditionalFactsEditor — client-side ref hints', () => {
  // AdditionalFactsEditor is a pure controlled component (see its own doc
  // comment): the red-border class reads props.modelValue, not the <input>
  // element's live DOM value, so exercising it means mounting with the
  // shape a real parent would re-render with AFTER receiving an emit —
  // typing alone never feeds back into props in an isolated unit test.
  it('flags a ref that does not match the lowercase-snake-case pattern', () => {
    const wrapper = mountEditor([{ ref: 'Not Valid!', value: '', instruction: '' }])
    expect(wrapper.find('[data-testid="additional-fact-ref"]').classes()).toContain('border-destructive')
  })

  it('does not flag a well-formed ref', () => {
    const wrapper = mountEditor([{ ref: 'limit_on_devices', value: '', instruction: '' }])
    expect(wrapper.find('[data-testid="additional-fact-ref"]').classes()).not.toContain('border-destructive')
  })

  it('does not flag an empty (not-yet-typed) ref', () => {
    const wrapper = mountEditor([{ ref: '', value: '', instruction: '' }])
    expect(wrapper.find('[data-testid="additional-fact-ref"]').classes()).not.toContain('border-destructive')
  })

  it('flags both rows sharing the same ref as duplicates', () => {
    const wrapper = mountEditor([
      { ref: 'limit_on_devices', value: 1, instruction: 'a' },
      { ref: 'limit_on_devices', value: 2, instruction: 'b' },
    ])
    const refInputs = wrapper.findAll('[data-testid="additional-fact-ref"]')
    expect(refInputs[0].classes()).toContain('border-destructive')
    expect(refInputs[1].classes()).toContain('border-destructive')
  })

  it('does not flag two different refs', () => {
    const wrapper = mountEditor([
      { ref: 'a_fact', value: 1, instruction: 'a' },
      { ref: 'b_fact', value: 2, instruction: 'b' },
    ])
    for (const input of wrapper.findAll('[data-testid="additional-fact-ref"]')) {
      expect(input.classes()).not.toContain('border-destructive')
    }
  })
})

describe('AdditionalFactsEditor — numeric precision', () => {
  it('an incomplete edit ("-") does not emit a coerced 0', async () => {
    const wrapper = mountEditor([{ ref: 'limit_on_devices', value: 5, instruction: 'i' }])
    await wrapper.find('[data-testid="additional-fact-value"]').setValue('-')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('clearing the number input does not emit a coerced 0', async () => {
    const wrapper = mountEditor([{ ref: 'limit_on_devices', value: 5, instruction: 'i' }])
    await wrapper.find('[data-testid="additional-fact-value"]').setValue('')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('finishing a negative number after the incomplete "-" keystroke emits the real value', async () => {
    const wrapper = mountEditor([{ ref: 'limit_on_devices', value: 5, instruction: 'i' }])
    const input = wrapper.find('[data-testid="additional-fact-value"]')
    await input.setValue('-')
    await input.setValue('-3')
    expect(lastEmitted(wrapper)).toEqual([{ ref: 'limit_on_devices', value: -3, instruction: 'i' }])
  })

  // AdditionalFactsEditor is a pure controlled component (see the ref-hints
  // describe block's own note above): imprecision is read off
  // props.modelValue, so these mount with the shape a real parent would
  // re-render with AFTER an emit, not by simulating keystrokes.
  it('flags an integer one past Number.MAX_SAFE_INTEGER as imprecise', () => {
    const wrapper = mountEditor([{ ref: 'serial', value: 9007199254740992, instruction: 'i' }])
    expect(wrapper.find('[data-testid="additional-fact-value"]').classes()).toContain('border-destructive')
  })

  it('does not flag Number.MAX_SAFE_INTEGER itself', () => {
    const wrapper = mountEditor([{ ref: 'serial', value: 9007199254740991, instruction: 'i' }])
    expect(wrapper.find('[data-testid="additional-fact-value"]').classes()).not.toContain('border-destructive')
  })

  it('flags a decimal with more significant digits than float64 can carry', () => {
    // Built via Number() at runtime, not a source literal: a 19-digit
    // decimal literal would itself already lose precision the moment this
    // file is parsed, before the test ever runs (the same class of bug
    // this component's own fix addresses) — ESLint's no-loss-of-precision
    // rule catches exactly that if written as a literal.
    const value = Number('1.234567890123456789')
    const wrapper = mountEditor([{ ref: 'ratio', value, instruction: 'i' }])
    expect(wrapper.find('[data-testid="additional-fact-value"]').classes()).toContain('border-destructive')
  })

  it('does not flag an ordinary decimal like 0.3', () => {
    const wrapper = mountEditor([{ ref: 'ratio', value: 0.3, instruction: 'i' }])
    expect(wrapper.find('[data-testid="additional-fact-value"]').classes()).not.toContain('border-destructive')
  })

  it('does not flag an ordinary whole number', () => {
    const wrapper = mountEditor([{ ref: 'limit_on_devices', value: 185000, instruction: 'i' }])
    expect(wrapper.find('[data-testid="additional-fact-value"]').classes()).not.toContain('border-destructive')
  })
})
