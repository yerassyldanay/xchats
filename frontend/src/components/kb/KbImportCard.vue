<script setup lang="ts">
// KbImportCard is the Знаний база "Импорт" tab's entry point: a dropzone +
// paste-URL input accumulate files/URLs LOCALLY (nothing is sent yet, same
// staging idea as Composer.vue's own files ref), "Начать импорт" opens
// KbImportDialog for the run's provider/target_type/guidance, and the
// tracked run's live progress renders via KbImportRunStatus underneath.
// This is the codebase's first drag-and-drop surface — MediaFieldPicker.vue
// (single hidden <input type="file">, no drop target) is the closest prior
// art, extended here into an actual dropzone.
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Link as LinkIcon, Upload, X } from 'lucide-vue-next'
import { useKbImport } from '@/stores/kbImport'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import KbImportDialog from './KbImportDialog.vue'
import KbImportRunStatus from './KbImportRunStatus.vue'

const kbi = useKbImport()
const { t } = useI18n()

onMounted(async () => {
  await kbi.loadLatest()
  kbi.startRealtime()
})
onBeforeUnmount(() => kbi.stopRealtime())

const pendingFiles = ref<File[]>([])
const pendingUrls = ref<string[]>([])
const urlInput = ref('')
const urlError = ref('')
const dragOver = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const dialogOpen = ref(false)

const hasPending = computed(() => pendingFiles.value.length > 0 || pendingUrls.value.length > 0)

function addFiles(list: FileList | null) {
  if (!list) return
  pendingFiles.value.push(...Array.from(list))
}
function onFilePick(e: Event) {
  const input = e.target as HTMLInputElement
  addFiles(input.files)
  input.value = '' // reset so picking the same file(s) again still fires 'change'
}
function onDrop(e: DragEvent) {
  dragOver.value = false
  addFiles(e.dataTransfer?.files ?? null)
}
function removeFile(i: number) {
  pendingFiles.value.splice(i, 1)
}

function isHttpURL(v: string): boolean {
  try {
    return ['http:', 'https:'].includes(new URL(v).protocol)
  } catch {
    return false
  }
}
function addUrl() {
  const v = urlInput.value.trim()
  urlError.value = ''
  if (!v) return
  if (!isHttpURL(v)) {
    urlError.value = t('kb.import.invalidUrl')
    return
  }
  if (!pendingUrls.value.includes(v)) pendingUrls.value.push(v)
  urlInput.value = ''
}
function removeUrl(i: number) {
  pendingUrls.value.splice(i, 1)
}

function openDialog() {
  if (!hasPending.value || kbi.isActive) return
  dialogOpen.value = true
}
async function handleSubmit(form: { provider: string; targetType: string; guidance: string }) {
  const ok = await kbi.submit({ ...form, urls: pendingUrls.value, files: pendingFiles.value })
  if (ok) {
    pendingFiles.value = []
    pendingUrls.value = []
    dialogOpen.value = false
  }
}
</script>

<template>
  <div class="space-y-4">
    <div
      class="rounded-lg border border-dashed p-6 text-center transition"
      :class="dragOver ? 'border-primary bg-primary/5' : 'border-border'"
      @dragover.prevent="dragOver = true"
      @dragleave.prevent="dragOver = false"
      @drop.prevent="onDrop"
    >
      <Upload class="w-6 h-6 mx-auto text-muted-foreground" />
      <p class="mt-2 text-sm text-muted-foreground">{{ t('kb.import.dropHint') }}</p>
      <Button variant="outline" size="sm" class="mt-3" @click="fileInput?.click()">{{ t('kb.import.browseButton') }}</Button>
      <input ref="fileInput" type="file" multiple class="hidden" @change="onFilePick" />
    </div>

    <div class="flex items-start gap-2">
      <div class="flex-1">
        <div class="relative">
          <LinkIcon class="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input v-model="urlInput" type="url" :placeholder="t('kb.import.urlPlaceholder')" class="pl-8" @keydown.enter.prevent="addUrl" />
        </div>
        <p v-if="urlError" class="mt-1 text-xs text-destructive">{{ urlError }}</p>
      </div>
      <Button variant="outline" size="sm" @click="addUrl">{{ t('kb.import.addUrl') }}</Button>
    </div>

    <div v-if="hasPending" class="flex flex-wrap gap-2">
      <span v-for="(f, i) in pendingFiles" :key="'f' + i" class="flex items-center gap-1.5 rounded-full bg-muted border border-border px-3 py-1 text-xs">
        <Upload class="w-3.5 h-3.5 text-muted-foreground" /> {{ f.name }}
        <button type="button" class="text-muted-foreground hover:text-destructive" @click="removeFile(i)">
          <X class="w-3.5 h-3.5" />
        </button>
      </span>
      <span v-for="(u, i) in pendingUrls" :key="'u' + i" class="flex items-center gap-1.5 rounded-full bg-muted border border-border px-3 py-1 text-xs max-w-xs">
        <LinkIcon class="w-3.5 h-3.5 text-muted-foreground shrink-0" /> <span class="truncate">{{ u }}</span>
        <button type="button" class="text-muted-foreground hover:text-destructive shrink-0" @click="removeUrl(i)">
          <X class="w-3.5 h-3.5" />
        </button>
      </span>
    </div>

    <div class="flex items-center gap-2">
      <Button size="sm" :disabled="!hasPending || kbi.isActive" @click="openDialog">{{ t('kb.import.submitButton') }}</Button>
      <span v-if="kbi.isActive" class="text-xs text-muted-foreground">{{ t('kb.import.activeRunNotice') }}</span>
    </div>

    <p v-if="kbi.error && !dialogOpen" class="text-sm text-destructive">{{ kbi.error }}</p>

    <KbImportRunStatus v-if="kbi.current" :run="kbi.current" />

    <KbImportDialog
      :open="dialogOpen"
      :busy="kbi.submitting"
      :error="dialogOpen ? kbi.error : ''"
      :pending-file-count="pendingFiles.length"
      :pending-url-count="pendingUrls.length"
      @update:open="(v) => (dialogOpen = v)"
      @submit="handleSubmit"
    />
  </div>
</template>
