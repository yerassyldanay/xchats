<script setup lang="ts">
// Two-column shell shared by Login and the forced ChangePassword screen: a
// grey brand panel (logo, headline, a short description of the product, its
// six capabilities, language switcher) beside the form column (default slot). Renders outside the nav sidebar, so the
// switcher here is the only way to pick a language before signing in — native
// language names, same three locales as the sidebar's account menu.
import { useI18n } from 'vue-i18n'
import { BookOpenCheck, CalendarCheck, GitPullRequestDraft, Inbox, Megaphone, ShieldCheck } from 'lucide-vue-next'

const { t, locale } = useI18n()

const LOCALES = [
  { code: 'ru', label: 'Русский' },
  { code: 'en', label: 'English' },
  { code: 'kk', label: 'Қазақша' },
] as const

// Everything the product does, in the order a team meets it.
const features = [
  { icon: Inbox, key: 'inbox' },
  { icon: BookOpenCheck, key: 'kb' },
  { icon: CalendarCheck, key: 'crm' },
  { icon: Megaphone, key: 'campaigns' },
  { icon: GitPullRequestDraft, key: 'review' },
  { icon: ShieldCheck, key: 'privacy' },
] as const
</script>

<template>
  <div class="grid min-h-full md:grid-cols-2">
    <div class="hidden flex-col justify-between gap-10 bg-secondary px-14 py-12 md:flex">
      <div class="flex items-center gap-2.5">
        <img src="/logo.png" alt="xchats logo" class="size-10 rounded-[10px] bg-slate-900 object-contain" />
        <span class="text-[22px] font-extrabold tracking-tight">xchats</span>
      </div>

      <div class="max-w-[520px]">
        <h1 class="mb-4 text-[38px] font-extrabold leading-[1.1] tracking-[-0.8px]">
          {{ t('login.heroLine1') }}<br />{{ t('login.heroLine2') }}
        </h1>
        <p class="mb-7 text-[15px] leading-relaxed text-muted-foreground">{{ t('login.heroLead') }}</p>
        <ul class="grid gap-x-6 gap-y-4 lg:grid-cols-2">
          <li v-for="f in features" :key="f.key" class="flex items-start gap-3">
            <span class="grid size-10 flex-none place-items-center rounded-xl bg-background shadow-[0_1px_2px_rgba(0,0,0,0.08)]">
              <component :is="f.icon" class="size-5" />
            </span>
            <span class="min-w-0">
              <span class="block text-[15px] font-bold leading-snug">{{ t(`login.features.${f.key}.title`) }}</span>
              <span class="mt-0.5 block text-[13px] leading-snug text-muted-foreground">{{ t(`login.features.${f.key}.desc`) }}</span>
            </span>
          </li>
        </ul>
      </div>

      <nav :aria-label="t('nav.language')" class="flex items-center gap-2 text-[13px]">
        <template v-for="(l, i) in LOCALES" :key="l.code">
          <span v-if="i > 0" class="text-muted-foreground" aria-hidden="true">·</span>
          <button
            type="button"
            class="transition-colors hover:text-foreground"
            :class="locale === l.code ? 'font-bold text-foreground' : 'text-muted-foreground'"
            :aria-current="locale === l.code ? 'true' : undefined"
            @click="locale = l.code"
          >
            {{ l.label }}
          </button>
        </template>
      </nav>
    </div>

    <div class="grid place-items-center px-6 py-12">
      <div class="w-full max-w-[420px]">
        <div class="mb-6 flex items-center gap-2 text-2xl font-extrabold md:hidden">
          <img src="/logo.png" alt="xchats logo" class="size-9 rounded-[10px] bg-slate-900 object-contain" />
          xchats
        </div>
        <slot />
        <nav :aria-label="t('nav.language')" class="mt-8 flex items-center justify-center gap-2 text-xs md:hidden">
          <template v-for="(l, i) in LOCALES" :key="l.code">
            <span v-if="i > 0" class="text-muted-foreground" aria-hidden="true">·</span>
            <button
              type="button"
              :class="locale === l.code ? 'font-bold text-foreground' : 'text-muted-foreground'"
              :aria-current="locale === l.code ? 'true' : undefined"
              @click="locale = l.code"
            >
              {{ l.label }}
            </button>
          </template>
        </nav>
      </div>
    </div>
  </div>
</template>
