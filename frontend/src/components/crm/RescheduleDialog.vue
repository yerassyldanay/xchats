<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { LoaderCircle } from 'lucide-vue-next'
import { useCrm } from '../../stores/crm'
import type { Followup } from '../../types'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

// TODO.md Tasks phase: the quick "Reschedule / Postpone" action — one click
// on a preset re-dates the task immediately (no separate confirm step); only
// the custom date/time needs an explicit Save. Every preset reuses the SAME
// PATCH /followups/:id endpoint FollowupDialog's own edit form uses, just
// with the action/note/assignee echoed back unchanged — only the due time
// moves.
const props = defineProps<{ followup: Followup }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved', f: Followup): void }>()

const { t } = useI18n()
const crm = useCrm()

const open = ref(true)
function onOpenChange(v: boolean) {
  if (!v) emit('close')
}

const saving = ref(false)
const error = ref('')
const showCustom = ref(false)
const customDate = ref(props.followup.due_date)
const customTime = ref(minutesToInput(props.followup.due_minute))

function minutesToInput(m: number | null): string {
  if (m === null) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(Math.floor(m / 60))}:${pad(m % 60)}`
}
function toDateInput(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

const currentlyDue = computed(() => {
  const fu = props.followup
  if (fu.due_minute === null) return `${fu.due_date} · ${t('crm.followups.allDay')}`
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${fu.due_date} · ${pad(Math.floor(fu.due_minute / 60))}:${pad(fu.due_minute % 60)}`
})

async function apply(due: Date) {
  saving.value = true
  error.value = ''
  try {
    const fu = props.followup
    const saved = await crm.rescheduleFollowup(fu.id, {
      customer_id: fu.customer_id,
      conversation_id: fu.conversation_id,
      channel: fu.channel,
      due_at: due.toISOString(),
      due_date: toDateInput(due),
      due_minute: due.getHours() * 60 + due.getMinutes(),
      action: fu.action,
      note: fu.note,
      assignee_user_id: fu.assignee_user_id,
    })
    emit('saved', saved)
    emit('close')
  } catch {
    error.value = t('crm.reschedule.errors.saveFailed')
  } finally {
    saving.value = false
  }
}

function plusHour() {
  void apply(new Date(Date.now() + 60 * 60 * 1000))
}
function tomorrowMorning() {
  const d = new Date()
  d.setDate(d.getDate() + 1)
  d.setHours(10, 0, 0, 0)
  void apply(d)
}
function nextWeek() {
  const d = new Date()
  d.setDate(d.getDate() + 7)
  d.setHours(10, 0, 0, 0)
  void apply(d)
}
function applyCustom() {
  if (!customDate.value) {
    error.value = t('crm.dialog.errors.dateRequired')
    return
  }
  const [y, mo, dd] = customDate.value.split('-').map(Number)
  const [h, mi] = customTime.value ? customTime.value.split(':').map(Number) : [9, 0]
  void apply(new Date(y, mo - 1, dd, h, mi))
}
</script>

<template>
  <Dialog :open="open" @update:open="onOpenChange">
    <DialogContent class="sm:max-w-[380px]">
      <DialogHeader>
        <DialogTitle>{{ t('crm.reschedule.title') }}</DialogTitle>
      </DialogHeader>

      <p class="-mt-2 text-[13px] text-muted-foreground">{{ t('crm.reschedule.currentlyDue', { when: currentlyDue }) }}</p>

      <div class="space-y-1.5">
        <Button variant="outline" class="w-full justify-start" :disabled="saving" data-testid="reschedule-plus-hour" @click="plusHour">
          {{ t('crm.reschedule.presets.plusHour') }}
        </Button>
        <Button variant="outline" class="w-full justify-start" :disabled="saving" data-testid="reschedule-tomorrow" @click="tomorrowMorning">
          {{ t('crm.reschedule.presets.tomorrow') }}
        </Button>
        <Button variant="outline" class="w-full justify-start" :disabled="saving" data-testid="reschedule-next-week" @click="nextWeek">
          {{ t('crm.reschedule.presets.nextWeek') }}
        </Button>
        <Button
          variant="outline"
          class="w-full justify-start"
          :class="showCustom ? 'border-primary/50 bg-primary/5' : ''"
          :disabled="saving"
          data-testid="reschedule-custom-toggle"
          @click="showCustom = !showCustom"
        >
          {{ t('crm.reschedule.presets.custom') }}
        </Button>
      </div>

      <div v-if="showCustom" class="grid grid-cols-2 gap-3">
        <label class="block">
          <span class="text-[13px] font-medium">{{ t('crm.dialog.date') }}</span>
          <Input v-model="customDate" type="date" class="mt-1.5 h-9" data-testid="reschedule-custom-date" />
        </label>
        <label class="block">
          <span class="text-[13px] font-medium">{{ t('crm.dialog.time') }}</span>
          <Input v-model="customTime" type="time" class="mt-1.5 h-9" data-testid="reschedule-custom-time" />
        </label>
      </div>

      <p v-if="error" class="text-[13px] text-destructive">{{ error }}</p>

      <DialogFooter>
        <Button variant="ghost" :disabled="saving" @click="emit('close')">{{ t('crm.dialog.cancel') }}</Button>
        <Button v-if="showCustom" :disabled="saving" data-testid="reschedule-custom-save" @click="applyCustom">
          <LoaderCircle v-if="saving" class="w-4 h-4 animate-spin" />
          {{ t('crm.dialog.save') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
