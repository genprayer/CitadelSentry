<script setup lang="ts">
import { LogIn, Users, MapPin, Signal, Info, Flame, Square } from 'lucide-vue-next'
import type { Server, SeedState } from '~/types'
import { useI18n } from '~/composables/useI18n'
import BaseCard from '~/components/ui/BaseCard.vue'
import BaseBadge from '~/components/ui/BaseBadge.vue'
import BaseButton from '~/components/ui/BaseButton.vue'
import BaseProgress from '~/components/ui/BaseProgress.vue'
import { isSeedableServerStatus } from '~/constants/seed'

interface Props {
  server: Server
  canSeed?: boolean
  isSeedingThisServer?: boolean
  isQueued?: boolean
  queueOrderNumber?: number
  isAnySeedingActive?: boolean
  missionState?: SeedState
  isStopping?: boolean
  missionTarget?: number
}

const props = withDefaults(defineProps<Props>(), {
  canSeed: true,
  isSeedingThisServer: false,
  isQueued: false,
  queueOrderNumber: 0,
  isAnySeedingActive: false,
  missionState: 'idle',
  isStopping: false,
  missionTarget: 0
})

const { t } = useI18n()
const displayTarget = computed(() => {
  if ((props.isSeedingThisServer || props.isQueued) && props.missionTarget && props.missionTarget > 0) {
    return props.missionTarget
  }
  return props.server.targetSeedCount > 0 ? props.server.targetSeedCount : t('common.unknown')
})
const missionLabel = computed(() => {
  switch (props.missionState) {
    case 'active': return t('dashboard.stateActive')
    case 'preparing': return t('dashboard.statePreparing')
    case 'transitioning': return t('dashboard.stateTransitioning')
    default: return t('dashboard.stateConnecting')
  }
})

const emit = defineEmits<{
  seed: [server: Server]
  join: [server: Server]
  stop: [server: Server]
  details: [server: Server]
}>()

const statusBadgeVariant = computed(() => {
  if (props.server.isSeedingActive) return 'warning'
  if (props.server.status === 'online') return 'success'
  if (props.server.status === 'full') return 'info'
  if (props.server.status === 'offline') return 'danger'
  return 'neutral'
})

const playerProgressPercent = computed(() => {
  if (!props.server.maxPlayers) return 0
  return Math.round((props.server.currentPlayers / props.server.maxPlayers) * 100)
})

const isSeedable = computed(() => isSeedableServerStatus(props.server.status))
</script>

<template>
  <BaseCard
    variant="panel"
    padding="md"
    :class="[
      'hover:border-accent/40 transition-colors flex flex-col justify-between space-y-4 group relative overflow-hidden bg-surface',
      isSeedingThisServer ? 'border-accent ring-1 ring-accent/30' : 'border-accent-secondary/15'
    ]"
  >
    <!-- Top: Name & Badges -->
    <div class="space-y-2">
      <div class="space-y-1.5">
        <h3 class="text-base font-bold text-primary font-sans group-hover:text-accent transition-colors">
          {{ server.name }}
        </h3>
        <div class="flex items-center space-x-1.5 flex-wrap gap-y-1">
          <BaseBadge v-if="isSeedingThisServer" :variant="missionState === 'active' ? 'success' : 'warning'" size="sm" dot class="animate-pulse">
            {{ missionLabel }}
          </BaseBadge>
          <BaseBadge v-else-if="isQueued" variant="info" size="sm">
            В ОЧЕРЕДИ #{{ queueOrderNumber }}
          </BaseBadge>
          <BaseBadge v-else-if="server.isSeedingActive" variant="warning" size="sm" dot>
            {{ $t('fleet.statusSeeding') }}
          </BaseBadge>
          <BaseBadge :variant="statusBadgeVariant" size="sm">
            {{ server.status.toUpperCase() }}
          </BaseBadge>
        </div>
      </div>

      <!-- Map & Layer Info -->
      <div class="flex items-center space-x-2 text-sm text-primary/70 font-mono">
        <MapPin class="w-4 h-4 text-accent/70 flex-shrink-0" />
        <span class="truncate">{{ server.currentMap }}</span>
        <span>•</span>
        <span class="text-primary/50 truncate">{{ server.currentLayer }}</span>
      </div>
    </div>

    <!-- Middle: Player Capacity & Queue Stats -->
    <div class="space-y-2 bg-surface/80 p-3 rounded-xl border border-accent-secondary/15">
      <div class="flex justify-between items-center text-sm font-mono">
        <div class="flex items-center space-x-1.5 text-primary">
          <Users class="w-4 h-4 text-accent" />
          <span class="font-bold text-primary text-base">{{ server.currentPlayers }}</span>
          <span class="text-primary/50">/ {{ server.maxPlayers }}</span>
        </div>

        <!-- Queue indicator -->
        <div v-if="server.publicQueue > 0" class="text-squad-orange font-semibold text-xs">
          +{{ server.publicQueue }} {{ $t('dashboard.queue') }}
        </div>
        <div v-else class="text-primary/40 text-xs">
          No queue
        </div>
      </div>

      <!-- Capacity Progress Bar -->
      <BaseProgress
        :value="playerProgressPercent"
        :show-value="false"
        :variant="server.isSeedingActive ? 'warning' : 'brand'"
      />

      <div class="flex items-center justify-between text-xs font-mono text-primary/70 pt-0.5">
        <span>Target: <strong class="text-primary font-bold">{{ displayTarget }}</strong></span>
        <div v-if="server.pingMs" class="flex items-center space-x-1 text-primary/70">
          <Signal class="w-3.5 h-3.5 text-accent" />
          <span>{{ server.pingMs }}ms</span>
        </div>
      </div>
    </div>

    <!-- Actions Row -->
    <div class="flex items-center justify-between gap-2 pt-1.5 border-t border-accent-secondary/15">
      <BaseButton
        variant="ghost"
        size="sm"
        class="text-xs"
        @click="emit('details', server)"
      >
        <Info class="w-3.5 h-3.5 mr-1 text-primary/50" />
        <span>Info</span>
      </BaseButton>

      <div class="flex items-center space-x-2">
        <BaseButton
          variant="secondary"
          size="sm"
          :title="isAnySeedingActive ? 'Прямое подключение к серверу через Steam (остановит активный сидинг)' : 'Прямое подключение к серверу через Steam'"
          @click="emit('join', server)"
        >
          <LogIn class="w-3.5 h-3.5 mr-1" />
          <span>{{ $t('fleet.join') }}</span>
        </BaseButton>

        <BaseButton
          v-if="isSeedingThisServer"
          variant="danger"
          size="sm"
          title="Остановить текущий сидинг"
          :loading="isStopping"
          @click="emit('stop', server)"
        >
          <Square class="w-3.5 h-3.5 mr-1" />
          <span>{{ $t('common.cancel') }}</span>
        </BaseButton>

        <BaseButton
          v-else
          variant="tactical"
          size="sm"
          :disabled="!canSeed || !isSeedable"
          :title="isQueued ? 'Сервер находится в очереди сидинга' : (isAnySeedingActive ? 'Запущен сидинг другого сервера' : 'Настроить и запустить сидинг')"
          @click="emit('seed', server)"
        >
          <Flame class="w-3.5 h-3.5 mr-1" />
          <span>{{ isQueued ? `#${queueOrderNumber} в очереди` : $t('fleet.seed') }}</span>
        </BaseButton>
      </div>
    </div>
  </BaseCard>
</template>
