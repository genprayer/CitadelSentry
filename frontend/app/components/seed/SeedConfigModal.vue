<script setup lang="ts">
import { Play, Zap, ShieldAlert, Copy, Check } from 'lucide-vue-next'
import type { Server, SeedPlan, PostSeedAction, TimeoutAction, PotatoSettings } from '~/types'
import {
  SEED_TARGET_PLAYER_PRESETS,
  SEED_DURATION_PRESETS,
  POST_SEED_ACTION_OPTIONS,
  TIMEOUT_ACTION_OPTIONS,
  isSeedableServerStatus
} from '~/constants/seed'
import { useSeed } from '~/composables/useSeed'
import { useSettings } from '~/composables/useSettings'
import { useServers } from '~/composables/useServers'
import { useI18n } from '~/composables/useI18n'
import { useBridge } from '~/composables/useBridge'
import BaseModal from '~/components/ui/BaseModal.vue'
import BaseButton from '~/components/ui/BaseButton.vue'
import BaseInput from '~/components/ui/BaseInput.vue'
import BaseSelect from '~/components/ui/BaseSelect.vue'
import BaseToggle from '~/components/ui/BaseToggle.vue'
import SeedPolicySummary from '~/components/seed/SeedPolicySummary.vue'

interface Props {
  modelValue: boolean
  server: Server | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  started: [plan: SeedPlan]
}>()

const { startSeed, checkRunningSquadPotatoState, restartSquadWithPotato, isLoading, isSessionStatusKnown, sessionStatusError } = useSeed()
const { settings, saveSettings, isLoaded, isSaving, error: settingsError } = useSettings()
const { servers } = useServers()
const { isSquadRunning } = useBridge()
const { t } = useI18n()

const runningPotatoState = ref<{ isRunning: boolean; matchesPotato: boolean; currentFps: number } | null>(null)
const allowContinueWithRunning = ref(false)
const isRestartingSquad = ref(false)
const showPotatoWarningModal = ref(false)
const isCopiedConsole = ref(false)
let copyConsoleTimeout: any = null

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

const copyConsoleCommand = async (id?: string) => {
  const port = getCitadelPort(id)
  const cmd = `connect 62.122.215.132:${port}`
  try {
    if (typeof navigator !== 'undefined' && navigator.clipboard) {
      await navigator.clipboard.writeText(cmd)
    }
    isCopiedConsole.value = true
    if (copyConsoleTimeout) clearTimeout(copyConsoleTimeout)
    copyConsoleTimeout = setTimeout(() => {
      isCopiedConsole.value = false
    }, 2500)
  } catch (err) {
    console.warn('Failed to copy console command', err)
  }
}

const currentDetectedFps = computed(() => {
  if (runningPotatoState.value?.currentFps) {
    return runningPotatoState.value.currentFps
  }
  if (isPotatoEnabled.value) {
    return potatoFps.value || 15
  }
  return '120+ (Стандартный)'
})

const currentDetectedResolution = computed(() => {
  if (isPotatoEnabled.value && isLowResEnabled.value) {
    return '640×480 (Окно картошки)'
  }
  return 'Исходное (Полный экран)'
})

const onLaunchButtonClick = () => {
  if (startBlockedReason.value) return
  if (!isPotatoEnabled.value || (isSquadRunning.value === true && !runningPotatoState.value?.matchesPotato && !allowContinueWithRunning.value)) {
    showPotatoWarningModal.value = true
    return
  }
  handleStart()
}

const handleConfirmCurrentSettings = () => {
  allowContinueWithRunning.value = true
  showPotatoWarningModal.value = false
  handleStart()
}

const handleApplyPotatoAndStart = async () => {
  showPotatoWarningModal.value = false
  if (!isPotatoEnabled.value) {
    await updatePotatoSettings({ enabled: true })
  }
  if (isSquadRunning.value === true) {
    await handleRestartSquad()
  } else {
    handleStart()
  }
}

const refreshRunningSquadState = async () => {
  if (isSquadRunning.value) {
    try {
      runningPotatoState.value = await checkRunningSquadPotatoState()
    } catch {
      runningPotatoState.value = null
    }
  } else {
    runningPotatoState.value = null
    allowContinueWithRunning.value = false
  }
}

watch([() => props.modelValue, isSquadRunning], () => {
  if (props.modelValue) {
    refreshRunningSquadState()
  }
}, { immediate: true })

const targetCount = ref<number>(settings.value.seed.defaultTargetPlayers)
const maxMinutes = ref<number>(settings.value.seed.defaultMaxDurationMinutes)
const selectedEndTime = ref<Date | null>(null)
const durationInput = computed({
  get: () => maxMinutes.value,
  set: (value: number) => {
    selectedEndTime.value = null
    maxMinutes.value = value
  }
})
const selectedServerIds = ref<string[]>([])
const postSeedAction = ref<PostSeedAction>('do_nothing')
const timeoutAction = ref<TimeoutAction>('abort')
const formError = ref<string | null>(null)

const availableServers = computed(() => {
  return servers.value.filter(s => isSeedableServerStatus(s.status))
})

const isSingleServer = computed(() => selectedServerIds.value.length <= 1)

const areTargetsAlreadyMet = computed(() => {
  const selected = servers.value.filter(s => selectedServerIds.value.includes(s.id))
  return selected.length > 0 && selected.every(server => server.currentPlayers >= Number(targetCount.value))
})

const isDurationTooShort = computed(() => {
  return Number(maxMinutes.value) < 5
})

const isPotatoEnabled = computed(() => {
  return !!settings.value.potato?.enabled
})

const startBlockedReason = computed(() => {
  if (!isSessionStatusKnown.value) return sessionStatusError.value || t('dashboard.sessionUnavailable')
  if (!isLoaded.value) return settingsError.value || t('modal.settingsUnavailable')
  if (isSaving.value) return t('modal.settingsSaving')
  if (isPotatoEnabled.value && isSquadRunning.value === true) {
    if (!runningPotatoState.value?.matchesPotato && !allowContinueWithRunning.value) {
      return t('modal.potatoStartBlocked')
    }
  } else if (isPotatoEnabled.value && isSquadRunning.value === null) {
    return t('potato.processUnknownNotice')
  }
  if (selectedServerIds.value.length === 0) return t('modal.noServersSelected')
  if (isDurationTooShort.value) return t('modal.minDurationWarning')
  return null
})

const fpsPresets = [1, 5, 10, 15, 30]

const potatoFps = computed(() => {
  return settings.value.potato?.fpsLimit
})

const updatePotatoSettings = async (changes: Partial<PotatoSettings>) => {
  if (!isLoaded.value || isSaving.value || !settings.value.potato) {
    formError.value = settingsError.value || t('modal.settingsUnavailable')
    return
  }
  try {
    await saveSettings({ ...settings.value, potato: { ...settings.value.potato, ...changes } })
  } catch (error) {
    formError.value = error instanceof Error ? error.message : String(error)
  }
}

const updatePotatoFps = (fps: number) => updatePotatoSettings({ fpsLimit: fps })
const togglePotato = (isEnabled: boolean) => updatePotatoSettings({ enabled: isEnabled })
const isLowResEnabled = computed(() => !!settings.value.potato?.lowResolution)
const toggleLowRes = (isEnabled: boolean) => updatePotatoSettings({ lowResolution: isEnabled })

const postSeedOptions = computed(() => POST_SEED_ACTION_OPTIONS.map(option => ({
  label: t(option.labelKey),
  value: option.value
})))

const timeoutOptions = computed(() => TIMEOUT_ACTION_OPTIONS.map(option => {
  const isContinueNext = option.value === 'continue_next'
  const disabled = isContinueNext && isSingleServer.value
  return {
    label: disabled ? `${t(option.labelKey)} (${t('modal.timeoutOnlyMulti')})` : t(option.labelKey),
    value: option.value,
    disabled
  }
}))

watch(isSingleServer, (single) => {
  if (single && timeoutAction.value === 'continue_next') {
    timeoutAction.value = 'abort'
  }
})

const policyWarningKeys = computed(() => {
  const warnings: string[] = []
  if (selectedServerIds.value.length > 1 && ['sleep', 'shutdown'].includes(postSeedAction.value)) {
    warnings.push('modal.postActionQueueWarning')
  }
  return warnings
})

const queuePreview = computed(() => {
  return selectedServerIds.value
    .map(id => servers.value.find(s => s.id === id))
    .filter((s): s is Server => !!s)
})

const getServerQueueIndex = (serverId: string): number => {
  return selectedServerIds.value.indexOf(serverId) + 1
}

const toggleServer = (serverId: string) => {
  const idx = selectedServerIds.value.indexOf(serverId)
  if (idx >= 0) {
    selectedServerIds.value.splice(idx, 1)
  } else {
    selectedServerIds.value.push(serverId)
  }
}

const selectOnlyCurrent = () => {
  if (props.server) {
    selectedServerIds.value = [props.server.id]
  }
}

const selectAll = () => {
  const allIds = availableServers.value.map(s => s.id)
  if (props.server && allIds.includes(props.server.id)) {
    selectedServerIds.value = [props.server.id, ...allIds.filter(id => id !== props.server?.id)]
  } else {
    selectedServerIds.value = allIds
  }
}

const localEndTime = computed({
  get: () => {
    if (!selectedEndTime.value && (!Number.isSafeInteger(maxMinutes.value) || maxMinutes.value <= 0)) return ''
    const end = selectedEndTime.value ?? new Date(Date.now() + maxMinutes.value * 60_000)
    return `${String(end.getHours()).padStart(2, '0')}:${String(end.getMinutes()).padStart(2, '0')}`
  },
  set: (value: string | number) => {
    const parts = String(value).split(':')
    if (parts.length !== 2) return
    const hours = Number(parts[0])
    const minutes = Number(parts[1])
    if (!Number.isInteger(hours) || !Number.isInteger(minutes)) return
    const now = new Date()
    const end = new Date(now)
    end.setHours(hours, minutes, 0, 0)
    if (end.getTime() <= now.getTime()) end.setDate(end.getDate() + 1)
    selectedEndTime.value = end
    maxMinutes.value = Math.ceil((end.getTime() - now.getTime()) / 60_000)
  }
})

watch(
  [() => props.server, isLoaded],
  ([srv, hasLoadedSettings]) => {
    if (srv && hasLoadedSettings) {
      targetCount.value = settings.value.seed.defaultTargetPlayers
      durationInput.value = settings.value.seed.defaultMaxDurationMinutes
      selectedServerIds.value = [srv.id]
      postSeedAction.value = settings.value.seed.defaultPostSeedAction as PostSeedAction
      timeoutAction.value = 'abort'
      formError.value = null
    }
  },
  { immediate: true }
)

const handleStart = async () => {
  if (!props.server) return
  if (!isSessionStatusKnown.value) {
    formError.value = sessionStatusError.value || t('dashboard.sessionUnavailable')
    return
  }
  if (!isLoaded.value || isSaving.value) {
    formError.value = settingsError.value || t('modal.settingsUnavailable')
    return
  }
  if (isPotatoEnabled.value && isSquadRunning.value !== false) {
    if (isSquadRunning.value === null) {
      formError.value = t('potato.processUnknownNotice')
      return
    }
    if (!runningPotatoState.value?.matchesPotato && !allowContinueWithRunning.value) {
      formError.value = t('potato.squadRunningNotice')
      return
    }
  }
  if (selectedServerIds.value.length === 0) {
    formError.value = t('modal.noServersSelected')
    return
  }
  if (selectedEndTime.value) {
    maxMinutes.value = Math.ceil((selectedEndTime.value.getTime() - Date.now()) / 60_000)
  }
  if (isDurationTooShort.value) {
    formError.value = t('modal.minDurationWarning')
    return
  }
  formError.value = null

  const primaryServer = servers.value.find(s => s.id === selectedServerIds.value[0])
  if (!primaryServer || selectedServerIds.value.some(id => !servers.value.some(server => server.id === id && isSeedableServerStatus(server.status)))) {
    formError.value = t('modal.selectedServerUnavailable')
    return
  }

  const plan: SeedPlan = {
    serverId: primaryServer.id,
    serverIds: [...selectedServerIds.value],
    serverName: primaryServer.name,
    serverIp: primaryServer.serverIp,
    gamePort: primaryServer.gamePort,
    queryPort: primaryServer.queryPort,
    targetPlayerCount: Number(targetCount.value),
    maxDurationMinutes: Number(maxMinutes.value),
    serverLimit: selectedServerIds.value.length,
    postSeedAction: postSeedAction.value,
    timeoutAction: timeoutAction.value,
    autoRecoverCrash: true,
    forceRunningSettings: allowContinueWithRunning.value
  }

  try {
    await startSeed(plan, primaryServer)
    emit('started', plan)
    emit('update:modelValue', false)
  } catch (err: any) {
    formError.value = err.message || t('common.error')
  }
}

const handleRestartSquad = async () => {
  if (!props.server) return
  isRestartingSquad.value = true
  formError.value = null
  const primaryServer = servers.value.find(s => s.id === selectedServerIds.value[0])
  if (!primaryServer) {
    isRestartingSquad.value = false
    return
  }
  const plan: SeedPlan = {
    serverId: primaryServer.id,
    serverIds: [...selectedServerIds.value],
    serverName: primaryServer.name,
    serverIp: primaryServer.serverIp,
    gamePort: primaryServer.gamePort,
    queryPort: primaryServer.queryPort,
    targetPlayerCount: Number(targetCount.value),
    maxDurationMinutes: Number(maxMinutes.value),
    serverLimit: selectedServerIds.value.length,
    postSeedAction: postSeedAction.value,
    timeoutAction: timeoutAction.value,
    autoRecoverCrash: true
  }
  try {
    await restartSquadWithPotato(plan, primaryServer)
    emit('started', plan)
    emit('update:modelValue', false)
  } catch (err: any) {
    formError.value = err.message || t('common.error')
  } finally {
    isRestartingSquad.value = false
  }
}
</script>

<template>
  <BaseModal
    :model-value="modelValue"
    :title="$t('modal.configTitle')"
    :subtitle="server?.name || 'Target Server'"
    max-width="lg"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div v-if="server" class="space-y-5 font-sans">
      <!-- Server Summary -->
      <div class="p-3 bg-surface rounded-lg border border-accent-secondary/20 flex items-center justify-between text-xs font-mono">
        <div>
          <p class="text-primary/60 uppercase">{{ $t('modal.currentMap') }}:</p>
          <p class="text-primary font-semibold mt-0.5">{{ server.currentMap }} ({{ server.currentLayer }})</p>
        </div>
        <div class="text-right">
          <p class="text-primary/60 uppercase">{{ $t('modal.currentPlayers') }}:</p>
          <p class="text-accent font-semibold mt-0.5">{{ server.currentPlayers }} / {{ server.maxPlayers }}</p>
        </div>
      </div>

      <!-- Direct Steam Connection Note for Empty Servers -->
      <div v-if="server" class="p-3 bg-surface border border-accent-secondary/20 rounded-xl space-y-1.5 text-xs">
        <div class="flex items-center justify-between">
          <span class="text-accent font-semibold font-sans flex items-center gap-1.5">
            <Zap class="w-3.5 h-3.5 text-accent" />
            <span>{{ $t('settings.directConnectTitle') || 'Прямое подключение к серверу' }}</span>
          </span>
          <span v-if="server.currentPlayers === 0" class="text-[10px] text-emerald-400 font-mono font-semibold uppercase px-1.5 py-0.5 rounded bg-emerald-500/10 border border-emerald-500/30">
            0/100 • Авто-сидинг готов
          </span>
        </div>
        <p class="text-[11px] text-primary/70 leading-normal">
          Подключение к 62.122.215.132:{{ getCitadelPort(server.id) }} выполняется полностью автоматически через протокол Steam.
        </p>
      </div>

      <!-- Error -->
      <div v-if="formError || settingsError" class="p-3 bg-rose-500/10 border border-rose-500/30 rounded-lg text-xs text-rose-300">
        {{ formError || settingsError }}
      </div>

      <!-- Target Players -->
      <div class="space-y-2">
        <label class="block text-xs font-medium text-primary flex items-center justify-between">
          <span>{{ $t('modal.targetThreshold') }}</span>
          <span class="text-accent font-mono font-bold">{{ targetCount }} {{ $t('common.players') }}</span>
        </label>
        <div class="flex items-center space-x-2">
          <button
            v-for="preset in SEED_TARGET_PLAYER_PRESETS"
            :key="preset"
            type="button"
            :class="[
              'flex-1 py-1.5 rounded text-xs font-mono border transition-colors',
              targetCount === preset
                ? 'bg-accent/20 text-accent border-accent/50 font-bold'
                : 'bg-surface text-primary/60 border-accent-secondary/20 hover:text-primary'
            ]"
            @click="targetCount = preset"
          >
            {{ preset }}
          </button>
        </div>
        <BaseInput v-model="targetCount" type="number" :hint="$t('modal.customTargetHint')" />
        <p v-if="areTargetsAlreadyMet" class="text-xs text-amber-400 font-mono mt-1 flex items-center space-x-1.5">
          <ShieldAlert class="w-3.5 h-3.5 shrink-0" />
          <span>{{ $t('modal.targetsAlreadyMet') }}</span>
        </p>
      </div>

      <!-- Max Duration -->
      <div class="space-y-2">
        <label class="block text-xs font-medium text-primary flex items-center justify-between">
          <span>{{ $t('modal.maxTimeout') }}</span>
          <span class="text-squad-orange font-mono font-bold">{{ maxMinutes }} {{ $t('common.minutes') }}</span>
        </label>
        <div class="flex items-center space-x-2">
          <button
            v-for="dur in SEED_DURATION_PRESETS"
            :key="dur"
            type="button"
            :class="[
              'flex-1 py-1.5 rounded text-xs font-mono border transition-colors',
              maxMinutes === dur
                ? 'bg-accent/20 text-accent border-accent/50 font-bold'
                : 'bg-surface text-primary/60 border-accent-secondary/20 hover:text-primary'
            ]"
            @click="durationInput = dur"
          >
            {{ dur }}m
          </button>
        </div>
        <BaseInput v-model="durationInput" type="number" min="5" :hint="$t('modal.customTimeoutHint')" />
        <p v-if="isDurationTooShort" class="text-xs text-rose-400 font-mono mt-1 flex items-center space-x-1.5">
          <ShieldAlert class="w-3.5 h-3.5 shrink-0" />
          <span>{{ $t('modal.minDurationWarning') }}</span>
        </p>
        <BaseInput
          v-model="localEndTime"
          type="time"
          :label="$t('modal.localEndTime')"
          :hint="$t('modal.localEndTimeHint')"
        />
      </div>

      <!-- Server Selection Checkboxes -->
      <div class="space-y-2">
        <div class="flex items-center justify-between">
          <div>
            <label class="block text-xs font-semibold text-primary">
              {{ $t('modal.serverSelection') }}
            </label>
            <p class="text-[11px] text-primary/60 mt-0.5">
              {{ $t('modal.serverSelectionHint') }}
            </p>
          </div>
          <div class="flex items-center space-x-1.5 text-xs font-mono">
            <button
              type="button"
              class="px-2 py-1 rounded bg-surface border border-accent-secondary/20 hover:text-accent text-primary/70 transition-colors"
              @click="selectOnlyCurrent"
            >
              {{ $t('modal.selectOnlyCurrent') }}
            </button>
            <button
              type="button"
              class="px-2 py-1 rounded bg-surface border border-accent-secondary/20 hover:text-accent text-primary/70 transition-colors"
              @click="selectAll"
            >
              {{ $t('modal.selectAll') }}
            </button>
          </div>
        </div>

        <!-- Low Resolution (640x480) Toggle -->
        <div v-if="isPotatoEnabled" class="pt-2 border-t border-accent-secondary/10 flex items-center justify-between animate-in fade-in duration-150">
          <div>
            <span class="text-xs font-semibold text-primary font-sans">{{ $t('potato.lowResolution') }}</span>
            <p class="text-[11px] text-primary/60">640x480 ({{ $t('potato.lowResolutionDesc') }})</p>
          </div>
          <BaseToggle
            :model-value="isLowResEnabled"
            @update:model-value="toggleLowRes"
            :disabled="!isLoaded || isSaving"
          />
        </div>

        <div class="space-y-1.5 max-h-48 overflow-y-auto pr-1 select-none">
          <div
            v-for="srv in availableServers"
            :key="srv.id"
            :class="[
              'flex items-center justify-between p-2.5 rounded-lg border transition-all cursor-pointer text-xs',
              selectedServerIds.includes(srv.id)
                ? 'bg-accent/15 border-accent/60 text-primary shadow-sm'
                : 'bg-surface/70 border-accent-secondary/15 text-primary/70 hover:border-accent-secondary/40'
            ]"
            @click="toggleServer(srv.id)"
          >
            <div class="flex items-center space-x-2.5 min-w-0">
              <input
                type="checkbox"
                :checked="selectedServerIds.includes(srv.id)"
                class="w-4 h-4 rounded border-accent/40 text-accent bg-surface focus:ring-accent/30 cursor-pointer pointer-events-none"
                tabindex="-1"
              />
              <span
                v-if="getServerQueueIndex(srv.id) > 0"
                class="px-1.5 py-0.5 rounded text-[10px] font-mono font-bold bg-accent text-black flex-shrink-0"
              >
                #{{ getServerQueueIndex(srv.id) }}
              </span>
              <div class="min-w-0">
                <p class="font-bold truncate text-primary">{{ srv.name }}</p>
                <p class="text-[11px] text-primary/50 truncate font-mono">{{ srv.currentMap }} ({{ srv.currentLayer }})</p>
              </div>
            </div>

            <div class="text-right flex-shrink-0 font-mono ml-2">
              <span class="text-accent font-bold">{{ srv.currentPlayers }}</span>
              <span class="text-primary/40">/{{ srv.maxPlayers }}</span>
            </div>
          </div>
        </div>

        <!-- Low Resolution (640x480) Toggle -->
        <div v-if="isPotatoEnabled" class="pt-2 border-t border-accent-secondary/10 flex items-center justify-between animate-in fade-in duration-150">
          <div>
            <span class="text-xs font-semibold text-primary font-sans">{{ $t('potato.lowResolution') }}</span>
            <p class="text-[11px] text-primary/60">640x480 ({{ $t('potato.lowResolutionDesc') }})</p>
          </div>
          <BaseToggle
            :model-value="isLowResEnabled"
            @update:model-value="toggleLowRes"
            :disabled="!isLoaded || isSaving"
          />
        </div>

        <!-- Queue order preview string -->
        <div v-if="queuePreview.length > 0" class="p-2 rounded bg-surface/50 border border-accent-secondary/10 text-[11px] text-primary/70 font-mono flex items-center space-x-1.5 overflow-x-auto">
          <span class="font-bold text-accent whitespace-nowrap">{{ $t('modal.queueOrder') }}:</span>
          <span class="truncate">{{ queuePreview.map((s, idx) => `#${idx + 1} ${s.name}`).join(' → ') }}</span>
        </div>
      </div>

      <!-- Quick Potato Mode Toggle -->
      <div class="p-3 bg-surface rounded-lg border border-accent-secondary/20 space-y-2.5">
        <div class="flex items-center justify-between">
          <div class="flex items-center space-x-2.5">
            <div class="w-7 h-7 rounded bg-accent/10 border border-accent/30 flex items-center justify-center text-accent">
              <Zap class="w-3.5 h-3.5" />
            </div>
            <div>
              <p class="text-xs font-semibold text-primary">
                {{ $t('potato.quickToggleTitle', { fps: potatoFps ?? $t('settings.unknown') }) }}
              </p>
              <p class="text-[11px] text-primary/60">{{ $t('potato.quickToggleDesc') }}</p>
            </div>
          </div>
          <BaseToggle
            :model-value="isPotatoEnabled"
            @update:model-value="togglePotato"
            :disabled="!isLoaded || isSaving"
          />
        </div>

        <!-- Inline FPS Presets when Potato Mode is Enabled -->
        <div v-if="isPotatoEnabled" class="pt-2 border-t border-accent-secondary/10 flex items-center space-x-2 animate-in fade-in duration-150">
          <span class="text-[11px] font-mono text-primary/50 whitespace-nowrap">{{ $t('potato.fpsLimit') }}:</span>
          <div class="flex items-center space-x-1.5 flex-1">
            <button
              v-for="fps in fpsPresets"
              :key="fps"
              :disabled="!isLoaded || isSaving"
              type="button"
              :class="[
                'flex-1 py-1 rounded text-xs font-mono border transition-colors',
                potatoFps === fps
                  ? 'bg-accent/20 text-accent border-accent/50 font-bold'
                  : 'bg-surface/80 text-primary/60 border-accent-secondary/20 hover:text-primary'
              ]"
              @click="updatePotatoFps(fps)"
            >
              {{ fps }}
            </button>
          </div>
        </div>

        <!-- Low Resolution (640x480) Toggle -->
        <div v-if="isPotatoEnabled" class="pt-2 border-t border-accent-secondary/10 flex items-center justify-between animate-in fade-in duration-150">
          <div>
            <span class="text-xs font-semibold text-primary font-sans">{{ $t('potato.lowResolution') }}</span>
            <p class="text-[11px] text-primary/60">640x480 ({{ $t('potato.lowResolutionDesc') }})</p>
          </div>
          <BaseToggle
            :model-value="isLowResEnabled"
            @update:model-value="toggleLowRes"
            :disabled="!isLoaded || isSaving"
          />
        </div>

        <!-- Potato settings are loaded only on a new Squad process. -->
        <div
          v-if="isPotatoEnabled && isSquadRunning !== false"
          class="p-2.5 rounded-lg bg-amber-500/10 border border-amber-500/20 text-[11px] text-amber-200/90 flex items-start space-x-2 animate-in fade-in duration-150"
        >
          <ShieldAlert class="w-3.5 h-3.5 text-amber-400 shrink-0 mt-0.5" />
          <span>{{ $t(isSquadRunning === true ? 'potato.squadRunningNotice' : 'potato.processUnknownNotice') }}</span>
        </div>
      </div>

      <!-- Post-Seed & Timeout Policies -->
      <div class="grid grid-cols-1 gap-4 pt-1">
        <BaseSelect
          v-model="timeoutAction"
          :label="$t('modal.timeoutAction')"
          :hint="$t('modal.timeoutActionHint')"
          :options="timeoutOptions"
        />
        <BaseSelect
          v-model="postSeedAction"
          :label="$t('modal.targetAction')"
          :hint="$t('modal.targetActionHint')"
          :options="postSeedOptions"
        />
      </div>

      <SeedPolicySummary
        :server-count="selectedServerIds.length"
        :target-players="Number(targetCount)"
        :max-duration-minutes="Number(maxMinutes)"
        :post-seed-action="postSeedAction"
        :timeout-action="timeoutAction"
      />

      <!-- Power Management Safety Alert -->
      <div
        v-if="['sleep', 'shutdown'].includes(postSeedAction) || ['sleep', 'shutdown'].includes(timeoutAction)"
        class="p-3 rounded-lg bg-amber-500/10 border border-amber-500/30 text-xs text-amber-200 flex items-start space-x-2"
        role="status"
      >
        <ShieldAlert class="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
        <span class="leading-relaxed">{{ $t('modal.powerSafetyNotice') }}</span>
      </div>

      <div
        v-for="warningKey in policyWarningKeys"
        :key="warningKey"
        class="p-3 rounded-lg bg-amber-500/10 border border-amber-500/30 text-xs text-amber-200"
        role="status"
      >
        {{ $t(warningKey) }}
      </div>

    </div>

    <template #footer>
      <div class="w-full space-y-3">
        <!-- Running Squad Potato Choice -->
        <div
          v-if="isPotatoEnabled && isSquadRunning === true && !runningPotatoState?.matchesPotato && !allowContinueWithRunning"
          class="p-3 bg-amber-500/10 border border-amber-500/30 rounded-xl space-y-2 text-xs text-amber-200"
        >
          <div class="flex items-center space-x-2 font-medium">
            <ShieldAlert class="w-4 h-4 text-amber-400 shrink-0" />
            <span>Squad уже запущен с другими настройками графики. Unreal Engine считывает конфиг только при старте.</span>
          </div>
          <div class="flex items-center justify-end space-x-2 pt-1">
            <BaseButton variant="secondary" size="sm" @click="allowContinueWithRunning = true">
              Продолжить без картошки
            </BaseButton>
            <BaseButton variant="tactical" size="sm" :loading="isRestartingSquad" @click="handleRestartSquad">
              Перезапустить с картошкой
            </BaseButton>
          </div>
        </div>

        <div
          v-else-if="isPotatoEnabled && isSquadRunning === true && runningPotatoState?.matchesPotato"
          class="p-2.5 bg-emerald-500/10 border border-emerald-500/30 rounded-lg text-xs text-emerald-300 flex items-center space-x-2"
        >
          <Zap class="w-4 h-4 text-emerald-400 shrink-0" />
          <span>Squad уже запущен в режиме картошки ({{ runningPotatoState.currentFps }} FPS). Перезапуск не требуется!</span>
        </div>

        <p v-if="startBlockedReason" id="seed-start-blocked-reason" role="status" class="text-xs text-amber-200">
          {{ startBlockedReason }}
        </p>
        <div class="flex items-center justify-end space-x-3">
          <BaseButton variant="ghost" size="sm" @click="emit('update:modelValue', false)">
            {{ $t('common.cancel') }}
          </BaseButton>
          <BaseButton
            variant="tactical"
            size="md"
            :loading="isLoading"
            :disabled="!!startBlockedReason"
            :aria-describedby="startBlockedReason ? 'seed-start-blocked-reason' : undefined"
            :title="startBlockedReason || undefined"
            @click="onLaunchButtonClick"
          >
            <Play class="w-4 h-4 mr-1.5" />
            <span>{{ $t('modal.launchMission') }}</span>
          </BaseButton>
        </div>
      </div>
    </template>
  </BaseModal>

  <!-- Explicit Warning Modal when Potato Mode is not active -->
  <BaseModal
    v-model="showPotatoWarningModal"
    title="Предупреждение о настройках Squad"
    subtitle="Проверка параметров графики перед сидингом"
    max-width="md"
    :close-on-backdrop="false"
  >
    <div class="space-y-4 text-xs font-sans">
      <div class="p-3 bg-amber-500/10 border border-amber-500/30 rounded-xl flex items-start gap-3">
        <ShieldAlert class="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />
        <div class="space-y-1">
          <p class="font-bold text-amber-300">
            {{ isSquadRunning ? 'Squad уже запущен со стандартной графикой' : 'Режим Картошки выключен' }}
          </p>
          <p class="text-primary/70 leading-relaxed text-[11px]">
            {{ isSquadRunning
              ? 'Игра сейчас открыта с обычными игровыми настройками. Сидинг в таком режиме будет нагружать видеокарту и занимать ресурсы ПК.'
              : 'Вы запускаете сидинг без оптимизации Режима Картошки. Squad будет потреблять стандартный объем памяти и ресурсов GPU.' }}
          </p>
        </div>
      </div>

      <!-- Current Detected Settings -->
      <div class="p-3 bg-surface border border-accent-secondary/20 rounded-xl space-y-2 font-mono text-[11px]">
        <div class="flex justify-between items-center pb-1.5 border-b border-accent-secondary/15">
          <span class="text-primary/50 uppercase">Лимит FPS:</span>
          <span class="text-accent font-bold">{{ currentDetectedFps }}</span>
        </div>
        <div class="flex justify-between items-center pb-1.5 border-b border-accent-secondary/15">
          <span class="text-primary/50 uppercase">Разрешение:</span>
          <span class="text-primary font-semibold">{{ currentDetectedResolution }}</span>
        </div>
        <div class="flex justify-between items-center">
          <span class="text-primary/50 uppercase">Потребление ресурсов:</span>
          <span class="text-amber-400 font-semibold">Стандартное (Высокое)</span>
        </div>
      </div>

      <p class="text-primary/60 text-[11px] leading-relaxed">
        Мы рекомендуем использовать Режим Картошки (1 FPS / 60 в меню) для фонового сидинга. Вы можете перезапустить Squad с оптимизацией либо продолжить сидинг в текущих настройках.
      </p>
    </div>

    <template #footer>
      <div class="flex flex-col sm:flex-row items-stretch sm:items-center justify-end gap-2 w-full">
        <BaseButton variant="ghost" size="sm" @click="showPotatoWarningModal = false">
          {{ $t('common.cancel') || 'Отмена' }}
        </BaseButton>
        <BaseButton variant="secondary" size="sm" :loading="isLoading" @click="handleConfirmCurrentSettings">
          Продолжить в текущих
        </BaseButton>
        <BaseButton variant="tactical" size="sm" :loading="isRestartingSquad" @click="handleApplyPotatoAndStart">
          <Zap class="w-3.5 h-3.5 mr-1.5" />
          <span>{{ isSquadRunning ? 'Перезапустить в Картошке' : 'Включить Картошку и начать' }}</span>
        </BaseButton>
      </div>
    </template>
  </BaseModal>
</template>
