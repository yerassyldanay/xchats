<script setup lang="ts">
// Showcase 3: the anti-hallucination mechanism, THE core positioning claim.
// The model is contractually forbidden from writing a number — the frame
// rule is literal ("ЦИФРЫ И КОНТАКТНЫЕ ДАННЫЕ САМ НЕ ПИШИ НИКОГДА") — so it
// emits a catalog placeholder like {{product.coffee-machine.price}}, and
// backend/aiprompt.SubstituteFactsLang injects the exact stored value
// afterward, failing the whole draft (never guessing) on an unknown token.
// This demo types out that exact raw-output shape, then "resolves" it —
// the same two-stage reality the real pipeline goes through, compressed
// into a few seconds of animation. Token/values below are real: taken from
// evals/runs/2026-07-27_15-44-33-1859's shop-kb-v1-10 scenario.
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRevealOnScroll } from '@/composables/useRevealOnScroll'
import { useTypewriter } from '@/composables/useTypewriter'
import SectionShell from './SectionShell.vue'

const { t } = useI18n()

const PRICE_TOKEN = '{{product.coffee-machine.price}}'
const DAYS_TOKEN = '{{delivery.astana.delivery_in_days}}'
const rawSentence = t('landing.templates.sentencePrefix') + PRICE_TOKEN + t('landing.templates.sentenceMiddle') + DAYS_TOKEN + t('landing.templates.sentenceSuffix')

// A separate, local reveal (not SectionShell's) — that one only fades the
// wrapper in; this one gates WHEN typing starts, which must wait until the
// demo is actually in view or an animation-delay-style effect would tick
// away invisibly and finish before the visitor ever scrolls to it.
const demoRef = ref<HTMLElement | null>(null)
const { visible: demoVisible } = useRevealOnScroll(demoRef, 0.4)

const startTyping = ref(false)
// immediate: true matters here, not just style — prefers-reduced-motion
// makes useRevealOnScroll's `visible` start at true (never false→true), so
// a plain watch() would never fire and the demo would never start typing.
watch(
  demoVisible,
  (v) => {
    if (v) startTyping.value = true
  },
  { immediate: true }
)

const { text: typedText, done: typedDone } = useTypewriter(rawSentence, startTyping)

const resolved = ref(false)
// immediate: true for the same reason as the watch above — under
// prefers-reduced-motion, useTypewriter's reduceMotion branch sets
// typedDone straight to true synchronously during setup, before this
// watch even registers, so a plain watch() would never see it change.
watch(
  typedDone,
  (v) => {
    if (v) window.setTimeout(() => (resolved.value = true), 700)
  },
  { immediate: true }
)
</script>

<template>
  <SectionShell
    id="templates"
    :eyebrow="t('landing.templates.eyebrow')"
    :title="t('landing.templates.title')"
    :description="t('landing.templates.description')"
  >
    <div ref="demoRef" class="landing-chat-demo">
      <p class="landing-bubble landing-bubble--customer">{{ t('landing.templates.question') }}</p>

      <p v-if="!resolved" class="landing-bubble landing-bubble--assistant">
        {{ typedText }}<span v-if="!typedDone" class="landing-cursor" aria-hidden="true" />
      </p>
      <p v-else class="landing-bubble landing-bubble--assistant">
        {{ t('landing.templates.sentencePrefix') }}<span class="landing-token is-resolved">{{ t('landing.templates.resolvedPrice') }}</span>{{
          t('landing.templates.sentenceMiddle')
        }}<span class="landing-token is-resolved">{{ t('landing.templates.resolvedDays') }}</span>{{ t('landing.templates.sentenceSuffix') }}
      </p>
    </div>

    <p class="landing-note">{{ t('landing.templates.note') }}</p>
  </SectionShell>
</template>
