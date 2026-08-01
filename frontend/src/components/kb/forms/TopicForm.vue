<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ListTree } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { useKbModal } from '@/composables/useKbModal'
import type { TopicRow } from '@/types'
import type { TopicPayload } from './payloads'
import KbFormDialog from './KbFormDialog.vue'

const { t } = useI18n()
const modal = useKbModal()

const open = computed(() => modal.isOpen.value && modal.session.value?.kind === 'topics')
const isEdit = computed(() => modal.session.value?.mode === 'edit')

const buf = reactive({ slug: '', title: '', body_md: '' })
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'topics') return
    const snap = s.snapshot as TopicRow | null
    buf.slug = snap?.slug ?? ''
    buf.title = snap?.title ?? ''
    buf.body_md = snap?.body_md ?? ''
  },
  { immediate: true }
)

const saveDisabled = computed(() => !buf.slug.trim())

function payload(): TopicPayload {
  return { slug: buf.slug, title: buf.title, body_md: buf.body_md }
}
function submit() {
  if (saveDisabled.value) return
  modal.submit(payload())
}
function retry() {
  modal.reloadAndRetry(payload())
}
</script>

<template>
  <KbFormDialog
    :open="open"
    :icon="ListTree"
    :title="isEdit ? t('kb.forms.topic.editTitle') : t('kb.forms.topic.createTitle')"
    :busy="modal.busy.value"
    :error="modal.error.value"
    :stale="modal.stale.value"
    :save-disabled="saveDisabled"
    @submit="submit"
    @reload-retry="retry"
    @close="modal.close()"
  >
    <Input v-model="buf.slug" :placeholder="t('kb.forms.topic.slug')" class="h-9 font-mono" :disabled="isEdit" />
    <Input v-model="buf.title" :placeholder="t('kb.forms.topic.title')" class="h-9" />
    <Textarea v-model="buf.body_md" rows="4" :placeholder="t('kb.forms.topic.body')" class="min-h-0 text-[14px]" />
  </KbFormDialog>
</template>
