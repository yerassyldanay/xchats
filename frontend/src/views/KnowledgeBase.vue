<script setup lang="ts">
// KnowledgeBase (/knowledge-base) is the ONE creation/edit surface — every
// write here STAGES into the draft (stores.stageChange/stageDelete), never
// touches the live ai_ tables directly, matching the rule the MCP connector
// already enforces ("every write lands in the DRAFT only"). Lists render
// pg.live exclusively, so published data stays visibly unchanged after any
// write; a persistent DraftBanner points at Черновик for the reviewer to
// actually publish it.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, FileText, Plus, WandSparkles } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { useEntityTabs } from '@/composables/useEntityTabs'
import { usePendingIndex } from '@/composables/usePendingIndex'
import { useKbModal } from '@/composables/useKbModal'
import { shortTime, formatBytes } from '@/lib/format'
import type { ContactRow, KbMaterial, PolicyRow, TariffInfoRow } from '@/types'
import { CONFIG_SECTIONS } from '@/components/kb/kbEntities'
import { kbActions, LIVE_CONFIG_ACTIONS } from '@/components/kb/records/actions'
import { kindOfMime, materialContentURL } from '@/components/kb/records/shared'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import PromptTab from '@/components/kb/PromptTab.vue'
import EntityTabs from '@/components/kb/EntityTabs.vue'
import RecordList from '@/components/kb/RecordList.vue'
import DraftBanner from '@/components/kb/DraftBanner.vue'
import DeliveryZoneRecord from '@/components/kb/records/DeliveryZoneRecord.vue'
import ContactsRecord from '@/components/kb/records/ContactsRecord.vue'
import PoliciesRecord from '@/components/kb/records/PoliciesRecord.vue'
import TariffInfoRecord from '@/components/kb/records/TariffInfoRecord.vue'
import AssistantFieldRecord from '@/components/kb/records/AssistantFieldRecord.vue'
import KbModalForms from '@/components/kb/forms/KbModalForms.vue'
import ConfirmDeleteDialog from '@/components/kb/forms/ConfirmDeleteDialog.vue'

const pg = usePlayground()
const { t } = useI18n()
const modal = useKbModal()
const { markFor } = usePendingIndex()

// KB-06: the entity guide's fixed kind order — all six content tabs (config/
// materials/prompt are structural, not "which kind does this fact belong
// to", so they're excluded), matching KB_ENTITY_ORDER's own order.
const ENTITY_GUIDE_KINDS = ['topics', 'products', 'tariffs', 'tariff_info', 'delivery_zones', 'contacts', 'policies'] as const

onMounted(async () => {
  // Both slices: `live` is what this page lists, `changes` is what
  // usePendingIndex needs to mark a published row with a pending change.
  await Promise.all([pg.load(), pg.loadLive()])
  pg.startRealtime()
})
onBeforeUnmount(() => pg.stopRealtime())

const { tabs, active } = useEntityTabs({
  source: 'live',
  extra: [
    { key: 'prompt', label: t('kb.page.promptTab'), icon: WandSparkles },
    { key: 'materials', label: t('kb.page.materialsTab'), icon: FileText },
  ],
})

// --- banner: a transient "Saved" confirmation after a manual write — every
// write on this page now commits straight to the live KB (KB-13), so unlike
// the old "staged, go publish" notice this one just confirms and auto-hides;
// pg.live already carries the fresh row (writeLive*/deleteLiveEntity
// reassign it from the write's own response), so there is nothing else to
// refresh here.
const bannerVisible = ref(false)
let bannerTimer: number | undefined
function flashSaved() {
  bannerVisible.value = true
  window.clearTimeout(bannerTimer)
  bannerTimer = window.setTimeout(() => (bannerVisible.value = false), 4000)
}
watch(() => modal.successCount.value, flashSaved)

// --- toolbar action: one contextual button per tab (decision 2). Обзор has
// none — each of its 5 cards carries its own Edit; Промпт/Файлы have none
// either (both read-only, no add affordance). ---------------------------
const LIVE = { target: 'live' as const }
const toolbar = computed(() => {
  switch (active.value) {
    case 'topics':
      return { label: t('kb.page.addTopic'), action: () => modal.openCreate('topics', LIVE) }
    case 'products':
      return { label: t('kb.page.addProduct'), action: () => modal.openCreate('products', LIVE) }
    case 'tariffs':
      return { label: t('kb.page.addTariff'), action: () => modal.openCreate('tariffs', LIVE) }
    case 'delivery_zones':
      return { label: t('kb.page.addZone'), action: () => modal.openCreate('delivery_zones', LIVE) }
    case 'contacts':
      return { label: t('kb.page.editContacts'), action: () => modal.openEdit('contacts', (pg.live?.contacts[0] ?? {}) as ContactRow, LIVE) }
    case 'policies':
      return { label: t('kb.page.editPolicies'), action: () => modal.openEdit('policies', (pg.live?.policies[0] ?? {}) as PolicyRow, LIVE) }
    case 'tariff_info':
      return { label: t('kb.page.editTariffInfo'), action: () => modal.openEdit('tariff_info', (pg.live?.tariff_info[0] ?? {}) as TariffInfoRow, LIVE) }
    default:
      return null
  }
})

// --- delete flow (topics/tariffs/products/delivery_zones only — contacts/
// policies have no delete affordance, kbActions already drops it). Deletes
// straight from ai_* (deleteLiveEntity, KB-13) — no draft/publish detour. --
const deleteTarget = ref<{ kind: 'topics' | 'tariffs' | 'products' | 'delivery_zones'; key: string } | null>(null)
function askDelete(kind: 'topics' | 'tariffs' | 'products' | 'delivery_zones', key: string) {
  deleteTarget.value = { kind, key }
}
async function confirmDelete() {
  if (!deleteTarget.value) return
  const ok = await pg.deleteLiveEntity(deleteTarget.value.kind, deleteTarget.value.key)
  if (ok) {
    deleteTarget.value = null
    flashSaved()
  }
}

// --- Зоны доставки: needs allZones (parent picker labels) + pending marks.
const zonesExist = computed(() => (pg.live?.zones.length ?? 0) > 0)

// --- Файлы (материалы): read-only list of kbd_materials. GET /kb already
// returns the org's whole materials table (kbstore.LiveView), and this page
// already loads it into pg.live in onMounted — no separate fetch needed, and
// nothing here is gated behind the tab being opened for the first time.
const materials = computed<KbMaterial[]>(() => pg.live?.materials ?? [])
function materialKind(m: KbMaterial): string {
  return m.media_kind || kindOfMime(m.mime_type)
}

watch(active, (a) => {
  if (a === 'prompt' && !pg.promptView) pg.loadPrompt()
})
</script>

<template>
  <div class="h-full bg-background flex flex-col min-w-0">
    <header class="px-8 py-4 flex items-center justify-between border-b border-border bg-card shrink-0">
      <div>
        <h1 class="text-lg font-bold tracking-tight">{{ t('kb.page.title') }}</h1>
        <p class="text-sm text-muted-foreground">{{ t('kb.page.subtitle') }}</p>
      </div>
    </header>

    <div v-if="pg.liveLoading && !pg.live" class="flex-1 grid place-items-center p-8">
      <div class="text-center max-w-sm">
        <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
          <WandSparkles class="w-6 h-6" />
        </div>
        <p class="text-sm text-muted-foreground">{{ t('kb.page.loading') }}</p>
      </div>
    </div>

    <div v-else class="flex-1 overflow-y-auto px-8 py-6 space-y-6">
      <DraftBanner :show="bannerVisible" @close="bannerVisible = false" />

      <div class="flex flex-wrap items-center gap-2">
        <EntityTabs :tabs="tabs" :active="active" @update:active="(k) => (active = k)" class="flex-1" />
        <Button v-if="toolbar" size="sm" variant="outline" class="border-dashed border-primary/50 text-primary hover:bg-primary/5" @click="toolbar.action">
          <Plus class="w-4 h-4" /> {{ toolbar.label }}
        </Button>
      </div>

      <!-- KB-06: a new operator sees six tabs and no explanation of when a
           fact belongs under Topics vs Products vs Policies — collapsed by
           default so it never competes with the tab strip for attention. -->
      <details class="rounded-md border border-border p-2.5 text-xs text-muted-foreground max-w-2xl" data-testid="entity-guide">
        <summary class="cursor-pointer font-medium text-foreground">{{ t('kb.page.entityGuideTitle') }}</summary>
        <dl class="mt-2 space-y-1.5">
          <div v-for="kind in ENTITY_GUIDE_KINDS" :key="kind">
            <dt class="inline font-medium text-foreground">{{ t(`kb.entities.${kind}.singular`) }}:</dt>
            <dd class="inline ml-1">{{ t(`kb.page.entityGuide.${kind}`) }}</dd>
          </div>
        </dl>
      </details>

      <!-- Обзор: 5 read cards, each with its own Edit -->
      <div v-show="active === 'config'" class="space-y-3" data-testid="live-tab-config">
        <AssistantFieldRecord
          v-for="section in CONFIG_SECTIONS"
          :key="section.key"
          :section="section"
          :value="(pg.live?.config as any)?.[section.key] ?? ''"
          :actions="LIVE_CONFIG_ACTIONS"
          :busy="pg.busy"
          @edit="pg.live?.config && modal.openEdit('config', pg.live.config, { field: section.key, target: 'live' })"
        />
      </div>

      <div v-show="active === 'topics'" class="space-y-3" data-testid="live-tab-topics">
        <p v-if="!pg.live?.topics.length" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.page.emptyTopics') }}</p>
        <RecordList kind="topics" @delete="(key) => askDelete('topics', key)" />
      </div>

      <div v-show="active === 'products'" class="space-y-3" data-testid="live-tab-products">
        <p v-if="!pg.live?.products.length" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.page.emptyProducts') }}</p>
        <RecordList kind="products" @delete="(key) => askDelete('products', key)" />
      </div>

      <div v-show="active === 'tariffs'" class="space-y-3" data-testid="live-tab-tariffs">
        <p v-if="!pg.live?.tariffs.length" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.page.emptyTariffs') }}</p>
        <RecordList kind="tariffs" @delete="(key) => askDelete('tariffs', key)" />
      </div>

      <div v-show="active === 'delivery_zones'" class="space-y-3" data-testid="live-tab-delivery_zones">
        <p v-if="!pg.live?.zones.length" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.page.emptyZones') }}</p>
        <DeliveryZoneRecord
          v-for="z in pg.live?.zones"
          :key="z.id"
          :row="z"
          :all-zones="pg.live?.zones ?? []"
          :pending-mark="markFor('delivery_zones', z.ref)"
          :actions="kbActions({ page: 'live' })"
          :busy="pg.busy"
          @edit="modal.openEdit('delivery_zones', z, LIVE)"
          @delete="askDelete('delivery_zones', z.ref)"
        />
      </div>

      <div v-show="active === 'contacts'" class="space-y-3 max-w-2xl">
        <ContactsRecord
          :row="pg.live?.contacts[0]"
          :pending-mark="markFor('contacts', 'support')"
          :actions="kbActions({ page: 'live', singleton: true })"
          :busy="pg.busy"
          @edit="modal.openEdit('contacts', (pg.live?.contacts[0] ?? {}) as ContactRow, LIVE)"
        />
      </div>

      <div v-show="active === 'policies'" class="space-y-3 max-w-2xl">
        <PoliciesRecord
          :row="pg.live?.policies[0]"
          :zones-exist="zonesExist"
          :pending-mark="markFor('policies', 'main')"
          :actions="kbActions({ page: 'live', singleton: true })"
          :busy="pg.busy"
          @edit="modal.openEdit('policies', (pg.live?.policies[0] ?? {}) as PolicyRow, LIVE)"
        />
      </div>

      <div v-show="active === 'tariff_info'" class="space-y-3 max-w-2xl">
        <TariffInfoRecord
          :row="pg.live?.tariff_info[0]"
          :pending-mark="markFor('tariff_info', 'main')"
          :actions="kbActions({ page: 'live', singleton: true })"
          :busy="pg.busy"
          @edit="modal.openEdit('tariff_info', (pg.live?.tariff_info[0] ?? {}) as TariffInfoRow, LIVE)"
        />
      </div>

      <div v-show="active === 'prompt'">
        <PromptTab />
      </div>

      <div v-show="active === 'materials'" class="space-y-3">
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <p class="text-xs text-muted-foreground">{{ t('kb.page.materialsHint') }}</p>
          <!-- KB-10: this tab was read-only with no path back to actually
               importing something — the ingestion panel lives on /draft. -->
          <RouterLink :to="{ name: 'draft' }" data-testid="materials-import-new">
            <Button size="sm" variant="outline">
              <Plus class="w-4 h-4" /> {{ t('kb.page.materialsImportNew') }}
            </Button>
          </RouterLink>
        </div>
        <p v-if="!materials.length" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.page.materialsEmpty') }}</p>
        <div
          v-for="m in materials"
          :key="m.id"
          class="rounded-lg border border-border bg-card p-4 flex items-start gap-4"
          :class="{ 'opacity-60': !m.has_content }"
        >
          <div class="w-14 h-14 rounded-lg border border-border overflow-hidden shrink-0 grid place-items-center bg-muted">
            <img
              v-if="m.has_content && materialKind(m) === 'image'"
              :src="materialContentURL(m.id)"
              loading="lazy"
              decoding="async"
              class="w-full h-full object-cover"
            />
            <FileText v-else class="w-6 h-6 text-muted-foreground" />
          </div>
          <div class="flex-1 min-w-0 space-y-1">
            <div class="flex items-center gap-2 flex-wrap">
              <p class="text-sm font-medium truncate">{{ m.filename || m.source_ref || '—' }}</p>
              <Badge variant="secondary" class="text-[11px]">{{ m.source_type }}</Badge>
              <Badge v-if="materialKind(m)" variant="secondary" class="text-[11px]">{{ materialKind(m) }}</Badge>
              <Badge variant="outline" class="text-[11px]">{{ m.status }}</Badge>
              <Badge v-if="m.processing_status && m.processing_status !== m.status" variant="outline" class="text-[11px]">{{ m.processing_status }}</Badge>
              <Badge v-if="m.customer_visibility" variant="outline" class="text-[11px]">{{ m.customer_visibility }}</Badge>
            </div>
            <p v-if="m.mime_type || m.size_bytes" class="text-xs text-muted-foreground">
              <span v-if="m.mime_type">{{ m.mime_type }}</span>
              <span v-if="m.size_bytes">{{ m.mime_type ? ' · ' : '' }}{{ formatBytes(m.size_bytes) }}</span>
            </p>
            <p v-if="m.visual_summary || m.operator_note" class="text-xs text-muted-foreground truncate">{{ m.visual_summary || m.operator_note }}</p>
            <p class="text-[11px] text-muted-foreground">
              {{ t('kb.page.materialsCreated') }} {{ shortTime(m.created_at) }} · {{ t('kb.fields.updatedAt') }} {{ shortTime(m.updated_at) }}
            </p>
          </div>
          <a
            v-if="m.has_content"
            :href="materialContentURL(m.id)"
            target="_blank"
            rel="noopener"
            class="shrink-0 text-xs font-medium text-primary hover:underline"
          >
            {{ t('kb.page.materialsDownload') }}
          </a>
        </div>
      </div>

      <p v-if="pg.liveError" class="flex items-center gap-2 text-sm text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0" /> {{ pg.liveError }}
      </p>
      <p v-else-if="pg.error" class="flex items-center gap-2 text-sm text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0" /> {{ pg.error }}
      </p>
    </div>

    <KbModalForms />
    <ConfirmDeleteDialog
      :open="!!deleteTarget"
      :busy="pg.busy"
      @update:open="(v) => !v && (deleteTarget = null)"
      @confirm="confirmDelete"
    />
  </div>
</template>
