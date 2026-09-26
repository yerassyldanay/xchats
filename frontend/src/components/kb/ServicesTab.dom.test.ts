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

// reka-ui's Dialog (ConfirmDeleteDialog's own building block) renders
// through a Teleport into document.body, outside @vue/test-utils' wrapper
// subtree — same reasoning/pattern as DraftKnowledgeBase.dom.test.ts's own
// openDialogAccept(). Earlier mounts in this file are never explicitly
// cleared from document.body, so always take the LAST match.
function lastInBody(testid: string): HTMLElement | null {
  const all = document.body.querySelectorAll(`[data-testid="${testid}"]`)
  return (all[all.length - 1] as HTMLElement | undefined) ?? null
}

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

  it('an archived child under a still-ACTIVE base stays reachable in the Archived view, base shown for context', async () => {
    // Regression test: the tree used to only nest a child under a base row
    // that ALSO passed the current filter, so an archived variant/addon
    // whose base stayed active had no base row to nest under in EITHER
    // view — invisible and unrestorable. Only a base's own archival
    // cascades to children; archiving one child alone is a valid,
    // independent action (PLAN.md), so this state is real, not corrupted
    // data.
    const { wrapper } = mountTab([
      service({ ref: 'haircut-women', service_type: 'base', sales_status: 'active' }),
      service({ ref: 'haircut-short', parent_ref: 'haircut-women', service_type: 'variant', name: 'Короткая', sales_status: 'inactive' }),
    ])
    await wrapper.find('[data-testid="services-filter-archived"]').trigger('click')

    expect(wrapper.findAll('[data-testid="service-base-row"]')).toHaveLength(1)
    expect(wrapper.findAll('[data-testid="service-child-row"]')).toHaveLength(1)
    expect(wrapper.text()).toContain('Короткая')
    expect(wrapper.find('[data-testid="service-base-context-badge"]').exists()).toBe(true)

    // The child's own switch/edit are fully functional — it isn't a ghost row.
    const { api } = await import('@/api/client')
    vi.mocked(api.patch).mockResolvedValueOnce(service({ ref: 'haircut-short', parent_ref: 'haircut-women', service_type: 'variant', sales_status: 'active' }))
    await wrapper.find('[data-testid="service-status-switch-haircut-short"]').trigger('click')
    await flushPromises()
    expect(api.patch).toHaveBeenCalledWith('/kb/services/haircut-short/status', { sales_status: 'active' })
  })

  it('the Active view is unaffected by the Archived-only context fallback (an active child always has an active base already)', () => {
    const { wrapper } = mountTab([
      service({ ref: 'haircut-women', service_type: 'base', sales_status: 'inactive' }),
      service({ ref: 'haircut-short', parent_ref: 'haircut-women', service_type: 'variant', sales_status: 'active' }),
    ])
    // Hand-built state validateService (backend) never actually allows —
    // the Active view must still hide it exactly as before, not extend the
    // context-only fallback to a direction it was never meant to cover.
    expect(wrapper.findAll('[data-testid="service-base-row"]')).toHaveLength(0)
    expect(wrapper.findAll('[data-testid="service-child-row"]')).toHaveLength(0)
  })
})

describe('ServicesTab — archiving a base with active children warns before cascading', () => {
  it('archiving a base WITH active children opens a confirmation dialog instead of PATCHing immediately', async () => {
    const { wrapper, pg } = mountTab([
      service({ ref: 'haircut-women', service_type: 'base', name: 'Женская стрижка', sales_status: 'active' }),
      service({ ref: 'haircut-short', parent_ref: 'haircut-women', service_type: 'variant', sales_status: 'active' }),
      service({ ref: 'hair-spa-mask', parent_ref: 'haircut-women', service_type: 'addon', sales_status: 'active' }),
    ])
    const { api } = await import('@/api/client')
    vi.mocked(api.patch).mockResolvedValueOnce(service({ ref: 'haircut-women', service_type: 'base', name: 'Женская стрижка', sales_status: 'inactive' }))

    await wrapper.find('[data-testid="service-status-switch-haircut-women"]').trigger('click')
    await flushPromises()

    expect(api.patch).not.toHaveBeenCalled()
    expect(pg.live?.services[0].sales_status).toBe('active')
    const dialogBody = lastInBody('confirm-body')
    expect(dialogBody).toBeTruthy()
    expect(dialogBody!.textContent).toContain('Женская стрижка')
    expect(dialogBody!.textContent).toContain('2')

    lastInBody('confirm-accept')!.click()
    await flushPromises()
    expect(api.patch).toHaveBeenCalledWith('/kb/services/haircut-women/status', { sales_status: 'inactive' })
  })

  it('cancelling the dialog leaves the base untouched, no PATCH', async () => {
    const { wrapper, pg } = mountTab([
      service({ ref: 'haircut-women', service_type: 'base', sales_status: 'active' }),
      service({ ref: 'haircut-short', parent_ref: 'haircut-women', service_type: 'variant', sales_status: 'active' }),
    ])
    const { api } = await import('@/api/client')

    await wrapper.find('[data-testid="service-status-switch-haircut-women"]').trigger('click')
    expect(lastInBody('confirm-body')).toBeTruthy()

    const cancelBtn = [...document.body.querySelectorAll('button')].filter((b) => b.textContent === 'Отмена').pop()
    cancelBtn!.click()
    await flushPromises()

    expect(api.patch).not.toHaveBeenCalled()
    expect(pg.live?.services[0].sales_status).toBe('active')
  })

  it('archiving a base with NO active children PATCHes immediately, no dialog (children already archived)', async () => {
    const { wrapper } = mountTab([
      service({ ref: 'haircut-women', service_type: 'base', sales_status: 'active' }),
      service({ ref: 'haircut-short', parent_ref: 'haircut-women', service_type: 'variant', sales_status: 'inactive' }),
    ])
    const { api } = await import('@/api/client')
    vi.mocked(api.patch).mockResolvedValueOnce(service({ ref: 'haircut-women', service_type: 'base', sales_status: 'inactive' }))
    const before = document.body.querySelectorAll('[data-testid="confirm-body"]').length

    await wrapper.find('[data-testid="service-status-switch-haircut-women"]').trigger('click')
    await flushPromises()

    expect(api.patch).toHaveBeenCalledWith('/kb/services/haircut-women/status', { sales_status: 'inactive' })
    expect(document.body.querySelectorAll('[data-testid="confirm-body"]').length).toBe(before)
  })

  it('restoring an archived base never shows the cascade dialog, even with children', async () => {
    const { wrapper } = mountTab([service({ ref: 'haircut-women', service_type: 'base', sales_status: 'inactive' })])
    const { api } = await import('@/api/client')
    vi.mocked(api.patch).mockResolvedValueOnce(service({ ref: 'haircut-women', service_type: 'base', sales_status: 'active' }))
    await wrapper.find('[data-testid="services-filter-archived"]').trigger('click')
    const before = document.body.querySelectorAll('[data-testid="confirm-body"]').length

    await wrapper.find('[data-testid="service-status-switch-haircut-women"]').trigger('click')
    await flushPromises()

    expect(api.patch).toHaveBeenCalledWith('/kb/services/haircut-women/status', { sales_status: 'active' })
    expect(document.body.querySelectorAll('[data-testid="confirm-body"]').length).toBe(before)
  })

  it('toggling a variant/addon (not a base) never shows the cascade dialog', async () => {
    const { wrapper } = mountTab([
      service({ ref: 'haircut-women', service_type: 'base', sales_status: 'active' }),
      service({ ref: 'haircut-short', parent_ref: 'haircut-women', service_type: 'variant', sales_status: 'active' }),
    ])
    const { api } = await import('@/api/client')
    vi.mocked(api.patch).mockResolvedValueOnce(service({ ref: 'haircut-short', parent_ref: 'haircut-women', service_type: 'variant', sales_status: 'inactive' }))
    const before = document.body.querySelectorAll('[data-testid="confirm-body"]').length

    await wrapper.find('[data-testid="service-status-switch-haircut-short"]').trigger('click')
    await flushPromises()

    expect(api.patch).toHaveBeenCalledWith('/kb/services/haircut-short/status', { sales_status: 'inactive' })
    expect(document.body.querySelectorAll('[data-testid="confirm-body"]').length).toBe(before)
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
