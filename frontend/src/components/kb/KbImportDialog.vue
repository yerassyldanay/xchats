<script setup lang="ts">
// KbImportDialog collects the pipeline's per-run settings (provider/
// target_type/guidance) for whatever files/URLs KbImportCard has already
// staged locally — it never touches the store itself, matching
// ConfirmDeleteDialog's stateless "tell the caller to go ahead" shape.
// Hand-rolled from the raw Dialog primitives (not KbFormDialog) because its
// submit button needs its own label ("Начать импорт"), not the shared
// generic "Сохранить" every *Form.vue dialog uses.
import { reactive, watch } from 'vue'
import { LoaderCircle } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

// Mirrors kbimport.Providers / kbimport.TargetTypes (backend/internal/
// kbimport/enqueue.go) exactly — the operator-facing vocabulary the backend
// itself validates against (ErrValidation, 400, on anything else).
const PROVIDERS = ['native', 'firecrawl', 'llamaparse'] as const
const TARGET_TYPES = ['auto', 'topics', 'products', 'tariffs', 'contacts', 'policies', 'delivery_zones'] as const

const props = withDefaults(
  defineProps<{
    open: boolean
    busy?: boolean
    error?: string
    pendingFileCount: number
    pendingUrlCount: number
  }>(),
  { busy: false, error: '' }
)
const emit = defineEmits<{ 'update:open': [boolean]; submit: [{ provider: string; targetType: string; guidance: string }] }>()
const { t } = useI18n()

const form = reactive({ provider: 'native' as string, targetType: 'auto' as string, guidance: '' })
// A fresh dialog always starts from the same defaults — otherwise the NEXT
// import silently inherits whatever guidance/provider the previous one left
// behind, which reads as the form having "remembered" something it never
// showed the user it kept.
watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) Object.assign(form, { provider: 'native', targetType: 'auto', guidance: '' })
  }
)

function targetTypeLabel(tt: string): string {
  return tt === 'auto' ? t('kb.import.targetTypeAuto') : t(`kb.entities.${tt}.plural`)
}

function submit() {
  emit('submit', { ...form })
}
</script>

<template>
  <Dialog :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{{ t('kb.import.dialog.title') }}</DialogTitle>
      </DialogHeader>
      <div class="px-5 py-5 space-y-4">
        <p class="text-sm text-muted-foreground">
          {{ t('kb.import.dialog.summary', { files: pendingFileCount, urls: pendingUrlCount }) }}
        </p>

        <div>
          <label class="text-xs font-medium text-muted-foreground">{{ t('kb.import.dialog.provider') }}</label>
          <Select v-model="form.provider">
            <SelectTrigger class="mt-1.5">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectItem v-for="p in PROVIDERS" :key="p" :value="p">{{ t(`kb.import.providers.${p}`) }}</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>

        <div>
          <label class="text-xs font-medium text-muted-foreground">{{ t('kb.import.dialog.targetType') }}</label>
          <Select v-model="form.targetType">
            <SelectTrigger class="mt-1.5">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectItem v-for="tt in TARGET_TYPES" :key="tt" :value="tt">{{ targetTypeLabel(tt) }}</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>

        <div>
          <label class="text-xs font-medium text-muted-foreground">{{ t('kb.import.dialog.guidance') }}</label>
          <Textarea
            v-model="form.guidance"
            rows="3"
            maxlength="2000"
            :placeholder="t('kb.import.dialog.guidancePlaceholder')"
            class="min-h-0 text-[14px] mt-1.5"
          />
        </div>

        <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
      </div>
      <DialogFooter>
        <Button variant="ghost" size="sm" @click="emit('update:open', false)">{{ t('kb.forms.cancel') }}</Button>
        <Button size="sm" :disabled="busy" @click="submit">
          <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" /> {{ t('kb.import.dialog.submit') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
