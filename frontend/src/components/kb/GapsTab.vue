<script setup lang="ts">
// Gaps tab (Пробелы в базе) — GET /kb/gaps' aggregated counts plus a bounded
// page of recent representative events (backend/internal/httpapi/kb_gaps.go).
// Read-only, exactly like the Промпт tab: it never exposes the customer-
// facing draft/message text, only the structured kb_gap diagnostic each
// escalating draft recorded (backend/aiprompt/kbgap.go) — see KbGapEvent's
// doc comment in types.ts.
import { computed, onMounted, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, RefreshCw, TriangleAlert } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { shortTime } from '@/lib/format'
import type { KbGapEntityType, KbGapFilter, KbGapReasonCode } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const pg = usePlayground()
const { t } = useI18n()

const REASON_CODES: KbGapReasonCode[] = [
  'missing_entity', 'missing_field', 'ambiguous_entity', 'conflicting_kb_data', 'unavailable_entity',
  'unsupported_request', 'human_requested', 'engine_error', 'other',
]
const ENTITY_TYPES: KbGapEntityType[] = ['product', 'tariff', 'tariff_info', 'contact', 'policy', 'delivery_zone', 'topic']

// 'all' is a sentinel, not a real reason/entity-type value — reka-ui's
// <SelectItem> forbids an empty-string value (that is reserved to mean
// "cleared, show the placeholder"), so "no filter on this dimension" needs
// its own selectable option instead.
const ALL = 'all'

// Local draft of the filter form — only pushed into pg.loadGaps on Apply, so
// typing an entity ref doesn't refetch on every keystroke.
const form = reactive({ reason: ALL, entityType: ALL, entityRef: '', from: '', to: '' })

onMounted(() => {
  if (!pg.gapsReport) pg.loadGaps()
})

// <input type="date"> gives "YYYY-MM-DD" in local time; the API filters on
// an RFC3339 instant, so a plain date is read as that day's start/end in the
// browser's own timezone — good enough for a report filter, not meant to be
// exact-to-the-second.
function apply() {
  const filter: KbGapFilter = {}
  if (form.reason !== ALL) filter.reason = form.reason
  if (form.entityType !== ALL) filter.entity_type = form.entityType
  if (form.entityRef.trim()) filter.entity_ref = form.entityRef.trim()
  if (form.from) filter.from = new Date(form.from + 'T00:00:00').toISOString()
  if (form.to) filter.to = new Date(form.to + 'T23:59:59.999').toISOString()
  pg.loadGaps(filter)
}
function reset() {
  form.reason = ALL
  form.entityType = ALL
  form.entityRef = ''
  form.from = ''
  form.to = ''
  pg.loadGaps({})
}

function reasonLabel(code: string): string {
  return t(`kb.gaps.reasons.${code}`)
}
function entityTypeLabel(type?: string): string {
  return type ? t(`kb.gaps.entityTypes.${type}`) : ''
}
function sourceLabel(source: string): string {
  return t(`kb.gaps.sources.${source}`)
}

const totalContentGaps = computed(() => (pg.gapsReport?.counts ?? []).reduce((sum, c) => sum + c.count, 0))
const totalOperational = computed(() => (pg.gapsReport?.operational_counts ?? []).reduce((sum, c) => sum + c.count, 0))
</script>

<template>
  <div class="space-y-4">
    <div class="rounded-xl border border-border bg-card overflow-hidden">
      <div class="p-5 pb-4 flex items-start justify-between gap-3 border-b border-border flex-wrap">
        <div class="flex gap-3">
          <div class="w-9 h-9 rounded-lg bg-primary/10 text-primary grid place-items-center shrink-0">
            <TriangleAlert class="w-4 h-4" />
          </div>
          <div>
            <h3 class="font-semibold leading-tight">{{ t('kb.gaps.title') }}</h3>
            <p class="text-xs text-muted-foreground mt-1 max-w-md">{{ t('kb.gaps.subtitle') }}</p>
          </div>
        </div>
        <Button size="sm" variant="outline" :disabled="pg.gapsLoading" @click="pg.loadGaps()">
          <RefreshCw class="w-4 h-4" :class="pg.gapsLoading ? 'animate-spin' : ''" /> {{ t('common.refresh') }}
        </Button>
      </div>

      <!-- filters -->
      <div class="p-5 border-b border-border grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3 items-end">
        <div class="space-y-1">
          <label class="text-xs text-muted-foreground">{{ t('kb.gaps.filters.reason') }}</label>
          <Select v-model="form.reason">
            <SelectTrigger data-testid="gaps-filter-reason"><SelectValue :placeholder="t('kb.gaps.filters.reasonAll')" /></SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectItem value="all">{{ t('kb.gaps.filters.reasonAll') }}</SelectItem>
                <SelectItem v-for="code in REASON_CODES" :key="code" :value="code">{{ reasonLabel(code) }}</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>
        <div class="space-y-1">
          <label class="text-xs text-muted-foreground">{{ t('kb.gaps.filters.entityType') }}</label>
          <Select v-model="form.entityType">
            <SelectTrigger data-testid="gaps-filter-entity-type"><SelectValue :placeholder="t('kb.gaps.filters.entityTypeAll')" /></SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectItem value="all">{{ t('kb.gaps.filters.entityTypeAll') }}</SelectItem>
                <SelectItem v-for="type in ENTITY_TYPES" :key="type" :value="type">{{ entityTypeLabel(type) }}</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>
        <div class="space-y-1">
          <label class="text-xs text-muted-foreground">{{ t('kb.gaps.filters.entityRef') }}</label>
          <Input v-model="form.entityRef" :placeholder="t('kb.gaps.filters.entityRefPlaceholder')" data-testid="gaps-filter-entity-ref" />
        </div>
        <div class="space-y-1">
          <label class="text-xs text-muted-foreground">{{ t('kb.gaps.filters.from') }}</label>
          <Input v-model="form.from" type="date" data-testid="gaps-filter-from" />
        </div>
        <div class="space-y-1">
          <label class="text-xs text-muted-foreground">{{ t('kb.gaps.filters.to') }}</label>
          <Input v-model="form.to" type="date" data-testid="gaps-filter-to" />
        </div>
        <div class="flex gap-2">
          <Button size="sm" class="flex-1" data-testid="gaps-filter-apply" @click="apply">{{ t('kb.gaps.filters.apply') }}</Button>
          <Button size="sm" variant="outline" @click="reset">{{ t('kb.gaps.filters.reset') }}</Button>
        </div>
      </div>

      <div v-if="pg.gapsLoading && !pg.gapsReport" class="p-10 text-center text-sm text-muted-foreground">{{ t('kb.gaps.loading') }}</div>
      <div v-else-if="pg.gapsLoadError && !pg.gapsReport" class="p-5">
        <p class="flex items-start gap-2 text-sm text-destructive rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> <span>{{ pg.gapsLoadError }}</span>
        </p>
        <Button size="sm" variant="outline" class="mt-3" @click="pg.loadGaps()">{{ t('common.retry') }}</Button>
      </div>

      <template v-else-if="pg.gapsReport">
        <!-- content-gap counts (default report) -->
        <div class="p-5 border-b border-border">
          <h4 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-3">{{ t('kb.gaps.countsTitle') }}</h4>
          <p class="text-xs text-muted-foreground mb-3">{{ t('kb.gaps.countsHint') }}</p>
          <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
            <div
              v-for="c in pg.gapsReport.counts"
              :key="c.reason_code"
              class="rounded-lg border border-border p-3.5"
              :class="c.count > 0 ? 'bg-amber-50 border-amber-200' : ''"
              data-testid="gaps-count-tile"
            >
              <div class="text-2xl font-bold leading-none" :class="c.count > 0 ? 'text-amber-700' : ''">{{ c.count }}</div>
              <div class="text-xs text-muted-foreground mt-1.5">{{ reasonLabel(c.reason_code) }}</div>
            </div>
          </div>
          <p v-if="totalContentGaps === 0" class="text-sm text-muted-foreground mt-3">{{ t('kb.gaps.recentEmpty') }}</p>
        </div>

        <!-- operational counts (kept distinguishable, never blended in) -->
        <div v-if="totalOperational > 0" class="px-5 py-4 border-b border-border">
          <h4 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-3">{{ t('kb.gaps.operationalTitle') }}</h4>
          <p class="text-xs text-muted-foreground mb-3">{{ t('kb.gaps.operationalHint') }}</p>
          <div class="flex flex-wrap gap-2">
            <Badge v-for="c in pg.gapsReport.operational_counts.filter((c) => c.count > 0)" :key="c.reason_code" variant="secondary">
              {{ reasonLabel(c.reason_code) }}: {{ c.count }}
            </Badge>
          </div>
        </div>

        <!-- recent representative events -->
        <div class="p-5">
          <h4 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-3">{{ t('kb.gaps.recentTitle') }}</h4>
          <p v-if="!pg.gapsReport.recent.length" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.gaps.recentEmpty') }}</p>
          <div v-else class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="text-left text-xs text-muted-foreground border-b border-border">
                  <th class="py-2 pr-3 font-medium">{{ t('kb.gaps.colChannel') }}</th>
                  <th class="py-2 pr-3 font-medium">{{ t('kb.gaps.colReason') }}</th>
                  <th class="py-2 pr-3 font-medium">{{ t('kb.gaps.colTarget') }}</th>
                  <th class="py-2 pr-3 font-medium">{{ t('kb.gaps.colMissingFields') }}</th>
                  <th class="py-2 pr-3 font-medium">{{ t('kb.gaps.colNote') }}</th>
                  <th class="py-2 pr-3 font-medium">{{ t('kb.gaps.colSource') }}</th>
                  <th class="py-2 pr-3 font-medium">{{ t('kb.gaps.colCreatedAt') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="e in pg.gapsReport.recent" :key="e.id" class="border-b border-border/60 last:border-0" data-testid="gaps-event-row">
                  <td class="py-2 pr-3 align-top"><Badge variant="outline" class="text-[11px]">{{ e.channel }}</Badge></td>
                  <td class="py-2 pr-3 align-top font-medium">{{ reasonLabel(e.reason_code) }}</td>
                  <td class="py-2 pr-3 align-top text-muted-foreground">
                    <template v-if="e.target_entity_ref">{{ entityTypeLabel(e.target_entity_type) }}: {{ e.target_entity_ref }}</template>
                    <template v-else>{{ t('kb.gaps.noTarget') }}</template>
                  </td>
                  <td class="py-2 pr-3 align-top">
                    <div class="flex flex-wrap gap-1">
                      <Badge v-for="f in e.missing_fields" :key="f" variant="secondary" class="text-[11px]">{{ f }}</Badge>
                    </div>
                  </td>
                  <td class="py-2 pr-3 align-top text-muted-foreground max-w-xs truncate" :title="e.escalation_reason">{{ e.escalation_reason }}</td>
                  <td class="py-2 pr-3 align-top text-muted-foreground">{{ sourceLabel(e.source) }}</td>
                  <td class="py-2 pr-3 align-top text-muted-foreground whitespace-nowrap">{{ shortTime(e.created_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>
