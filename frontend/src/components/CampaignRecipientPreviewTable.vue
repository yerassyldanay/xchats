<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert } from 'lucide-vue-next'
import type { CampaignRecipientPreviewResult } from '@/types'

// CampaignRecipientPreviewTable renders a parse-only preview result (POST
// .../preview) or a just-persisted result (PUT .../recipients) — the same
// shape either way. Hand-rolled divs, not a Table component (none exists in
// this app — see AccountSendingBudget.vue's own bars for the same reason).
const props = defineProps<{ result: CampaignRecipientPreviewResult; messageVariables?: string[] }>()
const { t } = useI18n()

// A very large pasted/uploaded list renders only its first ROW_CAP rows —
// this is a live preview widget, not the campaign's own persisted
// recipient list (that has its own paginated GET .../recipients view).
const ROW_CAP = 200
const visibleRows = computed(() => props.result.rows.slice(0, ROW_CAP))
const hiddenCount = computed(() => Math.max(0, props.result.rows.length - ROW_CAP))

// CAM-06: a column's Attribute key IS the {{variable}} name it feeds — the
// parser keys Attributes by the (trimmed) CSV header text verbatim (see
// ParseRecipients' own doc comment, backend/campaign/recipients.go) — so
// "detected columns" and "how they map to the message" are the same list,
// read straight off the already-parsed rows rather than re-deriving
// anything. name is reported too even though it is never itself an
// Attribute key (Name is a top-level field on every row).
const detectedColumns = computed(() => {
  const cols = new Set<string>()
  for (const row of props.result.rows) {
    if (row.name) cols.add('name')
    for (const key of Object.keys(row.attributes ?? {})) cols.add(key)
  }
  return [...cols].sort()
})
// phone is always the identity column, never a named Attribute — excluded
// here so a plain {{phone}} in the message never reads as "unmapped".
const unmatchedVariables = computed(() => (props.messageVariables ?? []).filter((v) => v !== 'phone' && !detectedColumns.value.includes(v)))
function chipText(col: string): string {
  return `${col} → {{${col}}}`
}

const toneFor: Record<string, string> = {
  valid: 'text-wa',
  invalid: 'text-destructive',
  duplicate: 'text-amber-600 dark:text-amber-400',
}
</script>

<template>
  <div>
    <div class="flex items-center gap-3 text-xs">
      <span class="text-wa font-medium">{{ t('campaigns.wizard.previewValid', { count: result.valid }) }}</span>
      <span class="text-destructive font-medium">{{ t('campaigns.wizard.previewInvalid', { count: result.invalid }) }}</span>
      <span class="text-amber-600 dark:text-amber-400 font-medium">{{ t('campaigns.wizard.previewDuplicate', { count: result.duplicate }) }}</span>
    </div>

    <div v-if="detectedColumns.length" class="mt-1.5 flex flex-wrap items-center gap-1.5 text-[11px]">
      <span class="text-muted-foreground">{{ t('campaigns.wizard.detectedColumns') }}</span>
      <span v-for="c in detectedColumns" :key="c" class="rounded-full border border-border bg-muted px-2 py-0.5 font-mono">{{ chipText(c) }}</span>
    </div>
    <p v-if="unmatchedVariables.length" class="mt-1 flex items-center gap-1.5 text-[11px] text-amber-600 dark:text-amber-400" data-testid="unmatched-variables-warning">
      <CircleAlert class="w-3.5 h-3.5 shrink-0" /> {{ t('campaigns.wizard.unmatchedVariables', { variables: unmatchedVariables.join(', ') }) }}
    </p>

    <div v-if="result.rows.length" class="mt-2 max-h-64 overflow-y-auto rounded-md border border-border divide-y divide-border">
      <div v-for="(row, i) in visibleRows" :key="i" class="flex items-center gap-2 px-2.5 py-1.5 text-xs">
        <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="toneFor[row.status].replace('text-', 'bg-')" />
        <span class="font-mono truncate">{{ row.raw }}</span>
        <span v-if="row.name" class="text-muted-foreground truncate">{{ row.name }}</span>
        <span v-if="row.reason" class="ml-auto shrink-0 truncate max-w-[45%]" :class="toneFor[row.status]" :title="row.reason">{{ row.reason }}</span>
        <span v-else-if="row.normalized_identity && row.normalized_identity !== row.raw" class="ml-auto shrink-0 font-mono text-muted-foreground">
          {{ row.normalized_identity }}
        </span>
      </div>
      <div v-if="hiddenCount > 0" class="px-2.5 py-1.5 text-xs text-muted-foreground text-center">+{{ hiddenCount }}</div>
    </div>
  </div>
</template>
