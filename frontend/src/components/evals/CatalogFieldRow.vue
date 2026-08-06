<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleHelp } from 'lucide-vue-next'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

// One raw tests.yaml field, shown as its actual key (never a renamed label)
// with a (?) tooltip carrying its evaluator-role explanation: this page
// shows exact schema fields for a developer audience, not friendly
// abstractions. Requires an ancestor <TooltipProvider> (EvalCatalog.vue
// provides one for the whole page).
const props = defineProps<{ fieldKey: string }>()
const { t, te } = useI18n()

// te() checks the RU locale (the fallback and source of truth) — a key legitimately
// undocumented simply renders without the (?) icon rather than an empty tooltip.
const hasDoc = computed(() => te(`evalCatalog.fields.${props.fieldKey}`, 'ru'))
</script>

<template>
  <div>
    <div class="flex items-center gap-1 mb-1">
      <code class="font-mono text-xs font-semibold">{{ fieldKey }}</code>
      <Tooltip v-if="hasDoc">
        <TooltipTrigger as-child>
          <button type="button" class="text-muted-foreground hover:text-foreground transition" :aria-label="t(`evalCatalog.fields.${fieldKey}`)">
            <CircleHelp class="w-3.5 h-3.5" />
          </button>
        </TooltipTrigger>
        <TooltipContent side="right" class="max-w-xs">{{ t(`evalCatalog.fields.${fieldKey}`) }}</TooltipContent>
      </Tooltip>
    </div>
    <slot />
  </div>
</template>
