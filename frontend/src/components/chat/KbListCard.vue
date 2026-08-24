<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import KbSourceBadge from './KbSourceBadge.vue'
import { iconFor, i18nKeyFor } from './kbCards'
import type { KbListData, KbRecord } from '@/types'

// kb_list — several related records at a glance. Each row shows the record's
// title and its two or three most identifying fields; the full record is a
// click away in the KB editor, so this card stays scannable rather than
// reproducing everything.
const props = defineProps<{ data: KbListData }>()
const { t, te } = useI18n()

const icon = computed(() => iconFor(props.data.kind))
const title = computed(() => {
  const key = i18nKeyFor(props.data.kind)
  return key && te(`${key}.plural`) ? t(`${key}.plural`) : t('chat.card.records')
})

// The fields worth showing in a row. Titles already carry the name, so it is
// skipped: repeating it in the summary line wastes the row's width.
const SUMMARY_LIMIT = 3
function summaryFields(record: KbRecord) {
  return record.fields.filter((f) => f.key !== 'name' && f.key !== 'title').slice(0, SUMMARY_LIMIT)
}
</script>

<template>
  <div class="rounded-xl border border-border bg-card overflow-hidden">
    <div class="flex items-center gap-2 border-b border-border bg-muted/40 px-3 py-2">
      <component :is="icon" class="w-4 h-4 text-muted-foreground shrink-0" />
      <span class="font-semibold text-sm">{{ title }}</span>
      <span class="text-xs text-muted-foreground">{{ data.records.length }}</span>
      <KbSourceBadge :source="data.source" class="ml-auto shrink-0" />
    </div>
    <ul class="divide-y divide-border">
      <li v-for="record in data.records" :key="`${record.kind}:${record.key}`" class="px-3 py-2">
        <div class="text-sm font-medium truncate">{{ record.title }}</div>
        <div class="mt-0.5 flex flex-wrap gap-x-4 gap-y-0.5">
          <span v-for="field in summaryFields(record)" :key="field.key" class="text-xs text-muted-foreground">
            {{ field.label }}: <span class="text-foreground">{{ field.value }}</span>
          </span>
        </div>
      </li>
    </ul>
  </div>
</template>
