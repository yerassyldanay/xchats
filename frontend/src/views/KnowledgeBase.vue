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
import { CircleAlert, FileText, Library, Plus, WandSparkles } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { useEntityTabs } from '@/composables/useEntityTabs'
import { usePendingIndex } from '@/composables/usePendingIndex'
import { useKbModal } from '@/composables/useKbModal'
import { shortTime, formatBytes } from '@/lib/format'
import type { ContactRow, KbMaterial, PolicyRow } from '@/types'
import { CONFIG_SECTIONS, ENTITY_META } from '@/components/kb/kbEntities'
import { kbActions, LIVE_CONFIG_ACTIONS } from '@/components/kb/records/actions'
import { kindOfMime, materialContentURL } from '@/components/kb/records/shared'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import PromptTab from '@/components/kb/PromptTab.vue'
import EntityTabs from '@/components/kb/EntityTabs.vue'
import RecordList from '@/components/kb/RecordList.vue'
import DraftBanner from '@/components/kb/DraftBanner.vue'
import KbEmptyTab from '@/components/kb/KbEmptyTab.vue'
import DeliveryZoneRecord from '@/components/kb/records/DeliveryZoneRecord.vue'
import ContactsRecord from '@/components/kb/records/ContactsRecord.vue'
import PoliciesRecord from '@/components/kb/records/PoliciesRecord.vue'
import AssistantFieldRecord from '@/components/kb/records/AssistantFieldRecord.vue'
import KbModalForms from '@/components/kb/forms/KbModalForms.vue'
import ConfirmDeleteDialog from '@/components/kb/forms/ConfirmDeleteDialog.vue'

const pg = usePlayground()
const { t } = useI18n()
const modal = useKbModal()
const { markFor } = usePendingIndex()

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

// --- banner: a persistent success notice after ANY staged write, pointing
// at Черновик — pg.live is deliberately never refetched when it appears. --
const bannerVisible = ref(false)
watch(() => modal.successCount.value, () => {
  bannerVisible.value = true
})

// --- toolbar action: one contextual button per tab (decision 2). Обзор has
// none — each of its 5 cards carries its own Edit; Промпт/Файлы have none
// either (both read-only, no add affordance). ---------------------------
const toolbar = computed(() => {
  switch (active.value) {
    case 'topics':
      return { label: t('kb.page.addTopic'), action: () => modal.openCreate('topics') }
    case 'products':
      return { label: t('kb.page.addProduct'), action: () => modal.openCreate('products') }
    case 'tariffs':
      return { label: t('kb.page.addTariff'), action: () => modal.openCreate('tariffs') }
    case 'delivery_zones':
      return { label: t('kb.page.addZone'), action: () => modal.openCreate('delivery_zones') }
    case 'contacts':
      return { label: t('kb.page.editContacts'), action: () => modal.openEdit('contacts', (pg.live?.contacts[0] ?? {}) as ContactRow) }
    case 'policies':
      return { label: t('kb.page.editPolicies'), action: () => modal.openEdit('policies', (pg.live?.policies[0] ?? {}) as PolicyRow) }
    default:
      return null
  }
})

// --- delete flow (topics/tariffs/products/delivery_zones only — contacts/
// policies have no delete affordance, kbActions already drops it). -------
const deleteTarget = ref<{ kind: 'topics' | 'tariffs' | 'products' | 'delivery_zones'; key: string } | null>(null)
function askDelete(kind: 'topics' | 'tariffs' | 'products' | 'delivery_zones', key: string) {
  deleteTarget.value = { kind, key }
}
async function confirmDelete() {
  if (!deleteTarget.value) return
  const ok = await pg.stageDelete(deleteTarget.value.kind, deleteTarget.value.key)
  if (ok) {
    deleteTarget.value = null
    bannerVisible.value = true
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
  <div class="flex h-full min-w-0 flex-col bg-background">
    <header class="shrink-0 border-b border-border bg-card px-4 py-5 sm:px-6 lg:px-8">
      <div class="flex items-center gap-2.5">
        <div class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
          <Library class="h-4 w-4" />
        </div>
        <h1 class="text-xl font-semibold tracking-tight text-foreground">{{ t('kb.page.title') }}</h1>
      </div>
      <p class="mt-1.5 text-sm text-muted-foreground">{{ t('kb.page.subtitle') }}</p>
    </header>

    <div v-if="pg.liveLoading && !pg.live" class="grid flex-1 place-items-center p-8">
      <div class="max-w-sm text-center">
        <div class="mx-auto mb-3 grid h-12 w-12 place-items-center rounded-xl bg-primary/10 text-primary">
          <WandSparkles class="h-6 w-6" />
        </div>
        <p class="text-sm text-muted-foreground">{{ t('kb.page.loading') }}</p>
      </div>
    </div>

    <div v-else class="flex-1 overflow-y-auto px-4 py-6 sm:px-6 lg:px-8">
      <div class="space-y-6">
        <DraftBanner :show="bannerVisible" @close="bannerVisible = false" />

        <div class="flex items-start justify-between gap-3">
          <EntityTabs :tabs="tabs" :active="active" class="flex-1" @update:active="(k) => (active = k)" />
          <Button v-if="toolbar" size="sm" variant="outline" class="shrink-0 border-dashed border-primary/50 text-primary hover:bg-primary/5" @click="toolbar.action">
            <Plus class="h-4 w-4" /> {{ toolbar.label }}
          </Button>
        </div>

        <!-- Обзор: 5 read cards, each with its own Edit -->
        <div v-show="active === 'config'" class="grid grid-cols-1 items-start gap-4 lg:grid-cols-2">
          <AssistantFieldRecord
            v-for="section in CONFIG_SECTIONS"
            :key="section.key"
            :section="section"
            :value="(pg.live?.config as any)?.[section.key] ?? ''"
            :actions="LIVE_CONFIG_ACTIONS"
            :busy="pg.busy"
            @edit="pg.live?.config && modal.openEdit('config', pg.live.config, { field: section.key })"
          />
        </div>

        <div v-show="active === 'topics'">
          <KbEmptyTab v-if="!pg.live?.topics.length" :icon="ENTITY_META.topics.icon" :message="t('kb.page.emptyTopics')" :action-label="toolbar?.label" @action="toolbar?.action?.()" />
          <div v-else class="grid grid-cols-1 items-start gap-4 xl:grid-cols-2">
            <RecordList kind="topics" @delete="(key) => askDelete('topics', key)" />
          </div>
        </div>

        <div v-show="active === 'products'">
          <KbEmptyTab v-if="!pg.live?.products.length" :icon="ENTITY_META.products.icon" :message="t('kb.page.emptyProducts')" :action-label="toolbar?.label" @action="toolbar?.action?.()" />
          <div v-else class="grid grid-cols-1 items-start gap-4 xl:grid-cols-2">
            <RecordList kind="products" @delete="(key) => askDelete('products', key)" />
          </div>
        </div>

        <div v-show="active === 'tariffs'">
          <KbEmptyTab v-if="!pg.live?.tariffs.length" :icon="ENTITY_META.tariffs.icon" :message="t('kb.page.emptyTariffs')" :action-label="toolbar?.label" @action="toolbar?.action?.()" />
          <div v-else class="grid grid-cols-1 items-start gap-4 xl:grid-cols-2">
            <RecordList kind="tariffs" @delete="(key) => askDelete('tariffs', key)" />
          </div>
        </div>

        <div v-show="active === 'delivery_zones'">
          <KbEmptyTab v-if="!pg.live?.zones.length" :icon="ENTITY_META.delivery_zones.icon" :message="t('kb.page.emptyZones')" :action-label="toolbar?.label" @action="toolbar?.action?.()" />
          <div v-else class="grid grid-cols-1 items-start gap-4 xl:grid-cols-2">
            <DeliveryZoneRecord
              v-for="z in pg.live?.zones"
              :key="z.id"
              :row="z"
              :all-zones="pg.live?.zones ?? []"
              :pending-mark="markFor('delivery_zones', z.ref)"
              :actions="kbActions({ page: 'live' })"
              :busy="pg.busy"
              @edit="modal.openEdit('delivery_zones', z)"
              @delete="askDelete('delivery_zones', z.ref)"
            />
          </div>
        </div>

        <div v-show="active === 'contacts'" class="max-w-2xl">
          <ContactsRecord
            :row="pg.live?.contacts[0]"
            :pending-mark="markFor('contacts', 'support')"
            :actions="kbActions({ page: 'live', singleton: true })"
            :busy="pg.busy"
            @edit="modal.openEdit('contacts', (pg.live?.contacts[0] ?? {}) as ContactRow)"
          />
        </div>

        <div v-show="active === 'policies'" class="max-w-2xl">
          <PoliciesRecord
            :row="pg.live?.policies[0]"
            :zones-exist="zonesExist"
            :pending-mark="markFor('policies', 'main')"
            :actions="kbActions({ page: 'live', singleton: true })"
            :busy="pg.busy"
            @edit="modal.openEdit('policies', (pg.live?.policies[0] ?? {}) as PolicyRow)"
          />
        </div>

        <div v-show="active === 'prompt'">
          <PromptTab />
        </div>

        <div v-show="active === 'materials'">
          <p class="mb-4 text-xs text-muted-foreground">{{ t('kb.page.materialsHint') }}</p>
          <KbEmptyTab v-if="!materials.length" :icon="FileText" :message="t('kb.page.materialsEmpty')" />
          <div v-else class="grid grid-cols-1 items-start gap-4 lg:grid-cols-2">
            <div
              v-for="m in materials"
              :key="m.id"
              class="flex items-start gap-4 rounded-xl border border-border bg-card p-4 transition-shadow hover:shadow-card"
              :class="{ 'opacity-60': !m.has_content }"
            >
              <div class="grid h-14 w-14 shrink-0 place-items-center overflow-hidden rounded-lg border border-border bg-muted">
                <img
                  v-if="m.has_content && materialKind(m) === 'image'"
                  :src="materialContentURL(m.id)"
                  loading="lazy"
                  decoding="async"
                  class="h-full w-full object-cover"
                />
                <FileText v-else class="h-6 w-6 text-muted-foreground" />
              </div>
              <div class="min-w-0 flex-1 space-y-1.5">
                <div class="flex flex-wrap items-center gap-1.5">
                  <p class="truncate text-sm font-medium">{{ m.filename || m.source_ref || '—' }}</p>
                </div>
                <div class="flex flex-wrap items-center gap-1.5">
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
                <p v-if="m.visual_summary || m.operator_note" class="truncate text-xs text-muted-foreground">{{ m.visual_summary || m.operator_note }}</p>
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
        </div>

        <p v-if="pg.liveError" class="flex items-center gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5 text-sm text-destructive">
          <CircleAlert class="h-4 w-4 shrink-0" /> {{ pg.liveError }}
        </p>
        <p v-else-if="pg.error" class="flex items-center gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5 text-sm text-destructive">
          <CircleAlert class="h-4 w-4 shrink-0" /> {{ pg.error }}
        </p>
      </div>
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
