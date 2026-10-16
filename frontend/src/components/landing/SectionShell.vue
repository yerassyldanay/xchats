<script setup lang="ts">
// SectionShell is the repeating wrapper for each landing section: a fade-up
// reveal (useRevealOnScroll) around an eyebrow pill, a title, an optional
// description, and whatever content the caller slots in. The head is always
// centered; `muted` puts the section on the app's #F7F7F7 surface band so
// white cards stand out against it.
import { ref } from 'vue'
import { useRevealOnScroll } from '@/composables/useRevealOnScroll'

withDefaults(defineProps<{ id: string; eyebrow: string; title: string; description?: string; muted?: boolean }>(), {
  description: undefined,
  muted: false,
})

const root = ref<HTMLElement | null>(null)
const { visible } = useRevealOnScroll(root)
</script>

<template>
  <section :id="id" ref="root" class="landing-section" :class="{ 'landing-section--muted': muted }">
    <div class="landing-shell landing-reveal" :class="{ 'is-visible': visible }">
      <div class="landing-section__head">
        <span class="landing-pill landing-pill--soft landing-section__eyebrow">{{ eyebrow }}</span>
        <h2 class="landing-section__title">{{ title }}</h2>
        <p v-if="description" class="landing-section__description">{{ description }}</p>
      </div>
      <!-- Exposed so a showcase's own timed reveal (a token flipping, a
           tool-call chip staggering in) can gate on the SAME visibility
           this wrapper uses — an animation-delay ticking while opacity:0
           would otherwise finish invisibly, and the payoff never shows. -->
      <slot :visible="visible" />
    </div>
  </section>
</template>
