<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { LoaderCircle, Search, X } from 'lucide-vue-next'
import { useAuth } from '../../stores/auth'
import { useCrm } from '../../stores/crm'
import { useInbox } from '../../stores/inbox'
import { api } from '../../api/client'
import { currentUtcOffsetMinutes, formatUtcOffset } from '../../lib/schedule'
import { initials, colorFor } from '../../lib/format'
import type { Customer, Followup, FollowupAction } from '../../types'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'

// Create or reschedule one follow-up. Date and time are entered in browser-
// local time and converted to UTC only at the wire boundary, exactly like
// AutomationSettingsDialog does for schedule windows — the backend stores both
// the UTC instant (which it orders and buckets by) and the local wall clock
// (which this form reads back), so a follow-up never drifts by an offset.
//
// customerId is optional (TODO.md "+ New Task" on /followups): opened from a
// conversation/customer sidebar it is always given; opened from the Tasks
// board's own "+ New task" button it is not, and the form grows a customer
// search/picker at the top instead — see needsCustomerPicker below.
const props = defineProps<{
  customerId?: string
  conversationId?: string | null
  channel?: string
  followup?: Followup | null
}>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved', f: Followup): void }>()

const { t } = useI18n()
const crm = useCrm()
const inbox = useInbox()
const auth = useAuth()

const needsCustomerPicker = computed(() => !props.customerId && !props.followup)
const pickedCustomer = ref<Customer | null>(null)
const customerQuery = ref('')
const customerResults = ref<Customer[]>([])
const searchingCustomers = ref(false)
let customerSearchTimer: number | undefined
watch(customerQuery, (q) => {
  window.clearTimeout(customerSearchTimer)
  const query = q.trim()
  if (!query) {
    customerResults.value = []
    return
  }
  customerSearchTimer = window.setTimeout(async () => {
    searchingCustomers.value = true
    try {
      const p = await api.get<{ items: Customer[] }>(`/customers?q=${encodeURIComponent(query)}&page_size=8`)
      customerResults.value = p.items
    } finally {
      searchingCustomers.value = false
    }
  }, 250)
})
function pickCustomer(c: Customer) {
  pickedCustomer.value = c
  customerQuery.value = ''
  customerResults.value = []
  error.value = ''
}
// effectiveCustomerId is what save() actually submits: the prop when the
// dialog was opened for a known customer, else whichever one the picker above
// resolved to.
const effectiveCustomerId = computed(() => props.customerId || pickedCustomer.value?.id || '')

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
// A new follow-up defaults to the person creating it. "Задачи" opens on «Мои»,
// so an unassigned default would put every follow-up a manager schedules
// somewhere they do not look — the one place the feature must not lose it.
// Rescheduling keeps whoever it is already assigned to.
const assignee = ref(
  props.followup ? props.followup.assignee_user_id || UNASSIGNED : auth.user?.id || UNASSIGNED,
)
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

const title = computed(() => {
  if (props.followup) return t('crm.dialog.editTitle')
  if (needsCustomerPicker.value) return t('crm.dialog.newTaskTitle')
  return t('crm.dialog.title')
})

async function save() {
  if (needsCustomerPicker.value && !pickedCustomer.value) {
    error.value = t('crm.dialog.errors.customerRequired')
    return
  }
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
      customer_id: effectiveCustomerId.value,
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
        <div v-if="needsCustomerPicker">
          <span class="text-[13px] font-medium">{{ t('crm.dialog.customer') }}</span>
          <div v-if="pickedCustomer" class="mt-1.5 flex items-center gap-2 rounded-lg border border-border px-2.5 py-2">
            <Avatar class="w-7 h-7 shrink-0">
              <AvatarFallback class="text-[11px]" :class="colorFor(pickedCustomer.id)">{{ initials(pickedCustomer.display_name) }}</AvatarFallback>
            </Avatar>
            <span class="min-w-0 flex-1 truncate text-sm font-medium">{{ pickedCustomer.display_name || '—' }}</span>
            <button
              type="button"
              class="shrink-0 text-muted-foreground hover:text-foreground"
              :aria-label="t('crm.dialog.changeCustomer')"
              @click="pickedCustomer = null"
            >
              <X class="w-4 h-4" />
            </button>
          </div>
          <div v-else class="relative mt-1.5">
            <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input v-model="customerQuery" class="h-9 pl-9" :placeholder="t('crm.dialog.customerSearchPlaceholder')" />
            <div
              v-if="customerQuery.trim() && (customerResults.length || searchingCustomers)"
              class="absolute z-10 mt-1 max-h-56 w-full overflow-y-auto rounded-lg border border-border bg-popover shadow-pop"
            >
              <p v-if="searchingCustomers && !customerResults.length" class="px-2.5 py-2 text-[13px] text-muted-foreground">
                {{ t('crm.dialog.searchingCustomers') }}
              </p>
              <button
                v-for="c in customerResults"
                :key="c.id"
                type="button"
                class="flex w-full items-center gap-2 px-2.5 py-2 text-left text-sm hover:bg-muted"
                @click="pickCustomer(c)"
              >
                <Avatar class="w-6 h-6 shrink-0">
                  <AvatarFallback class="text-[10px]" :class="colorFor(c.id)">{{ initials(c.display_name) }}</AvatarFallback>
                </Avatar>
                <span class="truncate">{{ c.display_name || '—' }}</span>
              </button>
            </div>
          </div>
        </div>

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
