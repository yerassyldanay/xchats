<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { CircleAlert, LoaderCircle, Lock, Mail, ShieldCheck, WandSparkles } from 'lucide-vue-next'
import { useAuth } from '../stores/auth'
import { ApiError } from '../api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'

const auth = useAuth()
const router = useRouter()
const { t, locale } = useI18n()
const email = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

// Login renders outside the nav rail, so its own switcher is the only way to
// pick a language before signing in. Native language names, same three
// locales and same convention as NavRail/LandingLangSwitcher.
const LOCALES = [
  { code: 'ru', label: 'Русский' },
  { code: 'en', label: 'English' },
  { code: 'kk', label: 'Қазақша' },
] as const

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await auth.login(email.value.trim(), password.value)
    router.push({ name: 'chatboard' })
  } catch (e) {
    error.value =
      e instanceof ApiError && e.errcode === 'UNAUTHORIZED'
        ? t('login.errBadCredentials')
        : t('login.errGeneric')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex min-h-full">
    <!-- brand panel (flat dark; no gradients/orbs) -->
    <div class="hidden md:flex w-1/2 bg-slate-900 text-slate-100 flex-col justify-center px-16">
      <div>
        <div class="flex items-center gap-3 text-2xl font-bold tracking-tight">
          <span class="w-11 h-11 rounded-lg bg-primary text-primary-foreground grid place-items-center font-bold">X</span>
          xchats
        </div>
        <h1 class="mt-8 text-3xl font-semibold leading-snug tracking-tight">{{ t('login.heroLine1') }}<br />{{ t('login.heroLine2') }}</h1>
        <ul class="mt-10 space-y-4 text-slate-300">
          <li class="flex items-center gap-3">
            <span class="w-7 h-7 rounded-md bg-white/5 grid place-items-center text-wa"><WhatsappIcon class="w-4 h-4" /></span>
            {{ t('login.benefitInbox') }}
          </li>
          <li class="flex items-center gap-3">
            <span class="w-7 h-7 rounded-md bg-white/5 grid place-items-center text-indigo-300"><WandSparkles class="w-4 h-4" /></span>
            {{ t('login.benefitAi') }}
          </li>
          <li class="flex items-center gap-3">
            <span class="w-7 h-7 rounded-md bg-white/5 grid place-items-center text-emerald-300"><ShieldCheck class="w-4 h-4" /></span>
            {{ t('login.benefitSecurity') }}
          </li>
        </ul>
      </div>
    </div>

    <!-- form -->
    <div class="flex w-full md:w-1/2 items-center justify-center bg-card px-6">
      <form class="w-full max-w-sm" @submit.prevent="submit">
        <div class="md:hidden flex items-center gap-2 text-2xl font-bold mb-6">
          <span class="w-9 h-9 rounded-md bg-primary text-primary-foreground grid place-items-center">X</span>
          xchats
        </div>
        <h2 class="text-2xl font-semibold tracking-tight mb-1">{{ t('login.title') }}</h2>
        <p class="text-sm text-muted-foreground mb-7">{{ t('login.subtitle') }}</p>

        <label class="block text-sm font-medium mb-1.5">{{ t('login.email') }}</label>
        <div class="relative mb-4">
          <Mail class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
          <Input
            v-model="email"
            type="email"
            autocomplete="username"
            required
            placeholder="you@example.com"
            class="pl-9"
          />
        </div>

        <label class="block text-sm font-medium mb-1.5">{{ t('login.password') }}</label>
        <div class="relative mb-5">
          <Lock class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
          <Input
            v-model="password"
            type="password"
            autocomplete="current-password"
            required
            placeholder="••••••••"
            class="pl-9"
          />
        </div>

        <p v-if="error" class="flex items-center gap-2 text-sm text-destructive mb-3">
          <CircleAlert class="w-4 h-4 shrink-0" /> {{ error }}
        </p>

        <Button type="submit" :disabled="busy" class="w-full h-11">
          <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
          {{ busy ? t('login.busy') : t('login.submit') }}
        </Button>

        <p class="mt-6 text-center text-xs text-muted-foreground">{{ t('login.noAccount') }}</p>

        <nav :aria-label="t('nav.language')" class="mt-5 flex items-center justify-center gap-2 text-xs">
          <template v-for="(l, i) in LOCALES" :key="l.code">
            <span v-if="i > 0" class="text-border" aria-hidden="true">·</span>
            <button
              type="button"
              class="transition hover:text-foreground"
              :class="locale === l.code ? 'font-medium text-foreground' : 'text-muted-foreground'"
              :aria-current="locale === l.code ? 'true' : undefined"
              @click="locale = l.code"
            >
              {{ l.label }}
            </button>
          </template>
        </nav>
      </form>
    </div>
  </div>
</template>
