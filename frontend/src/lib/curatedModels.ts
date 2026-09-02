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
