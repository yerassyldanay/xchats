<script setup lang="ts">
// AssistantFieldRecord is a read-only card for ONE assistant-config field —
// reused by Черновик's ConfigChangeGroup (one card per PENDING field only;
// decision 6) and База знаний's Обзор tab (one card per field, always, with
// a pending mark like every other record — see usePendingIndex). The caller
// supplies `actions`: ConfigChangeGroup passes edit+cancel (cancel clears
// just this field's patch, kbstore.CancelChange("config", field));
// KnowledgeBase passes edit-only (config has no delete affordance, and
// publishing the whole section lives one level up in ConfigChangeGroup —
// §1.5, the backend can only publish the entire staged patch, never one
// field in isolation).
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Badge } from '@/components/ui/badge'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import { CONFIG_ACCENT, CONFIG_SECTIONS } from '@/components/kb/kbEntities'
import type { ChangeType, ConfigField } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import { stateForChange, type RecordState } from './shared'

const props = defineProps<{
  field: ConfigField
  draftValue: string | number
  liveValue?: string | number
  changeType?: ChangeType // absent on База знаний when nothing is pending for this field
  actions: KbAction[]
  busy?: boolean
}>()
const emit = defineEmits<{ edit: []; cancel: [] }>()
const { t } = useI18n()

const section = computed(() => CONFIG_SECTIONS.find((s) => s.key === props.field)!)
const state = computed<RecordState>(() => (props.changeType ? stateForChange(props.changeType) : 'published'))
const diff = computed(() => props.changeType === 'updated' && props.liveValue !== undefined && String(props.liveValue) !== String(props.draftValue))

function lines(v: string | number): string[] {
  return String(v ?? '').split('\n').map((x) => x.trim()).filter(Boolean)
}
</script>

<template>
  <RecordShell
    :icon="section.icon"
    :label="t(section.titleKey)"
    :state="state"
    :actions="actions"
    :busy="busy"
    @edit="emit('edit')"
    @cancel="emit('cancel')"
  >
    <template v-if="section.render === 'badge'">
      <Badge variant="secondary" :class="CONFIG_ACCENT[section.accent].box + ' text-[13px] font-medium'">{{ draftValue }} слов</Badge>
    </template>
    <ul v-else-if="section.render === 'list'" class="space-y-1 text-sm">
      <li v-for="(ln, i) in lines(draftValue)" :key="i" class="flex gap-2">
        <span class="text-muted-foreground select-none">•</span><span>{{ ln }}</span>
      </li>
      <li v-if="!lines(draftValue).length" class="text-muted-foreground italic text-sm">— не задано —</li>
    </ul>
    <template v-else>
      <p v-for="(p, i) in lines(draftValue)" :key="i" class="text-sm" :class="i > 0 ? 'mt-2' : ''">{{ p }}</p>
      <p v-if="!lines(draftValue).length" class="text-muted-foreground italic text-sm">— не задано —</p>
    </template>
    <FieldDiffNote :show="diff" :was="String(liveValue ?? '')" />
  </RecordShell>
</template>
