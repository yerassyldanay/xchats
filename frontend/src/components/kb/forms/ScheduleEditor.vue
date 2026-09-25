<script setup lang="ts">
// ScheduleEditor is the shared 7-day (Monday-first) shift/break editor,
// used both by ContactsForm.vue (the salon's own fallback hours) and
// SpecialistFormDialog.vue (one master's shift) — v-model on a Schedule
// (ScheduleDay[]) array. A pure controlled component, same convention as
// MediaFieldPicker.vue/AdditionalFactsEditor.vue: no internal buffer, every
// change reconstructs the whole array from props.modelValue and emits it,
// so the owning form's payload() always carries this field's full current
// intent.
//
// Backend contract (playground.ts/types.ts's own doc comments): at most 7
// entries, one per WORKED weekday — a missing weekday is simply ABSENT from
// the array (an off-day), never present with empty times. Unchecking a day
// below removes its entry entirely rather than blanking its times.
import { useI18n } from 'vue-i18n'
import { Plus, Trash2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { WEEKDAY_ORDER } from '@/components/kb/kbEntities'
import type { Schedule, ScheduleDay, WeekdayRef } from '@/types'

const props = defineProps<{ modelValue: Schedule }>()
const emit = defineEmits<{ 'update:modelValue': [Schedule] }>()
const { t } = useI18n()

const DEFAULT_START = '10:00'
const DEFAULT_END = '19:00'

function dayOf(ref: WeekdayRef): ScheduleDay | undefined {
  return props.modelValue.find((d) => d.ref === ref)
}
function isWorked(ref: WeekdayRef): boolean {
  return !!dayOf(ref)
}
function weekdayLabel(ref: WeekdayRef): string {
  return t('kb.schedule.weekday.' + ref)
}

// emitReplace always re-orders to WEEKDAY_ORDER — a stable, readable payload
// regardless of which order the operator actually toggled days in.
function emitReplace(days: ScheduleDay[]) {
  const byRef = new Map(days.map((d) => [d.ref, d]))
  emit(
    'update:modelValue',
    WEEKDAY_ORDER.filter((r) => byRef.has(r)).map((r) => byRef.get(r)!)
  )
}

function toggleWorked(ref: WeekdayRef, worked: boolean) {
  const rest = props.modelValue.filter((d) => d.ref !== ref)
  emitReplace(worked ? [...rest, { ref, day: weekdayLabel(ref), start: DEFAULT_START, end: DEFAULT_END, breaks: [] }] : rest)
}
function patchDay(ref: WeekdayRef, patch: Partial<Omit<ScheduleDay, 'ref'>>) {
  const day = dayOf(ref)
  if (!day) return
  const rest = props.modelValue.filter((d) => d.ref !== ref)
  // day (the free-text label) is re-derived from the current locale on every
  // edit — it is captured at authoring time, and re-typing it here would be
  // busywork the operator never asked for.
  emitReplace([...rest, { ...day, ...patch, day: weekdayLabel(ref) }])
}
function addBreak(ref: WeekdayRef) {
  const day = dayOf(ref)
  if (!day) return
  patchDay(ref, { breaks: [...day.breaks, { start: day.start, end: day.end }] })
}
function removeBreak(ref: WeekdayRef, i: number) {
  const day = dayOf(ref)
  if (!day) return
  patchDay(ref, { breaks: day.breaks.filter((_, idx) => idx !== i) })
}
function patchBreak(ref: WeekdayRef, i: number, patch: Partial<{ start: string; end: string }>) {
  const day = dayOf(ref)
  if (!day) return
  patchDay(ref, { breaks: day.breaks.map((b, idx) => (idx === i ? { ...b, ...patch } : b)) })
}

// --- soft, inline validation only — never blocks submit, the backend is the
// authoritative check (see this component's own doc comment / the plan's
// "does not need to be bulletproof" note). One message per day, checked in
// the same order the backend validates: shift ordering, then each break's
// own ordering and containment within the shift, then overlaps between
// breaks on that day.
function warningFor(ref: WeekdayRef): string {
  const day = dayOf(ref)
  if (!day || !day.start || !day.end) return ''
  if (day.start >= day.end) return t('kb.schedule.errStartEnd')
  const timedBreaks = day.breaks.filter((b) => b.start && b.end)
  for (const b of timedBreaks) {
    if (b.start >= b.end) return t('kb.schedule.errBreakOrder')
    if (b.start < day.start || b.end > day.end) return t('kb.schedule.errBreakOutside')
  }
  const sorted = [...timedBreaks].sort((a, b) => a.start.localeCompare(b.start))
  for (let i = 1; i < sorted.length; i++) {
    if (sorted[i].start < sorted[i - 1].end) return t('kb.schedule.errBreakOverlap')
  }
  return ''
}
</script>

<template>
  <div class="space-y-2" data-testid="schedule-editor">
    <div
      v-for="ref in WEEKDAY_ORDER"
      :key="ref"
      class="rounded-md border border-border p-2.5 space-y-2"
      :data-testid="`schedule-day-${ref}`"
    >
      <div class="flex items-center gap-3 flex-wrap">
        <label class="flex items-center gap-2 h-9 cursor-pointer">
          <input
            type="checkbox"
            class="h-4 w-4 rounded border-border accent-primary"
            :checked="isWorked(ref)"
            :data-testid="`schedule-worked-${ref}`"
            @change="toggleWorked(ref, ($event.target as HTMLInputElement).checked)"
          />
          <span class="text-sm font-medium w-28 shrink-0">{{ weekdayLabel(ref) }}</span>
        </label>

        <template v-if="isWorked(ref)">
          <Input
            type="time"
            class="h-9 w-[120px] font-mono text-sm"
            :model-value="dayOf(ref)!.start"
            :aria-label="t('kb.schedule.start')"
            :data-testid="`schedule-start-${ref}`"
            @update:model-value="(v) => patchDay(ref, { start: String(v) })"
          />
          <span class="text-muted-foreground text-sm">–</span>
          <Input
            type="time"
            class="h-9 w-[120px] font-mono text-sm"
            :model-value="dayOf(ref)!.end"
            :aria-label="t('kb.schedule.end')"
            :data-testid="`schedule-end-${ref}`"
            @update:model-value="(v) => patchDay(ref, { end: String(v) })"
          />
        </template>
        <span v-else class="text-sm text-muted-foreground">{{ t('kb.schedule.dayOff') }}</span>
      </div>

      <div v-if="isWorked(ref)" class="pl-7 space-y-1.5">
        <div
          v-for="(b, i) in dayOf(ref)!.breaks"
          :key="i"
          class="flex items-center gap-2 flex-wrap"
          :data-testid="`schedule-break-${ref}-${i}`"
        >
          <span class="text-xs text-muted-foreground shrink-0">{{ t('kb.schedule.breaks') }}</span>
          <Input
            type="time"
            class="h-8 w-[110px] font-mono text-xs"
            :model-value="b.start"
            :aria-label="t('kb.schedule.breakStart')"
            :data-testid="`schedule-break-start-${ref}-${i}`"
            @update:model-value="(v) => patchBreak(ref, i, { start: String(v) })"
          />
          <span class="text-muted-foreground text-xs">–</span>
          <Input
            type="time"
            class="h-8 w-[110px] font-mono text-xs"
            :model-value="b.end"
            :aria-label="t('kb.schedule.breakEnd')"
            :data-testid="`schedule-break-end-${ref}-${i}`"
            @update:model-value="(v) => patchBreak(ref, i, { end: String(v) })"
          />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            class="h-7 w-7 shrink-0 text-destructive hover:bg-destructive/10"
            :title="t('kb.schedule.removeBreak')"
            :data-testid="`schedule-break-remove-${ref}-${i}`"
            @click="removeBreak(ref, i)"
          >
            <Trash2 class="w-3.5 h-3.5" />
          </Button>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          class="h-7 text-xs"
          :data-testid="`schedule-break-add-${ref}`"
          @click="addBreak(ref)"
        >
          <Plus class="w-3.5 h-3.5" /> {{ t('kb.schedule.addBreak') }}
        </Button>
        <p v-if="warningFor(ref)" class="text-[11px] text-destructive" :data-testid="`schedule-warning-${ref}`">
          {{ warningFor(ref) }}
        </p>
      </div>
    </div>
  </div>
</template>
