<script setup lang="ts">
// SpecialistsTab is the live roster view (Знаний база → Специалисты,
// TEST.md §4.2) — a bespoke table, not RecordList, because the columns
// (avatar, weekday pills, shift hours, booking-link pill, portfolio count,
// archive switch) don't fit RecordList's generic card shape. Reads
// pg.live?.specialists directly (falls back to []); no pagination — this
// catalog is small.
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { LoaderCircle, Pencil, UserRound } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { useKbModal } from '@/composables/useKbModal'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { WEEKDAY_ORDER } from '@/components/kb/kbEntities'
import { summarizeShiftHours } from '@/components/kb/records/shared'
import WeekdayPills from '@/components/kb/records/WeekdayPills.vue'
import type { SpecialistRow } from '@/types'

const pg = usePlayground()
const modal = useKbModal()
const { t } = useI18n()

const showArchived = ref(false)
const actioningRef = ref('')
const actionError = ref('')

const rows = computed(() => {
  const all = pg.live?.specialists ?? []
  return all.filter((r) => (showArchived.value ? r.sales_status !== 'active' : r.sales_status === 'active'))
})

function edit(row: SpecialistRow) {
  modal.openEdit('specialists', row, { target: 'live' })
}

// toggleStatus is the archive/restore control itself (the Switch below) —
// NO confirmation dialog, an instant write via pg.setSpecialistStatus
// (PATCH .../status, direct live write, no draft step) — same UX
// philosophy as CampaignTemplatesPanel.vue's toggleArchive(). Errors are
// caught here, not in the store, so only this one row's spinner/message is
// affected.
async function toggleStatus(row: SpecialistRow) {
  actionError.value = ''
  actioningRef.value = row.ref
  try {
    await pg.setSpecialistStatus(row.ref, row.sales_status === 'active' ? 'inactive' : 'active')
  } catch (e) {
    actionError.value = e instanceof ApiError ? e.message : t('kb.draft.errSaveChange')
  } finally {
    actioningRef.value = ''
  }
}

function shiftSummary(row: SpecialistRow): string {
  const uniform = summarizeShiftHours(row.schedule)
  if (uniform) return uniform
  if (row.schedule.length === 0) return t('kb.schedule.dayOff')
  return [...row.schedule]
    .sort((a, b) => WEEKDAY_ORDER.indexOf(a.ref) - WEEKDAY_ORDER.indexOf(b.ref))
    .map((d) => `${t('kb.schedule.weekdayShort.' + d.ref)} ${d.start}–${d.end}`)
    .join(', ')
}
</script>

<template>
  <div class="space-y-3" data-testid="specialists-tab">
    <div class="flex items-center justify-between gap-2 flex-wrap">
      <div class="flex items-center gap-1 rounded-lg border border-border p-0.5">
        <button
          type="button"
          class="rounded-md px-3 py-1.5 text-xs font-medium transition"
          :class="!showArchived ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'"
          data-testid="specialists-filter-active"
          @click="showArchived = false"
        >
          {{ t('kb.archive.filterActive') }}
        </button>
        <button
          type="button"
          class="rounded-md px-3 py-1.5 text-xs font-medium transition"
          :class="showArchived ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'"
          data-testid="specialists-filter-archived"
          @click="showArchived = true"
        >
          {{ t('kb.archive.filterArchived') }}
        </button>
      </div>
    </div>

    <p v-if="actionError" class="text-sm text-destructive">{{ actionError }}</p>

    <p v-if="!rows.length" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.page.emptySpecialists') }}</p>

    <div v-else class="overflow-x-auto rounded-lg border border-border">
      <table class="w-full text-sm">
        <thead>
          <tr class="text-left text-xs text-muted-foreground border-b border-border">
            <th class="py-2 px-3 font-medium">{{ t('kb.specialists.columns.master') }}</th>
            <th class="py-2 px-3 font-medium">{{ t('kb.fields.experience') }}</th>
            <th class="py-2 px-3 font-medium">{{ t('kb.specialists.columns.workingDays') }}</th>
            <th class="py-2 px-3 font-medium">{{ t('kb.specialists.columns.shiftHours') }}</th>
            <th class="py-2 px-3 font-medium">{{ t('kb.specialists.columns.bookingUrl') }}</th>
            <th class="py-2 px-3 font-medium">{{ t('kb.media.portfolio') }}</th>
            <th class="py-2 px-3 font-medium">{{ t('kb.specialists.columns.status') }}</th>
            <th class="py-2 px-3 font-medium"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.ref" class="border-b border-border/60 last:border-0" data-testid="specialist-row">
            <td class="py-2 px-3 align-top">
              <div class="flex items-center gap-2.5 min-w-[160px]">
                <div class="w-8 h-8 rounded-full bg-muted grid place-items-center shrink-0 text-muted-foreground">
                  <UserRound class="w-4 h-4" />
                </div>
                <div class="min-w-0">
                  <p class="font-medium truncate">{{ row.full_name || row.ref }}</p>
                  <p v-if="row.title" class="text-xs text-muted-foreground truncate">{{ row.title }}</p>
                </div>
              </div>
            </td>
            <td class="py-2 px-3 align-top">
              <Badge v-if="row.experience" variant="secondary" class="text-[11px]">{{ row.experience }}</Badge>
              <span v-else class="text-muted-foreground">—</span>
            </td>
            <td class="py-2 px-3 align-top">
              <WeekdayPills :schedule="row.schedule" />
            </td>
            <td class="py-2 px-3 align-top text-xs whitespace-nowrap">{{ shiftSummary(row) }}</td>
            <td class="py-2 px-3 align-top">
              <Badge v-if="row.booking_url" variant="outline" class="text-[11px] font-mono max-w-[180px] truncate inline-block align-bottom">
                {{ row.booking_url }}
              </Badge>
              <Badge v-else variant="secondary" class="text-[11px]">{{ t('kb.fields.bookingUrlFallback') }}</Badge>
            </td>
            <td class="py-2 px-3 align-top whitespace-nowrap">{{ t('kb.page.photosCount', { n: row.portfolio_images.length }) }}</td>
            <td class="py-2 px-3 align-top">
              <div class="flex items-center gap-2">
                <Switch
                  :model-value="row.sales_status === 'active'"
                  :disabled="actioningRef === row.ref"
                  :aria-label="t('kb.archive.toggleAria')"
                  :data-testid="`specialist-status-switch-${row.ref}`"
                  @update:model-value="() => toggleStatus(row)"
                />
                <LoaderCircle v-if="actioningRef === row.ref" class="w-3.5 h-3.5 animate-spin text-muted-foreground" />
                <span class="text-xs text-muted-foreground">{{ row.sales_status === 'active' ? t('kb.archive.active') : t('kb.archive.archived') }}</span>
              </div>
            </td>
            <td class="py-2 px-3 align-top">
              <Button variant="outline" size="sm" :data-testid="`specialist-edit-${row.ref}`" @click="edit(row)">
                <Pencil class="w-3.5 h-3.5" /> {{ t('kb.actions.edit') }}
              </Button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
