<script setup lang="ts">
// ProductCard is the Знаний база product tile: a square photo with a stock pill
// laid over it, then name, a two-line description and the price. On
// /knowledge-base it shows only what the published row says; a pending draft
// change is a MARK (see RecordShell's pendingMark contract), never the draft
// value, and Изменить / Удалить fade in on hover/focus.
//
// Черновик reuses the same tile for a staged product (pass `changeType`): it
// shows the DRAFT row, a Новый / Изменён / На удаление pill, the before/after
// caption under every changed field, a multi-select checkbox, and the review
// actions (Изменить / Опубликовать / Отменить) as always-visible buttons —
// reviewing is that page's whole job, so nothing hides behind a hover.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { LoaderCircle, Package } from 'lucide-vue-next'
import type { ProductRow } from '@/types'
import type { ChangeType } from '@/composables/draftChanges'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import type { KbAction, KbActionKey } from './actions'
import FieldDiffNote from './FieldDiffNote.vue'
import { changedFields, materialContentURL, mediaIds, RECORD_STATE_META, stateForChange } from './shared'

const props = defineProps<{
  row: ProductRow
  liveRow?: ProductRow
  changeType?: ChangeType
  pendingMark?: 'updated' | 'removed'
  actions: KbAction[]
  busy?: boolean
  blockedNote?: string
  selectable?: boolean
  selected?: boolean
}>()
const emit = defineEmits<{ edit: []; delete: []; publish: []; cancel: []; 'toggle-select': [] }>()
const { t } = useI18n()

// Featured image first, else the first gallery image.
const photoId = computed(() => mediaIds(props.row.featured_image)[0] ?? mediaIds(props.row.gallery_images)[0])
const STOCK_TONE: Record<string, string> = {
  in_stock: 'text-success',
  preorder: 'text-warning',
  on_demand: 'text-primary',
  unavailable: 'text-destructive',
}
const stockLabel = computed(() => t('kb.availabilityStatus.' + props.row.availability_status))
const canEdit = computed(() => props.actions.some((a) => a.key === 'edit'))
const canDelete = computed(() => props.actions.some((a) => a.key === 'delete'))

const reviewing = computed(() => !!props.changeType)
const state = computed(() => (props.changeType ? stateForChange(props.changeType) : 'published'))
const STATE_PILL: Record<string, string> = {
  new: 'bg-success-soft text-success',
  changed: 'bg-warning-soft text-warning',
  to_delete: 'bg-destructive/10 text-destructive',
}
const diff = computed(() => changedFields(props.row, props.liveRow, ['name', 'price', 'description', 'availability_status']))

function fire(key: KbActionKey) {
  if (key === 'edit') emit('edit')
  else if (key === 'delete') emit('delete')
  else if (key === 'publish') emit('publish')
  else if (key === 'cancel') emit('cancel')
}
</script>

<template>
  <article
    class="group relative"
    :class="cn(reviewing && state === 'to_delete' && 'opacity-80')"
    :data-testid="reviewing ? 'kb-record' : 'kb-product-card'"
  >
    <div
      class="relative aspect-square overflow-hidden rounded-2xl bg-secondary"
      :class="reviewing && selectable && selected ? 'ring-2 ring-primary ring-offset-2' : ''"
    >
      <img
        v-if="photoId"
        :src="materialContentURL(photoId)"
        :alt="row.name"
        loading="lazy"
        decoding="async"
        class="size-full object-cover"
      />
      <div v-else class="grid size-full place-items-center text-muted-foreground"><Package class="size-10" /></div>

      <span
        class="absolute left-3 top-3 rounded-full bg-background/95 px-2.5 py-1 text-xs font-bold shadow-[0_1px_2px_rgba(0,0,0,0.12)]"
        :class="STOCK_TONE[row.availability_status] ?? 'text-muted-foreground'"
      >
        {{ stockLabel }}
      </span>
      <span
        v-if="reviewing"
        class="absolute right-3 top-3 rounded-full px-2.5 py-1 text-xs font-bold shadow-[0_1px_2px_rgba(0,0,0,0.12)]"
        :class="STATE_PILL[state]"
        data-testid="kb-record-state"
      >
        {{ t(RECORD_STATE_META[state].labelKey) }}
      </span>
      <span
        v-else-if="pendingMark"
        class="absolute right-3 top-3 rounded-full bg-warning-soft px-2.5 py-1 text-xs font-bold text-warning"
        :title="t('kb.pendingMark.' + pendingMark)"
      >
        {{ t('kb.pendingMark.' + pendingMark) }}
      </span>

      <label
        v-if="reviewing && selectable"
        class="absolute bottom-3 left-3 grid size-8 cursor-pointer place-items-center rounded-full bg-background/95 shadow-[0_1px_2px_rgba(0,0,0,0.12)]"
      >
        <input
          type="checkbox"
          class="size-4 cursor-pointer accent-primary"
          :checked="selected"
          :disabled="busy"
          :aria-label="t('kb.draft.selection.selectRecord')"
          data-testid="kb-record-select"
          @change="emit('toggle-select')"
        />
      </label>

      <div
        v-if="!reviewing && (canEdit || canDelete)"
        :class="
          cn(
            'absolute inset-x-3 bottom-3 flex gap-2 transition-opacity',
            'opacity-0 group-focus-within:opacity-100 group-hover:opacity-100 [@media(hover:none)]:opacity-100',
          )
        "
      >
        <Button v-if="canEdit" size="sm" variant="outline" class="flex-1 bg-background" :disabled="busy" @click="emit('edit')">
          {{ t('kb.actions.edit') }}
        </Button>
        <Button v-if="canDelete" size="sm" variant="outline" class="bg-background text-destructive" :disabled="busy" @click="emit('delete')">
          {{ t('kb.actions.delete') }}
        </Button>
      </div>
    </div>

    <h3 class="mt-3 text-[17px] font-bold leading-snug" :class="reviewing && state === 'to_delete' ? 'line-through' : ''">{{ row.name || '—' }}</h3>
    <p class="mt-1 line-clamp-2 text-sm leading-snug text-muted-foreground">{{ row.description }}</p>
    <p class="mt-1.5 text-[17px] font-bold">{{ row.price || '—' }}</p>
    <!-- before/after captions, only on a staged change that has a published original -->
    <div v-if="diff.length" class="mt-1.5 space-y-1 [&>*]:mt-0">
      <FieldDiffNote :show="diff.includes('name')" :was="liveRow?.name ?? ''" :now="row.name" />
      <FieldDiffNote :show="diff.includes('price')" :was="liveRow?.price ?? ''" :now="row.price" />
      <FieldDiffNote
        :show="diff.includes('availability_status')"
        :was="liveRow ? t('kb.availabilityStatus.' + liveRow.availability_status) : ''"
        :now="stockLabel"
      />
      <FieldDiffNote :show="diff.includes('description')" :was="liveRow?.description ?? ''" :now="row.description" />
    </div>

    <template v-if="reviewing">
      <p v-if="blockedNote" class="mt-2 text-xs italic text-muted-foreground">{{ blockedNote }}</p>
      <div class="mt-3 flex flex-wrap items-center gap-2">
        <Button
          v-for="a in actions"
          :key="a.key"
          size="sm"
          :variant="a.variant"
          :class="a.destructive ? 'text-destructive' : ''"
          :disabled="busy"
          @click="fire(a.key)"
        >
          <LoaderCircle v-if="busy && a.key === 'publish'" class="size-4 animate-spin" />
          {{ t(a.labelKey) }}
        </Button>
      </div>
    </template>
  </article>
</template>
