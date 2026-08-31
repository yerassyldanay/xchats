<script setup lang="ts">
// CampaignTemplateFormDialog is CAM-14's create/edit modal for one message
// template — a plain name + message body, no account/channel/pace/schedule
// (templates are pure content, see the backend migration's own doc
// comment). Reused by both the Templates tab (Campaigns.vue) and the
// wizard's own "Save as template" action (CampaignWizard.vue) — a
// `template` prop present means edit, absent means create.
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { LoaderCircle } from 'lucide-vue-next'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { ApiError } from '@/api/client'
import { useCampaignTemplates } from '@/stores/campaignTemplates'
import type { CampaignTemplate } from '@/types'

const props = defineProps<{ open: boolean; template?: CampaignTemplate | null; initialBody?: string }>()
const emit = defineEmits<{ 'update:open': [boolean]; saved: [CampaignTemplate] }>()
const { t } = useI18n()
const templates = useCampaignTemplates()

const name = ref('')
const body = ref('')
const busy = ref(false)
const error = ref('')
const isEdit = computed(() => !!props.template)

// Reset (or seed from `template`/`initialBody`) every time the dialog opens
// — mirrors ConfirmDeleteDialog/KbFormDialog's own "controlled by `open`,
// state reset on open" pattern rather than on mount (this component is
// kept alive between opens, not re-mounted per open).
watch(
  () => props.open,
  (isOpen) => {
    if (!isOpen) return
    name.value = props.template?.name ?? ''
    body.value = props.template?.message_body ?? props.initialBody ?? ''
    error.value = ''
  },
  { immediate: true }
)

function close() {
  emit('update:open', false)
}

async function save() {
  if (!name.value.trim()) {
    error.value = t('campaigns.templates.errNameRequired')
    return
  }
  if (!body.value.trim()) {
    error.value = t('campaigns.templates.errBodyRequired')
    return
  }
  error.value = ''
  busy.value = true
  try {
    const saved = props.template
      ? await templates.update(props.template.id, { name: name.value.trim(), message_body: body.value })
      : await templates.create({ name: name.value.trim(), message_body: body.value })
    emit('saved', saved)
    close()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : t('campaigns.templates.errSaveFailed')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Dialog :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogContent class="max-h-[85vh] flex flex-col">
      <DialogHeader class="shrink-0">
        <DialogTitle>{{ isEdit ? t('campaigns.templates.editTitle') : t('campaigns.templates.newTitle') }}</DialogTitle>
      </DialogHeader>
      <div class="px-5 py-5 space-y-4 overflow-y-auto flex-1 min-h-0">
        <div>
          <label class="text-xs font-medium text-muted-foreground">{{ t('campaigns.templates.nameLabel') }}</label>
          <Input v-model="name" :placeholder="t('campaigns.templates.namePlaceholder')" class="mt-1.5" data-testid="template-name-input" />
        </div>
        <div>
          <label class="text-xs font-medium text-muted-foreground">{{ t('campaigns.templates.bodyLabel') }}</label>
          <Textarea v-model="body" :placeholder="t('campaigns.wizard.messagePlaceholder')" class="mt-1.5 min-h-[140px] text-sm" data-testid="template-body-input" />
        </div>
        <p v-if="error" class="text-sm text-destructive" data-testid="template-form-error">{{ error }}</p>
      </div>
      <DialogFooter class="shrink-0">
        <Button variant="ghost" size="sm" @click="close">{{ t('campaigns.templates.cancel') }}</Button>
        <Button size="sm" :disabled="busy" data-testid="template-form-save" @click="save">
          <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" /> {{ t('campaigns.templates.save') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
