<script setup lang="ts">
// DraftBanner is Знаний база's persistent success notice after a staged
// write — pg.live is deliberately NOT refetched when it appears (nothing
// live changed; the write landed in the draft), so the published list stays
// visibly unchanged while this banner points at where the change actually
// went.
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { CheckCircle2, X } from 'lucide-vue-next'

defineProps<{ show: boolean }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
</script>

<template>
  <div
    v-if="show"
    class="flex items-center gap-3 rounded-xl border border-emerald-300/60 bg-emerald-50 px-4 py-3 text-sm text-emerald-950 dark:border-emerald-400/25 dark:bg-emerald-500/10 dark:text-emerald-200"
  >
    <CheckCircle2 class="h-5 w-5 shrink-0 text-emerald-600 dark:text-emerald-400" />
    <span class="flex-1">{{ t('kb.banner.staged') }}</span>
    <RouterLink :to="{ name: 'playground' }" class="shrink-0 font-medium underline underline-offset-2">
      {{ t('kb.banner.viewDraft') }}
    </RouterLink>
    <button type="button" class="shrink-0 text-emerald-700/70 transition hover:text-emerald-900 dark:text-emerald-300/70 dark:hover:text-emerald-100" @click="emit('close')">
      <X class="h-4 w-4" />
    </button>
  </div>
</template>
