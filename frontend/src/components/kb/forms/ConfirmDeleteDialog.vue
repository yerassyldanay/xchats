<script setup lang="ts">
// ConfirmDeleteDialog is the shared staged-removal confirmation for
// /knowledge-base's Delete action — stateless by design (the caller owns
// which record is being deleted); confirming just tells the caller to go
// ahead, it never calls the store itself.
import { LoaderCircle } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

withDefaults(defineProps<{ open: boolean; busy?: boolean }>(), { busy: false })
const emit = defineEmits<{ 'update:open': [boolean]; confirm: [] }>()
const { t } = useI18n()
</script>

<template>
  <Dialog :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{{ t('kb.confirmDelete.title') }}</DialogTitle>
      </DialogHeader>
      <div class="px-5 py-5">
        <DialogDescription>{{ t('kb.confirmDelete.body') }}</DialogDescription>
      </div>
      <DialogFooter>
        <Button variant="ghost" size="sm" @click="emit('update:open', false)">{{ t('kb.confirmDelete.cancel') }}</Button>
        <Button variant="destructive" size="sm" :disabled="busy" @click="emit('confirm')">
          <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" /> {{ t('kb.confirmDelete.confirm') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
