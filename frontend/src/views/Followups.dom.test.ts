import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises, type VueWrapper } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useCrm } from '@/stores/crm'
import type { Followup } from '@/types'
import Followups from './Followups.vue'

const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: { ...actual.api, get: vi.fn(), post: vi.fn(), patch: vi.fn() },
  }
})

function followup(over: Partial<Followup>): Followup {
  return {
    id: 'fu-1',
    customer_id: 'c-1',
    customer_name: 'Иван Смирнов',
    conversation_id: 'chat-1',
    channel: 'whatsapp',
    due_at: new Date().toISOString(),
    due_date: '2026-09-02',
    due_minute: 600,
    action: 'call',
    note: '',
    assignee_user_id: null,
    assignee_name: '',
    state: 'open',
    completed_at: null,
    created_at: '2026-09-01T00:00:00Z',
    ...over,
  }
}

const BUCKETS_EMPTY = { today: 0, tomorrow: 0, this_week: 0, overdue: 0 }

// api.get/post/patch are ONE shared mock for the whole file (module-level
// vi.mock above) — without this, an earlier test's call history and queued
// mockResolvedValueOnce leak into the next test's assertions.
beforeEach(() => vi.clearAllMocks())

// Teleported dialog content (reka-ui's Dialog) is not removed automatically
// between tests — mirrors Channels.dom.test.ts's own body() + unmount pattern.
function body() {
  return new DOMWrapper(document.body)
}
let mounted: VueWrapper<any> | undefined
afterEach(() => {
  mounted?.unmount()
  mounted = undefined
})

async function mountBoard(opts: { open?: Followup[]; get?: Record<string, unknown> } = {}) {
  const { api } = await import('@/api/client')
  const open = opts.open ?? []
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path.startsWith('/followups/buckets')) return BUCKETS_EMPTY as never
    if (path.startsWith('/followups') && path.includes('state=open')) {
      return { items: open, page: 1, page_size: 200, total: open.length } as never
    }
    if (path.startsWith('/followups') && path.includes('state=completed')) {
      return { items: [], page: 1, page_size: 100, total: 0 } as never
    }
    if (path === '/crm/tags' || path === '/crm/statuses' || path === '/crm/custom-fields') return { items: [] } as never
    if (path.startsWith('/users')) return { items: [] } as never
    if (opts.get && path in opts.get) return opts.get[path] as never
    throw new Error(`unexpected GET ${path}`)
  })

  const pinia = testPinia()
  const wrapper = mountKb(Followups, { pinia })
  mounted = wrapper
  await flushPromises()
  return { wrapper, crm: useCrm(), api }
}

describe('Followups board — time grouping', () => {
  it('groups open tasks into Overdue/Today/Tomorrow/Later sections, all visible at once', async () => {
    const now = new Date()
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000)
    const nextWeek = new Date(now.getTime() + 10 * 24 * 60 * 60 * 1000)
    const { wrapper } = await mountBoard({
      open: [
        followup({ id: 'fu-overdue', customer_name: 'Просроченный клиент', due_at: yesterday.toISOString() }),
        followup({ id: 'fu-today', customer_name: 'Сегодняшний клиент', due_at: now.toISOString() }),
        followup({ id: 'fu-later', customer_name: 'Клиент на потом', due_at: nextWeek.toISOString() }),
      ],
    })

    expect(wrapper.find('[data-testid="followups-section-overdue"]').text()).toContain('Просроченный клиент')
    expect(wrapper.find('[data-testid="followups-section-today"]').text()).toContain('Сегодняшний клиент')
    expect(wrapper.find('[data-testid="followups-section-later"]').text()).toContain('Клиент на потом')
    // All three sections render simultaneously — nothing is hidden behind a filter.
    expect(wrapper.findAll('[data-testid="followup-card"]')).toHaveLength(3)
  })

  it('filters by search text across customer name and note', async () => {
    const { wrapper } = await mountBoard({
      open: [
        followup({ id: 'fu-1', customer_name: 'Айгуль' }),
        followup({ id: 'fu-2', customer_name: 'Марат', note: 'просил перезвонить айгуль' }),
        followup({ id: 'fu-3', customer_name: 'Бота' }),
      ],
    })
    expect(wrapper.findAll('[data-testid="followup-card"]')).toHaveLength(3)

    await wrapper.find('[data-testid="followups-search"]').setValue('айгуль')
    await flushPromises()

    const cards = wrapper.findAll('[data-testid="followup-card"]')
    expect(cards).toHaveLength(2) // matches by name (Айгуль) and by note (Марат's)
  })

  it('filters by action type', async () => {
    const { wrapper } = await mountBoard({
      open: [
        followup({ id: 'fu-call', action: 'call', customer_name: 'Звонок' }),
        followup({ id: 'fu-msg', action: 'message', customer_name: 'Сообщение' }),
      ],
    })
    const callTab = wrapper.findAll('button').find((b) => b.text() === 'Позвонить')
    expect(callTab).toBeDefined()
    await callTab!.trigger('click')
    await flushPromises()

    const cards = wrapper.findAll('[data-testid="followup-card"]')
    expect(cards).toHaveLength(1)
    expect(cards[0].text()).toContain('Звонок')
  })
})

describe('Followups board — actions', () => {
  it('completing a card removes it from the board and calls the complete endpoint', async () => {
    const { wrapper, api } = await mountBoard({ open: [followup({ id: 'fu-1' })] })
    vi.mocked(api.post).mockResolvedValueOnce({ ...followup({ id: 'fu-1', state: 'completed' }) } as never)

    const completeBtn = wrapper.findAll('button').find((b) => b.text().includes('Выполнено'))
    expect(completeBtn).toBeDefined()
    await completeBtn!.trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/followups/fu-1/complete')
    expect(wrapper.findAll('[data-testid="followup-card"]')).toHaveLength(0)
  })

  it('opens the conversation for a card with a linked chat', async () => {
    const { wrapper } = await mountBoard({ open: [followup({ id: 'fu-1', conversation_id: 'chat-42' })] })
    const openBtn = wrapper.findAll('button').find((b) => b.text().includes('Открыть диалог'))
    await openBtn!.trigger('click')
    expect(push).toHaveBeenCalledWith({ name: 'chatboard', params: { chatId: 'chat-42' } })
  })
})

describe('Followups board — "+ Новая задача" (customer picker)', () => {
  it('requires picking a customer, then creates the task against the picked one', async () => {
    const { wrapper, api } = await mountBoard({
      get: {
        '/customers?q=%D0%B0%D0%B9&page_size=8': {
          items: [{ id: 'cust-9', display_name: 'Айгуль Ахметова', phone: '', email: '', avatar_url: '', status_id: null, status: null, assignee_user_id: null, tags: [], identities: [], custom_fields: {}, created_at: '', updated_at: '' }],
        },
      },
    })

    await wrapper.find('[data-testid="followups-new-task"]').trigger('click')
    await flushPromises()

    // Save without picking a customer is rejected client-side.
    const dialogSave = body().findAll('button').find((b) => b.text() === 'Сохранить')
    await dialogSave!.trigger('click')
    await flushPromises()
    expect(body().text()).toContain('Выберите клиента.')
    expect(api.post).not.toHaveBeenCalledWith('/followups', expect.anything())

    // The customer search is debounced (250ms) — a real wait rather than
    // vi.useFakeTimers()+advanceTimersByTimeAsync (the pattern
    // CampaignTemplatesPanel.dom.test.ts uses for a debounce with no further
    // interaction afterward): here the debounce is followed by clicking a
    // result the debounce itself renders, and fake timers left that click
    // not reaching its handler — a real wait sidesteps the whole question.
    await body().find('input[placeholder*="Имя"]').setValue('ай')
    await new Promise((r) => setTimeout(r, 400))
    await flushPromises()

    const result = body().findAll('button').find((b) => b.text().includes('Айгуль Ахметова'))
    expect(result).toBeDefined()
    await result!.trigger('click')
    await flushPromises()

    vi.mocked(api.post).mockResolvedValueOnce(followup({ id: 'new-fu', customer_id: 'cust-9', customer_name: 'Айгуль Ахметова' }) as never)
    await body().findAll('button').find((b) => b.text() === 'Сохранить')!.trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/followups', expect.objectContaining({ customer_id: 'cust-9' }))
  })
})

describe('Followups board — Reschedule quick presets', () => {
  it('"+1 час" reschedules immediately with no extra confirmation step', async () => {
    const { wrapper, api } = await mountBoard({ open: [followup({ id: 'fu-1' })] })
    vi.mocked(api.patch).mockResolvedValueOnce(followup({ id: 'fu-1' }) as never)

    const rescheduleBtn = wrapper.findAll('button').find((b) => b.text().includes('Перенести'))
    await rescheduleBtn!.trigger('click')
    await flushPromises()

    await body().find('[data-testid="reschedule-plus-hour"]').trigger('click')
    await flushPromises()

    expect(api.patch).toHaveBeenCalledWith('/followups/fu-1', expect.objectContaining({ action: 'call' }))
    // The dialog closes itself after a preset applies.
    expect(body().find('[data-testid="reschedule-plus-hour"]').exists()).toBe(false)
  })
})

describe('Followups — Completed tab', () => {
  it('lists completed tasks with their completion time, and can reopen one', async () => {
    const { wrapper, api } = await mountBoard()
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path.startsWith('/followups/buckets')) return BUCKETS_EMPTY as never
      if (path.startsWith('/followups') && path.includes('state=open')) return { items: [], page: 1, page_size: 200, total: 0 } as never
      if (path.startsWith('/followups') && path.includes('state=completed')) {
        return {
          items: [followup({ id: 'done-1', state: 'completed', completed_at: '2026-09-01T12:00:00Z', customer_name: 'Завершённый клиент' })],
          page: 1, page_size: 100, total: 1,
        } as never
      }
      if (path === '/crm/tags' || path === '/crm/statuses' || path === '/crm/custom-fields') return { items: [] } as never
      if (path.startsWith('/users')) return { items: [] } as never
      throw new Error(`unexpected GET ${path}`)
    })

    // reka-ui's TabsTrigger selects on mousedown, not click — see
    // Channels.dom.test.ts's identical note.
    const completedTab = wrapper.findAll('button').find((b) => b.text() === 'Выполненные')
    await completedTab!.trigger('mousedown', { button: 0 })
    await flushPromises()

    expect(wrapper.find('[data-testid="completed-followup-row"]').text()).toContain('Завершённый клиент')

    vi.mocked(api.patch).mockResolvedValueOnce(followup({ id: 'done-1', state: 'open' }) as never)
    await wrapper.findAll('button').find((b) => b.text().includes('Вернуть в работу'))!.trigger('click')
    await flushPromises()

    expect(api.patch).toHaveBeenCalledWith('/followups/done-1', expect.objectContaining({ customer_id: 'c-1' }))
    expect(wrapper.find('[data-testid="completed-followup-row"]').exists()).toBe(false)
  })
})
