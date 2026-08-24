<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import KbSourceBadge from './KbSourceBadge.vue'
import { iconFor, i18nKeyFor } from './kbCards'
import type { KbItemData } from '@/types'

// kb_item — one knowledge-base record, shown in full. What the assistant
// answers "tell me about X" with.
const props = defineProps<{ data: KbItemData }>()
const { t, te } = useI18n()

const record = computed(() => props.data.record)
const icon = computed(() => iconFor(record.value.kind))
const kindLabel = computed(() => {
  const key = i18nKeyFor(record.value.kind)
  return key && te(`${key}.singular`) ? t(`${key}.singular`) : ''
})
</script>

<template>
  <div class="rounded-xl border border-border bg-card overflow-hidden">
    <div class="flex items-center gap-2 border-b border-border bg-muted/40 px-3 py-2">
      <component :is="icon" class="w-4 h-4 text-muted-foreground shrink-0" />
      <span class="font-semibold text-sm truncate">{{ record.title }}</span>
      <span v-if="kindLabel" class="text-xs text-muted-foreground truncate">{{ kindLabel }}</span>
      <KbSourceBadge :source="record.source" class="ml-auto shrink-0" />
    </div>
    <dl class="divide-y divide-border">
      <div v-for="field in record.fields" :key="field.key" class="grid grid-cols-[minmax(0,9rem)_1fr] gap-3 px-3 py-2">
        <dt class="text-xs text-muted-foreground pt-0.5">{{ field.label }}</dt>
        <dd class="text-sm whitespace-pre-wrap break-words">{{ field.value }}</dd>
      </div>
    </dl>
  </div>
</template>
