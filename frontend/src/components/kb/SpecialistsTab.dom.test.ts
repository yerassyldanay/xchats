import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, type VueWrapper } from '@vue/test-utils'
import { usePlayground } from '@/stores/playground'
import { mountKb, testPinia } from '@/test/mount'
import SpecialistsTab from './SpecialistsTab.vue'
import type { DraftView, SpecialistRow } from '@/types'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), patch: vi.fn(), del: vi.fn() } }
})

// useKbModal's session is a module-level singleton (see its own doc
// comment) — unmount after every test so an Edit click in one test can't
// leak an open dialog session into the next.
let mounted: VueWrapper | undefined
afterEach(() => {
  mounted?.unmount()
  mounted = undefined
})

function specialist(over: Partial<SpecialistRow> = {}): SpecialistRow {
  return {
    id: 'alina-kim', ref: 'alina-kim', full_name: 'Алина Ким', title: 'Топ-стилист / Колорист',
    experience: '7 лет',
    schedule: [{ ref: 'tue', day: 'Вторник', start: '10:00', end: '19:00', breaks: [] }],
    booking_url: 'https://xpayment.kz/book/aura-alina',
    portfolio_images: ['img-1', 'img-2'],
    sales_status: 'active',
    draft: false, updated_at: '',
    ...over,
  }
}

function emptyLive(specialists: SpecialistRow[]): DraftView {
  return {
    config: {
      organization_id: 'org-1', persona: '', mission: '', guardrails: '', language_policy: '',
      reply_max_words: 120, draft: false, base_version: 0, updated_at: '',
    },
    topics: [], tariffs: [], products: [], specialists, services: [],
    contacts: [], policies: [], tariff_info: [], zones: [], materials: [], requests: [],
  }
}

function mountTab(specialists: SpecialistRow[]) {
  const pinia = testPinia()
  const pg = usePlayground()
  pg.live = emptyLive(specialists)
  const wrapper = mountKb(SpecialistsTab, { pinia })
  mounted = wrapper
  return { wrapper, pg }
}

describe('SpecialistsTab — active/archived filtering', () => {
  it('defaults to showing only active specialists', () => {
    const { wrapper } = mountTab([
      specialist({ ref: 'active-1', full_name: 'Активный Мастер', sales_status: 'active' }),
      specialist({ ref: 'archived-1', full_name: 'Архивный Мастер', sales_status: 'inactive' }),
    ])

    expect(wrapper.text()).toContain('Активный Мастер')
    expect(wrapper.text()).not.toContain('Архивный Мастер')
    expect(wrapper.findAll('[data-testid="specialist-row"]')).toHaveLength(1)
  })

  it('«Показать архивные» (В архиве) filter switches to archived-only', async () => {
    const { wrapper } = mountTab([
      specialist({ ref: 'active-1', full_name: 'Активный Мастер', sales_status: 'active' }),
      specialist({ ref: 'archived-1', full_name: 'Архивный Мастер', sales_status: 'inactive' }),
    ])

    await wrapper.find('[data-testid="specialists-filter-archived"]').trigger('click')

    expect(wrapper.text()).not.toContain('Активный Мастер')
    expect(wrapper.text()).toContain('Архивный Мастер')
    expect(wrapper.findAll('[data-testid="specialist-row"]')).toHaveLength(1)

    await wrapper.find('[data-testid="specialists-filter-active"]').trigger('click')
    expect(wrapper.text()).toContain('Активный Мастер')
    expect(wrapper.text()).not.toContain('Архивный Мастер')
  })

  it('shows the empty state when the current filter matches nothing', () => {
    const { wrapper } = mountTab([])
    expect(wrapper.text()).toContain('Специалистов пока нет.')
  })
})

describe('SpecialistsTab — archive/restore is an instant PATCH, no confirmation', () => {
  it('toggling the status switch calls PATCH /kb/specialists/:ref/status with no window.confirm', async () => {
    const { wrapper, pg } = mountTab([specialist({ ref: 'alina-kim', sales_status: 'active' })])
    const { api } = await import('@/api/client')
    vi.mocked(api.patch).mockResolvedValueOnce(specialist({ ref: 'alina-kim', sales_status: 'inactive' }))
    const nativeConfirm = vi.spyOn(window, 'confirm')

    const statusSwitch = wrapper.find('[data-testid="specialist-status-switch-alina-kim"]')
    expect(statusSwitch.exists()).toBe(true)
    await statusSwitch.trigger('click')
    await flushPromises()

    expect(nativeConfirm).not.toHaveBeenCalled()
    expect(api.patch).toHaveBeenCalledWith('/kb/specialists/alina-kim/status', { sales_status: 'inactive' })
    // setSpecialistStatus splices the row in place (see playground.ts's own
    // doc comment) — pg.live still holds it, just no longer active, so the
    // default Active filter drops it from view immediately, no reload.
    expect(pg.live?.specialists[0].sales_status).toBe('inactive')
    expect(wrapper.find('[data-testid="specialist-row"]').exists()).toBe(false)

    nativeConfirm.mockRestore()
  })

  it('restoring from the Archived filter calls PATCH with sales_status: "active"', async () => {
    const { wrapper } = mountTab([specialist({ ref: 'alina-kim', sales_status: 'inactive' })])
    const { api } = await import('@/api/client')
    vi.mocked(api.patch).mockResolvedValueOnce(specialist({ ref: 'alina-kim', sales_status: 'active' }))
    await wrapper.find('[data-testid="specialists-filter-archived"]').trigger('click')

    await wrapper.find('[data-testid="specialist-status-switch-alina-kim"]').trigger('click')
    await flushPromises()

    expect(api.patch).toHaveBeenCalledWith('/kb/specialists/alina-kim/status', { sales_status: 'active' })
  })

  it('a failed PATCH shows an inline error instead of throwing, and the row stays put', async () => {
    const { wrapper } = mountTab([specialist({ ref: 'alina-kim', sales_status: 'active' })])
    const { api, ApiError } = await import('@/api/client')
    vi.mocked(api.patch).mockRejectedValueOnce(new ApiError('SERVER_ERROR', 500, 'Не удалось сохранить изменение.'))

    await wrapper.find('[data-testid="specialist-status-switch-alina-kim"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Не удалось сохранить изменение.')
    expect(wrapper.findAll('[data-testid="specialist-row"]')).toHaveLength(1)
  })
})

describe('SpecialistsTab — roster columns', () => {
  it('renders the master, experience, booking link, and portfolio count', () => {
    const { wrapper } = mountTab([specialist()])
    expect(wrapper.text()).toContain('Алина Ким')
    expect(wrapper.text()).toContain('Топ-стилист / Колорист')
    expect(wrapper.text()).toContain('7 лет')
    expect(wrapper.text()).toContain('https://xpayment.kz/book/aura-alina')
    expect(wrapper.text()).toContain('2 фото')
  })

  it('shows the fallback booking badge when the specialist has no booking_url of their own', () => {
    const { wrapper } = mountTab([specialist({ booking_url: '' })])
    expect(wrapper.text()).toContain('Основная салона')
  })
})
