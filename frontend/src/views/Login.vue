<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { CircleAlert, LoaderCircle, Lock, Mail, ShieldCheck, WandSparkles } from 'lucide-vue-next'
import { useAuth } from '../stores/auth'
import { ApiError } from '../api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'

const auth = useAuth()
const router = useRouter()
const email = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await auth.login(email.value.trim(), password.value)
    router.push({ name: 'chatboard' })
  } catch (e) {
    error.value =
      e instanceof ApiError && e.errcode === 'UNAUTHORIZED'
        ? 'Неверный email или пароль'
        : 'Не удалось войти. Попробуйте ещё раз.'
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
          XChats
        </div>
        <h1 class="mt-8 text-3xl font-semibold leading-snug tracking-tight">Командный инбокс<br />и ИИ-ассистент</h1>
        <ul class="mt-10 space-y-4 text-slate-300">
          <li class="flex items-center gap-3">
            <span class="w-7 h-7 rounded-md bg-white/5 grid place-items-center text-wa"><WhatsappIcon class="w-4 h-4" /></span>
            Единый инбокс WhatsApp
          </li>
          <li class="flex items-center gap-3">
            <span class="w-7 h-7 rounded-md bg-white/5 grid place-items-center text-indigo-300"><WandSparkles class="w-4 h-4" /></span>
            ИИ-ответы (предложить и отправить)
          </li>
          <li class="flex items-center gap-3">
            <span class="w-7 h-7 rounded-md bg-white/5 grid place-items-center text-emerald-300"><ShieldCheck class="w-4 h-4" /></span>
            Безопасность и контроль
          </li>
        </ul>
      </div>
    </div>

    <!-- form -->
    <div class="flex w-full md:w-1/2 items-center justify-center bg-card px-6">
      <form class="w-full max-w-sm" @submit.prevent="submit">
        <div class="md:hidden flex items-center gap-2 text-2xl font-bold mb-6">
          <span class="w-9 h-9 rounded-md bg-primary text-primary-foreground grid place-items-center">X</span>
          XChats
        </div>
        <h2 class="text-2xl font-semibold tracking-tight mb-1">Вход в аккаунт</h2>
        <p class="text-sm text-muted-foreground mb-7">Войдите, чтобы открыть инбокс</p>

        <label class="block text-sm font-medium mb-1.5">Email</label>
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

        <label class="block text-sm font-medium mb-1.5">Пароль</label>
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
          {{ busy ? 'Вход…' : 'Войти' }}
        </Button>

        <p class="mt-6 text-center text-xs text-muted-foreground">Нет аккаунта? Свяжитесь с администратором</p>
      </form>
    </div>
  </div>
</template>
