import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { testPinia } from '@/test/mount'
import { useCrm } from './crm'
import type { CustomerProfile } from '@/types'

// The sidebar's customerId watcher (CustomerPanel.vue) fires loadProfile for
// every chat the operator clicks through, even in quick succession — this
// exercises the same "latest request wins" guard at the store level, without
// mounting the component.
vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: { ...actual.api, get: vi.fn() },
  }
})

beforeEach(() => {
  vi.clearAllMocks()
})

function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

function profileFor(customerId: string): CustomerProfile {
  return {
    customer: {
      id: customerId,
      display_name: `Customer ${customerId}`,
      phone: '',
      email: '',
      avatar_url: '',
      status_id: '',
      status: null,
      assignee_user_id: null,
      tags: [],
      identities: [],
      custom_fields: {},
      created_at: '2026-08-30T00:00:00Z',
      updated_at: '2026-08-30T00:00:00Z',
    },
    latest_note: null,
    next_followup: null,
    conversations: [],
  }
}

describe('crm store — customer profile watcher race (INB-08)', () => {
  it('never lets a slow profile fetch for a previous customer overwrite the one now selected', async () => {
    testPinia()
    const { api } = await import('@/api/client')
    const crm = useCrm()

    const slow = deferred<CustomerProfile>() // customer A — the one the operator clicked first
    const fast = profileFor('B') // customer B — where they actually ended up

    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path === '/customers/A') return slow.promise as never
      if (path === '/customers/B') return fast as never
      throw new Error('unexpected GET ' + path)
    })

    void crm.loadProfile('A')
    await flushPromises()
    void crm.loadProfile('B')
    await flushPromises()

    expect(crm.profile?.customer.id).toBe('B')
    expect(crm.loadingProfile).toBe(false)

    slow.resolve(profileFor('A'))
    await flushPromises()

    expect(crm.profile?.customer.id).toBe('B') // A's stale response never applied
  })
})
