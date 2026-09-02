<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Paperclip, Send, Smile, TriangleAlert, X } from 'lucide-vue-next'
import { useInbox } from '../stores/inbox'
import { vAutosize } from '../lib/autosize'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'

defineProps<{ sending: boolean }>()
const emit = defineEmits<{ (e: 'send', text: string, files: File[]): void }>()

const inbox = useInbox()
const { t } = useI18n()
const files = ref<File[]>([])
const fileInput = ref<HTMLInputElement | null>(null)

// INB-07: a small curated set rather than a full picker (search, skin tones,
// categories) — "lightweight" per the flow doc, not a general-purpose widget.
const EMOJIS = [
  '😀', '😂', '🙂', '😉', '😍', '😘', '😅', '😊',
  '🙌', '👍', '👎', '🙏', '👋', '💪', '🤝', '👏',
  '❤️', '🔥', '🎉', '✅', '⏰', '📌', '💬', '📎',
  '😢', '😮', '😡', '🤔', '👌', '✨', '🚀', '☺️',
]
const emojiOpen = ref(false)
function pickEmoji(e: string) {
  inbox.composerText += e
  emojiOpen.value = false
}

function pick(e: Event) {
  const list = (e.target as HTMLInputElement).files
  if (list) files.value = Array.from(list)
}
function removeFile(i: number) {
  files.value.splice(i, 1)
}
// Text and attachments stay put until the store confirms the send actually
// succeeded (INB-09) — the two watchers below are the only places files
// get cleared, so a failed or still in-flight send never loses them.
function submit() {
  const text = inbox.composerText.trim()
  if (!text && files.value.length === 0) return
  if (inbox.activeSending) return
  emit('send', text, files.value)
}
function resetFiles() {
  files.value = []
  if (fileInput.value) fileInput.value.value = ''
}
// A send just finished for whichever chat is now on screen: only clear the
// picked files once it lands without an error.
watch(
  () => inbox.activeSending,
  (nowSending, wasSending) => {
    if (wasSending && !nowSending && !inbox.activeSendError) resetFiles()
  },
)
// Switching chats always presents a fresh composer — carrying chat A's
// staged attachments into chat B's box risks them being sent to the wrong
// customer the moment the operator hits Send there.
watch(() => inbox.activeId, resetFiles)
</script>

<template>
  <div class="border-t border-border bg-card px-4 py-3 shrink-0">
    <p
      v-if="inbox.activeSendError"
      class="mb-2 flex items-center gap-2 rounded-lg border border-destructive/20 bg-destructive/5 px-3 py-2 text-xs text-destructive"
    >
      <TriangleAlert class="w-3.5 h-3.5 shrink-0" />
      <span class="flex-1">{{ inbox.activeSendError }}</span>
      <button class="shrink-0 font-medium underline underline-offset-2 hover:no-underline" @click="submit">
        {{ t('common.retry') }}
      </button>
    </p>
    <div v-if="files.length" class="mb-2 flex flex-wrap gap-2">
      <span
        v-for="(f, i) in files"
        :key="i"
        class="flex items-center gap-1.5 rounded-full bg-muted border border-border px-3 py-1 text-xs"
      >
        <Paperclip class="w-3.5 h-3.5 text-muted-foreground" /> {{ f.name }}
        <button class="text-muted-foreground hover:text-destructive" @click="removeFile(i)">
          <X class="w-3.5 h-3.5" />
        </button>
      </span>
    </div>
    <div class="flex items-end gap-2">
      <Button variant="ghost" size="icon" class="shrink-0" :title="t('inbox.attachFile')" @click="fileInput?.click()">
        <Paperclip class="w-[18px] h-[18px]" />
      </Button>
      <input ref="fileInput" type="file" multiple class="hidden" @change="pick" />
      <div class="flex-1 flex items-end rounded-xl bg-muted border border-border focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/30 transition px-3">
        <Textarea
          v-model="inbox.composerText"
          v-autosize
          rows="1"
          :placeholder="t('inbox.messagePlaceholder')"
          class="flex-1 resize-none border-0 bg-transparent py-2.5 min-h-0 max-h-[40vh] overflow-y-auto text-[15px] shadow-none focus-visible:ring-0 focus-visible:ring-offset-0"
          @keydown.enter.exact.prevent="submit"
        />
        <DropdownMenu v-model:open="emojiOpen">
          <DropdownMenuTrigger as-child>
            <button class="pb-2.5 pl-2 text-muted-foreground hover:text-foreground transition" :title="t('inbox.emoji')" type="button">
              <Smile class="w-5 h-5" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" class="w-64 p-2">
            <div class="grid grid-cols-8 gap-0.5">
              <button
                v-for="e in EMOJIS"
                :key="e"
                type="button"
                class="grid place-items-center w-7 h-7 rounded-md text-lg hover:bg-accent transition"
                @click="pickEmoji(e)"
              >
                {{ e }}
              </button>
            </div>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <Button :disabled="sending" class="shrink-0" @click="submit">
        <Send class="w-4 h-4" /> {{ t('inbox.send') }}
      </Button>
    </div>
  </div>
</template>
