<script setup lang="ts">
// AssistantRecord is the shared /playground (draft) + /knowledge-base (live)
// singleton row for the org's one assistant-config row (persona/mission/
// guardrails/language_policy/reply_max_words) — plan Task 14 wires config
// into Playground for the first time (previously it had no config editor at
// all); on /knowledge-base it replaces the old per-field "Обзор" dialog
// editor with the same always-visible-form convention every other singleton
// (Контакты/Политики) already uses.
//
// Config has no natural key of its own — 'main' is kbstore.NaturalKeyMain,
// the same fixed singleton key the assistant MCP tool already addresses it by
// (backend/internal/kbstore/mcp_write.go's MCPUpsertAssistant).
import { computed, reactive, watch } from 'vue'
import { Sparkles } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import type { DraftConfig } from '@/types'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import { changedFields, recordState } from './shared'

const CONFIG_KEY = 'main'

const props = defineProps<{
  mode: 'draft' | 'live'
  draftRow?: DraftConfig
  liveRow?: DraftConfig
}>()

const pg = usePlayground()

const row = computed(() => (props.mode === 'draft' ? props.draftRow : props.liveRow))
const visible = computed(() => props.mode === 'live' || !!row.value)
const busy = computed(() => (props.mode === 'draft' ? pg.busy : pg.liveBusy))
// A live config row always exists once the org is seeded, so hasLiveCounterpart
// is really just "any published config at all" — closer to 'published' than
// 'new' in practice, matching recordState's contract.
const state = computed(() => recordState(props.mode, !!props.liveRow))
const diff = computed(() =>
  changedFields(props.draftRow, props.liveRow, ['persona', 'mission', 'guardrails', 'language_policy', 'reply_max_words'])
)

const buf = reactive({ persona: '', mission: '', guardrails: '', language_policy: '', reply_max_words: 0 })
function seed(c: DraftConfig | undefined) {
  buf.persona = c?.persona ?? ''
  buf.mission = c?.mission ?? ''
  buf.guardrails = c?.guardrails ?? ''
  buf.language_policy = c?.language_policy ?? ''
  buf.reply_max_words = c?.reply_max_words ?? 0
}
// Config carries no id, and there is only ever one per org — seed once, then
// only re-seed if this specific mode's row transitions from absent to present
// (a fresh SSE-driven draft/live reload of already-seeded fields must not
// clobber an in-progress edit).
let seededOnce = false
watch(
  row,
  (c) => {
    if (seededOnce && c) return
    if (!c) return
    seededOnce = true
    seed(c)
  },
  { immediate: true }
)

function save() {
  const patch = { ...buf }
  if (props.mode === 'draft') pg.patchConfig(patch)
  else pg.livePatchConfig(patch)
}
function approve() {
  pg.approveEntity('config', CONFIG_KEY)
}
</script>

<template>
  <RecordShell
    v-if="visible"
    :icon="Sparkles"
    label="Ассистент"
    :mode="mode"
    :state="state"
    :busy="busy"
    singleton
    @save="save"
    @approve="approve"
  >
    <div>
      <span class="text-xs font-medium text-muted-foreground">Персона</span>
      <Textarea v-model="buf.persona" rows="3" placeholder="Кто ассистент и как он общается с клиентом." class="mt-1 min-h-0 text-[14px]" />
      <FieldDiffNote :show="diff.includes('persona')" :was="liveRow?.persona ?? ''" />
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">Миссия</span>
      <Textarea v-model="buf.mission" rows="2" placeholder="Главная цель ассистента в каждом диалоге." class="mt-1 min-h-0 text-[14px]" />
      <FieldDiffNote :show="diff.includes('mission')" :was="liveRow?.mission ?? ''" />
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">Ограничения (по одному пункту на строку)</span>
      <Textarea v-model="buf.guardrails" rows="4" placeholder="Правила и запреты…" class="mt-1 min-h-0 text-[14px]" />
      <FieldDiffNote :show="diff.includes('guardrails')" :was="liveRow?.guardrails ?? ''" />
    </div>
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div>
        <span class="text-xs font-medium text-muted-foreground">Языковая политика</span>
        <Input v-model="buf.language_policy" placeholder="На каком языке отвечать клиенту." class="mt-1 h-9" />
        <FieldDiffNote :show="diff.includes('language_policy')" :was="liveRow?.language_policy ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">Макс. слов в ответе</span>
        <Input v-model.number="buf.reply_max_words" type="number" min="0" class="mt-1 h-9 font-mono" />
        <FieldDiffNote :show="diff.includes('reply_max_words')" :was="String(liveRow?.reply_max_words ?? '')" />
      </div>
    </div>
  </RecordShell>
</template>
