// payloads.ts is the discriminated union every KB create/edit form submits
// and stores.stageChange() dispatches on — one variant per content kind
// (config is NOT here: it stages through stores.patchConfig({ [field]: value })
// directly, one field at a time, never a whole-row payload).
export interface TopicPayload {
  kind: 'topics'
  slug: string
  title?: string
  body_md?: string
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
}

export type KbFormPayload =
  | TopicPayload
  | TariffPayload
  | ProductPayload
  | DeliveryZonePayload
  | ContactsPayload
  | PoliciesPayload
