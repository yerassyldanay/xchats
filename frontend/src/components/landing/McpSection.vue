<script setup lang="ts">
// Showcase 4: configuring the knowledge base from inside a ChatGPT/Claude
// chat over MCP. xchats has no URL fetcher or PDF extractor of its own —
// the honest mechanism is that the LLM CLIENT reads the attached
// documents itself and stores structured facts through the 13 kb_* MCP
// tools (backend/internal/mcpserver/tools.go). Every one of those writes
// lands in kbd_draft, never live — this is the same Черновик /playground
// reviews, which is why the closing card says "review", not "published".
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleCheck, FileText, Link2, Wrench } from 'lucide-vue-next'
import { useRevealOnScroll } from '@/composables/useRevealOnScroll'
import SectionShell from './SectionShell.vue'

const { t } = useI18n()

const TOOLS = ['kb_product_upsert', 'kb_tariff_upsert', 'kb_delivery_zone_upsert']

const demoRef = ref<HTMLElement | null>(null)
const { visible: demoVisible } = useRevealOnScroll(demoRef, 0.4)

const showTools = ref(false)
const showResult = ref(false)
// immediate: true matters here, not just style — prefers-reduced-motion
// makes useRevealOnScroll's `visible` start at true (never false→true), so
// a plain watch() would never fire and this demo would never populate.
watch(
  demoVisible,
  (v) => {
    if (!v) return
    showTools.value = true
    window.setTimeout(() => (showResult.value = true), 1500)
  },
  { immediate: true }
)
</script>

<template>
  <SectionShell id="mcp" :eyebrow="t('landing.mcp.eyebrow')" :title="t('landing.mcp.title')" :description="t('landing.mcp.description')" muted>
    <div ref="demoRef" class="landing-mcp-demo">
      <p class="landing-bubble landing-bubble--customer landing-bubble--wide">
        {{ t('landing.mcp.userMessage') }}
        <span class="landing-attachments">
          <span class="landing-attachment-chip"><FileText aria-hidden="true" /> {{ t('landing.mcp.pdfLabel') }}</span>
          <span class="landing-attachment-chip"><Link2 aria-hidden="true" /> {{ t('landing.mcp.urlLabel') }}</span>
        </span>
      </p>

      <div v-if="showTools" class="landing-tool-calls">
        <div v-for="(tool, i) in TOOLS" :key="tool" class="landing-tool-chip" :style="{ animationDelay: `${i * 220}ms` }">
          <Wrench aria-hidden="true" /> {{ tool }}
        </div>
      </div>

      <div v-if="showResult" class="landing-result-card">
        <CircleCheck aria-hidden="true" />
        <span><strong>{{ t('landing.mcp.resultTitle') }}</strong> — {{ t('landing.mcp.resultBody') }}</span>
      </div>
    </div>

    <p class="landing-note">{{ t('landing.mcp.note') }}</p>
  </SectionShell>
</template>
