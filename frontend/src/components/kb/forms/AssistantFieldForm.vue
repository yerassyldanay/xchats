<script setup lang="ts">
// AssistantFieldForm edits ONE config field — addressed by `field`, not a
// natural key, so unlike the other six forms it does not go through
// useKbModal's submit()/KbFormPayload (config has no row-shaped payload):
// it calls stores.patchConfig({ [field]: value }) directly. It still shares
// useKbModal's session/open/close so only one modal is ever open at a time,
// and reads busy/error/draftStale straight off the store — patchConfig runs
// through the SAME write() wrapper every other draft action does, so those
// flags mean exactly the same thing here.
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useKbModal } from '@/composables/useKbModal'
import { usePlayground } from '@/stores/playground'
import { CONFIG_SECTIONS } from '@/components/kb/kbEntities'
import type { DraftConfig } from '@/types'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import KbFormDialog from './KbFormDialog.vue'

const modal = useKbModal()
const pg = usePlayground()
const { t } = useI18n()

const buf = ref<string | number>('')

const section = () => CONFIG_SECTIONS.find((s) => s.key === modal.session.value?.field)

let seededFor = ''
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'config' || seededFor === s.id) return
    seededFor = s.id
    const snap = s.snapshot as DraftConfig | null
    const field = s.field
    buf.value = field && snap ? (snap[field] ?? '') : ''
  },
  { immediate: true }
)

async function submit() {
  const field = modal.session.value?.field
  if (!field) return
  const patch: Record<string, string | number> = {
    [field]: field === 'reply_max_words' ? Number(buf.value) || 0 : String(buf.value),
  }
  const result = await pg.patchConfig(patch)
  if (result !== undefined) {
    modal.close()
    modal.markSuccess()
  }
}
</script>

<template>
  <KbFormDialog
    :open="modal.isOpen.value && modal.session.value?.kind === 'config'"
    :title="section() ? t(section()!.i18nKey + '.title') : ''"
    :busy="pg.busy"
    :error="pg.error"
    :stale="pg.draftStale"
    @update:open="(v) => !v && modal.close()"
    @submit="submit"
    @reload-and-retry="submit"
  >
    <p v-if="section()" class="text-xs text-muted-foreground mb-3">{{ t(section()!.i18nKey + '.hint') }}</p>
    <Input v-if="section()?.render === 'badge'" v-model.number="buf" type="number" min="0" />
    <Textarea v-else v-model="buf as string" :rows="section()?.render === 'list' ? 7 : 5" class="text-[14px]" />
  </KbFormDialog>
</template>
