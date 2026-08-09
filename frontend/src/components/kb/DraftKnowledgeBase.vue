<script setup lang="ts">
// DraftKnowledgeBase is Черновик (/playground) — a review-only surface
// answering exactly one question: what unpublished changes will affect the
// knowledge base? No Add buttons, no create forms (decision 1); every
// change here originated on /knowledge-base and only gets published,
// edited in place, or cancelled from this page.
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Blocks, CircleAlert, LoaderCircle, Save } from 'lucide-vue-next'
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
import McpConnectCard from './McpConnectCard.vue'
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

// The AI-connection panel starts open (nothing to review yet is the most
// common first-run state) and collapses itself the FIRST time real data
// shows a queue waiting — so the review list gets the top of the page, not
// the "how do I connect an AI" instructions. A manual toggle afterwards is
// never fought.
const showConnect = ref(true)
onMounted(async () => {
  await Promise.all([pg.load(), pg.loadLive()])
  showConnect.value = isEmpty.value
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
  <div class="flex h-full min-w-0 flex-col bg-background">
    <header class="shrink-0 border-b border-border bg-card px-4 py-5 sm:px-6 lg:px-8">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div class="min-w-0">
          <div class="flex items-center gap-2.5">
            <div class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
              <Blocks class="h-4 w-4" />
            </div>
            <h1 class="text-xl font-semibold tracking-tight text-foreground">{{ t('kb.draft.pageTitle') }}</h1>
          </div>
          <p class="mt-1.5 text-sm text-muted-foreground">{{ t('kb.draft.pageSubtitle') }}</p>
        </div>
        <div v-if="!isEmpty" class="flex flex-wrap items-center justify-end gap-2">
          <Button variant="ghost" size="sm" :disabled="pg.busy || pg.approving" @click="discardAll">{{ t('kb.draft.discardAll') }}</Button>
          <Button size="sm" :disabled="pg.busy || pg.approving" @click="publishAll">
            <LoaderCircle v-if="pg.approving && !pg.publishingKey" class="h-4 w-4 animate-spin" />
            <Save v-else class="h-4 w-4" />
            {{ t('kb.draft.publishAll') }}<span v-if="counts.total"> · {{ counts.total }}</span>
          </Button>
        </div>
      </div>
    </header>

    <div class="flex flex-1 flex-col overflow-y-auto">
      <div class="shrink-0 px-4 pt-6 sm:px-6 lg:px-8">
        <McpConnectCard v-model:open="showConnect" />
      </div>

      <div v-if="loading && !pg.changes" class="grid flex-1 place-items-center p-8">
        <div class="max-w-sm text-center">
          <div class="mx-auto mb-3 grid h-12 w-12 place-items-center rounded-xl bg-primary/10 text-primary">
            <Blocks class="h-6 w-6" />
          </div>
          <p class="text-sm text-muted-foreground">{{ t('kb.draft.loading') }}</p>
        </div>
      </div>

      <DraftEmptyState v-else-if="isEmpty" />

      <div v-else class="px-4 py-6 sm:px-6 lg:px-8">
        <div class="space-y-6">
          <StatTiles :counts="counts" />
          <EntityTabs :tabs="tabs" :active="active" @update:active="(k) => (active = k)" />

          <div v-show="active === 'config'" class="max-w-3xl space-y-3">
            <ConfigChangeGroup />
          </div>

          <div v-show="active === 'topics'" class="grid grid-cols-1 items-start gap-4 xl:grid-cols-2">
            <ChangeList kind="topics" />
          </div>
          <div v-show="active === 'products'" class="grid grid-cols-1 items-start gap-4 xl:grid-cols-2">
            <ChangeList kind="products" />
          </div>
          <div v-show="active === 'tariffs'" class="grid grid-cols-1 items-start gap-4 xl:grid-cols-2">
            <ChangeList kind="tariffs" />
          </div>

          <div v-show="active === 'delivery_zones'" class="grid grid-cols-1 items-start gap-4 xl:grid-cols-2">
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

          <div v-show="active === 'contacts'" class="max-w-2xl">
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

          <div v-show="active === 'policies'" class="max-w-2xl">
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

          <p v-if="pg.gateReasons" class="flex items-start gap-2 rounded-lg bg-destructive/10 p-3 text-sm text-destructive">
            <CircleAlert class="mt-0.5 h-4 w-4 shrink-0" /> {{ t('kb.draft.gateBlocked') }} {{ pg.gateReasons }}
          </p>
          <p v-else-if="pg.error" class="flex items-center gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5 text-sm text-destructive">
            <CircleAlert class="h-4 w-4 shrink-0" /> {{ pg.error }}
          </p>
        </div>
      </div>
    </div>

    <KbModalForms />
  </div>
</template>
