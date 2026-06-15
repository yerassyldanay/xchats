<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter, useRoute, RouterLink } from 'vue-router'
import { useAuth } from '../stores/auth'
import { initials, colorFor } from '../lib/format'

// Persistent left navigation rail — always present on authed pages (inbox,
// accounts, instances). Rendered once by App.vue so it never disappears.
const auth = useAuth()
const router = useRouter()
const route = useRoute()
const menuOpen = ref(false)

const onAccounts = computed(() => route.name === 'accounts' || route.name === 'instances')

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <nav class="w-[68px] bg-rail flex flex-col items-center py-4 shrink-0">
    <div
      class="w-10 h-10 rounded-xl bg-gradient-to-br from-indigo-500 to-violet-500 text-white grid place-items-center font-extrabold text-lg shadow-rail"
    >
      X
    </div>

    <div class="mt-6 flex flex-col gap-2">
      <RouterLink
        :to="{ name: 'chatboard' }"
        class="w-11 h-11 rounded-xl grid place-items-center text-[19px] transition"
        :class="!onAccounts ? 'bg-brand text-white shadow-rail' : 'text-white/55 hover:text-white hover:bg-white/10'"
        title="Инбокс"
      >
        <i class="fa-solid fa-comment-dots"></i>
      </RouterLink>
      <RouterLink
        :to="{ name: 'accounts' }"
        class="w-11 h-11 rounded-xl grid place-items-center text-[19px] transition"
        :class="onAccounts ? 'bg-brand text-white shadow-rail' : 'text-white/55 hover:text-white hover:bg-white/10'"
        title="Номера WhatsApp"
      >
        <i class="fa-brands fa-whatsapp"></i>
      </RouterLink>
    </div>

    <div class="mt-auto relative">
      <button
        class="w-10 h-10 rounded-full grid place-items-center text-white text-xs font-semibold ring-2 ring-white/15 hover:ring-white/30 transition"
        :style="{ backgroundColor: colorFor(auth.user?.id || 'x') }"
        @click="menuOpen = !menuOpen"
      >
        {{ initials(auth.user?.name || auth.user?.email || '?') }}
      </button>
      <div
        v-if="menuOpen"
        class="absolute bottom-0 left-14 w-60 rounded-2xl border border-hair bg-white shadow-pop p-3 z-30"
      >
        <div class="flex items-center gap-3 px-1">
          <div
            class="w-10 h-10 rounded-full grid place-items-center text-white text-sm font-semibold shrink-0"
            :style="{ backgroundColor: colorFor(auth.user?.id || 'x') }"
          >
            {{ initials(auth.user?.name || auth.user?.email || '?') }}
          </div>
          <div class="min-w-0">
            <div class="text-sm font-semibold truncate">{{ auth.user?.name || '—' }}</div>
            <div class="text-xs text-muted truncate">{{ auth.user?.email }}</div>
          </div>
        </div>
        <div class="text-xs text-slate-400 mt-2 px-1">{{ auth.org?.name }}</div>
        <button
          class="mt-3 w-full flex items-center gap-2 text-left text-sm text-red-600 hover:bg-red-50 rounded-lg px-2 py-2 font-medium transition"
          @click="logout"
        >
          <i class="fa-solid fa-arrow-right-from-bracket"></i> Выйти
        </button>
      </div>
    </div>
  </nav>
</template>
