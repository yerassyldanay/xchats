<script setup lang="ts">
import KbItemCard from './KbItemCard.vue'
import KbListCard from './KbListCard.vue'
import KbComparisonCard from './KbComparisonCard.vue'
import type { ChatComponent } from '@/types'

// The one place a component `type` is turned into a card. An unrecognized
// type renders nothing rather than an error box: a backend that starts
// sending a component this build has never heard of should degrade to the
// prose answer, which is always complete on its own.
defineProps<{ component: ChatComponent }>()
</script>

<template>
  <KbItemCard v-if="component.type === 'kb_item'" :data="component.data" />
  <KbListCard v-else-if="component.type === 'kb_list'" :data="component.data" />
  <KbComparisonCard v-else-if="component.type === 'kb_comparison'" :data="component.data" />
</template>
