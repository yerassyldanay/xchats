<script setup lang="ts">
// MediaStrip replaces MediaChip's read-only "N attached" count with the
// actual media: an image thumbnail (opens MediaLightbox), an inline
// <video>, or a filename/size download row for anything else — the record
// card wiring (props ids -> which materials) is what MediaChip's own doc
// comment deferred to "the MCP KB Manager widget's and the Файлы tab's job";
// this is that job, now that GET /kb/materials/:id/content exists. The
// per-material render body lives in MediaThumb.vue, shared with
// MediaFieldPicker.vue's editable chips so the two can never diverge.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { FileText, Image as ImageIcon, Paperclip, Video as VideoIcon } from 'lucide-vue-next'
import { mediaFieldKind, mediaIds, type MediaKind } from './shared'
import MediaThumb from './MediaThumb.vue'

// field is the canonical column name (e.g. 'gallery_images') — passed
// instead of a hand-written kind so mediaFieldKind (shared.ts) stays the one
// place that mirrors the backend's field->kind registry, instead of a third
// copy of that matrix spread across every *Record.vue call site.
const props = defineProps<{ label: string; field: string; ids: string | string[] | null | undefined }>()
const { t } = useI18n()

const list = computed(() => mediaIds(props.ids))
const fieldKind = computed(() => mediaFieldKind(props.field))

// emptyIcon is the kind-appropriate glyph for a slot with nothing attached —
// Paperclip covers both 'audio' (no registry field uses it today) and an
// unrecognized field, same fallback MediaThumb already uses for an
// unresolvable attached id.
const EMPTY_ICON: Record<MediaKind | '', typeof ImageIcon> = {
  image: ImageIcon,
  video: VideoIcon,
  audio: Paperclip,
  document: FileText,
  '': Paperclip,
}
const emptyIcon = computed(() => EMPTY_ICON[fieldKind.value])
</script>

<template>
  <div class="space-y-1.5">
    <span class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground/75">{{ label }}</span>
    <div v-if="list.length" class="flex flex-wrap items-center gap-2">
      <MediaThumb v-for="id in list" :key="id" :id="id" :label="label" />
    </div>
    <span
      v-else
      class="inline-flex items-center gap-1 rounded-full border border-dashed border-border bg-muted/30 px-2 py-0.5 text-[11px] text-muted-foreground"
    >
      <component :is="emptyIcon" class="w-3 h-3" /> {{ t('kb.mediaStrip.empty') }}
    </span>
  </div>
</template>
