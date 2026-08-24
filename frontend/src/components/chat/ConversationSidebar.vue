<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessageSquare, Plus, Trash2 } from 'lucide-vue-next'
import { useChat } from '@/stores/chat'
import { shortTime } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

// The conversation list. Every thread here belongs to the signed-in operator
// alone (the backend scopes reads to (organization, user)), so there is no
// author column to show — only what was asked and when.
const chat = useChat()
const { t } = useI18n()

// A delete is confirmed inline rather than through a dialog: the row is right
// there, and a modal for "delete one of my own chats" is heavier than the
// action deserves.
const confirmingId = ref('')

async function remove(id: string) {
  if (confirmingId.value !== id) {
    confirmingId.value = id
    return
  }
  confirmingId.value = ''
  await chat.deleteConversation(id)
}
</script>

<template>
  <aside class="w-72 shrink-0 border-r border-border bg-card flex flex-col h-full">
    <div class="p-3 border-b border-border">
      <Button class="w-full justify-start gap-2" @click="chat.newConversation()">
        <Plus class="w-4 h-4" /> {{ t('chat.newChat') }}
      </Button>
    </div>

    <div class="flex-1 overflow-y-auto p-2">
      <div v-if="chat.loadingList && chat.conversations.length === 0" class="space-y-2 p-1">
        <Skeleton v-for="i in 4" :key="i" class="h-11 w-full rounded-lg" />
      </div>

      <p v-else-if="chat.conversations.length === 0" class="px-2 py-6 text-sm text-muted-foreground text-center">
        {{ t('chat.noConversations') }}
      </p>

      <ul v-else class="space-y-0.5">
        <li v-for="conversation in chat.conversations" :key="conversation.id">
          <div
            class="group flex items-center gap-2 rounded-lg px-2 py-2 cursor-pointer transition"
            :class="conversation.id === chat.activeId ? 'bg-muted' : 'hover:bg-muted/60'"
            @click="chat.openConversation(conversation.id)"
          >
            <MessageSquare class="w-4 h-4 shrink-0 text-muted-foreground" />
            <div class="min-w-0 flex-1">
              <div class="text-sm truncate" :class="conversation.title ? '' : 'text-muted-foreground italic'">
                {{ conversation.title || t('chat.untitled') }}
              </div>
              <div class="text-[11px] text-muted-foreground">{{ shortTime(conversation.updated_at) }}</div>
            </div>
            <button
              class="shrink-0 rounded p-1 opacity-0 group-hover:opacity-100 focus:opacity-100 transition"
              :class="confirmingId === conversation.id ? 'opacity-100 text-destructive' : 'text-muted-foreground hover:text-destructive'"
              :aria-label="confirmingId === conversation.id ? t('chat.confirmDelete') : t('chat.delete')"
              :title="confirmingId === conversation.id ? t('chat.confirmDelete') : t('chat.delete')"
              @click.stop="remove(conversation.id)"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </li>
      </ul>
    </div>
  </aside>
</template>
