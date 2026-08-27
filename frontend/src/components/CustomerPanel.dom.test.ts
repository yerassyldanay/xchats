import { describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useInbox } from '@/stores/inbox'
import { useCrm } from '@/stores/crm'
import type { Chat, CustomerProfile } from '@/types'
import CustomerPanel from './CustomerPanel.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: { ...actual.api, get: vi.fn(), post: vi.fn(), patch: vi.fn(), del: vi.fn() },
  }
})

const CUSTOMER_ID = 'cust-1'

const profile: CustomerProfile = {
  customer: {
    id: CUSTOMER_ID,
    display_name: 'Иван Смирнов',
    phone: '+7 777 123 45 67',
    email: 'ivan@example.com',
    avatar_url: '',
    status_id: 'st-2',
    status: { id: 'st-2', slug: 'interested', name: 'Интересуется', color: '', position: 2, is_default: false },
    assignee_user_id: 'u-1',
    tags: [
      { id: 'tag-1', slug: 'vip', name: 'VIP', color: '' },
      { id: 'tag-2', slug: 'xpayment', name: 'xpayment', color: '' },
    ],
    identities: [
      {
        id: 'id-1',
        channel: 'whatsapp',
        account_id: 'acc-1',
        contact_id: 'ct-1',
        external_id: '77771234567@s.whatsapp.net',
        username: '',
        phone: '77771234567',
        display_name: 'Иван',
      },
      {
        id: 'id-2',
        channel: 'telegram',
        account_id: 'acc-2',
        contact_id: 'ct-2',
        external_id: '99887766',
        username: 'john',
        phone: '',
        display_name: 'Иван',
      },
    ],
    custom_fields: {},
    created_at: '2026-08-08T10:00:00Z',
    updated_at: '2026-08-08T10:00:00Z',
  },
  latest_note: {
    id: 'n-1',
    customer_id: CUSTOMER_ID,
    author_user_id: 'u-1',
    author_name: 'Alice',
    body: 'Клиент пока не готов интегрировать xpayment.',
    created_at: '2026-08-08T12:00:00Z',
    updated_at: '2026-08-08T12:00:00Z',
  },
  next_followup: {
    id: 'f-1',
    customer_id: CUSTOMER_ID,
    customer_name: 'Иван Смирнов',
    conversation_id: 'chat-1',
    channel: 'whatsapp',
    due_at: '2026-08-20T06:00:00Z',
    due_date: '2026-08-20',
    due_minute: 660,
    action: 'call',
    note: 'Позвонить про xpayment',
    assignee_user_id: 'u-1',
    assignee_name: 'Alice',
    state: 'open',
    completed_at: null,
    created_at: '2026-08-08T12:05:00Z',
  },
  conversations: [],
}

const chat: Chat = {
  id: 'chat-1',
  channel: 'whatsapp',
  account_id: 'acc-1',
  contact: {
    id: 'ct-1',
    display_name: 'Иван',
    phone_number: '77771234567',
    phone_jid: '77771234567@s.whatsapp.net',
    lid_jid: '',
    push_name: 'Иван',
  },
  status: 'open',
  assignee_user_id: null,
  unread_count: 0,
  last_message_at: '2026-08-08T12:00:00Z',
  last_message_preview: 'привет',
  customer_id: CUSTOMER_ID,
}

async function mountPanel() {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === `/customers/${CUSTOMER_ID}`) return profile as never
    if (path === `/customers/${CUSTOMER_ID}/timeline`) {
      return {
        items: [
          {
            id: 'tl-1',
            source: 'crm',
            kind: 'status_changed',
            actor_user_id: 'u-1',
            summary: 'Новый → Интересуется',
            occurred_at: '2026-08-08T12:10:00Z',
          },
        ],
      } as never
    }
    throw new Error(`unexpected GET ${path}`)
  })

  const pinia = testPinia()
  const inbox = useInbox()
  inbox.chats = [chat]
  inbox.activeId = chat.id
  inbox.users = [{ id: 'u-1', email: 'alice@example.com', name: 'Alice', role: 'member', must_change_password: false }]

  const crm = useCrm()
  crm.statuses = [
    { id: 'st-1', slug: 'new', name: 'Новый', color: '', position: 1, is_default: true },
    { id: 'st-2', slug: 'interested', name: 'Интересуется', color: '', position: 2, is_default: false },
  ]
  crm.tags = [
    { id: 'tag-1', slug: 'vip', name: 'VIP', color: '' },
    { id: 'tag-2', slug: 'xpayment', name: 'xpayment', color: '' },
    { id: 'tag-3', slug: 'notready', name: 'Не готов', color: '' },
  ]
  crm.catalogsLoaded = true

  const wrapper = mountKb(CustomerPanel, { pinia })
  await flushPromises()
  return { wrapper, crm, api }
}

describe('CustomerPanel', () => {
  it('renders the profile the product brief specifies for an open conversation', async () => {
    const { wrapper } = await mountPanel()
    const text = wrapper.text()

    // Name, both channel identities, status, tags, note and next step — the
    // whole "who is this / what happened / what is next" set, on one screen.
    expect(text).toContain('VIP')
    expect(text).toContain('xpayment')
    expect(text).toContain('Клиент пока не готов интегрировать xpayment.')
    expect(text).toContain('Alice')
    expect(text).toContain('2026-08-20')
    expect(text).toContain('11:00')
    // Identity handles: the WhatsApp number without its JID suffix, the
    // Telegram @username.
    expect(text).toContain('77771234567')
    expect(text).toContain('@john')
    expect(text).not.toContain('@s.whatsapp.net')
    // The name is an editable field, not static text.
    const nameInput = wrapper.findAll('input').find((i) => i.element.value === 'Иван Смирнов')
    expect(nameInput).toBeDefined()
  })

  it('saves a note without leaving the conversation', async () => {
    const { wrapper, api } = await mountPanel()
    vi.mocked(api.post).mockResolvedValue({
      id: 'n-2',
      customer_id: CUSTOMER_ID,
      author_user_id: 'u-1',
      author_name: 'Alice',
      body: 'Перезвонить в понедельник',
      created_at: '2026-08-08T13:00:00Z',
      updated_at: '2026-08-08T13:00:00Z',
    } as never)

    const textarea = wrapper.find('textarea')
    await textarea.setValue('Перезвонить в понедельник')
    const addButton = wrapper.findAll('button').find((b) => b.text().includes('Добавить заметку'))
    expect(addButton).toBeDefined()
    await addButton!.trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith(`/customers/${CUSTOMER_ID}/notes`, {
      body: 'Перезвонить в понедельник',
    })
  })

  it('removes a tag through the API rather than only in local state', async () => {
    const { wrapper, api } = await mountPanel()
    vi.mocked(api.del).mockResolvedValue({ ...profile.customer, tags: [profile.customer.tags[1]] } as never)

    const removeVip = wrapper.findAll('button').find((b) => b.attributes('aria-label') === 'VIP')
    expect(removeVip).toBeDefined()
    await removeVip!.trigger('click')
    await flushPromises()

    expect(api.del).toHaveBeenCalledWith(`/customers/${CUSTOMER_ID}/tags/tag-1`)
  })

  it('shows the empty state when the conversation has no customer', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockReset()
    const pinia = testPinia()
    const inbox = useInbox()
    inbox.chats = [{ ...chat, customer_id: null }]
    inbox.activeId = chat.id

    const wrapper = mountKb(CustomerPanel, { pinia })
    await flushPromises()

    expect(wrapper.text()).toContain('У этого чата пока нет карточки клиента.')
    expect(api.get).not.toHaveBeenCalled()
  })
})
