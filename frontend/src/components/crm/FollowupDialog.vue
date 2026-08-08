<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { LoaderCircle } from 'lucide-vue-next'
import { useCrm } from '../../stores/crm'
import { useInbox } from '../../stores/inbox'
import { currentUtcOffsetMinutes, formatUtcOffset } from '../../lib/schedule'
import type { Followup, FollowupAction } from '../../types'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

// Create or reschedule one follow-up. Date and time are entered in browser-
// local time and converted to UTC only at the wire boundary, exactly like
// AutomationSettingsDialog does for schedule windows — the backend stores both
// the UTC instant (which it orders and buckets by) and the local wall clock
// (which this form reads back), so a follow-up never drifts by an offset.
const props = defineProps<{
  customerId: string
  conversationId?: string | null
  channel?: string
  followup?: Followup | null
}>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved', f: Followup): void }>()

const { t } = useI18n()
const crm = useCrm()
const inbox = useInbox()

const open = ref(true)
function onOpenChange(v: boolean) {
  if (!v) emit('close')
}

const offsetLabel = formatUtcOffset(currentUtcOffsetMinutes())
const UNASSIGNED = '__unassigned__'
const ACTIONS: FollowupAction[] = ['call', 'message', 'meeting', 'other']

// Seed from the follow-up being rescheduled, else default to tomorrow — the
// overwhelmingly common "call them back" case, and a date that is never
// already in the past.
function defaultDate(): string {
  const d = new Date()
  d.setDate(d.getDate() + 1)
  return toDateInput(d)
}
function toDateInput(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

const date = ref(props.followup?.due_date || defaultDate())
const time = ref(minutesToInput(props.followup?.due_minute ?? null))
const action = ref<FollowupAction>(props.followup?.action || 'call')
const note = ref(props.followup?.note || '')
const assignee = ref(props.followup?.assignee_user_id || UNASSIGNED)
const saving = ref(false)
const error = ref('')

function minutesToInput(m: number | null): string {
  if (m === null) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(Math.floor(m / 60))}:${pad(m % 60)}`
}
function inputToMinutes(v: string): number | null {
  if (!v) return null
  const [h, m] = v.split(':').map(Number)
  if (Number.isNaN(h) || Number.isNaN(m)) return null
  return h * 60 + m
}

const title = computed(() => (props.followup ? t('crm.dialog.editTitle') : t('crm.dialog.title')))

async function save() {
  if (!date.value) {
    error.value = t('crm.dialog.errors.dateRequired')
    return
  }
  saving.value = true
  error.value = ''
  try {
    const minutes = inputToMinutes(time.value)
    // An all-day follow-up is due at local 09:00 rather than midnight: it
    // should surface at the start of the working day, not sort above
    // everything that has a real time on it.
    const [y, mo, d] = date.value.split('-').map(Number)
    const local = new Date(y, mo - 1, d, minutes === null ? 9 : Math.floor(minutes / 60), minutes === null ? 0 : minutes % 60)
    const body = {
      customer_id: props.customerId,
      conversation_id: props.conversationId ?? null,
      channel: props.channel || '',
      due_at: local.toISOString(),
      due_date: date.value,
      due_minute: minutes,
      action: action.value,
      note: note.value,
      assignee_user_id: assignee.value === UNASSIGNED ? '' : assignee.value,
    }
    const saved = props.followup
      ? await crm.rescheduleFollowup(props.followup.id, body)
      : await crm.createFollowup(body)
    emit('saved', saved)
    emit('close')
  } catch {
    error.value = t('crm.dialog.errors.saveFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Dialog :open="open" @update:open="onOpenChange">
    <DialogContent class="sm:max-w-[440px]">
      <DialogHeader>
        <DialogTitle>{{ title }}</DialogTitle>
      </DialogHeader>

      <div class="space-y-4">
        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="text-[13px] font-medium">{{ t('crm.dialog.date') }}</span>
            <Input v-model="date" type="date" class="mt-1.5 h-9" />
          </label>
          <label class="block">
            <span class="text-[13px] font-medium">{{ t('crm.dialog.time') }}</span>
            <Input v-model="time" type="time" class="mt-1.5 h-9" />
          </label>
        </div>
        <p class="text-xs text-muted-foreground -mt-2">
          {{ t('crm.dialog.timeHint') }} {{ t('crm.dialog.timezoneHint', { offset: offsetLabel }) }}
        </p>

        <label class="block">
          <span class="text-[13px] font-medium">{{ t('crm.dialog.action') }}</span>
          <Select :model-value="action" @update:model-value="(v) => (action = v as FollowupAction)">
            <SelectTrigger class="mt-1.5 h-9 text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="a in ACTIONS" :key="a" :value="a">
                {{ t('crm.followups.action.' + a) }}
              </SelectItem>
            </SelectContent>
          </Select>
        </label>

        <label class="block">
          <span class="text-[13px] font-medium">{{ t('crm.dialog.assignee') }}</span>
          <Select :model-value="assignee" @update:model-value="(v) => (assignee = v as string)">
            <SelectTrigger class="mt-1.5 h-9 text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem :value="UNASSIGNED">{{ t('crm.panel.unassigned') }}</SelectItem>
              <SelectItem v-for="u in inbox.users" :key="u.id" :value="u.id">
                {{ u.name || u.email }}
              </SelectItem>
            </SelectContent>
          </Select>
        </label>

        <label class="block">
          <span class="text-[13px] font-medium">{{ t('crm.dialog.note') }}</span>
          <Textarea
            v-model="note"
            rows="3"
            :placeholder="t('crm.dialog.notePlaceholder')"
            class="mt-1.5 text-[14px]"
          />
        </label>

        <p v-if="error" class="text-[13px] text-destructive">{{ error }}</p>
      </div>

      <DialogFooter>
        <Button variant="ghost" :disabled="saving" @click="emit('close')">
          {{ t('crm.dialog.cancel') }}
        </Button>
        <Button :disabled="saving" @click="save">
          <LoaderCircle v-if="saving" class="w-4 h-4 animate-spin" />
          {{ saving ? t('crm.dialog.saving') : t('crm.dialog.save') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
