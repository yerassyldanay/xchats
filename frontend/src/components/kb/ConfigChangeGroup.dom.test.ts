import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { usePlayground } from '@/stores/playground'
import { useKbModal } from '@/composables/useKbModal'
import { classifyChanges } from '@/composables/draftChanges'
import { api } from '@/api/client'
import { emptyChangeSet } from '@/test/fixtures'
import { mountKb } from '@/test/mount'
import ConfigChangeGroup from './ConfigChangeGroup.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), del: vi.fn(), upload: vi.fn(), mediaURL: vi.fn(), realtimeURL: vi.fn() },
  }
})

beforeEach(() => vi.clearAllMocks())
afterEach(() => useKbModal().close())

function entriesFor(changes = emptyChangeSet()) {
  return classifyChanges(changes, null).find((g) => g.kind === 'config')?.entries ?? []
}

describe('ConfigChangeGroup', () => {
  it('renders one card for one staged field', () => {
    const changes = emptyChangeSet({ config: { persona: 'Дружелюбный' } })
    const wrapper = mountKb(ConfigChangeGroup, { props: { entries: entriesFor(changes) } })
    expect(wrapper.text()).toContain('Персона')
    expect(wrapper.text()).not.toContain('Миссия')
  })

  it('renders one card per changed field for multiple staged fields', () => {
    const changes = emptyChangeSet({ config: { persona: 'P', mission: 'M', reply_max_words: 80 } })
    const wrapper = mountKb(ConfigChangeGroup, { props: { entries: entriesFor(changes) } })
    expect(wrapper.text()).toContain('Персона')
    expect(wrapper.text()).toContain('Миссия')
    expect(wrapper.text()).toContain('Макс. слов в ответе')
  })

  it("the group's publish button calls approveEntity('config', 'main')", async () => {
    const changes = emptyChangeSet({ config: { persona: 'P' } })
    const wrapper = mountKb(ConfigChangeGroup, { props: { entries: entriesFor(changes) } })
    const pg = usePlayground()
    vi.mocked(api.post).mockResolvedValue(emptyChangeSet())
    const spy = vi.spyOn(pg, 'approveEntity')

    const publishBtn = wrapper.findAll('button').find((b) => b.text().includes('Опубликовать изменения ассистента'))
    await publishBtn!.trigger('click')
    await flushPromises()

    expect(spy).toHaveBeenCalledWith('config', 'main')
  })

  it("cancel-all calls cancelChange('config', 'main')", async () => {
    const changes = emptyChangeSet({ config: { persona: 'P', mission: 'M' } })
    const wrapper = mountKb(ConfigChangeGroup, { props: { entries: entriesFor(changes) } })
    const pg = usePlayground()
    vi.mocked(api.del).mockResolvedValue({ changed: true, changes: emptyChangeSet() })
    const spy = vi.spyOn(pg, 'cancelChange')

    const cancelAllBtn = wrapper.findAll('button').find((b) => b.text().includes('Отменить все изменения ассистента'))
    await cancelAllBtn!.trigger('click')
    await flushPromises()

    expect(spy).toHaveBeenCalledWith('config', 'main')
  })

  it('cancelling one field calls cancelChange with just that field, retaining the others in the request target', async () => {
    const changes = emptyChangeSet({ config: { persona: 'P', mission: 'M' } })
    const wrapper = mountKb(ConfigChangeGroup, { props: { entries: entriesFor(changes) } })
    const pg = usePlayground()
    vi.mocked(api.del).mockResolvedValue({ changed: true, changes: emptyChangeSet({ config: { mission: 'M' } }) })
    const spy = vi.spyOn(pg, 'cancelChange')

    const cancelFieldBtn = wrapper.findAll('button').find((b) => b.text() === 'Отменить изменение')
    await cancelFieldBtn!.trigger('click')
    await flushPromises()

    expect(spy).toHaveBeenCalledWith('config', expect.stringMatching(/persona|mission/))
    expect(spy).not.toHaveBeenCalledWith('config', 'main')
  })
})
