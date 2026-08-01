<script setup lang="ts">
// KnowledgeBase (База знаний, /knowledge-base) is the ONE creation surface —
// every write here stages into the draft (see stores/playground.ts's
// stageChange/stageDelete), never the live tables directly. Reads stay
// GET /kb: lists render pg.live exclusively, so published data is visibly
// unchanged right after any write — the pending effect shows up on
// Черновик (/playground), which this page always links back to via
// DraftBanner after a successful stage.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, FileText, Files, Plus, Sparkles, WandSparkles } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { useKbModal } from '@/composables/useKbModal'
import { useEntityTabs } from '@/composables/useEntityTabs'
import { usePendingIndex } from '@/composables/usePendingIndex'
import { shortTime } from '@/lib/format'
import { api, ApiError } from '@/api/client'
import type { ContactRow, KbMaterial, PolicyRow } from '@/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import EntityTabs from '@/components/kb/EntityTabs.vue'
import RecordList from '@/components/kb/RecordList.vue'
import DraftBanner from '@/components/kb/DraftBanner.vue'
import PromptTab from '@/components/kb/PromptTab.vue'
import ContactsRecord from '@/components/kb/records/ContactsRecord.vue'
import PoliciesRecord from '@/components/kb/records/PoliciesRecord.vue'
import AssistantFieldRecord from '@/components/kb/records/AssistantFieldRecord.vue'
import { kbActions } from '@/components/kb/records/actions'
import { CONFIG_SECTIONS, ENTITY_META } from '@/components/kb/kbEntities'
import TopicForm from '@/components/kb/forms/TopicForm.vue'
import ProductForm from '@/components/kb/forms/ProductForm.vue'
import TariffForm from '@/components/kb/forms/TariffForm.vue'
import DeliveryZoneForm from '@/components/kb/forms/DeliveryZoneForm.vue'
import ContactsForm from '@/components/kb/forms/ContactsForm.vue'
import PoliciesForm from '@/components/kb/forms/PoliciesForm.vue'
import AssistantFieldForm from '@/components/kb/forms/AssistantFieldForm.vue'

const pg = usePlayground()
const modal = useKbModal()
const { markFor } = usePendingIndex()
const { t } = useI18n()

onMounted(async () => {
  // usePendingIndex's badges classify against pg.changes too (see
  // usePendingIndex.ts) — without loading it here, a visitor who opens
  // /knowledge-base without having visited Черновик first in this session
  // would never see a pending-change mark on any published row.
  await Promise.all([pg.load(), pg.loadLive()])
  pg.startRealtime()
})
onBeforeUnmount(() => pg.stopRealtime())

const { tabs, active } = useEntityTabs({
  source: 'live',
  extra: [
    { key: 'prompt', label: t('kb.tabs.prompt'), icon: Sparkles },
    { key: 'materials', label: t('kb.tabs.materials'), icon: Files },
  ],
})

// --- per-tab toolbar action ("the one creation surface") --------------------
const BLANK_CONTACT: ContactRow = {
  id: 'support', slug: 'support', whatsapp: '', email: '', address: '', legal_information: '',
  callback_time: '', working_hours: '', phone: '', website: '', instagram: '',
  contact_card_image: null, location_map_image: null, company_legal_documents: [],
  draft: false, updated_at: '',
}
const BLANK_POLICY: PolicyRow = {
  id: 'main', slug: 'main', delivery_cost: '', delivery_in_days: '', free_delivery_from: '', min_order: '',
  prepayment: '', installment: '', return_period_in_days: '', warranty: '', outside_zones_note: '',
  commerce_policy_documents: [], draft: false, updated_at: '',
}
const toolbarAction = computed(() => {
  switch (active.value) {
    case 'topics':
      return { label: t('kb.toolbar.addTopic'), run: () => modal.openCreate('topics') }
    case 'products':
      return { label: t('kb.toolbar.addProduct'), run: () => modal.openCreate('products') }
    case 'tariffs':
      return { label: t('kb.toolbar.addTariff'), run: () => modal.openCreate('tariffs') }
    case 'delivery_zones':
      return { label: t('kb.toolbar.addZone'), run: () => modal.openCreate('delivery_zones') }
    case 'contacts':
      return { label: t('kb.toolbar.editContacts'), run: () => modal.openEdit('contacts', pg.live?.contacts?.[0] ?? BLANK_CONTACT) }
    case 'policies':
      return { label: t('kb.toolbar.editPolicies'), run: () => modal.openEdit('policies', pg.live?.policies?.[0] ?? BLANK_POLICY) }
    default:
      return null // Обзор (per-card Edit only), Промпт, Файлы — no toolbar affordance
  }
})

// --- Обзор: the 5 assistant-config sections as read cards --------------------
function configValue(key: (typeof CONFIG_SECTIONS)[number]['key']): string | number {
  const c = pg.live?.config as Record<string, unknown> | undefined
  return (c?.[key] as string | number) ?? ''
}
const configActions = kbActions({ page: 'live', singleton: true }) // edit only — config has no delete affordance
function editConfigField(field: (typeof CONFIG_SECTIONS)[number]['key']) {
  if (pg.live?.config) modal.openEdit('config', { [field]: configValue(field) }, { field })
}

// --- Файлы (материалы): read-only list of kbd_materials (GET /kb/materials) --
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
watch(active, (tab) => {
  if (tab === 'materials' && !materials.value.length) loadMaterials()
  if (tab === 'prompt' && !pg.promptView) pg.loadPrompt()
})

// --- staged-write banner ------------------------------------------------------
// pg.lastStagedAt is bumped ONLY on a genuine successful stageChange/
// stageDelete — distinct from the modal simply closing (also true of a plain
// "Отмена"/backdrop close with nothing saved).
const stagedKindLabel = ref('')
watch(
  () => pg.lastStagedAt,
  () => {
    if (pg.lastStagedKind) stagedKindLabel.value = t(ENTITY_META[pg.lastStagedKind].labelKey)
  }
)
</script>

<template>
  <div class="h-full bg-background flex flex-col min-w-0">
    <header class="px-8 py-4 flex items-center justify-between border-b border-border bg-card shrink-0">
      <div>
        <h1 class="text-lg font-bold tracking-tight">{{ t('kb.live.title') }}</h1>
        <p class="text-sm text-muted-foreground">{{ t('kb.live.subtitle') }}</p>
      </div>
    </header>

    <div v-if="pg.liveLoading && !pg.live" class="flex-1 grid place-items-center p-8">
      <div class="text-center max-w-sm">
        <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
          <WandSparkles class="w-6 h-6" />
        </div>
        <p class="text-sm text-muted-foreground">Загрузка базы знаний…</p>
      </div>
    </div>

    <div v-else class="flex-1 overflow-y-auto px-8 py-6 space-y-6">
      <DraftBanner :show="!!stagedKindLabel" :kind-label="stagedKindLabel" />

      <div class="flex flex-wrap items-center gap-2">
        <EntityTabs :tabs="tabs" v-model="active" class="flex-1" />
        <Button v-if="toolbarAction" size="sm" @click="toolbarAction.run()">
          <Plus class="w-4 h-4" /> {{ toolbarAction.label }}
        </Button>
      </div>

      <!-- Обзор -->
      <div v-show="active === 'config'" class="space-y-3 max-w-3xl">
        <AssistantFieldRecord
          v-for="section in CONFIG_SECTIONS"
          :key="section.key"
          :field="section.key"
          :draft-value="configValue(section.key)"
          :change-type="markFor('config', section.key)"
          :actions="configActions"
          @edit="editConfigField(section.key)"
        />
      </div>

      <!-- Темы / Товары / Тарифы / Зоны доставки -->
      <div v-show="active === 'topics'"><RecordList kind="topics" :rows="pg.live?.topics ?? []" /></div>
      <div v-show="active === 'products'"><RecordList kind="products" :rows="pg.live?.products ?? []" /></div>
      <div v-show="active === 'tariffs'"><RecordList kind="tariffs" :rows="pg.live?.tariffs ?? []" /></div>
      <div v-show="active === 'delivery_zones'"><RecordList kind="delivery_zones" :rows="pg.live?.zones ?? []" /></div>

      <!-- Контакты (the 'support' singleton) -->
      <div v-show="active === 'contacts'" class="space-y-3 max-w-2xl">
        <ContactsRecord
          :row="pg.live?.contacts?.[0]"
          :change-type="pg.live?.contacts?.[0] ? markFor('contacts', pg.live.contacts[0].id) : undefined"
          :actions="kbActions({ page: 'live', singleton: true })"
          @edit="toolbarAction?.run()"
        />
      </div>

      <!-- Политики (the 'main' commerce-policy singleton) -->
      <div v-show="active === 'policies'" class="space-y-3 max-w-2xl">
        <PoliciesRecord
          :row="pg.live?.policies?.[0]"
          :change-type="pg.live?.policies?.[0] ? markFor('policies', pg.live.policies[0].id) : undefined"
          :actions="kbActions({ page: 'live', singleton: true })"
          @edit="toolbarAction?.run()"
        />
      </div>

      <!-- Промпт -->
      <div v-show="active === 'prompt'"><PromptTab /></div>

      <!-- Файлы (материалы) — read-only -->
      <div v-show="active === 'materials'" class="space-y-3">
        <p class="text-xs text-muted-foreground">
          Материалы, добавленные через интеграции базы знаний, доступны здесь только для просмотра.
        </p>
        <div v-if="materialsLoading && !materials.length" class="text-sm text-muted-foreground py-6 text-center">Загрузка…</div>
        <div v-else-if="materialsError" class="flex items-center gap-2 text-sm text-destructive rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2">
          <CircleAlert class="w-4 h-4 shrink-0" /> {{ materialsError }}
          <Button size="sm" variant="outline" class="ml-auto" @click="loadMaterials">Повторить</Button>
        </div>
        <p v-else-if="!materials.length" class="text-sm text-muted-foreground py-6 text-center">Материалов пока нет.</p>
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
    </div>
  </div>

  <TopicForm />
  <ProductForm />
  <TariffForm />
  <DeliveryZoneForm />
  <ContactsForm />
  <PoliciesForm />
  <AssistantFieldForm />
</template>
