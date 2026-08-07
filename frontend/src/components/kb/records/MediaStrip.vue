<script setup lang="ts">
// MediaStrip replaces MediaChip's read-only "N attached" count with the
// actual media: an image thumbnail (opens MediaLightbox), an inline
// <video>, or a filename/size download row for anything else — the record
// card wiring (props ids -> which materials) is what MediaChip's own doc
// comment deferred to "the MCP KB Manager widget's and the Файлы tab's job";
// this is that job, now that GET /kb/materials/:id/content exists.
//
// Unlike RecordShell (deliberately store-free, see its own doc comment),
// this DOES read usePlayground() directly — resolving an id to a material's
// filename/mime/has_content IS this component's whole job, and every caller
// already sits inside a component tree that has the store initialized (the
// KB pages load it in onMounted before any record renders).
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { FileText, LoaderCircle, Paperclip } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { formatBytes } from '@/lib/format'
import { kindOfMime, materialContentURL, mediaIds } from './shared'
import MediaLightbox from './MediaLightbox.vue'

const props = defineProps<{ label: string; ids: string | string[] | null | undefined }>()
const pg = usePlayground()
const { t } = useI18n()

const list = computed(() => mediaIds(props.ids))
function kindOf(id: string): string {
  const m = pg.materialsById[id]
  if (!m) return ''
  return m.media_kind || kindOfMime(m.mime_type)
}

const lightboxOpen = ref(false)
const lightboxSrc = ref<string | null>(null)
const lightboxLabel = ref('')
function openLightbox(id: string) {
  lightboxSrc.value = materialContentURL(id)
  lightboxLabel.value = pg.materialsById[id]?.filename || props.label
  lightboxOpen.value = true
}
</script>

<template>
  <div v-if="list.length" class="space-y-1.5">
    <span class="text-xs font-medium text-muted-foreground">{{ label }}</span>
    <div class="flex flex-wrap items-center gap-2">
      <template v-for="id in list" :key="id">
        <span
          v-if="!pg.materialsById[id]"
          class="inline-flex items-center gap-1 rounded-full border border-dashed border-border bg-muted/30 px-2 py-0.5 text-[11px] text-muted-foreground"
        >
          <Paperclip class="w-3 h-3" /> {{ t('kb.mediaStrip.unavailable') }}
        </span>
        <span
          v-else-if="!pg.materialsById[id].has_content"
          class="inline-flex items-center gap-1 rounded-full border border-border bg-muted/50 px-2 py-0.5 text-[11px] text-muted-foreground"
        >
          <LoaderCircle class="w-3 h-3 animate-spin" /> {{ t('kb.mediaStrip.processing') }}
        </span>
        <button
          v-else-if="kindOf(id) === 'image'"
          type="button"
          :aria-label="t('kb.mediaStrip.openImage')"
          class="block w-16 h-16 rounded-lg border border-border overflow-hidden shrink-0 hover:ring-2 hover:ring-primary/40 transition"
          @click="openLightbox(id)"
        >
          <img :src="materialContentURL(id)" loading="lazy" decoding="async" class="w-full h-full object-cover" />
        </button>
        <video
          v-else-if="kindOf(id) === 'video'"
          :src="materialContentURL(id)"
          controls
          preload="metadata"
          class="h-16 rounded-lg border border-border bg-black"
        />
        <a
          v-else
          :href="materialContentURL(id)"
          target="_blank"
          rel="noopener"
          class="inline-flex items-center gap-1.5 max-w-[220px] rounded-full border border-border bg-muted/50 px-2.5 py-1 text-[11px] text-foreground hover:bg-muted transition"
        >
          <FileText class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
          <span class="truncate">{{ pg.materialsById[id].filename || label }}</span>
          <span v-if="pg.materialsById[id].size_bytes" class="text-muted-foreground shrink-0">({{ formatBytes(pg.materialsById[id].size_bytes) }})</span>
        </a>
      </template>
    </div>
    <MediaLightbox v-model:open="lightboxOpen" :src="lightboxSrc" :label="lightboxLabel" />
  </div>
</template>
