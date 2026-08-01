// kbActions is the action row RecordShell renders — moved out of the store
// (it needs no Pinia access) so it is unit-testable in isolation. RecordShell
// itself takes the resulting array as an explicit prop; ChangeList/
// RecordList compute it per-entry from this function.
import type { ChangeType } from '@/composables/draftChanges'

export type KbActionKey = 'edit' | 'publish' | 'cancel' | 'delete'
export interface KbAction {
  key: KbActionKey
  labelKey: string
  variant: 'default' | 'outline' | 'ghost'
  destructive?: boolean
}

export function kbActions(ctx: { page: 'draft' | 'live'; changeType?: ChangeType; singleton?: boolean }): KbAction[] {
  if (ctx.page === 'live') {
    const actions: KbAction[] = [{ key: 'edit', labelKey: 'kb.common.edit', variant: 'outline' }]
    if (!ctx.singleton) actions.push({ key: 'delete', labelKey: 'kb.common.delete', variant: 'ghost', destructive: true })
    return actions
  }
  // page === 'draft': a removed entry has nothing left to edit.
  if (ctx.changeType === 'removed') {
    return [
      { key: 'publish', labelKey: 'kb.common.publish', variant: 'default' },
      { key: 'cancel', labelKey: 'kb.common.cancelChange', variant: 'ghost', destructive: true },
    ]
  }
  return [
    { key: 'edit', labelKey: 'kb.common.edit', variant: 'outline' },
    { key: 'publish', labelKey: 'kb.common.publish', variant: 'default' },
    { key: 'cancel', labelKey: 'kb.common.cancelChange', variant: 'ghost', destructive: true },
  ]
}
