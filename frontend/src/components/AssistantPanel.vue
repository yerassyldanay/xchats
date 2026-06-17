<script setup lang="ts">
import { computed, reactive } from 'vue'
import { useInbox } from '../stores/inbox'
import { vAutosize } from '../lib/autosize'
import type { AiDraft, DraftMedia } from '../types'

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

const mediaMeta: Record<string, { icon: string; label: string; tile: string }> = {
  image: { icon: 'fa-image', label: 'Изображение', tile: 'bg-violet-100 text-violet-600' },
  audio: { icon: 'fa-microphone', label: 'Аудио', tile: 'bg-sky-100 text-sky-600' },
  video: { icon: 'fa-film', label: 'Видео', tile: 'bg-rose-100 text-rose-600' },
  document: { icon: 'fa-file-lines', label: 'Документ', tile: 'bg-amber-100 text-amber-600' },
}
function media(kind: string) {
  return mediaMeta[kind] ?? mediaMeta.document
}
function conf(d: AiDraft) {
  if (d.confidence === null || d.confidence === undefined) return null
  const pct = Math.round(d.confidence * 100)
  if (d.confidence >= 0.8) return { pct, cls: 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200/70' }
  if (d.confidence >= 0.5) return { pct, cls: 'bg-amber-50 text-amber-700 ring-1 ring-amber-200/70' }
  return { pct, cls: 'bg-rose-50 text-rose-700 ring-1 ring-rose-200/70' }
}

const attrs = computed(() => Object.entries(inbox.activeChat?.contact.attributes || {}).slice(0, 3))
const hasDrafts = computed(() => inbox.drafts.length > 0)
</script>

<template>
  <aside class="flex flex-col bg-white">
    <header class="h-16 px-4 flex items-center justify-between border-b border-hair shrink-0">
      <span class="flex items-center gap-2.5 font-semibold">
        <span class="w-8 h-8 rounded-xl bg-gradient-to-br from-brand to-violet-500 text-white grid place-items-center shadow-sm">
          <i class="fa-solid fa-wand-magic-sparkles text-[13px]"></i>
        </span>
        ИИ-помощник
      </span>
      <button
        v-if="inbox.activeChat && hasDrafts"
        :disabled="inbox.suggesting"
        class="icon-btn w-8 h-8 text-slate-400"
        title="Сгенерировать заново"
        @click="inbox.regenerate()"
      >
        <i class="fa-solid fa-rotate text-xs" :class="{ 'fa-spin': inbox.suggesting }"></i>
      </button>
    </header>

    <div class="flex-1 overflow-y-auto p-4 space-y-3">
      <template v-if="inbox.activeChat">
        <!-- generating shimmer -->
        <div v-if="inbox.suggesting && !hasDrafts" class="card p-4 space-y-3 animate-pulse">
          <div class="flex items-center gap-2">
            <span class="w-6 h-6 rounded-lg bg-brand-soft"></span>
            <span class="h-3 w-28 rounded-full bg-slate-100"></span>
          </div>
          <div class="space-y-2">
            <span class="block h-3 w-full rounded-full bg-slate-100"></span>
            <span class="block h-3 w-11/12 rounded-full bg-slate-100"></span>
            <span class="block h-3 w-2/3 rounded-full bg-slate-100"></span>
          </div>
          <p class="text-xs text-muted-foreground flex items-center gap-2">
            <i class="fa-solid fa-wand-magic-sparkles text-brand"></i> ИИ готовит ответ…
          </p>
        </div>

        <!-- option cards -->
        <article
          v-for="(d, i) in inbox.drafts"
          :key="d.id"
          class="card overflow-hidden"
        >
          <!-- card header -->
          <div class="flex items-center justify-between gap-2 px-4 pt-3.5 pb-2.5">
            <div class="flex items-center gap-2 min-w-0">
              <span class="text-[13px] font-semibold text-ink truncate">
                {{ i === 0 ? 'Рекомендуемый ответ' : 'Вариант ' + d.ordinal }}
              </span>
              <span v-if="conf(d)" class="badge text-[11px] px-2 py-0.5" :class="conf(d)!.cls">
                {{ conf(d)!.pct }}%
              </span>
            </div>
            <button class="icon-btn w-7 h-7 text-slate-400" title="В поле ввода" @click="toComposer(d)">
              <i class="fa-solid fa-arrow-right-from-bracket text-[11px]"></i>
            </button>
          </div>

          <!-- editable reply: grows to fit the whole text, never scrolls -->
          <div class="px-4 pt-1.5">
            <textarea
              v-model="vm(d).text"
              v-autosize
              rows="2"
              placeholder="Текст ответа…"
              class="w-full resize-none overflow-hidden rounded-xl border border-hair bg-panel/40 px-3 py-2.5 text-[14px] leading-snug text-ink outline-none transition focus:bg-white focus:border-brand focus:ring-4 focus:ring-brand/10"
            />
          </div>

          <!-- attachments -->
          <div v-if="vm(d).media.length" class="px-4 pt-2.5 space-y-1.5">
            <div
              v-for="md in vm(d).media"
              :key="md.asset_id"
              class="group flex items-center gap-2.5 rounded-xl border border-hair bg-white px-2.5 py-2"
            >
              <span class="w-8 h-8 rounded-lg grid place-items-center shrink-0" :class="media(md.media_kind).tile">
                <i class="fa-solid text-sm" :class="media(md.media_kind).icon"></i>
              </span>
              <div class="min-w-0 flex-1">
                <div class="text-[13px] font-medium text-ink truncate">{{ media(md.media_kind).label }}</div>
                <div class="text-[11px] text-muted-foreground truncate">{{ md.asset_id }}</div>
              </div>
              <button
                class="w-7 h-7 rounded-lg grid place-items-center text-slate-300 hover:text-rose-500 hover:bg-rose-50 transition"
                title="Открепить"
                @click="detach(d, md.asset_id)"
              >
                <i class="fa-solid fa-xmark text-xs"></i>
              </button>
            </div>
          </div>

          <!-- actions -->
          <div class="flex items-center gap-2 px-4 py-3 mt-1">
            <button class="flex-1 btn-wa btn-sm" :disabled="busy[d.id] || !vm(d).text.trim()" @click="approve(d)">
              <i class="fa-solid" :class="busy[d.id] ? 'fa-circle-notch fa-spin' : 'fa-paper-plane'"></i>
              {{ busy[d.id] ? 'Отправка…' : 'Отправить' }}
            </button>
            <button
              class="btn-ghost btn-sm px-2.5"
              :disabled="inbox.suggesting"
              title="Сгенерировать заново"
              @click="inbox.regenerate()"
            >
              <i class="fa-solid fa-rotate" :class="{ 'fa-spin': inbox.suggesting }"></i>
            </button>
          </div>
        </article>

        <!-- empty state + generate button -->
        <div v-if="!hasDrafts && !inbox.suggesting" class="pt-2">
          <div class="text-center px-2 pb-3">
            <div class="mx-auto w-11 h-11 rounded-2xl bg-brand-soft text-brand grid place-items-center mb-2.5">
              <i class="fa-solid fa-wand-magic-sparkles"></i>
            </div>
            <p class="text-[13px] font-medium text-ink">Ответ ещё не предложен</p>
            <p class="text-xs text-muted-foreground mt-0.5">Сгенерируйте черновик на основе базы знаний.</p>
          </div>
          <button class="w-full btn-brand" @click="inbox.suggest()">
            <i class="fa-solid fa-wand-magic-sparkles"></i> Подсказать ответ
          </button>
        </div>

        <button
          v-if="hasDrafts"
          class="w-full text-center text-[13px] text-muted-foreground hover:text-rose-500 py-1 transition"
          @click="inbox.drafts = []"
        >
          Отклонить
        </button>
      </template>

      <div v-else class="text-center text-sm text-slate-400 pt-12">
        <div class="mx-auto w-12 h-12 rounded-2xl bg-panel grid place-items-center text-slate-300 text-xl mb-3">
          <i class="fa-solid fa-wand-magic-sparkles"></i>
        </div>
        Выберите чат
      </div>
    </div>

    <!-- contact mini-profile -->
    <div v-if="inbox.activeChat" class="border-t border-hair p-4 shrink-0">
      <div class="text-[11px] uppercase tracking-wide text-slate-400 mb-1.5">Контакт</div>
      <div class="font-semibold">{{ inbox.activeChat.contact.display_name }}</div>
      <div class="text-sm text-muted-foreground">
        {{ inbox.activeChat.contact.phone_number || inbox.activeChat.contact.phone_jid }}
      </div>
      <dl v-if="attrs.length" class="mt-2.5 space-y-1">
        <div v-for="[k, val] in attrs" :key="k" class="flex justify-between text-xs">
          <dt class="text-slate-400">{{ k }}</dt>
          <dd class="text-slate-600 font-medium">{{ val }}</dd>
        </div>
      </dl>
    </div>
  </aside>
</template>
