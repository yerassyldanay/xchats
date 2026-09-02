<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, Copy, Github, Server, Sparkles } from 'lucide-vue-next'

const { t } = useI18n()

const QUICKSTART = 'make up && make seed-demo'
const copied = ref(false)
let copiedTimer: ReturnType<typeof setTimeout> | undefined
async function copyQuickstart() {
  try {
    await navigator.clipboard.writeText(QUICKSTART)
    copied.value = true
    clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => (copied.value = false), 1800)
  } catch {
    // Clipboard API unavailable (insecure context, permission denied) — the
    // command is still selectable/readable in the box, so this is a
    // best-effort convenience, not the only way to get the text.
  }
}
</script>

<template>
  <section class="landing-hero blog-shell">
    <div>
      <span class="badge badge--category">{{ t('landing.hero.eyebrow') }}</span>
      <h1 class="landing-hero__title">
        {{ t('landing.hero.titlePrefix') }} <em>{{ t('landing.hero.titleHighlight') }}</em> {{ t('landing.hero.titleSuffix') }}
      </h1>
      <p class="landing-hero__subtitle">{{ t('landing.hero.subtitle') }}</p>

      <button type="button" class="landing-quickstart" @click="copyQuickstart">
        <code class="landing-quickstart__cmd">$ {{ QUICKSTART }}</code>
        <span class="landing-quickstart__copy">
          <Check v-if="copied" class="w-4 h-4" aria-hidden="true" />
          <Copy v-else class="w-4 h-4" aria-hidden="true" />
          {{ copied ? t('common.copied') : t('common.copy') }}
        </span>
      </button>
      <p class="landing-quickstart__hint">{{ t('landing.hero.quickstartHint') }}</p>

      <div class="landing-hero__ctas">
        <RouterLink :to="{ name: 'login' }" class="landing-hero__cta">{{ t('landing.hero.ctaPrimary') }}</RouterLink>
        <a href="https://github.com/yerassyldanay/xchats" target="_blank" rel="noreferrer" class="landing-hero__cta landing-hero__cta--ghost">
          <Github class="w-4 h-4" aria-hidden="true" /> {{ t('landing.hero.ctaSecondary') }}
        </a>
      </div>

      <div class="landing-hero__trust">
        <span class="landing-hero__trust-item"><Github aria-hidden="true" /> {{ t('landing.hero.trust1') }}</span>
        <span class="landing-hero__trust-item"><Server aria-hidden="true" /> {{ t('landing.hero.trust2') }}</span>
        <span class="landing-hero__trust-item"><Sparkles aria-hidden="true" /> {{ t('landing.hero.trust3') }}</span>
      </div>
    </div>

    <div class="landing-hero__panel">
      <img src="/screenshots/inbox.png" :alt="t('landing.hero.screenshotAlt')" />
    </div>
  </section>
</template>
