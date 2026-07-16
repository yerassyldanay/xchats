<script setup lang="ts">
import { useRoute } from 'vue-router'

// Shared «Запуски | Каталог тестов» switcher — used in both EvalRuns.vue's and
// EvalCatalog.vue's header. Plain router-links styled as tabs (not the shadcn Tabs
// component): the two pages are separate ROUTES, not tab-panels of one component.
const route = useRoute()
</script>

<template>
  <nav class="flex items-center gap-1 mt-3">
    <router-link
      :to="{ name: 'evals' }"
      class="text-sm px-3 py-1.5 rounded-lg transition"
      :class="route.name === 'evals' ? 'bg-primary/10 text-primary font-medium' : 'text-muted-foreground hover:text-foreground hover:bg-muted'"
    >
      Запуски
    </router-link>
    <router-link
      :to="{ name: 'eval-catalog' }"
      class="text-sm px-3 py-1.5 rounded-lg transition"
      :class="route.name === 'eval-catalog' ? 'bg-primary/10 text-primary font-medium' : 'text-muted-foreground hover:text-foreground hover:bg-muted'"
    >
      Каталог тестов
    </router-link>
  </nav>
</template>
