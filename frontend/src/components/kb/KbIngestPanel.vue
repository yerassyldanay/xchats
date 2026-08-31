<script setup lang="ts">
// KbIngestPanel is Черновик's ingestion region — every way information
// enters the KB, one tab each: Ссылки/Файлы (the structured import
// pipeline, internal/kbimport) and ChatGPT / Claude (the MCP connector,
// McpConnectCard unchanged). This is where /playground's own "model-driven
// ingest" half of the 2026-08 product-boundary refinement actually lives;
// /knowledge-base stays the only MANUAL authoring surface (no Add buttons
// or record forms here).
//
// Owns the import run's realtime lifecycle (loadLatest/startRealtime) and
// hosts ONE shared KbImportRunStatus below the tab strip — KbImportCard
// itself no longer does either, since both the Ссылки and Файлы tabs stay
// mounted at once (v-show, not v-if) and would otherwise double-subscribe.
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Link as LinkIcon, Sparkles, Upload } from 'lucide-vue-next'
import { useKbImport } from '@/stores/kbImport'
import { useInbox } from '@/stores/inbox'
import { useAuth } from '@/stores/auth'
import type { KbTab } from '@/composables/useEntityTabs'
import EntityTabs from './EntityTabs.vue'
import KbImportCard from './KbImportCard.vue'
import KbImportRunStatus from './KbImportRunStatus.vue'
import McpConnectCard from './McpConnectCard.vue'

const kbi = useKbImport()
const inbox = useInbox()
const auth = useAuth()
const { t } = useI18n()

onMounted(async () => {
  await kbi.loadLatest()
  kbi.startRealtime()
  // KB-05: RunSummary.started_by is a raw user id (see KbImportRun's own
  // doc comment) — the org's user list, already fetched app-wide via
  // useInbox for chat assignment, resolves it to a display name without
  // this panel needing an endpoint of its own.
  if (inbox.users.length === 0) void inbox.loadUsers()
})
onBeforeUnmount(() => kbi.stopRealtime())

// KB-05: "you" reads better than the operator's own full name in their own
// import history, and needs no lookup at all if the user list hasn't
// loaded yet.
const startedByLabel = computed(() => {
  const id = kbi.current?.started_by
  if (!id) return ''
  if (id === auth.user?.id) return t('kb.import.startedByYou')
  return inbox.users.find((u) => u.id === id)?.name || ''
})

type IngestTabKey = 'urls' | 'files' | 'mcp'
const tabs: KbTab[] = [
  { key: 'urls', label: t('kb.import.tabs.urls'), icon: LinkIcon },
  { key: 'files', label: t('kb.import.tabs.files'), icon: Upload },
  { key: 'mcp', label: t('kb.import.tabs.mcp'), icon: Sparkles },
]
const active = ref<IngestTabKey>('urls')
</script>

<template>
  <div class="space-y-4">
    <EntityTabs :tabs="tabs" :active="active" @update:active="(k) => (active = k as IngestTabKey)" />

    <div v-show="active === 'urls'" data-testid="kb-import-tab-urls"><KbImportCard kind="url" /></div>
    <div v-show="active === 'files'" data-testid="kb-import-tab-files"><KbImportCard kind="file" /></div>
    <div v-show="active === 'mcp'"><McpConnectCard /></div>

    <KbImportRunStatus
      v-if="kbi.current"
      :run="kbi.current"
      :cancelling="kbi.cancelling"
      :started-by-label="startedByLabel"
      @cancel="kbi.cancel()"
    />
    <p v-if="kbi.cancelError" class="flex items-center gap-1.5 text-sm text-destructive">{{ kbi.cancelError }}</p>
  </div>
</template>
