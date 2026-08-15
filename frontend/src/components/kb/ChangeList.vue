<script setup lang="ts">
// ChangeList renders one kind's pending entries as read-only cards, wiring
// Изменить/Опубликовать/Отменить изменение — the topics/products/tariffs
// three, which share an identical card prop shape (row/liveRow/changeType/
// actions/busy/blockedNote). delivery_zones (needs allZones), contacts and
// policies (singletons, policies needs zonesExist) are special-cased
// directly in DraftKnowledgeBase.vue instead of forced through this generic
// shape; config is ConfigChangeGroup.vue's own concern entirely.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePlayground } from '@/stores/playground'
import { useDraftChanges } from '@/composables/useDraftChanges'
import { useKbModal } from '@/composables/useKbModal'
import { useDraftSelection } from '@/composables/useDraftSelection'
import { useCancelConfirm } from '@/composables/useCancelConfirm'
import type { ChangeEntry } from '@/composables/draftChanges'
import { kbActions } from './records/actions'
import TopicRecord from './records/TopicRecord.vue'
import ProductRecord from './records/ProductRecord.vue'
import TariffRecord from './records/TariffRecord.vue'

const props = defineProps<{ kind: 'topics' | 'products' | 'tariffs' }>()

const pg = usePlayground()
const { entriesFor } = useDraftChanges()
const modal = useKbModal()
const selection = useDraftSelection()
const cancelConfirm = useCancelConfirm()
const { t } = useI18n()

const COMPONENTS = { topics: TopicRecord, products: ProductRecord, tariffs: TariffRecord }
const component = computed(() => COMPONENTS[props.kind])
const entries = computed(() => entriesFor(props.kind))

function actionsFor(entry: ChangeEntry) {
  return kbActions({ page: 'draft', changeType: entry.type })
}
function isBusy(entry: ChangeEntry) {
  return pg.busy || pg.publishingKey === `${props.kind}:${entry.key}`
}
function blockedNote(entry: ChangeEntry) {
  return pg.gateBlockedKey === `${props.kind}:${entry.key}` ? t('kb.draft.cardBlockedNote') : undefined
}
function edit(entry: ChangeEntry) {
  const row = entry.type === 'removed' ? entry.liveRow : entry.draftRow
  if (row) modal.openEdit(props.kind, row)
}

// rowOf/liveRowOf exist for ONE reason: <component :is> can't let TypeScript
// narrow a prop type by the runtime `kind` match the way a static
// <TopicRecord v-if=...> chain could — the cast is confined to these two
// call sites, everything else in this file stays fully typed.
function rowOf(entry: ChangeEntry) {
  return (entry.type === 'removed' ? entry.liveRow : entry.draftRow) as any
}
function liveRowOf(entry: ChangeEntry) {
  return (entry.type === 'removed' ? undefined : entry.liveRow) as any
}
function publish(entry: ChangeEntry) {
  pg.approveEntity(props.kind, entry.key)
}
// Every cancel goes through the confirmation now — see useCancelConfirm's
// own doc comment. entry.type is passed so the dialog can say what THIS
// cancel actually does (destroy vs revert vs keep), which differs per state.
function cancel(entry: ChangeEntry) {
  cancelConfirm.requestOne(props.kind, entry.key, entry.type)
}
</script>

<template>
  <component
    :is="component"
    v-for="entry in entries"
    :key="entry.key"
    :row="rowOf(entry)"
    :live-row="liveRowOf(entry)"
    :change-type="entry.type"
    :actions="actionsFor(entry)"
    :busy="isBusy(entry)"
    :blocked-note="blockedNote(entry)"
    selectable
    :selected="selection.isSelected(kind, entry.key)"
    @edit="edit(entry)"
    @publish="publish(entry)"
    @cancel="cancel(entry)"
    @toggle-select="selection.toggle(kind, entry.key)"
  />
</template>
