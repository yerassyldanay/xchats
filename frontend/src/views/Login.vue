<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../stores/auth'
import { ApiError } from '../api/client'

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
    <!-- brand panel -->
    <div class="hidden md:flex w-1/2 bg-rail text-white flex-col justify-center px-16 relative overflow-hidden">
      <div class="absolute -top-24 -right-24 w-80 h-80 rounded-full bg-brand/20 blur-3xl"></div>
      <div class="absolute -bottom-24 -left-10 w-72 h-72 rounded-full bg-wa/10 blur-3xl"></div>
      <div class="relative">
        <div class="flex items-center gap-3 text-2xl font-extrabold tracking-tight">
          <span class="w-11 h-11 rounded-2xl bg-gradient-to-br from-indigo-500 to-violet-500 grid place-items-center shadow-rail">X</span>
          XChats
        </div>
        <h1 class="mt-8 text-3xl font-bold leading-snug">Командный инбокс<br />и ИИ-ассистент</h1>
        <ul class="mt-10 space-y-4 text-slate-300">
          <li class="flex items-center gap-3">
            <span class="w-7 h-7 rounded-lg bg-white/10 grid place-items-center text-wa"><i class="fa-brands fa-whatsapp"></i></span>
            Единый инбокс WhatsApp
          </li>
          <li class="flex items-center gap-3">
            <span class="w-7 h-7 rounded-lg bg-white/10 grid place-items-center text-brand"><i class="fa-solid fa-wand-magic-sparkles"></i></span>
            ИИ-ответы (предложить и отправить)
          </li>
          <li class="flex items-center gap-3">
            <span class="w-7 h-7 rounded-lg bg-white/10 grid place-items-center text-emerald-300"><i class="fa-solid fa-shield-halved"></i></span>
            Безопасность и контроль
          </li>
        </ul>
      </div>
    </div>

    <!-- form -->
    <div class="flex w-full md:w-1/2 items-center justify-center bg-white px-6">
      <form class="w-full max-w-sm" @submit.prevent="submit">
        <div class="md:hidden flex items-center gap-2 text-2xl font-extrabold mb-6">
          <span class="w-9 h-9 rounded-xl bg-gradient-to-br from-indigo-500 to-violet-500 text-white grid place-items-center">X</span>
          XChats
        </div>
        <h2 class="text-2xl font-bold mb-1">Вход в аккаунт</h2>
        <p class="text-sm text-muted mb-7">Войдите, чтобы открыть инбокс</p>

        <label class="block text-sm font-medium mb-1.5">Email</label>
        <div class="relative mb-4">
          <i class="fa-regular fa-envelope absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 text-sm"></i>
          <input
            v-model="email"
            type="email"
            autocomplete="username"
            required
            placeholder="you@example.com"
            class="field pl-9"
          />
        </div>

        <label class="block text-sm font-medium mb-1.5">Пароль</label>
        <div class="relative mb-5">
          <i class="fa-solid fa-lock absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 text-sm"></i>
          <input
            v-model="password"
            type="password"
            autocomplete="current-password"
            required
            placeholder="••••••••"
            class="field pl-9"
          />
        </div>

        <p v-if="error" class="flex items-center gap-2 text-sm text-red-600 mb-3">
          <i class="fa-solid fa-circle-exclamation"></i> {{ error }}
        </p>

        <button type="submit" :disabled="busy" class="w-full btn-brand h-11">
          <i v-if="busy" class="fa-solid fa-spinner fa-spin"></i>
          {{ busy ? 'Вход…' : 'Войти' }}
        </button>

        <p class="mt-6 text-center text-xs text-slate-500">Нет аккаунта? Свяжитесь с администратором</p>
      </form>
    </div>
  </div>
</template>
