// kbEntities is the single source of truth for "what are the seven KB entity
// kinds, in what order, with which icon/label/identity" — Черновик's dynamic
// tabs, База знаний's fixed tabs, and every shared list/form component all
// read from here instead of each re-declaring their own copy (the ~150 lines
// duplicated across the old Playground.vue/KnowledgeBase.vue).
import type { Component } from 'vue'
import {
  Globe, Hash, ListTree, MapPinned, Package, PanelsTopLeft, Phone, Receipt, ShieldCheck, Target, Truck, UserRound,
} from 'lucide-vue-next'
import type { ChangeKind, ConfigField, KbRow } from '@/composables/draftChanges'

// KB_ENTITY_ORDER fixes the display order every tab row and stat breakdown
// uses — config (Обзор) first, matching the pre-existing База знаний
// convention, then the six natural-keyed entity kinds.
export const KB_ENTITY_ORDER: ChangeKind[] = ['config', 'topics', 'products', 'tariffs', 'delivery_zones', 'contacts', 'policies']

export interface EntityMeta {
  kind: ChangeKind
  labelKey: string
  icon: Component
  singleton: boolean
  keyOf: (row: KbRow) => string
}

export const ENTITY_META: Record<ChangeKind, EntityMeta> = {
  config: { kind: 'config', labelKey: 'kb.tabs.overview', icon: PanelsTopLeft, singleton: true, keyOf: () => 'main' },
  topics: { kind: 'topics', labelKey: 'kb.tabs.topics', icon: ListTree, singleton: false, keyOf: (r) => r.id },
  products: { kind: 'products', labelKey: 'kb.tabs.products', icon: Package, singleton: false, keyOf: (r) => r.id },
  tariffs: { kind: 'tariffs', labelKey: 'kb.tabs.tariffs', icon: Receipt, singleton: false, keyOf: (r) => r.id },
  delivery_zones: { kind: 'delivery_zones', labelKey: 'kb.tabs.zones', icon: MapPinned, singleton: false, keyOf: (r) => r.id },
  contacts: { kind: 'contacts', labelKey: 'kb.tabs.contacts', icon: Phone, singleton: true, keyOf: (r) => r.id },
  policies: { kind: 'policies', labelKey: 'kb.tabs.policies', icon: Truck, singleton: true, keyOf: (r) => r.id },
}

export const PRICING_TYPES = [
  { key: 'fixed', label: 'Фиксированная' },
  { key: 'percentage', label: 'Процент' },
  { key: 'tiered', label: 'Пороговая' },
]

export const ZONE_LEVELS = [
  { key: 'city', label: 'Город' },
  { key: 'region', label: 'Регион' },
  { key: 'country', label: 'Страна' },
]

export type ConfigRender = 'text' | 'list' | 'badge'
export type ConfigAccent = 'violet' | 'emerald' | 'amber' | 'sky' | 'indigo'
export interface ConfigSection {
  key: ConfigField
  titleKey: string
  hintKey: string
  icon: Component
  accent: ConfigAccent
  render: ConfigRender
}

// CONFIG_SECTIONS is the 5-card assistant-config document — Обзор on both
// pages renders one card per section, in this order.
export const CONFIG_SECTIONS: ConfigSection[] = [
  { key: 'persona', titleKey: 'kb.config.persona.title', hintKey: 'kb.config.persona.hint', icon: UserRound, accent: 'violet', render: 'text' },
  { key: 'mission', titleKey: 'kb.config.mission.title', hintKey: 'kb.config.mission.hint', icon: Target, accent: 'emerald', render: 'text' },
  { key: 'guardrails', titleKey: 'kb.config.guardrails.title', hintKey: 'kb.config.guardrails.hint', icon: ShieldCheck, accent: 'amber', render: 'list' },
  { key: 'language_policy', titleKey: 'kb.config.language_policy.title', hintKey: 'kb.config.language_policy.hint', icon: Globe, accent: 'sky', render: 'text' },
  { key: 'reply_max_words', titleKey: 'kb.config.reply_max_words.title', hintKey: 'kb.config.reply_max_words.hint', icon: Hash, accent: 'indigo', render: 'badge' },
]

export const CONFIG_ACCENT: Record<ConfigAccent, { box: string; bar: string }> = {
  violet: { box: 'bg-violet-100 text-violet-600', bar: 'bg-violet-400' },
  emerald: { box: 'bg-emerald-100 text-emerald-600', bar: 'bg-emerald-400' },
  amber: { box: 'bg-amber-100 text-amber-600', bar: 'bg-amber-400' },
  sky: { box: 'bg-sky-100 text-sky-600', bar: 'bg-sky-400' },
  indigo: { box: 'bg-indigo-100 text-indigo-600', bar: 'bg-indigo-400' },
}
