<script setup lang="ts">
import type { PostSeedAction, TimeoutAction } from '~/types'
import { POST_SEED_ACTION_OPTIONS } from '~/constants/seed'
import { useI18n } from '~/composables/useI18n'

const props = defineProps<{
  serverCount: number
  targetPlayers: number
  maxDurationMinutes: number
  postSeedAction: PostSeedAction
  timeoutAction: TimeoutAction
}>()

const { t } = useI18n()

const successAction = computed(() => {
  const option = POST_SEED_ACTION_OPTIONS.find(option => option.value === props.postSeedAction)
  return option ? t(option.labelKey) : t('common.unknown')
})

const timeoutDescription = computed(() => {
  switch (props.timeoutAction) {
    case 'continue_next': return t('modal.policyTimeoutNext')
    case 'close_squad': return t('modal.policyTimeoutClose')
    case 'sleep': return t('modal.policyTimeoutSleep')
    case 'shutdown': return t('modal.policyTimeoutShutdown')
    case 'retry': return t('modal.policyTimeoutRetry')
    case 'abort':
    case 'notify_only': return t('modal.policyTimeoutStop')
    default: return t('common.unknown')
  }
})

const successDescription = computed(() => t('modal.policySuccess', { action: successAction.value }))
const targetDescription = computed(() => t(props.serverCount > 1 ? 'modal.policyTargetNext' : 'modal.policyTargetSingle'))
</script>

<template>
  <section class="rounded-lg border border-accent-secondary/20 bg-surface/80 p-3 space-y-2" aria-labelledby="seed-policy-summary-title">
    <h3 id="seed-policy-summary-title" class="text-xs font-semibold text-primary">{{ t('modal.policySummaryTitle') }}</h3>
    <ul class="list-disc pl-4 space-y-2 text-xs leading-relaxed text-primary/80">
      <li>{{ t('modal.policyBudget', { count: serverCount, target: targetPlayers, minutes: maxDurationMinutes }) }}</li>
      <li>{{ targetDescription }}</li>
      <li>{{ timeoutDescription }}</li>
      <li>{{ successDescription }}</li>
    </ul>
    <p v-if="timeoutAction === 'continue_next'" class="text-xs leading-relaxed text-amber-200">{{ t('modal.policySkippedNotice') }}</p>
  </section>
</template>
