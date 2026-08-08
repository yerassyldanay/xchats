// payloads.ts is the discriminated union every KB create/edit form submits
// and stores.stageChange() dispatches on — one variant per content kind
// (config is NOT here: it stages through stores.patchConfig({ [field]: value })
// directly, one field at a time, never a whole-row payload).
//
// Media fields mirror the canonical ai_* columns (backend/internal/kbstore's
// Draft* structs): a singular reference is `string | null` (never omitted —
// see MediaFieldPicker.vue's doc comment on why the form always sends the
// picker's full current state), a plural reference is `string[]` (never
// undefined; an empty array IS "detach everything").
export interface TopicPayload {
  kind: 'topics'
  slug: string
  title?: string
  body_md?: string
  featured_image?: string | null
  illustration_images?: string[]
  explainer_videos?: string[]
  reference_documents?: string[]
}

export interface TariffPayload {
  kind: 'tariffs'
  ref: string
  name?: string
  price?: string
  limit_text?: string
  fee?: string
  summary?: string
  pricing_type?: string
  advantages?: string
  disadvantages?: string
  sales_status?: string
  featured_image?: string | null
  pricing_images?: string[]
  explainer_videos?: string[]
  terms_documents?: string[]
}

export interface ProductPayload {
  kind: 'products'
  ref: string
  name?: string
  price?: string
  description?: string
  category?: string
  sales_status?: string
  in_stock?: boolean
  featured_image?: string | null
  gallery_images?: string[]
  demo_videos?: string[]
  certificate_documents?: string[]
  guarantee_documents?: string[]
}

export interface DeliveryZonePayload {
  kind: 'delivery_zones'
  ref: string
  name?: string
  zone_level: string
  parent_ref?: string
  delivery_available?: boolean
  delivery_cost?: string
  delivery_in_days?: string
  notes?: string
  sales_status?: string
}

export interface ContactsPayload {
  kind: 'contacts'
  whatsapp?: string
  email?: string
  address?: string
  legal_information?: string
  callback_time?: string
  working_hours?: string
  phone?: string
  website?: string
  instagram?: string
  contact_card_image?: string | null
  location_map_image?: string | null
  company_legal_documents?: string[]
}

export interface PoliciesPayload {
  kind: 'policies'
  delivery_cost?: string
  delivery_in_days?: string
  free_delivery_from?: string
  min_order?: string
  prepayment?: string
  installment?: string
  return_period_in_days?: string
  warranty?: string
  outside_zones_note?: string
  commerce_policy_documents?: string[]
}

export type KbFormPayload =
  | TopicPayload
  | TariffPayload
  | ProductPayload
  | DeliveryZonePayload
  | ContactsPayload
  | PoliciesPayload
