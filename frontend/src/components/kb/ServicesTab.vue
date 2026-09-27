<script setup lang="ts">
// ServicesTab is the live nested tree view (Знаний база → Услуги,
// TEST.md §4.3) — a bespoke component, not RecordList, because the shape
// (category header, base row, indented variant/addon children, specialist
// pills) doesn't fit RecordList's generic flat card list. Reads
// pg.live?.services/pg.live?.specialists directly (fall back to []); no
// pagination. The category/base/child tree is DERIVED with computed state
// (PLAN.md: "Derive service trees with computed state rather than mutating
// store rows") — pg.live.services rows are never touched here.
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { LoaderCircle, Pencil } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { useKbModal } from '@/composables/useKbModal'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import ConfirmDeleteDialog from './forms/ConfirmDeleteDialog.vue'
import type { ServiceRow } from '@/types'

const pg = usePlayground()
const modal = useKbModal()
const { t } = useI18n()

const showArchived = ref(false)
const actioningRef = ref('')
const actionError = ref('')

interface ServiceNode {
  row: ServiceRow
  children: ServiceRow[]
  // contextOnly: this base itself does NOT match the current Active/
  // Archived filter — it is shown only so an archived child has somewhere
  // to nest under (see tree's own doc comment below). Its own switch/edit
  // stay fully functional; it just isn't counted in the group's displayed
  // total, and the template flags it with a small "shown for context" note.
  contextOnly: boolean
}
interface CategoryGroup {
  category: string
  bases: ServiceNode[]
  count: number
}

function matchesStatusFilter(r: ServiceRow): boolean {
  return showArchived.value ? r.sales_status !== 'active' : r.sales_status === 'active'
}

// tree groups the currently-visible rows by category, then nests each
// variant/addon under its base (one level only, per the hierarchy
// invariant).
//
// A variant/addon can be archived on its own while its base stays active
// (restoring/archiving a single child is an explicit, independent action —
// only a BASE's own archival cascades downward, never the reverse) — so the
// Archived view can contain a child whose base does NOT itself match the
// archived filter. Earlier this only considered bases that were themselves
// in the filtered set, so that child had no base row to nest under and
// silently vanished from BOTH tabs: not in Active (it isn't active), not in
// Archived either (its base got filtered out of `bases` first). The
// reverse can't happen — validateService only allows an ACTIVE child under
// an ACTIVE base, so every active child's base is always itself active and
// already in the filtered set. contextOnlyBases below is therefore only
// ever populated in the Archived view.
const tree = computed<CategoryGroup[]>(() => {
  const allRows = pg.live?.services ?? []
  const visibleRows = allRows.filter(matchesStatusFilter)

  const childrenByParent = new Map<string, ServiceRow[]>()
  for (const r of visibleRows) {
    if (r.service_type === 'base') continue
    const list = childrenByParent.get(r.parent_ref) ?? []
    list.push(r)
    childrenByParent.set(r.parent_ref, list)
  }

  const baseByRef = new Map(allRows.filter((r) => r.service_type === 'base').map((r) => [r.ref, r]))
  const baseRefs = new Set(visibleRows.filter((r) => r.service_type === 'base').map((r) => r.ref))
  // Only the Archived view ever needs a context-only base: an active child
  // under a non-active base cannot occur through any real write path
  // (validateService requires an active child's base to be active too), so
  // deliberately not extending this fallback to the Active view keeps a
  // hand-built/corrupted state exactly as invisible as it always was there.
  if (showArchived.value) {
    for (const parentRef of childrenByParent.keys()) {
      if (!baseRefs.has(parentRef) && baseByRef.has(parentRef)) baseRefs.add(parentRef)
    }
  }

  const byCategory = new Map<string, ServiceNode[]>()
  for (const ref of baseRefs) {
    const base = baseByRef.get(ref)
    if (!base) continue
    const category = base.category || t('kb.services.noCategory')
    const list = byCategory.get(category) ?? []
    list.push({ row: base, children: childrenByParent.get(base.ref) ?? [], contextOnly: !matchesStatusFilter(base) })
    byCategory.set(category, list)
  }
  return [...byCategory.entries()].map(([category, groupBases]) => ({
    category,
    bases: groupBases,
    count: groupBases.reduce((sum, b) => sum + (b.contextOnly ? 0 : 1) + b.children.length, 0),
  }))
})

const specialistsByRef = computed(() => {
  const map = new Map<string, string>()
  for (const s of pg.live?.specialists ?? []) map.set(s.ref, s.full_name || s.ref)
  return map
})
// specialistLabel resolves a linked specialist's display name — clicking a
// pill is a deliberate no-op (a title tooltip is enough; cross-tab
// navigation is out of scope here).
function specialistLabel(ref: string): string {
  return specialistsByRef.value.get(ref) ?? ref
}

function durationText(row: ServiceRow): string {
  return row.duration == null ? '—' : t('kb.fields.durationMinutes', { n: row.duration })
}

function edit(row: ServiceRow) {
  modal.openEdit('services', row, { target: 'live' })
}

// activeChildCount counts row's currently-active variants/addons — used to
// decide whether archiving it needs a cascade warning. Zero for a
// variant/addon (they have no children of their own, per the one-level
// hierarchy) and for a base with only inactive/no children already.
function activeChildCount(row: ServiceRow): number {
  return (pg.live?.services ?? []).filter((s) => s.parent_ref === row.ref && s.sales_status === 'active').length
}

const archiveConfirmRow = ref<ServiceRow | null>(null)
const archiveConfirmCount = computed(() => (archiveConfirmRow.value ? activeChildCount(archiveConfirmRow.value) : 0))

// toggleStatus archives/restores immediately for every case EXCEPT the one
// with real, surprising blast radius: archiving an active base that still
// has active children. PutLiveService (backend) cascades that archive to
// every active child atomically and immediately — no undo — so it gets a
// confirmation naming exactly what will also be archived, instead of firing
// on the very first click the way every other toggle here still does.
async function toggleStatus(row: ServiceRow) {
  if (row.service_type === 'base' && row.sales_status === 'active' && activeChildCount(row) > 0) {
    archiveConfirmRow.value = row
    return
  }
  await applyStatusToggle(row)
}

async function applyStatusToggle(row: ServiceRow) {
  actionError.value = ''
  actioningRef.value = row.ref
  try {
    await pg.setServiceStatus(row.ref, row.sales_status === 'active' ? 'inactive' : 'active')
  } catch (e) {
    actionError.value = e instanceof ApiError ? e.message : t('kb.draft.errSaveChange')
  } finally {
    actioningRef.value = ''
  }
}

async function confirmCascadeArchive() {
  const row = archiveConfirmRow.value
  archiveConfirmRow.value = null
  if (row) await applyStatusToggle(row)
}
</script>

<template>
  <div class="space-y-3" data-testid="services-tab">
    <div class="flex items-center justify-between gap-2 flex-wrap">
      <div class="flex items-center gap-1 rounded-lg border border-border p-0.5">
        <button
          type="button"
          class="rounded-md px-3 py-1.5 text-xs font-medium transition"
          :class="!showArchived ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'"
          data-testid="services-filter-active"
          @click="showArchived = false"
        >
          {{ t('kb.archive.filterActive') }}
        </button>
        <button
          type="button"
          class="rounded-md px-3 py-1.5 text-xs font-medium transition"
          :class="showArchived ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'"
          data-testid="services-filter-archived"
          @click="showArchived = true"
        >
          {{ t('kb.archive.filterArchived') }}
        </button>
      </div>
    </div>

    <p v-if="actionError" class="text-sm text-destructive">{{ actionError }}</p>

    <p v-if="!tree.length" class="text-sm text-muted-foreground py-6 text-center">{{ t('kb.page.emptyServices') }}</p>

    <div v-for="group in tree" :key="group.category" class="rounded-lg border border-border overflow-hidden" data-testid="service-category-group">
      <div class="bg-muted/40 px-3 py-2 text-xs font-semibold text-muted-foreground flex items-center gap-1.5">
        {{ group.category }}
        <span class="font-normal text-muted-foreground/70" data-testid="service-category-count">· {{ t('kb.services.categoryCount', { n: group.count }) }}</span>
      </div>

      <div
        v-for="node in group.bases"
        :key="node.row.ref"
        class="border-t border-border first:border-t-0"
        data-testid="service-base-row"
        :data-context-only="node.contextOnly || undefined"
      >
        <div class="flex items-center gap-3 px-3 py-2.5 flex-wrap">
          <div class="flex-1 min-w-[160px]">
            <div class="flex items-center gap-1.5 flex-wrap">
              <p class="font-medium text-sm">{{ node.row.name || node.row.ref }}</p>
              <Badge
                v-if="node.contextOnly"
                variant="outline"
                class="text-[10px]"
                data-testid="service-base-context-badge"
                :title="t('kb.services.shownForContextTitle')"
              >
                {{ t('kb.services.shownForContext') }}
              </Badge>
            </div>
            <div v-if="node.row.specialist_refs.length" class="flex flex-wrap gap-1 mt-1">
              <Badge v-for="ref in node.row.specialist_refs" :key="ref" variant="secondary" class="text-[11px]" :title="ref">
                {{ specialistLabel(ref) }}
              </Badge>
            </div>
          </div>
          <span class="text-sm font-mono w-20 text-right shrink-0">{{ node.row.price || '—' }}</span>
          <span class="text-xs text-muted-foreground font-mono w-16 text-right shrink-0">{{ durationText(node.row) }}</span>
          <div class="flex items-center gap-1.5 shrink-0">
            <Switch
              :model-value="node.row.sales_status === 'active'"
              :disabled="actioningRef === node.row.ref"
              :aria-label="t('kb.archive.toggleAria')"
              :data-testid="`service-status-switch-${node.row.ref}`"
              @update:model-value="() => toggleStatus(node.row)"
            />
            <LoaderCircle v-if="actioningRef === node.row.ref" class="w-3.5 h-3.5 animate-spin text-muted-foreground" />
          </div>
          <Button variant="outline" size="sm" class="shrink-0" :data-testid="`service-edit-${node.row.ref}`" @click="edit(node.row)">
            <Pencil class="w-3.5 h-3.5" /> {{ t('kb.actions.edit') }}
          </Button>
        </div>

        <div
          v-for="(child, idx) in node.children"
          :key="child.ref"
          class="flex items-center gap-2 pl-4 pr-3 py-2 border-t border-border/60 bg-muted/10 flex-wrap"
          data-testid="service-child-row"
        >
          <span class="font-mono text-xs text-muted-foreground/50 shrink-0 select-none" aria-hidden="true">
            {{ idx === node.children.length - 1 ? '└─' : '├─' }}
          </span>
          <div class="flex-1 min-w-[160px]">
            <div class="flex items-center gap-1.5 flex-wrap">
              <Badge variant="outline" class="text-[10px]">{{ t('kb.serviceType.' + child.service_type) }}</Badge>
              <p class="text-sm">{{ child.name || child.ref }}</p>
              <Badge
                v-if="child.service_type === 'addon'"
                variant="outline"
                class="border-amber-300 text-amber-700 text-[10px]"
                :title="t('kb.services.addonNotStandalone')"
              >
                {{ t('kb.services.addonNotStandalone') }}
              </Badge>
            </div>
            <div v-if="child.specialist_refs.length" class="flex flex-wrap gap-1 mt-1">
              <Badge v-for="ref in child.specialist_refs" :key="ref" variant="secondary" class="text-[11px]" :title="ref">
                {{ specialistLabel(ref) }}
              </Badge>
            </div>
          </div>
          <span class="text-sm font-mono w-20 text-right shrink-0">{{ child.price || '—' }}</span>
          <span class="text-xs text-muted-foreground font-mono w-16 text-right shrink-0">{{ durationText(child) }}</span>
          <div class="flex items-center gap-1.5 shrink-0">
            <Switch
              :model-value="child.sales_status === 'active'"
              :disabled="actioningRef === child.ref"
              :aria-label="t('kb.archive.toggleAria')"
              :data-testid="`service-status-switch-${child.ref}`"
              @update:model-value="() => toggleStatus(child)"
            />
            <LoaderCircle v-if="actioningRef === child.ref" class="w-3.5 h-3.5 animate-spin text-muted-foreground" />
          </div>
          <Button variant="outline" size="sm" class="shrink-0" :data-testid="`service-edit-${child.ref}`" @click="edit(child)">
            <Pencil class="w-3.5 h-3.5" /> {{ t('kb.actions.edit') }}
          </Button>
        </div>
      </div>
    </div>

    <ConfirmDeleteDialog
      :open="archiveConfirmRow !== null"
      :busy="actioningRef === archiveConfirmRow?.ref"
      title-key="kb.services.archiveCascadeConfirm.title"
      body-key="kb.services.archiveCascadeConfirm.body"
      :body-params="{ name: archiveConfirmRow?.name || archiveConfirmRow?.ref, count: archiveConfirmCount }"
      confirm-key="kb.services.archiveCascadeConfirm.confirm"
      @update:open="(v) => !v && (archiveConfirmRow = null)"
      @confirm="confirmCascadeArchive"
    />
  </div>
</template>
