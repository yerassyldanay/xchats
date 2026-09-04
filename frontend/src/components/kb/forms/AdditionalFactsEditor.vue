<script setup lang="ts">
// AdditionalFactsEditor is the repeatable ref/value/instruction editor for
// aiprompt.AdditionalFact — seller-authored "virtual fact columns" on a
// product, tariff, or the tariff_info singleton (backend/aiprompt/facts.go).
// A pure controlled component, same convention as MediaFieldPicker.vue: no
// internal buffer, every keystroke reconstructs the whole array from
// props.modelValue and emits it — so the owning form's payload() always
// carries this field's full current intent, matching the whole-list-replace
// contract every plural field on these forms already follows (payloads.ts's
// doc comment): omit the field entirely to leave existing facts unchanged,
// send it (an empty array included) to replace the complete list.
//
// value's JS type IS the wire type: a number input emits a JS number (a
// JSON number on the wire), a checkbox emits a JS boolean, a text input
// emits a JS string — exactly the number | boolean | string union
// aiprompt.AdditionalFact.Value accepts (facts.go rejects null/array/object
// outright). Switching a row's type re-seeds `value` to that type's zero
// value rather than coercing the old one (a half-typed number string is not
// a meaningful boolean or vice versa).
//
// Full server-side validation (ref uniqueness within the record, no
// collision with a concrete column such as "price", instruction hygiene,
// count/length limits) is aiprompt.ValidateFacts' job, not duplicated here —
// this component only guards the one client-checkable rule (ref syntax) so
// a typo is caught before a round trip; everything else surfaces through
// the owning KbFormDialog's existing error banner on submit, same as every
// other field on these forms.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus, Trash2 } from 'lucide-vue-next'
import type { AdditionalFact } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const props = defineProps<{ modelValue: AdditionalFact[] }>()
const emit = defineEmits<{ 'update:modelValue': [AdditionalFact[]] }>()
const { t } = useI18n()

type ValueType = 'number' | 'boolean' | 'string'
const VALUE_TYPES: ValueType[] = ['string', 'number', 'boolean']

const REF_PATTERN = /^[a-z][a-z0-9_]{0,63}$/

function valueTypeOf(v: AdditionalFact['value']): ValueType {
  if (typeof v === 'number') return 'number'
  if (typeof v === 'boolean') return 'boolean'
  return 'string'
}

function refInvalid(ref: string): boolean {
  return ref.length > 0 && !REF_PATTERN.test(ref)
}

// MAX_SAFE_INTEGER_MAGNITUDE mirrors Number.MAX_SAFE_INTEGER: the largest
// whole number a JS `number` — and therefore this field's own wire value —
// can hold exactly. MAX_SAFE_SIGNIFICANT_DIGITS mirrors the backend's own
// decimal guard. Both are literals rather than references to a shared
// module so the boundary this component checks against is visible right
// where it's used — kept in lockstep with backend/aiprompt/facts.go's
// maxSafeIntegerMagnitude/maxSafeSignificantDigits by facts_test.go /
// this component's own dom test asserting the same two example values.
const MAX_SAFE_INTEGER_MAGNITUDE = 9007199254740991
const MAX_SAFE_SIGNIFICANT_DIGITS = 15

// numberValueImprecise flags a numeric fact whose CURRENT stored value the
// KB editor could not have edited exactly: this UI's number <input> parses
// every keystroke through JS `Number()`, which silently rounds a whole
// number beyond Number.MAX_SAFE_INTEGER and can likewise strip digits from
// an overly precise decimal — the same exact-value guarantee
// aiprompt.ValidateFacts (facts.go) enforces on submit. Reading off the
// already-stored number (not the raw keystroke) keeps this component
// buffer-free, per its own doc comment: a value big or precise enough to
// trip this either stayed the same magnitude after rounding (an integer
// past the safe boundary does) or printed back out just as long (an
// over-precise decimal's shortest round-trip form rarely gets shorter), so
// checking the stored value still catches both cases. Advisory only, the
// same non-blocking role refInvalid/duplicateRefs already play — the
// backend is still the authoritative check.
function numberValueImprecise(n: number): boolean {
  if (!Number.isFinite(n)) return false
  if (Number.isInteger(n)) return Math.abs(n) > MAX_SAFE_INTEGER_MAGNITUDE
  const digits = String(Math.abs(n))
    .replace(/[-.]/g, '')
    .replace(/^0+/, '')
    .replace(/0+$/, '')
  return digits.length > MAX_SAFE_SIGNIFICANT_DIGITS
}

// duplicateRefs marks every ref that collides with another row in THIS
// list, so the editor can flag it before the round trip that aiprompt.
// ValidateFacts' own duplicate check would otherwise catch first.
const duplicateRefs = computed(() => {
  const counts = new Map<string, number>()
  for (const f of props.modelValue) {
    if (!f.ref) continue
    counts.set(f.ref, (counts.get(f.ref) ?? 0) + 1)
  }
  const dup = new Set<string>()
  for (const [ref, n] of counts) if (n > 1) dup.add(ref)
  return dup
})

function updateAt(i: number, patch: Partial<AdditionalFact>) {
  emit('update:modelValue', props.modelValue.map((f, idx) => (idx === i ? { ...f, ...patch } : f)))
}
function setType(i: number, type: ValueType) {
  const value: AdditionalFact['value'] = type === 'number' ? 0 : type === 'boolean' ? false : ''
  updateAt(i, { value })
}
function setNumberValue(i: number, raw: string) {
  // An incomplete edit ('' while clearing the field, '-' as the first
  // keystroke of a negative number) is not yet a value — leaving it
  // unemitted keeps the row at its last real value instead of silently
  // coercing a not-yet-finished edit into a stored 0 (a value the operator
  // never actually typed).
  if (raw === '' || raw === '-') return
  const n = Number(raw)
  if (Number.isNaN(n)) return
  updateAt(i, { value: n })
}
function addFact() {
  emit('update:modelValue', [...props.modelValue, { ref: '', value: '', instruction: '' }])
}
function removeFact(i: number) {
  emit(
    'update:modelValue',
    props.modelValue.filter((_, idx) => idx !== i)
  )
}
</script>

<template>
  <div class="space-y-2" data-testid="additional-facts-editor">
    <div class="flex items-center justify-between">
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.facts.title') }}</span>
      <span class="text-[11px] text-muted-foreground">{{ t('kb.facts.hint') }}</span>
    </div>

    <div
      v-for="(fact, i) in modelValue"
      :key="i"
      class="rounded-md border border-border p-2.5 space-y-2"
      data-testid="additional-fact-row"
    >
      <div class="grid grid-cols-1 sm:grid-cols-[1fr_auto_1fr_auto] gap-2 items-start">
        <div>
          <Input
            :model-value="fact.ref"
            :placeholder="t('kb.facts.refPlaceholder')"
            class="h-9 font-mono text-sm"
            :class="{ 'border-destructive': refInvalid(fact.ref) || duplicateRefs.has(fact.ref) }"
            data-testid="additional-fact-ref"
            @update:model-value="(v) => updateAt(i, { ref: String(v) })"
          />
          <p v-if="refInvalid(fact.ref)" class="text-[11px] text-destructive mt-1">{{ t('kb.facts.refInvalid') }}</p>
          <p v-else-if="duplicateRefs.has(fact.ref)" class="text-[11px] text-destructive mt-1">{{ t('kb.facts.refDuplicate') }}</p>
        </div>

        <select
          :value="valueTypeOf(fact.value)"
          class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
          data-testid="additional-fact-type"
          @change="setType(i, ($event.target as HTMLSelectElement).value as ValueType)"
        >
          <option v-for="vt in VALUE_TYPES" :key="vt" :value="vt">{{ t('kb.facts.valueType.' + vt) }}</option>
        </select>

        <div>
          <Input
            v-if="valueTypeOf(fact.value) === 'string'"
            :model-value="fact.value as string"
            :placeholder="t('kb.facts.valuePlaceholder')"
            class="h-9 text-sm"
            data-testid="additional-fact-value"
            @update:model-value="(v) => updateAt(i, { value: String(v) })"
          />
          <Input
            v-else-if="valueTypeOf(fact.value) === 'number'"
            type="number"
            :model-value="fact.value as number"
            class="h-9 font-mono text-sm"
            :class="{ 'border-destructive': numberValueImprecise(fact.value as number) }"
            data-testid="additional-fact-value"
            @update:model-value="(v) => setNumberValue(i, String(v))"
          />
          <p v-if="valueTypeOf(fact.value) === 'number' && numberValueImprecise(fact.value as number)" class="text-[11px] text-destructive mt-1">
            {{ t('kb.facts.valueImprecise') }}
          </p>
          <label v-else class="flex items-center gap-2 h-9 px-1">
            <input
              type="checkbox"
              :checked="fact.value as boolean"
              class="h-4 w-4 rounded border-border"
              data-testid="additional-fact-value"
              @change="updateAt(i, { value: ($event.target as HTMLInputElement).checked })"
            />
            <span class="text-sm text-muted-foreground">{{ fact.value ? t('common.yes') : t('common.no') }}</span>
          </label>
        </div>

        <Button
          type="button"
          variant="ghost"
          size="icon"
          class="h-9 w-9 shrink-0 text-destructive hover:bg-destructive/10"
          :title="t('kb.facts.removeFact')"
          data-testid="additional-fact-remove"
          @click="removeFact(i)"
        >
          <Trash2 class="w-4 h-4" />
        </Button>
      </div>

      <Input
        :model-value="fact.instruction"
        :placeholder="t('kb.facts.instructionPlaceholder')"
        class="h-9 text-sm"
        data-testid="additional-fact-instruction"
        @update:model-value="(v) => updateAt(i, { instruction: String(v) })"
      />
    </div>

    <Button type="button" variant="outline" size="sm" data-testid="additional-fact-add" @click="addFact">
      <Plus class="w-4 h-4" /> {{ t('kb.facts.addFact') }}
    </Button>
  </div>
</template>
