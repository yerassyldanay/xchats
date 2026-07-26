<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, Copy } from 'lucide-vue-next'

// Small reusable clipboard button — same navigator.clipboard.writeText + timed-reset
// pattern as TestCaseCard.vue's copyText, just packaged as its own component since the
// catalog page needs it in several places (test id, tests_path, deep link).
const props = defineProps<{ text: string; labelKey: 'id' | 'path' | 'link' }>()
const { t } = useI18n()
const copied = ref(false)

async function copy() {
  try {
    await navigator.clipboard.writeText(props.text)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 1500)
  } catch {
    // no clipboard permission — nothing else to do, text stays visible/selectable
  }
}
</script>

<template>
  <button
    type="button"
    class="inline-flex items-center justify-center shrink-0 text-muted-foreground hover:text-foreground transition"
    :aria-label="t(`evalCatalog.copy.${labelKey}`)"
    :title="copied ? t('evalCatalog.copy.copied') : t(`evalCatalog.copy.${labelKey}`)"
    @click="copy"
  >
    <Check v-if="copied" class="w-3.5 h-3.5 text-emerald-600" />
    <Copy v-else class="w-3.5 h-3.5" />
  </button>
</template>
