<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, ChevronRight, Code2, Copy } from 'lucide-vue-next'
import { evalsApi } from '@/api/evals'
import { caseLevelRequirements, costLabel, execIsPass, type TestCaseGroup } from '@/lib/evalMatrix'
import type { VExecution } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import ContractTable from './ContractTable.vue'

// One evaluation question, collapsed by default—the supporting-evidence layer
// under the decision matrix. The matrix comes first; cards open for inspection. Expanded,
// each execution reads as three sections: Input (the case itself — message/history or
// source image, plus a case-level "what's expected" summary), Requirements (the full
// per-execution contract table — expected/actual/pass for every check), and Model
// result (the model's raw/injected/extracted text).
// `expanded` is CONTROLLED by the parent (not internal state) specifically so a
// page-level "expand all/collapse all" toolbar can drive every card at once —
// an uncontrolled per-card ref would only capture its OWN initial default and
// never react to a later bulk toggle.
const props = defineProps<{ group: TestCaseGroup; runId: string; expanded: boolean }>()
const emit = defineEmits<{ (e: 'toggle'): void }>()

const { t } = useI18n()

const rawJsonExec = ref<VExecution | null>(null)
const zoomedImage = ref<string | null>(null)
const copiedIndex = ref<number | null>(null)

const caseRequirements = computed(() => caseLevelRequirements(props.group))
const caseImageURL = computed(() => {
  const caseId = props.group.execs[0]?.subject.case_id
  return caseId ? evalsApi.inputImageURL(props.runId, caseId) : null
})

function badgeVariant(group: TestCaseGroup): { class: string; label: string } {
  // Same label in all three branches — only the colour encodes all/none/some.
  const label = t('evals.card.modelsPassed', { pass: group.passCount, total: group.total })
  if (group.passCount === group.total) return { class: 'bg-emerald-100 text-emerald-700 hover:bg-emerald-100 border-transparent', label }
  if (group.passCount === 0) return { class: 'bg-red-100 text-red-700 hover:bg-red-100 border-transparent', label }
  return { class: 'bg-amber-100 text-amber-700 hover:bg-amber-100 border-transparent', label }
}

async function copyText(text: string, i: number) {
  try {
    await navigator.clipboard.writeText(text)
    copiedIndex.value = i
    setTimeout(() => { if (copiedIndex.value === i) copiedIndex.value = null }, 1500)
  } catch {
    // no clipboard permission — text stays visible/selectable, nothing else to do
  }
}

function onImgError(ev: Event) {
  ;(ev.target as HTMLImageElement).style.display = 'none'
}
</script>

<template>
  <div class="rounded-xl border border-border bg-card overflow-hidden">
    <button class="w-full text-left px-4 py-3 flex items-center gap-3 hover:bg-muted/40 transition" @click="emit('toggle')">
      <component :is="expanded ? ChevronDown : ChevronRight" class="w-4 h-4 text-muted-foreground shrink-0" />
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2 flex-wrap">
          <code class="text-xs text-muted-foreground">{{ group.id }}</code>
          <Badge :class="badgeVariant(group).class" class="text-[11px]">{{ badgeVariant(group).label }}</Badge>
        </div>
        <p class="text-sm mt-0.5 truncate">{{ group.message }}</p>
      </div>
      <div class="flex items-center gap-1 shrink-0">
        <span
          v-for="(e, i) in group.execs"
          :key="i"
          class="w-2.5 h-2.5 rounded-full"
          :class="execIsPass(e) ? 'bg-emerald-500' : 'bg-red-500'"
          :title="`${e.variant.model}: ${execIsPass(e) ? t('evals.passed') : t('evals.failed')}`"
        />
      </div>
    </button>

    <div v-if="expanded" class="border-t border-border">
      <div class="px-4 py-3 bg-muted/30 space-y-2.5">
        <div class="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">{{ t('evals.card.input') }}</div>

        <template v-if="group.execs[0]?.family === 'extract'">
          <div class="grid gap-3 sm:grid-cols-[220px_1fr]">
            <button v-if="caseImageURL" type="button" class="block w-fit" @click="zoomedImage = caseImageURL">
              <img
                :src="caseImageURL"
                class="max-w-[220px] max-h-[220px] rounded-lg border border-border object-contain cursor-zoom-in transition hover:opacity-90"
                :alt="t('evals.sourceFile')"
                @error="onImgError"
              />
            </button>
            <div class="flex flex-wrap content-start gap-1.5">
              <span v-for="r in caseRequirements" :key="r.key" class="rounded bg-muted px-2 py-1 text-[11px]">
                <b class="text-foreground">{{ r.label }}:</b> {{ r.expected || '—' }}
              </span>
              <p v-if="!caseRequirements.length" class="text-xs italic text-muted-foreground">{{ t('evals.contract.unavailable') }}</p>
            </div>
          </div>
        </template>
        <template v-else>
          <p class="text-sm whitespace-pre-wrap">{{ group.message }}</p>
          <details v-if="group.execs[0]?.subject.history?.length">
            <summary class="text-xs text-muted-foreground cursor-pointer hover:text-foreground select-none">{{ t('evals.card.history', { n: group.execs[0].subject.history!.length }) }}</summary>
            <div class="mt-1.5 space-y-1">
              <div v-for="(h, i) in group.execs[0].subject.history" :key="i" class="text-xs text-muted-foreground">
                <span class="font-medium">{{ h.role === 'client' ? t('evals.card.roleClient') : t('evals.card.roleAssistant') }}:</span> {{ h.text }}
              </div>
            </div>
          </details>
          <div v-if="caseRequirements.length" class="flex flex-wrap gap-1.5">
            <span v-for="r in caseRequirements" :key="r.key" class="rounded bg-muted px-2 py-1 text-[11px]">
              <b class="text-foreground">{{ r.label }}:</b> {{ r.expected || '—' }}
            </span>
          </div>
        </template>
      </div>

      <div class="divide-y divide-border">
        <div v-for="(e, i) in group.execs" :key="i" class="p-4 space-y-3">
          <div class="flex items-center gap-2 flex-wrap">
            <code class="text-xs font-medium">{{ e.variant.model }}</code>
            <Badge v-if="e.output.error || e.output.parse_error" variant="destructive" class="text-[11px]">{{ t('evals.card.errorBadge') }}</Badge>
            <Badge v-else :variant="execIsPass(e) ? 'default' : 'destructive'" class="text-[11px]">{{ execIsPass(e) ? t('evals.passed') : t('evals.failed') }}</Badge>
            <Badge v-if="e.family === 'scenario' && !e.rollups.find((r) => r.key === 'contract_pass')?.pass" variant="outline" class="text-[11px] border-amber-400 text-amber-700">{{ t('evals.card.contractBroken') }}</Badge>
            <Badge v-if="e.scenario?.truncated" variant="outline" class="text-[11px] border-destructive text-destructive">{{ t('evals.card.truncated') }}</Badge>
            <Badge v-if="e.scenario?.reasoning_leak" variant="outline" class="text-[11px] border-destructive text-destructive">{{ t('evals.card.reasoningLeak') }}</Badge>
            <Badge v-if="e.retries" variant="outline" class="text-[11px] border-amber-400 text-amber-700">{{ t('evals.card.retries', { n: e.retries }) }}</Badge>
            <span v-if="e.variant.prompt?.name" class="text-xs text-muted-foreground font-mono">{{ e.variant.prompt.name }}@v{{ e.variant.prompt.version }}</span>
            <span class="ml-auto text-xs text-muted-foreground">{{ costLabel(e.cost.basis, e.cost.estimate_usd, t) }}<template v-if="e.latency_ms"> · {{ t('evals.duration.s', { s: (e.latency_ms / 1000).toFixed(1) }) }}</template></span>
          </div>

          <p v-if="e.output.error" class="text-xs text-destructive bg-red-50 rounded-lg px-2.5 py-2">{{ t('evals.card.callError') }} {{ e.output.error }}</p>
          <p v-else-if="e.output.parse_error" class="text-xs text-destructive bg-red-50 rounded-lg px-2.5 py-2">{{ t('evals.card.parseError') }} {{ e.output.parse_error }}</p>

          <div>
            <div class="text-[10px] font-medium uppercase tracking-wide text-muted-foreground mb-1.5">{{ t('evals.card.requirements') }}</div>
            <ContractTable :rows="e.contract ?? []" />
          </div>

          <details>
            <summary class="text-[10px] font-medium uppercase tracking-wide text-muted-foreground cursor-pointer select-none hover:text-foreground">
              {{ t('evals.card.allChecks', { n: e.scores?.length ?? 0 }) }}
            </summary>
            <table class="w-full text-xs mt-1.5">
              <tbody>
                <tr v-for="s in e.scores ?? []" :key="s.name" class="border-b border-border/60 align-top last:border-0">
                  <td class="py-1 pr-3 whitespace-nowrap"><code class="font-mono">{{ s.name }}</code></td>
                  <td class="py-1 pr-3">
                    <Badge v-if="s.status === 'pass'" class="border-transparent bg-emerald-100 text-[11px] text-emerald-700 hover:bg-emerald-100">PASS</Badge>
                    <Badge v-else-if="s.status === 'fail'" variant="destructive" class="text-[11px]">FAIL</Badge>
                    <Badge v-else-if="s.status === 'error'" variant="destructive" class="text-[11px]">ERROR</Badge>
                    <span v-else class="text-[11px] italic text-muted-foreground">{{ t('evals.card.notChecked') }}</span>
                  </td>
                  <td class="py-1 max-w-[320px] whitespace-pre-wrap wrap-break-word text-muted-foreground">{{ s.detail || '—' }}</td>
                </tr>
              </tbody>
            </table>
          </details>

          <div>
            <div class="text-[10px] font-medium uppercase tracking-wide text-muted-foreground mb-1.5">{{ t('evals.card.modelResult') }}</div>
            <div class="grid gap-3 sm:grid-cols-2">
              <div v-if="e.family === 'scenario'" class="space-y-1">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-medium text-muted-foreground">{{ t('evals.card.modelReply') }}</span>
                  <button v-if="e.output.reply_text" class="text-muted-foreground hover:text-foreground transition" @click="copyText(e.output.reply_text, i)">
                    <Copy class="w-3.5 h-3.5" />
                  </button>
                </div>
                <p class="text-sm font-mono bg-muted/50 rounded-lg p-2.5 whitespace-pre-wrap max-h-40 overflow-y-auto">{{ e.output.reply_text || '—' }}</p>
                <p v-if="copiedIndex === i" class="text-[11px] text-muted-foreground">{{ t('common.copied') }}</p>
              </div>
              <div v-if="e.family === 'scenario'" class="space-y-1">
                <div class="text-xs font-medium text-muted-foreground">{{ t('evals.card.afterInjection') }}</div>
                <p
                  class="text-sm rounded-lg p-2.5 whitespace-pre-wrap max-h-40 overflow-y-auto"
                  :class="e.scenario?.injected_text && e.scenario.injected_text !== e.output.reply_text ? 'bg-emerald-50' : 'bg-muted/50'"
                >{{ e.scenario?.injected_text || '—' }}</p>
              </div>
              <div v-if="e.family === 'extract'" class="space-y-1 sm:col-span-2">
                <div class="text-xs font-medium text-muted-foreground">{{ t('evals.card.extractedText') }}</div>
                <p class="text-sm font-mono bg-muted/50 rounded-lg p-2.5 whitespace-pre-wrap max-h-40 overflow-y-auto">{{ e.extract?.extracted_text || '—' }}</p>
              </div>
            </div>

            <div class="flex flex-wrap gap-1.5 text-[11px] text-muted-foreground mt-2">
              <span class="rounded bg-muted px-1.5 py-0.5">{{ t('evals.card.tokens', { in: e.cost.tokens_in, out: e.cost.tokens_out }) }}</span>
              <span class="rounded bg-muted px-1.5 py-0.5">{{ t('evals.card.costBasis') }} {{ e.cost.basis }}</span>
              <span v-if="e.variant.preprocessor" class="rounded bg-muted px-1.5 py-0.5">{{ t('evals.card.preprocessor') }} {{ e.variant.preprocessor }}</span>
              <span v-if="e.output.reasoning" class="rounded bg-amber-100 text-amber-700 px-1.5 py-0.5">{{ t('evals.card.hasReasoning') }}</span>
            </div>

            <div v-if="e.family === 'extract' && (e.extract?.language || e.extract?.summary || e.extract?.relates_to_hint)" class="grid grid-cols-2 gap-x-4 gap-y-1 text-[11px] text-muted-foreground bg-muted/40 rounded-lg p-2.5 mt-2">
              <span v-if="e.extract.language"><b class="text-foreground">{{ t('evals.card.language') }}</b> {{ e.extract.language }}</span>
              <span v-if="e.extract.summary" class="col-span-2"><b class="text-foreground">{{ t('evals.card.summary') }}</b> {{ e.extract.summary }}</span>
              <span v-if="e.extract.relates_to_hint" class="col-span-2"><b class="text-foreground">{{ t('evals.card.relatesTo') }}</b> {{ e.extract.relates_to_hint }}</span>
            </div>

            <button class="inline-flex items-center gap-1 text-xs text-primary hover:underline mt-2" @click="rawJsonExec = e">
              <Code2 class="w-3.5 h-3.5" /> Raw JSON
            </button>
          </div>
        </div>
      </div>
    </div>

    <Dialog :open="!!rawJsonExec" @update:open="(v) => { if (!v) rawJsonExec = null }">
      <DialogContent class="max-w-2xl">
        <DialogHeader><DialogTitle>{{ t('evals.card.rawJsonTitle') }}</DialogTitle></DialogHeader>
        <pre class="mx-5 mb-5 text-xs font-mono bg-muted/50 rounded-lg p-3 overflow-auto max-h-[65vh] whitespace-pre-wrap">{{ JSON.stringify(rawJsonExec, null, 2) }}</pre>
      </DialogContent>
    </Dialog>

    <Dialog :open="!!zoomedImage" @update:open="(v) => { if (!v) zoomedImage = null }">
      <DialogContent class="max-w-3xl">
        <DialogHeader><DialogTitle>{{ t('evals.sourceFileTitle') }}</DialogTitle></DialogHeader>
        <img v-if="zoomedImage" :src="zoomedImage" class="mx-5 mb-5 max-h-[75vh] w-auto rounded-lg border border-border object-contain" :alt="t('evals.sourceFile')" />
      </DialogContent>
    </Dialog>
  </div>
</template>
