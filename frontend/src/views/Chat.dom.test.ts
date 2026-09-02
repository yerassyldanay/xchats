import { describe, expect, it, vi } from 'vitest'
import { mountKb, testPinia } from '@/test/mount'
import { useChat } from '@/stores/chat'
import Chat from './Chat.vue'
import type { ChatMessage } from '@/types'

// The store's network calls are stubbed out: this file is about what the page
// renders for a given store state, not about how that state is fetched
// (stores/chat.test.ts covers that).
vi.mock('@/api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/client')>()),
  api: { get: vi.fn(async () => ({ conversations: [] })), post: vi.fn(), patch: vi.fn(), del: vi.fn() },
}))

function message(overrides: Partial<ChatMessage>): ChatMessage {
  return {
    id: 'm1',
    role: 'assistant',
    content: '',
    components: [],
    metadata: {},
    created_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function mountChat() {
  const pinia = testPinia()
  const chat = useChat()
  const wrapper = mountKb(Chat, { pinia })
  return { wrapper, chat }
}

describe('Chat view', () => {
  it('offers starter prompts when there is nothing to show yet', async () => {
    const { wrapper } = mountChat()
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Ассистент по базе знаний')
    // The empty state exists to say what this assistant is FOR — comparing
    // the live knowledge base against the draft.
    expect(text).toContain('Сравни текущие и черновиковые цены')
  })

  it('renders an assistant answer as Markdown', async () => {
    const { wrapper, chat } = mountChat()
    chat.messages = [message({ role: 'user', content: 'цена?' }), message({ id: 'a1', content: '**12 000 ₸**' })]
    await wrapper.vm.$nextTick()
    expect(wrapper.html()).toContain('<strong>12 000 ₸</strong>')
  })

  // The renderer runs with raw HTML disabled, so markup in an answer is shown
  // as text rather than mounted into the page.
  it('escapes HTML in an answer instead of rendering it', async () => {
    const { wrapper, chat } = mountChat()
    chat.messages = [message({ id: 'a1', content: 'careful: <img src=x onerror="alert(1)">' })]
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    expect(html).not.toContain('<img')
    expect(html).toContain('&lt;img')
  })

  it('renders a comparison card beneath the answer it belongs to', async () => {
    const { wrapper, chat } = mountChat()
    chat.messages = [
      message({
        id: 'a1',
        content: 'The draft price is lower.',
        components: [
          {
            type: 'kb_comparison',
            data: {
              kind: 'products',
              key: 'vitamin-d',
              title: 'Vitamin D',
              change: 'updated',
              real: null,
              draft: null,
              fields: [{ key: 'price', label: 'Price', real: '12 000 ₸', draft: '10 800 ₸' }],
            },
          },
        ],
      }),
    ]
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Vitamin D')
    expect(text).toContain('12 000 ₸')
    expect(text).toContain('10 800 ₸')
    expect(text).toContain('−1 200 ₸')
  })

  it('shows the error banner when a turn failed', async () => {
    const { wrapper, chat } = mountChat()
    chat.messages = [message({ role: 'user', content: 'цена?' })]
    chat.error = 'the AI provider rejected the configured API key — check Settings'
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[role="alert"]').text()).toContain('check Settings')
  })

  it('swaps the send button for a stop button while an answer is streaming', async () => {
    const { wrapper, chat } = mountChat()
    chat.messages = [message({ role: 'user', content: 'цена?' })]
    await wrapper.vm.$nextTick()
    expect(wrapper.find('button[aria-label="Остановить"]').exists()).toBe(false)

    chat.sending = true
    await wrapper.vm.$nextTick()
    expect(wrapper.find('button[aria-label="Остановить"]').exists()).toBe(true)
  })
})
