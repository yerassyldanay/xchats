<script setup lang="ts">
// KbEmptyTab is the inline "nothing here yet" row for one Знаний база tab —
// lighter than DraftEmptyState (a whole-page empty state), but still an
// icon + message + direct action instead of a lone sentence, so an empty
// tab still tells the operator what to do next rather than just stopping.
import { type Component } from 'vue'
import { Plus } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

withDefaults(defineProps<{ icon: Component; message: string; actionLabel?: string }>(), { actionLabel: undefined })
defineEmits<{ action: [] }>()
</script>

<template>
  <div class="flex flex-col items-center gap-3 rounded-xl border border-dashed border-border py-12 text-center">
    <div class="grid h-10 w-10 place-items-center rounded-lg bg-muted text-muted-foreground">
      <component :is="icon" class="h-5 w-5" />
    </div>
    <p class="max-w-xs text-sm text-muted-foreground">{{ message }}</p>
    <Button v-if="actionLabel" size="sm" variant="outline" @click="$emit('action')">
      <Plus class="h-4 w-4" /> {{ actionLabel }}
    </Button>
  </div>
</template>
