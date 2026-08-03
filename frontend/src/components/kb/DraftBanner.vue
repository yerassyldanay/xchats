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
  <div v-if="show" class="rounded-xl border border-emerald-300/60 bg-emerald-50 px-4 py-3 text-sm text-emerald-950 flex items-center gap-3">
    <CheckCircle2 class="w-5 h-5 shrink-0 text-emerald-600" />
    <span class="flex-1">{{ t('kb.banner.staged') }}</span>
    <RouterLink :to="{ name: 'playground' }" class="font-medium underline underline-offset-2 shrink-0">
      {{ t('kb.banner.viewDraft') }}
    </RouterLink>
    <button type="button" class="shrink-0 text-emerald-700/70 hover:text-emerald-900 transition" @click="emit('close')">
      <X class="w-4 h-4" />
    </button>
  </div>
</template>
