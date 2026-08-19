<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { annotateMediaExpect, formatYamlValue, notCheckedRequirements, resolveRequires } from '@/lib/evalCatalog'
import { TEST_FIELD_KEYS, type TestFieldKey } from '@/lib/evalFieldDocs'
import type { CatalogScenario, CatalogTestCase } from '@/types'
import CatalogFieldRow from './CatalogFieldRow.vue'
import FieldLegend from './FieldLegend.vue'
import CopyButton from './CopyButton.vue'

// The requirements centerpiece — every tests.yaml key shown RAW, in its
// authored order, with a (?) tooltip explaining what it evaluates: this is a
// developer audience, so exact schema over friendly renamed abstractions.
// Resolved fact values / broken-ref flags are ANNOTATIONS beneath the raw value, never
// a replacement for it. Read-only review, never a run/results view.
const props = defineProps<{ test: CatalogTestCase; scenario: CatalogScenario }>()
const { t } = useI18n()
const route = useRoute()

const requireGroups = computed(() => resolveRequires(props.test.requires, props.scenario.facts))
const mediaAnnotations = computed(() => annotateMediaExpect(props.test.media, props.scenario.media_tokens))
const notChecked = computed(() => notCheckedRequirements(props.test))
const deepLink = computed(() => `${window.location.origin}${route.fullPath}`)

// Presence predicates in TEST_FIELD_KEYS' own canonical order — presentKeys drives
// BOTH the side legend and, indirectly (via the v-if guards below), which rows the
// main column actually renders. Keeping this as a single source of truth means the
// legend can never list a field the main column doesn't also show, or vice versa.
const presentPredicates: Record<TestFieldKey, (t: CatalogTestCase) => boolean> = {
  id: () => true,
  source: () => true,
  message: () => true,
  history: (t) => !!t.history?.length,
  requires: (t) => !!t.requires?.length,
  escalate: (t) => t.escalate !== undefined,
  language: (t) => !!t.language,
  must_not_contain: (t) => !!t.must_not_contain?.length,
  must_contain_any: (t) => !!t.must_contain_any?.length,
  forbid_tokens: (t) => !!t.forbid_tokens?.length,
  media: (t) => !!t.media,
  outcomes: (t) => !!t.outcomes?.length,
  stock_check: (t) => !!t.stock_check,
  llm_checks: (t) => !!t.llm_checks?.length,
}
const presentKeys = computed(() => TEST_FIELD_KEYS.filter((k) => presentPredicates[k](props.test)))
</script>

<template>
  <div class="flex gap-6 items-start">
    <div class="max-w-3xl min-w-0 flex-1 space-y-4">
      <div>
        <div class="flex items-center gap-1.5">
          <code class="font-mono text-sm font-semibold wrap-break-word">{{ test.id }}</code>
          <CopyButton :text="test.id" :label="t('evalCatalog.copy.id')" />
          <CopyButton :text="deepLink" :label="t('evalCatalog.copy.link')" />
        </div>
        <p class="text-[11px] text-muted-foreground mt-1">
          {{ t('evalCatalog.fields.source') }} → <code class="font-mono">{{ test.source }}</code>
        </p>
      </div>

      <CatalogFieldRow field-key="message">
        <p class="text-sm whitespace-pre-wrap wrap-break-word rounded-lg bg-muted/40 p-2.5">{{ test.message }}</p>
      </CatalogFieldRow>

      <CatalogFieldRow v-if="test.history?.length" field-key="history">
        <pre class="font-mono text-xs whitespace-pre-wrap wrap-break-word rounded-lg bg-muted/40 p-2.5">{{ formatYamlValue(test.history) }}</pre>
      </CatalogFieldRow>

      <CatalogFieldRow v-if="test.requires?.length" field-key="requires">
        <pre class="font-mono text-xs whitespace-pre-wrap wrap-break-word rounded-lg bg-muted/40 p-2.5">{{ formatYamlValue(test.requires) }}</pre>
        <div class="pl-3 mt-1 space-y-0.5">
          <div v-for="(g, i) in requireGroups" :key="i" class="text-[11px]">
            <template v-for="(alt, j) in g.alternatives" :key="alt.token">
              <code class="font-mono text-muted-foreground">{{ alt.token }}</code>
              <template v-if="alt.value !== null"> → «{{ alt.value }}»</template>
              <span v-else class="text-destructive font-medium"> → {{ t('evalCatalog.valueNotFound') }}</span>
              <span v-if="j < g.alternatives.length - 1" class="text-muted-foreground"> / </span>
            </template>
          </div>
        </div>
      </CatalogFieldRow>

      <CatalogFieldRow v-if="test.escalate !== undefined" field-key="escalate">
        <p class="text-xs">{{ test.escalate ? 'обязательна' : 'эскалации быть не должно' }}</p>
      </CatalogFieldRow>

      <CatalogFieldRow v-if="test.language" field-key="language">
        <p class="text-xs font-mono">{{ test.language }}</p>
      </CatalogFieldRow>

      <CatalogFieldRow v-if="test.media" field-key="media">
        <pre class="font-mono text-xs whitespace-pre-wrap wrap-break-word rounded-lg bg-muted/40 p-2.5">{{ formatYamlValue(test.media) }}</pre>
        <div v-if="mediaAnnotations.length" class="pl-3 mt-1 space-y-0.5">
          <div v-for="a in mediaAnnotations" :key="`${a.key}:${a.name}`" class="text-[11px]">
            <code class="font-mono text-muted-foreground">{{ a.name }}</code>
            <span v-if="!a.found" class="text-destructive font-medium"> → {{ t('evalCatalog.mediaNotFound') }}</span>
          </div>
        </div>
      </CatalogFieldRow>

      <CatalogFieldRow v-if="test.must_not_contain?.length" field-key="must_not_contain">
        <pre class="font-mono text-xs whitespace-pre-wrap wrap-break-word rounded-lg bg-muted/40 p-2.5">{{ formatYamlValue(test.must_not_contain) }}</pre>
      </CatalogFieldRow>

      <CatalogFieldRow v-if="test.must_contain_any?.length" field-key="must_contain_any">
        <pre class="font-mono text-xs whitespace-pre-wrap wrap-break-word rounded-lg bg-muted/40 p-2.5">{{ formatYamlValue(test.must_contain_any) }}</pre>
      </CatalogFieldRow>

      <CatalogFieldRow v-if="test.forbid_tokens?.length" field-key="forbid_tokens">
        <pre class="font-mono text-xs whitespace-pre-wrap wrap-break-word rounded-lg bg-muted/40 p-2.5">{{ formatYamlValue(test.forbid_tokens) }}</pre>
      </CatalogFieldRow>

      <CatalogFieldRow v-if="test.outcomes?.length" field-key="outcomes">
        <pre class="font-mono text-xs whitespace-pre-wrap wrap-break-word rounded-lg bg-muted/40 p-2.5">{{ formatYamlValue(test.outcomes) }}</pre>
      </CatalogFieldRow>

      <CatalogFieldRow v-if="test.stock_check" field-key="stock_check">
        <p class="text-xs font-mono rounded-lg bg-muted/40 p-2.5 w-fit">{{ test.stock_check }}</p>
      </CatalogFieldRow>

      <CatalogFieldRow v-if="test.llm_checks?.length" field-key="llm_checks">
        <pre class="font-mono text-xs whitespace-pre-wrap wrap-break-word rounded-lg bg-muted/40 p-2.5">{{ formatYamlValue(test.llm_checks) }}</pre>
      </CatalogFieldRow>

      <div v-if="notChecked.length" class="text-[11px] text-muted-foreground border-t border-border pt-3">
        {{ t('evalCatalog.notDeclared') }}
        <template v-for="(k, i) in notChecked" :key="k">
          <code class="font-mono">{{ k }}</code><span v-if="i < notChecked.length - 1"> · </span>
        </template>
      </div>
    </div>

    <FieldLegend :declared-keys="presentKeys" class="hidden xl:block w-64 shrink-0 sticky top-0" />
  </div>
</template>
