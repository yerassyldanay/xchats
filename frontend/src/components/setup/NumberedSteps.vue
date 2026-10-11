<script setup lang="ts">
// Ordered how-to list: ink-filled numbered discs beside the instruction text.
defineProps<{ items: string[]; gap?: 'sm' | 'md' | 'lg' }>()
</script>

<template>
  <ol class="m-0 flex list-none flex-col p-0" :class="gap === 'sm' ? 'gap-3.5' : gap === 'lg' ? 'gap-[18px]' : 'gap-4'">
    <li v-for="(item, i) in items" :key="i" class="flex gap-3.5 text-base leading-[1.45]">
      <span class="grid size-7 flex-none place-items-center rounded-full bg-foreground text-[13px] font-bold text-background">
        {{ i + 1 }}
      </span>
      <span class="min-w-0 pt-0.5">{{ item }}</span>
    </li>
  </ol>
</template>
