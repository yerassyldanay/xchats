<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { sourceClasses } from './kbCards'
import type { KbSource } from '@/types'

// The provenance badge every KB card carries. It exists so no value in this
// UI is ever shown without saying which knowledge-base state it came from —
// the frontend half of the rule internal/chatkb enforces on the prompt side.
const props = defineProps<{ source: KbSource }>()
const { t } = useI18n()

const label = computed(() => (props.source === 'DRAFT_KB' ? t('chat.source.draft') : t('chat.source.real')))
</script>

<template>
  <span
    class="inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide"
    :class="sourceClasses(source)"
  >
    {{ label }}
  </span>
</template>
