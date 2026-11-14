<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Moon, Power, AlertTriangle, ShieldAlert, XCircle } from 'lucide-vue-next'
import { bridgeAdapter } from '~/adapters/wails/bridgeAdapter'
import { useSeed } from '~/composables/useSeed'
import { useI18n } from '~/composables/useI18n'
import { BRIDGE_EVENTS } from '~/types/bridge'
import BaseButton from '~/components/ui/BaseButton.vue'
import BaseProgress from '~/components/ui/BaseProgress.vue'

const { cancelPowerAction } = useSeed()
const { t } = useI18n()

const isOpen = ref(false)
const action = ref<'sleep' | 'shutdown' | null>(null)
const secondsLeft = ref(0)
const countdownTotal = ref(0)
const isCancelling = ref(false)
const cancelError = ref<string | null>(null)
const unsubscribeEvents: Array<() => void> = []

const handleImpending = (data: { action: 'sleep' | 'shutdown'; secondsLeft: number }) => {
  if (!data || (data.action !== 'sleep' && data.action !== 'shutdown') ||
    !Number.isSafeInteger(data.secondsLeft) || data.secondsLeft < 0) return
  if (!isOpen.value || action.value !== data.action || data.secondsLeft > secondsLeft.value) {
    countdownTotal.value = data.secondsLeft
    cancelError.value = null
  }
  action.value = data.action
  secondsLeft.value = data.secondsLeft
  isOpen.value = true
}

const handleCancelled = () => {
  isOpen.value = false
  isCancelling.value = false
}

const progressPercent = computed(() => {
  if (countdownTotal.value === 0) return 0
  return Math.round((secondsLeft.value / countdownTotal.value) * 100)
})

const isSleep = computed(() => action.value === 'sleep')

const titleText = computed(() => {
  return isSleep.value ? t('powerCountdown.titleSleep') : t('powerCountdown.titleShutdown')
})

const actionNameText = computed(() => {
  return isSleep.value ? t('powerCountdown.actionSleep') : t('powerCountdown.actionShutdown')
})

const onCancel = async () => {
  isCancelling.value = true
  cancelError.value = null
  try {
    await cancelPowerAction()
    isOpen.value = false
  } catch (err) {
    console.error('Failed to cancel power action', err)
    cancelError.value = err instanceof Error ? err.message : String(err)
  } finally {
    isCancelling.value = false
  }
}

const onKeyDown = (e: KeyboardEvent) => {
  if (isOpen.value && e.key === 'Escape') {
    onCancel()
  }
}

onMounted(() => {
  unsubscribeEvents.push(bridgeAdapter.onEvent<{ action: 'sleep' | 'shutdown'; secondsLeft: number }>(
    BRIDGE_EVENTS.POWER_ACTION_IMPENDING,
    handleImpending
  ))
  unsubscribeEvents.push(bridgeAdapter.onEvent(BRIDGE_EVENTS.POWER_ACTION_CANCELLED, handleCancelled))
  window.addEventListener('keydown', onKeyDown)
})

onUnmounted(() => {
  for (const unsubscribe of unsubscribeEvents) unsubscribe()
  window.removeEventListener('keydown', onKeyDown)
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="isOpen"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-background/95 animate-in fade-in duration-200"
    >
      <div
        class="w-full max-w-lg bg-surface border-2 border-amber-500/50 rounded-2xl shadow-2xl shadow-amber-500/10 overflow-hidden flex flex-col animate-in zoom-in-95 duration-200"
      >
        <!-- Top banner warning bar -->
        <div class="bg-gradient-to-r from-amber-600 via-amber-500 to-amber-600 px-4 py-2 flex items-center justify-center space-x-2 text-black font-mono text-xs font-bold uppercase tracking-wider">
          <AlertTriangle class="w-4 h-4 shrink-0 animate-bounce" />
          <span>{{ $t('powerCountdown.attention') }}</span>
          <AlertTriangle class="w-4 h-4 shrink-0 animate-bounce" />
        </div>

        <div class="p-6 space-y-6">
          <!-- Icon & Title -->
          <div class="flex items-center space-x-4">
            <div class="w-14 h-14 rounded-2xl bg-amber-500/15 border border-amber-500/30 flex items-center justify-center text-amber-400 shrink-0">
              <Moon v-if="isSleep" class="w-7 h-7" />
              <Power v-else class="w-7 h-7" />
            </div>
            <div>
              <h2 class="text-xl font-bold font-sans tracking-wide text-primary uppercase">
                {{ titleText }}
              </h2>
              <p class="text-xs text-primary/60 font-mono mt-0.5">
                {{ $t('powerCountdown.subtitle') }}
              </p>
            </div>
          </div>

          <!-- Big Countdown Display -->
          <div class="bg-surface-secondary/70 border border-amber-500/30 rounded-xl p-5 text-center space-y-3">
            <p class="text-xs font-mono uppercase tracking-widest text-primary/60">
              {{ $t('powerCountdown.warning', { action: actionNameText, seconds: secondsLeft }) }}
            </p>

            <div class="font-mono text-6xl font-extrabold text-amber-400 tracking-tight flex items-baseline justify-center space-x-2">
              <span>{{ secondsLeft }}</span>
              <span class="text-lg font-normal text-primary/40 uppercase">{{ $t('common.minutes') === 'мин' ? 'сек' : 'sec' }}</span>
            </div>

            <!-- Progress bar -->
            <div class="pt-2">
              <BaseProgress
                :value="progressPercent"
                variant="warning"
                :show-value="false"
                animated
              />
            </div>
          </div>

          <!-- Safety tip -->
          <div class="p-3 bg-amber-500/10 border border-amber-500/20 rounded-lg flex items-start space-x-2.5 text-xs text-amber-200/90 leading-relaxed">
            <ShieldAlert class="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
            <span>{{ $t('powerCountdown.safetyTip') }}</span>
          </div>

          <!-- Cancel Action Button -->
          <div class="flex flex-col sm:flex-row items-center justify-between gap-3 pt-2">
            <BaseButton
              variant="danger"
              size="lg"
              class="w-full font-bold text-sm py-3 uppercase tracking-wider shadow-lg shadow-rose-900/30"
              :loading="isCancelling"
              @click="onCancel"
            >
              <XCircle class="w-5 h-5 mr-2" />
              <span>{{ $t('powerCountdown.cancelButton') }}</span>
            </BaseButton>
          </div>
          <p v-if="cancelError" role="alert" class="text-xs text-rose-300">{{ cancelError }}</p>
        </div>
      </div>
    </div>
  </Teleport>
</template>
