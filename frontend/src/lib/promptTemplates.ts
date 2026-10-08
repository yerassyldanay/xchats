import type { PromptTemplateId } from '@/types'

// Locale keys under kb.template.profiles.<key> are camelCase (a hyphen is not a
// safe segment in a vue-i18n key path), while the template IDs the backend
// stores are the kebab-case text IDs of ai_prompt_templates.
const PROFILE_KEY: Record<PromptTemplateId, string> = {
  general: 'general',
  'online-shop': 'onlineShop',
  'service-business': 'serviceBusiness',
  'online-service': 'onlineService',
}

export function profileKey(id: string): string | undefined {
  return PROFILE_KEY[id as PromptTemplateId]
}

// MAX_TEMPLATE_CHARS mirrors aiprompt.MaxTemplateRunes. The server is the
// authority; this only lets the editor say "too long" before a round trip.
export const MAX_TEMPLATE_CHARS = 40000

export type TemplateIssue = 'empty' | 'reserved' | 'tooLong' | null

// templateIssue is the client-side mirror of the cheap, deterministic part of
// aiprompt.ValidateTemplateInstructions (empty / reserved slot & placeholder
// syntax / length). Identifier- and link-shaped text is left to the server.
export function templateIssue(text: string): TemplateIssue {
  if (text.trim() === '') return 'empty'
  if (text.includes('%%') || text.includes('{{') || text.includes('}}')) return 'reserved'
  if ([...text].length > MAX_TEMPLATE_CHARS) return 'tooLong'
  return null
}
