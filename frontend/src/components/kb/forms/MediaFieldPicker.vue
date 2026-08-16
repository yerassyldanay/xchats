<script setup lang="ts">
// MediaFieldPicker is the editable counterpart to records/MediaStrip.vue:
// chips for what's currently attached (each with an X to detach), an
// "attach existing" select over already-uploaded org materials, and a
// hidden file input to upload a brand new one. Mounted once per media
// column in each *Form.vue (ProductForm etc.) — see those files for the
// v-model wiring.
//
// modelValue is intentionally the UNION of both shapes a media column can
// take (string | string[] | null) rather than a generic component: every
// *Form.vue already casts at its own call site the same way TariffForm.vue
// casts a Switch's boolean (see that file), which is simpler than a
// generic SFC for five call sites. `multiple` tells this component which
// half of the union it is actually operating on at runtime.
//
// Every emit sends the picker's FULL current selection, never a partial
// patch — that is what lets the owning form's payload() always include an
// explicit value for this field (see payloads.ts's top doc comment): the
// backend's absent-means-unchanged contract exists for MCP callers that
// never touch media, not for this picker, which is the one browser surface
// that DOES touch it and so always states its full intent.
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { LoaderCircle, Upload, X } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { kindOfMime, mediaFieldKind, mediaIds, type MediaKind } from '@/components/kb/records/shared'
import MediaThumb from '@/components/kb/records/MediaThumb.vue'

const props = defineProps<{
  label: string
  field: string
  multiple: boolean
  modelValue: string | string[] | null
}>()
const emit = defineEmits<{ 'update:modelValue': [string | string[] | null] }>()

const pg = usePlayground()
const { t } = useI18n()

const selectedIds = computed(() => mediaIds(props.modelValue))
const kind = computed(() => mediaFieldKind(props.field))

// availableMaterials is "attach existing": every org material with actual
// bytes (has_content), matching this field's kind, that a customer-facing
// reply could ever cite (customer_visibility !== 'invisible' — pg.live.materials
// is the WHOLE kbd_materials table, including legacy url/text rows and
// provenance-only citations that were never meant to be attachable), and
// not already selected.
const availableMaterials = computed(() =>
  Object.values(pg.materialsById)
    .filter((m) => m.has_content && kindOfMime(m.mime_type) === kind.value && m.customer_visibility !== 'invisible')
    .filter((m) => !selectedIds.value.includes(m.id))
)

const ACCEPT_BY_KIND: Record<MediaKind | '', string> = {
  image: 'image/*',
  video: 'video/*',
  audio: 'audio/*',
  document: '.pdf,.doc,.docx,.txt,.rtf,.csv,application/*,text/*',
  '': '*/*',
}
const accept = computed(() => ACCEPT_BY_KIND[kind.value])

function attach(id: string) {
  if (props.multiple) {
    if (!selectedIds.value.includes(id)) emit('update:modelValue', [...selectedIds.value, id])
  } else {
    emit('update:modelValue', id)
  }
}
function detach(id: string) {
  emit('update:modelValue', props.multiple ? selectedIds.value.filter((x) => x !== id) : null)
}
function onAttachExisting(e: Event) {
  const select = e.target as HTMLSelectElement
  const id = select.value
  select.value = ''
  if (id) attach(id)
}

const uploadError = ref('')
async function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = '' // always reset, so picking the same file twice still fires 'change'
  if (!file) return
  uploadError.value = ''
  const id = await pg.uploadMaterial(file)
  if (id) {
    attach(id)
  } else {
    uploadError.value = pg.error || t('kb.mediaPicker.uploadFailed')
  }
}
</script>

<template>
  <div class="space-y-1.5">
    <span class="text-xs font-medium text-muted-foreground">{{ label }}</span>
    <div class="flex flex-wrap items-center gap-2">
      <div v-for="id in selectedIds" :key="id" class="relative">
        <MediaThumb :id="id" :label="label" />
        <button
          type="button"
          :aria-label="t('kb.mediaPicker.detach')"
          class="absolute -top-1.5 -right-1.5 rounded-full bg-background border border-border shadow-xs p-0.5 text-muted-foreground hover:text-destructive hover:border-destructive transition"
          @click="detach(id)"
        >
          <X class="w-3 h-3" />
        </button>
      </div>

      <select
        v-if="availableMaterials.length"
        class="h-8 max-w-[220px] text-xs rounded-md border border-border bg-background px-2 text-muted-foreground focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
        value=""
        @change="onAttachExisting"
      >
        <option value="" disabled>{{ t('kb.mediaPicker.attachExisting') }}</option>
        <option v-for="m in availableMaterials" :key="m.id" :value="m.id">{{ m.filename || m.id }}</option>
      </select>

      <label
        class="inline-flex items-center gap-1.5 h-8 px-2.5 rounded-md border border-dashed border-border text-xs text-muted-foreground hover:bg-muted transition"
        :class="pg.uploading ? 'opacity-60 cursor-not-allowed' : 'cursor-pointer'"
      >
        <LoaderCircle v-if="pg.uploading" class="w-3.5 h-3.5 animate-spin" />
        <Upload v-else class="w-3.5 h-3.5" />
        <span>{{ pg.uploading ? t('kb.mediaPicker.uploading') : t('kb.mediaPicker.upload') }}</span>
        <input type="file" class="hidden" :accept="accept" :disabled="pg.uploading" @change="onFileChange" />
      </label>
    </div>
    <p v-if="uploadError" class="text-xs text-destructive">{{ uploadError }}</p>
  </div>
</template>
