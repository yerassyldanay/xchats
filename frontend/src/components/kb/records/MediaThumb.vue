<script setup lang="ts">
// MediaThumb is the per-material render body MediaStrip (read-only card
// display) and MediaFieldPicker (editable picker chips) both need — an
// image thumbnail (opens MediaLightbox), an inline <video>, or a filename/
// size download row for anything else, plus the "unavailable"/"processing"
// placeholder states. Extracted out of MediaStrip so the two call sites
// render byte-identical markup instead of two copies that could drift.
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { FileText, LoaderCircle, Paperclip } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { formatBytes } from '@/lib/format'
import { kindOfMime, materialContentURL } from './shared'
import MediaLightbox from './MediaLightbox.vue'

// label is the fallback filename/lightbox title when the material itself
// carries none (mirrors MediaStrip's own prop of the same name).
const props = defineProps<{ id: string; label: string }>()
const pg = usePlayground()
const { t } = useI18n()

const material = computed(() => pg.materialsById[props.id])
const kind = computed(() => {
  const m = material.value
  if (!m) return ''
  return m.media_kind || kindOfMime(m.mime_type)
})

const lightboxOpen = ref(false)
</script>

<template>
  <span class="contents">
    <span
      v-if="!material"
      class="inline-flex items-center gap-1 rounded-full border border-dashed border-border bg-muted/30 px-2 py-0.5 text-[11px] text-muted-foreground"
    >
      <Paperclip class="w-3 h-3" /> {{ t('kb.mediaStrip.unavailable') }}
    </span>
    <span
      v-else-if="!material.has_content"
      class="inline-flex items-center gap-1 rounded-full border border-border bg-muted/50 px-2 py-0.5 text-[11px] text-muted-foreground"
    >
      <LoaderCircle class="w-3 h-3 animate-spin" /> {{ t('kb.mediaStrip.processing') }}
    </span>
    <button
      v-else-if="kind === 'image'"
      type="button"
      :aria-label="t('kb.mediaStrip.openImage')"
      class="block w-16 h-16 rounded-lg border border-border overflow-hidden shrink-0 hover:ring-2 hover:ring-primary/40 transition"
      @click="lightboxOpen = true"
    >
      <img :src="materialContentURL(id)" loading="lazy" decoding="async" class="w-full h-full object-cover" />
    </button>
    <video
      v-else-if="kind === 'video'"
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
      <span class="truncate">{{ material.filename || label }}</span>
      <span v-if="material.size_bytes" class="text-muted-foreground shrink-0">({{ formatBytes(material.size_bytes) }})</span>
    </a>
    <MediaLightbox v-model:open="lightboxOpen" :src="materialContentURL(id)" :label="material?.filename || label" />
  </span>
</template>
