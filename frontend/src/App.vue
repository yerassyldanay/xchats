<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, RouterView } from 'vue-router'
import NavRail from './components/NavRail.vue'

const route = useRoute()
// The rail shows on every authenticated page and never on login.
const showRail = computed(() => !!route.meta.requiresAuth)
</script>

<template>
  <div class="flex h-full">
    <NavRail v-if="showRail" />
    <div class="flex-1 min-w-0 h-full">
      <RouterView />
    </div>
  </div>
</template>
