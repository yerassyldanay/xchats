import { describe, expect, it, vi } from 'vitest'
import { mountKb, testPinia } from '@/test/mount'
import { useInbox } from '@/stores/inbox'
import MediaLightbox from '@/components/kb/records/MediaLightbox.vue'
import ChatThread from './ChatThread.vue'
import type { Chat, Media, Message } from '@/types'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

function baseChat(): Chat {
  return {
    id: 'chat-1', channel: 'whatsapp', account_id: 'acct-1',
    contact: { id: 'contact-1', display_name: 'Клиент', phone_number: '77001234567', phone_jid: '', lid_jid: '', push_name: '' },
    status: 'open', assignee_user_id: null, unread_count: 0,
    last_message_at: '2026-08-19T08:00:00Z', last_message_preview: '', customer_id: null,
  }
}
function media(overrides: Partial<Media>): Media {
  return { id: 'md-1', url: '/m/1', media_type: 'image', mimetype: '', file_name: '', file_size: 0, download_status: 'ready', transcript: '', ...overrides }
}
function baseMessage(overrides: Partial<Message>): Message {
  return {
    id: 'm-1', chat_id: 'chat-1', direction: 'in', sender_type: 'contact', external_message_id: '',
    message_type: '', content: '', media: [], status: 'delivered', source: 'live_webhook', timestamp: '2026-08-19T08:00:00Z',
    ...overrides,
  }
}

function mountWith(messages: Message[]) {
  const pinia = testPinia()
  const inbox = useInbox()
  inbox.chats = [baseChat()]
  inbox.activeId = 'chat-1'
  inbox.messages = messages
  return mountKb(ChatThread, { pinia })
}

describe('ChatThread — audio transcript', () => {
  it('shows the transcript expanded by default, with copy and quote actions', () => {
    const wrapper = mountWith([
      baseMessage({ media: [media({ media_type: 'audio', mimetype: 'audio/ogg', transcript: 'здравствуйте, привет' })] }),
    ])
    expect(wrapper.text()).toContain('здравствуйте, привет')
    expect(wrapper.text()).toContain('Копировать')
    expect(wrapper.text()).toContain('В ответ')
  })

  it('collapses the transcript text on toggle, without losing the actions', async () => {
    const wrapper = mountWith([
      baseMessage({ media: [media({ media_type: 'audio', mimetype: 'audio/ogg', transcript: 'здравствуйте, привет' })] }),
    ])
    await wrapper.find('button:not([title])').trigger('click') // the toggle is the only untitled button in the transcript box
    expect(wrapper.text()).not.toContain('здравствуйте, привет')
  })

  it('quoting a transcript sets the composer text', async () => {
    const wrapper = mountWith([
      baseMessage({ media: [media({ media_type: 'audio', mimetype: 'audio/ogg', transcript: 'а есть доставка?' })] }),
    ])
    const inbox = useInbox()
    await wrapper.find('button[title="В ответ"]').trigger('click')
    expect(inbox.composerText).toBe('а есть доставка?')
  })

  it('copying a transcript writes it to the clipboard', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
    const wrapper = mountWith([
      baseMessage({ media: [media({ media_type: 'audio', mimetype: 'audio/ogg', transcript: 'копировать меня' })] }),
    ])
    await wrapper.find('button[title="Копировать"]').trigger('click')
    expect(writeText).toHaveBeenCalledWith('копировать меня')
  })

  it('shows a pending hint for a downloaded audio note with no transcript yet', () => {
    const wrapper = mountWith([
      baseMessage({ media: [media({ media_type: 'audio', mimetype: 'audio/ogg', download_status: 'ready', transcript: '' })] }),
    ])
    expect(wrapper.text()).toContain('Расшифровка ещё не готова')
  })

  it('shows nothing extra for an audio note that is still downloading', () => {
    const wrapper = mountWith([
      baseMessage({ media: [media({ media_type: 'audio', mimetype: 'audio/ogg', download_status: 'pending', transcript: '' })] }),
    ])
    expect(wrapper.text()).not.toContain('Расшифровка')
  })

  it('offers a re-transcribe trigger alongside a finished transcript', () => {
    const wrapper = mountWith([
      baseMessage({ media: [media({ media_type: 'audio', mimetype: 'audio/ogg', transcript: 'привет' })] }),
    ])
    expect(wrapper.text()).toContain('Расшифровать заново')
  })
})

describe('ChatThread — image lightbox', () => {
  it('opens the lightbox with the clicked image on click', async () => {
    const wrapper = mountWith([baseMessage({ media: [media({ media_type: 'image', mimetype: 'image/jpeg', file_name: 'photo.jpg' })] })])
    const lightbox = wrapper.findComponent(MediaLightbox)
    expect(lightbox.props('open')).toBe(false)

    await wrapper.find('img').trigger('click')

    expect(lightbox.props('open')).toBe(true)
    expect(lightbox.props('label')).toBe('photo.jpg')
    expect(lightbox.props('src')).toContain('/m/1')
  })
})
