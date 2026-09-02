<script setup lang="ts">
import { computed } from 'vue'
import { Sparkles } from 'lucide-vue-next'
import { renderMarkdown } from '@/lib/markdown'
import KbComponent from './KbComponent.vue'
import type { ChatMessage } from '@/types'

// One turn in the transcript. A user turn is a right-aligned bubble of plain
// text; an assistant turn is left-aligned rendered Markdown, with its
// knowledge-base cards stacked underneath.
const props = defineProps<{ message: ChatMessage; streaming?: boolean }>()

const isUser = computed(() => props.message.role === 'user')
// v-html is safe here: renderMarkdown runs markdown-it with html:false, which
// escapes any raw markup instead of passing it through (see lib/markdown.ts).
const html = computed(() => renderMarkdown(props.message.content))
</script>

<template>
  <div class="flex gap-3" :class="isUser ? 'justify-end' : 'justify-start'">
    <div
      v-if="!isUser"
      class="mt-0.5 w-7 h-7 shrink-0 rounded-lg bg-primary/10 text-primary grid place-items-center"
      aria-hidden="true"
    >
      <Sparkles class="w-4 h-4" />
    </div>

    <div class="min-w-0" :class="isUser ? 'max-w-[85%]' : 'max-w-[85%] flex-1'">
      <div
        v-if="isUser"
        class="rounded-2xl rounded-tr-sm bg-primary px-4 py-2.5 text-primary-foreground text-sm whitespace-pre-wrap break-words"
      >
        {{ message.content }}
      </div>

      <template v-else>
        <div
          v-if="message.content"
          class="chat-prose text-sm leading-relaxed break-words"
          v-html="html"
        />
        <!-- Before the first token there is nothing to render but something
             is clearly happening, so the bubble says so rather than sitting
             empty. -->
        <div v-else-if="streaming" class="flex items-center gap-1 py-1.5" aria-hidden="true">
          <span class="w-1.5 h-1.5 rounded-full bg-muted-foreground/60 animate-bounce [animation-delay:-0.3s]" />
          <span class="w-1.5 h-1.5 rounded-full bg-muted-foreground/60 animate-bounce [animation-delay:-0.15s]" />
          <span class="w-1.5 h-1.5 rounded-full bg-muted-foreground/60 animate-bounce" />
        </div>

        <div v-if="message.components.length" class="mt-3 space-y-3">
          <KbComponent
            v-for="(component, i) in message.components"
            :key="`${component.type}:${i}`"
            :component="component"
          />
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
/* Assistant answers are Markdown, so the few block elements the model
   actually produces need spacing. Scoped and deliberately small — this is
   chat prose, not an article. */
.chat-prose :deep(p) {
  margin: 0 0 0.6rem;
}
.chat-prose :deep(p:last-child) {
  margin-bottom: 0;
}
.chat-prose :deep(ul),
.chat-prose :deep(ol) {
  margin: 0 0 0.6rem;
  padding-left: 1.25rem;
  list-style: revert;
}
.chat-prose :deep(li) {
  margin: 0.15rem 0;
}
.chat-prose :deep(h1),
.chat-prose :deep(h2),
.chat-prose :deep(h3) {
  font-weight: 600;
  margin: 0.9rem 0 0.4rem;
}
.chat-prose :deep(code) {
  background: hsl(var(--muted));
  border-radius: 0.25rem;
  padding: 0.05rem 0.3rem;
  font-size: 0.9em;
}
.chat-prose :deep(pre) {
  background: hsl(var(--muted));
  border-radius: 0.5rem;
  padding: 0.6rem 0.75rem;
  margin: 0 0 0.6rem;
  overflow-x: auto;
}
.chat-prose :deep(pre code) {
  background: transparent;
  padding: 0;
}
.chat-prose :deep(a) {
  color: hsl(var(--primary));
  text-decoration: underline;
}
.chat-prose :deep(table) {
  width: 100%;
  margin: 0 0 0.6rem;
  border-collapse: collapse;
  display: block;
  overflow-x: auto;
}
.chat-prose :deep(th),
.chat-prose :deep(td) {
  border: 1px solid hsl(var(--border));
  padding: 0.3rem 0.5rem;
  text-align: left;
}
.chat-prose :deep(blockquote) {
  border-left: 3px solid hsl(var(--border));
  padding-left: 0.75rem;
  margin: 0 0 0.6rem;
  color: hsl(var(--muted-foreground));
}
</style>
