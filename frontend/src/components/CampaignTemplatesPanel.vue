<script setup lang="ts">
// CampaignTemplatesPanel is CAM-14's Templates tab body (Campaigns.vue,
// ?tab=templates) — browse/search/filter the org's shared message library,
// create or edit one, and soft-archive/restore. Archiving is deliberately
// NOT behind a confirmation dialog (unlike Campaign Stop or a KB delete):
// it is a reversible move between two filtered views, never a destructive
// action — see the backend migration's own doc comment.
import { onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Archive, ArchiveRestore, CircleAlert, FileText, LoaderCircle, Pencil, Plus, Search } from 'lucide-vue-next'
import { useCampaignTemplates } from '@/stores/campaignTemplates'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import Pagination from './Pagination.vue'
import CampaignTemplateFormDialog from './CampaignTemplateFormDialog.vue'
import type { CampaignTemplate } from '@/types'

const { t } = useI18n()
const templates = useCampaignTemplates()

const PAGE_SIZE = 20
const filter = ref<'active' | 'archived'>('active')
const search = ref('')
const page = ref(1)
const actionError = ref('')
const actioningId = ref('')

async function load() {
  await templates.list({ archived: filter.value === 'archived', q: search.value.trim(), page: page.value, pageSize: PAGE_SIZE })
}
onMounted(load)

watch(filter, () => {
  page.value = 1
  void load()
})

// Debounced like the wizard's own recipient-preview input (CAM-04) — a
// search-as-you-type field that re-fetched on every keystroke would fire
// far more than necessary.
const SEARCH_DEBOUNCE_MS = 300
let searchTimer: ReturnType<typeof setTimeout> | null = null
watch(search, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    page.value = 1
    void load()
  }, SEARCH_DEBOUNCE_MS)
})

async function changePage(p: number) {
  page.value = p
  await load()
}

const formOpen = ref(false)
const editing = ref<CampaignTemplate | null>(null)
function openCreate() {
  editing.value = null
  formOpen.value = true
}
function openEdit(tmpl: CampaignTemplate) {
  editing.value = tmpl
  formOpen.value = true
}

// Computed entirely here, not inline in the template, so the template's own
// source text never contains a literal {{/}} pair for the Vue compiler's
// mustache scanner to trip over (see CampaignRecipientPreviewTable.vue's
// own chipText, the established precedent for this exact problem).
function variableTag(v: string): string {
  return `{{${v}}}`
}

async function toggleArchive(tmpl: CampaignTemplate) {
  actionError.value = ''
  actioningId.value = tmpl.id
  try {
    if (filter.value === 'active') await templates.archive(tmpl.id)
    else await templates.restore(tmpl.id)
  } catch (e) {
    actionError.value = e instanceof ApiError ? e.message : t('campaigns.templates.errActionFailed')
  } finally {
    actioningId.value = ''
  }
}
</script>

<template>
  <div class="mt-6">
    <div class="flex items-center gap-2 flex-wrap">
      <div class="relative flex-1 min-w-[200px] max-w-sm">
        <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
        <Input v-model="search" :placeholder="t('campaigns.templates.searchPlaceholder')" class="pl-8" data-testid="template-search" />
      </div>
      <div class="flex items-center gap-1 rounded-lg border border-border p-0.5">
        <button
          type="button"
          class="rounded-md px-3 py-1.5 text-xs font-medium transition"
          :class="filter === 'active' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'"
          data-testid="template-filter-active"
          @click="filter = 'active'"
        >
          {{ t('campaigns.templates.filterActive') }}
        </button>
        <button
          type="button"
          class="rounded-md px-3 py-1.5 text-xs font-medium transition"
          :class="filter === 'archived' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'"
          data-testid="template-filter-archived"
          @click="filter = 'archived'"
        >
          {{ t('campaigns.templates.filterArchived') }}
        </button>
      </div>
      <Button type="button" class="ml-auto" data-testid="template-new" @click="openCreate">
        <Plus class="w-4 h-4" /> {{ t('campaigns.templates.newTemplate') }}
      </Button>
    </div>

    <p v-if="actionError" class="mt-3 flex items-center gap-1.5 text-sm text-destructive">
      <CircleAlert class="w-4 h-4 shrink-0" /> {{ actionError }}
    </p>

    <p v-if="templates.loading" class="mt-8 text-sm text-muted-foreground">{{ t('campaigns.list.loading') }}</p>

    <div v-else-if="templates.templates.length === 0" class="mt-12 text-center">
      <FileText class="w-10 h-10 mx-auto text-muted-foreground/50" />
      <p class="mt-3 text-sm font-medium">
        {{ filter === 'active' ? t('campaigns.templates.emptyActive') : t('campaigns.templates.emptyArchived') }}
      </p>
    </div>

    <div v-else class="mt-6 space-y-2">
      <div v-for="tmpl in templates.templates" :key="tmpl.id" class="rounded-lg border border-border p-4" data-testid="template-card">
        <div class="flex items-start gap-3">
          <div class="min-w-0 flex-1">
            <div class="font-medium truncate">{{ tmpl.name }}</div>
            <p class="mt-1 text-xs text-muted-foreground line-clamp-2 whitespace-pre-line">{{ tmpl.message_body }}</p>
            <div v-if="tmpl.variables.length" class="mt-2 flex flex-wrap gap-1">
              <Badge v-for="v in tmpl.variables" :key="v" variant="secondary" class="font-mono text-[11px]">{{ variableTag(v) }}</Badge>
            </div>
          </div>
          <div class="flex items-center gap-1.5 shrink-0">
            <Button variant="outline" size="sm" data-testid="template-edit" @click="openEdit(tmpl)">
              <Pencil class="w-3.5 h-3.5" /> {{ t('campaigns.templates.edit') }}
            </Button>
            <Button variant="ghost" size="sm" :disabled="actioningId === tmpl.id" data-testid="template-toggle-archive" @click="toggleArchive(tmpl)">
              <LoaderCircle v-if="actioningId === tmpl.id" class="w-3.5 h-3.5 animate-spin" />
              <Archive v-else-if="filter === 'active'" class="w-3.5 h-3.5" />
              <ArchiveRestore v-else class="w-3.5 h-3.5" />
              {{ filter === 'active' ? t('campaigns.templates.archive') : t('campaigns.templates.restore') }}
            </Button>
          </div>
        </div>
      </div>
      <Pagination :page="page" :page-size="PAGE_SIZE" :total="templates.total" @update:page="changePage" />
    </div>

    <CampaignTemplateFormDialog :open="formOpen" :template="editing" @update:open="formOpen = $event" @saved="load" />
  </div>
</template>
