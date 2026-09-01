<script setup lang="ts">
// AssistantFieldRecord is a read-only display card for ONE assistant-config
// field (persona/mission/guardrails/language_policy/reply_max_words) —
// replaces the old whole-config AssistantRecord.vue now that config edits
// are per-field (forms/AssistantFieldForm.vue) and publishing is
// group-level (ConfigChangeGroup.vue), not per-field. Reused by Черновик
// (a pending field, actions Изменить/Отменить изменение — see
// records/actions.ts's CONFIG_FIELD_ACTIONS) and Знаний база's Обзор tab (a
// published field, actions Изменить only).
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ConfigSectionMeta } from '@/components/kb/kbEntities'
import type { ChangeType } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import { Badge } from '@/components/ui/badge'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import { stateForChange } from './shared'

const props = defineProps<{
  section: ConfigSectionMeta
  value: string | number
  liveValue?: string | number
  changeType?: ChangeType // absent on Знаний база; always 'updated' on Черновик (config fields are never added/removed, only patched)
  actions: KbAction[]
  busy?: boolean
}>()

defineEmits<{ edit: []; cancel: [] }>()
const { t } = useI18n()

const state = computed(() => (props.changeType ? stateForChange(props.changeType) : 'published'))
const changed = computed(() => props.liveValue !== undefined && String(props.value) !== String(props.liveValue))

function lines(v: string | number): string[] {
  return String(v || '')
    .split('\n')
    .map((x) => x.trim())
    .filter(Boolean)
}
</script>

<template>
  <RecordShell
    :icon="section.icon"
    :label="t(section.i18nKey + '.title')"
    :state="state"
    :actions="actions"
    :busy="busy"
    @edit="$emit('edit')"
    @cancel="$emit('cancel')"
  >
    <p class="text-xs text-muted-foreground">{{ t(section.i18nKey + '.hint') }}</p>

    <template v-if="section.render === 'badge'">
      <Badge v-if="value" variant="secondary" class="bg-indigo-100 text-indigo-700 text-[13px] font-medium">
        {{ value }} {{ t('kb.config.wordsUnit') }}
      </Badge>
      <span v-else class="text-muted-foreground italic text-sm">{{ t('kb.config.empty') }}</span>
    </template>
    <ul v-else-if="section.render === 'list'" class="space-y-1.5">
      <li v-for="(ln, i) in lines(value)" :key="i" class="flex gap-2 text-sm">
        <span class="text-muted-foreground select-none">•</span><span>{{ ln }}</span>
      </li>
      <li v-if="!lines(value).length" class="text-muted-foreground italic text-sm">{{ t('kb.config.empty') }}</li>
    </ul>
    <template v-else>
      <p v-for="(p, i) in lines(value)" :key="i" class="text-sm" :class="i > 0 ? 'mt-2' : ''">{{ p }}</p>
      <p v-if="!lines(value).length" class="text-muted-foreground italic text-sm">{{ t('kb.config.empty') }}</p>
    </template>

    <FieldDiffNote :show="changed" :was="String(liveValue ?? '')" :now="String(value ?? '')" />
  </RecordShell>
</template>
