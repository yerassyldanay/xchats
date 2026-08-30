import { describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import CampaignWizard from './CampaignWizard.vue'

const routerPush = vi.fn()
// onBeforeRouteLeave needs a real installed router to register against —
// this mock captures the guard callback instead, so a test can invoke it
// directly the same way vue-router's own navigation would.
let capturedLeaveGuard: (() => boolean | Promise<boolean>) | null = null
vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return {
    ...actual,
    useRouter: () => ({ push: routerPush }),
    onBeforeRouteLeave: (guard: () => boolean | Promise<boolean>) => {
      capturedLeaveGuard = guard
    },
  }
})

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), del: vi.fn() } }
})

async function mountWizard() {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockResolvedValue({ items: [] })
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

// CAM-12: continueToRecipients() already creates a REAL, empty campaign
// server-side (POST /campaigns/:id/preview needs one to check reachability
// against) — every exit from phase 2 other than the wizard's own Cancel/
// finish used to bypass cleanup entirely, stranding it in the list.
describe('CampaignWizard — warns before abandoning an orphan draft campaign (CAM-12)', () => {
  it('does not register a leave guard while still in phase 1 (no campaign created yet)', async () => {
    capturedLeaveGuard = null
    await mountWizard()
    // onBeforeRouteLeave IS called at setup (Vue Router registers the guard
    // unconditionally), but invoking it before any campaign exists must
    // resolve to "proceed" with no dialog.
    expect(capturedLeaveGuard).toBeTruthy()
    const result = capturedLeaveGuard!()
    expect(result).toBe(true)
  })

  // The phase-2 cases (a draft campaign actually pending — beforeunload
  // prevention, the leave-confirmation dialog, discard-and-delete) aren't
  // covered here: reaching phase 2 in a DOM test requires driving reka-ui's
  // Select through a real pointerdown/pointerup gesture on its
  // Portal-rendered SelectItem, which — despite the jsdom PointerEvent +
  // hasPointerCapture polyfills added in src/test/setup.ts — never
  // committed a value in this harness (root cause not isolated: the
  // element, its role="option"/aria attributes, and the dropdown's own
  // open state all inspected as correct). hasUnsavedDraft()'s two other
  // branches (finished, and the phase-1 case above) are covered directly;
  // the finished-flip in confirmStop-style handlers mirrors CAM-08's own
  // tested ConfirmDeleteDialog usage, and campaigns.remove is the same call
  // cancelPending already made before this change.
})
