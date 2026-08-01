<script setup lang="ts">
// TopicRecord is a read-only display card for one topic — shared by
// Черновик's ChangeList (row = the pending or published value, depending on
// changeType) and База знаний's RecordList (row = the published value,
// changeType absent). All writes happen through TopicForm.vue via the modal;
// this component only ever emits intent.
import { computed } from 'vue'
import { ListTree } from 'lucide-vue-next'
import type { TopicRow } from '@/types'
import type { ChangeType } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import MediaChip from './MediaChip.vue'
import { changedFields, mediaCount, stateForChange, type RecordState } from './shared'

const props = defineProps<{
  row: TopicRow
  liveRow?: TopicRow
  changeType?: ChangeType // absent on База знаний (published, no pending change)
  actions: KbAction[]
  busy?: boolean
}>()
defineEmits<{ edit: []; publish: []; cancel: []; delete: [] }>()

const state = computed<RecordState>(() => (props.changeType ? stateForChange(props.changeType) : 'published'))
const diff = computed(() => (props.changeType === 'updated' ? changedFields(props.row, props.liveRow, ['title', 'body_md']) : []))
</script>

<template>
  <RecordShell
    :icon="ListTree"
    label="Тема"
    :record-key="row.slug"
    :state="state"
    :actions="actions"
    :busy="busy"
    @edit="$emit('edit')"
    @publish="$emit('publish')"
    @cancel="$emit('cancel')"
    @delete="$emit('delete')"
  >
    <p class="text-sm font-medium">{{ row.title || '—' }}</p>
    <FieldDiffNote :show="diff.includes('title')" :was="liveRow?.title ?? ''" />
    <p class="text-sm text-foreground/80 whitespace-pre-line">{{ row.body_md || '—' }}</p>
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
