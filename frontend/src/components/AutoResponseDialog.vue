<script setup lang="ts">
// AutoResponseDialog configures one account's auto-response policy: a
// per-weekday schedule decides whether an arriving message qualifies, a
// delay gives an operator a grace period to step in first, and if nobody
// does, the backend sends — either the AI draft or a fixed canned text.
//
// The schedule UI models a per-day MODE, never a raw wrapping interval:
// <input type="time"> cannot hold "24:00", and a "-> Tue 09:00" next-day
// hint would describe semantics the backend does not implement (see
// internal/autoresponse's doc comment on why wraps are same-weekday
// complements, never a spill into the next day).
import { computed, reactive, ref, watch } from 'vue'
import { CalendarClock, CircleAlert, LoaderCircle } from 'lucide-vue-next'
import { useAccounts } from '@/stores/accounts'
import { ApiError } from '@/api/client'
import type { Account, AutoResponse } from '@/types'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger } from '@/components/ui/select'
import {
  DAY_MODES,
  DAY_MODE_LABELS,
  WEEKDAYS,
  WEEKDAY_FULL,
  WEEKDAY_SHORT,
  type DayMode,
  type DayState,
  coverageStrip,
  dayStatesToWindows,
  defaultDayState,
  formatCooldownSeconds,
  formatDelaySeconds,
  windowsToDayStates,
} from '@/lib/autoResponse'

const props = defineProps<{ account: Account }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()

const accounts = useAccounts()
const open = ref(true)
const busy = ref(false)
const error = ref('')

const buf = reactive({
  enabled: false,
  reply_mode: 'ai' as 'ai' | 'fixed',
  fixed_text: '',
  timezone: 'Asia/Almaty',
  delay_seconds: 120,
  cooldown_seconds: 300,
  skip_when_escalated: true,
  pause_when_assigned: false,
})
const days = reactive<Record<number, DayState>>(
  Object.fromEntries(WEEKDAYS.map((wd) => [wd, defaultDayState()])) as Record<number, DayState>,
)

let seededFor = ''
watch(
  () => props.account,
  (a) => {
    if (!a || seededFor === a.id) return
    seededFor = a.id
    const ar = a.auto_response
    buf.enabled = ar.enabled
    buf.reply_mode = ar.reply_mode
    buf.fixed_text = ar.fixed_text
    buf.timezone = ar.timezone || 'Asia/Almaty'
    buf.delay_seconds = ar.delay_seconds
    buf.cooldown_seconds = ar.cooldown_seconds
    buf.skip_when_escalated = ar.skip_when_escalated
    buf.pause_when_assigned = ar.pause_when_assigned
    const seeded = windowsToDayStates(ar.windows)
    for (const wd of WEEKDAYS) days[wd] = seeded[wd]
  },
  { immediate: true },
)

function onOpenChange(v: boolean) {
  if (!v) emit('close')
}

// «Применить к Пн–Пт» — copy Monday's schedule onto Tuesday through Friday,
// the one bulk affordance that makes a 5-day-workweek schedule fast to set.
function applyMondayToWeekdays() {
  for (const wd of [2, 3, 4, 5]) {
    days[wd] = { ...days[1] }
  }
}

const coverage = computed(() => coverageStrip(days))
const hasAnyCoverage = computed(() => coverage.value.some((row) => row.some(Boolean)))

const DELAY_OPTIONS = [30, 60, 120, 300, 600, 900, 1800]
const COOLDOWN_OPTIONS = [0, 60, 300, 600, 1800, 3600]
const TIMEZONES: { value: string; label: string }[] = [
  { value: 'Asia/Almaty', label: 'Алматы (UTC+5)' },
  { value: 'Asia/Aqtobe', label: 'Актобе (UTC+5)' },
  { value: 'Asia/Aqtau', label: 'Актау (UTC+5)' },
  { value: 'Asia/Atyrau', label: 'Атырау (UTC+5)' },
  { value: 'Asia/Oral', label: 'Уральск (UTC+5)' },
  { value: 'Asia/Qyzylorda', label: 'Кызылорда (UTC+5)' },
  { value: 'Europe/Moscow', label: 'Москва (UTC+3)' },
  { value: 'UTC', label: 'UTC' },
]
const timezoneLabel = computed(() => TIMEZONES.find((t) => t.value === buf.timezone)?.label ?? buf.timezone)

async function submit() {
  error.value = ''
  if (buf.enabled) {
    if (!hasAnyCoverage.value) {
      error.value = 'Укажите хотя бы один день с расписанием.'
      return
    }
    if (buf.reply_mode === 'fixed' && !buf.fixed_text.trim()) {
      error.value = 'Укажите текст автоответа для режима «Фиксированный текст».'
      return
    }
  }
  busy.value = true
  try {
    const payload: AutoResponse = {
      enabled: buf.enabled,
      reply_mode: buf.reply_mode,
      fixed_text: buf.fixed_text.trim(),
      timezone: buf.timezone,
      delay_seconds: buf.delay_seconds,
      cooldown_seconds: buf.cooldown_seconds,
      skip_when_escalated: buf.skip_when_escalated,
      pause_when_assigned: buf.pause_when_assigned,
      windows: dayStatesToWindows(days),
    }
    await accounts.saveAutoResponse(props.account.id, payload)
    emit('saved')
    emit('close')
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Не удалось сохранить настройки автоответа.'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Dialog :open="open" @update:open="onOpenChange">
    <DialogContent class="max-w-lg max-h-[85vh] overflow-y-auto">
      <DialogHeader>
        <DialogTitle>
          <span class="w-8 h-8 rounded-lg bg-primary/10 text-primary grid place-items-center">
            <CalendarClock class="w-4 h-4" />
          </span>
          Автоответ — {{ account.display_name }}
        </DialogTitle>
      </DialogHeader>

      <div class="px-5 py-5 space-y-5">
        <label class="flex items-center justify-between gap-3 rounded-md border border-border px-3 py-2.5">
          <span class="text-sm">
            <span class="font-medium">Автоответ включён</span>
            <br />
            <span class="text-xs text-muted-foreground">
              Если оператор не отвечает по расписанию, бот отправит подсказку сам.
            </span>
          </span>
          <Switch v-model="buf.enabled" />
        </label>

        <div class="space-y-2">
          <label class="text-xs font-medium text-muted-foreground">Что отправлять</label>
          <Select :model-value="buf.reply_mode" @update:model-value="(v) => (buf.reply_mode = v as 'ai' | 'fixed')">
            <SelectTrigger class="h-9">
              <span>{{ buf.reply_mode === 'fixed' ? 'Фиксированный текст' : 'Подсказка ИИ' }}</span>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="ai">Подсказка ИИ</SelectItem>
              <SelectItem value="fixed">Фиксированный текст</SelectItem>
            </SelectContent>
          </Select>
          <Textarea
            v-if="buf.reply_mode === 'fixed'"
            v-model="buf.fixed_text"
            rows="2"
            placeholder="Например: Сейчас нерабочее время, ответим утром."
            class="min-h-0 text-[14px]"
          />
        </div>

        <div class="space-y-1.5">
          <div class="flex items-center justify-between">
            <label class="text-xs font-medium text-muted-foreground">Расписание</label>
            <Button variant="ghost" size="sm" class="h-6 px-2 text-xs" @click="applyMondayToWeekdays">
              Применить Пн ко Вт–Пт
            </Button>
          </div>
          <div v-for="wd in WEEKDAYS" :key="wd" class="flex items-center gap-1.5">
            <span class="w-7 shrink-0 text-xs font-medium text-muted-foreground" :title="WEEKDAY_FULL[wd]">
              {{ WEEKDAY_SHORT[wd] }}
            </span>
            <Select :model-value="days[wd].mode" @update:model-value="(v) => (days[wd].mode = v as DayMode)">
              <SelectTrigger class="h-8 w-40 shrink-0 text-xs">
                <span>{{ DAY_MODE_LABELS[days[wd].mode] }}</span>
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="m in DAY_MODES" :key="m" :value="m">{{ DAY_MODE_LABELS[m] }}</SelectItem>
              </SelectContent>
            </Select>
            <template v-if="days[wd].mode === 'outside' || days[wd].mode === 'inside'">
              <Input v-model="days[wd].start" type="time" class="h-8 w-[6.5rem] px-2 text-xs" />
              <span class="text-xs text-muted-foreground">–</span>
              <Input v-model="days[wd].end" type="time" class="h-8 w-[6.5rem] px-2 text-xs" />
            </template>
          </div>
        </div>

        <!-- coverage strip: the only honest way to show these are per-day
             rules, not spills, and to catch a residual gap before saving -->
        <div class="space-y-1">
          <label class="text-xs font-medium text-muted-foreground">Покрытие недели (0–24ч)</label>
          <div class="space-y-0.5">
            <div v-for="(row, i) in coverage" :key="i" class="flex items-center gap-1.5">
              <span class="w-7 shrink-0 text-[10px] text-muted-foreground">{{ WEEKDAY_SHORT[WEEKDAYS[i]] }}</span>
              <div class="grid flex-1 grid-cols-[repeat(24,minmax(0,1fr))] gap-px">
                <span
                  v-for="(covered, h) in row"
                  :key="h"
                  class="h-2.5 rounded-[1px]"
                  :class="covered ? 'bg-primary' : 'bg-muted'"
                  :title="`${WEEKDAY_SHORT[WEEKDAYS[i]]} ${String(h).padStart(2, '0')}:00`"
                />
              </div>
            </div>
          </div>
          <p v-if="buf.enabled && !hasAnyCoverage" class="text-xs text-destructive">
            Расписание пустое — автоответ никогда не сработает.
          </p>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div class="space-y-1.5">
            <label class="text-xs font-medium text-muted-foreground">Часовой пояс</label>
            <Select :model-value="buf.timezone" @update:model-value="(v) => (buf.timezone = v as string)">
              <SelectTrigger class="h-9"><span>{{ timezoneLabel }}</span></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="tz in TIMEZONES" :key="tz.value" :value="tz.value">{{ tz.label }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="space-y-1.5">
            <label class="text-xs font-medium text-muted-foreground">Задержка перед отправкой</label>
            <Select
              :model-value="String(buf.delay_seconds)"
              @update:model-value="(v) => (buf.delay_seconds = Number(v))"
            >
              <SelectTrigger class="h-9"><span>{{ formatDelaySeconds(buf.delay_seconds) }}</span></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="s in DELAY_OPTIONS" :key="s" :value="String(s)">{{ formatDelaySeconds(s) }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="col-span-2 space-y-1.5">
            <label class="text-xs font-medium text-muted-foreground">
              Не повторять автоответ, если недавно уже отвечали
            </label>
            <Select
              :model-value="String(buf.cooldown_seconds)"
              @update:model-value="(v) => (buf.cooldown_seconds = Number(v))"
            >
              <SelectTrigger class="h-9"><span>{{ formatCooldownSeconds(buf.cooldown_seconds) }}</span></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="s in COOLDOWN_OPTIONS" :key="s" :value="String(s)">
                  {{ formatCooldownSeconds(s) }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        <div class="space-y-2">
          <label class="flex items-center justify-between gap-3">
            <span class="text-sm">Не отвечать, если ИИ передал диалог оператору</span>
            <Switch v-model="buf.skip_when_escalated" />
          </label>
          <label class="flex items-center justify-between gap-3">
            <span class="text-sm">Не отвечать, если чат уже назначен оператору</span>
            <Switch v-model="buf.pause_when_assigned" />
          </label>
        </div>

        <p v-if="error" class="flex items-start gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ error }}
        </p>

        <Button :disabled="busy" class="w-full" @click="submit">
          <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
          <CalendarClock v-else class="w-4 h-4" />
          {{ busy ? 'Сохранение…' : 'Сохранить' }}
        </Button>
      </div>
    </DialogContent>
  </Dialog>
</template>
