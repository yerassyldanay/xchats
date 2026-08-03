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
import { shortTime } from '@/lib/format'
import { api, ApiError } from '@/api/client'
import type { ContactRow, KbMaterial, PolicyRow } from '@/types'
import { CONFIG_SECTIONS } from '@/components/kb/kbEntities'
import { kbActions, LIVE_CONFIG_ACTIONS } from '@/components/kb/records/actions'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import PromptTab from '@/components/kb/PromptTab.vue'
import EntityTabs from '@/components/kb/EntityTabs.vue'
import RecordList from '@/components/kb/RecordList.vue'
import DraftBanner from '@/components/kb/DraftBanner.vue'
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

// --- Файлы (материалы): read-only list of kbd_materials (GET /kb/materials).
// No draft/attach/edit here — unrelated to this redesign.
const materials = ref<KbMaterial[]>([])
const materialsLoading = ref(false)
const materialsError = ref('')
async function loadMaterials() {
  materialsLoading.value = true
  materialsError.value = ''
  try {
    const res = await api.get<{ materials: KbMaterial[] }>('/kb/materials')
    materials.value = res.materials
  } catch (e) {
    materialsError.value = e instanceof ApiError ? e.message : 'Не удалось загрузить материалы.'
  } finally {
    materialsLoading.value = false
  }
}
function materialPreviewURL(m: KbMaterial): string | null {
  return m.media_kind === 'image' && m.blob_id ? api.mediaURL('/xchats/api/v1/media/' + m.blob_id) : null
}

watch(active, (a) => {
  if (a === 'materials' && !materials.value.length) loadMaterials()
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

      <!-- Обзор: 5 read cards, each with its own Edit -->
      <div v-show="active === 'config'" class="space-y-3">
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

      <div v-show="active === 'topics'" class="space-y-3">
        <p v-if="!pg.live?.topics.length" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.page.emptyTopics') }}</p>
        <RecordList kind="topics" @delete="(key) => askDelete('topics', key)" />
      </div>

      <div v-show="active === 'products'" class="space-y-3">
        <p v-if="!pg.live?.products.length" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.page.emptyProducts') }}</p>
        <RecordList kind="products" @delete="(key) => askDelete('products', key)" />
      </div>

      <div v-show="active === 'tariffs'" class="space-y-3">
        <p v-if="!pg.live?.tariffs.length" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.page.emptyTariffs') }}</p>
        <RecordList kind="tariffs" @delete="(key) => askDelete('tariffs', key)" />
      </div>

      <div v-show="active === 'delivery_zones'" class="space-y-3">
        <p v-if="!pg.live?.zones.length" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.page.emptyZones') }}</p>
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

      <div v-show="active === 'contacts'" class="space-y-3 max-w-2xl">
        <ContactsRecord
          :row="pg.live?.contacts[0]"
          :pending-mark="markFor('contacts', 'support')"
          :actions="kbActions({ page: 'live', singleton: true })"
          :busy="pg.busy"
          @edit="modal.openEdit('contacts', (pg.live?.contacts[0] ?? {}) as ContactRow)"
        />
      </div>

      <div v-show="active === 'policies'" class="space-y-3 max-w-2xl">
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

      <div v-show="active === 'materials'" class="space-y-3">
        <p class="text-xs text-muted-foreground">{{ t('kb.page.materialsHint') }}</p>
        <div v-if="materialsLoading && !materials.length" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.page.materialsLoading') }}</div>
        <div v-else-if="materialsError" class="flex items-center gap-2 text-sm text-destructive rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2">
          <CircleAlert class="w-4 h-4 shrink-0" /> {{ materialsError }}
          <Button size="sm" variant="outline" class="ml-auto" @click="loadMaterials">{{ t('kb.page.materialsRetry') }}</Button>
        </div>
        <p v-else-if="!materials.length" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.page.materialsEmpty') }}</p>
        <div v-for="m in materials" :key="m.id" class="rounded-lg border border-border bg-card p-4 flex items-center gap-4">
          <div class="w-14 h-14 rounded-lg border border-border overflow-hidden shrink-0 grid place-items-center bg-muted">
            <img v-if="materialPreviewURL(m)" :src="materialPreviewURL(m) ?? ''" class="w-full h-full object-cover" />
            <FileText v-else class="w-6 h-6 text-muted-foreground" />
          </div>
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 flex-wrap">
              <Badge variant="secondary" class="text-[11px]">{{ m.source_type }}</Badge>
              <Badge v-if="m.media_kind" variant="secondary" class="text-[11px]">{{ m.media_kind }}</Badge>
              <span class="text-xs text-muted-foreground">{{ m.status }}</span>
            </div>
            <p class="text-sm truncate mt-1">{{ m.source_ref || '—' }}</p>
          </div>
          <span class="text-xs text-muted-foreground shrink-0 whitespace-nowrap">{{ shortTime(m.created_at) }}</span>
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
