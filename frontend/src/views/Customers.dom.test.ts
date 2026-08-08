import { afterEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises, type VueWrapper } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useCrm } from '@/stores/crm'
import type { Customer } from '@/types'
import Customers from './Customers.vue'

// The view navigates to the chat screen when a customer is opened. A full
// vue-router instance is more machinery than a component-level mount needs, so
// useRouter is stubbed — mountKb already stubs RouterLink for the same reason.
const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: { ...actual.api, get: vi.fn(), post: vi.fn(), patch: vi.fn(), del: vi.fn() },
  }
})

function customer(id: string, name: string, channel: 'whatsapp' | 'telegram'): Customer {
  return {
    id,
    display_name: name,
    phone: '',
    email: '',
    avatar_url: '',
    status_id: null,
    status: null,
    assignee_user_id: null,
    tags: [],
    identities: [
      {
        id: 'i-' + id,
        channel,
        account_id: 'acc-1',
        contact_id: 'ct-' + id,
        external_id: channel === 'telegram' ? '99887766' : '77771234567@s.whatsapp.net',
        username: channel === 'telegram' ? 'john' : '',
        phone: channel === 'telegram' ? '' : '77771234567',
        display_name: name,
      },
    ],
    custom_fields: {},
    created_at: '2026-08-08T10:00:00Z',
    updated_at: '2026-08-08T10:00:00Z',
  }
}

const A = customer('c-1', 'Иван Смирнов', 'whatsapp')
const B = customer('c-2', 'Иван', 'telegram')

// The merge confirmation renders inside reka-ui's Dialog, which teleports its
// content to document.body — outside @vue/test-utils' own subtree. Mirrors
// AutomationSettingsDialog.dom.test.ts's body().
function body() {
  return new DOMWrapper(document.body)
}

// Teleported dialog content is not removed automatically between tests, so an
// explicit unmount keeps one test's buttons out of the next one's queries.
let mounted: VueWrapper<any> | undefined
afterEach(() => {
  mounted?.unmount()
  mounted = undefined
})

async function mountList() {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path.startsWith('/customers')) {
      return { items: [A, B], page: 1, page_size: 100, total: 2 } as never
    }
    if (path === '/crm/tags' || path === '/crm/statuses' || path === '/crm/custom-fields') {
      return { items: [] } as never
    }
    if (path.startsWith('/users')) return { items: [] } as never
    throw new Error(`unexpected GET ${path}`)
  })

  const pinia = testPinia()
  const wrapper = mountKb(Customers, { pinia })
  mounted = wrapper
  await flushPromises()
  return { wrapper, crm: useCrm(), api }
}

describe('Customers', () => {
  it('lists customers across channels', async () => {
    const { wrapper } = await mountList()
    const text = wrapper.text()
    expect(text).toContain('Иван Смирнов')
    expect(text).toContain('Всего: 2')
  })

  it('sends the search query and the browser timezone offset to the API', async () => {
    const { wrapper, crm, api } = await mountList()
    vi.mocked(api.get).mockClear()

    crm.query = 'али'
    await crm.loadCustomers()

    const called = vi.mocked(api.get).mock.calls.map((c) => String(c[0]))
    expect(called.some((p) => p.includes('q=') && p.includes('tz_offset_minutes='))).toBe(true)
    expect(wrapper.exists()).toBe(true)
  })

  it('merges only on an explicit two-customer selection, never automatically', async () => {
    const { wrapper, api } = await mountList()

    // Nothing selected: no merge affordance at all.
    expect(wrapper.findAll('button').some((b) => b.text().includes('Объединить'))).toBe(false)

    const boxes = wrapper.findAll('input[type="checkbox"]')
    expect(boxes.length).toBe(2)
    await boxes[0].trigger('change')
    await flushPromises()
    // One selected is still not enough — a merge always takes two.
    expect(wrapper.findAll('button').some((b) => b.text().includes('Объединить'))).toBe(false)

    await boxes[1].trigger('change')
    await flushPromises()
    const mergeBtn = wrapper.findAll('button').find((b) => b.text().includes('Объединить'))
    expect(mergeBtn).toBeDefined()

    vi.mocked(api.post).mockResolvedValue({ ...A } as never)
    await mergeBtn!.trigger('click')
    await flushPromises()

    // The confirmation lives in the teleported dialog, not in the page subtree.
    // The last match is the dialog's own confirm button; the earlier one is the
    // header action that opened it.
    const candidates = body().findAll('button').filter((b) => b.text().includes('Объединить'))
    const confirm = candidates[candidates.length - 1]
    expect(confirm).toBeDefined()
    await confirm!.trigger('click')
    await flushPromises()

    // The FIRST selected profile survives; the second is folded into it.
    expect(api.post).toHaveBeenCalledWith(`/customers/${A.id}/merge`, {
      source_customer_id: B.id,
    })
  })
})
