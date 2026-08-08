<script setup lang="ts">
// MediaLightbox is MediaStrip's full-size image viewer — a plain <img> in a
// wider Dialog, no pan/zoom/carousel. One instance per MediaStrip; reka-ui's
// Dialog only mounts its portal content while open, so an unopened lightbox
// costs nothing.
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

defineProps<{ open: boolean; src: string | null; label: string }>()
const emit = defineEmits<{ 'update:open': [boolean] }>()
</script>

<template>
  <Dialog :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogContent class="max-w-3xl">
      <DialogHeader>
        <DialogTitle class="truncate">{{ label }}</DialogTitle>
      </DialogHeader>
      <div class="p-5 grid place-items-center bg-muted/30">
        <img v-if="src" :src="src" :alt="label" class="max-w-full max-h-[75vh] object-contain rounded-md" />
      </div>
    </DialogContent>
  </Dialog>
</template>
