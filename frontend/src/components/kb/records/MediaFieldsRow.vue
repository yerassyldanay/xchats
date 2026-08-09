<script setup lang="ts">
// MediaFieldsRow is the shared layout every kb/records/*Record.vue arranges
// its MediaStrip children in — was a repeated `flex flex-col gap-2` full-
// width stack (one field per line, even when several sit empty); flex-wrap
// lets compact/empty strips flow side by side instead, so an all-empty
// media section reads as one quiet line rather than a wall of rows.
</script>

<template>
  <div class="flex flex-wrap gap-x-6 gap-y-3 border-t border-border/70 pt-4">
    <slot />
  </div>
</template>
