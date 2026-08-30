<script setup lang="ts">
// KbImportCard is one ingest tab's body — Ссылки or Файлы, kind picks which
// — hosted by KbIngestPanel. A dropzone/paste-URL input accumulates
// files/URLs LOCALLY (nothing is sent yet, same staging idea as
// Composer.vue's own files ref); provider/target-type/guidance render
// INLINE right below it — a previous version staged here then opened a
// separate KbImportDialog for those settings, which is gone now (no
// popups on this page at all). The provider <Select>'s options/
// availability come from useImportProviders, so this component never has
// to know the capability matrix itself.
// KbImportRunStatus renders once, shared by both tabs, in KbIngestPanel —
// not here.
//
// This is the codebase's first drag-and-drop surface — MediaFieldPicker.vue
// (single hidden <input type="file">, no drop target) is the closest prior
// art, extended here into an actual dropzone.
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, Link as LinkIcon, LoaderCircle, Upload, X } from 'lucide-vue-next'
import { useKbImport } from '@/stores/kbImport'
import { useImportProviders } from '@/composables/useImportProviders'
import { formatBytes } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const props = defineProps<{ kind: 'url' | 'file' }>()
const kbi = useKbImport()
const { t } = useI18n()

// Mirrors kbimport.TargetTypes (backend/internal/kbimport/enqueue.go)
// exactly — the operator-facing vocabulary the backend itself validates
// against (ErrValidation, 400, on anything else). assistant is absent: a
// persona is never inferred from imported content.
const TARGET_TYPES = ['auto', 'topics', 'products', 'tariffs', 'contacts', 'policies', 'delivery_zones'] as const
function targetTypeLabel(tt: string): string {
  return tt === 'auto' ? t('kb.import.targetTypeAuto') : t(`kb.entities.${tt}.plural`)
}

const pendingFiles = ref<File[]>([])
const pendingUrls = ref<string[]>([])
const urlInput = ref('')
const urlError = ref('')
const dragOver = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

// KB-11: mirrors kb_import.go's own limits (kbImportMaxFileBytes, the
// 10-file cap) exactly, enforced here BEFORE any network request — the
// backend previously was the first place a user learned an oversized file
// or an excessive count was rejected.
const MAX_FILES = 10
const MAX_FILE_BYTES = 50 * 1024 * 1024 // = kbImportMaxFileBytes (50 MiB) — an exact byte match, not a rounded display value
const fileError = ref('')

const provider = ref('native')
const targetType = ref('auto')
const guidance = ref('')

const kindRef = computed(() => props.kind)
const { options: providerOptions, hasStagedImage } = useImportProviders(kindRef, pendingFiles, provider)
const unavailableOptions = computed(() => providerOptions.value.filter((o) => !o.available))

const hasPending = computed(() => (props.kind === 'url' ? pendingUrls.value.length > 0 : pendingFiles.value.length > 0))

function addFiles(list: FileList | null) {
  if (!list) return
  fileError.value = ''
  const incoming = Array.from(list)
  const oversized = incoming.filter((f) => f.size > MAX_FILE_BYTES)
  const withinSize = incoming.filter((f) => f.size <= MAX_FILE_BYTES)

  const room = Math.max(0, MAX_FILES - pendingFiles.value.length)
  const overflowCount = Math.max(0, withinSize.length - room)
  pendingFiles.value.push(...withinSize.slice(0, room))

  const notices: string[] = []
  if (oversized.length) {
    notices.push(t('kb.import.filesTooLarge', { count: oversized.length, limit: formatBytes(MAX_FILE_BYTES), names: oversized.map((f) => f.name).join(', ') }))
  }
  if (overflowCount > 0) {
    notices.push(t('kb.import.tooManyFiles', { max: MAX_FILES, count: overflowCount }))
  }
  fileError.value = notices.join(' ')
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

// KB-15: statusUnknown (the last status load/refresh failed) must not
// silently enable a submit that might 409 against a run this client just
// can't currently see — see kbImport.ts's own doc comment on the getter.
const canSubmit = computed(
  () =>
    hasPending.value &&
    !kbi.isActive &&
    !kbi.statusUnknown &&
    providerOptions.value.some((o) => o.id === provider.value && o.available)
)
async function submit() {
  if (!canSubmit.value) return
  const ok = await kbi.submit({
    provider: provider.value,
    targetType: targetType.value,
    guidance: guidance.value,
    urls: pendingUrls.value,
    files: pendingFiles.value,
  })
  if (ok) {
    pendingFiles.value = []
    pendingUrls.value = []
    // provider/targetType deliberately survive a successful submit — likely
    // reused for the next similar batch; guidance is run-specific enough
    // that carrying it forward would read as a stale leftover instruction.
    guidance.value = ''
  }
}
</script>

<template>
  <div class="space-y-4">
    <div
      v-if="kind === 'file'"
      class="rounded-lg border border-dashed p-6 text-center transition"
      :class="dragOver ? 'border-primary bg-primary/5' : 'border-border'"
      @dragover.prevent="dragOver = true"
      @dragleave.prevent="dragOver = false"
      @drop.prevent="onDrop"
    >
      <Upload class="w-6 h-6 mx-auto text-muted-foreground" />
      <p class="mt-2 text-sm text-muted-foreground">{{ t('kb.import.dropHint') }}</p>
      <Button variant="outline" size="sm" class="mt-3" data-testid="kb-import-browse" @click="fileInput?.click()">{{ t('kb.import.browseButton') }}</Button>
      <input ref="fileInput" type="file" multiple class="hidden" data-testid="kb-import-file-input" @change="onFilePick" />
      <p class="mt-2 text-[11px] text-muted-foreground" data-testid="kb-import-file-limits">
        {{ t('kb.import.fileLimits', { max: MAX_FILES, limit: formatBytes(MAX_FILE_BYTES) }) }}
      </p>
      <p v-if="fileError" class="mt-2 flex items-start gap-1.5 text-left text-xs text-destructive" data-testid="kb-import-file-error">
        <CircleAlert class="w-3.5 h-3.5 shrink-0 mt-0.5" /> {{ fileError }}
      </p>
    </div>

    <div v-else class="flex items-start gap-2">
      <div class="flex-1">
        <div class="relative">
          <LinkIcon class="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input
            v-model="urlInput"
            type="url"
            :placeholder="t('kb.import.urlPlaceholder')"
            class="pl-8"
            data-testid="kb-import-url-input"
            @keydown.enter.prevent="addUrl"
          />
        </div>
        <p v-if="urlError" class="mt-1 text-xs text-destructive">{{ urlError }}</p>
      </div>
      <Button variant="outline" size="sm" data-testid="kb-import-add-url" @click="addUrl">{{ t('kb.import.addUrl') }}</Button>
    </div>

    <div v-if="hasPending" class="flex flex-wrap gap-2">
      <template v-if="kind === 'file'">
        <span v-for="(f, i) in pendingFiles" :key="'f' + i" class="flex items-center gap-1.5 rounded-full bg-muted border border-border px-3 py-1 text-xs">
          <Upload class="w-3.5 h-3.5 text-muted-foreground" /> {{ f.name }}
          <button type="button" class="text-muted-foreground hover:text-destructive" @click="removeFile(i)">
            <X class="w-3.5 h-3.5" />
          </button>
        </span>
      </template>
      <template v-else>
        <span v-for="(u, i) in pendingUrls" :key="'u' + i" class="flex items-center gap-1.5 rounded-full bg-muted border border-border px-3 py-1 text-xs max-w-xs">
          <LinkIcon class="w-3.5 h-3.5 text-muted-foreground shrink-0" /> <span class="truncate">{{ u }}</span>
          <button type="button" class="text-muted-foreground hover:text-destructive shrink-0" @click="removeUrl(i)">
            <X class="w-3.5 h-3.5" />
          </button>
        </span>
      </template>
    </div>

    <p v-if="hasStagedImage" class="flex items-start gap-2 text-xs text-muted-foreground">
      <CircleAlert class="w-3.5 h-3.5 shrink-0 mt-0.5" /> {{ t('kb.import.visionGapNotice') }}
    </p>

    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div>
        <label class="text-xs font-medium text-muted-foreground">{{ t('kb.import.provider') }}</label>
        <Select v-model="provider">
          <SelectTrigger class="mt-1.5" data-testid="kb-import-provider">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              <SelectItem v-for="opt in providerOptions" :key="opt.id" :value="opt.id" :disabled="!opt.available">
                {{ opt.displayName }} <span class="text-muted-foreground">— {{ opt.capsHint }}</span>
              </SelectItem>
            </SelectGroup>
          </SelectContent>
        </Select>
        <ul v-if="unavailableOptions.length" class="mt-1.5 space-y-0.5">
          <li v-for="opt in unavailableOptions" :key="opt.id" class="text-[11px] text-muted-foreground">
            {{ opt.displayName }}: {{ opt.reason }}
            <RouterLink v-if="opt.reasonKey === 'noCredential'" :to="{ name: 'settings' }" class="text-primary hover:underline">
              {{ t('kb.import.unavailable.settingsLink') }}
            </RouterLink>
          </li>
        </ul>
      </div>

      <div>
        <label class="text-xs font-medium text-muted-foreground">{{ t('kb.import.targetType') }}</label>
        <Select v-model="targetType">
          <SelectTrigger class="mt-1.5" data-testid="kb-import-target-type">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              <SelectItem v-for="tt in TARGET_TYPES" :key="tt" :value="tt">{{ targetTypeLabel(tt) }}</SelectItem>
            </SelectGroup>
          </SelectContent>
        </Select>
      </div>

      <div class="sm:col-span-2">
        <label class="text-xs font-medium text-muted-foreground">{{ t('kb.import.guidance') }}</label>
        <Textarea
          v-model="guidance"
          rows="2"
          maxlength="2000"
          :placeholder="t('kb.import.guidancePlaceholder')"
          class="min-h-0 text-[14px] mt-1.5"
          data-testid="kb-import-guidance"
        />
      </div>
    </div>

    <div class="flex items-center gap-2">
      <Button size="sm" :disabled="!canSubmit || kbi.submitting" data-testid="kb-import-submit" @click="submit">
        <LoaderCircle v-if="kbi.submitting" class="w-4 h-4 animate-spin" /> {{ t('kb.import.submitButton') }}
      </Button>
      <span v-if="kbi.statusUnknown" class="flex items-center gap-1.5 text-xs text-destructive" data-testid="kb-import-status-unknown">
        {{ t('kb.import.statusUnknownNotice') }}
        <button type="button" class="font-medium underline underline-offset-2 shrink-0" data-testid="kb-import-retry-status" @click="kbi.loadLatest()">
          {{ t('kb.import.retryLoadStatus') }}
        </button>
      </span>
      <span v-else-if="kbi.isActive" class="text-xs text-muted-foreground">{{ t('kb.import.activeRunNotice') }}</span>
    </div>

    <p v-if="kbi.error" class="text-sm text-destructive">{{ kbi.error }}</p>
  </div>
</template>
