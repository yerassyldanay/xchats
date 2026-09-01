<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'

// CAM-11: the campaigns list and the detail page's Recipients/History tabs
// all stop at whatever the current page holds, with totals the API already
// returns but no way to reach anything past the first page. Deliberately
// Prev/Next only, never one button per page (contrast EvalRuns.vue's own
// numbered pagination, sized for a small, bounded list of eval launches) —
// a campaign's own recipient or event history can run into the thousands.
const props = defineProps<{ page: number; pageSize: number; total: number }>()
const emit = defineEmits<{ 'update:page': [page: number] }>()

const { t } = useI18n()

const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))
const rangeStart = computed(() => (props.total === 0 ? 0 : (props.page - 1) * props.pageSize + 1))
const rangeEnd = computed(() => Math.min(props.page * props.pageSize, props.total))
</script>

<template>
  <div v-if="total > 0" class="flex items-center justify-between gap-4 text-xs text-muted-foreground" data-testid="pagination">
    <p>{{ t('common.pageRange', { from: rangeStart, to: rangeEnd, total }) }}</p>
    <div v-if="totalPages > 1" class="flex items-center gap-1">
      <button
        type="button"
        class="w-7 h-7 rounded-lg border border-border grid place-items-center disabled:opacity-40 hover:bg-muted transition"
        :disabled="page <= 1"
        :aria-label="t('common.prevPage')"
        @click="emit('update:page', page - 1)"
      >
        <ChevronLeft class="w-3.5 h-3.5" />
      </button>
      <button
        type="button"
        class="w-7 h-7 rounded-lg border border-border grid place-items-center disabled:opacity-40 hover:bg-muted transition"
        :disabled="page >= totalPages"
        :aria-label="t('common.nextPage')"
        @click="emit('update:page', page + 1)"
      >
        <ChevronRight class="w-3.5 h-3.5" />
      </button>
    </div>
  </div>
</template>
