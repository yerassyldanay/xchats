<script setup lang="ts">
// DraftKnowledgeBase is Черновик (/playground) — model-driven ingest (the
// ingestion panel: submit a URL/file to the structured import pipeline, or
// connect ChatGPT/Claude over MCP) PLUS review of whatever ends up staged,
// from either source. This refines, rather than reverses, the 2026-08-03
// decision that made this page review-only: /knowledge-base remains the
// sole MANUAL authoring surface — no Add buttons or record-create forms
// live here, only what a model proposed gets edited in place, published,
// or cancelled from this page.
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, LoaderCircle, Save, WandSparkles } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { useDraftChanges } from '@/composables/useDraftChanges'
import { useEntityTabs } from '@/composables/useEntityTabs'
import { useKbModal } from '@/composables/useKbModal'
import { useDraftSelection, type SelectionTarget } from '@/composables/useDraftSelection'
import { useCancelConfirm } from '@/composables/useCancelConfirm'
import type { ChangeEntry, ChangeKind } from '@/composables/draftChanges'
import type { ContactRow, DeliveryZoneRow, PolicyRow } from '@/types'
import { Button } from '@/components/ui/button'
import EntityTabs from './EntityTabs.vue'
import StatTiles from './StatTiles.vue'
import ChangeList from './ChangeList.vue'
import ConfigChangeGroup from './ConfigChangeGroup.vue'
import DraftEmptyState from './DraftEmptyState.vue'
import KbIngestPanel from './KbIngestPanel.vue'
import DeliveryZoneRecord from './records/DeliveryZoneRecord.vue'
import ContactsRecord from './records/ContactsRecord.vue'
import PoliciesRecord from './records/PoliciesRecord.vue'
import { kbActions } from './records/actions'
import KbModalForms from './forms/KbModalForms.vue'
import ConfirmDeleteDialog from './forms/ConfirmDeleteDialog.vue'

const pg = usePlayground()
const { t } = useI18n()
const { loading, counts, entriesFor, isEmpty } = useDraftChanges()
const { tabs, active } = useEntityTabs({ source: 'draft' })
const modal = useKbModal()
const selection = useDraftSelection()
const cancelConfirm = useCancelConfirm()

onMounted(async () => {
  await Promise.all([pg.load(), pg.loadLive()])
  pg.startRealtime()
})
onBeforeUnmount(() => {
  pg.stopRealtime()
  // The selection is a module-level singleton (so a card deep in ChangeList
  // and the bulk bar up here share it), which means it would otherwise
  // outlive this page and reappear on the next visit.
  selection.clear()
})

// --- multi-select -------------------------------------------------------

// Every selectable entry currently in the draft, across all kinds — the
// bulk bar deliberately spans tabs (rejecting three products and a tariff
// is one review decision, not two). Singletons (contacts/policies) are
// excluded: they render exactly one card on a tab of their own, so a
// checkbox there is noise, and config has its own «Отменить все изменения
// ассистента» already.
const SELECTABLE_KINDS: ChangeKind[] = ['topics', 'products', 'tariffs', 'delivery_zones']
const selectableTargets = computed<SelectionTarget[]>(() =>
  SELECTABLE_KINDS.flatMap((kind) => entriesFor(kind).map((e) => ({ kind, key: e.key })))
)
// Only the entries on the tab currently being looked at — what «Выбрать
// все» acts on, so it never silently ticks cards the operator can't see.
const visibleTargets = computed<SelectionTarget[]>(() =>
  SELECTABLE_KINDS.includes(active.value as ChangeKind)
    ? entriesFor(active.value as ChangeKind).map((e) => ({ kind: active.value as ChangeKind, key: e.key }))
    : []
)
const allVisibleSelected = computed(() => selection.allSelected(visibleTargets.value))

function toggleSelectAll() {
  selection.setMany(visibleTargets.value, !allVisibleSelected.value)
}

// Drop selected keys the backend no longer has. pg.changes is replaced
// wholesale on every publish/cancel/SSE refresh, so this is the one place
// that keeps the bulk bar's count honest.
watch(selectableTargets, (live) => selection.prune(live))

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

// --- cancel confirmation copy -------------------------------------------

// The dialog says what THIS cancel actually does, because that is entirely
// determined by the card's state: an `added` entry is destroyed (nothing to
// revert to), an `updated` one goes back to the published value, and a
// `removed` one is KEPT. A bulk request spans states, so it falls back to a
// plain count.
const confirmTitleKey = computed(() => {
  const req = cancelConfirm.pending.value
  if (!req) return 'kb.draft.cancelConfirm.titleUpdated'
  if (req.targets.length > 1) return 'kb.draft.cancelConfirm.titleBulk'
  return req.changeType === 'added' ? 'kb.draft.cancelConfirm.titleAdded' : 'kb.draft.cancelConfirm.titleUpdated'
})
const confirmBodyKey = computed(() => {
  const req = cancelConfirm.pending.value
  if (!req) return 'kb.draft.cancelConfirm.bodyUpdated'
  if (req.targets.length > 1) return 'kb.draft.cancelConfirm.bodyBulk'
  if (req.changeType === 'added') return 'kb.draft.cancelConfirm.bodyAdded'
  if (req.changeType === 'removed') return 'kb.draft.cancelConfirm.bodyRemoved'
  return 'kb.draft.cancelConfirm.bodyUpdated'
})
const confirmBodyParams = computed(() => {
  const req = cancelConfirm.pending.value
  return req && req.targets.length > 1 ? { count: req.targets.length } : undefined
})
const confirmAcceptKey = computed(() => {
  const req = cancelConfirm.pending.value
  return req && req.targets.length === 1 && req.changeType === 'added'
    ? 'kb.actions.removeFromDraft'
    : 'kb.draft.cancelConfirm.accept'
})
</script>

<template>
  <div class="h-full bg-background flex flex-col min-w-0">
    <header class="px-8 py-4 flex items-center justify-between gap-4 border-b border-border bg-card shrink-0">
      <div class="min-w-0">
        <h1 class="text-lg font-bold tracking-tight">{{ t('kb.draft.pageTitle') }}</h1>
        <p class="text-sm text-muted-foreground">{{ t('kb.draft.pageSubtitle') }}</p>
      </div>
      <div v-if="!isEmpty" class="flex items-center gap-2 shrink-0">
        <Button variant="ghost" size="sm" :disabled="pg.busy || pg.approving" data-testid="discard-all" @click="discardAll">{{ t('kb.draft.discardAll') }}</Button>
        <Button size="sm" :disabled="pg.busy || pg.approving" data-testid="publish-all" @click="publishAll">
          <LoaderCircle v-if="pg.approving && !pg.publishingKey" class="w-4 h-4 animate-spin" />
          <Save v-else class="w-4 h-4" />
          {{ t('kb.draft.publishAll') }}<span v-if="counts.total"> · {{ counts.total }}</span>
        </Button>
      </div>
    </header>

    <div class="flex-1 overflow-y-auto flex flex-col">
      <div class="px-8 pt-6 shrink-0">
        <KbIngestPanel />
      </div>

      <div v-if="loading && !pg.changes" class="flex-1 grid place-items-center p-8">
        <div class="text-center max-w-sm">
          <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
            <WandSparkles class="w-6 h-6" />
          </div>
          <p class="text-sm text-muted-foreground">{{ t('kb.draft.loading') }}</p>
        </div>
      </div>

      <!-- Review region: always rendered once loaded, regardless of whether
           anything is pending — StatTiles reads all zeros rather than
           vanishing, and DraftEmptyState now stands in for the tabs/change
           lists only, not for this whole region (the ingestion panel above
           must stay reachable even with an empty draft). -->
      <div v-else class="px-8 py-6 space-y-6">
        <div>
          <h2 class="text-sm font-semibold text-muted-foreground">{{ t('kb.draft.reviewHeading') }}</h2>
        </div>

        <StatTiles :counts="counts" />

        <DraftEmptyState v-if="isEmpty" />
        <template v-else>
          <EntityTabs :tabs="tabs" :active="active" @update:active="(k) => (active = k)" />

          <!-- Bulk bar: only once something is ticked, so the default review
               view stays exactly as it was. Counts across tabs on purpose. -->
          <div
            v-if="!selection.isEmpty.value"
            class="flex items-center gap-3 flex-wrap rounded-lg border border-primary/40 bg-primary/5 px-4 py-2.5"
            data-testid="draft-bulk-bar"
          >
            <span class="text-sm font-medium">{{ t('kb.draft.selection.selected', { count: selection.count.value }) }}</span>
            <Button
              variant="ghost"
              size="sm"
              class="text-destructive"
              :disabled="pg.busy || pg.approving"
              data-testid="bulk-cancel"
              @click="cancelConfirm.requestSelected()"
            >
              {{ t('kb.draft.selection.cancelSelected') }}
            </Button>
            <Button variant="ghost" size="sm" class="ml-auto" data-testid="bulk-clear" @click="selection.clear()">
              {{ t('kb.draft.selection.clear') }}
            </Button>
          </div>
          <div v-if="visibleTargets.length" class="flex">
            <Button variant="ghost" size="sm" data-testid="bulk-select-all" @click="toggleSelectAll">
              {{ allVisibleSelected ? t('kb.draft.selection.deselectAll') : t('kb.draft.selection.selectAll') }}
            </Button>
          </div>

          <div v-show="active === 'config'" class="space-y-3 max-w-3xl" data-testid="draft-tab-config">
            <ConfigChangeGroup />
          </div>

          <div v-show="active === 'topics'" class="space-y-3" data-testid="draft-tab-topics">
            <ChangeList kind="topics" />
          </div>
          <div v-show="active === 'products'" class="space-y-3" data-testid="draft-tab-products">
            <ChangeList kind="products" />
          </div>
          <div v-show="active === 'tariffs'" class="space-y-3" data-testid="draft-tab-tariffs">
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
              selectable
              :selected="selection.isSelected('delivery_zones', entry.key)"
              @edit="editEntry('delivery_zones', entry)"
              @publish="pg.approveEntity('delivery_zones', entry.key)"
              @cancel="cancelConfirm.requestOne('delivery_zones', entry.key, entry.type)"
              @toggle-select="selection.toggle('delivery_zones', entry.key)"
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
              @cancel="cancelConfirm.requestOne('contacts', contactEntry.key, contactEntry.type)"
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
              @cancel="cancelConfirm.requestOne('policies', policyEntry.key, policyEntry.type)"
            />
          </div>
        </template>

        <p v-if="pg.gateReasons" class="flex items-start gap-2 text-sm text-destructive rounded-lg bg-destructive/10 p-3">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ t('kb.draft.gateBlocked') }} {{ pg.gateReasons }}
        </p>
        <p v-else-if="pg.error" class="flex items-center gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0" /> {{ pg.error }}
        </p>
      </div>
    </div>

    <KbModalForms />

    <ConfirmDeleteDialog
      :open="cancelConfirm.isOpen.value"
      :busy="cancelConfirm.busy.value"
      :title-key="confirmTitleKey"
      :body-key="confirmBodyKey"
      :body-params="confirmBodyParams"
      :confirm-key="confirmAcceptKey"
      @update:open="(v) => !v && cancelConfirm.close()"
      @confirm="cancelConfirm.confirm()"
    />
  </div>
</template>
