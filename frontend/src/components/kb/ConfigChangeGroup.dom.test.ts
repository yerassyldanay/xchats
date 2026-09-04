import { beforeEach, describe, expect, it, vi } from 'vitest'
import { usePlayground } from '@/stores/playground'
import { useCancelConfirm } from '@/composables/useCancelConfirm'
import { mountKb, testPinia } from '@/test/mount'
import ConfigChangeGroup from './ConfigChangeGroup.vue'
import AssistantFieldRecord from './records/AssistantFieldRecord.vue'
import type { DraftChangeSet, DraftView } from '@/types'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: { ...actual.api, get: vi.fn(), post: vi.fn(), patch: vi.fn(), del: vi.fn() },
  }
})

function seed(configPatch: DraftChangeSet['config']) {
  const pinia = testPinia()
  const pg = usePlayground()
  pg.changes = {
    base_version: 1, updated_at: '', config: configPatch,
    topics: [], tariffs: [], products: [], contacts: [], policies: [], tariff_info: [], zones: [], deletes: [],
  }
  pg.live = {
    config: {
      organization_id: 'org-1', persona: 'Старая персона', mission: 'Старая миссия', guardrails: '',
      language_policy: '', reply_max_words: 120, draft: false, base_version: 0, updated_at: '',
    },
    topics: [], tariffs: [], products: [], contacts: [], policies: [], tariff_info: [], zones: [], materials: [], requests: [],
  } as DraftView
  return { pinia, pg }
}

describe('ConfigChangeGroup', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // useCancelConfirm's request is a module-level singleton (one dialog per
    // page), so a pending request from a previous test would leak into the
    // next one's assertion.
    testPinia()
    useCancelConfirm().close()
  })

  it('renders nothing when config has no pending change', () => {
    const { pinia } = seed(null)
    const wrapper = mountKb(ConfigChangeGroup, { pinia })
    expect(wrapper.findAllComponents(AssistantFieldRecord)).toHaveLength(0)
    expect(wrapper.text()).toBe('')
  })

  it('one staged field renders exactly one card', () => {
    const { pinia } = seed({ persona: 'Новая персона' })
    const wrapper = mountKb(ConfigChangeGroup, { pinia })
    expect(wrapper.findAllComponents(AssistantFieldRecord)).toHaveLength(1)
  })

  it('multiple staged fields render one card each', () => {
    const { pinia } = seed({ persona: 'Новая персона', mission: 'Новая миссия' })
    const wrapper = mountKb(ConfigChangeGroup, { pinia })
    expect(wrapper.findAllComponents(AssistantFieldRecord)).toHaveLength(2)
  })

  // Cancel is confirmed rather than immediate now (useCancelConfirm), so
  // what this component is responsible for is REQUESTING the right target
  // and touching nothing until the operator agrees. The dialog itself lives
  // on the page (DraftKnowledgeBase), not here.
  it('cancelling one field requests confirmation for just that field, without touching the store', async () => {
    const { pinia, pg } = seed({ persona: 'Новая персона', mission: 'Новая миссия' })
    const cancelSpy = vi.spyOn(pg, 'cancelChange').mockResolvedValue(true)
    const wrapper = mountKb(ConfigChangeGroup, { pinia })

    const cards = wrapper.findAllComponents(AssistantFieldRecord)
    await cards[0].vm.$emit('cancel')

    expect(cancelSpy).not.toHaveBeenCalled()
    expect(useCancelConfirm().pending.value?.targets).toEqual([{ kind: 'config', key: cards[0].props('section').key }])
  })

  it('the group publish button calls approveEntity(config, main)', async () => {
    const { pinia, pg } = seed({ persona: 'Новая персона' })
    const approveSpy = vi.spyOn(pg, 'approveEntity').mockResolvedValue(true)
    const wrapper = mountKb(ConfigChangeGroup, { pinia })

    const publishBtn = wrapper.findAll('button').find((b) => b.text().includes('Опубликовать изменения ассистента'))
    expect(publishBtn).toBeTruthy()
    await publishBtn!.trigger('click')

    expect(approveSpy).toHaveBeenCalledWith('config', 'main')
  })

  it('the cancel-all button requests confirmation for config/main, without touching the store', async () => {
    const { pinia, pg } = seed({ persona: 'Новая персона' })
    const cancelSpy = vi.spyOn(pg, 'cancelChange').mockResolvedValue(true)
    const wrapper = mountKb(ConfigChangeGroup, { pinia })

    const cancelAllBtn = wrapper.findAll('button').find((b) => b.text().includes('Отменить все изменения ассистента'))
    expect(cancelAllBtn).toBeTruthy()
    await cancelAllBtn!.trigger('click')

    expect(cancelSpy).not.toHaveBeenCalled()
    expect(useCancelConfirm().pending.value?.targets).toEqual([{ kind: 'config', key: 'main' }])
  })
})
