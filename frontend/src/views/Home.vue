<script setup lang="ts">
// The public landing page (/) — says what xchats is (a free, open-source team
// inbox for WhatsApp and Telegram with an AI assistant that answers only from
// your knowledge base), walks through every section of the app, then covers
// channels, roles, the no-made-up-numbers guarantee, MCP setup, downloads and
// architecture. It is built in the signed-in app's own visual language —
// Manrope, ink on white, bordered cards, pills, underline tabs — via
// landing.css, so the public page and the app read as one product. (The
// prerendered blog is a separate surface and keeps its own sheet.)
import '@/components/landing/landing.css'
import LandingNav from '@/components/landing/LandingNav.vue'
import LandingHero from '@/components/landing/LandingHero.vue'
import OverviewSection from '@/components/landing/OverviewSection.vue'
import FeaturesSection from '@/components/landing/FeaturesSection.vue'
import ProductShowcase from '@/components/landing/ProductShowcase.vue'
import PlatformsSection from '@/components/landing/PlatformsSection.vue'
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
      <OverviewSection />
      <FeaturesSection />
      <ProductShowcase />
      <PlatformsSection />
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
