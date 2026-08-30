import { describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import CampaignWizard from './CampaignWizard.vue'

vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return { ...actual, useRouter: () => ({ push: vi.fn() }) }
})

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn().mockResolvedValue({ items: [] }) } }
})

async function mountWizard() {
  const pinia = testPinia()
  const wrapper = mountKb(CampaignWizard, { pinia })
  await flushPromises()
  return wrapper
}

// CAM-02: the parser only ever recognizes double-brace {{var}} — the
// placeholder used to show single braces, teaching the wrong syntax. These
// chips are the actual fix operators get: correct syntax inserted for them,
// never hand-typed.
describe('CampaignWizard — message variable chips (CAM-02)', () => {
  it('the placeholder itself uses double braces, matching the parser', async () => {
    const wrapper = await mountWizard()
    const textarea = wrapper.find('textarea[placeholder]')
    expect(textarea.attributes('placeholder')).toContain('{{name}}')
    expect(textarea.attributes('placeholder')).not.toMatch(/[^{]\{name\}[^}]/)
  })

  it('clicking a quick-insert chip appends the correctly-bracketed token and it is detected', async () => {
    const wrapper = await mountWizard()
    await wrapper.find('[data-testid="insert-var-name"]').trigger('click')
    await flushPromises()

    const textarea = wrapper.find('textarea[placeholder]').element as HTMLTextAreaElement
    expect(textarea.value).toBe('{{name}}')
    expect(wrapper.text()).toContain('name')

    await wrapper.find('[data-testid="insert-var-phone"]').trigger('click')
    expect(textarea.value).toBe('{{name}}{{phone}}')
  })

  it('a custom variable name is inserted with the correct double-brace syntax', async () => {
    const wrapper = await mountWizard()
    await wrapper.find('[data-testid="add-custom-var"]').trigger('click')
    await wrapper.find('[data-testid="custom-var-input"]').setValue('promo code')
    await wrapper.find('[data-testid="custom-var-input"]').trigger('keydown.enter')
    await flushPromises()

    const textarea = wrapper.find('textarea[placeholder]').element as HTMLTextAreaElement
    expect(textarea.value).toBe('{{promo_code}}')
  })

  it('inserts at the cursor position, not always at the end', async () => {
    const wrapper = await mountWizard()
    const textareaWrapper = wrapper.find('textarea[placeholder]')
    const textarea = textareaWrapper.element as HTMLTextAreaElement

    await textareaWrapper.setValue('Hi , welcome!')
    textarea.setSelectionRange(3, 3) // right after "Hi "
    await wrapper.find('[data-testid="insert-var-name"]').trigger('click')
    await flushPromises()

    expect(textarea.value).toBe('Hi {{name}}, welcome!')
  })
})
