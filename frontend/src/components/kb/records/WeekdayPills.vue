<script setup lang="ts">
// WeekdayPills is the compact 7-dot weekday indicator shared by
// SpecialistRecord.vue (draft review) and SpecialistsTab.vue (live roster)
// — TEST.md §4.2: worked days in an accent color, off days greyed out.
// Pure props-in, no store access, same convention as the rest of kb/records.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { WEEKDAY_ORDER } from '@/components/kb/kbEntities'
import type { Schedule } from '@/types'

const props = defineProps<{ schedule: Schedule }>()
const { t } = useI18n()

const workedRefs = computed(() => new Set(props.schedule.map((d) => d.ref)))
</script>

<template>
  <div class="flex items-center gap-1" data-testid="weekday-pills">
    <span
      v-for="ref in WEEKDAY_ORDER"
      :key="ref"
      class="inline-flex items-center justify-center w-6 h-6 rounded-full text-[11px] font-medium shrink-0"
      :class="workedRefs.has(ref) ? 'bg-primary/15 text-primary' : 'bg-muted text-muted-foreground/50'"
      :title="t('kb.schedule.weekday.' + ref)"
      :data-testid="`weekday-pill-${ref}`"
    >
      {{ t('kb.schedule.weekdayShort.' + ref) }}
    </span>
  </div>
</template>
