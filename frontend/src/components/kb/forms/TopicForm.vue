<script setup lang="ts">
// TopicForm is always mounted on /knowledge-base (and on any Черновик card
// with an Изменить action) and simply stays closed until useKbModal's shared
// session targets kind 'topics' — see useKbModal.ts's doc comment for why
// the session is module-level singleton state rather than a prop.
import { reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useKbModal } from '@/composables/useKbModal'
import type { TopicRow } from '@/types'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import KbFormDialog from './KbFormDialog.vue'
import MediaFieldPicker from './MediaFieldPicker.vue'
import type { TopicPayload } from './payloads'

const modal = useKbModal()
const { t } = useI18n()

const buf = reactive({
  slug: '', title: '', body_md: '',
  featured_image: null as string | null,
  illustration_images: [] as string[],
  explainer_videos: [] as string[],
  reference_documents: [] as string[],
})
const isEdit = () => modal.session.value?.mode === 'edit'

// Seed exactly once per session id, and only for a session actually
// targeting this form's kind — a background store refresh never replaces
// the session itself (only pg.changes/pg.live), so this never re-fires for
// the SAME open dialog.
let seededFor = ''
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'topics' || seededFor === s.id) return
    seededFor = s.id
    const snap = s.snapshot as TopicRow | null
    buf.slug = snap?.slug ?? ''
    buf.title = snap?.title ?? ''
    buf.body_md = snap?.body_md ?? ''
    buf.featured_image = snap?.featured_image ?? null
    buf.illustration_images = [...(snap?.illustration_images ?? [])]
    buf.explainer_videos = [...(snap?.explainer_videos ?? [])]
    buf.reference_documents = [...(snap?.reference_documents ?? [])]
  },
  { immediate: true }
)

function payload(): TopicPayload {
  return {
    kind: 'topics', slug: buf.slug, title: buf.title, body_md: buf.body_md,
    featured_image: buf.featured_image,
    illustration_images: buf.illustration_images,
    explainer_videos: buf.explainer_videos,
    reference_documents: buf.reference_documents,
  }
}
function submit() {
  if (!buf.slug.trim()) return
  modal.submit(payload())
}
function retry() {
  modal.reloadAndRetry(payload())
}
</script>

<template>
  <KbFormDialog
    :open="modal.isOpen.value && modal.session.value?.kind === 'topics'"
    :title="isEdit() ? t('kb.forms.editTopic') : t('kb.forms.newTopic')"
    :busy="modal.busy.value"
    :error="modal.error.value"
    :stale="modal.stale.value"
    @update:open="(v) => !v && modal.close()"
    @submit="submit"
    @reload-and-retry="retry"
  >
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.forms.slug') }}</span>
      <Input v-model="buf.slug" :placeholder="t('kb.forms.slugHint')" class="h-9 mt-1" :disabled="isEdit()" />
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.title') }}</span>
      <Input v-model="buf.title" class="h-9 mt-1" />
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.body') }}</span>
      <Textarea v-model="buf.body_md" rows="4" class="min-h-0 text-[14px] mt-1" />
    </div>
    <MediaFieldPicker
      :label="t('kb.media.image')" field="featured_image" :multiple="false"
      :model-value="buf.featured_image" @update:model-value="(v) => (buf.featured_image = v as string | null)"
    />
    <MediaFieldPicker
      :label="t('kb.media.illustrations')" field="illustration_images" :multiple="true"
      :model-value="buf.illustration_images" @update:model-value="(v) => (buf.illustration_images = v as string[])"
    />
    <MediaFieldPicker
      :label="t('kb.media.videos')" field="explainer_videos" :multiple="true"
      :model-value="buf.explainer_videos" @update:model-value="(v) => (buf.explainer_videos = v as string[])"
    />
    <MediaFieldPicker
      :label="t('kb.media.documents')" field="reference_documents" :multiple="true"
      :model-value="buf.reference_documents" @update:model-value="(v) => (buf.reference_documents = v as string[])"
    />
  </KbFormDialog>
</template>
