<script setup lang="ts">
// RecordList renders one kind's PUBLISHED rows on Знаний база — the
// topics/products/tariffs three, which share an identical card prop shape.
// delivery_zones/contacts/policies are special-cased directly in
// KnowledgeBase.vue, same split as ChangeList.vue/DraftKnowledgeBase.vue.
// Each row's badge picks up a pending MARK (never the pending value itself)
// from usePendingIndex — a reviewer sees "this has an unpublished change"
// without this page turning into a second draft editor.
import { computed } from 'vue'
import { usePlayground } from '@/stores/playground'
import { usePendingIndex } from '@/composables/usePendingIndex'
import { useKbModal } from '@/composables/useKbModal'
import type { ProductRow, TariffRow, TopicRow } from '@/types'
import { kbActions } from './records/actions'
import TopicRecord from './records/TopicRecord.vue'
import ProductRecord from './records/ProductRecord.vue'
import TariffRecord from './records/TariffRecord.vue'

const props = defineProps<{ kind: 'topics' | 'products' | 'tariffs' }>()
const emit = defineEmits<{ delete: [key: string] }>()

const pg = usePlayground()
const { markFor } = usePendingIndex()
const modal = useKbModal()

const COMPONENTS = { topics: TopicRecord, products: ProductRecord, tariffs: TariffRecord }
const component = computed(() => COMPONENTS[props.kind])
const rows = computed(() => pg.live?.[props.kind] ?? [])
const actions = kbActions({ page: 'live' })

function edit(row: TopicRow | ProductRow | TariffRow) {
  modal.openEdit(props.kind, row)
}

// asRow exists for ONE reason: <component :is> can't let TypeScript narrow
// a prop type by the runtime `kind` match the way a static
// <TopicRecord v-if=...> chain could — see ChangeList.vue's identical note.
function asRow(row: TopicRow | ProductRow | TariffRow) {
  return row as any
}
</script>

<template>
  <component
    :is="component"
    v-for="row in rows"
    :key="row.id"
    :row="asRow(row)"
    :pending-mark="markFor(kind, row.id)"
    :actions="actions"
    :busy="pg.busy"
    @edit="edit(row)"
    @delete="emit('delete', row.id)"
  />
</template>
