<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, Clock, LoaderCircle, Plus, Trash2 } from 'lucide-vue-next'
import { useAccounts } from '../stores/accounts'
import { ApiError } from '../api/client'
import type { Account, AutomationMode, ScheduleWindow } from '../types'
import {
  currentUtcOffsetMinutes,
  formatUtcOffset,
  utcToLocal,
  localToUtc,
  minutesToHHMM,
  hhmmToMinutes,
  endMinutesFromInput,
  endInputFromMinutes,
  alwaysWindows,
  nightsWindows,
  weekendsWindows,
  detectPreset,
  type PresetName,
} from '../lib/schedule'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const props = defineProps<{ account: Account }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()
const { t } = useI18n()
const accounts = useAccounts()

const open = ref(true)
function onOpenChange(v: boolean) {
  if (!v) emit('close')
}

// Computed once for the dialog's lifetime — the browser's own zone does not
// change while a settings dialog is open.
const offsetMinutes = currentUtcOffsetMinutes()
const offsetLabel = formatUtcOffset(offsetMinutes)

const MODES: AutomationMode[] = ['off', 'suggestions', 'scheduled_auto']
const mode = ref<AutomationMode>(props.account.automation.mode)

const waitMode = ref<'default' | 'custom'>(props.account.automation.wait_seconds_override === null ? 'default' : 'custom')
const customWait = ref<number>(props.account.automation.wait_seconds_override ?? props.account.automation.default_wait_seconds)

// The schedule is edited in browser-local time and converted to/from UTC
// only at the wire boundary (load here, save on submit) — see lib/schedule.ts.
// Kept regardless of the selected mode so switching away from
// scheduled_auto and back within one editing session never discards it.
const localWindows = ref<ScheduleWindow[]>(utcToLocal(props.account.automation.schedule, offsetMinutes))

const activePreset = computed<PresetName>(() => detectPreset(localWindows.value))

const WEEKDAYS = [0, 1, 2, 3, 4, 5, 6]
const WEEKDAY_KEYS = ['sun', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat']

function applyPreset(name: PresetName) {
  if (name === 'always') localWindows.value = alwaysWindows()
  else if (name === 'nights') localWindows.value = nightsWindows()
  else if (name === 'weekends') localWindows.value = weekendsWindows()
  else if (name === 'custom' && localWindows.value.length === 0) {
    localWindows.value = [{ weekday: 1, start_minute: 9 * 60, end_minute: 18 * 60 }]
  }
}

function addWindow() {
  localWindows.value = [...localWindows.value, { weekday: 1, start_minute: 9 * 60, end_minute: 18 * 60 }]
}
function removeWindow(i: number) {
  localWindows.value = localWindows.value.filter((_, idx) => idx !== i)
}
function setStart(w: ScheduleWindow, hhmm: string) {
  w.start_minute = hhmmToMinutes(hhmm)
}
function setEnd(w: ScheduleWindow, hhmm: string) {
  w.end_minute = endMinutesFromInput(hhmm)
}

const invalidWindow = computed(() => localWindows.value.some((w) => w.start_minute === w.end_minute))
const invalidWait = computed(
  () => waitMode.value === 'custom' && (!Number.isInteger(customWait.value) || customWait.value < 0 || customWait.value > 60),
)

const busy = ref(false)
const error = ref('')

async function submit() {
  error.value = ''
  if (invalidWait.value) {
    error.value = t('automation.dialog.errors.invalidWait')
    return
  }
  if (invalidWindow.value) {
    error.value = t('automation.dialog.errors.invalidWindow')
    return
  }
  busy.value = true
  try {
    // "Always" is bypassed around the generic per-window UTC conversion on
    // purpose — see lib/schedule.ts's alwaysWindows doc comment for why
    // (it's the one shape a non-hour-aligned offset would otherwise split
    // into up to 14 rows for no behavioral difference).
    const schedule = activePreset.value === 'always' ? alwaysWindows() : localToUtc(localWindows.value, offsetMinutes)
    await accounts.updateAutomation(props.account.id, {
      mode: mode.value,
      wait_seconds_override: waitMode.value === 'default' ? null : customWait.value,
      schedule,
    })
    emit('saved')
    emit('close')
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : t('automation.dialog.errors.saveFailed')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Dialog :open="open" @update:open="onOpenChange">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>
          <span class="w-8 h-8 rounded-lg bg-primary/10 text-primary grid place-items-center">
            <Clock class="w-4 h-4" />
          </span>
          {{ t('automation.dialog.title', { name: account.display_name }) }}
        </DialogTitle>
      </DialogHeader>

      <div class="px-5 py-5 space-y-5 max-h-[70vh] overflow-y-auto">
        <!-- mode -->
        <div>
          <span class="text-xs font-medium text-muted-foreground">{{ t('automation.dialog.modeLabel') }}</span>
          <div class="mt-1.5 space-y-2">
            <button
              v-for="m in MODES"
              :key="m"
              type="button"
              class="w-full text-left rounded-lg border px-3 py-2.5 transition"
              :class="mode === m ? 'border-primary bg-primary/5' : 'border-border hover:bg-muted/50'"
              @click="mode = m"
            >
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">{{ t(`automation.dialog.mode.${m}`) }}</span>
                <span
                  class="w-4 h-4 rounded-full border shrink-0"
                  :class="mode === m ? 'border-primary bg-primary' : 'border-border'"
                />
              </div>
              <p class="mt-0.5 text-xs text-muted-foreground">{{ t(`automation.dialog.mode.${m}Hint`) }}</p>
            </button>
          </div>
        </div>

        <!-- message wait -->
        <div>
          <span class="text-xs font-medium text-muted-foreground">{{ t('automation.dialog.waitLabel') }}</span>
          <p class="mt-0.5 text-xs text-muted-foreground">{{ t('automation.dialog.waitHint') }}</p>
          <div class="mt-1.5 flex items-center gap-2">
            <Button
              type="button"
              size="sm"
              :variant="waitMode === 'default' ? 'default' : 'outline'"
              @click="waitMode = 'default'"
            >
              {{ t('automation.dialog.waitDefault', { seconds: account.automation.default_wait_seconds }) }}
            </Button>
            <Button
              type="button"
              size="sm"
              :variant="waitMode === 'custom' ? 'default' : 'outline'"
              @click="waitMode = 'custom'"
            >
              {{ t('automation.dialog.waitCustom') }}
            </Button>
            <div v-if="waitMode === 'custom'" class="flex items-center gap-1.5">
              <Input v-model.number="customWait" type="number" min="0" max="60" class="w-20 h-9" />
              <span class="text-xs text-muted-foreground">{{ t('automation.dialog.waitSecondsUnit') }}</span>
            </div>
          </div>
        </div>

        <!-- schedule -->
        <div v-if="mode === 'scheduled_auto'">
          <div class="flex items-center justify-between">
            <span class="text-xs font-medium text-muted-foreground">{{ t('automation.dialog.scheduleLabel') }}</span>
            <span class="text-[11px] text-muted-foreground">{{ t('automation.dialog.scheduleTimezoneHint', { offset: offsetLabel }) }}</span>
          </div>

          <div class="mt-1.5 flex flex-wrap gap-1.5">
            <Button
              v-for="p in (['always', 'nights', 'weekends', 'custom'] as PresetName[])"
              :key="p"
              type="button"
              size="sm"
              :variant="activePreset === p ? 'default' : 'outline'"
              @click="applyPreset(p)"
            >
              {{ t(`automation.dialog.presets.${p}`) }}
            </Button>
          </div>

          <div class="mt-3 space-y-2">
            <div
              v-for="(w, i) in localWindows"
              :key="i"
              class="flex items-center gap-2 rounded-md border border-border p-2"
            >
              <select
                v-model.number="w.weekday"
                class="h-9 w-28 shrink-0 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <option v-for="d in WEEKDAYS" :key="d" :value="d">{{ t(`automation.dialog.weekday.${WEEKDAY_KEYS[d]}`) }}</option>
              </select>
              <input
                type="time"
                :value="minutesToHHMM(w.start_minute)"
                class="h-9 w-[105px] shrink-0 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                @change="setStart(w, ($event.target as HTMLInputElement).value)"
              />
              <span class="text-xs text-muted-foreground shrink-0">{{ t('automation.dialog.rangeTo') }}</span>
              <input
                type="time"
                :value="endInputFromMinutes(w.end_minute)"
                class="h-9 w-[105px] shrink-0 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                @change="setEnd(w, ($event.target as HTMLInputElement).value)"
              />
              <Button type="button" variant="ghost" size="icon" class="w-8 h-8 ml-auto shrink-0 text-destructive hover:bg-destructive/10" :title="t('automation.dialog.removeWindow')" @click="removeWindow(i)">
                <Trash2 class="w-4 h-4" />
              </Button>
            </div>

            <Button type="button" variant="outline" size="sm" @click="addWindow">
              <Plus class="w-4 h-4" /> {{ t('automation.dialog.addWindow') }}
            </Button>
          </div>

          <p class="mt-2 text-[11px] text-muted-foreground">{{ t('automation.dialog.overnightHint') }}</p>
          <p v-if="localWindows.length === 0" class="mt-2 text-xs text-amber-600 dark:text-amber-400">
            {{ t('automation.dialog.noWindows') }}
          </p>
        </div>

        <p v-if="error" class="flex items-start gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ error }}
        </p>
      </div>

      <DialogFooter>
        <Button type="button" variant="ghost" @click="emit('close')">{{ t('automation.dialog.cancel') }}</Button>
        <Button type="button" :disabled="busy" @click="submit">
          <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
          {{ busy ? t('automation.dialog.saving') : t('automation.dialog.save') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
