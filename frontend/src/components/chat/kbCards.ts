import type { Component } from 'vue'
import { Sparkles } from 'lucide-vue-next'
import { ENTITY_META } from '@/components/kb/kbEntities'
import type { KbRecordKind, KbSource } from '@/types'

// Shared vocabulary for the chat's KB cards.
//
// Entity kinds come straight from the KB editor's own ENTITY_META (the
// backend deliberately speaks the same plural vocabulary — see
// internal/chatkb's Kind), so a product card in the chat carries the same
// icon and the same localized entity name as the product row on
// /knowledge-base. One place to rename an entity, not two.

/** iconFor returns the KB editor's icon for an entity kind. */
export function iconFor(kind: KbRecordKind | ''): Component {
  return kind && kind in ENTITY_META ? ENTITY_META[kind as keyof typeof ENTITY_META].icon : Sparkles
}

/** i18nKeyFor returns the kb.entities.<kind> key for an entity kind. */
export function i18nKeyFor(kind: KbRecordKind | ''): string {
  return kind && kind in ENTITY_META ? ENTITY_META[kind as keyof typeof ENTITY_META].i18nKey : ''
}

// sourceClasses tints a source badge. Live and draft are deliberately far
// apart visually — an operator glancing at a card has to be able to tell
// which state a number came from without reading the label.
export function sourceClasses(source: KbSource): string {
  return source === 'DRAFT_KB'
    ? 'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-700/60 dark:bg-amber-950/40 dark:text-amber-200'
    : 'border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-700/60 dark:bg-emerald-950/40 dark:text-emerald-200'
}
