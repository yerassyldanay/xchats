import type { AiDraft, Message } from '@/types'

// sourceOf derives an AI draft's "based on ..." badge (AssistantPanel.vue)
// purely from data already loaded client-side — the trigger message's own
// media — rather than a dedicated backend field: a draft has no stored
// notion of "what kind of attachment prompted it," but its
// trigger_message_id plus that message's media array (an audio transcript
// or an image) already say the same thing.
export function sourceOf(draft: AiDraft, messages: Message[]): 'audio' | 'image' | null {
  if (!draft.trigger_message_id) return null
  const msg = messages.find((m) => m.id === draft.trigger_message_id)
  if (!msg) return null
  if (msg.media.some((md) => md.media_type === 'audio')) return 'audio'
  if (msg.media.some((md) => md.media_type === 'image')) return 'image'
  return null
}
