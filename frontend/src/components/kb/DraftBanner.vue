<script setup lang="ts">
// DraftBanner is База знаний's persistent success banner after a stage — it
// deliberately does NOT refetch `live` (nothing published changed); it just
// points at Черновик to review/publish it.
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { CircleCheck } from 'lucide-vue-next'

const props = defineProps<{ show: boolean; kindLabel?: string }>()
const { t } = useI18n()
</script>

<template>
  <div
    v-if="show"
    class="rounded-xl border border-emerald-300/60 bg-emerald-50 px-4 py-3 text-sm text-emerald-950 flex items-center gap-2 flex-wrap"
  >
    <CircleCheck class="w-4 h-4 shrink-0" />
    <span>{{ t('kb.banner.staged', { kind: props.kindLabel ?? '' }) }}</span>
    <RouterLink :to="{ name: 'playground' }" class="font-semibold underline underline-offset-2 ml-auto">
      {{ t('kb.banner.goToDraft') }}
    </RouterLink>
  </div>
</template>
