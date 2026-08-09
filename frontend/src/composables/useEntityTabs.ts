import { computed, ref, watch, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { ENTITY_META, KB_ENTITY_ORDER, KB_TAB_GROUP_OF, rowsOf, type KbTabGroup } from '@/components/kb/kbEntities'
import { usePlayground } from '@/stores/playground'
import { useDraftChanges } from './useDraftChanges'

export interface KbTab {
  key: string
  label: string
  icon: Component
  count?: number
  group?: KbTabGroup // 'live' only — see KB_TAB_GROUP_OF's own doc comment
}

// useEntityTabs builds the pill tab row for both pages:
//   - source 'draft' (Черновик): only kinds with pending entries, each
//     showing its total change count — a dynamic list. When the active tab's
//     kind runs out of entries (its last change was published or cancelled),
//     falls back to the first remaining tab automatically.
//   - source 'live' (Знаний база): the fixed seven kinds in KB_ENTITY_ORDER,
//     no counts, plus whatever `extra` tabs the page appends (Промпт/Файлы),
//     each tagged with its display group.
export function useEntityTabs(opts: { source: 'draft' | 'live'; extra?: KbTab[] }) {
  const { t } = useI18n()
  const { groups } = useDraftChanges()
  const pg = usePlayground()

  const tabs = computed<KbTab[]>(() => {
    const base: KbTab[] =
      opts.source === 'draft'
        ? groups.value.map((g) => ({
            key: g.kind,
            label: t(`${ENTITY_META[g.kind].i18nKey}.plural`),
            icon: ENTITY_META[g.kind].icon,
            count: g.counts.total,
          }))
        : // 'live' (Знаний база): a count straight off the published rows this
          // page already loaded — config has no natural row count (it is one
          // fixed identity, not a list) and is left uncounted.
          KB_ENTITY_ORDER.map((kind) => ({
            key: kind,
            label: t(`${ENTITY_META[kind].i18nKey}.plural`),
            icon: ENTITY_META[kind].icon,
            count: kind === 'config' ? undefined : rowsOf(kind, pg.live).length,
            group: KB_TAB_GROUP_OF[kind],
          }))
    const extra = (opts.extra ?? []).map((tab) => (opts.source === 'live' ? { ...tab, group: KB_TAB_GROUP_OF[tab.key] } : tab))
    return [...base, ...extra]
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
