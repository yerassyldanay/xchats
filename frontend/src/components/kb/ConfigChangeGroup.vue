<script setup lang="ts">
// ConfigChangeGroup is Черновик's Обзор tab body: one read card per PENDING
// assistant-config field (never one per section — untouched fields render
// nothing, decision 6), plus the section-level publish/cancel-all §1.5
// requires: ApproveSelector{Kind:"config"} always publishes the WHOLE
// pending patch, so there is no per-field Publish — only Изменить and
// Отменить изменение live on the card (records/actions.ts's
// CONFIG_FIELD_ACTIONS).
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { LoaderCircle, Save } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { useDraftChanges } from '@/composables/useDraftChanges'
import { useKbModal } from '@/composables/useKbModal'
import { useCancelConfirm } from '@/composables/useCancelConfirm'
import type { ChangeEntry } from '@/composables/draftChanges'
import { CONFIG_SECTIONS, NATURAL_KEY_MAIN } from './kbEntities'
import { CONFIG_FIELD_ACTIONS } from './records/actions'
import { Button } from '@/components/ui/button'
import AssistantFieldRecord from './records/AssistantFieldRecord.vue'

const pg = usePlayground()
const { entriesFor } = useDraftChanges()
const modal = useKbModal()
const cancelConfirm = useCancelConfirm()
const { t } = useI18n()

const entries = computed(() => entriesFor('config'))

function sectionFor(entry: ChangeEntry) {
  return CONFIG_SECTIONS.find((s) => s.key === entry.field)!
}
function isFieldBusy(entry: ChangeEntry) {
  return pg.busy || pg.publishingKey === `config:${entry.key}`
}
function edit(entry: ChangeEntry) {
  const base = pg.live?.config
  if (!base || !entry.field) return
  modal.openEdit('config', { ...base, [entry.field]: entry.draftValue ?? base[entry.field] }, { field: entry.field })
}
// Both cancels go through the shared confirmation (useCancelConfirm) — a
// config edit is prose an operator typed by hand, so discarding it silently
// on one click was the same misclick hazard as an entity card's.
// entry.type is always 'updated' for a config field (a section is either
// patched or absent), which is also the accurate description: cancelling
// restores the published value.
function cancelField(entry: ChangeEntry) {
  cancelConfirm.requestOne('config', entry.key, entry.type)
}
function publishSection() {
  pg.approveEntity('config', NATURAL_KEY_MAIN)
}
function cancelSection() {
  cancelConfirm.requestOne('config', NATURAL_KEY_MAIN, 'updated')
}
</script>

<template>
  <div v-if="entries.length" class="space-y-3">
    <AssistantFieldRecord
      v-for="entry in entries"
      :key="entry.key"
      :section="sectionFor(entry)"
      :value="entry.draftValue ?? ''"
      :live-value="entry.liveValue"
      change-type="updated"
      :actions="CONFIG_FIELD_ACTIONS"
      :busy="isFieldBusy(entry)"
      @edit="edit(entry)"
      @cancel="cancelField(entry)"
    />
    <div class="flex items-center gap-2 pt-1">
      <Button size="sm" variant="ghost" class="text-destructive" :disabled="pg.busy || pg.approving" @click="cancelSection">
        {{ t('kb.config.cancelAllSection') }}
      </Button>
      <Button size="sm" :disabled="pg.busy || pg.approving" @click="publishSection">
        <LoaderCircle v-if="pg.approving && pg.publishingKey === `config:${NATURAL_KEY_MAIN}`" class="w-4 h-4 animate-spin" />
        <Save v-else class="w-4 h-4" />
        {{ t('kb.config.publishSection') }}
      </Button>
    </div>
  </div>
</template>
