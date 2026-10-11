<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronDown } from 'lucide-vue-next'
import { useSetupWizardContext } from '@/composables/useSetupWizard'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'
import InstagramIcon from '@/components/icons/InstagramIcon.vue'
import MessengerIcon from '@/components/icons/MessengerIcon.vue'
import ChoiceCard from './ChoiceCard.vue'

const { t } = useI18n()
const w = useSetupWizardContext()
</script>

<template>
  <div>
    <div class="mb-2 text-sm font-bold text-muted-foreground">{{ t('setup.stepOf', { n: 2 }) }}</div>
    <h1 class="mb-2.5 text-[34px] font-extrabold leading-tight tracking-tight">{{ t('setup.channel.title') }}</h1>
    <p class="mb-7 text-[17px] leading-normal text-muted-foreground">{{ t('setup.channel.subtitle') }}</p>

    <div role="radiogroup" :aria-label="t('setup.channel.title')" class="grid grid-cols-2 gap-3.5">
      <ChoiceCard :selected="w.channel === 'whatsapp'" class="flex flex-col gap-4 p-[22px]" :class="w.channel === 'whatsapp' ? 'p-[21px]' : ''" @click="w.pickChannel('whatsapp')">
        <span class="grid size-12 place-items-center rounded-[14px] bg-wa text-white"><WhatsappIcon class="size-[26px]" /></span>
        <span>
          <span class="block text-lg font-bold">{{ t('setup.channel.whatsapp') }}</span>
          <span class="mt-0.5 block text-sm text-muted-foreground">{{ t('setup.channel.whatsappTagline') }}</span>
        </span>
      </ChoiceCard>
      <ChoiceCard :selected="w.channel === 'telegram'" class="flex flex-col gap-4 p-[22px]" :class="w.channel === 'telegram' ? 'p-[21px]' : ''" @click="w.pickChannel('telegram')">
        <span class="grid size-12 place-items-center rounded-[14px] bg-[#229ED9] text-white"><TelegramIcon class="size-[26px]" /></span>
        <span>
          <span class="block text-lg font-bold">{{ t('setup.channel.telegram') }}</span>
          <span class="mt-0.5 block text-sm text-muted-foreground">{{ t('setup.channel.telegramTagline') }}</span>
        </span>
      </ChoiceCard>
    </div>

    <button
      type="button"
      class="mt-6 flex items-center gap-1.5 text-[15px] font-semibold"
      :aria-expanded="w.showMoreChannels"
      @click="w.showMoreChannels = !w.showMoreChannels"
    >
      {{ t('setup.channel.more') }}
      <ChevronDown class="size-4 transition-transform" :class="w.showMoreChannels ? 'rotate-180' : ''" />
    </button>
    <ul v-if="w.showMoreChannels" class="mt-3.5 flex flex-col gap-2.5">
      <li v-for="c in [
        { icon: WhatsappIcon, bg: 'bg-teal-600', name: t('setup.channel.cloud') },
        { icon: InstagramIcon, bg: 'bg-fuchsia-600', name: t('setup.channel.instagram') },
        { icon: MessengerIcon, bg: 'bg-[#0866FF]', name: t('setup.channel.messenger') },
      ]" :key="c.name" class="flex items-center gap-3.5 rounded-[14px] border border-[#EBEBEB] px-[18px] py-4 opacity-55">
        <span class="grid size-9 place-items-center rounded-[10px] text-white" :class="c.bg"><component :is="c.icon" class="size-[18px]" /></span>
        <div class="flex-1">
          <div class="text-base font-bold">{{ c.name }}</div>
          <div class="text-[13px] text-muted-foreground">{{ t('setup.channel.metaTagline') }}</div>
        </div>
        <span class="rounded-full bg-secondary px-2.5 py-1 text-xs font-bold">{{ t('setup.channel.soon') }}</span>
      </li>
    </ul>
  </div>
</template>
