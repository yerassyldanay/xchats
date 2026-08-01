<script setup lang="ts">
// AssistantFieldForm edits ONE assistant-config field — text / list / number
// per its CONFIG_SECTIONS.render kind. Publishing/cancelling the whole config
// section is ConfigChangeGroup.vue's job (§1.5: the backend can only publish
// the WHOLE staged config patch, never one field), so this form only ever
// stages a single field's edit.
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { useKbModal } from '@/composables/useKbModal'
import { CONFIG_SECTIONS } from '@/components/kb/kbEntities'
import type { DraftConfig } from '@/types'
import type { ConfigFieldPayload } from './payloads'
import KbFormDialog from './KbFormDialog.vue'

const { t } = useI18n()
const modal = useKbModal()

const open = computed(() => modal.isOpen.value && modal.session.value?.kind === 'config')
const field = computed(() => modal.session.value?.field)
const section = computed(() => CONFIG_SECTIONS.find((s) => s.key === field.value))

const text = ref('')
const num = ref(0)
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'config' || !s.field) return
    const snap = s.snapshot as Partial<DraftConfig> | null
    const v = snap ? snap[s.field] : undefined
    if (s.field === 'reply_max_words') num.value = Number(v) || 0
    else text.value = String(v ?? '')
  },
  { immediate: true }
)

function payload(): ConfigFieldPayload {
  const f = field.value!
  return { field: f, value: f === 'reply_max_words' ? Number(num.value) || 0 : text.value }
}
function submit() {
  modal.submit(payload())
}
function retry() {
  modal.reloadAndRetry(payload())
}
</script>

<template>
  <KbFormDialog
    v-if="section"
    :open="open" :icon="section.icon" :title="t(section.titleKey)"
    :busy="modal.busy.value" :error="modal.error.value" :stale="modal.stale.value"
    @submit="submit" @reload-retry="retry" @close="modal.close()"
  >
    <p class="text-xs text-muted-foreground mb-1">{{ t(section.hintKey) }}</p>
    <Input v-if="section.render === 'badge'" v-model.number="num" type="number" min="0" />
    <Textarea v-else v-model="text" :rows="section.render === 'list' ? 6 : 4" class="text-[14px]" />
  </KbFormDialog>
</template>
