<script setup lang="ts">
// KbFormDialog is the shared modal shell every kb/forms/*Form.vue wraps its
// own field inputs in: Dialog + title + body slot + footer, plus the
// stale-submit UI from spec §2.6 — a DRAFT_STALE conflict shows an inline
// notice and swaps the primary button for «Обновить и повторить» instead of
// silently discarding whatever the operator typed.
import { LoaderCircle } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

withDefaults(
  defineProps<{
    open: boolean
    title: string
    busy?: boolean
    error?: string
    stale?: boolean
  }>(),
  { busy: false, error: '', stale: false }
)
const emit = defineEmits<{ 'update:open': [boolean]; submit: []; reloadAndRetry: [] }>()
const { t } = useI18n()
</script>

<template>
  <Dialog :open="open" @update:open="(v) => emit('update:open', v)">
    <!-- max-h + flex column so a tall body (ProductForm's five media
         pickers, for instance) scrolls internally instead of overflowing
         the viewport — DialogContent itself has no height cap and clips
         with overflow-hidden, so without this a tall form would be
         partially unreachable. -->
    <DialogContent class="max-h-[85vh] flex flex-col">
      <DialogHeader class="shrink-0">
        <DialogTitle>{{ title }}</DialogTitle>
      </DialogHeader>
      <div class="px-5 py-5 space-y-4 overflow-y-auto flex-1 min-h-0">
        <slot />
        <div v-if="stale" class="rounded-lg border border-amber-300/60 bg-amber-50 px-3 py-2 text-sm text-amber-950">
          <p class="font-medium">{{ t('kb.forms.staleTitle') }}</p>
          <p class="mt-0.5">{{ t('kb.forms.staleBody') }}</p>
        </div>
        <p v-else-if="error" class="text-sm text-destructive">{{ error }}</p>
      </div>
      <DialogFooter class="shrink-0">
        <Button variant="ghost" size="sm" @click="emit('update:open', false)">{{ t('kb.forms.cancel') }}</Button>
        <Button v-if="stale" size="sm" :disabled="busy" @click="emit('reloadAndRetry')">
          <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" /> {{ t('kb.forms.reloadAndRetry') }}
        </Button>
        <Button v-else size="sm" :disabled="busy" @click="emit('submit')">
          <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" /> {{ t('kb.forms.save') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
