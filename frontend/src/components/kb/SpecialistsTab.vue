<script setup lang="ts">
// SpecialistsTab is the live roster view (Знаний база → Специалисты,
// TEST.md §4.2) — a bespoke table, not RecordList, because the columns
// (avatar, weekday pills, shift hours, booking-link pill, portfolio count,
// archive switch) don't fit RecordList's generic card shape. Reads
// pg.live?.specialists directly (falls back to []); no pagination — this
// catalog is small.
import { computed, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Building2, Link2, LoaderCircle, Pencil, Undo2, UserRound } from 'lucide-vue-next'
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

// --- archive/restore toggle: optimistic switch + 5s undo toast -----------
// toggleStatus no longer writes immediately — it stages the flip (the
// Switch below reflects it right away via displayedStatus) and starts a 5s
// timer. Only when that timer elapses (undoPending was never called) does
// pg.setSpecialistStatus actually fire — PATCH .../status, direct live
// write, no draft step, same as before. Undoing within the window means the
// live KB is never touched at all. Only one toggle is ever pending at a
// time: starting a new one on a different row immediately commits whatever
// was pending before it, so there is never more than one undo toast to
// reason about.
interface PendingToggle {
  ref: string
  name: string
  from: 'active' | 'inactive'
  to: 'active' | 'inactive'
  timer: ReturnType<typeof setTimeout>
}
const pending = ref<PendingToggle | null>(null)

function displayedStatus(row: SpecialistRow): string {
  return pending.value?.ref === row.ref ? pending.value.to : row.sales_status
}

async function commitPending() {
  const p = pending.value
  if (!p) return
  pending.value = null
  actionError.value = ''
  actioningRef.value = p.ref
  try {
    await pg.setSpecialistStatus(p.ref, p.to)
  } catch (e) {
    actionError.value = e instanceof ApiError ? e.message : t('kb.draft.errSaveChange')
  } finally {
    actioningRef.value = ''
  }
}

function undoPending() {
  if (!pending.value) return
  clearTimeout(pending.value.timer)
  pending.value = null
}

function toggleStatus(row: SpecialistRow) {
  if (pending.value && pending.value.ref !== row.ref) {
    clearTimeout(pending.value.timer)
    void commitPending()
  }
  const from: 'active' | 'inactive' = row.sales_status === 'active' ? 'active' : 'inactive'
  const to = from === 'active' ? 'inactive' : 'active'
  const timer = setTimeout(() => void commitPending(), 5000)
  pending.value = { ref: row.ref, name: row.full_name || row.ref, from, to, timer }
}

// A pending toggle whose row navigates away with it still unresolved commits
// immediately instead of leaking a dangling timer — leaving without clicking
// "Отменить" is still exactly the same "didn't undo it" intent.
onUnmounted(() => {
  if (pending.value) {
    clearTimeout(pending.value.timer)
    void commitPending()
  }
})

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

    <div
      v-if="pending"
      class="flex items-center gap-3 rounded-lg border border-primary/30 bg-primary/5 px-3 py-2 text-sm"
      data-testid="specialist-undo-toast"
    >
      <span class="flex-1">
        {{ t('kb.archive.toastChanged', { name: pending.name, status: pending.to === 'active' ? t('kb.archive.active') : t('kb.archive.archived') }) }}
      </span>
      <Button variant="outline" size="sm" class="h-7 shrink-0" data-testid="specialist-undo-toggle" @click="undoPending">
        <Undo2 class="w-3.5 h-3.5" /> {{ t('kb.archive.undo') }}
      </Button>
    </div>

    <p v-if="!rows.length" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.page.emptySpecialists') }}</p>

    <template v-else>
      <!-- Desktop/tablet: the full table. Hidden below md — an 8-column table
           has no room to breathe at reception-desk iPad width, where it just
           clips instead of scrolling usably. -->
      <div class="hidden md:block overflow-x-auto rounded-lg border border-border">
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
                <Badge
                  v-if="row.booking_url"
                  variant="outline"
                  class="text-[11px] font-mono max-w-[180px] truncate inline-flex items-center gap-1 align-bottom"
                  :title="t('kb.fields.bookingUrlPersonal')"
                >
                  <Link2 class="w-3 h-3 shrink-0 text-primary" />{{ row.booking_url }}
                </Badge>
                <Badge v-else variant="secondary" class="text-[11px] inline-flex items-center gap-1" :title="t('kb.fields.bookingUrlFallback')">
                  <Building2 class="w-3 h-3 shrink-0" />{{ t('kb.fields.bookingUrlFallback') }}
                </Badge>
              </td>
              <td class="py-2 px-3 align-top whitespace-nowrap">{{ t('kb.page.photosCount', { n: row.portfolio_images.length }) }}</td>
              <td class="py-2 px-3 align-top">
                <div class="flex items-center gap-2">
                  <Switch
                    :model-value="displayedStatus(row) === 'active'"
                    :disabled="actioningRef === row.ref"
                    :aria-label="t('kb.archive.toggleAria')"
                    :data-testid="`specialist-status-switch-${row.ref}`"
                    @update:model-value="() => toggleStatus(row)"
                  />
                  <LoaderCircle v-if="actioningRef === row.ref" class="w-3.5 h-3.5 animate-spin text-muted-foreground" />
                  <span class="text-xs text-muted-foreground">{{ displayedStatus(row) === 'active' ? t('kb.archive.active') : t('kb.archive.archived') }}</span>
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

      <!-- Mobile/reception-desk tablet: one card per specialist instead of a
           table that clips. Same data, same actions, stacked. -->
      <div class="md:hidden space-y-2.5" data-testid="specialists-card-stack">
        <div
          v-for="row in rows"
          :key="row.ref"
          class="rounded-lg border border-border p-3 space-y-2.5"
          data-testid="specialist-card"
        >
          <div class="flex items-center gap-2.5">
            <div class="w-9 h-9 rounded-full bg-muted grid place-items-center shrink-0 text-muted-foreground">
              <UserRound class="w-4.5 h-4.5" />
            </div>
            <div class="min-w-0 flex-1">
              <p class="font-medium truncate">{{ row.full_name || row.ref }}</p>
              <p v-if="row.title" class="text-xs text-muted-foreground truncate">{{ row.title }}</p>
            </div>
            <Badge v-if="row.experience" variant="secondary" class="text-[11px] shrink-0">{{ row.experience }}</Badge>
          </div>

          <div class="flex items-center gap-2 flex-wrap">
            <WeekdayPills :schedule="row.schedule" />
            <span class="text-xs text-muted-foreground">{{ shiftSummary(row) }}</span>
          </div>

          <div class="flex items-center justify-between gap-2 flex-wrap">
            <Badge
              v-if="row.booking_url"
              variant="outline"
              class="text-[11px] font-mono max-w-[220px] truncate inline-flex items-center gap-1"
              :title="t('kb.fields.bookingUrlPersonal')"
            >
              <Link2 class="w-3 h-3 shrink-0 text-primary" />{{ row.booking_url }}
            </Badge>
            <Badge v-else variant="secondary" class="text-[11px] inline-flex items-center gap-1" :title="t('kb.fields.bookingUrlFallback')">
              <Building2 class="w-3 h-3 shrink-0" />{{ t('kb.fields.bookingUrlFallback') }}
            </Badge>
            <span class="text-xs text-muted-foreground">{{ t('kb.page.photosCount', { n: row.portfolio_images.length }) }}</span>
          </div>

          <div class="flex items-center justify-between gap-2 pt-1.5 border-t border-border/60">
            <div class="flex items-center gap-2">
              <Switch
                :model-value="displayedStatus(row) === 'active'"
                :disabled="actioningRef === row.ref"
                :aria-label="t('kb.archive.toggleAria')"
                :data-testid="`specialist-status-switch-mobile-${row.ref}`"
                @update:model-value="() => toggleStatus(row)"
              />
              <LoaderCircle v-if="actioningRef === row.ref" class="w-3.5 h-3.5 animate-spin text-muted-foreground" />
              <span class="text-xs text-muted-foreground">{{ displayedStatus(row) === 'active' ? t('kb.archive.active') : t('kb.archive.archived') }}</span>
            </div>
            <Button variant="outline" size="sm" :data-testid="`specialist-edit-mobile-${row.ref}`" @click="edit(row)">
              <Pencil class="w-3.5 h-3.5" /> {{ t('kb.actions.edit') }}
            </Button>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
