<script setup lang="ts">
// ConfirmDeleteDialog is База знаний's shared staged-removal confirmation —
// deleting a published row never touches the live table directly, it stages
// a pending removal (pg.stageDelete), so the copy here is explicit that the
// record survives until the change is published from Черновик.
import { CircleAlert, LoaderCircle } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'

withDefaults(defineProps<{ open: boolean; busy?: boolean }>(), { busy: false })
const emit = defineEmits<{ confirm: []; close: [] }>()
const { t } = useI18n()
</script>

<template>
  <Dialog :open="open" @update:open="(v) => { if (!v) emit('close') }">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>
          <span class="w-8 h-8 rounded-lg bg-destructive/10 text-destructive grid place-items-center">
            <CircleAlert class="w-4 h-4" />
          </span>
          {{ t('kb.confirmDelete.title') }}
        </DialogTitle>
      </DialogHeader>
      <div class="px-5 py-5">
        <p class="text-sm text-muted-foreground">{{ t('kb.confirmDelete.body') }}</p>
      </div>
      <DialogFooter>
        <Button variant="ghost" size="sm" :disabled="busy" @click="emit('close')">{{ t('kb.confirmDelete.cancel') }}</Button>
        <Button variant="destructive" size="sm" :disabled="busy" @click="emit('confirm')">
          <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" /> {{ t('kb.confirmDelete.confirm') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
