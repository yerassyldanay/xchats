<script setup lang="ts">
// DraftKnowledgeBase is Черновик (/playground) — a review-only surface
// answering exactly one question: what unpublished changes will affect the
// knowledge base? No Add buttons, no create forms (decision 1); every
// change here originated on /knowledge-base and only gets published,
// edited in place, or cancelled from this page.
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, LoaderCircle, Save, WandSparkles } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { useDraftChanges } from '@/composables/useDraftChanges'
import { useEntityTabs } from '@/composables/useEntityTabs'
import { useKbModal } from '@/composables/useKbModal'
import type { ChangeEntry } from '@/composables/draftChanges'
import type { ContactRow, DeliveryZoneRow, PolicyRow } from '@/types'
import { Button } from '@/components/ui/button'
import EntityTabs from './EntityTabs.vue'
import StatTiles from './StatTiles.vue'
import ChangeList from './ChangeList.vue'
import ConfigChangeGroup from './ConfigChangeGroup.vue'
import DraftEmptyState from './DraftEmptyState.vue'
import DeliveryZoneRecord from './records/DeliveryZoneRecord.vue'
import ContactsRecord from './records/ContactsRecord.vue'
import PoliciesRecord from './records/PoliciesRecord.vue'
import { kbActions } from './records/actions'
import KbModalForms from './forms/KbModalForms.vue'

const pg = usePlayground()
const { t } = useI18n()
const { loading, counts, entriesFor, isEmpty } = useDraftChanges()
const { tabs, active } = useEntityTabs({ source: 'draft' })
const modal = useKbModal()

onMounted(async () => {
  await Promise.all([pg.load(), pg.loadLive()])
  pg.startRealtime()
})
onBeforeUnmount(() => pg.stopRealtime())

async function publishAll() {
  await pg.approve()
}
async function discardAll() {
  if (pg.pendingTotal && window.confirm(t('kb.draft.discardConfirm'))) await pg.discard()
}

function isBusy(kind: string, key: string) {
  return pg.busy || pg.publishingKey === `${kind}:${key}`
}
function blockedNote(kind: string, key: string) {
  return pg.gateBlockedKey === `${kind}:${key}` ? t('kb.draft.cardBlockedNote') : undefined
}
function editEntry(kind: 'delivery_zones' | 'contacts' | 'policies', entry: ChangeEntry) {
  const row = entry.type === 'removed' ? entry.liveRow : entry.draftRow
  if (row) modal.openEdit(kind, row)
}

// delivery_zones/contacts/policies need extra props ChangeList's generic
// three (topics/products/tariffs) don't — allZones for the parent picker,
// zonesExist for the delivery-answer-is-governed-elsewhere hint — so they
// render directly here rather than through ChangeList.
const zoneEntries = computed(() => entriesFor('delivery_zones'))
const allZonesForDisplay = computed(() => [...(pg.live?.zones ?? []), ...(pg.changes?.zones ?? [])])
const contactEntry = computed(() => entriesFor('contacts')[0])
const policyEntry = computed(() => entriesFor('policies')[0])
const zonesExist = computed(() => (pg.live?.zones.length ?? 0) > 0)

// entriesFor(kind) is runtime-guaranteed to return only entries of that
// kind, but ChangeEntry's draftRow/liveRow are typed against the broad
// KbRow union — these narrow to the specific row type each static
// (non-dynamic-:is) component below actually declares.
function zoneRowOf(entry: ChangeEntry) {
  return (entry.type === 'removed' ? entry.liveRow : entry.draftRow) as DeliveryZoneRow
}
function contactRowOf(entry: ChangeEntry) {
  return (entry.type === 'removed' ? entry.liveRow : entry.draftRow) as ContactRow | undefined
}
function policyRowOf(entry: ChangeEntry) {
  return (entry.type === 'removed' ? entry.liveRow : entry.draftRow) as PolicyRow | undefined
}
</script>

<template>
  <div class="h-full bg-background flex flex-col min-w-0">
    <header class="px-8 py-4 flex items-center justify-between gap-4 border-b border-border bg-card shrink-0">
      <div class="min-w-0">
        <h1 class="text-lg font-bold tracking-tight">{{ t('kb.draft.pageTitle') }}</h1>
        <p class="text-sm text-muted-foreground">{{ t('kb.draft.pageSubtitle') }}</p>
      </div>
      <div v-if="!isEmpty" class="flex items-center gap-2 shrink-0">
        <Button variant="ghost" size="sm" :disabled="pg.busy || pg.approving" @click="discardAll">{{ t('kb.draft.discardAll') }}</Button>
        <Button size="sm" :disabled="pg.busy || pg.approving" @click="publishAll">
          <LoaderCircle v-if="pg.approving && !pg.publishingKey" class="w-4 h-4 animate-spin" />
          <Save v-else class="w-4 h-4" />
          {{ t('kb.draft.publishAll') }}<span v-if="counts.total"> · {{ counts.total }}</span>
        </Button>
      </div>
    </header>

    <div v-if="loading && !pg.changes" class="flex-1 grid place-items-center p-8">
      <div class="text-center max-w-sm">
        <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
          <WandSparkles class="w-6 h-6" />
        </div>
        <p class="text-sm text-muted-foreground">{{ t('kb.draft.loading') }}</p>
      </div>
    </div>

    <DraftEmptyState v-else-if="isEmpty" />

    <div v-else class="flex-1 overflow-y-auto px-8 py-6 space-y-6">
      <StatTiles :counts="counts" />
      <EntityTabs :tabs="tabs" :active="active" @update:active="(k) => (active = k)" />

      <div v-show="active === 'config'" class="space-y-3 max-w-3xl">
        <ConfigChangeGroup />
      </div>

      <div v-show="active === 'topics'" class="space-y-3">
        <ChangeList kind="topics" />
      </div>
      <div v-show="active === 'products'" class="space-y-3">
        <ChangeList kind="products" />
      </div>
      <div v-show="active === 'tariffs'" class="space-y-3">
        <ChangeList kind="tariffs" />
      </div>

      <div v-show="active === 'delivery_zones'" class="space-y-3">
        <DeliveryZoneRecord
          v-for="entry in zoneEntries"
          :key="entry.key"
          :row="zoneRowOf(entry)"
          :live-row="entry.type === 'removed' ? undefined : (entry.liveRow as DeliveryZoneRow | undefined)"
          :change-type="entry.type"
          :all-zones="allZonesForDisplay"
          :actions="kbActions({ page: 'draft', changeType: entry.type })"
          :busy="isBusy('delivery_zones', entry.key)"
          :blocked-note="blockedNote('delivery_zones', entry.key)"
          @edit="editEntry('delivery_zones', entry)"
          @publish="pg.approveEntity('delivery_zones', entry.key)"
          @cancel="pg.cancelChange('delivery_zones', entry.key)"
        />
      </div>

      <div v-show="active === 'contacts'" class="space-y-3 max-w-2xl">
        <ContactsRecord
          v-if="contactEntry"
          :row="contactRowOf(contactEntry)"
          :live-row="contactEntry.type === 'removed' ? undefined : (contactEntry.liveRow as ContactRow | undefined)"
          :change-type="contactEntry.type"
          :actions="kbActions({ page: 'draft', changeType: contactEntry.type, singleton: true })"
          :busy="isBusy('contacts', contactEntry.key)"
          :blocked-note="blockedNote('contacts', contactEntry.key)"
          @edit="editEntry('contacts', contactEntry)"
          @publish="pg.approveEntity('contacts', contactEntry.key)"
          @cancel="pg.cancelChange('contacts', contactEntry.key)"
        />
      </div>

      <div v-show="active === 'policies'" class="space-y-3 max-w-2xl">
        <PoliciesRecord
          v-if="policyEntry"
          :row="policyRowOf(policyEntry)"
          :live-row="policyEntry.type === 'removed' ? undefined : (policyEntry.liveRow as PolicyRow | undefined)"
          :change-type="policyEntry.type"
          :zones-exist="zonesExist"
          :actions="kbActions({ page: 'draft', changeType: policyEntry.type, singleton: true })"
          :busy="isBusy('policies', policyEntry.key)"
          :blocked-note="blockedNote('policies', policyEntry.key)"
          @edit="editEntry('policies', policyEntry)"
          @publish="pg.approveEntity('policies', policyEntry.key)"
          @cancel="pg.cancelChange('policies', policyEntry.key)"
        />
      </div>

      <p v-if="pg.gateReasons" class="flex items-start gap-2 text-sm text-destructive rounded-lg bg-destructive/10 p-3">
        <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ t('kb.draft.gateBlocked') }} {{ pg.gateReasons }}
      </p>
      <p v-else-if="pg.error" class="flex items-center gap-2 text-sm text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0" /> {{ pg.error }}
      </p>
    </div>

    <KbModalForms />
  </div>
</template>
