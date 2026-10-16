<script setup lang="ts">
// DraftKnowledgeBase is Черновик (/draft) — model-driven ingest (the
// ingestion panel: submit a URL/file to the structured import pipeline, or
// connect ChatGPT/Claude over MCP) PLUS review of whatever ends up staged,
// from either source. This refines, rather than reverses, the 2026-08-03
// decision that made this page review-only: /knowledge-base remains the
// sole MANUAL authoring surface — no Add buttons or record-create forms
// live here, only what a model proposed gets edited in place, published,
// or cancelled from this page.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, CircleCheck, LoaderCircle, WandSparkles, X } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { useDraftChanges } from '@/composables/useDraftChanges'
import { useEntityTabs } from '@/composables/useEntityTabs'
import { useKbModal } from '@/composables/useKbModal'
import { useDraftSelection, type SelectionTarget } from '@/composables/useDraftSelection'
import { useCancelConfirm } from '@/composables/useCancelConfirm'
import type { ChangeEntry, ChangeKind } from '@/composables/draftChanges'
import type { ContactRow, DeliveryZoneRow, PolicyRow, TariffInfoRow } from '@/types'
import { RouterLink } from 'vue-router'
import { Button } from '@/components/ui/button'
import EntityTabs from './EntityTabs.vue'
import ChangeList from './ChangeList.vue'
import ConfigChangeGroup from './ConfigChangeGroup.vue'
import DraftEmptyState from './DraftEmptyState.vue'
import KbIngestPanel from './KbIngestPanel.vue'
import DeliveryZoneRecord from './records/DeliveryZoneRecord.vue'
import ContactsRecord from './records/ContactsRecord.vue'
import PoliciesRecord from './records/PoliciesRecord.vue'
import TariffInfoRecord from './records/TariffInfoRecord.vue'
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

// KB-03: once a publish empties the draft, bridge the operator straight to
// testing the result instead of leaving them at a bare empty state with no
// next step. approveWith (both approve() and every approveEntity() call —
// ChangeList.vue's own publish() among them — funnel through it) is the one
// choke point every publish path shares, so watching ITS OWN approving flag
// catches all of them uniformly without wrapping each card's @publish
// handler individually. Discard never touches `approving`, so a
// discard-emptied draft correctly never triggers this — there is nothing
// new published to go test.
const justPublished = ref(false)
watch(
  () => pg.approving,
  (approving, wasApproving) => {
    if (wasApproving && !approving && !pg.error && !pg.gateReasons && isEmpty.value) {
      justPublished.value = true
    }
  }
)

// --- multi-select -------------------------------------------------------

// Every selectable entry currently in the draft, across all kinds — the
// bulk bar deliberately spans tabs (rejecting three products and a tariff
// is one review decision, not two). Singletons (contacts/policies) are
// excluded: they render exactly one card on a tab of their own, so a
// checkbox there is noise, and config has its own «Отменить все изменения
// ассистента» already.
const SELECTABLE_KINDS: ChangeKind[] = ['topics', 'products', 'tariffs', 'specialists', 'services', 'delivery_zones']
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

// «Темы · 3» above the active tab's cards — the same heading /knowledge-base
// puts over its lists, counting what is pending on this tab.
const sectionTitle = computed(() => {
  const label = tabs.value.find((tb) => tb.key === active.value)?.label ?? ''
  return SELECTABLE_KINDS.includes(active.value as ChangeKind) ? `${label} · ${visibleTargets.value.length}` : label
})

function toggleSelectAll() {
  selection.setMany(visibleTargets.value, !allVisibleSelected.value)
}

// Drop selected keys the backend no longer has. pg.changes is replaced
// wholesale on every publish/cancel/SSE refresh, so this is the one place
// that keeps the bulk bar's count honest.
watch(selectableTargets, (live) => selection.prune(live))

// KB-08: Publish all commits every pending change straight to live channels
// and Discard all destroys every pending change — both irreversible, both
// now gated behind the app's own styled confirmation dialog (ConfirmDeleteDialog,
// reused) instead of firing immediately (Publish all) or falling back to a
// bare window.confirm (Discard all) that looked out of place next to every
// other styled dialog in this app.
const publishConfirmOpen = ref(false)
const discardConfirmOpen = ref(false)

function requestPublishAll() {
  if (counts.value.total) publishConfirmOpen.value = true
}
async function confirmPublishAll() {
  publishConfirmOpen.value = false
  await pg.approve()
}
function requestDiscardAll() {
  if (pg.pendingTotal) discardConfirmOpen.value = true
}
async function confirmDiscardAll() {
  discardConfirmOpen.value = false
  await pg.discard()
}

function isBusy(kind: string, key: string) {
  return pg.busy || pg.publishingKey === `${kind}:${key}`
}
function blockedNote(kind: string, key: string) {
  return pg.gateBlockedKey === `${kind}:${key}` ? t('kb.draft.cardBlockedNote') : undefined
}

// KB-09: a gate reason's own kind is already the tab's key (ChangeKind) —
// tabs is dynamic (useEntityTabs' own doc comment: only kinds with
// pending entries), which the offending entity's tab always is, since
// that pending entry is exactly what tripped the gate.
function entityTabLabel(kind: string): string {
  return tabs.value.find((tb) => tb.key === kind)?.label || kind
}
function goToTab(kind: string) {
  active.value = kind
}
function editEntry(kind: 'delivery_zones' | 'contacts' | 'policies' | 'tariff_info', entry: ChangeEntry) {
  const row = entry.type === 'removed' ? entry.liveRow : entry.draftRow
  if (row) modal.openEdit(kind, row)
}

// delivery_zones/contacts/policies/tariff_info need extra props ChangeList's
// generic three (topics/products/tariffs) don't — allZones for the parent
// picker, zonesExist for the delivery-answer-is-governed-elsewhere hint (or,
// for the singletons, simply no natural key to show) — so they render
// directly here rather than through ChangeList.
const zoneEntries = computed(() => entriesFor('delivery_zones'))
const allZonesForDisplay = computed(() => [...(pg.live?.zones ?? []), ...(pg.changes?.zones ?? [])])
const contactEntry = computed(() => entriesFor('contacts')[0])
const policyEntry = computed(() => entriesFor('policies')[0])
const tariffInfoEntry = computed(() => entriesFor('tariff_info')[0])
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
function tariffInfoRowOf(entry: ChangeEntry) {
  return (entry.type === 'removed' ? entry.liveRow : entry.draftRow) as TariffInfoRow | undefined
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
    <header class="flex shrink-0 flex-wrap items-start justify-between gap-4 px-10 pb-5 pt-10">
      <div>
        <h1 class="mb-1.5 text-[32px] font-extrabold leading-tight tracking-tight">{{ t('kb.draft.pageTitle') }}</h1>
        <p class="text-base text-muted-foreground">{{ t('kb.draft.pageSubtitle') }}</p>
      </div>
      <Button as-child variant="outline" size="lg" class="px-5 text-[15px]" data-testid="draft-back-to-kb">
        <RouterLink :to="{ name: 'knowledge-base' }">{{ t('nav.knowledgeBase') }}</RouterLink>
      </Button>
    </header>

    <div class="flex-1 space-y-6 overflow-y-auto px-10 pb-16">
      <!-- Summary strip, same shape as the one /knowledge-base shows above its
           tabs: how much is waiting, plus the two bulk actions. -->
      <section
        v-if="pg.changes && !isEmpty"
        class="flex flex-wrap items-center justify-between gap-x-6 gap-y-3 rounded-2xl bg-primary-soft px-6 py-4 text-base"
        data-testid="draft-summary"
        role="status"
      >
        <p>
          <b class="font-bold">{{ t('kb.draft.summary.title', counts.total) }}</b>
          <span class="text-muted-foreground">
            · {{ t('kb.draft.summary.breakdown', { added: counts.added, updated: counts.updated, removed: counts.removed }) }}
          </span>
        </p>
        <div class="flex items-center gap-5">
          <button
            type="button"
            class="font-bold underline underline-offset-4 hover:text-destructive disabled:opacity-50"
            :disabled="pg.busy || pg.approving"
            data-testid="discard-all"
            @click="requestDiscardAll"
          >
            {{ t('kb.draft.discardAll') }}
          </button>
          <Button size="lg" class="px-6" :disabled="pg.busy || pg.approving" data-testid="publish-all" @click="requestPublishAll">
            <LoaderCircle v-if="pg.approving && !pg.publishingKey" class="w-4 h-4 animate-spin" />
            {{ t('kb.draft.publishAll') }}
          </Button>
        </div>
      </section>

      <div v-if="loading && !pg.changes" class="grid place-items-center p-8">
        <div class="text-center max-w-sm">
          <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
            <WandSparkles class="w-6 h-6" />
          </div>
          <p class="text-sm text-muted-foreground">{{ t('kb.draft.loading') }}</p>
        </div>
      </div>

      <!-- Review region: rendered once loaded. DraftEmptyState stands in for
           the tabs/change lists only, never for this whole region (the
           ingestion panel below must stay reachable with an empty draft). -->
      <div v-else class="space-y-6">
        <div
          v-if="justPublished && isEmpty"
          class="flex items-center gap-3 rounded-2xl bg-success-soft px-5 py-4 text-sm text-success"
          data-testid="publish-success-banner"
        >
          <CircleCheck class="w-4 h-4 shrink-0 text-wa" />
          <span class="flex-1">{{ t('kb.draft.publishedBanner.message') }}</span>
          <RouterLink :to="{ name: 'simulator' }" class="font-medium text-primary hover:underline shrink-0">
            {{ t('kb.draft.publishedBanner.testInSimulator') }}
          </RouterLink>
          <RouterLink :to="{ name: 'knowledge-base' }" class="font-medium text-primary hover:underline shrink-0">
            {{ t('kb.draft.publishedBanner.viewLive') }}
          </RouterLink>
          <button
            type="button"
            class="text-muted-foreground hover:text-foreground shrink-0"
            :aria-label="t('common.close')"
            data-testid="publish-success-dismiss"
            @click="justPublished = false"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <DraftEmptyState v-if="isEmpty" />
        <template v-else>
          <EntityTabs :tabs="tabs" :active="active" @update:active="(k) => (active = k)" />

          <div class="flex flex-wrap items-center justify-between gap-3" data-testid="draft-section-header">
            <h2 class="text-2xl font-extrabold tracking-tight">{{ sectionTitle }}</h2>
            <Button v-if="visibleTargets.length" variant="outline" data-testid="bulk-select-all" @click="toggleSelectAll">
              {{ allVisibleSelected ? t('kb.draft.selection.deselectAll') : t('kb.draft.selection.selectAll') }}
            </Button>
          </div>

          <!-- Bulk bar: only once something is ticked, so the default review
               view stays exactly as it was. Counts across tabs on purpose. -->
          <div
            v-if="!selection.isEmpty.value"
            class="flex items-center gap-3 flex-wrap rounded-2xl border border-primary/40 bg-primary-soft px-5 py-3"
            data-testid="draft-bulk-bar"
          >
            <span class="text-sm font-semibold">{{ t('kb.draft.selection.selected', { count: selection.count.value }) }}</span>
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

          <div v-show="active === 'config'" class="space-y-3 max-w-3xl" data-testid="draft-tab-config">
            <ConfigChangeGroup />
          </div>

          <div v-show="active === 'topics'" class="space-y-3" data-testid="draft-tab-topics">
            <ChangeList kind="topics" />
          </div>
          <div v-show="active === 'products'" class="grid gap-x-5 gap-y-8 [grid-template-columns:repeat(auto-fill,minmax(240px,1fr))]" data-testid="draft-tab-products">
            <ChangeList kind="products" />
          </div>
          <div v-show="active === 'tariffs'" class="space-y-3" data-testid="draft-tab-tariffs">
            <ChangeList kind="tariffs" />
          </div>
          <div v-show="active === 'specialists'" class="space-y-3" data-testid="draft-tab-specialists">
            <ChangeList kind="specialists" />
          </div>
          <div v-show="active === 'services'" class="space-y-3" data-testid="draft-tab-services">
            <ChangeList kind="services" />
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

          <div v-show="active === 'tariff_info'" class="space-y-3 max-w-2xl">
            <TariffInfoRecord
              v-if="tariffInfoEntry"
              :row="tariffInfoRowOf(tariffInfoEntry)"
              :live-row="tariffInfoEntry.type === 'removed' ? undefined : (tariffInfoEntry.liveRow as TariffInfoRow | undefined)"
              :change-type="tariffInfoEntry.type"
              :actions="kbActions({ page: 'draft', changeType: tariffInfoEntry.type, singleton: true })"
              :busy="isBusy('tariff_info', tariffInfoEntry.key)"
              :blocked-note="blockedNote('tariff_info', tariffInfoEntry.key)"
              @edit="editEntry('tariff_info', tariffInfoEntry)"
              @publish="pg.approveEntity('tariff_info', tariffInfoEntry.key)"
              @cancel="cancelConfirm.requestOne('tariff_info', tariffInfoEntry.key, tariffInfoEntry.type)"
            />
          </div>
        </template>

        <div v-if="pg.gateReasons" class="rounded-lg bg-destructive/10 p-3 space-y-1.5" data-testid="gate-error-banner">
          <p class="flex items-start gap-2 text-sm text-destructive">
            <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ t('kb.draft.gateBlocked') }}
          </p>
          <ul v-if="pg.gateReasonDetails.length" class="ml-6 space-y-1 text-sm text-destructive">
            <li v-for="(r, i) in pg.gateReasonDetails" :key="i">
              {{ r.message }}
              <button
                v-if="r.kind"
                type="button"
                class="font-medium underline underline-offset-2 hover:no-underline"
                data-testid="gate-reason-fix-link"
                @click="goToTab(r.kind)"
              >
                {{ t('kb.draft.fixInTab', { tab: entityTabLabel(r.kind) }) }}
              </button>
            </li>
          </ul>
          <!-- Fallback for the rare case the structured payload didn't come
               through (an older cached response, a malformed reasons[]) —
               the flat message alone, exactly as this banner always showed. -->
          <p v-else class="ml-6 text-sm text-destructive">{{ pg.gateReasons }}</p>
        </div>
        <p v-else-if="pg.error" class="flex items-center gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0" /> {{ pg.error }}
        </p>
      </div>

      <!-- Adding changes lives below the review: paste a link, upload files,
           or hand the job to ChatGPT/Claude. Always reachable, even with an
           empty draft. -->
      <section class="pt-6" data-testid="draft-import-section">
        <h2 class="mb-4 text-2xl font-extrabold tracking-tight">{{ t('kb.draft.importHeading') }}</h2>
        <KbIngestPanel />
      </section>
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

    <ConfirmDeleteDialog
      :open="publishConfirmOpen"
      :busy="pg.approving"
      title-key="kb.draft.publishConfirm.title"
      body-key="kb.draft.publishConfirm.body"
      :body-params="{ added: counts.added, updated: counts.updated, removed: counts.removed }"
      confirm-key="kb.draft.publishConfirm.accept"
      @update:open="(v) => !v && (publishConfirmOpen = false)"
      @confirm="confirmPublishAll"
    />

    <ConfirmDeleteDialog
      :open="discardConfirmOpen"
      :busy="pg.busy"
      title-key="kb.draft.discardConfirmDialog.title"
      body-key="kb.draft.discardConfirmDialog.body"
      :body-params="{ count: counts.total }"
      confirm-key="kb.draft.discardConfirmDialog.accept"
      @update:open="(v) => !v && (discardConfirmOpen = false)"
      @confirm="confirmDiscardAll"
    />
  </div>
</template>
