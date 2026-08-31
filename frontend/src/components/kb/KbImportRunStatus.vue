<script setup lang="ts">
// KbImportRunStatus is a pure props-in display of one KbImportRun — no
// store access, so it renders identically whether fed the actively-tracked
// run (KbImportCard) or, later, a picked entry from a run history list
// (KB-14). Materials show pass-1 progress; the synthesis block only
// appears once pass 2 has actually started (run.synthesis is omitted until
// then, same как RunSummary.synthesis's own omitempty on the wire).
//
// KB-04/05: elapsed time, a step summary, and who started the run are all
// derived here from data RunSummary already carries (or, for
// startedByLabel, a name the CALLER resolves against the org's own
// already-loaded user list — the API itself only ever returns a raw user
// id, see KbImportRun.started_by's own doc comment — keeping this
// component's own "no store access" rule intact). Cancel stays a plain
// emit for the same reason: the caller owns the actual store call.
import { computed, onMounted, onUnmounted, ref, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, CircleCheck, Clock, FileText, Link as LinkIcon, LoaderCircle, User, X } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { formatElapsed } from '@/lib/format'
import type { KbImportMaterialStatus, KbImportRun, KbImportRunStatus as RunStatus } from '@/types'

const props = withDefaults(defineProps<{ run: KbImportRun; cancellable?: boolean; cancelling?: boolean; startedByLabel?: string }>(), {
  cancellable: true,
  cancelling: false,
  startedByLabel: '',
})
const emit = defineEmits<{ cancel: [] }>()
const { t } = useI18n()

const RUN_STATUS_META: Record<RunStatus, { labelKey: string; cls: string; spin: boolean }> = {
  extracting: { labelKey: 'kb.import.status.extracting', cls: 'bg-sky-100 text-sky-700', spin: true },
  synthesizing: { labelKey: 'kb.import.status.synthesizing', cls: 'bg-sky-100 text-sky-700', spin: true },
  built: { labelKey: 'kb.import.status.built', cls: 'bg-emerald-100 text-emerald-700', spin: false },
  failed: { labelKey: 'kb.import.status.failed', cls: 'bg-red-100 text-red-700', spin: false },
  needs_human: { labelKey: 'kb.import.status.needs_human', cls: 'bg-amber-100 text-amber-700', spin: false },
  cancelled: { labelKey: 'kb.import.status.cancelled', cls: 'bg-secondary text-secondary-foreground', spin: false },
}
const MATERIAL_STATUS_META: Record<KbImportMaterialStatus['processing_status'], { labelKey: string; cls: string; spin: boolean }> = {
  queued: { labelKey: 'kb.import.materialStatus.queued', cls: 'bg-secondary text-secondary-foreground', spin: false },
  extracting: { labelKey: 'kb.import.materialStatus.extracting', cls: 'bg-sky-100 text-sky-700', spin: true },
  parsed: { labelKey: 'kb.import.materialStatus.parsed', cls: 'bg-emerald-100 text-emerald-700', spin: false },
  needs_human: { labelKey: 'kb.import.materialStatus.needs_human', cls: 'bg-amber-100 text-amber-700', spin: false },
  failed: { labelKey: 'kb.import.materialStatus.failed', cls: 'bg-red-100 text-red-700', spin: false },
  cancelled: { labelKey: 'kb.import.materialStatus.cancelled', cls: 'bg-secondary text-secondary-foreground', spin: false },
}
const KIND_ICON: Record<KbImportMaterialStatus['kind'], Component> = { url: LinkIcon, file: FileText }

const runMeta = computed(() => RUN_STATUS_META[props.run.status])

// KB-04: a live elapsed-time counter — ticked every second while the run
// is still going, so the operator sees it moving rather than a number
// frozen at whatever it read on mount. Terminal runs need no ticking at
// all (the elapsed span is already fixed), so the interval simply never
// starts for one.
const nowTick = ref(Date.now())
let timer: ReturnType<typeof setInterval> | null = null
const TERMINAL: RunStatus[] = ['built', 'failed', 'needs_human', 'cancelled']
onMounted(() => {
  if (!TERMINAL.includes(props.run.status)) {
    timer = setInterval(() => {
      nowTick.value = Date.now()
    }, 1000)
  }
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
})
const elapsedLabel = computed(() => {
  if (!props.run.started_at) return ''
  const startedMs = new Date(props.run.started_at).getTime()
  if (Number.isNaN(startedMs)) return ''
  return formatElapsed(nowTick.value - startedMs, t)
})

// KB-04: "Step 1/2: Parsed 3/5 files" — a terminal material (parsed, or
// one that gave up: needs_human/failed/cancelled) counts as "done" for
// this count even when the run's own overall outcome is not a success.
const DONE_MATERIAL_STATUSES: KbImportMaterialStatus['processing_status'][] = ['parsed', 'needs_human', 'failed', 'cancelled']
const doneMaterials = computed(() => props.run.materials.filter((m) => DONE_MATERIAL_STATUSES.includes(m.processing_status)).length)
const stepLabel = computed(() => {
  if (props.run.status === 'extracting') {
    return t('kb.import.stepExtracting', { done: doneMaterials.value, total: props.run.materials.length })
  }
  if (props.run.status === 'synthesizing') return t('kb.import.stepSynthesizing')
  return ''
})

const showCancel = computed(() => props.cancellable && props.run.cancelable && !TERMINAL.includes(props.run.status))
</script>

<template>
  <div class="rounded-lg border border-border bg-card p-4 space-y-3" data-testid="kb-import-run-status">
    <div class="flex items-center gap-2 flex-wrap">
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.import.runLabel') }}</span>
      <Badge variant="secondary" :class="runMeta.cls + ' text-[11px] font-medium'" data-testid="kb-import-run-badge">
        <LoaderCircle v-if="runMeta.spin" class="w-3 h-3 animate-spin" />
        {{ t(runMeta.labelKey) }}
      </Badge>
      <Button
        v-if="showCancel"
        type="button"
        variant="outline"
        size="sm"
        class="ml-auto h-7 text-destructive hover:bg-destructive/10"
        :disabled="cancelling"
        data-testid="kb-import-cancel"
        @click="emit('cancel')"
      >
        <LoaderCircle v-if="cancelling" class="w-3.5 h-3.5 animate-spin" />
        <X v-else class="w-3.5 h-3.5" />
        {{ t('kb.import.cancelButton') }}
      </Button>
    </div>

    <div class="flex items-center gap-3 flex-wrap text-xs text-muted-foreground">
      <span v-if="startedByLabel" class="flex items-center gap-1"><User class="w-3.5 h-3.5" /> {{ t('kb.import.startedBy', { name: startedByLabel }) }}</span>
      <span v-if="elapsedLabel" class="flex items-center gap-1" data-testid="kb-import-elapsed"><Clock class="w-3.5 h-3.5" /> {{ t('kb.import.elapsedFor', { elapsed: elapsedLabel }) }}</span>
      <span v-if="stepLabel">{{ stepLabel }}</span>
    </div>

    <div class="space-y-1.5">
      <div
        v-for="m in run.materials"
        :key="m.id"
        class="flex items-center gap-2 flex-wrap rounded-md border border-border px-3 py-2"
      >
        <component :is="KIND_ICON[m.kind]" class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
        <span class="text-sm truncate flex-1 min-w-0" :title="m.label">{{ m.label }}</span>
        <Badge variant="secondary" :class="MATERIAL_STATUS_META[m.processing_status].cls + ' text-[11px] font-medium shrink-0'">
          <LoaderCircle v-if="MATERIAL_STATUS_META[m.processing_status].spin" class="w-3 h-3 animate-spin" />
          {{ t(MATERIAL_STATUS_META[m.processing_status].labelKey) }}
        </Badge>
        <p v-if="m.error" class="basis-full text-xs text-destructive">{{ m.error }}</p>
      </div>
    </div>

    <div v-if="run.synthesis" class="space-y-2 border-t border-border pt-3">
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.import.synthesis.title') }}</span>

      <div v-if="run.synthesis.applied?.length" class="space-y-1">
        <div v-for="(a, i) in run.synthesis.applied" :key="'a' + i" class="flex items-center gap-2 text-sm">
          <CircleCheck class="w-3.5 h-3.5 text-emerald-600 shrink-0" />
          <code class="text-[13px] font-mono">{{ a.type }}:{{ a.key }}</code>
          <Badge variant="outline" class="text-[11px]">{{ t(a.created ? 'kb.import.synthesis.created' : 'kb.import.synthesis.updated') }}</Badge>
        </div>
      </div>

      <div v-if="run.synthesis.dropped?.length" class="space-y-1">
        <div v-for="(d, i) in run.synthesis.dropped" :key="'d' + i" class="flex items-start gap-2 text-sm">
          <CircleAlert class="w-3.5 h-3.5 text-amber-600 shrink-0 mt-0.5" />
          <span class="text-muted-foreground">{{ d.tool }}: {{ d.reason }}</span>
        </div>
      </div>

      <p v-if="run.synthesis.notes" class="text-xs text-muted-foreground italic">{{ run.synthesis.notes }}</p>

      <p v-if="!run.synthesis.applied?.length && !run.synthesis.dropped?.length && run.synthesis.status === 'running'" class="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Clock class="w-3.5 h-3.5" /> {{ t('kb.import.synthesis.running') }}
      </p>
    </div>
  </div>
</template>
