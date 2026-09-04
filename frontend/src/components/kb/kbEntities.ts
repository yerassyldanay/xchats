// kbEntities.ts is the one shared vocabulary table for every KB entity kind —
// icon, i18n key, singleton-ness, natural-key extraction, and which
// DraftChangeSet/DraftView field actually carries it. Both Черновик and Base
// знаний import this instead of each re-declaring their own tab list/icon
// map/pricing-type options (previously ~150 duplicated lines across the two
// pages).
import type { Component } from 'vue'
import { Globe, Hash, Info, ListTree, MapPinned, Package, Phone, Receipt, ShieldCheck, Sparkles, Target, Truck, UserRound } from 'lucide-vue-next'
import type { ChangeKind, KbRow } from '@/composables/draftChanges'
import type { DraftChangeSet, DraftView } from '@/types'

// NATURAL_KEY_MAIN is the fixed key for a true singleton entity's
// whole-entity operations (approveEntity('config', NATURAL_KEY_MAIN),
// cancelChange('config', NATURAL_KEY_MAIN)) — mirrors the backend's
// kbstore.NaturalKeyMain. Contacts/policies do NOT need this: their rows
// already carry the right key as `.id` (domain.ContactSlug/PolicySlug), so
// ENTITY_META.keyOf reads it off the row like every other kind.
export const NATURAL_KEY_MAIN = 'main'

// KB_ENTITY_ORDER is the fixed display order — Обзор first (it is the
// assistant config, conceptually "above" the content tables), then content
// kinds in the same order the pre-redesign pages already used. Черновик
// filters this down to kinds with pending entries (useEntityTabs); Знаний
// база shows all seven plus Промпт/Файлы.
export const KB_ENTITY_ORDER: ChangeKind[] = ['config', 'topics', 'products', 'tariffs', 'tariff_info', 'delivery_zones', 'contacts', 'policies']

// payloadField is DraftChangeSet/DraftView's own field name for this kind —
// almost always identical to the kind, with ONE deliberate exception:
// delivery_zones's field is `zones`, not `delivery_zones` (matches the Go
// struct tag `json:"zones"` — see kbstore.DraftView.Zones's doc comment).
// Every array lookup goes through this, never `changes[kind]` directly, so
// that trap can only ever be wrong in one place.
type PayloadField = 'config' | 'topics' | 'tariffs' | 'products' | 'contacts' | 'policies' | 'tariff_info' | 'zones'

export interface EntityMeta {
  icon: Component
  i18nKey: string // -> kb.entities.<kind>.{singular,plural}
  singleton: boolean
  keyOf: (row: KbRow) => string
  payloadField: PayloadField
}

const idOf = (row: KbRow) => row.id

export const ENTITY_META: Record<ChangeKind, EntityMeta> = {
  config: { icon: Sparkles, i18nKey: 'kb.entities.config', singleton: true, keyOf: () => NATURAL_KEY_MAIN, payloadField: 'config' },
  topics: { icon: ListTree, i18nKey: 'kb.entities.topics', singleton: false, keyOf: idOf, payloadField: 'topics' },
  products: { icon: Package, i18nKey: 'kb.entities.products', singleton: false, keyOf: idOf, payloadField: 'products' },
  tariffs: { icon: Receipt, i18nKey: 'kb.entities.tariffs', singleton: false, keyOf: idOf, payloadField: 'tariffs' },
  delivery_zones: { icon: MapPinned, i18nKey: 'kb.entities.delivery_zones', singleton: false, keyOf: idOf, payloadField: 'zones' },
  contacts: { icon: Phone, i18nKey: 'kb.entities.contacts', singleton: true, keyOf: idOf, payloadField: 'contacts' },
  policies: { icon: Truck, i18nKey: 'kb.entities.policies', singleton: true, keyOf: idOf, payloadField: 'policies' },
  tariff_info: { icon: Info, i18nKey: 'kb.entities.tariff_info', singleton: true, keyOf: idOf, payloadField: 'tariff_info' },
}

// rowsOf/deletesOf are the one place a caller reads a ChangeKind's array out
// of a DraftChangeSet/DraftView without risking the zones/delivery_zones typo.
export function rowsOf(kind: ChangeKind, view: DraftChangeSet | DraftView | null | undefined): KbRow[] {
  if (!view || kind === 'config') return []
  return (view[ENTITY_META[kind].payloadField as Exclude<PayloadField, 'config'>] as KbRow[]) ?? []
}

export const PRICING_TYPES = ['fixed', 'percentage', 'tiered'] as const
export type PricingType = (typeof PRICING_TYPES)[number]

export const ZONE_LEVELS = ['city', 'region', 'country'] as const
export type ZoneLevel = (typeof ZONE_LEVELS)[number]

// AVAILABILITY_STATUSES mirrors aiprompt's closed vocabulary
// (backend/aiprompt/types.go's Product.AvailabilityStatus doc comment):
// in_stock/preorder/on_demand render full prose+facts+media in the customer
// prompt, unavailable suppresses all of it down to the name alone.
export const AVAILABILITY_STATUSES = ['in_stock', 'preorder', 'on_demand', 'unavailable'] as const
export type AvailabilityStatus = (typeof AVAILABILITY_STATUSES)[number]

// --- Обзор (assistant config) sections --------------------------------------

export type ConfigField = 'persona' | 'mission' | 'guardrails' | 'language_policy' | 'reply_max_words'
export type ConfigRender = 'text' | 'list' | 'badge' // badge = numeric, shown as a Badge in read mode

export interface ConfigSectionMeta {
  key: ConfigField
  icon: Component
  accent: 'violet' | 'emerald' | 'amber' | 'sky' | 'indigo'
  render: ConfigRender
  i18nKey: string // -> kb.config.<field>.{title,hint}
}

export const CONFIG_SECTIONS: ConfigSectionMeta[] = [
  { key: 'persona', icon: UserRound, accent: 'violet', render: 'text', i18nKey: 'kb.config.persona' },
  { key: 'mission', icon: Target, accent: 'emerald', render: 'text', i18nKey: 'kb.config.mission' },
  { key: 'guardrails', icon: ShieldCheck, accent: 'amber', render: 'list', i18nKey: 'kb.config.guardrails' },
  { key: 'language_policy', icon: Globe, accent: 'sky', render: 'text', i18nKey: 'kb.config.language_policy' },
  { key: 'reply_max_words', icon: Hash, accent: 'indigo', render: 'badge', i18nKey: 'kb.config.reply_max_words' },
]

export const CONFIG_ACCENT: Record<ConfigSectionMeta['accent'], { box: string; bar: string }> = {
  violet: { box: 'bg-violet-100 text-violet-600', bar: 'bg-violet-400' },
  emerald: { box: 'bg-emerald-100 text-emerald-600', bar: 'bg-emerald-400' },
  amber: { box: 'bg-amber-100 text-amber-600', bar: 'bg-amber-400' },
  sky: { box: 'bg-sky-100 text-sky-600', bar: 'bg-sky-400' },
  indigo: { box: 'bg-indigo-100 text-indigo-600', bar: 'bg-indigo-400' },
}
