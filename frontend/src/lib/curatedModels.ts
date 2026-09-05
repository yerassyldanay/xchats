// Curated model suggestions for the AI Engine settings' model Combobox
// (TODO.md "Dynamic provider credentials form and model combobox") —
// popular, known-good picks per provider, shown as a starting point. The
// combobox itself is plain-text underneath, so nothing here is a closed
// list: an operator can always type a model id that isn't in this file.
export const CURATED_MODELS: Record<string, string[]> = {
  openrouter: [
    'google/gemini-2.5-flash',
    'google/gemini-2.5-pro',
    'openai/gpt-4o',
    'openai/gpt-4o-mini',
    'anthropic/claude-3.5-sonnet',
    'deepseek/deepseek-chat',
  ],
  openai: ['gpt-4o', 'gpt-4o-mini', 'gpt-4-turbo', 'o1', 'o3-mini'],
  gemini: ['gemini-2.5-flash', 'gemini-2.5-pro', 'gemini-1.5-flash', 'gemini-1.5-pro'],
}

// Multimodal (vision-capable) models per provider — a NARROWER list than
// CURATED_MODELS above, since not every chat model on a provider also
// accepts an image_url content part. Used by the Vision Model combobox
// (AiEngineTab.vue), scoped to whichever provider is the current "Default
// provider" — see LLMSettings.VisionModel's own doc comment for why there
// is no separate vision provider selector.
export const CURATED_VISION_MODELS: Record<string, string[]> = {
  openrouter: ['google/gemini-2.5-flash', 'google/gemini-2.5-pro', 'openai/gpt-4o', 'openai/gpt-4o-mini'],
  openai: ['gpt-4o', 'gpt-4o-mini', 'gpt-4-turbo'],
  gemini: ['gemini-2.5-flash', 'gemini-2.5-pro', 'gemini-1.5-flash', 'gemini-1.5-pro'],
}

// Transcription models per STT provider (internal/stt) — only OpenAI and
// Groq serve the OpenAI-compatible /audio/transcriptions endpoint this
// feature depends on (see internal/stt.DefaultBaseURL's own doc comment).
export const CURATED_STT_MODELS: Record<string, string[]> = {
  openai: ['whisper-1', 'gpt-4o-transcribe', 'gpt-4o-mini-transcribe'],
  groq: ['whisper-large-v3-turbo', 'whisper-large-v3', 'distil-whisper-large-v3-en'],
}
