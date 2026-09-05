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
  // Matches ChatThread.vue's own isAudio/isImage: media_type is the primary
  // signal, with a mimetype fallback for a row whose media_type came back
  // generic — keeping the two checks in sync means a voice note that renders
  // an audio player in the thread always gets the same badge here.
  if (msg.media.some((md) => md.media_type === 'audio' || md.mimetype.startsWith('audio/'))) return 'audio'
  if (msg.media.some((md) => md.media_type === 'image' || md.mimetype.startsWith('image/'))) return 'image'
  return null
}
