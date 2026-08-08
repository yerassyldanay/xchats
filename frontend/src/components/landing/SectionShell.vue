<script setup lang="ts">
// SectionShell is the repeating wrapper for each of the four MCP/platforms/
// team/templates showcase sections: a fade-up reveal (useRevealOnScroll)
// around an eyebrow badge, a title, an optional description, and whatever
// demo content the caller slots in. Every showcase's content block
// (.landing-platforms, .landing-team-card, .landing-chat-demo,
// .landing-mcp-demo — see landing.css) is itself centered, so the head is
// centered too rather than offering a left-aligned variant nothing here
// uses.
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
    <div class="blog-shell landing-reveal" :class="{ 'is-visible': visible }">
      <div class="landing-section__head landing-section__head--center">
        <span class="badge badge--category landing-section__eyebrow">{{ eyebrow }}</span>
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
