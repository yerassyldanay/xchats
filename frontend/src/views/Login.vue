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
    <div class="hidden md:flex w-1/2 bg-navy text-white flex-col justify-center px-14">
      <div class="text-3xl font-bold tracking-tight">
        <span class="text-brand">X</span>Chats
      </div>
      <h1 class="mt-4 text-2xl font-semibold leading-snug">Командный инбокс<br />и ИИ-ассистент</h1>
      <ul class="mt-8 space-y-3 text-slate-300">
        <li class="flex items-center gap-3"><span class="text-wa">●</span> Единый инбокс WhatsApp</li>
        <li class="flex items-center gap-3"><span class="text-wa">●</span> ИИ-ответы (предложить и отправить)</li>
        <li class="flex items-center gap-3"><span class="text-wa">●</span> Безопасность и контроль</li>
      </ul>
    </div>

    <!-- form -->
    <div class="flex w-full md:w-1/2 items-center justify-center bg-white px-6">
      <form class="w-full max-w-sm" @submit.prevent="submit">
        <h2 class="text-xl font-semibold mb-6">Вход в аккаунт</h2>

        <label class="block text-sm font-medium mb-1">Email</label>
        <input
          v-model="email"
          type="email"
          autocomplete="username"
          required
          class="w-full mb-4 rounded-lg border border-hair px-3 py-2 outline-none focus:border-brand"
        />

        <label class="block text-sm font-medium mb-1">Пароль</label>
        <input
          v-model="password"
          type="password"
          autocomplete="current-password"
          required
          class="w-full mb-5 rounded-lg border border-hair px-3 py-2 outline-none focus:border-brand"
        />

        <p v-if="error" class="text-sm text-red-600 mb-3">{{ error }}</p>

        <button
          type="submit"
          :disabled="busy"
          class="w-full rounded-lg bg-brand py-2.5 font-medium text-white hover:opacity-90 disabled:opacity-60"
        >
          {{ busy ? 'Вход…' : 'Войти' }}
        </button>

        <p class="mt-6 text-center text-xs text-slate-500">
          Нет аккаунта? Свяжитесь с администратором
        </p>
      </form>
    </div>
  </div>
</template>
