<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, onBeforeRouteLeave, useRouter } from 'vue-router'
import { CircleAlert, LoaderCircle, Megaphone, Plus, Trash2 } from 'lucide-vue-next'
import { useCampaigns } from '@/stores/campaigns'
import { useAccounts } from '@/stores/accounts'
import { ApiError } from '@/api/client'
import {
  currentUtcOffsetMinutes,
  formatUtcOffset,
  localToUtc,
  minutesToHHMM,
  hhmmToMinutes,
  endMinutesFromInput,
  endInputFromMinutes,
} from '@/lib/schedule'
import { fingerprintOf } from '@/lib/recipientFingerprint'
import type { ScheduleWindow, CampaignRecipientPreviewResult } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import CampaignRecipientPreviewTable from '@/components/CampaignRecipientPreviewTable.vue'
import ConfirmDeleteDialog from '@/components/kb/forms/ConfirmDeleteDialog.vue'

const { t } = useI18n()
const router = useRouter()
const campaigns = useCampaigns()
const accounts = useAccounts()

onMounted(() => {
  if (accounts.accounts.length === 0) void accounts.load()
})

// noAccounts gates the account picker on a settled empty list — never while
// the first load is still in flight, which would flash the "connect a
// channel" callout at an operator who has one.
const noAccounts = computed(() => !accounts.loading && accounts.accounts.length === 0)

// --- details -----------------------------------------------------------
// The wizard is two phases on one page, not a true multi-step flow with
// back/forward navigation: 'details' collects name/account/message and
// creates the campaign as a draft; 'recipients' (now that a real campaign
// id exists) handles the recipient list and pace. This split exists
// because POST /campaigns/:id/preview resolves reachability against a
// SPECIFIC campaign's own channel/account (backend/internal/httpapi/
// campaigns.go's parseCampaignRecipients) — there is no "preview before a
// campaign exists" endpoint, so the draft has to be created first.
const phase = ref<'details' | 'recipients'>('details')
const pendingCampaignId = ref('')

const name = ref('')
const accountId = ref('')
const messageBody = ref('')
const variablesDetected = computed(() => [...messageBody.value.matchAll(/\{\{\s*([A-Za-z0-9_]+)\s*\}\}/g)].map((m) => m[1]))
const uniqueVariables = computed(() => [...new Set(variablesDetected.value)])

// CAM-02: the parser only ever recognizes double-brace {{var}} — a single-
// brace placeholder or hint would teach the wrong syntax outright. Quick-
// insert chips below the textarea (native <textarea>, not the <Textarea>
// wrapper, so a plain HTMLTextAreaElement ref reports selectionStart/End)
// insert the correct token at the cursor instead of requiring exact typing.
const messageTextareaEl = ref<HTMLTextAreaElement | null>(null)
const QUICK_VARIABLES = ['name', 'phone'] as const
const showCustomVariable = ref(false)
const customVariableName = ref('')

function insertVariable(varName: string) {
  const token = `{{${varName}}}`
  const el = messageTextareaEl.value
  if (!el) {
    messageBody.value += token
    return
  }
  const start = el.selectionStart ?? messageBody.value.length
  const end = el.selectionEnd ?? messageBody.value.length
  messageBody.value = messageBody.value.slice(0, start) + token + messageBody.value.slice(end)
  const caret = start + token.length
  void nextTick(() => {
    el.focus()
    el.setSelectionRange(caret, caret)
  })
}
function chipLabel(varName: string): string {
  return '+ {{' + varName + '}}'
}
function confirmCustomVariable() {
  const varName = customVariableName.value.trim().replace(/\s+/g, '_')
  if (!varName) return
  insertVariable(varName)
  customVariableName.value = ''
  showCustomVariable.value = false
}

const detailsError = ref('')
const continuing = ref(false)
async function continueToRecipients() {
  if (!name.value.trim()) {
    detailsError.value = t('campaigns.wizard.errNameRequired')
    return
  }
  if (!messageBody.value.trim()) {
    detailsError.value = t('campaigns.wizard.errMessageRequired')
    return
  }
  if (!accountId.value) {
    detailsError.value = t('campaigns.wizard.errAccountRequired')
    return
  }
  detailsError.value = ''
  continuing.value = true
  try {
    const c = await campaigns.create({ name: name.value.trim(), account_id: accountId.value, message_body: messageBody.value })
    pendingCampaignId.value = c.id
    phase.value = 'recipients'
  } catch (e) {
    detailsError.value = e instanceof ApiError ? e.message : t('campaigns.wizard.errCreateFailed')
  } finally {
    continuing.value = false
  }
}

// cancelPending discards the draft continueToRecipients created. Without
// this, backing out of the wizard's second phase would strand an empty
// campaign in the list with no recipients and no way to remove it. A
// failed delete is deliberately swallowed: the operator asked to leave, and
// the campaign is still reachable (and deletable) from the list.
//
// finished marks that pendingCampaignId no longer needs the CAM-12 guards
// below watching over it — either the wizard completed (finish()) or the
// operator explicitly discarded it (cancelPending, or the leave-confirm
// dialog's own Discard) — so navigating away from here is never again
// silently losing track of unreviewed work.
const cancelling = ref(false)
const finished = ref(false)
async function cancelPending() {
  finished.value = true
  const id = pendingCampaignId.value
  cancelling.value = true
  try {
    if (id) await campaigns.remove(id)
  } catch {
    // fall through to the list either way
  } finally {
    cancelling.value = false
    await router.push({ name: 'campaigns' })
  }
}

// --- CAM-12: warn before abandoning an orphan draft campaign -------------
// continueToRecipients() already created a real, empty campaign server-side
// (POST /campaigns/:id/preview needs a real campaign to check reachability
// against — there is no "preview before a campaign exists" endpoint). Every
// exit from phase 2 other than Cancel/finish used to bypass that cleanup
// entirely: browser back, a nav-rail click, refresh, or tab close all left
// the empty campaign sitting in the list with nothing pointing back at it.
function hasUnsavedDraft(): boolean {
  return phase.value === 'recipients' && !!pendingCampaignId.value && !finished.value
}

// beforeunload cannot show custom UI — the browser's own native prompt is
// the only option the platform allows here, unlike every other confirm in
// this app.
function handleBeforeUnload(e: BeforeUnloadEvent) {
  if (!hasUnsavedDraft()) return
  e.preventDefault()
  e.returnValue = ''
}
onMounted(() => window.addEventListener('beforeunload', handleBeforeUnload))
onBeforeUnmount(() => window.removeEventListener('beforeunload', handleBeforeUnload))

// In-app navigation (nav rail, browser back/forward within the SPA) DOES
// support a real styled dialog — onBeforeRouteLeave can await one before
// deciding whether to let the navigation proceed.
const leaveConfirmOpen = ref(false)
let resolveLeave: ((proceed: boolean) => void) | null = null
onBeforeRouteLeave(() => {
  if (!hasUnsavedDraft()) return true
  leaveConfirmOpen.value = true
  return new Promise<boolean>((resolve) => {
    resolveLeave = resolve
  })
})
function stayOnWizard() {
  leaveConfirmOpen.value = false
  resolveLeave?.(false)
  resolveLeave = null
}
async function discardAndLeave() {
  finished.value = true // stop hasUnsavedDraft from re-triggering while the delete/navigation below settles
  leaveConfirmOpen.value = false
  try {
    if (pendingCampaignId.value) await campaigns.remove(pendingCampaignId.value)
  } catch {
    // The campaign stays reachable and deletable from the list either way —
    // cleanup failing here must never hide it or block the navigation the
    // operator already confirmed.
  }
  resolveLeave?.(true)
  resolveLeave = null
}

// --- recipients ----------------------------------------------------------
const pastedText = ref('')
const uploadedFile = ref<File | null>(null)
function onFileChange(e: Event) {
  uploadedFile.value = (e.target as HTMLInputElement).files?.[0] ?? null
}
const previewResult = ref<CampaignRecipientPreviewResult | null>(null)
const previewing = ref(false)
const previewError = ref('')
// CAM-09: the fingerprint of whatever input the LAST successful preview
// actually checked. previewStale compares it against the CURRENT input on
// every render, so editing pasted text or picking a different/cleared file
// invalidates the preview immediately — no explicit change handler needed,
// it falls out of being a computed().
const previewedFingerprint = ref('')
const previewStale = computed(() => fingerprintOf(pastedText.value, uploadedFile.value) !== previewedFingerprint.value)
async function checkRecipients() {
  previewing.value = true
  previewError.value = ''
  const fp = fingerprintOf(pastedText.value, uploadedFile.value)
  try {
    previewResult.value = await campaigns.preview(pendingCampaignId.value, { text: pastedText.value, file: uploadedFile.value ?? undefined })
    previewedFingerprint.value = fp
  } catch (e) {
    previewError.value = e instanceof ApiError ? e.message : t('campaigns.wizard.errNoRecipients')
    previewResult.value = null
  } finally {
    previewing.value = false
  }
}
const canCheckRecipients = computed(() => pastedText.value.trim() !== '' || !!uploadedFile.value)

// --- pace + schedule -----------------------------------------------------
const paceMode = ref<'inherit' | 'custom'>('inherit')
const minInterval = ref(90)
const jitter = ref(30)

const offsetMinutes = currentUtcOffsetMinutes()
const offsetLabel = formatUtcOffset(offsetMinutes)
const localWindows = ref<ScheduleWindow[]>([])
function addWindow() {
  localWindows.value = [...localWindows.value, { weekday: 1, start_minute: 9 * 60, end_minute: 18 * 60 }]
}
function removeWindow(i: number) {
  localWindows.value = localWindows.value.filter((_, idx) => idx !== i)
}
const WEEKDAYS = [0, 1, 2, 3, 4, 5, 6]
const WEEKDAY_KEYS = ['sun', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat']
const invalidWindow = computed(() => localWindows.value.some((w) => w.start_minute === w.end_minute))

const scheduleMode = ref<'now' | 'later'>('now')
const scheduleAtLocal = ref('')

// --- finish: save recipients + pace/schedule, then go to the campaign ----
const creating = ref(false)
const error = ref('')

async function finish() {
  if (previewStale.value) {
    error.value = t('campaigns.wizard.errPreviewStale')
    return
  }
  if (!previewResult.value || previewResult.value.valid === 0) {
    error.value = t('campaigns.wizard.errNoRecipients')
    return
  }
  if (invalidWindow.value) {
    error.value = t('campaigns.limits.errInvalidWindow')
    return
  }
  error.value = ''
  creating.value = true
  const id = pendingCampaignId.value
  try {
    await campaigns.replaceRecipients(id, { text: pastedText.value, file: uploadedFile.value ?? undefined })
    // Recipients are committed server-side from here on — this is no longer
    // an empty orphan draft even if the pace/schedule PATCH below fails, so
    // the CAM-12 leave guards have nothing left to protect.
    finished.value = true
  } catch (e) {
    // The campaign already exists — send the operator there to retry the
    // recipient save rather than getting stuck on this page.
    error.value = e instanceof ApiError ? e.message : t('campaigns.wizard.errSaveRecipientsFailed')
    creating.value = false
    await router.push({ name: 'campaign-detail', params: { campaignId: id } })
    return
  }
  try {
    const patch: Record<string, unknown> = {}
    if (paceMode.value === 'custom') {
      patch.min_interval_seconds = minInterval.value
      patch.jitter_seconds = jitter.value
    }
    if (localWindows.value.length > 0) {
      patch.windows = localToUtc(localWindows.value, offsetMinutes)
    }
    if (scheduleMode.value === 'later' && scheduleAtLocal.value) {
      patch.schedule_at = new Date(scheduleAtLocal.value).toISOString()
    }
    if (Object.keys(patch).length > 0) {
      await campaigns.update(id, patch)
    }
    await router.push({ name: 'campaign-detail', params: { campaignId: id } })
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : t('campaigns.wizard.errCreateFailed')
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <div class="flex-1 overflow-y-auto px-8 py-6 max-w-2xl">
    <h1 class="text-xl font-semibold flex items-center gap-2"><Megaphone class="w-5 h-5" /> {{ t('campaigns.wizard.title') }}</h1>

    <section class="mt-6 space-y-4">
      <h2 class="text-sm font-semibold">{{ t('campaigns.wizard.detailsHeading') }}</h2>
      <div>
        <label class="text-xs font-medium text-muted-foreground">{{ t('campaigns.wizard.nameLabel') }}</label>
        <Input v-model="name" :disabled="phase === 'recipients'" :placeholder="t('campaigns.wizard.namePlaceholder')" class="mt-1.5" />
      </div>
      <div>
        <label class="text-xs font-medium text-muted-foreground">{{ t('campaigns.wizard.accountLabel') }}</label>
        <!-- With no connected channel the Select would open onto an empty
             list and "Сохранить" would fail on a requirement the operator
             has no way to satisfy from this page. Send them to Channels
             instead of leaving them at a dead end. -->
        <div v-if="noAccounts" class="mt-1.5 rounded-md border border-border bg-muted/40 p-3">
          <p class="text-sm">{{ t('campaigns.wizard.noAccounts') }}</p>
          <RouterLink :to="{ name: 'accounts' }" class="mt-2 inline-block">
            <Button type="button" variant="outline" size="sm">{{ t('campaigns.wizard.connectAccount') }}</Button>
          </RouterLink>
        </div>
        <Select v-else v-model="accountId" :disabled="phase === 'recipients'">
          <SelectTrigger class="mt-1.5">
            <SelectValue :placeholder="t('campaigns.wizard.accountPlaceholder')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="a in accounts.accounts" :key="a.id" :value="a.id">{{ a.display_name || a.external_handle }}</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div>
        <label class="text-xs font-medium text-muted-foreground">{{ t('campaigns.wizard.messageLabel') }}</label>
        <p class="mt-0.5 text-xs text-muted-foreground">{{ t('campaigns.wizard.messageHint') }}</p>
        <textarea
          ref="messageTextareaEl"
          v-model="messageBody"
          :disabled="phase === 'recipients'"
          :placeholder="t('campaigns.wizard.messagePlaceholder')"
          class="mt-1.5 min-h-[100px] flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
        />
        <div v-if="phase === 'details'" class="mt-1.5 flex flex-wrap items-center gap-1.5">
          <button
            v-for="v in QUICK_VARIABLES"
            :key="v"
            type="button"
            class="rounded-full border border-border bg-muted px-2.5 py-1 text-xs font-mono hover:bg-accent"
            :data-testid="`insert-var-${v}`"
            @click="insertVariable(v)"
          >
            {{ chipLabel(v) }}
          </button>
          <Input
            v-if="showCustomVariable"
            v-model="customVariableName"
            :placeholder="t('campaigns.wizard.customVariablePlaceholder')"
            class="h-7 w-36 text-xs"
            data-testid="custom-var-input"
            @keydown.enter.prevent="confirmCustomVariable"
            @blur="!customVariableName.trim() && (showCustomVariable = false)"
          />
          <button
            v-else
            type="button"
            class="rounded-full border border-dashed border-border px-2.5 py-1 text-xs text-muted-foreground hover:bg-accent"
            data-testid="add-custom-var"
            @click="showCustomVariable = true"
          >
            + {{ t('campaigns.wizard.customVariable') }}
          </button>
        </div>
        <p v-if="uniqueVariables.length" class="mt-1 text-xs text-muted-foreground">
          {{ t('campaigns.wizard.variablesDetected', { variables: uniqueVariables.join(', ') }) }}
        </p>
      </div>
      <p v-if="detailsError" class="flex items-center gap-1.5 text-sm text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0" /> {{ detailsError }}
      </p>
      <Button v-if="phase === 'details'" type="button" :disabled="continuing || noAccounts" @click="continueToRecipients">
        <LoaderCircle v-if="continuing" class="w-4 h-4 animate-spin" />
        {{ t('campaigns.actions.save') }}
      </Button>
    </section>

    <section v-if="phase === 'recipients'" class="mt-8 space-y-3">
      <h2 class="text-sm font-semibold">{{ t('campaigns.wizard.recipientsHeading') }}</h2>
      <p class="text-xs text-muted-foreground">{{ t('campaigns.wizard.recipientsHint') }}</p>
      <div>
        <label class="text-xs font-medium text-muted-foreground">{{ t('campaigns.wizard.pasteLabel') }}</label>
        <Textarea v-model="pastedText" :placeholder="t('campaigns.wizard.pastePlaceholder')" class="mt-1.5 min-h-[120px] font-mono text-xs" />
      </div>
      <div class="flex items-center gap-2">
        <label class="text-xs font-medium text-muted-foreground shrink-0">{{ t('campaigns.wizard.uploadLabel') }}</label>
        <input type="file" accept=".csv,.txt" class="text-xs" @change="onFileChange" />
      </div>
      <Button type="button" variant="outline" size="sm" :disabled="!canCheckRecipients || previewing" @click="checkRecipients">
        <LoaderCircle v-if="previewing" class="w-4 h-4 animate-spin" />
        {{ previewing ? t('campaigns.wizard.checking') : t('campaigns.wizard.checkReachability') }}
      </Button>
      <p v-if="previewError" class="flex items-center gap-1.5 text-sm text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0" /> {{ previewError }}
      </p>
      <CampaignRecipientPreviewTable v-if="previewResult" :result="previewResult" />
      <p v-else class="text-xs text-muted-foreground">{{ t('campaigns.wizard.previewEmpty') }}</p>
      <p v-if="previewResult && previewStale" class="flex items-center gap-1.5 text-xs text-amber-600" data-testid="preview-stale-notice">
        <CircleAlert class="w-3.5 h-3.5 shrink-0" /> {{ t('campaigns.wizard.previewStaleNotice') }}
      </p>
    </section>

    <section v-if="phase === 'recipients'" class="mt-8 space-y-3">
      <h2 class="text-sm font-semibold">{{ t('campaigns.wizard.paceHeading') }}</h2>
      <div class="flex items-center gap-2">
        <Button type="button" size="sm" :variant="paceMode === 'inherit' ? 'default' : 'outline'" @click="paceMode = 'inherit'">
          {{ t('campaigns.wizard.paceInherit') }}
        </Button>
        <Button type="button" size="sm" :variant="paceMode === 'custom' ? 'default' : 'outline'" @click="paceMode = 'custom'">
          {{ t('campaigns.wizard.paceCustom') }}
        </Button>
      </div>
      <div v-if="paceMode === 'custom'" class="flex items-center gap-4">
        <div class="flex items-center gap-1.5">
          <span class="text-xs text-muted-foreground">{{ t('campaigns.wizard.minIntervalLabel') }}</span>
          <Input v-model.number="minInterval" type="number" min="1" class="w-20 h-9" />
          <span class="text-xs text-muted-foreground">{{ t('campaigns.wizard.secondsUnit') }}</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="text-xs text-muted-foreground">{{ t('campaigns.wizard.jitterLabel') }}</span>
          <Input v-model.number="jitter" type="number" min="0" class="w-20 h-9" />
          <span class="text-xs text-muted-foreground">{{ t('campaigns.wizard.secondsUnit') }}</span>
        </div>
      </div>

      <div>
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-muted-foreground">{{ t('campaigns.limits.windowsHeading') }}</span>
          <span class="text-[11px] text-muted-foreground">{{ t('campaigns.limits.scheduleTimezoneHint', { offset: offsetLabel }) }}</span>
        </div>
        <div class="mt-1.5 space-y-2">
          <div v-for="(w, i) in localWindows" :key="i" class="flex items-center gap-2 rounded-md border border-border p-2">
            <select
              v-model.number="w.weekday"
              class="h-9 w-28 shrink-0 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
            >
              <option v-for="d in WEEKDAYS" :key="d" :value="d">{{ t(`campaigns.limits.weekday.${WEEKDAY_KEYS[d]}`) }}</option>
            </select>
            <input
              type="time"
              :value="minutesToHHMM(w.start_minute)"
              class="h-9 w-[105px] shrink-0 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
              @change="w.start_minute = hhmmToMinutes(($event.target as HTMLInputElement).value)"
            />
            <span class="text-xs text-muted-foreground shrink-0">{{ t('campaigns.limits.rangeTo') }}</span>
            <input
              type="time"
              :value="endInputFromMinutes(w.end_minute)"
              class="h-9 w-[105px] shrink-0 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
              @change="w.end_minute = endMinutesFromInput(($event.target as HTMLInputElement).value)"
            />
            <Button type="button" variant="ghost" size="icon" class="w-8 h-8 ml-auto shrink-0 text-destructive hover:bg-destructive/10" :title="t('campaigns.limits.removeWindow')" @click="removeWindow(i)">
              <Trash2 class="w-4 h-4" />
            </Button>
          </div>
          <Button type="button" variant="outline" size="sm" @click="addWindow">
            <Plus class="w-4 h-4" /> {{ t('campaigns.limits.addWindow') }}
          </Button>
        </div>
        <p class="mt-2 text-[11px] text-muted-foreground">{{ t('campaigns.limits.overnightHint') }}</p>
      </div>

      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('campaigns.wizard.scheduleLabel') }}</span>
        <div class="mt-1.5 flex items-center gap-2">
          <Button type="button" size="sm" :variant="scheduleMode === 'now' ? 'default' : 'outline'" @click="scheduleMode = 'now'">
            {{ t('campaigns.wizard.scheduleNow') }}
          </Button>
          <Button type="button" size="sm" :variant="scheduleMode === 'later' ? 'default' : 'outline'" @click="scheduleMode = 'later'">
            {{ t('campaigns.wizard.scheduleLater') }}
          </Button>
          <input
            v-if="scheduleMode === 'later'"
            v-model="scheduleAtLocal"
            type="datetime-local"
            class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
          />
        </div>
      </div>
    </section>

    <p v-if="error" class="mt-6 flex items-center gap-1.5 text-sm text-destructive">
      <CircleAlert class="w-4 h-4 shrink-0" /> {{ error }}
    </p>

    <div v-if="phase === 'recipients'" class="mt-6 flex items-center gap-2 pb-10">
      <Button type="button" :disabled="creating || previewStale" @click="finish">
        <LoaderCircle v-if="creating" class="w-4 h-4 animate-spin" />
        {{ creating ? t('campaigns.wizard.creating') : t('campaigns.wizard.create') }}
      </Button>
      <Button type="button" variant="ghost" :disabled="cancelling" @click="cancelPending">
        {{ t('campaigns.actions.cancel') }}
      </Button>
    </div>
    <div v-else class="mt-6 pb-10">
      <RouterLink :to="{ name: 'campaigns' }">
        <Button type="button" variant="ghost">{{ t('campaigns.actions.cancel') }}</Button>
      </RouterLink>
    </div>

    <ConfirmDeleteDialog
      :open="leaveConfirmOpen"
      title-key="campaigns.wizard.leaveConfirm.title"
      body-key="campaigns.wizard.leaveConfirm.body"
      confirm-key="campaigns.wizard.leaveConfirm.accept"
      @update:open="(v) => !v && stayOnWizard()"
      @confirm="discardAndLeave"
    />
  </div>
</template>
