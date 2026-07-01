<script setup lang="ts">
import { computed, reactive, type Component } from 'vue'
import { FileText, Film, Image, LoaderCircle, Mic, PenLine, RotateCw, Send, WandSparkles, X } from 'lucide-vue-next'
import { useInbox } from '../stores/inbox'
import { vAutosize } from '../lib/autosize'
import type { AiDraft, DraftMedia } from '../types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Textarea } from '@/components/ui/textarea'
import { Skeleton } from '@/components/ui/skeleton'

const inbox = useInbox()

// per-option local edit state (text + kept media), keyed by draft id
const edits = reactive<Record<string, { text: string; media: DraftMedia[] }>>({})
const busy = reactive<Record<string, boolean>>({})

function vm(d: AiDraft) {
  if (!edits[d.id]) edits[d.id] = { text: d.draft_text, media: [...d.media] }
  return edits[d.id]
}
function detach(d: AiDraft, ref: string) {
  vm(d).media = vm(d).media.filter((m) => m.asset_id !== ref)
}
async function approve(d: AiDraft) {
  busy[d.id] = true
  try {
    const v = vm(d)
    await inbox.approve(d.id, v.text, v.media.map((m) => m.asset_id))
  } finally {
    busy[d.id] = false
  }
}
function toComposer(d: AiDraft) {
  inbox.composerText = vm(d).text
}

const mediaMeta: Record<string, { icon: Component; label: string; tile: string }> = {
  image: { icon: Image, label: 'Изображение', tile: 'bg-violet-500/10 text-violet-600 dark:text-violet-400' },
  audio: { icon: Mic, label: 'Аудио', tile: 'bg-sky-500/10 text-sky-600 dark:text-sky-400' },
  video: { icon: Film, label: 'Видео', tile: 'bg-rose-500/10 text-rose-600 dark:text-rose-400' },
  document: { icon: FileText, label: 'Документ', tile: 'bg-amber-500/10 text-amber-600 dark:text-amber-400' },
}
function media(kind: string) {
  return mediaMeta[kind] ?? mediaMeta.document
}
function conf(d: AiDraft) {
  if (d.confidence === null || d.confidence === undefined) return null
  const pct = Math.round(d.confidence * 100)
  if (d.confidence >= 0.8) return { pct, cls: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 ring-1 ring-emerald-500/20' }
  if (d.confidence >= 0.5) return { pct, cls: 'bg-amber-500/10 text-amber-700 dark:text-amber-400 ring-1 ring-amber-500/20' }
  return { pct, cls: 'bg-rose-500/10 text-rose-700 dark:text-rose-400 ring-1 ring-rose-500/20' }
}

const attrs = computed(() => Object.entries(inbox.activeChat?.contact.attributes || {}).slice(0, 3))
const hasDrafts = computed(() => inbox.drafts.length > 0)
</script>

<template>
  <aside class="flex flex-col bg-card">
    <header class="h-16 px-4 flex items-center justify-between border-b border-border shrink-0">
      <span class="flex items-center gap-2.5 font-semibold">
        <span class="w-8 h-8 rounded-lg bg-primary text-primary-foreground grid place-items-center">
          <WandSparkles class="w-4 h-4" />
        </span>
        ИИ-помощник
      </span>
      <Button
        v-if="inbox.activeChat && hasDrafts"
        variant="ghost"
        size="icon"
        class="w-8 h-8 text-muted-foreground"
        :disabled="inbox.suggesting"
        title="Сгенерировать заново"
        @click="inbox.regenerate()"
      >
        <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': inbox.suggesting }" />
      </Button>
    </header>

    <div class="flex-1 overflow-y-auto p-4 space-y-3">
      <template v-if="inbox.activeChat">
        <!-- generating shimmer -->
        <div v-if="inbox.suggesting && !hasDrafts" class="rounded-lg border border-border bg-card p-4 space-y-3">
          <div class="flex items-center gap-2">
            <Skeleton class="w-6 h-6 rounded-md" />
            <Skeleton class="h-3 w-28" />
          </div>
          <div class="space-y-2">
            <Skeleton class="h-3 w-full" />
            <Skeleton class="h-3 w-11/12" />
            <Skeleton class="h-3 w-2/3" />
          </div>
          <p class="text-xs text-muted-foreground flex items-center gap-2">
            <WandSparkles class="w-3.5 h-3.5 text-primary" /> ИИ готовит ответ…
          </p>
        </div>

        <!-- option cards -->
        <article
          v-for="(d, i) in inbox.drafts"
          :key="d.id"
          class="rounded-lg border border-border bg-card overflow-hidden"
        >
          <!-- card header -->
          <div class="flex items-center justify-between gap-2 px-4 pt-3.5 pb-2.5">
            <div class="flex items-center gap-2 min-w-0">
              <span class="text-[13px] font-semibold truncate">
                {{ i === 0 ? 'Рекомендуемый ответ' : 'Вариант ' + d.ordinal }}
              </span>
              <Badge v-if="conf(d)" variant="secondary" class="text-[11px] px-2 py-0.5" :class="conf(d)!.cls">
                {{ conf(d)!.pct }}%
              </Badge>
            </div>
            <Button variant="ghost" size="icon" class="w-7 h-7 text-muted-foreground" title="В поле ввода" @click="toComposer(d)">
              <PenLine class="w-3.5 h-3.5" />
            </Button>
          </div>

          <!-- editable reply: grows to fit the whole text, never scrolls -->
          <div class="px-4 pt-1.5">
            <Textarea
              v-model="vm(d).text"
              v-autosize
              rows="2"
              placeholder="Текст ответа…"
              class="min-h-0 resize-none overflow-hidden rounded-lg bg-muted/40 text-[14px] leading-snug"
            />
          </div>

          <!-- attachments -->
          <div v-if="vm(d).media.length" class="px-4 pt-2.5 space-y-1.5">
            <div
              v-for="md in vm(d).media"
              :key="md.asset_id"
              class="flex items-center gap-2.5 rounded-lg border border-border bg-background px-2.5 py-2"
            >
              <span class="w-8 h-8 rounded-md grid place-items-center shrink-0" :class="media(md.media_kind).tile">
                <component :is="media(md.media_kind).icon" class="w-4 h-4" />
              </span>
              <div class="min-w-0 flex-1">
                <div class="text-[13px] font-medium truncate">{{ media(md.media_kind).label }}</div>
                <div class="text-[11px] text-muted-foreground truncate">{{ md.asset_id }}</div>
              </div>
              <button
                class="w-7 h-7 rounded-md grid place-items-center text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition"
                title="Открепить"
                @click="detach(d, md.asset_id)"
              >
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <!-- actions -->
          <div class="flex items-center gap-2 px-4 py-3 mt-1">
            <Button size="sm" class="flex-1" :disabled="busy[d.id] || !vm(d).text.trim()" @click="approve(d)">
              <LoaderCircle v-if="busy[d.id]" class="w-4 h-4 animate-spin" />
              <Send v-else class="w-4 h-4" />
              {{ busy[d.id] ? 'Отправка…' : 'Отправить' }}
            </Button>
            <Button
              variant="outline"
              size="sm"
              class="px-2.5"
              :disabled="inbox.suggesting"
              title="Сгенерировать заново"
              @click="inbox.regenerate()"
            >
              <RotateCw class="w-4 h-4" :class="{ 'animate-spin': inbox.suggesting }" />
            </Button>
          </div>
        </article>

        <!-- empty state + generate button -->
        <div v-if="!hasDrafts && !inbox.suggesting" class="pt-2">
          <div class="text-center px-2 pb-3">
            <div class="mx-auto w-11 h-11 rounded-xl bg-primary/10 text-primary grid place-items-center mb-2.5">
              <WandSparkles class="w-5 h-5" />
            </div>
            <p class="text-[13px] font-medium">Ответ ещё не предложен</p>
            <p class="text-xs text-muted-foreground mt-0.5">Сгенерируйте черновик на основе базы знаний.</p>
          </div>
          <Button class="w-full" @click="inbox.suggest()">
            <WandSparkles class="w-4 h-4" /> Подсказать ответ
          </Button>
        </div>

        <button
          v-if="hasDrafts"
          class="w-full text-center text-[13px] text-muted-foreground hover:text-destructive py-1 transition"
          @click="inbox.drafts = []"
        >
          Отклонить
        </button>
      </template>

      <div v-else class="text-center text-sm text-muted-foreground pt-12">
        <div class="mx-auto w-12 h-12 rounded-xl bg-muted grid place-items-center text-muted-foreground mb-3">
          <WandSparkles class="w-6 h-6" />
        </div>
        Выберите чат
      </div>
    </div>

    <!-- contact mini-profile -->
    <div v-if="inbox.activeChat" class="border-t border-border p-4 shrink-0">
      <div class="text-[11px] uppercase tracking-wide text-muted-foreground mb-1.5">Контакт</div>
      <div class="font-semibold">{{ inbox.activeChat.contact.display_name }}</div>
      <div class="text-sm text-muted-foreground">
        {{ inbox.activeChat.contact.phone_number || inbox.activeChat.contact.phone_jid }}
      </div>
      <dl v-if="attrs.length" class="mt-2.5 space-y-1">
        <div v-for="[k, val] in attrs" :key="k" class="flex justify-between text-xs">
          <dt class="text-muted-foreground">{{ k }}</dt>
          <dd class="font-medium">{{ val }}</dd>
        </div>
      </dl>
    </div>
  </aside>
</template>
