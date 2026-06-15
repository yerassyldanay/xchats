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

const attrs = computed(() => {
  const a = inbox.activeChat?.contact.attributes || {}
  return Object.entries(a).slice(0, 3)
})
</script>

<template>
  <aside class="flex flex-col bg-white">
    <header class="h-14 px-4 flex items-center border-b border-hair shrink-0">
      <span class="font-semibold">Подсказки ИИ</span>
    </header>

    <div class="flex-1 overflow-y-auto p-3 space-y-3">
      <template v-if="inbox.activeChat">
        <!-- empty state -->
        <div v-if="!inbox.drafts.length" class="text-center py-6">
          <button
            :disabled="inbox.suggesting"
            class="rounded-lg bg-brand text-white px-4 py-2.5 font-medium hover:opacity-90 disabled:opacity-60"
            @click="inbox.suggest()"
          >
            {{ inbox.suggesting ? 'Думаю…' : 'Подсказать ответ' }}
          </button>
          <p class="mt-3 text-xs text-slate-400">ИИ предложит 1–3 варианта ответа</p>
        </div>

        <!-- option cards -->
        <div
          v-for="d in inbox.drafts"
          :key="d.id"
          class="rounded-xl border border-hair p-3 shadow-sm"
        >
          <div class="flex items-center justify-between mb-1">
            <span class="text-xs font-semibold text-brand">Вариант {{ d.ordinal }}</span>
            <span
              v-if="d.confidence !== null && d.confidence < 0.7"
              class="w-2 h-2 rounded-full bg-amber-400"
              title="Низкая уверенность"
            />
          </div>

          <template v-if="d.escalate">
            <p class="text-sm text-slate-600">ИИ не знает — ответьте вручную.</p>
            <p v-if="d.escalation_reason" class="text-xs text-slate-400 mt-1">{{ d.escalation_reason }}</p>
          </template>

          <template v-else>
            <textarea
              v-model="vm(d).text"
              rows="3"
              class="w-full resize-none rounded-lg border border-hair px-2 py-1.5 text-sm outline-none focus:border-brand"
            />
            <div v-if="vm(d).media.length" class="mt-2 flex flex-wrap gap-1.5">
              <span
                v-for="md in vm(d).media"
                :key="md.asset_id"
                class="flex items-center gap-1 rounded-full bg-panel border border-hair px-2.5 py-1 text-xs"
              >
                {{ md.media_kind === 'image' ? '🖼' : md.media_kind === 'audio' ? '🎙' : '📄' }}
                {{ md.asset_id }}
                <button class="text-slate-400 hover:text-red-500" @click="detach(d, md.asset_id)">×</button>
              </span>
            </div>
            <div class="mt-2 flex items-center gap-2">
              <button
                class="flex-1 rounded-lg bg-wa text-white text-sm py-1.5 font-medium hover:opacity-90"
                @click="approve(d)"
              >
                Принять и отправить
              </button>
              <button class="text-sm text-brand hover:underline" @click="edit(d)">Редактировать</button>
            </div>
          </template>
        </div>
      </template>

      <p v-else class="text-center text-sm text-slate-400 py-6">Выберите чат</p>
    </div>

    <!-- contact mini-profile -->
    <div v-if="inbox.activeChat" class="border-t border-hair p-4 shrink-0">
      <div class="text-xs uppercase tracking-wide text-slate-400 mb-1">Контакт</div>
      <div class="font-medium">{{ inbox.activeChat.contact.display_name }}</div>
      <div class="text-sm text-slate-500">
        {{ inbox.activeChat.contact.phone_number || inbox.activeChat.contact.phone_jid }}
      </div>
      <dl v-if="attrs.length" class="mt-2 space-y-0.5">
        <div v-for="[k, val] in attrs" :key="k" class="flex justify-between text-xs">
          <dt class="text-slate-400">{{ k }}</dt>
          <dd class="text-slate-600">{{ val }}</dd>
        </div>
      </dl>
    </div>
  </aside>
</template>
