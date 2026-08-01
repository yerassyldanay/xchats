// Discriminated-union-ish form payload types — the kind is already known from
// context (ModalSession.kind / the stageChange(kind, payload) call site), so
// these stay plain interfaces rather than a tagged union.
import type { ConfigField } from '@/composables/draftChanges'

export interface TopicPayload {
  slug: string
  title: string
  body_md: string
}
export interface ProductPayload {
  ref: string
  name: string
  price: string
  description: string
  category: string
  sales_status: string
  in_stock: boolean
}
export interface TariffPayload {
  ref: string
  name: string
  price: string
  limit_text: string
  fee: string
  summary: string
  pricing_type: string
  advantages: string
  disadvantages: string
  sales_status: string
}
export interface ZonePayload {
  ref: string
  name: string
  zone_level: string
  parent_ref: string
  delivery_available: boolean
  delivery_cost: string
  delivery_in_days: string
  notes: string
  sales_status: string
}
export interface ContactsPayload {
  whatsapp: string
  email: string
  address: string
  legal_information: string
  callback_time: string
  working_hours: string
  phone: string
  website: string
  instagram: string
}
export interface PoliciesPayload {
  delivery_cost: string
  delivery_in_days: string
  free_delivery_from: string
  min_order: string
  prepayment: string
  installment: string
  return_period_in_days: string
  warranty: string
  outside_zones_note: string
}
export interface ConfigFieldPayload {
  field: ConfigField
  value: string | number
}

export type KbFormPayload =
  | TopicPayload
  | ProductPayload
  | TariffPayload
  | ZonePayload
  | ContactsPayload
  | PoliciesPayload
  | ConfigFieldPayload
