<script setup lang="ts">
// DraftKnowledgeBase is Черновик (/playground) — review-only: it answers
// exactly one question, "what unpublished changes will affect the knowledge
// base?" There are no Add buttons and no create forms anywhere on this page;
// every write lives on База знаний (see views/KnowledgeBase.vue) and stages
// into the SAME draft this page reviews.
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, LoaderCircle, Save, WandSparkles } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { usePlayground } from '@/stores/playground'
import { useDraftChanges } from '@/composables/useDraftChanges'
import { useEntityTabs } from '@/composables/useEntityTabs'
import EntityTabs from './EntityTabs.vue'
import StatTiles from './StatTiles.vue'
import ChangeList from './ChangeList.vue'
import ConfigChangeGroup from './ConfigChangeGroup.vue'
import DraftEmptyState from './DraftEmptyState.vue'
import TopicForm from './forms/TopicForm.vue'
import ProductForm from './forms/ProductForm.vue'
import TariffForm from './forms/TariffForm.vue'
import DeliveryZoneForm from './forms/DeliveryZoneForm.vue'
import ContactsForm from './forms/ContactsForm.vue'
import PoliciesForm from './forms/PoliciesForm.vue'
import AssistantFieldForm from './forms/AssistantFieldForm.vue'

const pg = usePlayground()
const { t } = useI18n()
const { loading, groups, counts, entriesFor, isEmpty } = useDraftChanges()
const { tabs, active } = useEntityTabs({ source: 'draft' })

onMounted(async () => {
  await Promise.all([pg.load(), pg.loadLive()])
  pg.startRealtime()
})
onBeforeUnmount(() => pg.stopRealtime())

const publishAllBusy = computed(() => pg.approving && !pg.publishingKey)

async function discardAll() {
  if (pg.pendingTotal && window.confirm('Отклонить весь черновик? Действие нельзя отменить.')) await pg.discard()
}
</script>

<template>
  <div class="h-full bg-background flex flex-col min-w-0">
    <header class="px-8 py-4 flex items-center justify-between gap-4 border-b border-border bg-card shrink-0">
      <div class="min-w-0">
        <h1 class="text-lg font-bold tracking-tight">{{ t('kb.draft.title') }}</h1>
        <p class="text-sm text-muted-foreground">{{ t('kb.draft.subtitle') }}</p>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <Button variant="ghost" size="sm" :disabled="!pg.pendingTotal || pg.busy" @click="discardAll">{{ t('kb.draft.rejectAll') }}</Button>
        <Button size="sm" :disabled="!pg.pendingTotal || pg.approving" @click="pg.approve()">
          <LoaderCircle v-if="publishAllBusy" class="w-4 h-4 animate-spin" />
          <Save v-else class="w-4 h-4" />
          {{ t('kb.draft.publishAll') }}<span v-if="pg.pendingTotal"> · {{ pg.pendingTotal }}</span>
        </Button>
      </div>
    </header>

    <div v-if="loading && isEmpty" class="flex-1 grid place-items-center p-8">
      <div class="text-center max-w-sm">
        <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
          <WandSparkles class="w-6 h-6" />
        </div>
        <p class="text-sm text-muted-foreground">Загрузка черновика…</p>
      </div>
    </div>

    <DraftEmptyState v-else-if="isEmpty" />

    <div v-else class="flex-1 overflow-y-auto px-8 py-6 space-y-6">
      <StatTiles :counts="counts" />

      <EntityTabs :tabs="tabs" v-model="active" />

      <div v-for="group in groups" :key="group.kind" v-show="active === group.kind">
        <ConfigChangeGroup v-if="group.kind === 'config'" :entries="group.entries" />
        <ChangeList v-else :kind="group.kind" :entries="entriesFor(group.kind)" />
      </div>

      <div v-if="pg.gateReasons" class="rounded-lg bg-destructive/10 p-3 space-y-1">
        <p class="flex items-start gap-2 text-sm font-medium text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ t('kb.draft.gateErrorTitle') }}
        </p>
        <p class="text-sm text-destructive/90 pl-6">{{ pg.gateReasons }}</p>
      </div>
      <p v-else-if="pg.error" class="flex items-center gap-2 text-sm text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0" /> {{ pg.error }}
      </p>
    </div>
  </div>

  <TopicForm />
  <ProductForm />
  <TariffForm />
  <DeliveryZoneForm />
  <ContactsForm />
  <PoliciesForm />
  <AssistantFieldForm />
</template>
