// useEntityTabs drives both pages' tab row: Черновик shows only kinds with
// at least one pending entry (source: 'draft'); База знаний always shows
// every kind (source: 'live'), plus its own non-entity tabs (Промпт/Файлы)
// via `extra`.
import { computed, ref, watch, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { ENTITY_META, KB_ENTITY_ORDER } from '@/components/kb/kbEntities'
import { useDraftChanges } from './useDraftChanges'

export interface KbTab {
  key: string
  label: string
  icon: Component
  count?: number
}

export function useEntityTabs(opts: { source: 'draft' | 'live'; extra?: KbTab[] }) {
  const { t } = useI18n()
  const { groups } = useDraftChanges()

  const tabs = computed<KbTab[]>(() => {
    const entityTabs: KbTab[] =
      opts.source === 'draft'
        ? groups.value.map((g) => ({ key: g.kind, label: t(ENTITY_META[g.kind].labelKey), icon: ENTITY_META[g.kind].icon, count: g.counts.total }))
        : KB_ENTITY_ORDER.map((kind) => ({ key: kind, label: t(ENTITY_META[kind].labelKey), icon: ENTITY_META[kind].icon }))
    return [...entityTabs, ...(opts.extra ?? [])]
  })

  const active = ref('')
  watch(
    tabs,
    (list) => {
      if (!list.some((tab) => tab.key === active.value)) {
        active.value = list[0]?.key ?? ''
      }
    },
    { immediate: true }
  )

  return { tabs, active }
}
