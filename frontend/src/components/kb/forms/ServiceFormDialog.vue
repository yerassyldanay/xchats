<script setup lang="ts">
// ServiceFormDialog is the ONE create/edit dialog for a service (base, or a
// variant/addon nested under a base) — used both by the live
// ServicesTab.vue (modal.openCreate('services', {target:'live'})) and the
// draft page's edit action on a staged row — same dual-purpose role
// ProductForm.vue already plays for products. ref is disabled once editing
// — PLAN.md: "Keep refs immutable after creation."
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useKbModal } from '@/composables/useKbModal'
import { usePlayground } from '@/stores/playground'
import type { ServiceRow } from '@/types'
import { SERVICE_TYPES } from '@/components/kb/kbEntities'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import KbFormDialog from './KbFormDialog.vue'
import type { ServicePayload } from './payloads'

const modal = useKbModal()
const pg = usePlayground()
const { t } = useI18n()

const buf = reactive({
  ref: '', category: '', name: '', service_type: 'base' as string,
  parent_ref: '', price: '', duration: null as number | null, description: '',
  specialist_refs: [] as string[],
  sales_status: 'active',
})
const isEdit = () => modal.session.value?.mode === 'edit'

// Parent picker source: LIVE base services (pg.live?.services), matching
// the exact choice DeliveryZoneForm.vue already makes for its own
// parent-zone picker ("Published zones only ... a zone's parent must
// already exist live") — this dialog itself is only ever opened against the
// live lane (SpecialistsTab/ServicesTab.vue always pass {target:'live'}),
// so there is no separate "draft" set of services to reconcile against, and
// a parent staged-but-unpublished elsewhere would not yet be a valid,
// resolvable parent for the customer-facing prompt anyway. Excludes this
// row itself so a service can never become its own parent while editing.
const parentOptions = computed(() => (pg.live?.services ?? []).filter((s) => s.service_type === 'base' && s.ref !== buf.ref))
// Active specialists only — an archived master should not be newly linkable
// from this dialog (existing links on an already-archived master still ride
// along in specialist_refs untouched; this list only governs what can be
// ADDED here).
const specialistOptions = computed(() => (pg.live?.specialists ?? []).filter((s) => s.sales_status === 'active'))

let seededFor = ''
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'services' || seededFor === s.id) return
    seededFor = s.id
    const snap = s.snapshot as ServiceRow | null
    buf.ref = snap?.ref ?? ''
    buf.category = snap?.category ?? ''
    buf.name = snap?.name ?? ''
    buf.service_type = snap?.service_type || 'base'
    buf.parent_ref = snap?.parent_ref ?? ''
    buf.price = snap?.price ?? ''
    buf.duration = snap?.duration ?? null
    buf.description = snap?.description ?? ''
    buf.specialist_refs = [...(snap?.specialist_refs ?? [])]
    buf.sales_status = snap?.sales_status || 'active'
  },
  { immediate: true }
)

// A base service can never carry a parent (PLAN.md's one-level hierarchy) —
// clear it the moment the operator picks that type, so payload() never
// sends a stale parent_ref the backend would reject.
function setServiceType(type: string) {
  buf.service_type = type
  if (type === 'base') buf.parent_ref = ''
}

function toggleSpecialist(ref: string, checked: boolean) {
  buf.specialist_refs = checked ? [...buf.specialist_refs, ref] : buf.specialist_refs.filter((r) => r !== ref)
}

// duration must be genuinely nullable (blank = unset, never 0) — mirrors
// AdditionalFactsEditor.vue's own setNumberValue guard: an incomplete edit
// ('' while clearing, '-' mid-typed) is simply not-yet-a-value.
function onDurationInput(raw: string) {
  if (raw === '') {
    buf.duration = null
    return
  }
  const n = Number(raw)
  if (Number.isNaN(n)) return
  buf.duration = n
}

function payload(): ServicePayload {
  return { kind: 'services', ...buf, parent_ref: buf.service_type === 'base' ? '' : buf.parent_ref }
}
function submit() {
  if (!buf.ref.trim() || !buf.name.trim()) return
  if (buf.service_type !== 'base' && !buf.parent_ref) return
  modal.submit(payload())
}
function retry() {
  modal.reloadAndRetry(payload())
}
</script>

<template>
  <KbFormDialog
    :open="modal.isOpen.value && modal.session.value?.kind === 'services'"
    :title="isEdit() ? t('kb.forms.editService') : t('kb.forms.newService')"
    :busy="modal.busy.value"
    :error="modal.error.value"
    :stale="modal.stale.value"
    @update:open="(v) => !v && modal.close()"
    @submit="submit"
    @reload-and-retry="retry"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.forms.ref') }}</span>
        <Input v-model="buf.ref" :placeholder="t('kb.forms.serviceRefHint')" class="h-9 mt-1 font-mono" :disabled="isEdit()" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.name') }}</span>
        <Input v-model="buf.name" class="h-9 mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.category') }}</span>
        <Input v-model="buf.category" class="h-9 mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.serviceType') }}</span>
        <select
          :value="buf.service_type"
          class="h-9 mt-1 w-full rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
          data-testid="service-type-select"
          @change="setServiceType(($event.target as HTMLSelectElement).value)"
        >
          <option v-for="st in SERVICE_TYPES" :key="st" :value="st">{{ t('kb.serviceType.' + st) }}</option>
        </select>
      </div>
      <div v-if="buf.service_type !== 'base'">
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.parentService') }}</span>
        <select
          v-model="buf.parent_ref"
          class="h-9 mt-1 w-full rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
          data-testid="service-parent-select"
        >
          <option value="" disabled>{{ t('kb.forms.selectParentService') }}</option>
          <option v-for="p in parentOptions" :key="p.ref" :value="p.ref">{{ p.name || p.ref }}</option>
        </select>
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.price') }}</span>
        <Input v-model="buf.price" class="h-9 mt-1 font-mono" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.duration') }}</span>
        <Input
          type="number"
          min="0"
          :model-value="buf.duration ?? ''"
          class="h-9 mt-1 font-mono"
          data-testid="service-duration-input"
          @update:model-value="(v) => onDurationInput(String(v))"
        />
      </div>
      <label class="flex items-center gap-2 px-1 h-9 mt-4">
        <Switch :model-value="buf.sales_status === 'active'" @update:model-value="(v) => (buf.sales_status = v ? 'active' : 'inactive')" />
        <span class="text-sm text-muted-foreground">{{ t('kb.fields.salesStatusActive') }}</span>
      </label>
    </div>
    <div v-if="buf.service_type === 'addon'" class="rounded-md border border-amber-300/60 bg-amber-50 px-3 py-2 text-xs text-amber-950">
      {{ t('kb.services.addonNotStandalone') }}
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.description') }}</span>
      <Textarea v-model="buf.description" rows="3" class="min-h-0 text-[14px] mt-1" />
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.specialistRefs') }}</span>
      <div v-if="specialistOptions.length" class="mt-1.5 flex flex-wrap gap-3">
        <label v-for="sp in specialistOptions" :key="sp.ref" class="flex items-center gap-1.5 text-sm">
          <input
            type="checkbox"
            class="h-4 w-4 rounded border-border accent-primary"
            :checked="buf.specialist_refs.includes(sp.ref)"
            :data-testid="`service-specialist-${sp.ref}`"
            @change="toggleSpecialist(sp.ref, ($event.target as HTMLInputElement).checked)"
          />
          {{ sp.full_name || sp.ref }}
        </label>
      </div>
      <p v-else class="text-xs text-muted-foreground mt-1">{{ t('kb.forms.noActiveSpecialists') }}</p>
    </div>
  </KbFormDialog>
</template>
