<script setup lang="ts">
import { computed, ref } from 'vue'
import { notCheckedExtractRequirements } from '@/lib/evalCatalog'
import { evalsApi } from '@/api/evals'
import type { CatalogExtractCase } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

const props = defineProps<{ extractCase: CatalogExtractCase }>()

const imageURL = computed(() => evalsApi.catalogImageURL(props.extractCase.image))
const notChecked = computed(() => notCheckedExtractRequirements(props.extractCase))
const fieldEntries = computed(() => Object.entries(props.extractCase.fields ?? {}))
const zoomed = ref(false)

const universalChecks = [
  { label: 'Валидный JSON', expected: 'корректный JSON-объект' },
  { label: 'Без утечки рассуждений', expected: 'нет <think>/<thinking> меток в тексте' },
]
</script>

<template>
  <div class="max-w-3xl space-y-5">
    <div class="grid gap-4 sm:grid-cols-[240px_1fr]">
      <button type="button" class="block w-fit" @click="zoomed = true">
        <img :src="imageURL" class="max-w-[240px] max-h-[240px] rounded-lg border border-border object-contain cursor-zoom-in hover:opacity-90 transition" alt="исходный файл" />
      </button>
      <div class="space-y-3">
        <div>
          <p class="text-xs text-muted-foreground font-mono mb-1">{{ extractCase.id }}</p>
          <p class="text-[11px] text-muted-foreground">источник: <code>{{ extractCase.source }}</code></p>
        </div>

        <div v-if="fieldEntries.length">
          <div class="text-xs font-medium mb-1">Классификация</div>
          <div class="flex flex-wrap gap-1.5">
            <Badge v-for="[k, v] in fieldEntries" :key="k" variant="outline" class="text-[11px]">{{ k }}: {{ v }}</Badge>
          </div>
        </div>
      </div>
    </div>

    <div class="space-y-3">
      <div v-if="extractCase.text_contains_all?.length">
        <div class="text-xs font-medium mb-1">Обязательные фразы</div>
        <div class="flex flex-wrap gap-1.5">
          <code v-for="p in extractCase.text_contains_all" :key="p" class="text-[11px] rounded bg-muted px-1.5 py-0.5">{{ p }}</code>
        </div>
      </div>
      <div v-if="extractCase.identify_contains_all?.length">
        <div class="text-xs font-medium mb-1">Тема/описание (обязательно всё)</div>
        <div class="flex flex-wrap gap-1.5">
          <code v-for="p in extractCase.identify_contains_all" :key="p" class="text-[11px] rounded bg-muted px-1.5 py-0.5">{{ p }}</code>
        </div>
      </div>
      <div v-if="extractCase.identify_contains_any?.length">
        <div class="text-xs font-medium mb-1">Тема/описание (любое из)</div>
        <div class="flex flex-wrap gap-1.5">
          <code v-for="p in extractCase.identify_contains_any" :key="p" class="text-[11px] rounded bg-muted px-1.5 py-0.5">{{ p }}</code>
        </div>
      </div>
      <div>
        <div class="text-xs font-medium mb-1">Разрешённые числа</div>
        <p v-if="!extractCase.allowed_numbers?.length" class="text-xs text-muted-foreground">числа не ожидаются — любое найденное число будет отмечено как выдуманное</p>
        <div v-else class="flex flex-wrap gap-1.5">
          <code v-for="n in extractCase.allowed_numbers" :key="n" class="text-[11px] rounded bg-muted px-1.5 py-0.5">{{ n }}</code>
        </div>
      </div>
      <div v-if="extractCase.required_numbers?.length">
        <div class="text-xs font-medium mb-1">Обязательные числа</div>
        <div class="flex flex-wrap gap-1.5">
          <code v-for="n in extractCase.required_numbers" :key="n" class="text-[11px] rounded bg-muted px-1.5 py-0.5">{{ n }}</code>
        </div>
      </div>
      <div v-if="extractCase.forbid_currency" class="text-xs">
        <span class="font-medium">Запрет валюты:</span> упоминаний валюты быть не должно
      </div>
    </div>

    <div v-if="notChecked.length">
      <div class="text-[10px] font-medium uppercase tracking-wide text-muted-foreground mb-1.5">Не проверяется в этом случае</div>
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

    <Dialog :open="zoomed" @update:open="(v) => { zoomed = v }">
      <DialogContent class="max-w-3xl">
        <DialogHeader><DialogTitle>Исходный файл</DialogTitle></DialogHeader>
        <img :src="imageURL" class="mx-5 mb-5 max-h-[75vh] w-auto rounded-lg border border-border object-contain" alt="исходный файл" />
      </DialogContent>
    </Dialog>
  </div>
</template>
