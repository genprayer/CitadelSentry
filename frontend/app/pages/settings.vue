<script setup lang="ts">
import {
  Settings,
  Sliders,
  Play,
  Zap,
  Gamepad2,
  Radio,
  Save,
  RotateCcw,
  CheckCircle2,
  AlertCircle,
  Globe,
  FolderOpen,
  Copy,
  Check,
  FileText,
  Terminal,
  RefreshCw,
  Trash2
} from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'
import { useSettings } from '~/composables/useSettings'
import { useBridge } from '~/composables/useBridge'
import { useI18n } from '~/composables/useI18n'
import { useFeedbackTimer } from '~/composables/use-feedback-timer'
import { bridgeAdapter } from '~/adapters/wails'
import type { AppSettings, UpdateInfo } from '~/types'
import GeneralSettings from '~/components/settings/GeneralSettings.vue'
import SeedSettings from '~/components/settings/SeedSettings.vue'
import PowerSettings from '~/components/settings/PowerSettings.vue'
import PotatoSettings from '~/components/settings/PotatoSettings.vue'
import BaseCard from '~/components/ui/BaseCard.vue'
import BaseButton from '~/components/ui/BaseButton.vue'
import BaseBadge from '~/components/ui/BaseBadge.vue'
import BaseModal from '~/components/ui/BaseModal.vue'
import CiTooltip from '~/components/ui/CiTooltip.vue'

type SettingsTab = 'general' | 'seed' | 'power' | 'potato' | 'diagnostics'

const route = useRoute()
const router = useRouter()
const { settings, draftSettings, hasUnsavedChanges, saveSuccess, isSaving, isLoading, isLoaded, error, saveDraft, resetDraft, syncDraft, loadSettings } = useSettings()
const { status, isBridgeReady, platform, openLogsFolder, getRecentLogs, clearLogs } = useBridge()
const { t } = useI18n()

const activeTab = ref<SettingsTab>('general')

watch(
  () => route.query?.tab,
  (newTab: any) => {
    if (newTab && ['general', 'seed', 'power', 'potato', 'diagnostics'].includes(newTab as string)) {
      activeTab.value = newTab as SettingsTab
    }
  },
  { immediate: true }
)

const setTab = (tab: SettingsTab) => {
  activeTab.value = tab
  router.replace({ query: { ...route.query, tab } })
}

const networkEndpointDisplay = computed(() => {
  const url = settings.value?.webApiBaseUrl || ''
  try {
    return new URL(url).hostname
  } catch {
    return url.replace(/^https?:\/\//, '')
  }
})

const defaultPotato: import('~/types').PotatoSettings = {
  enabled: true,
  fpsLimit: 1,
  lowResolution: false,
  lowCpuPriority: true,
  autoRestoreOnExit: true
}

const potatoDraft = computed({
  get: () => draftSettings.value?.potato || defaultPotato,
  set: (val: import('~/types').PotatoSettings) => {
    if (draftSettings.value) {
      draftSettings.value.potato = val
    }
  }
})

const schedulePathFeedback = useFeedbackTimer()
const scheduleLogsFeedback = useFeedbackTimer()
const saveError = ref<string | null>(null)
const updateInfo = ref<UpdateInfo | null>(null)
const updateError = ref<string | null>(null)
const isCheckingUpdate = ref(false)
const isInstallingUpdate = ref(false)

const handleUpdateButton = async () => {
  if (isCheckingUpdate.value || isInstallingUpdate.value) return
  updateError.value = null
  updateInfo.value = null
  isCheckingUpdate.value = true
  try {
    updateInfo.value = await bridgeAdapter.checkForUpdates()
    if (!updateInfo.value.available) return
    isCheckingUpdate.value = false
    isInstallingUpdate.value = true
    await bridgeAdapter.installUpdate(updateInfo.value.latestVersion)
  } catch (err) {
    updateError.value = err instanceof Error ? err.message : String(err)
    isInstallingUpdate.value = false
  } finally {
    isCheckingUpdate.value = false
  }
}

onMounted(async () => {
  if (!isLoaded.value) {
    try {
      await loadSettings()
    } catch (e) {
      console.warn('Failed to load settings in settings page', e)
    }
  } else {
    syncDraft()
  }
  try {
    updateError.value = await bridgeAdapter.getUpdateLastError() || null
  } catch {
    updateError.value = null
  }
})

const isCopyingPath = ref(false)
const handleCopyLogPath = async () => {
  const path = status.value?.logFilePath
  if (!path) return
  try {
    await navigator.clipboard.writeText(path)
    isCopyingPath.value = true
    schedulePathFeedback(() => {
      isCopyingPath.value = false
    }, 2500)
  } catch (err) {
    console.warn('Failed to copy log path to clipboard', err)
  }
}

const handleOpenLogsFolder = async () => {
  try {
    await openLogsFolder()
  } catch (err) {
    console.error('Failed to open logs folder', err)
  }
}

const isLogsModalOpen = ref(false)
const logsList = ref<string[]>([])
const isLoadingLogs = ref(false)
const isClearingLogs = ref(false)
const hasLoadedLogs = ref(false)
const logsError = ref<string | null>(null)
const isCopyingAllLogs = ref(false)
let logsRequestGeneration = 0

const loadLogs = async () => {
  if (isClearingLogs.value) return
  const generation = ++logsRequestGeneration
  isLoadingLogs.value = true
  logsError.value = null
  try {
    const lines = await getRecentLogs(250)
    if (generation !== logsRequestGeneration) return
    logsList.value = lines
    hasLoadedLogs.value = true
  } catch (err) {
    if (generation !== logsRequestGeneration) return
    logsError.value = err instanceof Error ? err.message : String(err)
  } finally {
    if (generation === logsRequestGeneration) isLoadingLogs.value = false
  }
}

const handleOpenLogsModal = async () => {
  isLogsModalOpen.value = true
  await loadLogs()
}

const handleCopyAllLogs = async () => {
  if (logsList.value.length === 0) return
  try {
    await navigator.clipboard.writeText(logsList.value.join('\n'))
    isCopyingAllLogs.value = true
    scheduleLogsFeedback(() => {
      isCopyingAllLogs.value = false
    }, 2500)
  } catch (err) {
    console.warn('Failed to copy logs to clipboard', err)
  }
}

const handleClearLogs = async () => {
  if (isClearingLogs.value) return
  ++logsRequestGeneration
  isLoadingLogs.value = false
  isClearingLogs.value = true
  logsError.value = null
  try {
    await clearLogs()
    logsList.value = []
    hasLoadedLogs.value = true
  } catch (err) {
    logsError.value = err instanceof Error ? err.message : String(err)
  } finally {
    isClearingLogs.value = false
  }
}

const handleSave = async () => {
  saveError.value = null
  try {
    await saveDraft()
  } catch (err) {
    saveError.value = err instanceof Error ? err.message : String(err)
  }
}

const handleReset = () => {
  resetDraft()
}
</script>

<template>
  <div class="space-y-6 max-w-4xl mx-auto">
    <!-- Top Settings Header with Quick Actions -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-accent-secondary/15">
      <div>
        <div class="flex items-center space-x-2">
          <h2 class="text-lg font-bold font-sans tracking-wide uppercase text-primary flex items-center space-x-2">
            <Settings class="w-5 h-5 text-accent" />
            <span>{{ $t('settings.title') }}</span>
          </h2>
          <BaseBadge v-if="hasUnsavedChanges" variant="warning" size="sm">
            {{ $t('settings.unsavedChanges') }}
          </BaseBadge>
        </div>
        <p class="text-xs text-primary/60 mt-0.5">
          {{ $t('settings.subtitle') }}
        </p>
      </div>

      <!-- Header Action Buttons -->
      <div class="flex items-center space-x-2 self-start sm:self-auto">
        <!-- Compact Update Button with Context Tooltip -->
        <CiTooltip
          :title="$t('settings.updateTitle')"
          :description="$t('settings.updateDescription')"
        >
          <BaseButton
            variant="secondary"
            size="sm"
            :loading="isCheckingUpdate || isInstallingUpdate"
            :disabled="!isBridgeReady || isCheckingUpdate || isInstallingUpdate"
            @click="handleUpdateButton"
          >
            <RefreshCw :class="['w-3.5 h-3.5 mr-1.5', isCheckingUpdate ? 'animate-spin text-accent' : 'text-accent']" />
            <span>{{ $t('settings.checkUpdates') }}</span>
          </BaseButton>
        </CiTooltip>

        <BaseButton
          v-if="hasUnsavedChanges"
          variant="ghost"
          size="sm"
          :disabled="!isLoaded || isSaving"
          @click="handleReset"
        >
          <RotateCcw class="w-3.5 h-3.5 mr-1.5" />
          <span>{{ $t('settings.resetDraft') || 'Сбросить' }}</span>
        </BaseButton>

        <BaseButton
          v-if="hasUnsavedChanges"
          variant="tactical"
          size="sm"
          :loading="isSaving"
          :disabled="!isLoaded"
          @click="handleSave"
        >
          <Save class="w-3.5 h-3.5 mr-1.5" />
          <span>{{ $t('settings.saveChanges') }}</span>
        </BaseButton>
      </div>
    </div>

    <!-- Update Status Alerts if active -->
    <div v-if="updateInfo && !updateInfo.available" class="p-2.5 bg-emerald-500/10 border border-emerald-500/30 rounded-lg flex items-center space-x-2 text-xs text-emerald-300">
      <CheckCircle2 class="w-4 h-4 text-emerald-400 flex-shrink-0" />
      <span>{{ $t('settings.upToDate', { version: updateInfo.currentVersion }) }}</span>
    </div>

    <div v-if="updateInfo?.available" class="p-2.5 bg-accent/15 border border-accent/40 rounded-lg flex items-center justify-between text-xs text-accent">
      <div class="flex items-center space-x-2">
        <RefreshCw class="w-4 h-4 text-accent animate-spin flex-shrink-0" />
        <span>{{ $t('settings.updateAvailable', { version: updateInfo.latestVersion }) }}</span>
      </div>
      <BaseButton size="xs" variant="tactical" :loading="isInstallingUpdate" @click="handleUpdateButton">
        {{ $t('settings.checkUpdates') }}
      </BaseButton>
    </div>

    <div v-if="updateError" class="p-2.5 bg-rose-500/10 border border-rose-500/30 rounded-lg flex items-center space-x-2 text-xs text-rose-300">
      <AlertCircle class="w-4 h-4 text-rose-400 flex-shrink-0" />
      <span>{{ updateError }}</span>
    </div>

    <!-- Save Feedback Alerts -->
    <div v-if="saveSuccess" class="p-3 bg-emerald-500/10 border border-emerald-500/30 rounded-lg flex items-center space-x-2 text-xs text-emerald-300">
      <CheckCircle2 class="w-4 h-4 text-emerald-400 flex-shrink-0" />
      <span>{{ $t('settings.savedSuccess') }}</span>
    </div>

    <div v-if="saveError || error" class="p-3 bg-rose-500/10 border border-rose-500/30 rounded-lg flex items-center space-x-2 text-xs text-rose-300">
      <AlertCircle class="w-4 h-4 text-rose-400 flex-shrink-0" />
      <span>{{ saveError || error }}</span>
    </div>

    <!-- Modular Horizontal Tabs Navigation -->
    <div class="flex items-center gap-1.5 p-1 bg-surface border border-accent-secondary/15 rounded-xl overflow-x-auto select-none shadow-inner">
      <button
        type="button"
        :class="[
          'flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-sans font-bold tracking-wider uppercase transition-all shrink-0',
          activeTab === 'general'
            ? 'bg-accent/15 text-accent border border-accent/40 shadow-sm'
            : 'text-primary/70 hover:text-primary hover:bg-surface/80 border border-transparent'
        ]"
        @click="setTab('general')"
      >
        <Sliders class="w-4 h-4" />
        <span>{{ $t('settings.tabGeneral') || 'Основные параметры' }}</span>
      </button>

      <button
        type="button"
        :class="[
          'flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-sans font-bold tracking-wider uppercase transition-all shrink-0',
          activeTab === 'seed'
            ? 'bg-accent/15 text-accent border border-accent/40 shadow-sm'
            : 'text-primary/70 hover:text-primary hover:bg-surface/80 border border-transparent'
        ]"
        @click="setTab('seed')"
      >
        <Play class="w-4 h-4" />
        <span>{{ $t('settings.tabSeed') || 'Автосидинг' }}</span>
      </button>

      <button
        type="button"
        :class="[
          'flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-sans font-bold tracking-wider uppercase transition-all shrink-0',
          activeTab === 'power'
            ? 'bg-accent/15 text-accent border border-accent/40 shadow-sm'
            : 'text-primary/70 hover:text-primary hover:bg-surface/80 border border-transparent'
        ]"
        @click="setTab('power')"
      >
        <Zap class="w-4 h-4" />
        <span>{{ $t('settings.tabPower') || 'Электропитание' }}</span>
      </button>

      <button
        type="button"
        :class="[
          'flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-sans font-bold tracking-wider uppercase transition-all shrink-0',
          activeTab === 'potato'
            ? 'bg-accent/15 text-accent border border-accent/40 shadow-sm'
            : 'text-primary/70 hover:text-primary hover:bg-surface/80 border border-transparent'
        ]"
        @click="setTab('potato')"
      >
        <Gamepad2 class="w-4 h-4" />
        <span>{{ $t('settings.tabPotato') || 'Мод картошки' }}</span>
      </button>

      <button
        type="button"
        :class="[
          'flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-sans font-bold tracking-wider uppercase transition-all shrink-0',
          activeTab === 'diagnostics'
            ? 'bg-accent/15 text-accent border border-accent/40 shadow-sm'
            : 'text-primary/70 hover:text-primary hover:bg-surface/80 border border-transparent'
        ]"
        @click="setTab('diagnostics')"
      >
        <Radio class="w-4 h-4" />
        <span>{{ $t('settings.tabDiagnostics') || 'Диагностика и логи' }}</span>
      </button>
    </div>

    <!-- Active Tab Isolated View -->
    <div class="mt-4 animate-in fade-in duration-150">
      <!-- 1. General Settings Form -->
      <div v-if="activeTab === 'general'">
        <GeneralSettings v-if="draftSettings?.general" v-model="draftSettings.general" />
      </div>

      <!-- 2. Seed Engine Settings Form -->
      <div v-else-if="activeTab === 'seed'">
        <SeedSettings v-if="draftSettings?.seed" v-model="draftSettings.seed" />
      </div>

      <!-- 3. Power Management Settings Form -->
      <div v-else-if="activeTab === 'power'">
        <PowerSettings v-if="draftSettings?.power" v-model="draftSettings.power" />
      </div>

      <!-- 4. Potato Mode Form -->
      <div v-else-if="activeTab === 'potato'">
        <PotatoSettings v-model="potatoDraft" />
      </div>

      <!-- 5. System Bridge Diagnostics & Application Logs -->
      <div v-else-if="activeTab === 'diagnostics'" class="space-y-4">
        <BaseCard variant="panel" padding="lg" class="space-y-4 font-mono text-xs">
          <div class="flex items-center justify-between pb-3 border-b border-accent-secondary/15">
            <div class="flex items-center space-x-2">
              <Radio class="w-4 h-4 text-accent" />
              <span class="font-bold uppercase text-primary font-sans text-sm">{{ $t('settings.diagnosticsTitle') }}</span>
            </div>
            <BaseBadge :variant="isBridgeReady ? 'success' : 'warning'" size="sm" :dot="isBridgeReady">
              {{ isBridgeReady ? $t('settings.bridgeConnected') : $t('settings.bridgeDisconnected') }}
            </BaseBadge>
          </div>

          <div class="grid grid-cols-2 sm:grid-cols-5 gap-3 text-primary/70">
            <div>
              <p class="text-[10px] uppercase text-primary/40 font-sans font-semibold">{{ $t('settings.platform') }}</p>
              <p class="text-primary font-semibold mt-0.5 uppercase">{{ platform }}</p>
            </div>
            <div>
              <p class="text-[10px] uppercase text-primary/40 font-sans font-semibold">{{ $t('settings.appVersion') }}</p>
              <p class="text-primary font-semibold mt-0.5">{{ status?.appVersion || '1.3.0' }}</p>
            </div>
            <div>
              <p class="text-[10px] uppercase text-primary/40 font-sans font-semibold">{{ $t('settings.squadProcess') }}</p>
              <p class="text-primary font-semibold mt-0.5">{{ status?.squadRunning === null ? $t('settings.unknown') : status?.squadRunning ? $t('settings.active') : $t('settings.inactive') }}</p>
            </div>
            <div>
              <p class="text-[10px] uppercase text-primary/40 font-sans font-semibold">{{ $t('settings.steamProcess') }}</p>
              <p class="text-primary font-semibold mt-0.5">{{ status?.steamRunning === null ? $t('settings.unknown') : status?.steamRunning ? $t('settings.active') : $t('settings.inactive') }}</p>
            </div>
            <div>
              <p class="text-[10px] uppercase text-primary/40 font-sans font-semibold">{{ $t('settings.networkEndpoint') }}</p>
              <p class="text-accent font-semibold mt-0.5 truncate" :title="settings?.webApiBaseUrl">{{ networkEndpointDisplay }}</p>
            </div>
          </div>

          <p v-if="status?.processStatusError" class="mt-3 text-xs text-amber-300">{{ status.processStatusError }}</p>

          <!-- Application Logs Bar -->
          <div v-if="status?.logFilePath" class="pt-4 border-t border-accent-secondary/15 flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs font-mono">
            <div class="flex items-center space-x-2.5 truncate min-w-0">
              <FileText class="w-4 h-4 text-accent shrink-0" />
              <div class="truncate min-w-0">
                <span class="text-[10px] uppercase text-primary/40 block font-sans font-semibold">{{ $t('settings.logPathLabel') }}</span>
                <span class="text-primary/80 truncate block text-[11px]" :title="status.logFilePath">{{ status.logFilePath }}</span>
              </div>
            </div>
            <div class="flex items-center space-x-2 shrink-0">
              <CiTooltip title="Журнал работы" description="Открыть встроенный просмотрщик последних событий и логов сессий">
                <BaseButton variant="secondary" size="sm" @click="handleOpenLogsModal">
                  <Terminal class="w-3.5 h-3.5 mr-1.5 text-accent" />
                  <span>{{ $t('settings.viewLogs') }}</span>
                </BaseButton>
              </CiTooltip>

              <CiTooltip title="Путь к файлу" description="Скопировать абсолютный системный путь к файлу логов в буфер обмена">
                <BaseButton variant="secondary" size="sm" @click="handleCopyLogPath">
                  <Check v-if="isCopyingPath" class="w-3.5 h-3.5 mr-1.5 text-accent" />
                  <Copy v-else class="w-3.5 h-3.5 mr-1.5 text-accent" />
                  <span>{{ isCopyingPath ? $t('settings.pathCopied') : $t('settings.copyLogPath') }}</span>
                </BaseButton>
              </CiTooltip>

              <CiTooltip title="Папка с логами" description="Открыть системный файловый менеджер в директории журналов Citadel">
                <BaseButton variant="secondary" size="sm" @click="handleOpenLogsFolder">
                  <FolderOpen class="w-3.5 h-3.5 mr-1.5 text-accent" />
                  <span>{{ $t('settings.openLogsFolder') }}</span>
                </BaseButton>
              </CiTooltip>
            </div>
          </div>
        </BaseCard>
      </div>
    </div>

    <!-- Live Logs Viewer Modal -->
    <BaseModal
      v-model="isLogsModalOpen"
      :title="$t('settings.logsModalTitle')"
      :subtitle="$t('settings.logsModalSubtitle')"
      maxWidth="xl"
    >
      <div class="space-y-3">
        <div class="flex items-center justify-between pb-2 border-b border-accent-secondary/15 text-xs">
          <span class="text-primary/60 font-mono text-[11px]">
            {{ logsList.length }} lines loaded
          </span>
          <div class="flex items-center space-x-2">
            <BaseButton variant="ghost" size="sm" :loading="isLoadingLogs" :disabled="isClearingLogs" @click="loadLogs">
              <RefreshCw class="w-3.5 h-3.5 mr-1 text-primary/60" />
              <span>{{ $t('settings.refreshLogs') }}</span>
            </BaseButton>
            <BaseButton variant="secondary" size="sm" @click="handleCopyAllLogs">
              <Check v-if="isCopyingAllLogs" class="w-3.5 h-3.5 mr-1 text-accent" />
              <Copy v-else class="w-3.5 h-3.5 mr-1 text-accent" />
              <span>{{ isCopyingAllLogs ? $t('settings.allLogsCopied') : $t('settings.copyAllLogs') }}</span>
            </BaseButton>
            <BaseButton variant="danger" size="sm" :loading="isClearingLogs" @click="handleClearLogs">
              <Trash2 class="w-3.5 h-3.5 mr-1" />
              <span>{{ $t('settings.clearLogs') }}</span>
            </BaseButton>
          </div>
        </div>

        <div v-if="logsError" role="alert" class="p-3 bg-rose-500/10 border border-rose-500/30 rounded-lg text-xs text-rose-300">
          {{ logsError }}
        </div>
        <div class="h-96 overflow-y-auto bg-black/70 rounded-lg p-3 font-mono text-[11px] leading-relaxed border border-accent-secondary/20 select-text">
          <div v-if="isLoadingLogs && !hasLoadedLogs" class="h-full flex items-center justify-center text-primary/40">
            {{ $t('common.loading') }}
          </div>
          <div v-else-if="hasLoadedLogs && !logsError && logsList.length === 0" class="h-full flex items-center justify-center text-primary/40">
            {{ $t('settings.noLogs') }}
          </div>
          <div v-else class="space-y-0.5 whitespace-pre-wrap break-all">
            <div
              v-for="(line, idx) in logsList"
              :key="idx"
              :class="{
                'text-red-400 font-semibold': line.includes('[ERROR]'),
                'text-amber-400': line.includes('[WARN]'),
                'text-accent font-semibold': line.includes('PRE-LAUNCH') || line.includes('APPLICATION SETTINGS') || line.includes('STOPPING SEED'),
                'text-primary/80': !line.includes('[ERROR]') && !line.includes('[WARN]') && !line.includes('===')
              }"
            >
              {{ line }}
            </div>
          </div>
        </div>
      </div>
    </BaseModal>
  </div>
</template>