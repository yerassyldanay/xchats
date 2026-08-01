<script setup lang="ts">
// RecordShell is the shared chrome every kb/records/*Record.vue wraps its own
// read-only field body in: an icon + label + <code> natural key + state badge
// header, and an action row — the caller computes its OWN actions (via
// kbActions() or a fixed set, e.g. AssistantFieldRecord's edit+cancel) and
// passes it down, so this component stays Pinia-free and purely props-in/
// events-out.
import type { Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { LoaderCircle } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import type { KbAction, KbActionKey } from './actions'
import { RECORD_STATE_META, type RecordState } from './shared'

withDefaults(
  defineProps<{
    icon: Component
    label: string
    recordKey?: string // omitted for a true singleton (contacts/policies/config) — no natural key to show
    state: RecordState
    actions: KbAction[]
    busy?: boolean
  }>(),
  { busy: false, recordKey: undefined }
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
        {{ RECORD_STATE_META[state].label }}
      </Badge>
    </div>

    <slot />

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
        <LoaderCircle v-if="busy && a.key === 'publish'" class="w-4 h-4 animate-spin" />
        {{ t(a.labelKey) }}
      </Button>
    </div>
  </div>
</template>
