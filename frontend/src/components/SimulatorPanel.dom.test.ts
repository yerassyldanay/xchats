import { describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises } from '@vue/test-utils'
import { mountKb } from '@/test/mount'
import SimulatorPanel from './SimulatorPanel.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, del: vi.fn(), post: vi.fn() } }
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
