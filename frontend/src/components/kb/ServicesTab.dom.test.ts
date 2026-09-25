import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, type VueWrapper } from '@vue/test-utils'
import { usePlayground } from '@/stores/playground'
import { mountKb, testPinia } from '@/test/mount'
import ServicesTab from './ServicesTab.vue'
import type { DraftView, ServiceRow, SpecialistRow } from '@/types'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), patch: vi.fn(), del: vi.fn() } }
})

// useKbModal's session is a module-level singleton (see SpecialistsTab.dom.
// test.ts's own note) — unmount after every test.
let mounted: VueWrapper | undefined
afterEach(() => {
  mounted?.unmount()
  mounted = undefined
})

function service(over: Partial<ServiceRow> = {}): ServiceRow {
  return {
    id: 'haircut-women', ref: 'haircut-women', parent_ref: '', service_type: 'base',
    category: 'Волосы', name: 'Женская стрижка', price: '10 000 ₸', duration: 60,
    description: '', specialist_refs: [], sales_status: 'active',
    draft: false, updated_at: '',
    ...over,
  }
}

function specialist(over: Partial<SpecialistRow> = {}): SpecialistRow {
  return {
    id: 'alina-kim', ref: 'alina-kim', full_name: 'Алина Ким', title: '', experience: '',
    schedule: [], booking_url: '', portfolio_images: [], sales_status: 'active',
    draft: false, updated_at: '',
    ...over,
  }
}

function emptyLive(services: ServiceRow[], specialists: SpecialistRow[] = []): DraftView {
  return {
    config: {
      organization_id: 'org-1', persona: '', mission: '', guardrails: '', language_policy: '',
      reply_max_words: 120, draft: false, base_version: 0, updated_at: '',
    },
    topics: [], tariffs: [], products: [], specialists, services,
    contacts: [], policies: [], tariff_info: [], zones: [], materials: [], requests: [],
  }
}

function mountTab(services: ServiceRow[], specialists: SpecialistRow[] = []) {
  const pinia = testPinia()
  const pg = usePlayground()
  pg.live = emptyLive(services, specialists)
  const wrapper = mountKb(ServicesTab, { pinia })
  mounted = wrapper
  return { wrapper, pg }
}

describe('ServicesTab — tree nesting', () => {
  it('groups bases by category and nests variant/addon children under their base', () => {
    const { wrapper } = mountTab([
      service({ ref: 'haircut-women', service_type: 'base', category: 'Волосы', name: 'Женская стрижка' }),
      service({ ref: 'haircut-short', parent_ref: 'haircut-women', service_type: 'variant', category: 'Волосы', name: 'Короткая стрижка' }),
      service({ ref: 'hair-spa-mask', parent_ref: 'haircut-women', service_type: 'addon', category: 'Волосы', name: 'Спа-уход' }),
      service({ ref: 'manicure-gel', parent_ref: '', service_type: 'base', category: 'Ногти', name: 'Маникюр' }),
    ])

    const groups = wrapper.findAll('[data-testid="service-category-group"]')
    expect(groups).toHaveLength(2)
    expect(wrapper.findAll('[data-testid="service-base-row"]')).toHaveLength(2)
    expect(wrapper.findAll('[data-testid="service-child-row"]')).toHaveLength(2)
    expect(wrapper.text()).toContain('Женская стрижка')
    expect(wrapper.text()).toContain('Короткая стрижка')
    expect(wrapper.text()).toContain('Спа-уход')
  })

  it('an addon carries the "не продается отдельно" badge, a plain variant does not', () => {
    const { wrapper } = mountTab([
      service({ ref: 'haircut-women', service_type: 'base' }),
      service({ ref: 'haircut-short', parent_ref: 'haircut-women', service_type: 'variant', name: 'Короткая' }),
      service({ ref: 'hair-spa-mask', parent_ref: 'haircut-women', service_type: 'addon', name: 'Спа-уход' }),
    ])
    const rows = wrapper.findAll('[data-testid="service-child-row"]')
    const variantRow = rows.find((r) => r.text().includes('Короткая'))!
    const addonRow = rows.find((r) => r.text().includes('Спа-уход'))!
    expect(variantRow.text()).not.toContain('Не продается отдельно')
    expect(addonRow.text()).toContain('Не продается отдельно')
  })

  it('resolves a specialist_ref pill to the specialist\'s display name', () => {
    const { wrapper } = mountTab(
      [service({ ref: 'haircut-women', specialist_refs: ['alina-kim'] })],
      [specialist({ ref: 'alina-kim', full_name: 'Алина Ким' })]
    )
    expect(wrapper.find('[data-testid="service-base-row"]').text()).toContain('Алина Ким')
  })
})

describe('ServicesTab — tree branch connectors', () => {
  it('marks every child but the last with a ├─ connector, and the last with └─', () => {
    const { wrapper } = mountTab([
      service({ ref: 'haircut-women', service_type: 'base' }),
      service({ ref: 'haircut-short', parent_ref: 'haircut-women', service_type: 'variant', name: 'Короткая' }),
      service({ ref: 'hair-spa-mask', parent_ref: 'haircut-women', service_type: 'addon', name: 'Спа-уход' }),
    ])
    const rows = wrapper.findAll('[data-testid="service-child-row"]')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toMatch(/^├─/)
    expect(rows[1].text()).toMatch(/^└─/)
  })

  it('a single child is its own last child — gets └─, not ├─', () => {
    const { wrapper } = mountTab([
      service({ ref: 'haircut-women', service_type: 'base' }),
      service({ ref: 'haircut-short', parent_ref: 'haircut-women', service_type: 'variant', name: 'Короткая' }),
    ])
    expect(wrapper.find('[data-testid="service-child-row"]').text()).toMatch(/^└─/)
  })
})

describe('ServicesTab — category count chip', () => {
  it('counts every base plus its children, not just base rows', () => {
    const { wrapper } = mountTab([
      service({ ref: 'haircut-women', service_type: 'base', category: 'Волосы' }),
      service({ ref: 'haircut-short', parent_ref: 'haircut-women', service_type: 'variant', category: 'Волосы' }),
      service({ ref: 'hair-spa-mask', parent_ref: 'haircut-women', service_type: 'addon', category: 'Волосы' }),
      service({ ref: 'coloring-airtouch', parent_ref: '', service_type: 'base', category: 'Волосы', name: 'Airtouch' }),
    ])
    const chip = wrapper.find('[data-testid="service-category-count"]')
    expect(chip.exists()).toBe(true)
    expect(chip.text()).toContain('4')
  })
})

describe('ServicesTab — active/archived filtering', () => {
  it('defaults to active only, and an archived base\'s children disappear with it', () => {
    const { wrapper } = mountTab([
      service({ ref: 'haircut-women', service_type: 'base', sales_status: 'inactive' }),
      service({ ref: 'haircut-short', parent_ref: 'haircut-women', service_type: 'variant', sales_status: 'active' }),
    ])
    expect(wrapper.findAll('[data-testid="service-base-row"]')).toHaveLength(0)
    expect(wrapper.findAll('[data-testid="service-child-row"]')).toHaveLength(0)
  })

  it('«В архиве» filter shows archived rows', async () => {
    const { wrapper } = mountTab([service({ ref: 'haircut-women', sales_status: 'inactive' })])
    await wrapper.find('[data-testid="services-filter-archived"]').trigger('click')
    expect(wrapper.findAll('[data-testid="service-base-row"]')).toHaveLength(1)
  })
})

describe('ServicesTab — archive/restore is an instant PATCH (unchanged, no undo window here)', () => {
  it('toggling the status switch calls PATCH /kb/services/:ref/status immediately', async () => {
    const { wrapper, pg } = mountTab([service({ ref: 'haircut-women', sales_status: 'active' })])
    const { api } = await import('@/api/client')
    vi.mocked(api.patch).mockResolvedValueOnce(service({ ref: 'haircut-women', sales_status: 'inactive' }))

    await wrapper.find('[data-testid="service-status-switch-haircut-women"]').trigger('click')
    await flushPromises()

    expect(api.patch).toHaveBeenCalledWith('/kb/services/haircut-women/status', { sales_status: 'inactive' })
    expect(pg.live?.services[0].sales_status).toBe('inactive')
  })
})
