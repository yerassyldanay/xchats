<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Library, Radio, Sparkles } from 'lucide-vue-next'

const { t } = useI18n()
const items = [
  { n: 1, icon: Sparkles, title: 'setup.welcome.s1Title', text: 'setup.welcome.s1Text' },
  { n: 2, icon: Radio, title: 'setup.welcome.s2Title', text: 'setup.welcome.s2Text' },
  { n: 3, icon: Library, title: 'setup.welcome.s3Title', text: 'setup.welcome.s3Text' },
] as const
</script>

<template>
  <div class="pt-6">
    <h1 class="mb-3 text-[40px] font-extrabold leading-[1.15] tracking-tight">{{ t('setup.welcome.title') }}</h1>
    <p class="mb-10 text-lg leading-normal text-muted-foreground">{{ t('setup.welcome.subtitle') }}</p>
    <ul>
      <li v-for="it in items" :key="it.n" class="flex gap-5 border-b border-[#EBEBEB] py-6 last:border-b-0">
        <span class="w-7 text-[22px] font-extrabold">{{ it.n }}</span>
        <div class="flex-1">
          <div class="mb-1 text-xl font-bold">{{ t(it.title) }}</div>
          <div class="text-base leading-[1.45] text-muted-foreground">{{ t(it.text) }}</div>
        </div>
        <span class="grid size-14 flex-none place-items-center rounded-[14px] bg-secondary">
          <component :is="it.icon" class="size-6" />
        </span>
      </li>
    </ul>
  </div>
</template>
