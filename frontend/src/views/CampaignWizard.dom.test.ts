import { describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import CampaignWizard from './CampaignWizard.vue'

// reka-ui's Dialog renders through a Teleport into document.body, and (as
// elsewhere in this codebase — see DraftKnowledgeBase.dom.test.ts's own
// note) an earlier test's closed dialog can leave stale nodes behind since
// nothing here unmounts between tests. Always take the LAST match, which
// belongs to whichever dialog THIS test just opened.
function body() {
  return new DOMWrapper(document.body)
}
function lastButtonMatching(predicate: (b: DOMWrapper<Element>) => boolean) {
  const all = body().findAll('button').filter(predicate)
  return all[all.length - 1]
}

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
  return {
    ...actual,
    api: {
      ...actual.api,
      get: vi.fn(),
      post: vi.fn(),
      patch: vi.fn(),
      del: vi.fn(),
      previewCampaignRecipients: vi.fn(),
      replaceCampaignRecipients: vi.fn(),
    },
  }
})

async function mountWizard() {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockResolvedValue({ items: [] })
  const pinia = testPinia()
  const wrapper = mountKb(CampaignWizard, { pinia })
  await flushPromises()
  return wrapper
}

const BUDGET_FIXTURE = { account_id: 'acct-1', min_interval_seconds: 60, jitter_seconds: 10, paused: false, allowed: true, throttled_by: 0, next_send_at: '', tiers: [] }

// Reaching phase 2 through the real UI means driving reka-ui's Select
// through a genuine pointerdown/pointerup gesture on its Portal-rendered
// SelectItem — attempted at length elsewhere in this file's history and
// never got jsdom to commit a value (see the CAM-12 section below). This
// sets accountId directly on the mounted instance instead: script-setup's
// dev-mode instance proxy supports reading AND writing top-level bindings,
// so continueToRecipients() still runs for real off of it — the only thing
// skipped is the Select's own open/click/select sequence, which is not
// what any test past this point is about.
async function mountWizardInPhase2() {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/accounts') return { items: [{ id: 'acct-1', display_name: 'Acct', external_handle: '' }] } as any
    if (path === '/accounts/acct-1/sending-budget') return BUDGET_FIXTURE as any
    throw new Error(`unexpected GET ${path}`)
  })
  vi.mocked(api.post).mockResolvedValueOnce({ id: 'camp-1', name: 'Test campaign' } as any)
  const pinia = testPinia()
  const wrapper = mountKb(CampaignWizard, { pinia })
  await flushPromises()

  ;(wrapper.vm as any).name = 'Test campaign'
  ;(wrapper.vm as any).messageBody = 'Hi {{name}}'
  ;(wrapper.vm as any).accountId = 'acct-1'
  await flushPromises()
  const continueBtn = wrapper.findAll('button').find((b) => b.text() === 'Продолжить к получателям →')
  await continueBtn!.trigger('click')
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

  it('beforeunload is prevented once a draft campaign exists, and stops being prevented after Cancel', async () => {
    const wrapper = await mountWizardInPhase2()
    const { api } = await import('@/api/client')
    vi.mocked(api.del).mockResolvedValueOnce(undefined as any)

    const evt = new Event('beforeunload', { cancelable: true }) as BeforeUnloadEvent
    window.dispatchEvent(evt)
    expect(evt.defaultPrevented).toBe(true)

    await wrapper.findAll('button').find((b) => b.text() === 'Отмена')!.trigger('click')
    await flushPromises()

    const evt2 = new Event('beforeunload', { cancelable: true }) as BeforeUnloadEvent
    window.dispatchEvent(evt2)
    expect(evt2.defaultPrevented).toBe(false)
  })

  it('the route-leave guard blocks navigation and shows a styled confirmation, not a native confirm', async () => {
    await mountWizardInPhase2()
    expect(capturedLeaveGuard).toBeTruthy()

    const pending = capturedLeaveGuard!()
    expect(pending).toBeInstanceOf(Promise)
    await flushPromises()
    expect(body().text()).toContain('Уйти, не завершив создание рассылки?')
  })

  it('choosing Stay resolves the guard to false and never deletes the campaign', async () => {
    await mountWizardInPhase2()
    const { api } = await import('@/api/client')
    vi.mocked(api.del).mockClear()

    const pending = capturedLeaveGuard!() as Promise<boolean>
    await flushPromises()
    const stayBtn = lastButtonMatching((b) => b.text() === 'Отмена')
    await stayBtn!.trigger('click')

    expect(await pending).toBe(false)
    expect(api.del).not.toHaveBeenCalled()
  })

  it('choosing Discard deletes the pending campaign and resolves the guard to true', async () => {
    await mountWizardInPhase2()
    const { api } = await import('@/api/client')
    vi.mocked(api.del).mockClear()
    vi.mocked(api.del).mockResolvedValueOnce(undefined as any)

    const pending = capturedLeaveGuard!() as Promise<boolean>
    await flushPromises()
    const accepts = body().findAll('[data-testid="confirm-accept"]')
    await accepts[accepts.length - 1].trigger('click')
    await flushPromises()

    expect(await pending).toBe(true)
    expect(api.del).toHaveBeenCalledWith('/campaigns/camp-1')
  })
})

// CAM-01: "Save" implied the campaign was already complete — operators
// hesitated on a button that, in fact, only starts phase 2.
describe('CampaignWizard — step indicator and continue label (CAM-01)', () => {
  it('the phase-1 action button reads Continue, not Save', async () => {
    const wrapper = await mountWizard()
    expect(wrapper.findAll('button').some((b) => b.text() === 'Продолжить к получателям →')).toBe(true)
    expect(wrapper.findAll('button').some((b) => b.text() === 'Сохранить')).toBe(false)
  })

  it('highlights step 1 in phase 1 and step 2 once phase 2 is reached', async () => {
    const wrapper = await mountWizard()
    expect(wrapper.find('[data-testid="wizard-step-details"]').classes()).toContain('bg-primary')
    expect(wrapper.find('[data-testid="wizard-step-recipients"]').classes()).not.toContain('bg-primary')

    const wrapper2 = await mountWizardInPhase2()
    expect(wrapper2.find('[data-testid="wizard-step-details"]').classes()).not.toContain('bg-primary')
    expect(wrapper2.find('[data-testid="wizard-step-recipients"]').classes()).toContain('bg-primary')
  })
})

// CAM-03: "Variables used: name, promo_code" named the template's own
// shape, never what the message actually looks like once rendered.
describe('CampaignWizard — message preview with sample data (CAM-03)', () => {
  it('the preview toggle is hidden until the message has content', async () => {
    const wrapper = await mountWizard()
    expect(wrapper.find('[data-testid="toggle-message-preview"]').exists()).toBe(false)

    await wrapper.find('textarea[placeholder]').setValue('Hi {{name}}')
    expect(wrapper.find('[data-testid="toggle-message-preview"]').exists()).toBe(true)
  })

  it('renders the message with sample values substituted, not the raw template', async () => {
    const wrapper = await mountWizard()
    await wrapper.find('textarea[placeholder]').setValue('Hi {{name}}, code: {{promo_code}}, ask {{agent_name}}')
    await wrapper.find('[data-testid="toggle-message-preview"]').trigger('click')

    const bubble = wrapper.find('[data-testid="message-preview-bubble"]')
    expect(bubble.text()).toBe('Hi Aigul, code: SUMMER2026, ask [agent_name]')
  })
})

// CAM-05: the account's own live budget was only ever visible AFTER
// creation, on the detail page — too late to pick a different account.
describe('CampaignWizard — account budget visibility (CAM-05)', () => {
  it('shows the live sending budget once an account is chosen', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path === '/accounts') return { items: [{ id: 'acct-1', display_name: 'Acct', external_handle: '' }] } as any
      if (path === '/accounts/acct-1/sending-budget') return BUDGET_FIXTURE as any
      throw new Error(`unexpected GET ${path}`)
    })
    const pinia = testPinia()
    const wrapper = mountKb(CampaignWizard, { pinia })
    await flushPromises()
    expect(wrapper.text()).not.toContain('Бюджет отправки')

    ;(wrapper.vm as any).accountId = 'acct-1'
    await flushPromises()
    expect(wrapper.text()).toContain('Бюджет отправки')
  })
})

// CAM-04: reachability used to be a manual prerequisite — paste or upload,
// then separately notice and click Check, or Create would just reject.
describe('CampaignWizard — automatic reachability checking (CAM-04)', () => {
  it('typing pasted text auto-checks 400ms after the operator stops typing', async () => {
    const wrapper = await mountWizardInPhase2()
    const { api } = await import('@/api/client')
    vi.mocked(api.previewCampaignRecipients).mockResolvedValueOnce({ rows: [], total: 1, valid: 1, invalid: 0, duplicate: 0 })

    vi.useFakeTimers()
    try {
      await wrapper.find('[data-testid="paste-recipients"]').setValue('77011234567,Aigul')
      expect(api.previewCampaignRecipients).not.toHaveBeenCalled()
      await vi.advanceTimersByTimeAsync(400)
      expect(api.previewCampaignRecipients).toHaveBeenCalledTimes(1)
    } finally {
      vi.useRealTimers()
    }
  })

  it('selecting a file checks immediately, not on the paste-text debounce', async () => {
    const wrapper = await mountWizardInPhase2()
    const { api } = await import('@/api/client')
    // Module-level vi.fn()s have no automatic call-history reset between
    // tests in this file (see KnowledgeBase.dom.test.ts's identical note).
    vi.mocked(api.previewCampaignRecipients).mockClear()
    vi.mocked(api.previewCampaignRecipients).mockResolvedValueOnce({ rows: [], total: 1, valid: 1, invalid: 0, duplicate: 0 })

    const file = new File(['phone,name\n77011234567,Aigul'], 'recipients.csv', { type: 'text/csv' })
    const input = wrapper.find('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
    await input.trigger('change')
    await flushPromises()

    expect(api.previewCampaignRecipients).toHaveBeenCalledTimes(1)
  })

  it('clicking Create with an unchecked list runs the check automatically and proceeds once it is valid', async () => {
    const wrapper = await mountWizardInPhase2()
    const { api } = await import('@/api/client')
    routerPush.mockClear()
    vi.mocked(api.previewCampaignRecipients).mockClear()
    vi.mocked(api.replaceCampaignRecipients).mockClear()
    vi.mocked(api.previewCampaignRecipients).mockResolvedValueOnce({ rows: [{ raw: '77011234567', status: 'valid' }], total: 1, valid: 1, invalid: 0, duplicate: 0 })
    vi.mocked(api.replaceCampaignRecipients).mockResolvedValueOnce({ rows: [], total: 1, valid: 1, invalid: 0, duplicate: 0 })
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path.startsWith('/campaigns/camp-1/recipients')) return { items: [], total: 0 } as any
      if (path === '/campaigns/camp-1') return { id: 'camp-1' } as any
      if (path === '/accounts/acct-1/sending-budget') return BUDGET_FIXTURE as any
      throw new Error(`unexpected GET ${path}`)
    })

    // Fake timers here purely to CONTAIN the debounce watcher's own
    // setTimeout that setting pastedText below unavoidably schedules —
    // never advanced, so it is simply discarded on vi.useRealTimers()
    // rather than surviving as a real, still-pending 400ms timer that
    // could fire mid-way through a LATER test. This test is about
    // Create's own auto-check-on-click, already covered by the debounce
    // test above.
    vi.useFakeTimers()
    try {
      ;(wrapper.vm as any).pastedText = '77011234567,Aigul'
      await flushPromises()
      const createBtn = wrapper.findAll('button').find((b) => b.text().includes('Создать рассылку'))
      await createBtn!.trigger('click')
      await flushPromises()
    } finally {
      vi.useRealTimers()
    }

    expect(api.previewCampaignRecipients).toHaveBeenCalledTimes(1)
    expect(api.replaceCampaignRecipients).toHaveBeenCalledTimes(1)
    expect(routerPush).toHaveBeenCalledWith({ name: 'campaign-detail', params: { campaignId: 'camp-1' }, query: { created: '1' } })
  })
})

// CAM-06: the placeholder's two example lines say nothing about header
// rows, separators, or country codes — all auto-detected server-side.
describe('CampaignWizard — CSV/text format help and sample download (CAM-06)', () => {
  it('the format help disclosure explains the header/separator/phone rules', async () => {
    const wrapper = await mountWizardInPhase2()
    const text = wrapper.text()
    expect(text).toContain('Формат CSV/текста')
    expect(text).toContain('запятая')
  })

  it('downloading the sample CSV saves a file named recipients-sample.csv', async () => {
    const wrapper = await mountWizardInPhase2()
    const realCreateElement = document.createElement.bind(document)
    const captured: { anchor: HTMLAnchorElement | null } = { anchor: null }
    const createSpy = vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
      const el = realCreateElement(tag)
      if (tag === 'a') {
        captured.anchor = el as HTMLAnchorElement
        vi.spyOn(el, 'click').mockImplementation(() => {})
      }
      return el
    })
    try {
      await wrapper.find('[data-testid="download-sample-csv"]').trigger('click')
      expect(captured.anchor?.download).toBe('recipients-sample.csv')
    } finally {
      createSpy.mockRestore()
    }
  })
})

// CAM-10: a blank or already-past "later" date used to fall through
// silently — no schedule_at patch was ever sent.
describe('CampaignWizard — scheduled launch requires a valid future date (CAM-10)', () => {
  async function selectLater(wrapper: Awaited<ReturnType<typeof mountWizardInPhase2>>) {
    const laterBtn = wrapper.findAll('button').find((b) => b.text() === 'В назначенное время')
    await laterBtn!.trigger('click')
  }

  it('blocks Create with an inline error when later is chosen but no date is set', async () => {
    const wrapper = await mountWizardInPhase2()
    const { api } = await import('@/api/client')
    vi.mocked(api.previewCampaignRecipients).mockClear()
    vi.mocked(api.replaceCampaignRecipients).mockClear()
    await selectLater(wrapper)

    const createBtn = wrapper.findAll('button').find((b) => b.text().includes('Создать рассылку'))
    await createBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="schedule-error"]').exists()).toBe(true)
    expect(api.previewCampaignRecipients).not.toHaveBeenCalled()
    expect(api.replaceCampaignRecipients).not.toHaveBeenCalled()
  })

  it('blocks Create with an inline error when the chosen date is already in the past', async () => {
    const wrapper = await mountWizardInPhase2()
    await selectLater(wrapper)
    ;(wrapper.vm as any).scheduleAtLocal = '2020-01-01T10:00'
    await flushPromises()

    const createBtn = wrapper.findAll('button').find((b) => b.text().includes('Создать рассылку'))
    await createBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="schedule-error"]').text()).toContain('должно быть в будущем')
  })

  it('a valid future date proceeds, patches schedule_at, and skips the launch-now redirect flag', async () => {
    const wrapper = await mountWizardInPhase2()
    const { api } = await import('@/api/client')
    routerPush.mockClear()
    vi.mocked(api.previewCampaignRecipients).mockClear()
    vi.mocked(api.replaceCampaignRecipients).mockClear()
    vi.mocked(api.previewCampaignRecipients).mockResolvedValueOnce({ rows: [], total: 1, valid: 1, invalid: 0, duplicate: 0 })
    vi.mocked(api.replaceCampaignRecipients).mockResolvedValueOnce({ rows: [], total: 1, valid: 1, invalid: 0, duplicate: 0 })
    vi.mocked(api.patch).mockResolvedValueOnce({ id: 'camp-1' } as any)
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path.startsWith('/campaigns/camp-1/recipients')) return { items: [], total: 0 } as any
      if (path === '/campaigns/camp-1') return { id: 'camp-1' } as any
      if (path === '/accounts/acct-1/sending-budget') return BUDGET_FIXTURE as any
      throw new Error(`unexpected GET ${path}`)
    })

    await selectLater(wrapper)
    const future = new Date(Date.now() + 24 * 60 * 60 * 1000)
    const pad = (n: number) => String(n).padStart(2, '0')
    const local = `${future.getFullYear()}-${pad(future.getMonth() + 1)}-${pad(future.getDate())}T${pad(future.getHours())}:${pad(future.getMinutes())}`
    ;(wrapper.vm as any).scheduleAtLocal = local

    // Fake timers to contain the debounce watcher's own setTimeout (see the
    // identical note on the CAM-04 Create test above).
    vi.useFakeTimers()
    try {
      ;(wrapper.vm as any).pastedText = '77011234567,Aigul'
      await flushPromises()
      const createBtn = wrapper.findAll('button').find((b) => b.text().includes('Создать рассылку'))
      await createBtn!.trigger('click')
      await flushPromises()
    } finally {
      vi.useRealTimers()
    }

    expect(wrapper.find('[data-testid="schedule-error"]').exists()).toBe(false)
    expect(api.patch).toHaveBeenCalledWith('/campaigns/camp-1', expect.objectContaining({ schedule_at: expect.any(String) }))
    expect(routerPush).toHaveBeenCalledWith({ name: 'campaign-detail', params: { campaignId: 'camp-1' } })
  })
})
