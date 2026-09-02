<script setup lang="ts">
// CAM-15/17: the wizard is now a real 3-step, back-navigable flow — Audience
// (Who) -> Message (What) -> Schedule (When & Launch) — instead of the old
// two one-way "phases" that made the operator write a message before they
// even knew who they were sending it to. The campaign row is created as
// soon as a name+account exist (see ensureCampaign below), with a throwaway
// placeholder message_body: POST /campaigns/:id/preview has no "preview
// before a campaign exists" counterpart (backend/internal/httpapi/
// campaigns.go), so SOME real campaign id is a hard prerequisite for the
// audience step's own reachability counters. The placeholder is never
// visible to anyone — Step 2 always overwrites message_body with real
// content before Step 3 is reachable, and the CAM-12 leave-guard below
// deletes the whole row if the operator abandons the wizard first.
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, onBeforeRouteLeave, useRouter } from 'vue-router'
import { CircleAlert, FlaskConical, LoaderCircle, Megaphone, Plus, Save, Trash2 } from 'lucide-vue-next'
import { useCampaigns } from '@/stores/campaigns'
import { useAccounts } from '@/stores/accounts'
import { useCampaignTemplates } from '@/stores/campaignTemplates'
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
import { formatElapsed } from '@/lib/format'
import { fingerprintOf } from '@/lib/recipientFingerprint'
import type { ScheduleWindow, CampaignRecipientPreviewResult, CampaignTemplate, SendingBudget } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import CampaignRecipientPreviewTable from '@/components/CampaignRecipientPreviewTable.vue'
import ConfirmDeleteDialog from '@/components/kb/forms/ConfirmDeleteDialog.vue'
import AccountSendingBudget from '@/components/AccountSendingBudget.vue'
import CampaignTemplateFormDialog from '@/components/CampaignTemplateFormDialog.vue'

const { t } = useI18n()
const router = useRouter()
const campaigns = useCampaigns()
const accounts = useAccounts()
const templates = useCampaignTemplates()

onMounted(() => {
  if (accounts.accounts.length === 0) void accounts.load()
  void loadTemplateOptions()
})

const noAccounts = computed(() => !accounts.loading && accounts.accounts.length === 0)
// CAM-16: the Simulator account (always present as of GetOrCreateSimulatorAccount
// being called from handleListAccounts) is pinned first so operators notice
// the always-safe test option before any live channel.
const sortedAccounts = computed(() =>
  [...accounts.accounts].sort((a, b) => Number(b.channel === 'simulator') - Number(a.channel === 'simulator')),
)

const step = ref<'audience' | 'message' | 'schedule'>('audience')
const pendingCampaignId = ref('')
const finished = ref(false)
const DRAFT_MESSAGE_PLACEHOLDER = '(draft)'

const name = ref('')
const accountId = ref('')
const isSimulatorSelected = computed(() => accounts.accounts.find((a) => a.id === accountId.value)?.channel === 'simulator')

// ensureCampaign creates the draft campaign the moment name+account are both
// present, or keeps an already-created one in sync with either field — idempotently
// re-callable (from checkRecipients' own debounce and from continueToMessage) so a
// retry after a transient failure never creates a duplicate campaign. The sync branch
// only PATCHes when name/account actually changed since the last successful call —
// otherwise every debounced recipient check would re-send an identical no-op patch.
const audienceError = ref('')
let syncedName = ''
let syncedAccountId = ''
async function ensureCampaign(): Promise<string | null> {
  if (!name.value.trim() || !accountId.value) return null
  try {
    if (!pendingCampaignId.value) {
      const c = await campaigns.create({ name: name.value.trim(), account_id: accountId.value, message_body: DRAFT_MESSAGE_PLACEHOLDER })
      pendingCampaignId.value = c.id
      syncedName = name.value.trim()
      syncedAccountId = accountId.value
    } else if (syncedName !== name.value.trim() || syncedAccountId !== accountId.value) {
      await campaigns.update(pendingCampaignId.value, { name: name.value.trim(), account_id: accountId.value })
      syncedName = name.value.trim()
      syncedAccountId = accountId.value
    }
  } catch (e) {
    audienceError.value = e instanceof ApiError ? e.message : t('campaigns.wizard.errCreateFailed')
    return null
  }
  return pendingCampaignId.value
}

// cancelPending discards the draft ensureCampaign created. finished marks
// that pendingCampaignId no longer needs the CAM-12 guards below watching
// over it — either the wizard completed (launchCampaign/saveAsDraft) or the
// operator explicitly discarded it (cancelPending, or the leave-confirm
// dialog's own Discard).
const cancelling = ref(false)
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
// Unlike the old two-phase wizard, a real (placeholder-message) campaign
// row can now exist from partway through Step 1 onward — this guard starts
// protecting it from that same moment, not just once Step 2 is reached.
function hasUnsavedDraft(): boolean {
  return !!pendingCampaignId.value && !finished.value
}

function handleBeforeUnload(e: BeforeUnloadEvent) {
  if (!hasUnsavedDraft()) return
  e.preventDefault()
  e.returnValue = ''
}
onMounted(() => window.addEventListener('beforeunload', handleBeforeUnload))
onBeforeUnmount(() => window.removeEventListener('beforeunload', handleBeforeUnload))

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
  finished.value = true
  leaveConfirmOpen.value = false
  try {
    if (pendingCampaignId.value) await campaigns.remove(pendingCampaignId.value)
  } catch {
    // The campaign stays reachable and deletable from the list either way.
  }
  resolveLeave?.(true)
  resolveLeave = null
}

// --- Step 1: audience ------------------------------------------------------
const pastedText = ref('')
const uploadedFile = ref<File | null>(null)
const previewResult = ref<CampaignRecipientPreviewResult | null>(null)
const previewing = ref(false)
const previewError = ref('')
// CAM-09: the fingerprint of whatever input the LAST successful preview
// actually checked, compared against the CURRENT input on every render.
const previewedFingerprint = ref('')
const previewStale = computed(() => fingerprintOf(pastedText.value, uploadedFile.value) !== previewedFingerprint.value)
// recipientsCommitted tracks whether the CURRENT (non-stale) preview has
// already been persisted via replaceRecipients — reset the instant the
// preview goes stale, so a later revisit of this step never skips a needed
// re-save just because SOME preview was committed at some earlier point.
const recipientsCommitted = ref(false)
watch(previewStale, (stale) => {
  if (stale) recipientsCommitted.value = false
})

async function checkRecipients() {
  const id = await ensureCampaign()
  if (!id) return
  previewing.value = true
  previewError.value = ''
  const fp = fingerprintOf(pastedText.value, uploadedFile.value)
  try {
    previewResult.value = await campaigns.preview(id, { text: pastedText.value, file: uploadedFile.value ?? undefined })
    previewedFingerprint.value = fp
  } catch (e) {
    previewError.value = e instanceof ApiError ? e.message : t('campaigns.wizard.errNoRecipients')
    previewResult.value = null
  } finally {
    previewing.value = false
  }
}
const audienceReady = computed(() => !!name.value.trim() && !!accountId.value)
const canCheckRecipients = computed(() => audienceReady.value && (pastedText.value.trim() !== '' || !!uploadedFile.value))

const PREVIEW_DEBOUNCE_MS = 400
let previewDebounceTimer: ReturnType<typeof setTimeout> | null = null
watch(pastedText, () => {
  if (previewDebounceTimer) clearTimeout(previewDebounceTimer)
  if (!pastedText.value.trim() || !audienceReady.value) return
  previewDebounceTimer = setTimeout(() => void checkRecipients(), PREVIEW_DEBOUNCE_MS)
})
onBeforeUnmount(() => {
  if (previewDebounceTimer) clearTimeout(previewDebounceTimer)
})
function onFileChange(e: Event) {
  uploadedFile.value = (e.target as HTMLInputElement).files?.[0] ?? null
  if (uploadedFile.value && audienceReady.value) void checkRecipients()
}

// CAM-06: the placeholder's two example lines don't say whether a header
// row is required, which separators work, or how a country code should be
// written — all of it is auto-detected server-side.
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

async function continueToMessage() {
  audienceError.value = ''
  if (!name.value.trim()) {
    audienceError.value = t('campaigns.wizard.errNameRequired')
    return
  }
  if (!accountId.value) {
    audienceError.value = t('campaigns.wizard.errAccountRequired')
    return
  }
  continuing.value = true
  try {
    const id = await ensureCampaign()
    if (!id) return
    if (!previewResult.value || previewStale.value) {
      if (!canCheckRecipients.value) {
        audienceError.value = t('campaigns.wizard.errNoRecipients')
        return
      }
      await checkRecipients()
    }
    if (!previewResult.value || previewResult.value.valid === 0) {
      audienceError.value = previewError.value || t('campaigns.wizard.errNoRecipients')
      return
    }
    if (!recipientsCommitted.value) {
      await campaigns.replaceRecipients(id, { text: pastedText.value, file: uploadedFile.value ?? undefined })
      recipientsCommitted.value = true
    }
    step.value = 'message'
  } catch (e) {
    audienceError.value = e instanceof ApiError ? e.message : t('campaigns.wizard.errSaveRecipientsFailed')
  } finally {
    continuing.value = false
  }
}
const continuing = ref(false)

// --- Step 2: message ---------------------------------------------------
const messageBody = ref('')
const variablesDetected = computed(() => [...messageBody.value.matchAll(/\{\{\s*([A-Za-z0-9_]+)\s*\}\}/g)].map((m) => m[1]))
const uniqueVariables = computed(() => [...new Set(variablesDetected.value)])
const messageTextareaEl = ref<HTMLTextAreaElement | null>(null)

// detectedColumns/unmatchedVariables mirror CampaignRecipientPreviewTable's
// own computeds (same source data, same rules) — duplicated locally rather
// than shared, since this component needs them to drive live insert chips
// and the autocomplete menu, not just render a read-only warning line.
const detectedColumns = computed(() => {
  const cols = new Set<string>()
  for (const row of previewResult.value?.rows ?? []) {
    if (row.name) cols.add('name')
    for (const key of Object.keys(row.attributes ?? {})) cols.add(key)
  }
  return [...cols].sort()
})
const chipVariables = computed(() => ['phone', ...detectedColumns.value.filter((c) => c !== 'phone')])
const unmatchedVariables = computed(() => uniqueVariables.value.filter((v) => v !== 'phone' && !detectedColumns.value.includes(v)))

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
// Computed here, not inline in the template, so the template's own source
// text never contains a literal {{/}} pair for the Vue compiler's mustache
// scanner to trip over (see CampaignTemplatesPanel.vue's own variableTag).
function variableToken(varName: string): string {
  return `{{${varName}}}`
}

// --- CAM-15: inline floating {{variable}} autocomplete -------------------
// Typing { or {{ opens a small menu right under the caret, filtered by
// whatever was typed since the brace — Enter/click inserts the full
// {{token}}, replacing exactly the open brace run rather than appending.
const OPEN_BRACE_RE = /\{\{?([A-Za-z0-9_]*)$/
const showAutocomplete = ref(false)
const autocompleteQuery = ref('')
const autocompleteMatchStart = ref(0)
const autocompleteIndex = ref(0)
const autocompletePos = ref({ top: 0, left: 0 })
const autocompleteCandidates = computed(() => {
  const q = autocompleteQuery.value.toLowerCase()
  return chipVariables.value.filter((c) => c.toLowerCase().startsWith(q))
})

// caretCoordinates measures where `position` falls inside the textarea by
// mirroring its text and computed styles into an off-screen div — jsdom
// (this app's DOM test environment) has no real layout engine, so this
// always resolves to {0, 0} there; real browsers place the menu precisely.
function caretCoordinates(el: HTMLTextAreaElement, position: number): { top: number; left: number } {
  const div = document.createElement('div')
  const style = getComputedStyle(el)
  const mirrored = [
    'boxSizing', 'width', 'paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft',
    'borderTopWidth', 'borderRightWidth', 'borderBottomWidth', 'borderLeftWidth',
    'fontStyle', 'fontVariant', 'fontWeight', 'fontSize', 'fontFamily', 'lineHeight',
    'textAlign', 'textTransform', 'letterSpacing', 'wordSpacing', 'whiteSpace', 'wordWrap',
  ] as const
  for (const prop of mirrored) (div.style as unknown as Record<string, string>)[prop] = (style as unknown as Record<string, string>)[prop]
  div.style.position = 'absolute'
  div.style.visibility = 'hidden'
  div.style.whiteSpace = 'pre-wrap'
  div.style.overflowWrap = 'break-word'
  document.body.appendChild(div)
  div.textContent = el.value.slice(0, position)
  const span = document.createElement('span')
  span.textContent = el.value.slice(position) || '.'
  div.appendChild(span)
  const top = span.offsetTop - el.scrollTop
  const left = span.offsetLeft
  document.body.removeChild(div)
  return { top, left }
}

function recomputeAutocomplete() {
  const el = messageTextareaEl.value
  if (!el) {
    showAutocomplete.value = false
    return
  }
  const caret = el.selectionStart ?? el.value.length
  const m = OPEN_BRACE_RE.exec(el.value.slice(0, caret))
  if (!m) {
    showAutocomplete.value = false
    return
  }
  autocompleteQuery.value = m[1]
  autocompleteMatchStart.value = caret - m[0].length
  autocompleteIndex.value = 0
  const coords = caretCoordinates(el, caret)
  const MENU_WIDTH = 224
  autocompletePos.value = { top: coords.top + 22, left: Math.max(0, Math.min(coords.left, el.clientWidth - MENU_WIDTH)) }
  showAutocomplete.value = true
}
function closeAutocomplete() {
  showAutocomplete.value = false
}
function chooseAutocomplete(varName: string) {
  const el = messageTextareaEl.value
  const caret = el?.selectionStart ?? messageBody.value.length
  const before = messageBody.value.slice(0, autocompleteMatchStart.value)
  const after = messageBody.value.slice(caret)
  const token = `{{${varName}}}`
  messageBody.value = before + token + after
  const newCaret = before.length + token.length
  showAutocomplete.value = false
  void nextTick(() => {
    el?.focus()
    el?.setSelectionRange(newCaret, newCaret)
  })
}
function handleMessageKeydown(e: KeyboardEvent) {
  if (!showAutocomplete.value || autocompleteCandidates.value.length === 0) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    autocompleteIndex.value = (autocompleteIndex.value + 1) % autocompleteCandidates.value.length
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    autocompleteIndex.value = (autocompleteIndex.value - 1 + autocompleteCandidates.value.length) % autocompleteCandidates.value.length
  } else if (e.key === 'Enter') {
    e.preventDefault()
    chooseAutocomplete(autocompleteCandidates.value[autocompleteIndex.value])
  } else if (e.key === 'Escape') {
    e.preventDefault()
    closeAutocomplete()
  }
}
// Arrow-left/right/Home/End move the caret with no native `input` event —
// everything else that can reposition it (typing, backspace, paste) already
// goes through @input, and Up/Down/Enter/Escape are fully owned by keydown
// above while the menu is open (re-deriving on their keyup would race
// Vue's own not-yet-flushed DOM update right after an Enter-insert).
const CARET_MOVE_KEYS = new Set(['ArrowLeft', 'ArrowRight', 'Home', 'End'])
function handleMessageKeyup(e: KeyboardEvent) {
  if (CARET_MOVE_KEYS.has(e.key)) recomputeAutocomplete()
}

// CAM-03: "Variables used: name, promo_code" is a fact about the template,
// not a preview of the actual message. Substituted with representative
// sample values (never a real recipient's data), so an unmapped custom
// variable falls back to a bracketed placeholder naming itself.
const SAMPLE_VARIABLE_VALUES: Record<string, string> = { name: 'Aigul', phone: '77011234567', code: 'SUMMER2026', promo_code: 'SUMMER2026' }
const showMessagePreview = ref(false)
const renderedMessagePreview = computed(() =>
  messageBody.value.replace(/\{\{\s*([A-Za-z0-9_]+)\s*\}\}/g, (_, v: string) => SAMPLE_VARIABLE_VALUES[v] ?? `[${v}]`),
)

// CAM-14: pick a saved template straight into the message, or save what was
// just typed back into the library without leaving the wizard.
const templateOptions = ref<CampaignTemplate[]>([])
async function loadTemplateOptions() {
  try {
    const res = await templates.list({ archived: false, pageSize: 100 })
    templateOptions.value = res.items
  } catch {
    // The picker just stays empty — typing a message from scratch always works.
  }
}
const selectedTemplateId = ref('')
watch(selectedTemplateId, (id) => {
  if (!id) return
  const tmpl = templateOptions.value.find((x) => x.id === id)
  if (tmpl) messageBody.value = tmpl.message_body
})
const saveTemplateOpen = ref(false)
const templateJustSaved = ref(false)
watch(messageBody, () => {
  templateJustSaved.value = false
})
function onTemplateSaved() {
  templateJustSaved.value = true
}

const messageError = ref('')
const continuingToSchedule = ref(false)
async function continueToSchedule() {
  messageError.value = ''
  if (!messageBody.value.trim()) {
    messageError.value = t('campaigns.wizard.errMessageRequired')
    return
  }
  continuingToSchedule.value = true
  try {
    await campaigns.update(pendingCampaignId.value, { message_body: messageBody.value })
    step.value = 'schedule'
  } catch (e) {
    messageError.value = e instanceof ApiError ? e.message : t('campaigns.wizard.errCreateFailed')
  } finally {
    continuingToSchedule.value = false
  }
}
function backToAudience() {
  step.value = 'audience'
}

// --- Step 3: pace + schedule + launch -------------------------------------
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
const scheduleError = ref('')
watch([scheduleMode, scheduleAtLocal], () => {
  scheduleError.value = ''
})

// budgetForEstimate backs the pre-flight duration estimate only — the
// visible AccountSendingBudget widget below does its own independent
// fetch+poll, kept fully self-contained rather than threading a prop
// through for one read.
const budgetForEstimate = ref<SendingBudget | null>(null)
watch(step, (s) => {
  if (s !== 'schedule' || !accountId.value) return
  campaigns
    .fetchSendingBudget(accountId.value)
    .then((b) => {
      budgetForEstimate.value = b
    })
    .catch(() => {
      budgetForEstimate.value = null
    })
})
const estimatedDurationLabel = computed(() => {
  const validCount = previewResult.value?.valid ?? 0
  if (validCount <= 1) return ''
  let interval: number
  let jit: number
  if (paceMode.value === 'custom') {
    interval = minInterval.value
    jit = jitter.value
  } else if (budgetForEstimate.value) {
    interval = budgetForEstimate.value.min_interval_seconds
    jit = budgetForEstimate.value.jitter_seconds
  } else {
    return ''
  }
  const totalMs = (validCount - 1) * (interval + jit / 2) * 1000
  return formatElapsed(totalMs, t)
})

function backToMessage() {
  step.value = 'message'
}

function validateScheduleFields(): boolean {
  scheduleError.value = ''
  scheduleActionError.value = ''
  if (scheduleMode.value === 'later') {
    const when = scheduleAtLocal.value ? new Date(scheduleAtLocal.value) : null
    if (!when || Number.isNaN(when.getTime())) {
      scheduleError.value = t('campaigns.wizard.errScheduleRequired')
      scheduleAtInputEl.value?.focus()
      return false
    }
    if (when.getTime() <= Date.now()) {
      scheduleError.value = t('campaigns.wizard.errScheduleInPast')
      scheduleAtInputEl.value?.focus()
      return false
    }
  }
  if (invalidWindow.value) {
    scheduleActionError.value = t('campaigns.limits.errInvalidWindow')
    return false
  }
  return true
}
function paceSchedulePatch(): Record<string, unknown> {
  const patch: Record<string, unknown> = {}
  if (paceMode.value === 'custom') {
    patch.min_interval_seconds = minInterval.value
    patch.jitter_seconds = jitter.value
  }
  if (localWindows.value.length > 0) {
    patch.windows = localToUtc(localWindows.value, offsetMinutes)
  }
  if (scheduleMode.value === 'later') {
    patch.schedule_at = new Date(scheduleAtLocal.value).toISOString()
  }
  return patch
}

const scheduleActionError = ref('')
const savingDraft = ref(false)
async function saveAsDraft() {
  if (!validateScheduleFields()) return
  savingDraft.value = true
  try {
    const patch = paceSchedulePatch()
    if (Object.keys(patch).length > 0) await campaigns.update(pendingCampaignId.value, patch)
    finished.value = true
    await router.push({ name: 'campaign-detail', params: { campaignId: pendingCampaignId.value }, query: { created: '1' } })
  } catch (e) {
    scheduleActionError.value = e instanceof ApiError ? e.message : t('campaigns.wizard.errCreateFailed')
  } finally {
    savingDraft.value = false
  }
}

// CAM-17: Launch always calls the same unified start action regardless of
// scheduleMode — POST /campaigns/:id/start resolves 'running' vs
// 'scheduled' itself from whether schedule_at is set and still in the
// future (backend/internal/httpapi/campaigns.go's handleStartCampaign), so
// there is no separate "arm the schedule" step to forget.
const launching = ref(false)
async function launchCampaign() {
  if (!validateScheduleFields()) return
  launching.value = true
  try {
    const patch = paceSchedulePatch()
    if (Object.keys(patch).length > 0) await campaigns.update(pendingCampaignId.value, patch)
    await campaigns.start(pendingCampaignId.value)
    finished.value = true
    await router.push({ name: 'campaign-detail', params: { campaignId: pendingCampaignId.value } })
  } catch (e) {
    scheduleActionError.value = e instanceof ApiError ? e.message : t('campaigns.wizard.errCreateFailed')
  } finally {
    launching.value = false
  }
}
</script>

<template>
  <div class="flex-1 overflow-y-auto px-8 py-6 max-w-2xl">
    <h1 class="text-xl font-semibold flex items-center gap-2"><Megaphone class="w-5 h-5" /> {{ t('campaigns.wizard.title') }}</h1>

    <!-- CAM-15: a real 3-step stepper — clicking back to an already-reached
         step is allowed (nothing is lost, all fields stay populated in this
         same component's state); a step ahead of what's been reached yet is
         disabled rather than skippable. -->
    <div class="mt-3 flex items-center gap-2 text-xs font-medium" data-testid="wizard-step-indicator">
      <button
        type="button"
        class="flex items-center gap-1.5 rounded-full px-2.5 py-1"
        data-testid="wizard-step-audience"
        :class="step === 'audience' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'"
        @click="step = 'audience'"
      >
        <span class="grid place-items-center w-4 h-4 rounded-full text-[10px]" :class="step === 'audience' ? 'bg-primary-foreground text-primary' : 'bg-muted'">1</span>
        {{ t('campaigns.wizard.stepAudience') }}
      </button>
      <span class="text-muted-foreground">→</span>
      <button
        type="button"
        class="flex items-center gap-1.5 rounded-full px-2.5 py-1 disabled:cursor-not-allowed disabled:opacity-50"
        data-testid="wizard-step-message"
        :disabled="!pendingCampaignId"
        :class="step === 'message' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'"
        @click="pendingCampaignId && (step = 'message')"
      >
        <span class="grid place-items-center w-4 h-4 rounded-full text-[10px]" :class="step === 'message' ? 'bg-primary-foreground text-primary' : 'bg-muted'">2</span>
        {{ t('campaigns.wizard.stepMessage') }}
      </button>
      <span class="text-muted-foreground">→</span>
      <button
        type="button"
        class="flex items-center gap-1.5 rounded-full px-2.5 py-1 disabled:cursor-not-allowed disabled:opacity-50"
        data-testid="wizard-step-schedule"
        :disabled="!pendingCampaignId || !messageBody.trim()"
        :class="step === 'schedule' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'"
        @click="pendingCampaignId && messageBody.trim() && (step = 'schedule')"
      >
        <span class="grid place-items-center w-4 h-4 rounded-full text-[10px]" :class="step === 'schedule' ? 'bg-primary-foreground text-primary' : 'bg-muted'">3</span>
        {{ t('campaigns.wizard.stepSchedule') }}
      </button>
    </div>

    <!-- Step 1: Who -->
    <section v-if="step === 'audience'" class="mt-6 space-y-4">
      <h2 class="text-sm font-semibold">{{ t('campaigns.wizard.audienceHeading') }}</h2>
      <div>
        <label class="text-xs font-medium text-muted-foreground">{{ t('campaigns.wizard.nameLabel') }}</label>
        <Input v-model="name" :placeholder="t('campaigns.wizard.namePlaceholder')" class="mt-1.5" />
      </div>
      <div>
        <label class="text-xs font-medium text-muted-foreground">{{ t('campaigns.wizard.accountLabel') }}</label>
        <div v-if="noAccounts" class="mt-1.5 rounded-md border border-border bg-muted/40 p-3">
          <p class="text-sm">{{ t('campaigns.wizard.noAccounts') }}</p>
          <RouterLink :to="{ name: 'channels' }" class="mt-2 inline-block">
            <Button type="button" variant="outline" size="sm">{{ t('campaigns.wizard.connectAccount') }}</Button>
          </RouterLink>
        </div>
        <Select v-else v-model="accountId">
          <SelectTrigger class="mt-1.5">
            <SelectValue :placeholder="t('campaigns.wizard.accountPlaceholder')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="a in sortedAccounts" :key="a.id" :value="a.id">
              <span class="flex items-center gap-1.5">
                {{ a.display_name || a.external_handle }}
                <Badge v-if="a.channel === 'simulator'" variant="secondary" class="gap-1 text-[10px] px-1.5 py-0">
                  <FlaskConical class="w-2.5 h-2.5" />{{ t('campaigns.wizard.simulatorBadge') }}
                </Badge>
              </span>
            </SelectItem>
          </SelectContent>
        </Select>
        <!-- CAM-05: live budget/quota headroom visible before any recipient
             is ever touched, not only after creation on the detail page. -->
        <AccountSendingBudget v-if="accountId && !noAccounts" :account-id="accountId" class="mt-2" />
        <p v-if="isSimulatorSelected" class="mt-2 flex items-start gap-1.5 rounded-md border border-border bg-muted/40 p-2.5 text-xs text-muted-foreground" data-testid="simulator-notice">
          <FlaskConical class="w-3.5 h-3.5 shrink-0 mt-0.5" />
          {{ t('campaigns.wizard.simulatorNotice') }}
        </p>
      </div>

      <div v-if="!noAccounts">
        <label class="text-xs font-medium text-muted-foreground">{{ t('campaigns.wizard.pasteLabel') }}</label>
        <p v-if="!audienceReady" class="mt-0.5 text-xs text-muted-foreground">{{ t('campaigns.wizard.audienceNeedsDetails') }}</p>
        <Textarea v-model="pastedText" :placeholder="t('campaigns.wizard.pastePlaceholder')" class="mt-1.5 min-h-[120px] font-mono text-xs" data-testid="paste-recipients" />
        <div class="mt-2 flex items-center gap-2">
          <label class="text-xs font-medium text-muted-foreground shrink-0">{{ t('campaigns.wizard.uploadLabel') }}</label>
          <input type="file" accept=".csv,.txt" class="text-xs" @change="onFileChange" />
        </div>
        <!-- CAM-06: the placeholder's two example lines say nothing about
             header rows, separators, or country codes — all auto-detected
             server-side, invisible from here without reading the Go source. -->
        <details class="mt-2 rounded-md border border-border p-2.5 text-xs text-muted-foreground">
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
        <Button type="button" variant="outline" size="sm" class="mt-2" :disabled="!canCheckRecipients || previewing" @click="checkRecipients">
          <LoaderCircle v-if="previewing" class="w-4 h-4 animate-spin" />
          {{ previewing ? t('campaigns.wizard.checking') : t('campaigns.wizard.checkReachability') }}
        </Button>
        <p v-if="previewError" class="mt-2 flex items-center gap-1.5 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0" /> {{ previewError }}
        </p>
        <CampaignRecipientPreviewTable v-if="previewResult" class="mt-2" :result="previewResult" :message-variables="[]" />
        <p v-else class="mt-2 text-xs text-muted-foreground">{{ t('campaigns.wizard.previewEmpty') }}</p>
        <p v-if="previewResult && previewStale" class="mt-1 flex items-center gap-1.5 text-xs text-amber-600" data-testid="preview-stale-notice">
          <CircleAlert class="w-3.5 h-3.5 shrink-0" /> {{ t('campaigns.wizard.previewStaleNotice') }}
        </p>
      </div>

      <p v-if="audienceError" class="flex items-center gap-1.5 text-sm text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0" /> {{ audienceError }}
      </p>
      <Button type="button" :disabled="continuing || noAccounts" @click="continueToMessage">
        <LoaderCircle v-if="continuing" class="w-4 h-4 animate-spin" />
        {{ t('campaigns.wizard.continueToMessage') }}
      </Button>
    </section>

    <!-- Step 2: What -->
    <section v-if="step === 'message'" class="mt-6 space-y-4">
      <h2 class="text-sm font-semibold">{{ t('campaigns.wizard.messageHeading') }}</h2>

      <div v-if="templateOptions.length">
        <label class="text-xs font-medium text-muted-foreground">{{ t('campaigns.wizard.templatePickerLabel') }}</label>
        <Select v-model="selectedTemplateId">
          <SelectTrigger class="mt-1.5" data-testid="template-picker">
            <SelectValue :placeholder="t('campaigns.wizard.templatePickerPlaceholder')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="tmpl in templateOptions" :key="tmpl.id" :value="tmpl.id">{{ tmpl.name }}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div>
        <label class="text-xs font-medium text-muted-foreground">{{ t('campaigns.wizard.messageLabel') }}</label>
        <p class="mt-0.5 text-xs text-muted-foreground">{{ t('campaigns.wizard.autocompleteTip') }}</p>
        <div class="relative mt-1.5">
          <textarea
            ref="messageTextareaEl"
            v-model="messageBody"
            :placeholder="t('campaigns.wizard.messagePlaceholder')"
            class="min-h-[120px] flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
            data-testid="message-textarea"
            @input="recomputeAutocomplete"
            @keydown="handleMessageKeydown"
            @keyup="handleMessageKeyup"
            @click="recomputeAutocomplete"
            @blur="closeAutocomplete"
          />
          <div
            v-if="showAutocomplete"
            class="absolute z-20 w-56 max-h-48 overflow-y-auto rounded-md border border-border bg-popover shadow-md py-1"
            :style="{ top: autocompletePos.top + 'px', left: autocompletePos.left + 'px' }"
            data-testid="var-autocomplete-menu"
          >
            <button
              v-for="(c, i) in autocompleteCandidates"
              :key="c"
              type="button"
              class="flex w-full items-center gap-1.5 px-2.5 py-1.5 text-left text-xs font-mono"
              :class="i === autocompleteIndex ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/50'"
              data-testid="var-autocomplete-item"
              @mousedown.prevent="chooseAutocomplete(c)"
            >
              {{ variableToken(c) }}
            </button>
            <p v-if="autocompleteCandidates.length === 0" class="px-2.5 py-1.5 text-xs text-muted-foreground">
              {{ t('campaigns.wizard.autocompleteNoMatches') }}
            </p>
          </div>
        </div>

        <div class="mt-1.5 flex flex-wrap items-center gap-1.5">
          <span class="text-xs text-muted-foreground">{{ t('campaigns.wizard.insertFromListLabel') }}</span>
          <button
            v-for="v in chipVariables"
            :key="v"
            type="button"
            class="rounded-full border border-border bg-muted px-2.5 py-1 text-xs font-mono hover:bg-accent"
            :data-testid="`insert-var-${v}`"
            @click="insertVariable(v)"
          >
            {{ chipLabel(v) }}
          </button>
        </div>
        <p v-if="uniqueVariables.length" class="mt-1 text-xs text-muted-foreground">
          {{ t('campaigns.wizard.variablesDetected', { variables: uniqueVariables.join(', ') }) }}
        </p>
        <p v-if="unmatchedVariables.length" class="mt-1 flex items-center gap-1.5 text-xs text-amber-600 dark:text-amber-400" data-testid="unmatched-variables-warning">
          <CircleAlert class="w-3.5 h-3.5 shrink-0" /> {{ t('campaigns.wizard.unmatchedVariables', { variables: unmatchedVariables.join(', ') }) }}
        </p>

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

      <div class="flex items-center gap-2">
        <Button type="button" variant="outline" size="sm" :disabled="!messageBody.trim()" data-testid="save-as-template" @click="saveTemplateOpen = true">
          <Save class="w-3.5 h-3.5" /> {{ t('campaigns.templates.saveFromWizard') }}
        </Button>
        <span v-if="templateJustSaved" class="text-xs text-wa" data-testid="template-saved-notice">{{ t('campaigns.templates.savedFromWizard') }}</span>
      </div>

      <p v-if="messageError" class="flex items-center gap-1.5 text-sm text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0" /> {{ messageError }}
      </p>
      <div class="flex items-center gap-2">
        <Button type="button" variant="ghost" @click="backToAudience">{{ t('campaigns.wizard.stepBack') }}</Button>
        <Button type="button" :disabled="continuingToSchedule" @click="continueToSchedule">
          <LoaderCircle v-if="continuingToSchedule" class="w-4 h-4 animate-spin" />
          {{ t('campaigns.wizard.continueToSchedule') }}
        </Button>
      </div>

      <CampaignTemplateFormDialog :open="saveTemplateOpen" :initial-body="messageBody" @update:open="saveTemplateOpen = $event" @saved="onTemplateSaved" />
    </section>

    <!-- Step 3: When & Launch -->
    <section v-if="step === 'schedule'" class="mt-6 space-y-3">
      <h2 class="text-sm font-semibold">{{ t('campaigns.wizard.scheduleHeading') }}</h2>
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
        <p v-if="scheduleMode === 'later' && scheduleError" class="mt-1.5 flex items-center gap-1.5 text-xs text-destructive" data-testid="schedule-error">
          <CircleAlert class="w-3.5 h-3.5 shrink-0" /> {{ scheduleError }}
        </p>
      </div>

      <!-- Pre-flight summary: what Launch is actually about to do. -->
      <div class="rounded-lg border border-border p-3 space-y-2" data-testid="preflight-summary">
        <h3 class="text-xs font-semibold flex items-center gap-1.5"><Megaphone class="w-3.5 h-3.5" /> {{ t('campaigns.wizard.summaryHeading') }}</h3>
        <p class="text-sm" data-testid="summary-reachable">{{ t('campaigns.wizard.summaryReachable', { count: previewResult?.valid ?? 0 }) }}</p>
        <p v-if="estimatedDurationLabel" class="text-sm text-muted-foreground" data-testid="summary-duration">{{ t('campaigns.wizard.summaryDuration', { duration: estimatedDurationLabel }) }}</p>
        <AccountSendingBudget v-if="accountId" :account-id="accountId" />
      </div>

      <p v-if="scheduleActionError" class="flex items-center gap-1.5 text-sm text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0" /> {{ scheduleActionError }}
      </p>
      <div class="flex items-center gap-2 pb-10">
        <Button type="button" variant="ghost" @click="backToMessage">{{ t('campaigns.wizard.stepBack') }}</Button>
        <Button type="button" variant="outline" :disabled="savingDraft || launching" data-testid="save-as-draft" @click="saveAsDraft">
          <LoaderCircle v-if="savingDraft" class="w-4 h-4 animate-spin" />
          {{ savingDraft ? t('common.saving') : t('campaigns.wizard.saveAsDraft') }}
        </Button>
        <Button type="button" :disabled="savingDraft || launching" data-testid="launch-campaign" @click="launchCampaign">
          <LoaderCircle v-if="launching" class="w-4 h-4 animate-spin" />
          {{ launching ? t('campaigns.wizard.launching') : t('campaigns.wizard.launchCampaign') }}
        </Button>
      </div>
    </section>

    <div v-if="pendingCampaignId" class="mt-2">
      <Button type="button" variant="ghost" :disabled="cancelling" @click="cancelPending">{{ t('campaigns.actions.cancel') }}</Button>
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
