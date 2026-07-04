export interface User {
  id: string
  email: string
  name: string
}
export interface Organization {
  id: string
  name: string
}
export interface Contact {
  id: string
  display_name: string
  phone_number: string
  phone_jid: string
  lid_jid: string
  push_name: string
  attributes?: Record<string, unknown>
}
export interface Chat {
  id: string
  wa_account_id: string
  contact: Contact
  status: string
  assignee_user_id: string | null
  unread_count: number
  last_message_at: string | null
  last_message_preview: string
}
export interface Media {
  id: string
  url: string
  media_type: string
  mimetype: string
  file_name: string
  file_size: number
}
export interface Message {
  id: string
  chat_id: string
  direction: 'in' | 'out'
  sender_type: string
  evolution_message_id: string
  message_type: string
  content: string
  media: Media[]
  status: string
  source: string
  timestamp: string | null
}
export interface WhatsAppAccount {
  id: string
  instance_name: string
  display_name: string
  connection_status: string // connecting | qr_required | connected | disconnected | error
  assigned: boolean
  owner_jid: string
  phone_number: string
  last_live_event_at: string | null
  created_at: string | null
}
export interface QrResponse {
  status: string // qr_required | connecting | connected
  qr_code?: string | null
  qr_base64?: string | null
  pairing_code?: string | null
  wa_account_id?: string
  instance_name?: string
}
export interface EvolutionInstance {
  name: string
  connection_status: string
  owner_jid: string
  phone_number: string
  created_at: string | null
  managed: boolean
  stale: boolean
}
export interface DraftMedia {
  asset_id: string
  media_kind: string
  url: string
}
export interface AiDraft {
  id: string
  chat_id: string
  trigger_message_id: string | null
  ordinal: number
  draft_text: string
  media: DraftMedia[]
  context_status: string
  confidence: number | null
  escalate: boolean
  escalation_reason: string
  status: string
  created_at: string
}

// --- Knowledge Base (Playground) — see plan/7.1-endpoints.md, plan/15 ---
// An entity is either LIVE (a row in a live ai_ table) or «Черновик» — present
// in the kbd_draft blob, flagged `draft: true` here. There is no more
// review_state/proposed-approved-rejected: a pending entity IS the "Правки" set.

export interface DraftConfig {
  organization_id: string
  persona: string
  mission: string
  guardrails: string
  language_policy: string
  reply_max_words: number
  draft: boolean
  base_version: number
  updated_at: string
}
export interface TopicRow {
  id: string // = slug (blob entries carry no DB row id)
  slug: string
  lang: string
  title: string
  keywords: string
  body_md: string
  draft: boolean
  provenance?: string
  updated_at: string
}
export interface AssetRow {
  id: string // = ref
  ref: string
  kind: string
  owner_kind: string // 'topic' | 'product' | 'tariff' | '' (unattached)
  owner_ref: string
  title: string
  description: string
  url: string
  lang: string
  draft: boolean
  provenance?: string
  updated_at: string
}
// Facts lane — typed entity rows. Every exact fact (price, limit, phone, …) is a
// CONCRETE COLUMN here, never a generic key→value; the brain quotes it in replies
// as a {{table.slug.field}} token. `id` = the row's natural key (ref, or the
// contact singleton lang) since blob entries carry no DB row id.
export interface TariffRow {
  id: string // = ref
  ref: string
  lang: string
  name: string
  price: string
  limit_text: string
  fee: string
  summary: string
  pricing_type: string // fixed | percentage | tiered
  advantages: string
  disadvantages: string
  draft: boolean
  provenance?: string
  updated_at: string
}
export interface ProductRow {
  id: string // = ref
  ref: string
  lang: string
  name: string
  price: string
  description: string
  category: string
  availability: string
  draft: boolean
  provenance?: string
  updated_at: string
}
export interface ContactRow {
  id: string // = lang (the 'support' singleton, one row per language)
  slug: string
  lang: string
  whatsapp: string
  email: string
  address: string
  legal: string
  callback_time: string
  working_hours: string
  phone: string
  website: string
  instagram: string
  draft: boolean
  provenance?: string
  updated_at: string
}
// PolicyRow — org commerce-policy scalars (delivery/payment/returns terms), a
// structural clone of ContactRow (id = lang, the 'main' singleton).
export interface PolicyRow {
  id: string // = lang
  slug: string
  lang: string
  delivery_cost: string
  delivery_time: string
  free_delivery_from: string
  min_order: string
  prepayment: string
  installment: string
  return_period: string
  warranty: string
  draft: boolean
  provenance?: string
  updated_at: string
}
export interface KbMaterial {
  id: string
  source_type: string
  source_ref: string
  blob_id: string
  extracted_text: string
  media_kind: string
  status: string // ready | pending | failed
  extraction: string
  created_at: string
  updated_at: string
}
export interface KbRequest {
  id: string
  material_id: string | null
  req_type: string // confirm_fact | describe_media | comment
  prompt: string
  context: string // JSON string, e.g. {"suggested":"5000 ₸"}
  target: string // JSON string, e.g. {table,slug,field,lang} | {asset_ref} | {material_id}
  state: string // pending | resolved | dismissed
  resolution: string | null
  created_at: string
  resolved_at: string | null
}
export interface DraftView {
  config: DraftConfig
  topics: TopicRow[]
  assets: AssetRow[]
  tariffs: TariffRow[]
  products: ProductRow[]
  contacts: ContactRow[]
  policies: PolicyRow[]
  materials: KbMaterial[]
  requests: KbRequest[]
}
