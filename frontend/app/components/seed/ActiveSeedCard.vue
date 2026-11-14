<script setup lang="ts">
import { Square, Clock, Users, Flame, Radio, AlertCircle, FastForward, Copy, Check } from 'lucide-vue-next'
import { useSeed } from '~/composables/useSeed'
import { useFeedbackTimer } from '~/composables/use-feedback-timer'
import BaseCard from '~/components/ui/BaseCard.vue'
import BaseButton from '~/components/ui/BaseButton.vue'
import BaseProgress from '~/components/ui/BaseProgress.vue'
import SeedStateIndicator from '~/components/seed/SeedStateIndicator.vue'
import SentryChronometer from '~/components/sentry/SentryChronometer.vue'
import CiTooltip from '~/components/ui/CiTooltip.vue'
import { useI18n } from '~/composables/useI18n'

const {
  session,
  queue,
  isSeedingActive,
  isSeedingIdle,
  progressPercent,
  formattedElapsedTime,
  formattedRemainingTime,
  stopSeed,
  skipToNextServer,
  isLoading,
  error,
  sessionStatusError,
  isSessionStatusKnown,
  fetchCurrentSession
} = useSeed()
const { t } = useI18n()

const emit = defineEmits<{
  selectServer: []
}>()

const isSkipping = ref(false)
const isCopied = ref(false)
const scheduleCopyFeedback = useFeedbackTimer()

const getCitadelPort = (id?: string) => {
  switch (id) {
    case '1': return 7787
    case '2': return 7792
    case '3': return 7797
    case '4': return 7802
    case '6': return 7812
    default: return 7787
  }
}

const handleCopyConnectCommand = async () => {
  const port = getCitadelPort(session.value?.server?.id || session.value?.plan?.serverId)
  const cmd = `connect 62.122.215.132:${port}`
  try {
    await navigator.clipboard.writeText(cmd)
    isCopied.value = true
    scheduleCopyFeedback(() => {
      isCopied.value = false
    }, 2500)
  } catch (err) {
    console.warn('Failed to copy connect command', err)
  }
}

const handleSkip = async () => {
  if (isSkipping.value) return
  isSkipping.value = true
  try {
    await skipToNextServer()
  } catch (err) {
    console.error('Failed to skip server', err)
  } finally {
    isSkipping.value = false
  }
}

const stateLabel = computed(() => {
  switch (session.value.state) {
    case 'active': return t('dashboard.stateActive')
    case 'connecting': return t('dashboard.stateConnecting')
    case 'transitioning': return t('dashboard.stateTransitioning')
    case 'preparing': return t('dashboard.statePreparing')
    case 'completed': return t('dashboard.stateCompleted')
    case 'timed_out': return t('dashboard.stateTimedOut')
    case 'stopped': return t('dashboard.stateStopped')
    case 'failed': return t('dashboard.stateFailed')
    default: return t('dashboard.stateIdle')
  }
})

const postSeedLabel = computed(() => {
  switch (session.value.plan?.postSeedAction) {
    case 'close_squad': return t('dashboard.actionCloseSquad')
    case 'shutdown': return t('dashboard.actionShutdown')
    case 'sleep': return t('dashboard.actionSleep')
    case 'do_nothing': return t('dashboard.actionDoNothing')
    default: return t('common.unknown')
  }
})

const handleStop = async () => {
  try {
    await stopSeed()
  } catch (err) {
    console.error('Failed to stop seed session', err)
  }
}
</script>

<template>
  <div class="space-y-4">
    <p v-if="error && isSessionStatusKnown" class="text-sm text-rose-300" role="alert">{{ error }}</p>
    <BaseCard v-if="!isSessionStatusKnown" variant="panel" padding="lg" class="space-y-3 border-amber-500/40">
      <p class="font-semibold text-amber-300">{{ $t('dashboard.sessionUnavailable') }}</p>
      <p class="text-sm text-primary/70">{{ error || sessionStatusError || $t('dashboard.sessionUnavailableDescription') }}</p>
      <div class="flex gap-3">
        <BaseButton variant="secondary" :loading="isLoading" @click="fetchCurrentSession">{{ $t('common.retry') }}</BaseButton>
        <BaseButton v-if="isSeedingActive" variant="danger" :loading="isLoading" @click="handleStop">{{ $t('dashboard.stopSeed') }}</BaseButton>
      </div>
    </BaseCard>
    <!-- Active / In-Progress Seeding HUD -->
    <BaseCard
      v-else-if="!isSeedingIdle"
      variant="elevated"
      padding="lg"
      class="border-accent/40 relative overflow-hidden bg-surface"
    >
      <!-- Top Tactical Header -->
      <div class="flex flex-wrap items-center justify-between gap-3 pb-4 border-b border-accent-secondary/15">
        <div class="flex items-center space-x-3">
          <div class="p-2.5 rounded-xl bg-accent/10 border border-accent/30 text-accent">
            <Flame class="w-6 h-6 animate-pulse text-squad-orange" />
          </div>
          <div>
            <div class="flex items-center space-x-2">
              <h2 class="text-base font-bold text-primary font-sans tracking-wide uppercase">
                {{ session.server?.name || session.plan?.serverName || t('dashboard.operationalSeed') }}
              </h2>
            </div>
            <p class="text-xs text-primary/60 font-mono flex items-center space-x-2 mt-0.5">
              <span v-if="session.server?.currentMap">{{ session.server.currentMap }}</span>
              <span v-if="session.server?.currentLayer">• {{ session.server.currentLayer }}</span>
              <span v-if="session.plan?.serverIp">• {{ session.plan.serverIp }}:{{ session.plan.gamePort }}</span>
            </p>
          </div>
        </div>

        <!-- State Badge -->
        <SeedStateIndicator :state="session.state" size="md" />
      </div>

      <p v-if="['preparing', 'transitioning', 'connecting'].includes(session.state)" class="mt-3 text-xs text-primary/70" role="status">
        {{ t('dashboard.connectionPending') }}
      </p>

      <!-- Main Telemetry Grid with Tactical Chronometer -->
      <div class="grid grid-cols-1 lg:grid-cols-4 gap-4 py-4 items-center">
        <!-- Live Chronometer Mission Dial (1 column) -->
        <div class="flex justify-center">
          <SentryChronometer size="sm" :show-digital-time="false" />
        </div>

        <!-- Telemetry Stat Blocks (3 columns) -->
        <div class="lg:col-span-3 grid grid-cols-1 sm:grid-cols-3 gap-3">
          <!-- Progress Counter -->
          <div class="bg-surface/80 p-4 rounded-xl border border-accent-secondary/20 space-y-1.5">
            <div class="flex items-center justify-between text-xs font-sans text-primary/70">
              <span class="flex items-center space-x-1.5">
                <Users class="w-4 h-4 text-accent" />
                <span>{{ $t('dashboard.targetPlayers') }}</span>
              </span>
              <span class="text-accent font-bold">{{ progressPercent }}%</span>
            </div>

            <div class="text-2xl font-bold font-mono text-primary flex items-baseline space-x-1.5">
              <span>{{ session.currentPlayerCount }}</span>
              <span class="text-sm font-normal text-primary/50">/ {{ session.targetPlayerCount }}</span>
            </div>

            <BaseProgress :value="progressPercent" :show-value="false" variant="brand" :animated="isSeedingActive" />
          </div>

          <!-- Elapsed & Timeout Countdown -->
          <div class="bg-surface/80 p-4 rounded-xl border border-accent-secondary/20 space-y-1.5">
            <div class="flex items-center justify-between text-xs font-sans text-primary/70">
              <span class="flex items-center space-x-1.5">
                <Clock class="w-4 h-4 text-accent" />
                <span>{{ $t('dashboard.timeElapsed') }}</span>
              </span>
              <span v-if="formattedRemainingTime" class="text-squad-orange font-semibold text-[11px]">
                {{ formattedRemainingTime }} {{ $t('dashboard.timeRemaining') }}
              </span>
            </div>

            <div class="text-2xl font-bold font-mono text-primary">
              {{ formattedElapsedTime }}
            </div>

            <p class="text-[11px] font-mono text-primary/60 flex items-center justify-between">
              <span>{{ $t('dashboard.maxDuration') }}: <strong>{{ session.plan?.maxDurationMinutes ?? $t('common.unknown') }} {{ $t('common.minutes') }}</strong></span>
              <span v-if="session.plan?.serverLimit !== undefined && session.plan?.serverLimit > 1" class="text-accent">
                {{ $t('dashboard.serverLimit') }}: {{ session.plan.serverLimit }}
              </span>
            </p>
          </div>

          <!-- Squad Status & Queue Order -->
          <div class="bg-surface/80 p-4 rounded-xl border border-accent-secondary/20 space-y-1.5">
            <div class="flex items-center justify-between text-xs font-sans text-primary/70">
              <span class="flex items-center space-x-1.5">
                <Radio class="w-4 h-4 text-emerald-400" />
                <span>{{ $t('dashboard.squadAndQueue') }}</span>
              </span>
              <span class="text-[11px] text-primary/60 font-bold">
                #{{ session.currentQueueIndex ?? $t('common.unknown') }} / {{ session.totalQueueCount ?? $t('common.unknown') }}
              </span>
            </div>

            <div class="text-lg font-bold font-mono text-primary flex items-center space-x-2">
              <span v-if="session.queuePosition > 0" class="text-squad-orange">
                {{ $t('dashboard.queue') }}: {{ session.queuePosition }}
              </span>
              <span v-else-if="session.state === 'active'" class="text-emerald-400">
                {{ $t('dashboard.connected') }}
              </span>
              <span v-else class="text-primary/70">
                {{ stateLabel }}
              </span>
            </div>

            <p class="text-[11px] font-mono text-primary/60 truncate">
              {{ $t('dashboard.postSeed') }}: <strong class="text-primary">{{ postSeedLabel }}</strong>
            </p>
          </div>
        </div>
      </div>

      <p v-if="session.skippedServerCount" class="mb-3 text-xs text-amber-200" role="status">
        {{ t('dashboard.skippedServers', { count: session.skippedServerCount }) }}
      </p>

      <!-- Error alert if session failed -->
      <div v-if="session.errorMessage" class="p-3 bg-rose-500/10 border border-rose-500/30 rounded-lg flex items-center space-x-2 text-xs text-rose-300 my-2">
        <AlertCircle class="w-4 h-4 text-rose-400 flex-shrink-0" />
        <span>{{ session.errorMessage }}</span>
      </div>

      <!-- Automatic Steam Direct Connect Status -->
      <div
        v-if="session.state === 'connecting' || session.state === 'preparing'"
        class="p-3 bg-surface border border-accent/30 rounded-xl space-y-1.5 text-xs"
      >
        <div class="flex items-center justify-between">
          <span class="text-accent font-semibold flex items-center gap-1.5 font-sans">
            <Radio class="w-3.5 h-3.5 animate-pulse text-accent" />
            <span>{{ $t('settings.directConnectTitle') || 'Прямое подключение к серверу' }}</span>
          </span>
          <span class="text-[10px] text-emerald-400 font-mono font-semibold bg-emerald-500/10 border border-emerald-500/20 px-2 py-0.5 rounded">
            62.122.215.132:{{ getCitadelPort(session.server?.id || session.plan?.serverId) }}
          </span>
        </div>
        <p class="text-[11px] text-primary/70 leading-normal">
          Автоматическое подключение через прямой протокол Steam. Вход на сервер выполняется автоматически даже при 0 игроков.
        </p>
      </div>

      <!-- Action Footer -->
      <div class="pt-4 border-t border-accent-secondary/15 flex flex-wrap items-center justify-between gap-3">
        <div class="text-xs font-mono text-primary/50 flex items-center space-x-2">
          <span>{{ $t('dashboard.serverLimit') }}: <strong>{{ session.plan?.serverLimit !== undefined ? (session.plan.serverLimit === 0 ? $t('dashboard.allServers') : session.plan.serverLimit) : $t('common.unknown') }}</strong></span>
        </div>

        <div class="flex items-center space-x-3">
          <CiTooltip
            v-if="isSeedingActive"
            title="Пропустить сервер"
            description="Завершить текущий сервер и перейти к следующему в очереди миссии"
          >
            <BaseButton
              variant="secondary"
              size="md"
              :loading="isSkipping"
              @click="handleSkip"
            >
              <FastForward class="w-4 h-4 mr-2 text-accent" />
              <span>{{ $t('dashboard.skipServer') }}</span>
            </BaseButton>
          </CiTooltip>

          <BaseButton
            v-if="isSeedingActive"
            variant="danger"
            size="md"
            :loading="isLoading"
            @click="handleStop"
          >
            <Square class="w-4 h-4 mr-2" />
            <span>{{ $t('dashboard.stopSeed') }}</span>
          </BaseButton>
          <BaseButton
            v-else
            variant="secondary"
            size="md"
            @click="emit('selectServer')"
          >
            {{ $t('dashboard.selectServer') }}
          </BaseButton>
        </div>
      </div>
    </BaseCard>

    <!-- Standby / Idle Sentry Chronometer HUD Card -->
    <BaseCard
      v-else
      variant="panel"
      padding="lg"
      class="border-accent/30 text-center space-y-6 max-w-2xl mx-auto my-6 bg-surface shadow-xl relative overflow-hidden"
    >
      <!-- Subtle Radial Glow -->
      <div class="absolute -inset-10 bg-[radial-gradient(circle,rgba(235,225,200,0.04)_0%,transparent_70%)] pointer-events-none" />

      <!-- Sentry Chronometer Centerpiece -->
      <div class="py-2 flex justify-center relative z-10">
        <SentryChronometer size="md" @marker-click="emit('selectServer')" />
      </div>

      <div class="space-y-1.5 relative z-10">
        <h3 class="text-base font-bold font-sans tracking-wide text-primary uppercase">{{ $t('dashboard.idleStatus') }}</h3>
        <p class="text-xs text-primary/60 max-w-md mx-auto leading-relaxed font-sans">
          {{ $t('dashboard.idleDesc') }}
        </p>
      </div>

      <div class="pt-1 flex items-center justify-center space-x-3 relative z-10">
        <BaseButton
          variant="tactical"
          size="md"
          @click="emit('selectServer')"
        >
          <Flame class="w-4 h-4 mr-2 text-squad-orange" />
          <span>{{ $t('fleet.title') }}</span>
        </BaseButton>
      </div>
    </BaseCard>
  </div>
</template>
