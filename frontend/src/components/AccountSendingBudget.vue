<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, Gauge, LoaderCircle } from 'lucide-vue-next'
import { useCampaigns } from '@/stores/campaigns'
import type { SendingBudget } from '@/types'

// AccountSendingBudget is the shared LIVE budget widget — the same
// GET /accounts/:id/sending-budget read the Accounts page, the campaign
// wizard, and a campaign's own detail page all render from, so the number
// can never disagree between them (backend/internal/store/campaigns.go's
// own SendingBudget doc comment). Polls rather than listening for a
// dedicated SSE event: the budget shifts continuously (a rolling window's
// own edge sliding past, not just discrete send events), so periodic
// re-reads are simpler and just as accurate as trying to model that live.
const props = defineProps<{ accountId: string }>()
const { t, locale } = useI18n()
const campaigns = useCampaigns()

const budget = ref<SendingBudget | null>(null)
const error = ref('')
let timer: ReturnType<typeof setInterval> | null = null

async function load() {
  try {
    budget.value = await campaigns.fetchSendingBudget(props.accountId)
    error.value = ''
  } catch {
    error.value = t('campaigns.budget.errLoadFailed')
  }
}

const POLL_MS = 15000
function start() {
  void load()
  timer = setInterval(load, POLL_MS)
}
function stop() {
  if (timer) clearInterval(timer)
  timer = null
}

onMounted(start)
onUnmounted(stop)
watch(
  () => props.accountId,
  () => {
    budget.value = null
    stop()
    start()
  },
)

function formatWhen(iso: string): string {
  return new Date(iso).toLocaleString(locale.value === 'en' ? 'en-US' : 'ru-RU', {
    day: 'numeric',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  })
}
function formatWindow(seconds: number): string {
  if (seconds % 3600 === 0) return `${seconds / 3600}h`
  if (seconds % 60 === 0) return `${seconds / 60}m`
  return `${seconds}s`
}
</script>

<template>
  <div class="rounded-lg border border-border p-3">
    <div class="flex items-center gap-2 text-xs font-medium text-muted-foreground">
      <Gauge class="w-3.5 h-3.5" />
      {{ t('campaigns.budget.title') }}
      <LoaderCircle v-if="!budget && !error" class="w-3.5 h-3.5 animate-spin ml-auto" />
    </div>

    <p v-if="error" class="mt-2 flex items-center gap-1.5 text-xs text-destructive">
      <CircleAlert class="w-3.5 h-3.5 shrink-0" /> {{ error }}
    </p>

    <template v-else-if="budget">
      <div class="mt-2 flex items-center gap-1.5 text-sm">
        <span class="w-2 h-2 rounded-full shrink-0" :class="budget.paused ? 'bg-muted-foreground' : budget.allowed ? 'bg-wa' : 'bg-amber-500'" />
        <span class="font-medium">
          {{ budget.paused ? t('campaigns.budget.paused') : budget.allowed ? t('campaigns.budget.allowed') : t('campaigns.budget.throttled') }}
        </span>
      </div>
      <p v-if="!budget.paused && !budget.allowed" class="mt-0.5 text-xs text-muted-foreground">
        {{ t('campaigns.budget.nextSendAt', { when: formatWhen(budget.next_send_at) }) }}
      </p>

      <ul v-if="budget.tiers.length" class="mt-2 space-y-1">
        <li v-for="tier in budget.tiers" :key="tier.window_seconds" class="flex items-center gap-2 text-xs">
          <span class="text-muted-foreground w-16 shrink-0">{{ formatWindow(tier.window_seconds) }}</span>
          <span class="flex-1 h-1.5 rounded-full bg-muted overflow-hidden">
            <span
              class="block h-full rounded-full bg-primary"
              :style="{ width: Math.min(100, ((tier.used ?? 0) / tier.max_sends) * 100) + '%' }"
            />
          </span>
          <span class="text-muted-foreground w-14 shrink-0 text-right">{{ t('campaigns.budget.used', { used: tier.used ?? 0, max: tier.max_sends }) }}</span>
        </li>
      </ul>
    </template>
  </div>
</template>
