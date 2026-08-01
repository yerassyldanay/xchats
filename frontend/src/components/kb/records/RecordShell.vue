<script setup lang="ts">
// RecordShell is the shared chrome every kb/records/*Record.vue wraps its own
// field body in: an icon + label + <code> natural key + state badge header,
// and an action row driven by kbActions(mode) — so a live row's
// Сохранить/Удалить and a draft row's Сохранить/Принять/Отклонить are ONE
// styled button set, not seven components independently reinventing it
// (plan Task 14 — shared record components with a real diff contract).
import { computed, type Component } from 'vue'
import { LoaderCircle } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { kbActions, type KbAction } from '@/stores/playground'
import { RECORD_STATE_META, type RecordState } from './shared'

const props = withDefaults(
  defineProps<{
    icon: Component
    label: string
    recordKey?: string // omitted for a true singleton (contacts/policies/config) — no natural key to show
    mode: 'draft' | 'live'
    state: RecordState
    busy?: boolean
    singleton?: boolean
    pending?: boolean
  }>(),
  { busy: false, singleton: false, pending: true, recordKey: undefined }
)

const emit = defineEmits<{ save: []; approve: []; reject: []; delete: [] }>()

const actions = computed<KbAction[]>(() => kbActions(props.mode, { singleton: props.singleton, pending: props.pending }))
function fire(key: KbAction['key']) {
  switch (key) {
    case 'save':
      emit('save')
      break
    case 'approve':
      emit('approve')
      break
    case 'reject':
      emit('reject')
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
        <LoaderCircle v-if="busy && a.key === 'save'" class="w-4 h-4 animate-spin" />
        {{ a.label }}
      </Button>
    </div>
  </div>
</template>
