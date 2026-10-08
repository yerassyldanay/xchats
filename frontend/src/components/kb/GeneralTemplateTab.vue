<script setup lang="ts">
// General Template tab — the operator-editable INSTRUCTIONS of the assistant's
// system prompt (ai_prompt_templates, GET/PUT /kb/templates).
//
// Two clearly separate cards: the CONTROLS (choose a profile, Save, Discard,
// status and errors) and the INSTRUCTION TEXT itself. Only natural-language
// rules are editable here; the response format, the assistant block, the channel
// line and every piece of knowledge-base data are added automatically on the
// server and are shown, read-only, in the Final Template tab. Facts stay
// editable through the existing knowledge-base tabs.
//
// Choosing a profile is optional (General is the default). Unsaved edits are
// kept PER PROFILE in a local map, so switching profiles never loses text; Save
// writes the selected profile's text only and makes that profile active.
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, FileCog, Info, TriangleAlert } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { PROMPT_TEMPLATE_IDS, type PromptTemplateId } from '@/types'
import { MAX_TEMPLATE_CHARS, profileKey, templateIssue } from '@/lib/promptTemplates'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

const emit = defineEmits<{ 'view-final': [] }>()

const pg = usePlayground()
const { t } = useI18n()

const selected = ref<PromptTemplateId>('general')
// Unsaved local edits, keyed by profile. A key exists only while the text differs
// from what the server holds.
const drafts = reactive<Partial<Record<PromptTemplateId, string>>>({})
let initialised = false

onMounted(() => {
  if (!pg.templates) pg.loadTemplates()
})

const activeId = computed(() => pg.templates?.active_template_id)

function serverText(id: PromptTemplateId): string {
  return pg.templates?.templates.find((x) => x.id === id)?.instructions ?? ''
}

// The first load opens on the active profile. A later server refresh (SSE, a save
// from another tab) only drops local drafts that now equal the server text; it
// never replaces text the operator is still editing.
watch(
  () => pg.templates,
  (view) => {
    if (!view) return
    if (!initialised) {
      selected.value = view.active_template_id
      initialised = true
    }
    for (const id of PROMPT_TEMPLATE_IDS) {
      if (drafts[id] !== undefined && drafts[id] === serverText(id)) delete drafts[id]
    }
  },
  { immediate: true },
)

const text = computed<string>({
  get: () => drafts[selected.value] ?? serverText(selected.value),
  set: (v) => {
    pg.templateSaveError = ''
    saved.value = false
    if (v === serverText(selected.value)) delete drafts[selected.value]
    else drafts[selected.value] = v
  },
})

function isDirty(id: PromptTemplateId): boolean {
  return drafts[id] !== undefined
}

function profileName(id: PromptTemplateId): string {
  const k = profileKey(id)
  return k ? t(`kb.template.profiles.${k}.name`) : id
}
function profileDescription(id: PromptTemplateId): string {
  const k = profileKey(id)
  return k ? t(`kb.template.profiles.${k}.description`) : ''
}

const issue = computed(() => (pg.templates ? templateIssue(text.value) : null))
const issueMessage = computed(() => {
  switch (issue.value) {
    case 'empty':
      return t('kb.template.errEmpty')
    case 'reserved':
      return t('kb.template.errReserved')
    case 'tooLong':
      return t('kb.template.errTooLong', { n: MAX_TEMPLATE_CHARS })
    default:
      return ''
  }
})
// A server rejection (for example a link or file name in the text) or a failed save.
const errorMessage = computed(() => issueMessage.value || pg.templateSaveError)

const canSave = computed(
  () => !!pg.templates && !pg.templateSaving && !issue.value && (isDirty(selected.value) || selected.value !== activeId.value),
)

const saved = ref(false)
let savedTimer: number | undefined
async function save() {
  if (!canSave.value) return
  const id = selected.value
  const ok = await pg.saveTemplate(id, text.value)
  if (ok) {
    delete drafts[id]
    saved.value = true
    window.clearTimeout(savedTimer)
    savedTimer = window.setTimeout(() => (saved.value = false), 4000)
  }
}

function discard() {
  delete drafts[selected.value]
  pg.templateSaveError = ''
}

function select(id: PromptTemplateId) {
  selected.value = id
  pg.templateSaveError = ''
  saved.value = false
}

const textareaId = 'general-template-instructions'
const charCount = computed(() => [...text.value].length)
</script>

<template>
  <div class="space-y-4 max-w-4xl">
    <div v-if="pg.templatesLoading && !pg.templates" class="p-10 text-center text-sm text-muted-foreground" data-testid="template-loading">
      {{ t('kb.template.loading') }}
    </div>
    <div v-else-if="pg.templatesLoadError && !pg.templates" class="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5">
      <p class="flex items-start gap-2 text-sm text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> <span>{{ pg.templatesLoadError }}</span>
      </p>
      <Button size="sm" variant="outline" class="mt-3" @click="pg.loadTemplates()">{{ t('common.retry') }}</Button>
    </div>

    <template v-else-if="pg.templates">
      <!-- 1. CONTROLS — choosing a profile and saving. Never part of the instruction text. -->
      <section class="rounded-xl border border-border bg-card p-5 space-y-4" data-testid="template-controls">
        <div class="flex gap-3">
          <div class="w-9 h-9 rounded-lg bg-primary/10 text-primary grid place-items-center shrink-0">
            <FileCog class="w-4 h-4" />
          </div>
          <div>
            <h3 class="font-semibold leading-tight">{{ t('kb.template.title') }}</h3>
            <p class="text-xs text-muted-foreground mt-1 max-w-xl">{{ t('kb.template.subtitle') }}</p>
          </div>
        </div>

        <p
          v-if="!pg.templates.kb_configured"
          class="flex items-start gap-2 text-sm rounded-lg border border-amber-300/60 bg-amber-50 text-amber-900 px-3 py-2.5"
          role="status"
          data-testid="template-not-configured"
        >
          <TriangleAlert class="w-4 h-4 shrink-0 mt-0.5" />
          <span>
            <span class="font-medium">{{ t('kb.template.notConfiguredTitle') }}.</span>
            {{ t('kb.template.notConfiguredBody') }}
          </span>
        </p>

        <div>
          <h4 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{{ t('kb.template.controlsTitle') }}</h4>
          <p class="text-xs text-muted-foreground mt-1">{{ t('kb.template.controlsHint') }}</p>
          <div role="radiogroup" :aria-label="t('kb.template.controlsTitle')" class="mt-3 grid grid-cols-1 md:grid-cols-2 gap-3">
            <button
              v-for="id in PROMPT_TEMPLATE_IDS"
              :key="id"
              type="button"
              role="radio"
              :aria-checked="selected === id"
              :data-testid="`template-profile-${id}`"
              class="text-left rounded-lg border px-3.5 py-3 transition focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
              :class="selected === id ? 'border-primary bg-primary/5' : 'border-border hover:bg-muted/60'"
              @click="select(id)"
            >
              <span class="flex items-center gap-2">
                <span class="font-medium text-sm">{{ profileName(id) }}</span>
                <Badge v-if="activeId === id" class="bg-emerald-100 text-emerald-700 hover:bg-emerald-100" :data-testid="`template-active-${id}`">
                  {{ t('kb.template.active') }}
                </Badge>
                <span
                  v-if="isDirty(id)"
                  class="w-2 h-2 rounded-full bg-amber-500"
                  :title="t('kb.template.unsaved')"
                  :aria-label="t('kb.template.unsaved')"
                  role="img"
                  :data-testid="`template-dirty-${id}`"
                />
              </span>
              <span class="block text-xs text-muted-foreground mt-1">{{ profileDescription(id) }}</span>
            </button>
          </div>
        </div>

        <div class="flex items-center gap-2 flex-wrap">
          <Button size="sm" :disabled="!canSave" data-testid="template-save" @click="save">
            {{ pg.templateSaving ? t('kb.template.saving') : t('kb.template.save') }}
          </Button>
          <Button size="sm" variant="outline" :disabled="!isDirty(selected) || pg.templateSaving" data-testid="template-discard" @click="discard">
            {{ t('kb.template.discard') }}
          </Button>
          <span v-if="saved" class="text-xs text-emerald-700" role="status" data-testid="template-saved">{{ t('kb.template.saved') }}</span>
        </div>

        <p v-if="errorMessage" class="flex items-start gap-2 text-sm text-destructive" role="alert" data-testid="template-error">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> <span>{{ errorMessage }}</span>
        </p>
      </section>

      <!-- 2. INSTRUCTION TEXT — only natural-language rules. -->
      <section class="rounded-xl border border-border bg-card p-5 space-y-3" data-testid="template-text">
        <div>
          <label :for="textareaId" class="text-sm font-semibold">
            {{ t('kb.template.instructionsLabel', { profile: profileName(selected) }) }}
          </label>
          <p class="text-xs text-muted-foreground mt-1">{{ t('kb.template.instructionsHint') }}</p>
        </div>
        <Textarea
          :id="textareaId"
          v-model="text"
          :rows="24"
          spellcheck="false"
          class="font-mono text-[12.5px] leading-relaxed"
          data-testid="template-instructions"
        />
        <p class="text-xs text-muted-foreground text-right">{{ t('kb.template.chars', { n: charCount }) }}</p>

        <p class="flex items-start gap-2 text-xs text-muted-foreground rounded-lg bg-muted px-3 py-2.5">
          <Info class="w-3.5 h-3.5 shrink-0 mt-0.5" />
          <span>
            {{ t('kb.template.protectedNote') }}
            <button type="button" class="text-primary font-medium hover:underline" data-testid="template-view-final" @click="emit('view-final')">
              {{ t('kb.template.viewFinal') }}
            </button>
          </span>
        </p>
      </section>
    </template>
  </div>
</template>
