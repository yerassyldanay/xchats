<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { Check } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

const { t } = useI18n()
</script>

<template>
  <div class="pt-6 text-center">
    <div class="mx-auto mb-6 grid size-[72px] place-items-center rounded-full bg-success-soft text-success">
      <Check class="size-[34px]" />
    </div>
    <h1 class="mb-3 text-[40px] font-extrabold tracking-tight">{{ t('setup.done.title') }}</h1>
    <p class="mx-auto mb-9 max-w-[480px] text-lg leading-normal text-muted-foreground">{{ t('setup.done.subtitle') }}</p>
    <div class="flex flex-wrap justify-center gap-3">
      <Button as-child size="lg" class="px-6 text-base"><RouterLink :to="{ name: 'simulator' }">{{ t('setup.done.test') }}</RouterLink></Button>
      <Button as-child size="lg" variant="outline" class="px-6 text-base"><RouterLink :to="{ name: 'dashboard' }">{{ t('setup.done.home') }}</RouterLink></Button>
    </div>
  </div>
</template>
