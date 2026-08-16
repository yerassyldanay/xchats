// actions.ts is the single source of truth for the action row every kb card
// renders — moved out of the Pinia store (it has no business being there:
// it derives a button set from three plain values, nothing store-shaped)
// and rewritten for the Черновик/Знаний база split: a draft-mode card now
// offers Изменить/Опубликовать/Отменить изменение, never the old
// Сохранить/Принять/Отклонить trio whose "Отклонить" quietly staged a live
// tombstone (see stores/playground.ts's kbActions, superseded by this file).
import type { ChangeType } from '@/composables/draftChanges'

export type KbActionKey = 'edit' | 'publish' | 'cancel' | 'delete'

export interface KbAction {
  key: KbActionKey
  labelKey: string // vue-i18n key, e.g. 'kb.actions.edit'
  variant: 'default' | 'outline' | 'ghost'
  destructive?: boolean
}

const EDIT: KbAction = { key: 'edit', labelKey: 'kb.actions.edit', variant: 'outline' }
const PUBLISH: KbAction = { key: 'publish', labelKey: 'kb.actions.publish', variant: 'default' }
const CANCEL: KbAction = { key: 'cancel', labelKey: 'kb.actions.cancel', variant: 'ghost', destructive: true }
// REMOVE_FROM_DRAFT is CANCEL's label for an `added` entry only — same key,
// same handler, same endpoint. «Отменить изменение» is accurate for an
// `updated` entry (the published value comes back) and for a `removed` one
// (the record is KEPT), but on a NEW card there is nothing to revert to:
// the record is destroyed outright, and for an imported one that means
// re-running a paid crawl to get it back. The button now says so.
const REMOVE_FROM_DRAFT: KbAction = { key: 'cancel', labelKey: 'kb.actions.removeFromDraft', variant: 'ghost', destructive: true }
const DELETE: KbAction = { key: 'delete', labelKey: 'kb.actions.delete', variant: 'ghost', destructive: true }

// kbActions is the action row for the six content kinds (topics, tariffs,
// products, delivery zones, contacts, policies) on BOTH pages:
//   - page 'live' (Знаний база — RecordList.vue): Изменить + Удалить, minus
//     Удалить for a singleton (contacts/policies have no delete affordance
//     at all — there is nothing to "delete back to").
//   - page 'draft' (Черновик — ChangeList.vue): Изменить + Опубликовать +
//     Отменить изменение, minus Изменить for a `removed` entry (a staged
//     deletion has nothing to edit — only publish it or cancel it).
// Config's per-field cards do NOT go through this function — see
// CONFIG_FIELD_ACTIONS below; config has no per-field publish at all
// (publishing is group-level, ConfigChangeGroup.vue's own button).
export function kbActions(ctx: { page: 'draft' | 'live'; changeType?: ChangeType; singleton?: boolean }): KbAction[] {
  if (ctx.page === 'live') {
    return ctx.singleton ? [EDIT] : [EDIT, DELETE]
  }
  if (ctx.changeType === 'removed') return [PUBLISH, CANCEL]
  return [EDIT, PUBLISH, ctx.changeType === 'added' ? REMOVE_FROM_DRAFT : CANCEL]
}

// CONFIG_FIELD_ACTIONS is the fixed action set for one staged assistant-
// config field card (ConfigChangeGroup.vue) — Изменить + Отменить изменение
// only. There is no per-field Publish: ApproveSelector{Kind:"config"}
// publishes the WHOLE pending patch, so publishing lives on the section
// («Опубликовать изменения ассистента»), not on an individual field.
export const CONFIG_FIELD_ACTIONS: KbAction[] = [EDIT, CANCEL]

// LIVE_CONFIG_ACTIONS is Знаний база's Обзор tab — Изменить only, on a
// PUBLISHED field with no pending edit to cancel (decision 5: every config
// section card gets an Edit action).
export const LIVE_CONFIG_ACTIONS: KbAction[] = [EDIT]
