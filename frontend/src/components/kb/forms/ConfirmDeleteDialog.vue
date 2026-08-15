<script setup lang="ts">
// ConfirmDeleteDialog is the shared destructive-action confirmation —
// stateless by design (the caller owns WHAT is being acted on); confirming
// just tells the caller to go ahead, it never calls the store itself.
//
// Two callers now: /knowledge-base's Удалить (the original, which relies on
// every copy default below) and Черновик's Отменить изменение, which passes
// its own title/body/confirm-label because what that action does depends on
// the card's change type — see useCancelConfirm.
import { LoaderCircle } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

withDefaults(
  defineProps<{
    open: boolean
    busy?: boolean
    titleKey?: string
    bodyKey?: string
    bodyParams?: Record<string, unknown>
    confirmKey?: string
  }>(),
  {
    busy: false,
    titleKey: 'kb.confirmDelete.title',
    bodyKey: 'kb.confirmDelete.body',
    bodyParams: undefined,
    confirmKey: 'kb.confirmDelete.confirm',
  }
)
const emit = defineEmits<{ 'update:open': [boolean]; confirm: [] }>()
const { t } = useI18n()
</script>

<template>
  <Dialog :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{{ t(titleKey) }}</DialogTitle>
      </DialogHeader>
      <div class="px-5 py-5">
        <DialogDescription data-testid="confirm-body">{{ bodyParams ? t(bodyKey, bodyParams) : t(bodyKey) }}</DialogDescription>
      </div>
      <DialogFooter>
        <Button variant="ghost" size="sm" @click="emit('update:open', false)">{{ t('kb.confirmDelete.cancel') }}</Button>
        <Button variant="destructive" size="sm" :disabled="busy" data-testid="confirm-accept" @click="emit('confirm')">
          <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" /> {{ t(confirmKey) }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
