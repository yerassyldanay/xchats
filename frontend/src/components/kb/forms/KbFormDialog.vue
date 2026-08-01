<script setup lang="ts">
// KbFormDialog is the shared modal shell every typed KB form wraps its own
// fields in: header (icon+title) / body slot / footer, plus the ONE stale-
// submit affordance (§2.6): the dialog never closes on failure, so a typed
// buffer always survives — it closes only when the caller's submit handler
// reports success.
import type { Component } from 'vue'
import { CircleAlert, LoaderCircle } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'

withDefaults(
  defineProps<{
    open: boolean
    icon: Component
    title: string
    busy?: boolean
    error?: string
    stale?: boolean
    saveDisabled?: boolean
  }>(),
  { busy: false, error: '', stale: false, saveDisabled: false }
)

const emit = defineEmits<{ submit: []; reloadRetry: []; close: [] }>()
const { t } = useI18n()
</script>

<template>
  <Dialog :open="open" @update:open="(v) => { if (!v) emit('close') }">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>
          <span class="w-8 h-8 rounded-lg bg-primary/10 text-primary grid place-items-center">
            <component :is="icon" class="w-4 h-4" />
          </span>
          {{ title }}
        </DialogTitle>
      </DialogHeader>
      <div class="px-5 py-5 space-y-3 max-h-[65vh] overflow-y-auto">
        <slot />

        <div v-if="stale" class="rounded-lg border border-amber-300/60 bg-amber-50 px-3 py-3 text-sm text-amber-950 space-y-2">
          <p class="font-semibold">{{ t('kb.stale.title') }}</p>
          <p>{{ t('kb.stale.body') }}</p>
          <Button size="sm" :disabled="busy" @click="emit('reloadRetry')">
            <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" /> {{ t('kb.stale.reloadAndRetry') }}
          </Button>
        </div>
        <p v-else-if="error" class="flex items-center gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0" /> {{ error }}
        </p>
      </div>
      <DialogFooter>
        <Button variant="ghost" size="sm" :disabled="busy" @click="emit('close')">{{ t('kb.common.cancel') }}</Button>
        <Button v-if="!stale" size="sm" :disabled="busy || saveDisabled" @click="emit('submit')">
          <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" /> {{ t('kb.common.save') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
