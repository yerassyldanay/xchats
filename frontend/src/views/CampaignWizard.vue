<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
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
import AccountSendingBudget from '@/components/AccountSendingBudget.vue'

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

// CAM-03: "Variables used: name, promo_code" is a fact about the template,
// not a preview of the actual message — the operator cannot tell how line
// breaks, spacing, or a variable's real value will look until a message
// has already gone out. Substituted with representative sample values
// (never a real recipient's data — nothing is loaded yet at this phase),
// so an unmapped custom variable falls back to a bracketed placeholder
// naming itself rather than rendering blank.
const SAMPLE_VARIABLE_VALUES: Record<string, string> = { name: 'Aigul', phone: '77011234567', code: 'SUMMER2026', promo_code: 'SUMMER2026' }
const showMessagePreview = ref(false)
const renderedMessagePreview = computed(() =>
  messageBody.value.replace(/\{\{\s*([A-Za-z0-9_]+)\s*\}\}/g, (_, v: string) => SAMPLE_VARIABLE_VALUES[v] ?? `[${v}]`),
)

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

// CAM-04: the reachability check used to be a manual prerequisite — paste
// or upload a list, then separately notice and click Check, or Create would
// just reject with a red error. A file selection is a single discrete
// action (check it immediately); typed/pasted text checks itself 400ms
// after the operator stops typing, so it never fires on every keystroke.
const PREVIEW_DEBOUNCE_MS = 400
let previewDebounceTimer: ReturnType<typeof setTimeout> | null = null
watch(pastedText, () => {
  if (previewDebounceTimer) clearTimeout(previewDebounceTimer)
  if (!pastedText.value.trim()) return
  previewDebounceTimer = setTimeout(() => void checkRecipients(), PREVIEW_DEBOUNCE_MS)
})
onBeforeUnmount(() => {
  if (previewDebounceTimer) clearTimeout(previewDebounceTimer)
})
function onFileChange(e: Event) {
  uploadedFile.value = (e.target as HTMLInputElement).files?.[0] ?? null
  if (uploadedFile.value) void checkRecipients()
}

// CAM-06: the placeholder's two example lines don't say whether a header
// row is required, which separators work, or how a country code should be
// written — all of it is auto-detected server-side (see ParseRecipients'
// own doc comment, backend/campaign/recipients.go), but the operator has
// no way to know that without reading the Go source.
const SAMPLE_CSV = 'phone,name,promo_code\n77011234567,Aigul,SUMMER2026\n77022222222,Bota,SUMMER2026\n'
function downloadSampleCsv() {
  const blob = new Blob([SAMPLE_CSV], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'recipients-sample.csv'
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

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
const scheduleAtInputEl = ref<HTMLInputElement | null>(null)
// CAM-10: a blank or already-past "later" date used to fall through
// silently — no schedule_at patch was ever sent, and the campaign landed
// on the detail page as an ordinary Draft with no explanation that
// scheduling was ignored.
const scheduleError = ref('')
watch([scheduleMode, scheduleAtLocal], () => {
  scheduleError.value = ''
})

// --- finish: save recipients + pace/schedule, then go to the campaign ----
const creating = ref(false)
const error = ref('')

async function finish() {
  error.value = ''
  scheduleError.value = ''

  if (scheduleMode.value === 'later') {
    const when = scheduleAtLocal.value ? new Date(scheduleAtLocal.value) : null
    if (!when || Number.isNaN(when.getTime())) {
      scheduleError.value = t('campaigns.wizard.errScheduleRequired')
      scheduleAtInputEl.value?.focus()
      return
    }
    if (when.getTime() <= Date.now()) {
      scheduleError.value = t('campaigns.wizard.errScheduleInPast')
      scheduleAtInputEl.value?.focus()
      return
    }
  }
  if (invalidWindow.value) {
    error.value = t('campaigns.limits.errInvalidWindow')
    return
  }

  creating.value = true
  // CAM-04: Create is no longer blocked on a manual Check first — an
  // unchecked or now-stale list is checked here, in the background, and
  // creation proceeds automatically once it comes back valid.
  if (!previewResult.value || previewStale.value) {
    if (!canCheckRecipients.value) {
      error.value = t('campaigns.wizard.errNoRecipients')
      creating.value = false
      return
    }
    await checkRecipients()
  }
  if (!previewResult.value || previewResult.value.valid === 0) {
    error.value = previewError.value || t('campaigns.wizard.errNoRecipients')
    creating.value = false
    return
  }

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
    // scheduleAtLocal is guaranteed present and in the future here — the
    // earlier guard above already returned otherwise.
    if (scheduleMode.value === 'later') {
      patch.schedule_at = new Date(scheduleAtLocal.value).toISOString()
    }
    if (Object.keys(patch).length > 0) {
      await campaigns.update(id, patch)
    }
    // CAM-07: landing on the detail page in plain Draft status with no
    // explanation left operators assuming "Create" had already started
    // sending. Tell the detail page to greet a just-created, launch-now
    // campaign with an explicit "click Start" prompt — never for one that
    // was deliberately scheduled, which starts itself.
    if (scheduleMode.value === 'now') {
      await router.push({ name: 'campaign-detail', params: { campaignId: id }, query: { created: '1' } })
    } else {
      await router.push({ name: 'campaign-detail', params: { campaignId: id } })
    }
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

    <!-- CAM-01: the wizard is two phases on one page with no back/forward
         navigation (see phase's own doc comment above) — this indicator
         orients the operator without implying either step is a separate
         screen they could navigate to directly. -->
    <div class="mt-3 flex items-center gap-2 text-xs font-medium" data-testid="wizard-step-indicator">
      <span
        class="flex items-center gap-1.5 rounded-full px-2.5 py-1"
        data-testid="wizard-step-details"
        :class="phase === 'details' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground'"
      >
        <span class="grid place-items-center w-4 h-4 rounded-full text-[10px]" :class="phase === 'details' ? 'bg-primary-foreground text-primary' : 'bg-muted'">1</span>
        {{ t('campaigns.wizard.stepDetails') }}
      </span>
      <span class="text-muted-foreground">→</span>
      <span
        class="flex items-center gap-1.5 rounded-full px-2.5 py-1"
        data-testid="wizard-step-recipients"
        :class="phase === 'recipients' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground'"
      >
        <span class="grid place-items-center w-4 h-4 rounded-full text-[10px]" :class="phase === 'recipients' ? 'bg-primary-foreground text-primary' : 'bg-muted'">2</span>
        {{ t('campaigns.wizard.stepRecipients') }}
      </span>
    </div>

    <section class="mt-6 space-y-4">
      <h2 class="text-sm font-semibold">{{ t('campaigns.wizard.detailsHeading') }}</h2>
      <div>
        <label class="text-xs font-medium text-muted-foreground">{{ t('campaigns.wizard.nameLabel') }}</label>
        <Input v-model="name" :disabled="phase === 'recipients'" :placeholder="t('campaigns.wizard.namePlaceholder')" class="mt-1.5" />
      </div>
      <div>
        <label class="text-xs font-medium text-muted-foreground">{{ t('campaigns.wizard.accountLabel') }}</label>
        <!-- With no connected channel the Select would open onto an empty
             list and Continue would fail on a requirement the operator has
             no way to satisfy from this page. Send them to Channels instead
             of leaving them at a dead end. -->
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
        <!-- CAM-05: the account's own live budget/rate-limit headroom was
             only ever visible AFTER creation, on the detail page — an
             operator could configure hundreds of recipients against an
             account that's already throttled or paused and only find out
             once it was too late to pick a different one. -->
        <AccountSendingBudget v-if="accountId && !noAccounts" :account-id="accountId" class="mt-2" />
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
        <!-- CAM-03: "Variables used: name, promo_code" is the template's own
             shape, not a preview of the actual message — line breaks,
             spacing, and a variable's real value were all invisible until
             a message had already gone out. -->
        <div v-if="messageBody.trim()" class="mt-1.5">
          <button type="button" class="text-xs font-medium text-primary hover:underline" data-testid="toggle-message-preview" @click="showMessagePreview = !showMessagePreview">
            {{ showMessagePreview ? t('campaigns.wizard.previewHide') : t('campaigns.wizard.previewShow') }}
          </button>
          <div v-if="showMessagePreview" class="mt-2 max-w-xs">
            <div class="rounded-2xl rounded-bl-sm border border-wa/20 bg-wa/10 px-3 py-2 text-sm whitespace-pre-wrap" data-testid="message-preview-bubble">
              {{ renderedMessagePreview }}
            </div>
            <p class="mt-1 text-[11px] text-muted-foreground">{{ t('campaigns.wizard.previewSampleHint') }}</p>
          </div>
        </div>
      </div>
      <p v-if="detailsError" class="flex items-center gap-1.5 text-sm text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0" /> {{ detailsError }}
      </p>
      <Button v-if="phase === 'details'" type="button" :disabled="continuing || noAccounts" @click="continueToRecipients">
        <LoaderCircle v-if="continuing" class="w-4 h-4 animate-spin" />
        {{ t('campaigns.wizard.continueButton') }}
      </Button>
    </section>

    <section v-if="phase === 'recipients'" class="mt-8 space-y-3">
      <h2 class="text-sm font-semibold">{{ t('campaigns.wizard.recipientsHeading') }}</h2>
      <p class="text-xs text-muted-foreground">{{ t('campaigns.wizard.recipientsHint') }}</p>
      <div>
        <label class="text-xs font-medium text-muted-foreground">{{ t('campaigns.wizard.pasteLabel') }}</label>
        <Textarea v-model="pastedText" :placeholder="t('campaigns.wizard.pastePlaceholder')" class="mt-1.5 min-h-[120px] font-mono text-xs" data-testid="paste-recipients" />
      </div>
      <div class="flex items-center gap-2">
        <label class="text-xs font-medium text-muted-foreground shrink-0">{{ t('campaigns.wizard.uploadLabel') }}</label>
        <input type="file" accept=".csv,.txt" class="text-xs" @change="onFileChange" />
      </div>
      <!-- CAM-06: the placeholder's two example lines don't say whether a
           header row is required, which separators work, or how a country
           code should be written — all of it auto-detected server-side,
           invisible from here without reading the Go source. -->
      <details class="rounded-md border border-border p-2.5 text-xs text-muted-foreground">
        <summary class="cursor-pointer font-medium text-foreground">{{ t('campaigns.wizard.formatHelpTitle') }}</summary>
        <ul class="mt-2 list-disc space-y-1 pl-4">
          <li>{{ t('campaigns.wizard.formatHelpHeader') }}</li>
          <li>{{ t('campaigns.wizard.formatHelpSeparator') }}</li>
          <li>{{ t('campaigns.wizard.formatHelpPhone') }}</li>
          <li>{{ t('campaigns.wizard.formatHelpColumns') }}</li>
        </ul>
        <button type="button" class="mt-2 font-medium text-primary hover:underline" data-testid="download-sample-csv" @click="downloadSampleCsv">
          {{ t('campaigns.wizard.downloadSample') }}
        </button>
      </details>
      <Button type="button" variant="outline" size="sm" :disabled="!canCheckRecipients || previewing" @click="checkRecipients">
        <LoaderCircle v-if="previewing" class="w-4 h-4 animate-spin" />
        {{ previewing ? t('campaigns.wizard.checking') : t('campaigns.wizard.checkReachability') }}
      </Button>
      <p v-if="previewError" class="flex items-center gap-1.5 text-sm text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0" /> {{ previewError }}
      </p>
      <CampaignRecipientPreviewTable v-if="previewResult" :result="previewResult" :message-variables="uniqueVariables" />
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
            ref="scheduleAtInputEl"
            v-model="scheduleAtLocal"
            type="datetime-local"
            class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
            :class="scheduleError ? 'border-destructive' : 'border-border'"
          />
        </div>
        <!-- CAM-10: required only once "later" is actually selected, and
             cleared the moment either mode or the date value changes so a
             fixed error doesn't survive a correction. -->
        <p v-if="scheduleMode === 'later' && scheduleError" class="mt-1.5 flex items-center gap-1.5 text-xs text-destructive" data-testid="schedule-error">
          <CircleAlert class="w-3.5 h-3.5 shrink-0" /> {{ scheduleError }}
        </p>
      </div>
    </section>

    <p v-if="error" class="mt-6 flex items-center gap-1.5 text-sm text-destructive">
      <CircleAlert class="w-4 h-4 shrink-0" /> {{ error }}
    </p>

    <div v-if="phase === 'recipients'" class="mt-6 flex items-center gap-2 pb-10">
      <Button type="button" :disabled="creating" @click="finish">
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
