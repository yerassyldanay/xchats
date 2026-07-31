<script setup lang="ts">
// TopicRecord is the shared /playground (draft) + /knowledge-base (live) row
// for one topic — replaces the two pages' previously independent, duplicated
// markup (plan Task 14).
import { computed, reactive, watch } from 'vue'
import { ListTree } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import type { TopicRow } from '@/types'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import MediaChip from './MediaChip.vue'
import { changedFields, mediaCount, recordState } from './shared'

const props = defineProps<{
  mode: 'draft' | 'live'
  draftRow?: TopicRow
  liveRow?: TopicRow
}>()

const pg = usePlayground()

const row = computed(() => (props.mode === 'draft' ? props.draftRow : props.liveRow))
const busy = computed(() => (props.mode === 'draft' ? pg.busy : pg.liveBusy))
const state = computed(() => recordState(props.mode, !!props.liveRow))
const diff = computed(() => changedFields(props.draftRow, props.liveRow, ['title', 'body_md']))

const buf = reactive({ title: '', body_md: '' })
let seededFor = ''
watch(
  row,
  (r) => {
    if (!r || seededFor === r.id) return
    seededFor = r.id
    buf.title = r.title
    buf.body_md = r.body_md
  },
  { immediate: true }
)

function save() {
  const r = row.value
  if (!r) return
  if (props.mode === 'draft') pg.upsertTopic({ slug: r.slug, ...buf })
  else pg.liveUpsertTopic({ slug: r.slug, ...buf })
}
function approve() {
  if (row.value) pg.approveEntity('topics', row.value.slug)
}
function reject() {
  if (row.value) pg.deleteTopic(row.value.slug)
}
function del() {
  if (row.value) pg.liveDeleteTopic(row.value.slug)
}
</script>

<template>
  <RecordShell
    v-if="row"
    :icon="ListTree"
    label="Тема"
    :record-key="row.slug"
    :mode="mode"
    :state="state"
    :busy="busy"
    @save="save"
    @approve="approve"
    @reject="reject"
    @delete="del"
  >
    <Input v-model="buf.title" placeholder="Название" class="h-9" />
    <FieldDiffNote :show="diff.includes('title')" :was="liveRow?.title ?? ''" />
    <Textarea v-model="buf.body_md" rows="3" placeholder="Текст темы…" class="min-h-0 text-[14px]" />
    <FieldDiffNote :show="diff.includes('body_md')" :was="liveRow?.body_md ?? ''" />
    <div class="flex items-center gap-1.5 flex-wrap">
      <MediaChip label="Изображение" :count="mediaCount(row.featured_image)" />
      <MediaChip label="Иллюстрации" :count="mediaCount(row.illustration_images)" />
      <MediaChip label="Видео" :count="mediaCount(row.explainer_videos)" />
      <MediaChip label="Аудио" :count="mediaCount(row.narration_audio_files)" />
      <MediaChip label="Документы" :count="mediaCount(row.reference_documents)" />
    </div>
  </RecordShell>
</template>
