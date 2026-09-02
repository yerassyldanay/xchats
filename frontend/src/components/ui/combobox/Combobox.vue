<script setup lang="ts">
// A minimal searchable combobox: a plain text input (so typing a custom
// value — one not in `options` — always just works, per TODO.md's "allow
// operators to enter custom model names") with a filtered dropdown of
// suggestions layered on top. Deliberately not built on a Combobox
// primitive from reka-ui: the library exposes Select/Dialog/DropdownMenu/
// Tabs/Tooltip/Avatar (all used elsewhere in this app) but no Combobox part,
// so this follows the same "wrap a styled div + Input" shape those
// hand-rolled patterns (e.g. FollowupDialog.vue's customer picker) already
// use rather than reaching for a part that doesn't exist.
import { computed, ref } from 'vue'
import { onClickOutside } from '@vueuse/core'
import { Check } from 'lucide-vue-next'
import { cn } from '@/lib/utils'
import { Input } from '@/components/ui/input'

const props = defineProps<{
  modelValue: string
  options: string[]
  placeholder?: string
  class?: string
}>()
const emit = defineEmits<{ (e: 'update:modelValue', v: string): void }>()
// Passthrough attrs (data-testid, etc.) belong on the actual <input> a test
// or a11y tool would look for, not the positioning wrapper around it.
defineOptions({ inheritAttrs: false })

const root = ref<HTMLElement | null>(null)
const open = ref(false)

const filtered = computed(() => {
  const q = props.modelValue.trim().toLowerCase()
  if (!q) return props.options
  return props.options.filter((o) => o.toLowerCase().includes(q))
})

function onInput(v: string | number) {
  emit('update:modelValue', String(v))
  open.value = true
}
function pick(o: string) {
  emit('update:modelValue', o)
  open.value = false
}

onClickOutside(root, () => {
  open.value = false
})
</script>

<template>
  <div ref="root" class="relative">
    <Input
      v-bind="$attrs"
      :model-value="modelValue"
      :placeholder="placeholder"
      :class="cn('font-mono', props.class)"
      autocomplete="off"
      role="combobox"
      :aria-expanded="open"
      @update:model-value="onInput"
      @focus="open = true"
      @keydown.escape="open = false"
      @keydown.enter="open = false"
    />
    <div
      v-if="open && filtered.length"
      class="absolute z-50 mt-1 max-h-56 w-full overflow-y-auto rounded-md border border-border bg-popover p-1 text-popover-foreground shadow-pop"
    >
      <button
        v-for="o in filtered"
        :key="o"
        type="button"
        class="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm transition hover:bg-muted"
        @click="pick(o)"
      >
        <Check class="w-3.5 h-3.5 shrink-0" :class="o === modelValue ? 'opacity-100' : 'opacity-0'" />
        <span class="truncate font-mono">{{ o }}</span>
      </button>
    </div>
  </div>
</template>
