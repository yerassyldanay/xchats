import { beforeEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises } from '@vue/test-utils'
import { mountKb } from '@/test/mount'
import SimulatorPanel from './SimulatorPanel.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, del: vi.fn(), post: vi.fn(), get: vi.fn() } }
})

// Every test here mounts with no authenticated user, so the persistence key
// (SimulatorPanel.vue's storageKey) always resolves to the same 'sim-user-default'
// fallback — clear it before each test so one test's session never leaks
// into the next (same pattern as ChatList.dom.test.ts's collapse-toggle suite).
// api.get/post/del are ONE shared mock for the whole file (module-level
// vi.mock above): without clearing, an earlier test's recorded calls (and
// any mockImplementation it installed) leak into the next test's assertions.
beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
})

// reka-ui's Dialog renders through a Teleport into document.body.
function body() {
  return new DOMWrapper(document.body)
}

// KB-12: "Clear simulator data" hard-deletes every Simulator conversation/
// customer for the organization — it must never fire without confirmation,
// matching the same styled-dialog pattern used everywhere else destructive
// actions live (ConfirmDeleteDialog, reused here).
describe('SimulatorPanel — Clear simulator data requires confirmation', () => {
  it('does not call DELETE /simulator/data until the operator confirms, then reports the result', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.del).mockResolvedValueOnce({ conversations_deleted: 3, customers_deleted: 2 } as any)

    const wrapper = mountKb(SimulatorPanel)
    await flushPromises()

    const clearBtn = wrapper.find('[data-testid="simulator-clear-data"]')
    expect(clearBtn.exists()).toBe(true)
    await clearBtn.trigger('click')
    expect(api.del).not.toHaveBeenCalled()

    const accept = body().find('[data-testid="confirm-accept"]')
    expect(accept.exists()).toBe(true)
    await accept.trigger('click')
    await flushPromises()

    expect(api.del).toHaveBeenCalledWith('/simulator/data')
    expect(wrapper.find('[data-testid="simulator-clear-success"]').text()).toContain('3')
    expect(wrapper.find('[data-testid="simulator-clear-success"]').text()).toContain('2')
  })

  it('surfaces an API error instead of a silent failure', async () => {
    const { api, ApiError } = await import('@/api/client')
    vi.mocked(api.del).mockRejectedValueOnce(new ApiError('INTERNAL', 500, 'boom'))

    const wrapper = mountKb(SimulatorPanel)
    await flushPromises()
    await wrapper.find('[data-testid="simulator-clear-data"]').trigger('click')
    await body().find('[data-testid="confirm-accept"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="simulator-clear-error"]').text()).toContain('boom')
  })
})

// KB-02: the environment toggle picks which KB a send answers against.
describe('SimulatorPanel — test environment toggle (KB-02)', () => {
  it('defaults to live (use_draft: false) and switches to the draft on toggle', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.post).mockResolvedValue({
      conversation_id: 'c1', message_id: 'm1',
      draft: { id: 'd1', text: 'ok', reply_language: 'ru', escalate: false },
    } as any)

    const wrapper = mountKb(SimulatorPanel)
    await flushPromises()

    await wrapper.find('[data-testid="simulator-input"]').setValue('Привет')
    await wrapper.find('[data-testid="simulator-send"]').trigger('click')
    await flushPromises()
    expect(api.post).toHaveBeenLastCalledWith('/simulator/messages', expect.objectContaining({ use_draft: false }))
    expect(wrapper.find('[data-testid="simulator-message-draft-badge"]').exists()).toBe(false)

    // reka-ui's TabsTrigger selects on mousedown, not click.
    await wrapper.find('[data-testid="simulator-env-draft"]').trigger('mousedown', { button: 0 })
    await flushPromises()
    await wrapper.find('[data-testid="simulator-input"]').setValue('Ещё вопрос')
    await wrapper.find('[data-testid="simulator-send"]').trigger('click')
    await flushPromises()
    expect(api.post).toHaveBeenLastCalledWith('/simulator/messages', expect.objectContaining({ use_draft: true }))
    // The reply generated while the draft toggle was active is labelled as such.
    expect(wrapper.findAll('[data-testid="simulator-message-draft-badge"]')).toHaveLength(1)
  })
})

// TODO.md Simulator phase: the session used to be crypto.randomUUID() on
// every mount, which lost the whole thread (and minted a duplicate Inbox
// chat) on a plain page refresh — these pin the localStorage-backed fix.
describe('SimulatorPanel — persistent session', () => {
  it('restores a prior thread\'s history (messages + latest draft) on mount instead of showing the empty hero', async () => {
    localStorage.setItem('sim-user-default', JSON.stringify({ ref: 'sim-user-default', conversationId: 'chat-1' }))
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path === '/chats/chat-1/messages?limit=80') {
        return {
          items: [{
            id: 'm1', chat_id: 'chat-1', direction: 'in', sender_type: 'contact',
            external_message_id: 'x', message_type: 'conversation', content: 'Привет',
            media: [], status: 'received', source: 'simulator', timestamp: null,
          }],
        } as any
      }
      if (path === '/chats/chat-1/ai-drafts') {
        return {
          items: [{
            id: 'd1', chat_id: 'chat-1', trigger_message_id: 'm1', ordinal: 0,
            draft_text: 'Здравствуйте! Чем могу помочь?', context_status: 'ok', confidence: null,
            escalate: false, escalation_reason: '', status: 'suggested', created_at: '',
          }],
        } as any
      }
      throw new Error(`unexpected GET ${path}`)
    })

    const wrapper = mountKb(SimulatorPanel)
    await flushPromises()

    expect(wrapper.find('[data-testid="simulator-hero"]').exists()).toBe(false)
    const rows = wrapper.findAll('[data-testid="simulator-message"]')
    expect(rows).toHaveLength(2)
    expect(rows[0].attributes('data-role')).toBe('user')
    expect(rows[0].text()).toContain('Привет')
    expect(rows[1].attributes('data-role')).toBe('assistant')
    expect(rows[1].text()).toContain('Здравствуйте! Чем могу помочь?')
  })

  it('shows the onboarding hero, not a spinner-then-nothing, for a brand new session', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(async () => {
      throw new Error('a fresh session must never call GET — there is no conversationId to fetch yet')
    })

    const wrapper = mountKb(SimulatorPanel)
    await flushPromises()

    expect(api.get).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="simulator-hero"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-testid="simulator-hero-suggestion"]').length).toBeGreaterThan(0)
  })

  it('"+ New conversation" clears the thread on screen and mints a fresh session, without touching the old one', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockResolvedValue({ items: [] } as any)
    vi.mocked(api.post).mockResolvedValueOnce({
      conversation_id: 'chat-1', message_id: 'm1',
      draft: { id: 'd1', text: 'ok', reply_language: 'ru', escalate: false },
    } as any)

    const wrapper = mountKb(SimulatorPanel)
    await flushPromises()
    await wrapper.find('[data-testid="simulator-input"]').setValue('Привет')
    await wrapper.find('[data-testid="simulator-send"]').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('[data-testid="simulator-message"]')).toHaveLength(2)

    const before = JSON.parse(localStorage.getItem('sim-user-default')!)
    expect(before.conversationId).toBe('chat-1')

    await wrapper.find('[data-testid="simulator-new-conversation"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="simulator-hero"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-testid="simulator-message"]')).toHaveLength(0)
    const after = JSON.parse(localStorage.getItem('sim-user-default')!)
    expect(after.conversationId).toBeNull()
    expect(after.ref).not.toBe(before.ref) // a genuinely new thread, not a reset of the old one
  })

  it('a hero suggestion sends immediately, with its own text, on one click', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.post).mockResolvedValueOnce({
      conversation_id: 'chat-1', message_id: 'm1',
      draft: { id: 'd1', text: 'ok', reply_language: 'ru', escalate: false },
    } as any)

    const wrapper = mountKb(SimulatorPanel)
    await flushPromises()
    const chip = wrapper.findAll('[data-testid="simulator-hero-suggestion"]')[0]
    const suggestionText = chip.text()
    await chip.trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/simulator/messages', expect.objectContaining({ text: suggestionText }))
    expect(wrapper.find('[data-testid="simulator-message"]').text()).toContain(suggestionText)
  })
})
