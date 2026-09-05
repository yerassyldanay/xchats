import { describe, expect, it } from 'vitest'
import { sourceOf } from './draftSource'
import type { AiDraft, Media, Message } from '@/types'

function media(overrides: Partial<Media>): Media {
  return { id: 'md-1', url: '/m/1', media_type: 'image', mimetype: '', file_name: '', file_size: 0, download_status: 'ready', transcript: '', ...overrides }
}
function message(overrides: Partial<Message>): Message {
  return {
    id: 'm-1', chat_id: 'chat-1', direction: 'in', sender_type: 'contact', external_message_id: '',
    message_type: '', content: '', media: [], status: 'delivered', source: 'live_webhook', timestamp: null,
    ...overrides,
  }
}
function draft(overrides: Partial<AiDraft>): AiDraft {
  return {
    id: 'd-1', chat_id: 'chat-1', trigger_message_id: 'm-1', ordinal: 1, draft_text: 'x',
    context_status: '', confidence: null, escalate: false, escalation_reason: '', status: 'suggested', created_at: '',
    ...overrides,
  }
}

describe('sourceOf', () => {
  it('returns "audio" when the trigger message has an audio attachment', () => {
    const messages = [message({ media: [media({ media_type: 'audio' })] })]
    expect(sourceOf(draft({}), messages)).toBe('audio')
  })

  it('returns "image" when the trigger message has an image attachment', () => {
    const messages = [message({ media: [media({ media_type: 'image' })] })]
    expect(sourceOf(draft({}), messages)).toBe('image')
  })

  it('prefers "audio" over "image" when a message somehow carries both', () => {
    const messages = [message({ media: [media({ media_type: 'image' }), media({ id: 'md-2', media_type: 'audio' })] })]
    expect(sourceOf(draft({}), messages)).toBe('audio')
  })

  it('returns null for a text-only trigger message', () => {
    const messages = [message({ content: 'Привет' })]
    expect(sourceOf(draft({}), messages)).toBeNull()
  })

  it('returns null for a document attachment (neither audio nor image)', () => {
    const messages = [message({ media: [media({ media_type: 'document' })] })]
    expect(sourceOf(draft({}), messages)).toBeNull()
  })

  it('returns null when the draft has no trigger_message_id', () => {
    const messages = [message({ media: [media({ media_type: 'audio' })] })]
    expect(sourceOf(draft({ trigger_message_id: null }), messages)).toBeNull()
  })

  it('returns null when the trigger message is not (yet) loaded', () => {
    expect(sourceOf(draft({ trigger_message_id: 'not-loaded' }), [])).toBeNull()
  })
})
