<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { VContractRow } from '@/types'
import { Badge } from '@/components/ui/badge'

// The Requirements panel: requirement | expected | actual | verdict, one row
// per check the test/case declares plus the family's universal safety checks —
// pass/fail here is always what the run's own verdict already computed
// (evals/harness/viewmodel.go), never re-graded client-side. Rows arrive pre-ordered
// (requirement rows first, then safety) — a divider is inserted at the first "safety"
// row rather than re-sorting, so this stays a pure display of what the export sent.
defineProps<{ rows: VContractRow[] }>()
const { t } = useI18n()

function isFirstSafetyRow(rows: VContractRow[], i: number): boolean {
  return rows[i].kind === 'safety' && (i === 0 || rows[i - 1].kind !== 'safety')
}
</script>

<template>
  <div>
    <table v-if="rows.length" class="w-full text-xs">
      <thead>
        <tr class="text-left text-muted-foreground">
          <th class="pb-1 pr-3 font-medium">{{ t('evals.contract.requirement') }}</th>
          <th class="pb-1 pr-3 font-medium">{{ t('evals.contract.expected') }}</th>
          <th class="pb-1 pr-3 font-medium">{{ t('evals.contract.actual') }}</th>
          <th class="pb-1 font-medium">{{ t('evals.contract.verdict') }}</th>
        </tr>
      </thead>
      <tbody>
        <template v-for="(row, i) in rows" :key="row.key">
          <tr v-if="isFirstSafetyRow(rows, i)">
            <td colspan="4" class="pt-2.5 pb-1 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">{{ t('evals.contract.universalChecks') }}</td>
          </tr>
          <tr class="border-b border-border/60 align-top last:border-0">
            <td class="py-1.5 pr-3 whitespace-nowrap">{{ row.label }}</td>
            <td class="max-w-[240px] py-1.5 pr-3 whitespace-pre-wrap wrap-break-word text-muted-foreground">{{ row.expected || '—' }}</td>
            <td class="max-w-[240px] py-1.5 pr-3 whitespace-pre-wrap wrap-break-word">{{ row.actual || '—' }}</td>
            <td class="py-1.5">
              <Badge v-if="row.pass === true" class="border-transparent bg-emerald-100 text-[11px] text-emerald-700 hover:bg-emerald-100">PASS</Badge>
              <Badge v-else-if="row.pass === false" variant="destructive" class="text-[11px]">FAIL</Badge>
              <span v-else class="text-[11px] italic text-muted-foreground">{{ t('evals.na') }}</span>
            </td>
          </tr>
        </template>
      </tbody>
    </table>
    <p v-else class="text-xs italic text-muted-foreground">{{ t('evals.contract.unavailable') }}</p>
  </div>
</template>
