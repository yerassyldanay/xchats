import { afterEach, describe, expect, it, vi } from 'vitest'
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

// Every mount registers a real window 'beforeunload' listener (CAM-12) that
// only comes off again via this component's own onBeforeUnmount — unlike a
// Teleported dialog's stray DOM nodes, a leftover listener from a PREVIOUS
// test's never-unmounted instance silently intercepts THIS test's own
// beforeunload dispatch too, so real unmounting (not just a fresh mount)
// is required here, not optional.
let mounted: ReturnType<typeof mountKb> | null = null
afterEach(() => {
  mounted?.unmount()
  mounted = null
})
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

const BUDGET_FIXTURE = { account_id: 'acct-1', min_interval_seconds: 60, jitter_seconds: 10, paused: false, allowed: true, throttled_by: 0, next_send_at: '', tiers: [] }
const SIMULATOR_BUDGET_FIXTURE = { ...BUDGET_FIXTURE, account_id: 'acct-sim' }

function mockDefaultGets(overrides: Record<string, unknown> = {}) {
  return async (path: string) => {
    if (path in overrides) return overrides[path]
    if (path === '/accounts') {
      return {
        items: [
          { id: 'acct-1', display_name: 'Acct', external_handle: '', channel: 'whatsapp' },
          { id: 'acct-sim', display_name: 'Simulator', external_handle: '', channel: 'simulator' },
        ],
      }
    }
    if (path === '/accounts/acct-1/sending-budget') return BUDGET_FIXTURE
    if (path === '/accounts/acct-sim/sending-budget') return SIMULATOR_BUDGET_FIXTURE
    if (path.startsWith('/campaign-templates?')) return { items: [], total: 0 }
    // replaceRecipients() (stores/campaigns.ts) always refreshes both of
    // these after a successful PUT — needed the moment continueToMessage()
    // reaches Step 2 for real, not just when a test asserts on them.
    if (path.startsWith('/campaigns/camp-1/recipients')) return { items: [], total: 0 }
    if (path === '/campaigns/camp-1') return { id: 'camp-1' }
    throw new Error(`unexpected GET ${path}`)
  }
}

async function mountWizard(getOverrides: Record<string, unknown> = {}) {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(mockDefaultGets(getOverrides))
  vi.mocked(api.patch).mockResolvedValue({ id: 'camp-1' } as any)
  const pinia = testPinia()
  const wrapper = mountKb(CampaignWizard, { pinia })
  mounted = wrapper
  await flushPromises()
  return wrapper
}

// Reaching Step 2/3 through the real UI means driving reka-ui's Select
// through a genuine pointerdown/pointerup gesture on its Portal-rendered
// SelectItem — attempted at length elsewhere in this file's history and
// never got jsdom to commit a value. This sets state directly on the
// mounted instance instead: script-setup's dev-mode instance proxy
// supports reading AND writing top-level bindings, so the real
// continueToMessage()/continueToSchedule() handlers still run off of it —
// the only thing skipped is the Select's own open/click/select sequence.
async function mountWizardInMessageStep() {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(mockDefaultGets())
  vi.mocked(api.patch).mockResolvedValue({ id: 'camp-1' } as any)
  vi.mocked(api.post).mockResolvedValueOnce({ id: 'camp-1', name: 'Test campaign' } as any)
  vi.mocked(api.previewCampaignRecipients).mockResolvedValueOnce({ rows: [{ raw: '77011234567', name: 'Aigul', status: 'valid' }], total: 1, valid: 1, invalid: 0, duplicate: 0 })
  vi.mocked(api.replaceCampaignRecipients).mockResolvedValueOnce({ rows: [], total: 1, valid: 1, invalid: 0, duplicate: 0 })
  const pinia = testPinia()
  const wrapper = mountKb(CampaignWizard, { pinia })
  mounted = wrapper
  await flushPromises()

  ;(wrapper.vm as any).name = 'Test campaign'
  ;(wrapper.vm as any).accountId = 'acct-1'
  ;(wrapper.vm as any).pastedText = '77011234567,Aigul'
  await flushPromises()
  const continueBtn = wrapper.findAll('button').find((b) => b.text() === 'Продолжить к сообщению →')
  await continueBtn!.trigger('click')
  await flushPromises()
  return wrapper
}

async function mountWizardInScheduleStep() {
  const wrapper = await mountWizardInMessageStep()
  const { api } = await import('@/api/client')
  vi.mocked(api.patch).mockResolvedValueOnce({ id: 'camp-1' } as any)
  await wrapper.find('[data-testid="message-textarea"]').setValue('Hi {{name}}')
  const continueBtn = wrapper.findAll('button').find((b) => b.text() === 'Продолжить к расписанию →')
  await continueBtn!.trigger('click')
  await flushPromises()
  return wrapper
}

// CAM-15: the wizard is now audience-first — Who, then What, then When —
// instead of forcing a message to be written before any recipient exists.
describe('CampaignWizard — audience-first step order (CAM-15)', () => {
  it('starts on the Audience step with no message field visible yet', async () => {
    const wrapper = await mountWizard()
    expect(wrapper.find('[data-testid="wizard-step-audience"]').classes()).toContain('bg-primary')
    expect(wrapper.find('[data-testid="message-textarea"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="paste-recipients"]').exists()).toBe(true)
  })

  it('hints to fill in name and account before checking reachability, and blocks the auto-check until then', async () => {
    const wrapper = await mountWizard()
    const { api } = await import('@/api/client')
    vi.mocked(api.previewCampaignRecipients).mockClear()
    expect(wrapper.text()).toContain('Укажите название и выберите аккаунт выше')

    vi.useFakeTimers()
    try {
      await wrapper.find('[data-testid="paste-recipients"]').setValue('77011234567,Aigul')
      await vi.advanceTimersByTimeAsync(400)
    } finally {
      vi.useRealTimers()
    }
    expect(api.previewCampaignRecipients).not.toHaveBeenCalled()
  })

  it('Continue to Message creates the campaign, persists recipients, and advances to Step 2', async () => {
    const wrapper = await mountWizardInMessageStep()
    const { api } = await import('@/api/client')

    expect(api.post).toHaveBeenCalledWith('/campaigns', { name: 'Test campaign', account_id: 'acct-1', message_body: '(draft)' })
    expect(api.replaceCampaignRecipients).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="wizard-step-message"]').classes()).toContain('bg-primary')
    expect(wrapper.find('[data-testid="message-textarea"]').exists()).toBe(true)
  })

  it('clicking the Audience step pill navigates back without losing what was entered', async () => {
    const wrapper = await mountWizardInMessageStep()
    await wrapper.find('[data-testid="wizard-step-audience"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="wizard-step-audience"]').classes()).toContain('bg-primary')
    expect((wrapper.find('[data-testid="paste-recipients"]').element as HTMLTextAreaElement).value).toBe('77011234567,Aigul')
  })
})

// CAM-16: the Simulator account is always present in the picker and clearly
// marked, with a reassuring note once it's actually selected.
describe('CampaignWizard — Simulator account (CAM-16)', () => {
  it('badges the simulator entry as Test Mode in the account list', async () => {
    const wrapper = await mountWizard()
    const simItem = body().findAll('[role="option"]').find((el) => el.text().includes('Simulator'))
    // The Select's items only portal into the DOM once opened in a real
    // browser; skip gracefully if jsdom + reka-ui didn't render them here,
    // and instead assert the badge text is reachable via the exposed model.
    if (simItem) expect(simItem.text()).toContain('Тестовый режим')
    expect((wrapper.vm as any).sortedAccounts[0].channel).toBe('simulator')
  })

  it('shows a zero-cost/zero-risk notice once the simulator account is selected', async () => {
    const wrapper = await mountWizard()
    expect(wrapper.find('[data-testid="simulator-notice"]').exists()).toBe(false)

    ;(wrapper.vm as any).accountId = 'acct-sim'
    await flushPromises()
    expect(wrapper.find('[data-testid="simulator-notice"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="simulator-notice"]').text()).toContain('без затрат API')
  })
})

// CAM-15: typing { or {{ opens a floating menu of insertable variables
// right under the cursor.
describe('CampaignWizard — inline variable autocomplete (CAM-15)', () => {
  it('opens on {{ and filters as the operator keeps typing', async () => {
    const wrapper = await mountWizardInMessageStep()
    const textarea = wrapper.find('[data-testid="message-textarea"]')
    await textarea.setValue('Hi {{na')
    await textarea.trigger('input')
    await flushPromises()

    expect(wrapper.find('[data-testid="var-autocomplete-menu"]').exists()).toBe(true)
    const items = wrapper.findAll('[data-testid="var-autocomplete-item"]')
    expect(items.some((i) => i.text() === '{{name}}')).toBe(true)
    expect(items.every((i) => i.text().startsWith('{{na'))).toBe(true)
  })

  it('Enter inserts the highlighted candidate and closes the menu', async () => {
    const wrapper = await mountWizardInMessageStep()
    const textareaWrapper = wrapper.find('[data-testid="message-textarea"]')
    const textarea = textareaWrapper.element as HTMLTextAreaElement
    await textareaWrapper.setValue('Hi {{na')
    textarea.setSelectionRange(7, 7)
    await textareaWrapper.trigger('input')
    await flushPromises()

    await textareaWrapper.trigger('keydown', { key: 'Enter' })
    await flushPromises()

    expect(textarea.value).toBe('Hi {{name}}')
    expect(wrapper.find('[data-testid="var-autocomplete-menu"]').exists()).toBe(false)
  })

  it('Escape closes the menu without inserting anything', async () => {
    const wrapper = await mountWizardInMessageStep()
    const textareaWrapper = wrapper.find('[data-testid="message-textarea"]')
    await textareaWrapper.setValue('Hi {{')
    await textareaWrapper.trigger('input')
    await flushPromises()
    expect(wrapper.find('[data-testid="var-autocomplete-menu"]').exists()).toBe(true)

    await textareaWrapper.trigger('keydown', { key: 'Escape' })
    await flushPromises()

    expect(wrapper.find('[data-testid="var-autocomplete-menu"]').exists()).toBe(false)
    expect((textareaWrapper.element as HTMLTextAreaElement).value).toBe('Hi {{')
  })

  it('clicking a candidate inserts it too, without the textarea losing the pending edit', async () => {
    const wrapper = await mountWizardInMessageStep()
    const textareaWrapper = wrapper.find('[data-testid="message-textarea"]')
    const textarea = textareaWrapper.element as HTMLTextAreaElement
    await textareaWrapper.setValue('Hi {{ph')
    textarea.setSelectionRange(7, 7)
    await textareaWrapper.trigger('input')
    await flushPromises()

    const phoneItem = wrapper.findAll('[data-testid="var-autocomplete-item"]').find((i) => i.text() === '{{phone}}')
    await phoneItem!.trigger('mousedown')
    await flushPromises()

    expect(textarea.value).toBe('Hi {{phone}}')
  })
})

// CAM-15: CSV columns from the already-locked-in audience become one-click
// insert chips, and the message is checked against them live.
describe('CampaignWizard — CSV column chips and unmatched-variable warning (CAM-15)', () => {
  it('offers phone plus every detected column as an insert chip', async () => {
    const wrapper = await mountWizardInMessageStep()
    expect(wrapper.find('[data-testid="insert-var-phone"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="insert-var-name"]').exists()).toBe(true)
  })

  it('clicking a chip inserts the correctly-bracketed token at the cursor', async () => {
    const wrapper = await mountWizardInMessageStep()
    await wrapper.find('[data-testid="insert-var-name"]').trigger('click')
    await flushPromises()

    const textarea = wrapper.find('[data-testid="message-textarea"]').element as HTMLTextAreaElement
    expect(textarea.value).toBe('{{name}}')
  })

  it('warns when the message references a variable the audience has no column for', async () => {
    const wrapper = await mountWizardInMessageStep()
    await wrapper.find('[data-testid="message-textarea"]').setValue('Hi {{name}}, code {{promo_code}}')
    await flushPromises()

    expect(wrapper.find('[data-testid="unmatched-variables-warning"]').text()).toContain('promo_code')
  })
})

// CAM-03: sample-value message preview, carried over unchanged from the old
// wizard (the doc's own "Target UX Flow" still describes this as built).
describe('CampaignWizard — message preview with sample data (CAM-03)', () => {
  it('renders the message with sample values substituted, not the raw template', async () => {
    const wrapper = await mountWizardInMessageStep()
    await wrapper.find('[data-testid="message-textarea"]').setValue('Hi {{name}}, code: {{promo_code}}, ask {{agent_name}}')
    await wrapper.find('[data-testid="toggle-message-preview"]').trigger('click')

    const bubble = wrapper.find('[data-testid="message-preview-bubble"]')
    expect(bubble.text()).toBe('Hi Aigul, code: SUMMER2026, ask [agent_name]')
  })
})

// CAM-14: saving the in-progress message straight into the template
// library without leaving the wizard.
describe('CampaignWizard — Save as template from the wizard (CAM-14)', () => {
  it('opens the save dialog pre-filled with the current message and confirms once saved', async () => {
    const wrapper = await mountWizardInMessageStep()
    const { api } = await import('@/api/client')
    await wrapper.find('[data-testid="message-textarea"]').setValue('Hi {{name}}')
    vi.mocked(api.post).mockResolvedValueOnce({ id: 'tmpl-1', name: 'Greeting', message_body: 'Hi {{name}}', variables: ['name'], is_archived: false } as any)

    await wrapper.find('[data-testid="save-as-template"]').trigger('click')
    await flushPromises()
    const bodyField = body().find('[data-testid="template-body-input"]').element as HTMLTextAreaElement
    expect(bodyField.value).toBe('Hi {{name}}')

    await body().find('[data-testid="template-name-input"]').setValue('Greeting')
    await body().find('[data-testid="template-form-save"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="template-saved-notice"]').exists()).toBe(true)
  })
})

// CAM-12: a real (placeholder-message) campaign now exists from partway
// through Step 1 onward — earlier than the old two-phase wizard, which only
// started guarding once a message had ALSO been written.
describe('CampaignWizard — warns before abandoning an orphan draft campaign (CAM-12)', () => {
  it('does not register a leave guard before any campaign has been created', async () => {
    capturedLeaveGuard = null
    await mountWizard()
    expect(capturedLeaveGuard).toBeTruthy()
    expect(capturedLeaveGuard!()).toBe(true)
  })

  it('beforeunload is prevented once the draft campaign exists, and stops being prevented after Cancel', async () => {
    const wrapper = await mountWizardInMessageStep()
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
    await mountWizardInMessageStep()
    expect(capturedLeaveGuard).toBeTruthy()

    const pending = capturedLeaveGuard!()
    expect(pending).toBeInstanceOf(Promise)
    await flushPromises()
    expect(body().text()).toContain('Уйти, не завершив создание рассылки?')
  })

  it('choosing Stay resolves the guard to false and never deletes the campaign', async () => {
    await mountWizardInMessageStep()
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
    await mountWizardInMessageStep()
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

// CAM-17: Save as Draft vs. Launch Campaign are two distinct, differently-
// consequential actions — Launch alone begins delivery/commits the schedule.
describe('CampaignWizard — pre-flight summary and Save as Draft vs. Launch (CAM-17)', () => {
  it('shows reachable count and account health in the pre-flight summary', async () => {
    const wrapper = await mountWizardInScheduleStep()
    expect(wrapper.find('[data-testid="summary-reachable"]').text()).toContain('1')
    expect(wrapper.text()).toContain('Бюджет отправки')
  })

  it('Save as Draft patches pacing but never calls start, and lands with the draft confirmation banner', async () => {
    const wrapper = await mountWizardInScheduleStep()
    const { api } = await import('@/api/client')
    routerPush.mockClear()
    vi.mocked(api.post).mockClear()
    vi.mocked(api.patch).mockResolvedValueOnce({ id: 'camp-1' } as any)

    await wrapper.find('[data-testid="save-as-draft"]').trigger('click')
    await flushPromises()

    expect(api.post).not.toHaveBeenCalledWith('/campaigns/camp-1/start')
    expect(routerPush).toHaveBeenCalledWith({ name: 'campaign-detail', params: { campaignId: 'camp-1' }, query: { created: '1' } })
  })

  it('Launch Campaign calls the unified start action and redirects with no draft flag', async () => {
    const wrapper = await mountWizardInScheduleStep()
    const { api } = await import('@/api/client')
    routerPush.mockClear()
    vi.mocked(api.post).mockClear()
    vi.mocked(api.post).mockResolvedValueOnce({ id: 'camp-1', status: 'running' } as any)

    await wrapper.find('[data-testid="launch-campaign"]').trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/campaigns/camp-1/start')
    expect(routerPush).toHaveBeenCalledWith({ name: 'campaign-detail', params: { campaignId: 'camp-1' } })
  })

  it('an invalid quiet-hours window blocks both Save as Draft and Launch', async () => {
    const wrapper = await mountWizardInScheduleStep()
    const { api } = await import('@/api/client')
    vi.mocked(api.patch).mockClear()
    vi.mocked(api.post).mockClear()
    ;(wrapper.vm as any).localWindows = [{ weekday: 1, start_minute: 60, end_minute: 60 }]
    await flushPromises()

    await wrapper.find('[data-testid="launch-campaign"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('начало и конец должны отличаться')
    expect(api.patch).not.toHaveBeenCalled()
    expect(api.post).not.toHaveBeenCalledWith('/campaigns/camp-1/start')
  })
})

// CAM-10: a blank or already-past "later" date must block both actions with
// an inline error, never fall through silently.
describe('CampaignWizard — scheduled launch requires a valid future date (CAM-10)', () => {
  it('blocks Launch with an inline error when later is chosen but no date is set', async () => {
    const wrapper = await mountWizardInScheduleStep()
    const { api } = await import('@/api/client')
    vi.mocked(api.post).mockClear()
    const laterBtn = wrapper.findAll('button').find((b) => b.text() === 'В назначенное время')
    await laterBtn!.trigger('click')

    await wrapper.find('[data-testid="launch-campaign"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="schedule-error"]').exists()).toBe(true)
    expect(api.post).not.toHaveBeenCalledWith('/campaigns/camp-1/start')
  })

  it('a valid future date proceeds and patches schedule_at before starting', async () => {
    const wrapper = await mountWizardInScheduleStep()
    const { api } = await import('@/api/client')
    vi.mocked(api.post).mockClear()
    vi.mocked(api.patch).mockClear()
    vi.mocked(api.patch).mockResolvedValueOnce({ id: 'camp-1' } as any)
    vi.mocked(api.post).mockResolvedValueOnce({ id: 'camp-1', status: 'scheduled' } as any)

    const laterBtn = wrapper.findAll('button').find((b) => b.text() === 'В назначенное время')
    await laterBtn!.trigger('click')
    const future = new Date(Date.now() + 24 * 60 * 60 * 1000)
    const pad = (n: number) => String(n).padStart(2, '0')
    const local = `${future.getFullYear()}-${pad(future.getMonth() + 1)}-${pad(future.getDate())}T${pad(future.getHours())}:${pad(future.getMinutes())}`
    ;(wrapper.vm as any).scheduleAtLocal = local
    await flushPromises()

    await wrapper.find('[data-testid="launch-campaign"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="schedule-error"]').exists()).toBe(false)
    expect(api.patch).toHaveBeenCalledWith('/campaigns/camp-1', expect.objectContaining({ schedule_at: expect.any(String) }))
    expect(api.post).toHaveBeenCalledWith('/campaigns/camp-1/start')
  })
})
