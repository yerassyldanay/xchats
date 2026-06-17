<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { CircleAlert, FileText, LoaderCircle, Plus, Save, Trash2, WandSparkles } from 'lucide-vue-next'
import { usePlayground } from '../stores/playground'
import { shortTime } from '../lib/format'
import { api } from '../api/client'
import type { AssetRow, TopicRow, ValueRow } from '../types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'

const pg = usePlayground()
const tab = ref('overview')

onMounted(async () => {
  await pg.load()
  pg.startRealtime()
})
onBeforeUnmount(() => pg.stopRealtime())

// --- config (Обзор) ---
const cfg = reactive({ persona: '', mission: '', guardrails: '', language_policy: '', reply_max_words: 0 })
function seedCfg() {
  const c = pg.draft?.config
  if (!c) return
  cfg.persona = c.persona
  cfg.mission = c.mission
  cfg.guardrails = c.guardrails
  cfg.language_policy = c.language_policy
  cfg.reply_max_words = c.reply_max_words
}
onMounted(seedCfg)
async function saveConfig() {
  await pg.patchConfig({ ...cfg })
}

// --- per-row edit buffers (lazy; persist until saved) ---
const tBuf = reactive<Record<string, { title: string; keywords: string; body_md: string; lang: string }>>({})
function vmTopic(t: TopicRow) {
  if (!tBuf[t.id]) tBuf[t.id] = { title: t.title, keywords: t.keywords, body_md: t.body_md, lang: t.lang || 'ru' }
  return tBuf[t.id]
}
const aBuf = reactive<Record<string, { description: string; topic_slug: string }>>({})
function vmAsset(a: AssetRow) {
  if (!aBuf[a.id]) aBuf[a.id] = { description: a.description, topic_slug: a.topic_slug }
  return aBuf[a.id]
}
const vBuf = reactive<Record<string, { value_text: string; description: string }>>({})
function vmValue(v: ValueRow) {
  if (!vBuf[v.id]) vBuf[v.id] = { value_text: v.value_text, description: v.description }
  return vBuf[v.id]
}

// --- new-row forms ---
const newTopic = reactive({ slug: '', title: '', keywords: '', body_md: '' })
async function addTopic() {
  if (!newTopic.slug.trim()) return
  await pg.upsertTopic({ ...newTopic })
  newTopic.slug = newTopic.title = newTopic.keywords = newTopic.body_md = ''
}
const newValue = reactive({ token: '', value_text: '', description: '' })
async function addValue() {
  if (!newValue.token.trim()) return
  await pg.upsertValue({ ...newValue })
  newValue.token = newValue.value_text = newValue.description = ''
}
const assetFile = ref<HTMLInputElement | null>(null)
const assetTopic = ref('')
async function uploadAsset(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (f) await pg.uploadAsset(f, { topic_slug: assetTopic.value || undefined })
  if (assetFile.value) assetFile.value.value = ''
}

// --- review badge ---
const reviewMeta: Record<string, { label: string; cls: string }> = {
  proposed: { label: 'на проверке', cls: 'bg-amber-500/10 text-amber-600 dark:text-amber-400' },
  approved: { label: 'подтверждено', cls: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' },
  rejected: { label: 'отклонено', cls: 'bg-destructive/10 text-destructive' },
}

// --- Правки (pending review across kinds) ---
type Pending = { kind: 'topics' | 'assets' | 'values'; id: string; label: string }
const pendingRows = computed<Pending[]>(() => {
  const d = pg.draft
  if (!d) return []
  const out: Pending[] = []
  for (const t of d.topics) if (t.review_state === 'proposed') out.push({ kind: 'topics', id: t.id, label: 'Тема · ' + (t.title || t.slug) })
  for (const a of d.assets) if (a.review_state === 'proposed') out.push({ kind: 'assets', id: a.id, label: 'Медиа · ' + (a.title || a.ref) })
  for (const v of d.values) if (v.review_state === 'proposed') out.push({ kind: 'values', id: v.id, label: 'Значение · ' + v.token })
  return out
})

// --- Последние изменения ---
const recent = computed(() => {
  const d = pg.draft
  if (!d) return [] as { label: string; at: string }[]
  const rows = [
    ...d.topics.map((t) => ({ label: 'Тема · ' + (t.title || t.slug), at: t.updated_at })),
    ...d.assets.map((a) => ({ label: 'Медиа · ' + (a.title || a.ref), at: a.updated_at })),
    ...d.values.map((v) => ({ label: 'Значение · ' + v.token, at: v.updated_at })),
  ]
  return rows.sort((a, b) => (b.at || '').localeCompare(a.at || '')).slice(0, 6)
})

const tabs = [
  { key: 'overview', label: 'Обзор' },
  { key: 'topics', label: 'Темы' },
  { key: 'assets', label: 'Медиа-ресурсы' },
  { key: 'values', label: 'Значения' },
  { key: 'review', label: 'Правки' },
]
function isImage(a: AssetRow) {
  return a.kind === 'image'
}
</script>

<template>
  <div class="flex h-full bg-background">
    <div class="flex-1 flex flex-col min-w-0">
      <header class="px-8 py-4 flex items-center justify-between border-b border-border bg-card shrink-0">
        <div>
          <h1 class="text-lg font-bold tracking-tight">Редактор базы знаний</h1>
          <p class="text-sm text-muted-foreground">Управляйте темами, медиа и значениями</p>
        </div>
        <Button v-if="pg.hasDraft" size="sm" :disabled="pg.publishing" @click="pg.publish()">
          <LoaderCircle v-if="pg.publishing" class="w-4 h-4 animate-spin" />
          <Save v-else class="w-4 h-4" />
          Сохранить в базу
        </Button>
      </header>

      <!-- empty -->
      <div v-if="!pg.hasDraft && !pg.loading" class="flex-1 grid place-items-center p-8">
        <div class="text-center max-w-sm">
          <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
            <WandSparkles class="w-6 h-6" />
          </div>
          <p class="font-medium">Черновик не открыт</p>
          <p class="text-sm text-muted-foreground mt-0.5 mb-4">Откройте черновик, чтобы редактировать базу знаний.</p>
          <Button :disabled="pg.busy" @click="pg.open()"><WandSparkles class="w-4 h-4" /> Открыть черновик</Button>
        </div>
      </div>

      <div v-else class="flex-1 overflow-y-auto px-8 py-6 space-y-6">
        <!-- stat cards -->
        <div class="grid grid-cols-4 gap-4">
          <div class="rounded-lg border border-border bg-card p-4">
            <div class="text-2xl font-bold leading-none">{{ pg.counts.topics }}</div>
            <div class="text-sm text-muted-foreground mt-1">Темы</div>
          </div>
          <div class="rounded-lg border border-border bg-card p-4">
            <div class="text-2xl font-bold leading-none">{{ pg.counts.assets }}</div>
            <div class="text-sm text-muted-foreground mt-1">Медиа-ресурсы</div>
          </div>
          <div class="rounded-lg border border-border bg-card p-4">
            <div class="text-2xl font-bold leading-none">{{ pg.counts.values }}</div>
            <div class="text-sm text-muted-foreground mt-1">Значения</div>
          </div>
          <div class="rounded-lg border border-border bg-card p-4">
            <div class="text-2xl font-bold leading-none">{{ pg.pending }}</div>
            <div class="text-sm text-muted-foreground mt-1">Правки</div>
          </div>
        </div>

        <Tabs :model-value="tab" @update:model-value="(v) => (tab = v as string)">
          <TabsList>
            <TabsTrigger v-for="t in tabs" :key="t.key" :value="t.key" class="flex-none px-4">{{ t.label }}</TabsTrigger>
          </TabsList>

          <!-- Обзор: config -->
          <TabsContent value="overview" class="space-y-3 max-w-2xl">
            <div>
              <label class="text-xs font-medium text-muted-foreground">Персона</label>
              <Textarea v-model="cfg.persona" rows="2" class="mt-1 min-h-0" />
            </div>
            <div>
              <label class="text-xs font-medium text-muted-foreground">Миссия</label>
              <Textarea v-model="cfg.mission" rows="2" class="mt-1 min-h-0" />
            </div>
            <div>
              <label class="text-xs font-medium text-muted-foreground">Ограничения (guardrails)</label>
              <Textarea v-model="cfg.guardrails" rows="2" class="mt-1 min-h-0" />
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="text-xs font-medium text-muted-foreground">Языковая политика</label>
                <Input v-model="cfg.language_policy" class="mt-1" />
              </div>
              <div>
                <label class="text-xs font-medium text-muted-foreground">Макс. слов в ответе</label>
                <Input v-model.number="cfg.reply_max_words" type="number" class="mt-1" />
              </div>
            </div>
            <Button size="sm" :disabled="pg.busy" @click="saveConfig"><Save class="w-4 h-4" /> Сохранить настройки</Button>
          </TabsContent>

          <!-- Темы -->
          <TabsContent value="topics" class="space-y-3">
            <div class="rounded-lg border border-dashed border-border p-3 grid grid-cols-1 sm:grid-cols-2 gap-2">
              <Input v-model="newTopic.slug" placeholder="slug (напр. tariffs)" class="h-9" />
              <Input v-model="newTopic.title" placeholder="Название" class="h-9" />
              <Input v-model="newTopic.keywords" placeholder="Ключевые слова" class="h-9" />
              <Button size="sm" :disabled="pg.busy || !newTopic.slug.trim()" @click="addTopic"><Plus class="w-4 h-4" /> Добавить тему</Button>
            </div>
            <p v-if="!pg.draft?.topics.length" class="text-sm text-muted-foreground py-6 text-center">Тем пока нет.</p>
            <div v-for="t in pg.draft?.topics" :key="t.id" class="rounded-lg border border-border bg-card p-4 space-y-2">
              <div class="flex items-center justify-between gap-2">
                <code class="text-[13px] font-mono font-medium">{{ t.slug }}</code>
                <div class="flex items-center gap-2">
                  <Badge variant="secondary" :class="reviewMeta[t.review_state]?.cls">{{ reviewMeta[t.review_state]?.label }}</Badge>
                  <Button variant="ghost" size="icon" class="w-8 h-8 text-destructive hover:bg-destructive/10" :disabled="pg.busy" @click="pg.deleteTopic(t.slug)"><Trash2 class="w-4 h-4" /></Button>
                </div>
              </div>
              <Input v-model="vmTopic(t).title" placeholder="Название" class="h-9" />
              <Input v-model="vmTopic(t).keywords" placeholder="Ключевые слова" class="h-9" />
              <Textarea v-model="vmTopic(t).body_md" rows="3" placeholder="Текст темы…" class="min-h-0 text-[14px]" />
              <div class="flex items-center gap-2">
                <Button size="sm" :disabled="pg.busy" @click="pg.upsertTopic({ slug: t.slug, ...vmTopic(t) })"><Save class="w-4 h-4" /> Сохранить</Button>
                <template v-if="t.review_state === 'proposed'">
                  <Button size="sm" variant="outline" :disabled="pg.busy" @click="pg.review('topics', t.id, 'approved')">Подтвердить</Button>
                  <Button size="sm" variant="ghost" class="text-destructive" :disabled="pg.busy" @click="pg.review('topics', t.id, 'rejected')">Отклонить</Button>
                </template>
              </div>
            </div>
          </TabsContent>

          <!-- Медиа-ресурсы -->
          <TabsContent value="assets" class="space-y-3">
            <div class="rounded-lg border border-dashed border-border p-3 flex items-center gap-2">
              <Input v-model="assetTopic" placeholder="Тема (slug, необязательно)" class="h-9 flex-1" />
              <input ref="assetFile" type="file" class="hidden" @change="uploadAsset" />
              <Button size="sm" variant="outline" :disabled="pg.busy" @click="assetFile?.click()"><Plus class="w-4 h-4" /> Загрузить медиа</Button>
            </div>
            <p v-if="!pg.draft?.assets.length" class="text-sm text-muted-foreground py-6 text-center">Медиа-ресурсов пока нет.</p>
            <div v-for="a in pg.draft?.assets" :key="a.id" class="rounded-lg border border-border bg-card p-4 flex gap-3">
              <div class="w-16 h-16 rounded-md border border-border bg-muted grid place-items-center overflow-hidden shrink-0 text-muted-foreground">
                <img v-if="isImage(a)" :src="api.mediaURL(a.url)" :alt="a.title" class="w-full h-full object-cover" />
                <FileText v-else class="w-6 h-6" />
              </div>
              <div class="min-w-0 flex-1 space-y-2">
                <div class="flex items-center justify-between gap-2">
                  <span class="text-[13px] font-medium truncate">{{ a.title || a.ref }}</span>
                  <div class="flex items-center gap-2">
                    <Badge variant="secondary" :class="reviewMeta[a.review_state]?.cls">{{ reviewMeta[a.review_state]?.label }}</Badge>
                    <Button variant="ghost" size="icon" class="w-8 h-8 text-destructive hover:bg-destructive/10" :disabled="pg.busy" @click="pg.deleteAsset(a.ref)"><Trash2 class="w-4 h-4" /></Button>
                  </div>
                </div>
                <Input v-model="vmAsset(a).topic_slug" placeholder="Тема (slug)" class="h-9" />
                <Textarea v-model="vmAsset(a).description" rows="2" placeholder="Описание…" class="min-h-0 text-[13px]" />
                <div class="flex items-center gap-2">
                  <Button size="sm" :disabled="pg.busy" @click="pg.patchAsset(a.ref, { ...vmAsset(a) })"><Save class="w-4 h-4" /> Сохранить</Button>
                  <template v-if="a.review_state === 'proposed'">
                    <Button size="sm" variant="outline" :disabled="pg.busy" @click="pg.review('assets', a.id, 'approved')">Подтвердить</Button>
                    <Button size="sm" variant="ghost" class="text-destructive" :disabled="pg.busy" @click="pg.review('assets', a.id, 'rejected')">Отклонить</Button>
                  </template>
                </div>
              </div>
            </div>
          </TabsContent>

          <!-- Значения -->
          <TabsContent value="values" class="space-y-3">
            <div class="rounded-lg border border-dashed border-border p-3 grid grid-cols-1 sm:grid-cols-2 gap-2">
              <Input v-model="newValue.token" placeholder="TOKEN (напр. PRICE_BASIC)" class="h-9 font-mono" />
              <Input v-model="newValue.value_text" placeholder="Значение (напр. 5000 ₸)" class="h-9" />
              <Input v-model="newValue.description" placeholder="Описание" class="h-9" />
              <Button size="sm" :disabled="pg.busy || !newValue.token.trim()" @click="addValue"><Plus class="w-4 h-4" /> Добавить значение</Button>
            </div>
            <p v-if="!pg.draft?.values.length" class="text-sm text-muted-foreground py-6 text-center">Значений пока нет.</p>
            <div v-for="v in pg.draft?.values" :key="v.id" class="rounded-lg border border-border bg-card p-4 space-y-2">
              <div class="flex items-center justify-between gap-2">
                <code class="text-[13px] font-mono font-medium">{{ v.token }}</code>
                <div class="flex items-center gap-2">
                  <Badge variant="secondary" :class="reviewMeta[v.review_state]?.cls">{{ reviewMeta[v.review_state]?.label }}</Badge>
                  <Button variant="ghost" size="icon" class="w-8 h-8 text-destructive hover:bg-destructive/10" :disabled="pg.busy" @click="pg.deleteValue(v.token, v.lang)"><Trash2 class="w-4 h-4" /></Button>
                </div>
              </div>
              <Input v-model="vmValue(v).value_text" placeholder="Значение" class="h-9" />
              <Input v-model="vmValue(v).description" placeholder="Описание" class="h-9" />
              <div class="flex items-center gap-2">
                <Button size="sm" :disabled="pg.busy" @click="pg.upsertValue({ token: v.token, lang: v.lang, ...vmValue(v) })"><Save class="w-4 h-4" /> Сохранить</Button>
                <template v-if="v.review_state === 'proposed'">
                  <Button size="sm" variant="outline" :disabled="pg.busy" @click="pg.review('values', v.id, 'approved')">Подтвердить</Button>
                  <Button size="sm" variant="ghost" class="text-destructive" :disabled="pg.busy" @click="pg.review('values', v.id, 'rejected')">Отклонить</Button>
                </template>
              </div>
            </div>
          </TabsContent>

          <!-- Правки -->
          <TabsContent value="review" class="space-y-2">
            <p v-if="!pendingRows.length" class="text-sm text-muted-foreground py-6 text-center">Нет строк на проверке — всё подтверждено.</p>
            <div v-for="r in pendingRows" :key="r.id" class="rounded-lg border border-border bg-card px-4 py-3 flex items-center justify-between gap-3">
              <span class="text-sm truncate">{{ r.label }}</span>
              <div class="flex items-center gap-2 shrink-0">
                <Button size="sm" variant="outline" :disabled="pg.busy" @click="pg.review(r.kind, r.id, 'approved')">Подтвердить</Button>
                <Button size="sm" variant="ghost" class="text-destructive" :disabled="pg.busy" @click="pg.review(r.kind, r.id, 'rejected')">Отклонить</Button>
              </div>
            </div>
          </TabsContent>
        </Tabs>

        <p v-if="pg.gateReasons" class="flex items-start gap-2 text-sm text-destructive rounded-lg bg-destructive/10 p-3">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ pg.gateReasons }}
        </p>
        <p v-else-if="pg.error" class="flex items-center gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0" /> {{ pg.error }}
        </p>
      </div>
    </div>

    <!-- right rail -->
    <aside class="w-72 shrink-0 border-l border-border bg-card overflow-y-auto p-5 space-y-5 hidden xl:block">
      <div>
        <h3 class="text-sm font-semibold mb-2">Быстрый доступ</h3>
        <div class="flex flex-col gap-1">
          <button v-for="t in tabs" :key="t.key" class="text-left text-sm rounded-md px-2 py-1.5 hover:bg-muted transition" :class="tab === t.key ? 'bg-muted font-medium' : 'text-muted-foreground'" @click="tab = t.key">{{ t.label }}</button>
        </div>
      </div>
      <div>
        <h3 class="text-sm font-semibold mb-2">Последние изменения</h3>
        <ul class="space-y-2">
          <li v-for="(r, i) in recent" :key="i" class="text-xs">
            <div class="truncate">{{ r.label }}</div>
            <div class="text-muted-foreground">{{ shortTime(r.at) }}</div>
          </li>
          <li v-if="!recent.length" class="text-xs text-muted-foreground">—</li>
        </ul>
      </div>
      <div v-if="pg.hasDraft">
        <h3 class="text-sm font-semibold mb-2">Готовность к публикации</h3>
        <div class="h-2 rounded-full bg-muted overflow-hidden">
          <div class="h-full bg-primary transition-all" :style="{ width: Math.round(pg.readiness * 100) + '%' }" />
        </div>
        <p class="text-xs text-muted-foreground mt-1.5">{{ pg.pending ? pg.pending + ' на проверке' : 'Готово к публикации' }}</p>
      </div>
    </aside>
  </div>
</template>
