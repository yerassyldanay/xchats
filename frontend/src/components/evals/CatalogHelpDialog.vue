<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { BookOpen } from 'lucide-vue-next'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

// «Справка» — a short bilingual summary of the check taxonomy (condensed from
// evals/VOCABULARY.md's §Checks), surfaced right on the page instead of only living in
// a repo file nobody opens mid-review.
const { t } = useI18n()
const open = ref(false)
</script>

<template>
  <button
    type="button"
    class="inline-flex items-center gap-1.5 rounded-lg border border-border px-2.5 py-1.5 text-xs text-muted-foreground hover:bg-muted/60 transition"
    @click="open = true"
  >
    <BookOpen class="w-3.5 h-3.5" />
    {{ t('evalCatalog.help.title') }}
  </button>

  <Dialog :open="open" @update:open="(v) => { open = v }">
    <DialogContent class="max-w-lg">
      <DialogHeader><DialogTitle>{{ t('evalCatalog.help.title') }}</DialogTitle></DialogHeader>
      <div class="px-5 py-4 space-y-3 text-sm text-muted-foreground leading-relaxed">
        <p>{{ t('evalCatalog.help.p1') }}</p>
        <p>{{ t('evalCatalog.help.p2') }}</p>
        <p>{{ t('evalCatalog.help.p3') }}</p>
        <p>{{ t('evalCatalog.help.p4') }}</p>
      </div>
    </DialogContent>
  </Dialog>
</template>
