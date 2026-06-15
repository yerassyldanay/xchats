<script setup lang="ts">
import { computed, reactive } from 'vue'
import { useInbox } from '../stores/inbox'
import type { AiDraft, DraftMedia } from '../types'

const inbox = useInbox()

// per-option local edit state (text + kept media), keyed by draft id
const edits = reactive<Record<string, { text: string; media: DraftMedia[] }>>({})
function vm(d: AiDraft) {
  if (!edits[d.id]) edits[d.id] = { text: d.draft_text, media: [...d.media] }
  return edits[d.id]
}
function detach(d: AiDraft, ref: string) {
  const v = vm(d)
  v.media = v.media.filter((m) => m.asset_id !== ref)
}
function approve(d: AiDraft) {
  const v = vm(d)
  inbox.approve(
    d.id,
    v.text,
    v.media.map((m) => m.asset_id)
  )
}
function edit(d: AiDraft) {
  inbox.composerText = vm(d).text
}
function mediaIcon(kind: string) {
  return kind === 'image' ? 'fa-image' : kind === 'audio' ? 'fa-microphone' : 'fa-file-lines'
}

const attrs = computed(() => {
  const a = inbox.activeChat?.contact.attributes || {}
  return Object.entries(a).slice(0, 3)
})
</script>

<template>
  <aside class="flex flex-col bg-white">
    <header class="h-16 px-4 flex items-center justify-between border-b border-hair shrink-0">
      <span class="flex items-center gap-2 font-semibold">
        <span class="w-7 h-7 rounded-lg bg-brand-soft text-brand grid place-items-center">
          <i class="fa-solid fa-wand-magic-sparkles text-sm"></i>
        </span>
        ИИ-помощник
      </span>
    </header>

    <div class="flex-1 overflow-y-auto p-4 space-y-3">
      <template v-if="inbox.activeChat">
        <button
          :disabled="inbox.suggesting"
          class="w-full btn-brand"
          @click="inbox.suggest()"
        >
          <i class="fa-solid fa-wand-magic-sparkles"></i>
          {{ inbox.suggesting ? 'Думаю…' : 'Подсказать ответ' }}
        </button>

        <div v-if="!inbox.drafts.length" class="text-center pt-2">
          <p class="text-xs text-slate-400">ИИ предложит 1–3 варианта ответа</p>
        </div>

        <p v-else class="text-[13px] font-medium text-muted pt-1">{{ inbox.drafts.length }} варианта ответа</p>

        <!-- option cards -->
        <div v-for="d in inbox.drafts" :key="d.id" class="card p-3.5">
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-2">
              <span class="text-[13px] font-semibold">Вариант {{ d.ordinal }}</span>
              <span v-if="d.ordinal === 1" class="badge bg-brand-soft text-brand">Рекомендуется</span>
              <span
                v-if="d.confidence !== null && d.confidence < 0.7"
                class="w-2 h-2 rounded-full bg-amber-400"
                title="Низкая уверенность"
              />
            </div>
            <button class="icon-btn w-7 h-7 text-slate-400" title="Редактировать" @click="edit(d)">
              <i class="fa-solid fa-pen text-xs"></i>
            </button>
          </div>

          <template v-if="d.escalate">
            <p class="text-sm text-slate-600">ИИ не знает — ответьте вручную.</p>
            <p v-if="d.escalation_reason" class="text-xs text-slate-400 mt-1">{{ d.escalation_reason }}</p>
          </template>

          <template v-else>
            <textarea
              v-model="vm(d).text"
              rows="3"
              class="w-full resize-none rounded-xl border border-hair px-3 py-2 text-sm outline-none focus:border-brand focus:ring-4 focus:ring-brand/10 transition"
            />
            <div v-if="vm(d).media.length" class="mt-2 space-y-1.5">
              <div
                v-for="md in vm(d).media"
                :key="md.asset_id"
                class="flex items-center gap-2 rounded-xl bg-panel border border-hair px-3 py-2 text-xs"
              >
                <i class="fa-solid text-slate-500" :class="mediaIcon(md.media_kind)"></i>
                <span class="truncate flex-1">{{ md.asset_id }}</span>
                <button class="text-slate-400 hover:text-red-500" @click="detach(d, md.asset_id)">
                  <i class="fa-solid fa-xmark"></i>
                </button>
              </div>
            </div>
            <button class="mt-2 w-full flex items-center justify-center gap-2 rounded-xl border border-dashed border-hair py-2 text-[13px] text-brand hover:bg-brand-soft transition">
              <i class="fa-solid fa-plus"></i> Добавить файл
            </button>
            <div class="mt-3 flex items-center gap-2">
              <button class="flex-1 btn-wa btn-sm" @click="approve(d)">
                <i class="fa-solid fa-paper-plane"></i> Принять и отправить
              </button>
              <button class="btn-ghost btn-sm" @click="edit(d)">Редактировать</button>
            </div>
          </template>
        </div>

        <button
          v-if="inbox.drafts.length"
          class="w-full text-center text-[13px] text-muted hover:text-red-500 py-1 transition"
          @click="inbox.drafts = []"
        >
          Отклонить
        </button>
      </template>

      <div v-else class="text-center text-sm text-slate-400 pt-10">
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
      <div class="text-sm text-muted">
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
