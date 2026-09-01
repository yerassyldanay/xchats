<script setup lang="ts">
// KbImportHistoryDialog is KB-14's import history: the backend could always
// list every run (GET /kb/imports), but the ingestion panel only ever
// fetched and showed the single latest one. Deliberately a DIALOG rather
// than a route/page — the current-run card in KbIngestPanel stays mounted
// and prominent underneath it the whole time (AC1/AC2: history must never
// replace that card, only sit alongside it as an overlay).
//
// Fully controlled from KbIngestPanel (open + which run is selected, both
// props/emits) so the panel can mirror them into the URL (AC3) — this
// component owns only its own page number and the fetched detail object,
// neither of which needs to survive a refresh on its own.
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronLeft } from 'lucide-vue-next'
import { api, ApiError } from '@/api/client'
import { useKbImport, resolveStartedByLabel } from '@/stores/kbImport'
import { useInbox } from '@/stores/inbox'
import { useAuth } from '@/stores/auth'
import { RUN_STATUS_META } from '@/lib/kbImportStatus'
import { formatDateTime } from '@/lib/format'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import Pagination from '@/components/Pagination.vue'
import KbImportRunStatus from './KbImportRunStatus.vue'
import type { KbImportRun } from '@/types'

const props = defineProps<{ open: boolean; initialRunId?: string }>()
const emit = defineEmits<{ 'update:open': [boolean]; 'update:selectedRunId': [string] }>()

const { t, locale } = useI18n()
const kbi = useKbImport()
const inbox = useInbox()
const auth = useAuth()

const PAGE_SIZE = 10
const page = ref(1)
const detail = ref<KbImportRun | null>(null)
const detailLoading = ref(false)
const detailError = ref('')

function ownerLabel(userId: string): string {
  return resolveStartedByLabel(userId, auth.user?.id, inbox.users)
}
function sourcesSummary(run: KbImportRun): string {
  return run.materials.map((m) => m.label).join(', ')
}

function openRow(run: KbImportRun) {
  detail.value = run
  detailError.value = ''
  emit('update:selectedRunId', run.run_id)
}
async function openRunById(id: string) {
  detailLoading.value = true
  detailError.value = ''
  try {
    detail.value = await api.getKbImportRun(id)
    emit('update:selectedRunId', id)
  } catch (e) {
    detail.value = null
    detailError.value = e instanceof ApiError ? e.message : t('kb.import.history.errLoadRun')
    await kbi.loadHistory(page.value, PAGE_SIZE)
  } finally {
    detailLoading.value = false
  }
}
function backToList() {
  detail.value = null
  detailError.value = ''
  emit('update:selectedRunId', '')
  void kbi.loadHistory(page.value, PAGE_SIZE)
}

// Opening the dialog either jumps straight to a specific run (deep link /
// page-refresh restore) or shows page 1 of the list — never both at once.
// immediate: true so a page that mounts with `open` already true (the
// refresh/deep-link case itself — KbIngestPanel seeds it from ?khist/?krun)
// still loads, not just a later false->true transition.
watch(
  () => props.open,
  (isOpen) => {
    if (!isOpen) return
    if (inbox.users.length === 0) void inbox.loadUsers()
    if (props.initialRunId) {
      void openRunById(props.initialRunId)
    } else {
      detail.value = null
      detailError.value = ''
      page.value = 1
      void kbi.loadHistory(1, PAGE_SIZE)
    }
  },
  { immediate: true }
)
watch(page, (p) => {
  if (props.open && !detail.value) void kbi.loadHistory(p, PAGE_SIZE)
})
</script>

<template>
  <Dialog :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogContent class="max-w-2xl max-h-[85vh] flex flex-col">
      <DialogHeader class="shrink-0">
        <DialogTitle>{{ detail ? t('kb.import.history.runTitle') : t('kb.import.history.title') }}</DialogTitle>
      </DialogHeader>

      <div class="px-5 py-4 overflow-y-auto flex-1 min-h-0 space-y-3">
        <template v-if="detail">
          <Button variant="ghost" size="sm" data-testid="history-back" @click="backToList">
            <ChevronLeft class="w-4 h-4" /> {{ t('kb.import.history.back') }}
          </Button>
          <KbImportRunStatus :run="detail" :cancellable="false" :started-by-label="ownerLabel(detail.started_by)" />
        </template>

        <template v-else>
          <div v-if="detailLoading" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.import.history.loading') }}</div>
          <template v-else>
            <p v-if="detailError" class="text-sm text-destructive" data-testid="history-detail-error">{{ detailError }}</p>
            <p v-if="kbi.historyError" class="text-sm text-destructive">{{ kbi.historyError }}</p>

            <div v-if="kbi.historyLoading" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.import.history.loading') }}</div>
            <p v-else-if="!kbi.history.length" class="text-sm text-muted-foreground py-6 text-center" data-testid="history-empty">
              {{ t('kb.import.history.empty') }}
            </p>
            <template v-else>
              <button
                v-for="run in kbi.history"
                :key="run.run_id"
                type="button"
                data-testid="history-row"
                class="w-full text-left rounded-lg border border-border p-3 hover:bg-muted transition space-y-1.5"
                @click="openRow(run)"
              >
                <div class="flex items-center gap-2 flex-wrap">
                  <Badge variant="secondary" :class="RUN_STATUS_META[run.status].cls + ' text-[11px] font-medium'">
                    {{ t(RUN_STATUS_META[run.status].labelKey) }}
                  </Badge>
                  <span v-if="ownerLabel(run.started_by)" class="text-xs text-muted-foreground">{{ ownerLabel(run.started_by) }}</span>
                  <span class="text-xs text-muted-foreground">{{ formatDateTime(run.started_at, locale) }}</span>
                  <span v-if="run.finished_at" class="text-xs text-muted-foreground">→ {{ formatDateTime(run.finished_at, locale) }}</span>
                </div>
                <p class="text-sm truncate text-muted-foreground">{{ sourcesSummary(run) }}</p>
              </button>

              <Pagination :page="page" :page-size="PAGE_SIZE" :total="kbi.historyTotal" @update:page="(p) => (page = p)" />
            </template>
          </template>
        </template>
      </div>
    </DialogContent>
  </Dialog>
</template>
