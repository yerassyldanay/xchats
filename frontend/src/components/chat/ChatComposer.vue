<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Send, Square } from 'lucide-vue-next'
import { vAutosize } from '@/lib/autosize'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

// The question box. Enter sends, Shift+Enter breaks a line — the convention
// every chat UI shares, and the reason this is a textarea rather than an
// input.
const props = defineProps<{ sending: boolean }>()
const emit = defineEmits<{ (e: 'send', text: string): void; (e: 'stop'): void }>()

const { t } = useI18n()
const text = ref('')

function submit() {
  const value = text.value.trim()
  if (!value || props.sending) return
  emit('send', value)
  text.value = ''
}

function onEnter(e: KeyboardEvent) {
  if (e.shiftKey) return
  e.preventDefault()
  submit()
}
</script>

<template>
  <div class="border-t border-border bg-card px-4 py-3 shrink-0">
    <div class="mx-auto flex max-w-3xl items-end gap-2">
      <Textarea
        v-model="text"
        v-autosize
        rows="1"
        :placeholder="t('chat.placeholder')"
        :aria-label="t('chat.placeholder')"
        class="min-h-0 max-h-48 resize-none overflow-y-auto py-2.5"
        @keydown.enter="onEnter"
      />
      <Button
        v-if="sending"
        variant="outline"
        size="icon"
        class="shrink-0"
        :aria-label="t('chat.stop')"
        :title="t('chat.stop')"
        @click="emit('stop')"
      >
        <Square class="w-4 h-4" />
      </Button>
      <Button
        v-else
        size="icon"
        class="shrink-0"
        :disabled="!text.trim()"
        :aria-label="t('chat.send')"
        :title="t('chat.send')"
        @click="submit"
      >
        <Send class="w-4 h-4" />
      </Button>
    </div>
  </div>
</template>
