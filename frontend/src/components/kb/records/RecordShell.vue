<script setup lang="ts">
// RecordShell is the shared chrome every kb/records/*Record.vue wraps its own
// field body in: a kind eyebrow + record heading + natural key, a state
// badge, an optional media row, and a footer action row driven by an
// explicit `actions` prop (owned by the caller via records/actions.ts's
// kbActions()/CONFIG_FIELD_ACTIONS — this component only renders the action
// set and fires the matching event). No usePlayground() import: this is a
// pure props-in/events-out shell, reused by both Черновик (draft review) and
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
    iconClass?: string // overrides the icon's own colour (e.g. CONFIG_ACCENT's icon tone) — omit to inherit the muted eyebrow colour
    label: string // kind eyebrow, e.g. "Тема" — always shown
    heading?: string // the record's OWN name/title — the card's visual hero
    recordKey?: string // natural key (slug/ref) — omitted for a true singleton (contacts/policies/config)
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
  { iconClass: undefined, heading: undefined, recordKey: undefined, busy: false, pendingMark: undefined, busyKey: undefined, blockedNote: undefined, updatedAt: undefined }
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
  <div class="group rounded-xl border border-border bg-card transition-shadow hover:shadow-card">
    <div class="p-4 sm:p-5">
      <div class="flex flex-wrap items-start justify-between gap-x-3 gap-y-2">
        <div class="min-w-[8rem] flex-1">
          <div class="flex flex-wrap items-center gap-x-1.5 gap-y-1 text-muted-foreground">
            <component :is="icon" class="h-3.5 w-3.5 shrink-0" :class="iconClass" />
            <span class="text-[11px] font-medium uppercase tracking-wide">{{ label }}</span>
            <code v-if="recordKey" class="text-[11px] font-mono text-muted-foreground/70">{{ recordKey }}</code>
          </div>
          <h3 v-if="heading" class="mt-1 truncate text-[15px] font-semibold leading-snug text-foreground">{{ heading }}</h3>
        </div>
        <div class="flex min-w-0 flex-wrap items-center justify-end gap-1.5">
          <slot name="trailing" />
          <Badge variant="secondary" :class="RECORD_STATE_META[state].cls + ' px-2 py-0.5 text-[11px] font-medium'">
            {{ t(RECORD_STATE_META[state].labelKey) }}
          </Badge>
          <Badge v-if="pendingMark" variant="outline" class="border-amber-300 px-2 py-0.5 text-[11px] font-medium text-amber-700 dark:border-amber-400/40 dark:text-amber-400">
            {{ t('kb.pendingMark.' + pendingMark) }}
          </Badge>
        </div>
      </div>

      <div class="mt-4 space-y-4">
        <slot />
      </div>

      <div v-if="$slots.media" class="mt-4">
        <slot name="media" />
      </div>

      <p v-if="blockedNote" class="mt-3 text-xs italic text-muted-foreground">{{ blockedNote }}</p>
    </div>

    <div class="flex flex-wrap items-center gap-2 border-t border-border/70 px-4 py-2.5 sm:px-5" :class="updatedAt ? 'justify-between' : 'justify-end'">
      <span v-if="updatedAt" class="text-[11px] text-muted-foreground" :title="updatedAt">{{ t('kb.fields.updatedAt') }} {{ shortTime(updatedAt) }}</span>
      <div class="flex flex-wrap items-center justify-end gap-2">
        <Button
          v-for="a in actions"
          :key="a.key"
          size="sm"
          :variant="a.variant"
          :class="a.destructive ? 'text-destructive hover:text-destructive' : ''"
          :disabled="busy"
          @click="fire(a.key)"
        >
          <LoaderCircle v-if="busy && busyKey === a.key" class="h-4 w-4 animate-spin" />
          {{ t(a.labelKey) }}
        </Button>
      </div>
    </div>
  </div>
</template>
