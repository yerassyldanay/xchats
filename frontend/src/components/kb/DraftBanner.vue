<script setup lang="ts">
// DraftBanner is Знаний база's transient "saved" confirmation after a
// manual write — every write on this page commits straight to the live KB
// (KB-13), so this just confirms the record was saved; the page's own
// bannerVisible/flashSaved (KnowledgeBase.vue) auto-hides it after a few
// seconds, the close button just dismisses it early.
import { useI18n } from 'vue-i18n'
import { CheckCircle2, X } from 'lucide-vue-next'

defineProps<{ show: boolean }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
</script>

<template>
  <div v-if="show" class="rounded-xl border border-emerald-300/60 bg-emerald-50 px-4 py-3 text-sm text-emerald-950 flex items-center gap-3">
    <CheckCircle2 class="w-5 h-5 shrink-0 text-emerald-600" />
    <span class="flex-1">{{ t('kb.banner.saved') }}</span>
    <button type="button" class="shrink-0 text-emerald-700/70 hover:text-emerald-900 transition" @click="emit('close')">
      <X class="w-4 h-4" />
    </button>
  </div>
</template>
