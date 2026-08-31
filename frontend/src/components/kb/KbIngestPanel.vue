<script setup lang="ts">
// KbIngestPanel is Черновик's ingestion region — every way information
// enters the KB, one tab each: Ссылки/Файлы (the structured import
// pipeline, internal/kbimport) and ChatGPT / Claude (the MCP connector,
// McpConnectCard unchanged). This is where /draft's own "model-driven
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
import { useRoute, useRouter } from 'vue-router'
import { History, Link as LinkIcon, Sparkles, Upload } from 'lucide-vue-next'
import { resolveStartedByLabel, useKbImport } from '@/stores/kbImport'
import { useInbox } from '@/stores/inbox'
import { useAuth } from '@/stores/auth'
import { queryString } from '@/lib/queryState'
import { Button } from '@/components/ui/button'
import type { KbTab } from '@/composables/useEntityTabs'
import EntityTabs from './EntityTabs.vue'
import KbImportCard from './KbImportCard.vue'
import KbImportRunStatus from './KbImportRunStatus.vue'
import KbImportHistoryDialog from './KbImportHistoryDialog.vue'
import McpConnectCard from './McpConnectCard.vue'

const kbi = useKbImport()
const inbox = useInbox()
const auth = useAuth()
const { t } = useI18n()
const route = useRoute()
const router = useRouter()

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
const startedByLabel = computed(() => resolveStartedByLabel(kbi.current?.started_by ?? '', auth.user?.id, inbox.users))

type IngestTabKey = 'urls' | 'files' | 'mcp'
const tabs: KbTab[] = [
  { key: 'urls', label: t('kb.import.tabs.urls'), icon: LinkIcon },
  { key: 'files', label: t('kb.import.tabs.files'), icon: Upload },
  { key: 'mcp', label: t('kb.import.tabs.mcp'), icon: Sparkles },
]
const active = ref<IngestTabKey>('urls')

// KB-14: history dialog open state and the selected run both live in the
// URL (?khist=1&krun=<id>), same queryString/router.replace idiom CAM-11
// established for Campaigns' list state — a refresh or a shared link
// re-opens the dialog on the exact run being reviewed instead of silently
// dropping back to the ingestion tabs.
const selectedRunId = ref(queryString(route.query.krun))
// A bare ?krun=<id> (no khist) still counts as "open" — there is no reason
// to require both params for a deep link to work.
const historyOpen = ref(route.query.khist === '1' || !!selectedRunId.value)
function syncHistoryQuery() {
  const query: Record<string, string> = { ...(route.query as Record<string, string>) }
  if (historyOpen.value) query.khist = '1'
  else delete query.khist
  if (historyOpen.value && selectedRunId.value) query.krun = selectedRunId.value
  else delete query.krun
  void router.replace({ query })
}
function setHistoryOpen(v: boolean) {
  historyOpen.value = v
  if (!v) selectedRunId.value = ''
  syncHistoryQuery()
}
function setSelectedRunId(id: string) {
  selectedRunId.value = id
  syncHistoryQuery()
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between gap-2">
      <EntityTabs :tabs="tabs" :active="active" @update:active="(k) => (active = k as IngestTabKey)" />
      <Button variant="ghost" size="sm" data-testid="kb-import-history-open" @click="setHistoryOpen(true)">
        <History class="w-4 h-4" /> {{ t('kb.import.history.open') }}
      </Button>
    </div>

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

    <KbImportHistoryDialog
      :open="historyOpen"
      :initial-run-id="selectedRunId || undefined"
      @update:open="setHistoryOpen"
      @update:selected-run-id="setSelectedRunId"
    />
  </div>
</template>
