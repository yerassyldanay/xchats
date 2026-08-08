<script setup lang="ts">
// Showcase 2: multi-team with admin/member roles and per-conversation
// assignment — a mock of the real /accounts + /chatboard UI (role badges,
// the "Мои/Неназначенные/Все" filter pills, assigning a chat to a
// teammate), not a live data fetch.
import { useI18n } from 'vue-i18n'
import { UserRound } from 'lucide-vue-next'
import SectionShell from './SectionShell.vue'

const { t } = useI18n()
</script>

<template>
  <SectionShell id="team" :eyebrow="t('landing.team.eyebrow')" :title="t('landing.team.title')" :description="t('landing.team.description')" muted>
    <template #default="{ visible }">
      <div class="landing-team-card">
        <div class="landing-team-card__head">
          <span class="landing-role-badge landing-role-badge--admin">{{ t('landing.team.roleAdmin') }}</span>
          <span class="landing-role-badge">{{ t('landing.team.roleMember') }}</span>
        </div>

        <div class="landing-filter-pills">
          <span class="landing-filter-pill landing-filter-pill--active">{{ t('landing.team.filterMine') }}</span>
          <span class="landing-filter-pill">{{ t('landing.team.filterUnassigned') }}</span>
          <span class="landing-filter-pill">{{ t('landing.team.filterAll') }}</span>
        </div>

        <div class="landing-chat-row">
          <div class="landing-chat-row__avatar" aria-hidden="true"><UserRound class="w-4 h-4" /></div>
          <div class="landing-chat-row__body">
            <div class="landing-chat-row__name">{{ t('landing.team.customerName') }}</div>
            <div class="landing-chat-row__preview">{{ t('landing.team.messagePreview') }}</div>
          </div>
          <span v-if="visible" class="landing-assign-chip">{{ t('landing.team.assignedTo') }}</span>
        </div>
      </div>

      <p class="landing-footnote">{{ t('landing.team.note') }}</p>
    </template>
  </SectionShell>
</template>
