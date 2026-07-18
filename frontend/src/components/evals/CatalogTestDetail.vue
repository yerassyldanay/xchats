<script setup lang="ts">
import { computed } from 'vue'
import { notCheckedRequirements, resolveMediaExpectation, resolveRequires } from '@/lib/evalCatalog'
import type { CatalogScenario, CatalogTestCase } from '@/types'
import { Badge } from '@/components/ui/badge'

// The Requirements centerpiece — every expectation as a resolved, sourced sentence.
// Read-only review, never a run/results view (no pass/fail here — this is BEFORE any
// model is ever called).
const props = defineProps<{ test: CatalogTestCase; scenario: CatalogScenario }>()

const requireGroups = computed(() => resolveRequires(props.test.requires, props.scenario.facts))
const mediaExpectation = computed(() => resolveMediaExpectation(props.test.media, props.scenario.media_refs, props.scenario.media_groups))
const notChecked = computed(() => notCheckedRequirements(props.test))

const universalChecks = [
  { label: 'Валидный JSON', expected: 'корректный JSON-объект' },
  { label: 'Поля контракта', expected: 'все обязательные поля присутствуют' },
  { label: 'Без неразрешённых плейсхолдеров', expected: 'нет неизвестных токенов, ответ не заблокирован' },
  { label: 'Валидные ссылки на медиа', expected: 'все ссылки существуют в каталоге' },
  { label: 'Без выдуманных чисел', expected: 'нет чисел вне ответа модели' },
  { label: 'Не больше вложений, чем разрешает промпт', expected: 'не более 3 ссылок / 2 групп (правило 3 фрейма)' },
]
</script>

<template>
  <div class="max-w-3xl space-y-5">
    <div>
      <div class="text-[10px] font-medium uppercase tracking-wide text-muted-foreground mb-1.5">Вход</div>
      <p class="text-xs text-muted-foreground font-mono mb-1">{{ test.id }}</p>
      <p class="text-sm whitespace-pre-wrap">{{ test.message }}</p>
      <div v-if="test.history?.length" class="mt-2 space-y-1 rounded-lg bg-muted/40 p-2.5">
        <div v-for="(h, i) in test.history" :key="i" class="text-xs text-muted-foreground">
          <span class="font-medium">{{ h.role === 'client' ? 'Клиент' : 'Ассистент' }}:</span> {{ h.text }}
        </div>
      </div>
      <p class="text-[11px] text-muted-foreground mt-1.5">источник: <code>{{ test.source }}</code></p>
    </div>

    <div>
      <div class="text-[10px] font-medium uppercase tracking-wide text-muted-foreground mb-2">Требования</div>
      <div class="space-y-3">
        <div v-if="requireGroups.length">
          <div class="text-xs font-medium mb-1">Обязательные факты</div>
          <ol class="space-y-1.5">
            <li v-for="(g, i) in requireGroups" :key="i" class="text-xs">
              <span class="text-muted-foreground mr-1">{{ i + 1 }}.</span>
              <span v-if="g.alternatives.length > 1" class="text-muted-foreground">Одно из:</span>
              <ul :class="g.alternatives.length > 1 ? 'ml-4 mt-0.5 space-y-0.5' : 'inline'">
                <li v-for="alt in g.alternatives" :key="alt.token" :class="g.alternatives.length > 1 ? '' : 'inline'">
                  <code class="font-mono">{{ alt.token }}</code>
                  <template v-if="alt.value !== null">
                    → <span class="font-medium">«{{ alt.value }}»</span>
                  </template>
                  <span v-else class="text-destructive font-medium">→ значение не найдено</span>
                </li>
              </ul>
            </li>
          </ol>
        </div>

        <div v-if="test.language" class="text-xs">
          <span class="font-medium">Язык ответа:</span> {{ test.language }}
        </div>

        <div v-if="test.escalate !== undefined" class="text-xs">
          <span class="font-medium">Эскалация:</span>
          {{ test.escalate ? 'обязательна' : 'эскалации быть не должно' }}
        </div>

        <div v-if="mediaExpectation">
          <div class="text-xs font-medium mb-1">
            Медиа
            <span v-if="mediaExpectation.exclusive" class="text-muted-foreground font-normal">(только из этого списка)</span>
          </div>
          <p v-if="mediaExpectation.forbid" class="text-xs">медиа быть не должно</p>
          <ul v-else class="space-y-0.5">
            <li v-for="e in [...mediaExpectation.refs, ...mediaExpectation.groups]" :key="e.name" class="text-xs">
              <code class="font-mono">{{ e.name }}</code>
              <span v-if="!e.found" class="text-destructive font-medium"> → не найдено в каталоге</span>
            </li>
          </ul>
        </div>

        <div v-if="test.must_not_contain?.length" class="text-xs">
          <div class="font-medium mb-1">Запрещённые фразы</div>
          <ul class="space-y-0.5">
            <li v-for="p in test.must_not_contain" :key="p"><code class="font-mono">{{ p }}</code></li>
          </ul>
        </div>
      </div>
    </div>

    <div v-if="notChecked.length">
      <div class="text-[10px] font-medium uppercase tracking-wide text-muted-foreground mb-1.5">Не проверяется этим тестом</div>
      <div class="flex flex-wrap gap-1.5">
        <Badge v-for="item in notChecked" :key="item" variant="outline" class="text-[11px] text-muted-foreground">{{ item }}</Badge>
      </div>
    </div>

    <div>
      <div class="text-[10px] font-medium uppercase tracking-wide text-muted-foreground mb-1.5">Всегда применяемые проверки</div>
      <div class="grid grid-cols-2 gap-1.5">
        <div v-for="c in universalChecks" :key="c.label" class="text-[11px] rounded-lg border border-border px-2 py-1.5">
          <div class="font-medium">{{ c.label }}</div>
          <div class="text-muted-foreground">{{ c.expected }}</div>
        </div>
      </div>
    </div>
  </div>
</template>
