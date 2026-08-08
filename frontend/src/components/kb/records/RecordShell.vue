<script setup lang="ts">
// RecordShell is the shared chrome every kb/records/*Record.vue wraps its own
// field body in: an icon + label + <code> natural key + state badge header,
// and an action row driven by an explicit `actions` prop (owned by the
// caller via records/actions.ts's kbActions()/CONFIG_FIELD_ACTIONS — this
// component no longer decides the action set itself, only renders it and
// fires the matching event). No usePlayground() import: this is a pure
// props-in/events-out shell, reused by both Черновик (draft review) and
// Знаний база (RecordList's published rows).
import { type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { LoaderCircle } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { shortTime } from '@/lib/format'
import type { KbAction, KbActionKey } from './actions'
import { RECORD_STATE_META, type RecordState } from './shared'

withDefaults(
  defineProps<{
    icon: Component
    label: string
    recordKey?: string // omitted for a true singleton (contacts/policies/config) — no natural key to show
    state: RecordState
    // pendingMark: Знаний база only — a published row that ALSO has a
    // pending draft change. Deliberately just a mark, never the draft
    // value itself (usePendingIndex's own contract) — this component only
    // ever renders `label`/`row` fields from its own row prop, so there is
    // no path for a draft value to leak in here even by accident.
    pendingMark?: 'updated' | 'removed'
    actions: KbAction[]
    busy?: boolean
    busyKey?: KbActionKey // which action's spinner shows while busy — omit to just disable the row
    blockedNote?: string // §2.7: a neutral pointer to a page-level gate failure — never "this record is invalid"
    updatedAt?: string // row.updated_at — the one DB column every *Record.vue didn't already surface somewhere in its own field body
  }>(),
  { busy: false, recordKey: undefined, pendingMark: undefined, busyKey: undefined, blockedNote: undefined, updatedAt: undefined }
)

const emit = defineEmits<{ edit: []; publish: []; cancel: []; delete: [] }>()
const { t } = useI18n()

function fire(key: KbActionKey) {
  switch (key) {
    case 'edit':
      emit('edit')
      break
    case 'publish':
      emit('publish')
      break
    case 'cancel':
      emit('cancel')
      break
    case 'delete':
      emit('delete')
      break
  }
}
</script>

<template>
  <div class="rounded-lg border border-border bg-card p-4 space-y-2">
    <div class="flex items-center gap-2 flex-wrap">
      <span class="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground">
        <component :is="icon" class="w-3.5 h-3.5" /> {{ label }}
      </span>
      <code v-if="recordKey" class="text-[13px] font-mono font-medium">{{ recordKey }}</code>
      <Badge variant="secondary" :class="RECORD_STATE_META[state].cls + ' text-[11px] font-medium'">
        {{ t(RECORD_STATE_META[state].labelKey) }}
      </Badge>
      <Badge v-if="pendingMark" variant="outline" class="border-amber-300 text-amber-700 text-[11px] font-medium">
        {{ t('kb.pendingMark.' + pendingMark) }}
      </Badge>
      <span v-if="updatedAt" class="ml-auto text-[11px] text-muted-foreground" :title="updatedAt">
        {{ t('kb.fields.updatedAt') }} {{ shortTime(updatedAt) }}
      </span>
    </div>

    <slot />

    <p v-if="blockedNote" class="text-xs text-muted-foreground italic">{{ blockedNote }}</p>

    <div class="flex items-center gap-2 flex-wrap">
      <Button
        v-for="a in actions"
        :key="a.key"
        size="sm"
        :variant="a.variant"
        :class="a.destructive ? 'text-destructive' : ''"
        :disabled="busy"
        @click="fire(a.key)"
      >
        <LoaderCircle v-if="busy && busyKey === a.key" class="w-4 h-4 animate-spin" />
        {{ t(a.labelKey) }}
      </Button>
    </div>
  </div>
</template>
