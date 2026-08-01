<script setup lang="ts">
// ChangeList renders one entity kind's pending change entries on Черновик,
// wiring each card's edit/publish/cancel to the right store call. config is
// never routed through here — see ConfigChangeGroup.vue.
import { computed, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePlayground } from '@/stores/playground'
import { useKbModal } from '@/composables/useKbModal'
import type { ChangeEntry, ChangeKind, KbRow } from '@/composables/draftChanges'
import { ENTITY_META } from '@/components/kb/kbEntities'
import { kbActions } from './records/actions'
import TopicRecord from './records/TopicRecord.vue'
import ProductRecord from './records/ProductRecord.vue'
import TariffRecord from './records/TariffRecord.vue'
import DeliveryZoneRecord from './records/DeliveryZoneRecord.vue'
import ContactsRecord from './records/ContactsRecord.vue'
import PoliciesRecord from './records/PoliciesRecord.vue'

const props = defineProps<{ kind: ChangeKind; entries: ChangeEntry[] }>()
const pg = usePlayground()
const modal = useKbModal()
const { t } = useI18n()

const RECORD_COMPONENTS: Partial<Record<ChangeKind, Component>> = {
  topics: TopicRecord,
  products: ProductRecord,
  tariffs: TariffRecord,
  delivery_zones: DeliveryZoneRecord,
  contacts: ContactsRecord,
  policies: PoliciesRecord,
}
const component = computed(() => RECORD_COMPONENTS[props.kind])
const kindLabel = computed(() => t(ENTITY_META[props.kind].labelKey))

// Parent-zone name lookup for DeliveryZoneRecord's read-only display — every
// zone the reviewer might see, live or pending.
const allZones = computed(() => [...(pg.live?.zones ?? []), ...(pg.changes?.zones ?? [])])

function rowFor(entry: ChangeEntry): KbRow | undefined {
  return entry.type === 'removed' ? entry.liveRow : entry.draftRow
}
function isBusy(entry: ChangeEntry): boolean {
  return pg.busy || pg.publishingKey === `${props.kind}:${entry.key}`
}
function edit(entry: ChangeEntry) {
  const row = rowFor(entry)
  if (row) modal.openEdit(props.kind, row)
}
function publish(entry: ChangeEntry) {
  pg.approveEntity(props.kind, entry.key)
}
function cancel(entry: ChangeEntry) {
  pg.cancelChange(props.kind, entry.key)
}
</script>

<template>
  <div class="space-y-3">
    <template v-for="entry in entries" :key="entry.key">
      <component
        :is="component"
        v-if="component && rowFor(entry)"
        :row="rowFor(entry)"
        :live-row="entry.liveRow"
        :change-type="entry.type"
        :actions="kbActions({ page: 'draft', changeType: entry.type })"
        :busy="isBusy(entry)"
        :all-zones="kind === 'delivery_zones' ? allZones : undefined"
        @edit="edit(entry)"
        @publish="publish(entry)"
        @cancel="cancel(entry)"
      />
      <!-- A staged removal whose published record already vanished (approved
           elsewhere between loads) — a minimal fallback card, never claiming
           to show the record itself. -->
      <div v-else class="rounded-lg border border-border bg-card p-4 space-y-2">
        <div class="flex items-center gap-2 flex-wrap">
          <span class="text-xs font-medium text-muted-foreground">{{ kindLabel }}</span>
          <code class="text-[13px] font-mono font-medium">{{ entry.key }}</code>
          <span class="rounded-full bg-destructive/10 text-destructive text-[11px] font-medium px-2 py-0.5">{{ t('kb.stats.removed') }}</span>
        </div>
        <p class="text-xs text-muted-foreground">{{ t('kb.draft.removedFallbackTitle') }}</p>
        <div class="flex items-center gap-2 flex-wrap">
          <button
            v-for="a in kbActions({ page: 'draft', changeType: 'removed' })"
            :key="a.key"
            type="button"
            class="inline-flex items-center rounded-md px-3 py-1.5 text-[13px] font-medium border border-border hover:bg-muted"
            :class="a.destructive ? 'text-destructive' : ''"
            :disabled="isBusy(entry)"
            @click="a.key === 'publish' ? publish(entry) : cancel(entry)"
          >
            {{ t(a.labelKey) }}
          </button>
        </div>
      </div>
    </template>
  </div>
</template>
