<script setup lang="ts">
import type { SeedState } from '~/types'
import BaseBadge from '~/components/ui/BaseBadge.vue'
import { useI18n } from '~/composables/useI18n'

interface Props {
  state: SeedState
  size?: 'sm' | 'md'
}

const props = withDefaults(defineProps<Props>(), {
  size: 'md'
})
const { t } = useI18n()

const badgeConfig = computed(() => {
  switch (props.state) {
    case 'active':
      return { variant: 'success' as const, label: t('dashboard.stateActive'), dot: true }
    case 'connecting':
      return { variant: 'warning' as const, label: t('dashboard.stateConnecting'), dot: true }
    case 'transitioning':
      return { variant: 'warning' as const, label: t('dashboard.stateTransitioning'), dot: true }
    case 'preparing':
      return { variant: 'info' as const, label: t('dashboard.statePreparing'), dot: true }
    case 'completed':
      return { variant: 'brand' as const, label: t('dashboard.stateCompleted'), dot: false }
    case 'timed_out':
      return { variant: 'warning' as const, label: t('dashboard.stateTimedOut'), dot: false }
    case 'stopped':
      return { variant: 'neutral' as const, label: t('dashboard.stateStopped'), dot: false }
    case 'failed':
      return { variant: 'danger' as const, label: t('dashboard.stateFailed'), dot: false }
    case 'idle':
    default:
      return { variant: 'neutral' as const, label: t('dashboard.stateIdle'), dot: false }
  }
})
</script>

<template>
  <BaseBadge
    :variant="badgeConfig.variant"
    :size="size"
    :dot="badgeConfig.dot"
  >
    {{ badgeConfig.label }}
  </BaseBadge>
</template>
