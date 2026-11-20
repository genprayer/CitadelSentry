<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Activity, ShieldCheck, Info, X } from 'lucide-vue-next'
import { useSettings } from '~/composables/useSettings'
import { useUpdater } from '~/composables/useUpdater'
import BaseButton from '~/components/ui/BaseButton.vue'
import BaseBadge from '~/components/ui/BaseBadge.vue'

const { settings, isLoaded, setTelemetryOptIn } = useSettings()
const { isModalOpen: isUpdaterModalOpen } = useUpdater()

const step = ref<'prompt' | 'declined_info'>('prompt')
const isSaving = ref(false)
const isDismissed = ref(false)
const isPromptActive = ref(false)

watch(
  () => [isLoaded.value, settings.value.general?.telemetryPromptShown, isUpdaterModalOpen.value],
  ([loaded, shown, updaterOpen]) => {
    if (loaded && shown === false && !updaterOpen && !isDismissed.value) {
      isPromptActive.value = true
    }
  },
  { immediate: true }
)

const isModalOpen = computed(() => {
  return isPromptActive.value && !isDismissed.value
})

const handleAccept = async () => {
  if (isSaving.value) return
  isSaving.value = true
  try {
    await setTelemetryOptIn(true)
    isDismissed.value = true
    isPromptActive.value = false
  } finally {
    isSaving.value = false
  }
}

const handleDecline = async () => {
  if (isSaving.value) return
  isSaving.value = true
  try {
    await setTelemetryOptIn(false)
    step.value = 'declined_info'
  } finally {
    isSaving.value = false
  }
}

const handleFinish = () => {
  isDismissed.value = true
  isPromptActive.value = false
}

const onKeyDown = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && isModalOpen.value && !isSaving.value) {
    if (step.value === 'declined_info') {
      handleFinish()
    } else {
      handleDecline()
    }
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKeyDown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeyDown)
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="isModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-background/95 animate-in fade-in duration-200"
    >
      <div
        class="w-full max-w-md bg-surface border-2 border-accent/40 rounded-2xl shadow-2xl shadow-accent/15 overflow-hidden flex flex-col animate-in zoom-in-95 duration-200"
      >
        <!-- Top accent banner -->
        <div class="bg-gradient-to-r from-accent/80 via-accent to-accent/80 px-4 py-2 flex items-center justify-between text-background font-mono text-xs font-bold uppercase tracking-wider">
          <div class="flex items-center space-x-2">
            <Activity class="w-4 h-4 shrink-0" />
            <span>{{ $t('telemetry.modalTitle') }}</span>
          </div>
          <button
            v-if="!isSaving"
            type="button"
            class="text-background/80 hover:text-background transition-colors focus:outline-none"
            @click="step === 'declined_info' ? handleFinish() : handleDecline()"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="p-6 space-y-5">
          <!-- Step 1: Initial Prompt -->
          <template v-if="step === 'prompt'">
            <div class="flex items-center space-x-4">
              <div class="w-13 h-13 rounded-2xl bg-accent/15 border border-accent/30 flex items-center justify-center text-accent shrink-0">
                <ShieldCheck class="w-7 h-7" />
              </div>
              <div>
                <h2 class="text-base font-bold font-sans tracking-wide text-primary uppercase">
                  {{ $t('telemetry.modalTitle') }}
                </h2>
                <div class="flex items-center space-x-2 mt-1">
                  <BaseBadge variant="neutral" size="sm">
                    {{ $t('telemetry.badge') }}
                  </BaseBadge>
                  <span class="text-xs text-primary/60 font-mono">100% Opt-In</span>
                </div>
              </div>
            </div>

            <p class="text-xs text-primary/80 leading-relaxed font-sans bg-background/50 p-3.5 rounded-xl border border-accent-secondary/15">
              {{ $t('telemetry.modalDesc') }}
            </p>

            <div class="flex items-center justify-end space-x-3 pt-2">
              <BaseButton
                variant="secondary"
                size="sm"
                :disabled="isSaving"
                @click="handleDecline"
              >
                {{ $t('telemetry.declineButton') }}
              </BaseButton>
              <BaseButton
                variant="primary"
                size="sm"
                :loading="isSaving"
                @click="handleAccept"
              >
                {{ $t('telemetry.acceptButton') }}
              </BaseButton>
            </div>
          </template>

          <!-- Step 2: Information if declined -->
          <template v-else>
            <div class="flex items-center space-x-4">
              <div class="w-13 h-13 rounded-2xl bg-sky-500/15 border border-sky-500/30 flex items-center justify-center text-sky-400 shrink-0">
                <Info class="w-7 h-7" />
              </div>
              <div>
                <h2 class="text-base font-bold font-sans tracking-wide text-primary uppercase">
                  {{ $t('telemetry.declinedTitle') }}
                </h2>
                <span class="text-xs text-sky-400 font-mono">Отключено</span>
              </div>
            </div>

            <div class="space-y-2.5 font-sans text-xs bg-background/50 p-3.5 rounded-xl border border-accent-secondary/15">
              <p class="text-primary/80 leading-relaxed">
                {{ $t('telemetry.declinedDesc') }}
              </p>
              <div class="p-2.5 bg-background/80 rounded-lg border border-accent/25 font-mono text-xs text-accent font-semibold flex items-center space-x-2">
                <span>📁</span>
                <span>{{ $t('telemetry.declinedPath') }}</span>
              </div>
            </div>

            <div class="flex items-center justify-end pt-2">
              <BaseButton
                variant="primary"
                size="sm"
                @click="handleFinish"
              >
                {{ $t('telemetry.finishButton') }}
              </BaseButton>
            </div>
          </template>
        </div>
      </div>
    </div>
  </Teleport>
</template>
