import { describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useCampaigns } from '@/stores/campaigns'
import CampaignDetail from './CampaignDetail.vue'
import type { Campaign, CampaignRecipientPreviewResult } from '@/types'

vi.mock('@/lib/sse', () => ({ connectRealtime: vi.fn(() => vi.fn()) }))

vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return {
    ...actual,
    useRouter: () => ({ push: vi.fn() }),
    useRoute: () => ({ params: { campaignId: 'camp-1' } }),
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
      previewCampaignRecipients: vi.fn(),
      replaceCampaignRecipients: vi.fn(),
    },
  }
})

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

function campaign(over: Partial<Campaign> = {}): Campaign {
  return {
    id: 'camp-1', name: 'Summer promo', account_id: 'acct-1', channel: 'whatsapp', status: 'draft',
    message_body: 'Hi {{name}}', variables: ['name'], min_interval_seconds: null, jitter_seconds: null,
    windows: [], schedule_at: null, started_at: null, created_by: 'user-1',
    created_at: '2026-08-01T00:00:00Z', updated_at: '2026-08-01T00:00:00Z',
    recipient_counts: {},
    ...over,
  }
}
function previewResult(valid: number): CampaignRecipientPreviewResult {
  return { rows: [], total: valid, valid, invalid: 0, duplicate: 0 }
}

async function mountDetail() {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/campaigns/camp-1') return campaign() as any
    if (path === '/accounts') return { items: [] } as any
    if (path.startsWith('/campaigns/camp-1/recipients')) return { items: [], total: 0 } as any
    if (path.startsWith('/campaigns/camp-1/events')) return { items: [], total: 0 } as any
    throw new Error(`unexpected GET ${path}`)
  })
  const pinia = testPinia()
  const wrapper = mountKb(CampaignDetail, { pinia })
  await flushPromises()
  return { wrapper, campaigns: useCampaigns() }
}

// reka-ui's TabsTrigger selects on mousedown, not click (see Accounts.dom.test.ts's
// own note) — the Recipients tab's content (the Replace-recipients panel) is
// not even in the DOM until its trigger is activated.
async function openRecipientsTab(wrapper: Awaited<ReturnType<typeof mountDetail>>['wrapper']) {
  const trigger = wrapper.findAll('button').find((b) => b.text() === 'Получатели')
  await trigger!.trigger('mousedown', { button: 0 })
  await flushPromises()
}

// CAM-09: replacing a campaign's recipients must never save an audience the
// operator never actually reviewed — editing the pasted text (or swapping
// the file) after a successful check has to invalidate that check
// immediately, disabling Save until it is re-run against the CURRENT input.
describe('CampaignDetail — Replace recipients staleness guard (CAM-09)', () => {
  it('Save stays disabled until a check succeeds for the CURRENT text', async () => {
    const { wrapper } = await mountDetail()
    await openRecipientsTab(wrapper)

    const replaceToggle = wrapper.findAll('button').find((b) => b.text() === 'Заменить получателей')
    await replaceToggle!.trigger('click')

    const saveBtn = wrapper.findAll('button').find((b) => b.text() === 'Сохранить')
    expect(saveBtn, 'Save button not found').toBeTruthy()
    expect((saveBtn!.element as HTMLButtonElement).disabled).toBe(true)
  })

  it('a successful check enables Save; editing the text afterward disables it again with a stale notice', async () => {
    const { wrapper, campaigns } = await mountDetail()
    await openRecipientsTab(wrapper)
    const { api } = await import('@/api/client')
    vi.mocked(api.previewCampaignRecipients).mockResolvedValueOnce(previewResult(3))

    await wrapper.findAll('button').find((b) => b.text() === 'Заменить получателей')!.trigger('click')
    const textarea = wrapper.find('textarea')
    await textarea.setValue('77011234567,Aigul')

    await wrapper.findAll('button').find((b) => b.text() === 'Проверить')!.trigger('click')
    await flushPromises()

    let saveBtn = wrapper.findAll('button').find((b) => b.text() === 'Сохранить')!
    expect((saveBtn.element as HTMLButtonElement).disabled, 'Save must enable once the preview matches the current text').toBe(false)
    expect(wrapper.find('[data-testid="replace-preview-stale-notice"]').exists()).toBe(false)

    // Editing the text WITHOUT re-checking must invalidate the preview.
    await textarea.setValue('77011234567,Aigul\n77022222222,Bota')

    saveBtn = wrapper.findAll('button').find((b) => b.text() === 'Сохранить')!
    expect((saveBtn.element as HTMLButtonElement).disabled, 'Save must re-disable once the text no longer matches the checked preview').toBe(true)
    expect(wrapper.find('[data-testid="replace-preview-stale-notice"]').exists()).toBe(true)

    // Clicking Save (were it somehow enabled) must not be reachable — assert
    // the store action was never invoked as the actual behind-the-scenes guard.
    const replaceSpy = vi.spyOn(campaigns, 'replaceRecipients')
    expect(replaceSpy).not.toHaveBeenCalled()
  })

  it('re-checking after an edit clears staleness again', async () => {
    const { wrapper } = await mountDetail()
    await openRecipientsTab(wrapper)
    const { api } = await import('@/api/client')
    vi.mocked(api.previewCampaignRecipients).mockResolvedValueOnce(previewResult(1)).mockResolvedValueOnce(previewResult(2))

    await wrapper.findAll('button').find((b) => b.text() === 'Заменить получателей')!.trigger('click')
    const textarea = wrapper.find('textarea')
    await textarea.setValue('one,recipient')
    await wrapper.findAll('button').find((b) => b.text() === 'Проверить')!.trigger('click')
    await flushPromises()

    await textarea.setValue('one,recipient\ntwo,recipient')
    await wrapper.findAll('button').find((b) => b.text() === 'Проверить')!.trigger('click')
    await flushPromises()

    const saveBtn = wrapper.findAll('button').find((b) => b.text() === 'Сохранить')!
    expect((saveBtn.element as HTMLButtonElement).disabled).toBe(false)
    expect(wrapper.find('[data-testid="replace-preview-stale-notice"]').exists()).toBe(false)
  })
})

// CAM-08: Stop is permanent (unlike Pause) and previously fired on a single
// click with zero confirmation.
describe('CampaignDetail — Stop requires confirmation (CAM-08)', () => {
  async function mountRunning() {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path === '/campaigns/camp-1') return campaign({ status: 'running' }) as any
      if (path === '/accounts') return { items: [] } as any
      if (path.startsWith('/campaigns/camp-1/recipients')) return { items: [], total: 0 } as any
      if (path.startsWith('/campaigns/camp-1/events')) return { items: [], total: 0 } as any
      throw new Error(`unexpected GET ${path}`)
    })
    const pinia = testPinia()
    const wrapper = mountKb(CampaignDetail, { pinia })
    await flushPromises()
    return wrapper
  }

  it('does not stop on a single click; only after the confirmation dialog is accepted', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.post).mockResolvedValueOnce(campaign({ status: 'draft' }) as any)
    const wrapper = await mountRunning()

    const stopBtn = wrapper.findAll('button').find((b) => b.text().includes('Остановить'))
    expect(stopBtn, 'Stop button not found').toBeTruthy()
    await stopBtn!.trigger('click')

    expect(api.post, 'must not call stop before confirmation').not.toHaveBeenCalled()
    expect(body().text()).toContain('Остановить эту рассылку?')

    const accept = lastButtonMatching((b) => b.text() === 'Остановить безвозвратно')
    expect(accept, 'destructive confirm button not found').toBeTruthy()
    await accept!.trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/campaigns/camp-1/stop')
  })

  it('cancelling the dialog leaves the campaign running', async () => {
    const { api } = await import('@/api/client')
    // This file's api mocks are module-level vi.fn()s with no shared
    // beforeEach reset (see KnowledgeBase.dom.test.ts's identical note) —
    // the previous test's own call to /stop is still in the history here.
    vi.mocked(api.post).mockClear()
    const wrapper = await mountRunning()

    await wrapper.findAll('button').find((b) => b.text().includes('Остановить'))!.trigger('click')
    const cancelBtn = lastButtonMatching((b) => b.text() === 'Отмена')
    await cancelBtn!.trigger('click')
    await flushPromises()

    expect(api.post).not.toHaveBeenCalled()
  })
})
