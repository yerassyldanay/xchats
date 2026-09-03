<script setup lang="ts">
// The public landing page (/) — positions xchats as a free, open-source AI
// assistant that connects to multiple social platforms, eliminates
// hallucinations via strict template-based responses, and is configurable
// directly from ChatGPT/Claude over MCP. Deliberately built on the blog's
// visual language (landing.css imports the SAME tokens.css/chrome.css the
// prerendered blog uses — see landing.css's own header comment) rather
// than the authenticated app's Tailwind/shadcn one: this page and the blog
// are the two public-facing surfaces, and now read as one design system.
import '@/components/landing/landing.css'
import LandingNav from '@/components/landing/LandingNav.vue'
import LandingHero from '@/components/landing/LandingHero.vue'
import PlatformsSection from '@/components/landing/PlatformsSection.vue'
import ProductShowcase from '@/components/landing/ProductShowcase.vue'
import TeamSection from '@/components/landing/TeamSection.vue'
import TemplatesSection from '@/components/landing/TemplatesSection.vue'
import McpSection from '@/components/landing/McpSection.vue'
import DesktopSection from '@/components/landing/DesktopSection.vue'
import ArchitectureSection from '@/components/landing/ArchitectureSection.vue'
import HowItWorksStrip from '@/components/landing/HowItWorksStrip.vue'
import LandingLinks from '@/components/landing/LandingLinks.vue'
import LandingCta from '@/components/landing/LandingCta.vue'
import LandingFooter from '@/components/landing/LandingFooter.vue'
</script>

<template>
  <div class="landing h-full overflow-y-auto">
    <LandingNav />
    <main>
      <LandingHero />
      <PlatformsSection />
      <ProductShowcase />
      <TeamSection />
      <TemplatesSection />
      <McpSection />
      <DesktopSection />
      <ArchitectureSection />
      <HowItWorksStrip />
      <LandingLinks />
      <LandingCta />
    </main>
    <LandingFooter />
  </div>
</template>
