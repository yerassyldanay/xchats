<script setup lang="ts">
// TopicRecord is a read-only display card for one topic — reused by
// Черновик (a pending ChangeEntry, actions Изменить/Опубликовать/Отменить)
// and Знаний база (a published row, actions Изменить/Удалить). It never
// calls the store or holds an edit buffer: editing happens in a modal
// (forms/TopicForm.vue via useKbModal), this component only displays and
// relays action clicks.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ListTree } from 'lucide-vue-next'
import type { TopicRow } from '@/types'
import type { ChangeType } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import RecordShell from './RecordShell.vue'
import RecordField from './RecordField.vue'
import MediaFieldsRow from './MediaFieldsRow.vue'
import MediaStrip from './MediaStrip.vue'
import { changedFields, stateForChange } from './shared'

const props = defineProps<{
  row: TopicRow
  liveRow?: TopicRow
  changeType?: ChangeType // absent on Знаний база — a published row has no draft context
  pendingMark?: 'updated' | 'removed' // Знаний база only — see RecordShell's own doc comment
  actions: KbAction[]
  busy?: boolean
  blockedNote?: string
}>()

defineEmits<{ edit: []; publish: []; cancel: []; delete: [] }>()
const { t } = useI18n()

const state = computed(() => (props.changeType ? stateForChange(props.changeType) : 'published'))
const diff = computed(() => changedFields(props.row, props.liveRow, ['title', 'body_md']))
</script>

<template>
  <RecordShell
    :icon="ListTree"
    :label="t('kb.entities.topics.singular')"
    :heading="row.title || row.slug"
    :record-key="row.slug"
    :state="state"
    :pending-mark="pendingMark"
    :actions="actions"
    :busy="busy"
    :blocked-note="blockedNote"
    :updated-at="row.updated_at"
    @edit="$emit('edit')"
    @publish="$emit('publish')"
    @cancel="$emit('cancel')"
    @delete="$emit('delete')"
  >
    <!-- the heading already shows the current title; this only appears when it just changed -->
    <RecordField v-if="diff.includes('title')" :label="t('kb.fields.title')" :value="row.title" diff-show :diff-was="liveRow?.title" span />
    <RecordField :label="t('kb.fields.body')" :diff-show="diff.includes('body_md')" :diff-was="liveRow?.body_md" span>
      <span class="whitespace-pre-line">{{ row.body_md || '—' }}</span>
    </RecordField>

    <template #media>
      <MediaFieldsRow>
        <MediaStrip :label="t('kb.media.image')" field="featured_image" :ids="row.featured_image" />
        <MediaStrip :label="t('kb.media.illustrations')" field="illustration_images" :ids="row.illustration_images" />
        <MediaStrip :label="t('kb.media.videos')" field="explainer_videos" :ids="row.explainer_videos" />
        <MediaStrip :label="t('kb.media.documents')" field="reference_documents" :ids="row.reference_documents" />
      </MediaFieldsRow>
    </template>
  </RecordShell>
</template>
