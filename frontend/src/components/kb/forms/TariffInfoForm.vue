<script setup lang="ts">
// TariffInfoForm edits the org-wide tariff_info singleton — a structural
// clone of ContactsForm.vue/PoliciesForm.vue, minus every prose field and
// MediaFieldPicker (ai_tariff_info carries no media columns at all): the
// whole record is additional_facts.
import { reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useKbModal } from '@/composables/useKbModal'
import type { AdditionalFact, TariffInfoRow } from '@/types'
import KbFormDialog from './KbFormDialog.vue'
import AdditionalFactsEditor from './AdditionalFactsEditor.vue'
import type { TariffInfoPayload } from './payloads'

const modal = useKbModal()
const { t } = useI18n()

const buf = reactive({
  additional_facts: [] as AdditionalFact[],
})

let seededFor = ''
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'tariff_info' || seededFor === s.id) return
    seededFor = s.id
    const snap = s.snapshot as TariffInfoRow | null
    buf.additional_facts = [...(snap?.additional_facts ?? [])]
  },
  { immediate: true }
)

function payload(): TariffInfoPayload {
  return { kind: 'tariff_info', ...buf }
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
    :open="modal.isOpen.value && modal.session.value?.kind === 'tariff_info'"
    :title="t('kb.forms.editTariffInfo')"
    :busy="modal.busy.value"
    :error="modal.error.value"
    :stale="modal.stale.value"
    @update:open="(v) => !v && modal.close()"
    @submit="submit"
    @reload-and-retry="retry"
  >
    <p class="text-xs text-muted-foreground">{{ t('kb.forms.tariffInfoHint') }}</p>
    <AdditionalFactsEditor v-model="buf.additional_facts" />
  </KbFormDialog>
</template>
