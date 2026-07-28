<script setup lang="ts">
// Every save area on /knowledge-base ends with this pair: a working
// «Сохранить» (writes to the live tables via /kb/*) and a visible-but-disabled
// «В черновик» — draft save is a later milestone (see plan "Future Work");
// showing it now (disabled, tooltipped) previews where it lands rather than
// hiding the concept entirely.
import { FileEdit, LoaderCircle, Save } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'

withDefaults(defineProps<{ busy?: boolean; disabled?: boolean }>(), { busy: false, disabled: false })
const emit = defineEmits<{ save: [] }>()
</script>

<template>
  <div class="flex items-center gap-2">
    <Button size="sm" :disabled="busy || disabled" @click="emit('save')">
      <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" /><Save v-else class="w-4 h-4" /> Сохранить
    </Button>
    <TooltipProvider :delay-duration="200">
      <Tooltip>
        <TooltipTrigger as-child>
          <span class="inline-flex" tabindex="0">
            <Button variant="outline" size="sm" disabled class="pointer-events-none">
              <FileEdit class="w-4 h-4" /> В черновик
            </Button>
          </span>
        </TooltipTrigger>
        <TooltipContent>скоро</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  </div>
</template>
