<script setup lang="ts">
// ConfigChangeGroup renders Черновик's Обзор section: one AssistantFieldRecord
// card per changed config field, plus the section-level publish/cancel-all
// (§1.5 — the backend can only publish or clear the WHOLE staged config
// patch, never one field in isolation, so the UI is shaped to match that).
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { LoaderCircle } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { usePlayground } from '@/stores/playground'
import { useKbModal } from '@/composables/useKbModal'
import { useLiveIndex } from '@/composables/useLiveIndex'
import type { ChangeEntry, ConfigField } from '@/composables/draftChanges'
import AssistantFieldRecord from './records/AssistantFieldRecord.vue'
import type { KbAction } from './records/actions'

defineProps<{ entries: ChangeEntry[] }>()
const pg = usePlayground()
const modal = useKbModal()
const { config: liveConfig } = useLiveIndex()
const { t } = useI18n()

const fieldActions: KbAction[] = [
  { key: 'edit', labelKey: 'kb.common.edit', variant: 'outline' },
  { key: 'cancel', labelKey: 'kb.common.cancelChange', variant: 'ghost', destructive: true },
]

const cancelAllBusy = computed(() => pg.busy)
const publishAllBusy = computed(() => pg.publishingKey === 'config:main')
const anyBusy = computed(() => pg.busy || pg.approving)

function liveValueFor(field?: ConfigField): string | number | undefined {
  return field && liveConfig.value ? liveConfig.value[field] : undefined
}
// Re-editing an already-staged field seeds the form from the CURRENT PENDING
// value (entry.draftValue), never the published baseline — otherwise
// clicking Edit on a changed field would silently discard what is already
// staged for it.
function edit(entry: ChangeEntry) {
  if (!entry.field || entry.draftValue === undefined) return
  modal.openEdit('config', { [entry.field]: entry.draftValue }, { field: entry.field })
}
function cancelField(entry: ChangeEntry) {
  pg.cancelChange('config', entry.key)
}
function publishAll() {
  pg.approveEntity('config', 'main')
}
function cancelAll() {
  pg.cancelChange('config', 'main')
}
</script>

<template>
  <div class="space-y-3">
    <AssistantFieldRecord
      v-for="entry in entries"
      :key="entry.key"
      :field="entry.field!"
      :draft-value="entry.draftValue!"
      :live-value="liveValueFor(entry.field)"
      change-type="updated"
      :actions="fieldActions"
      :busy="pg.busy"
      @edit="edit(entry)"
      @cancel="cancelField(entry)"
    />
    <div class="flex items-center gap-2">
      <Button variant="ghost" size="sm" class="text-destructive" :disabled="anyBusy" @click="cancelAll">
        <LoaderCircle v-if="cancelAllBusy" class="w-4 h-4 animate-spin" /> {{ t('kb.draft.cancelConfigGroup') }}
      </Button>
      <Button size="sm" :disabled="anyBusy" @click="publishAll">
        <LoaderCircle v-if="publishAllBusy" class="w-4 h-4 animate-spin" /> {{ t('kb.draft.publishConfigGroup') }}
      </Button>
    </div>
  </div>
</template>
