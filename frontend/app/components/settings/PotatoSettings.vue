<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { Zap, RotateCcw, ShieldCheck, CheckCircle2, AlertCircle, FileCode, Save, Plus, Trash2, Cpu, History } from 'lucide-vue-next'
import type { PotatoSettings } from '~/types'
import { useSettings } from '~/composables/useSettings'
import { useBridge } from '~/composables/useBridge'
import { useI18n } from '~/composables/useI18n'
import { useFeedbackTimer } from '~/composables/use-feedback-timer'
import BaseCard from '~/components/ui/BaseCard.vue'
import BaseToggle from '~/components/ui/BaseToggle.vue'
import BaseButton from '~/components/ui/BaseButton.vue'
import BaseBadge from '~/components/ui/BaseBadge.vue'
import CiTooltip from '~/components/ui/CiTooltip.vue'

interface Props {
  modelValue?: PotatoSettings
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: () => ({
    enabled: true,
    fpsLimit: 1,
    lowResolution: false,
    lowCpuPriority: true,
    autoRestoreOnExit: true
  })
})

const emit = defineEmits<{
  'update:modelValue': [value: PotatoSettings]
}>()

const potato = computed(() => ({
  enabled: props.modelValue?.enabled ?? true,
  fpsLimit: props.modelValue?.fpsLimit ?? 1,
  lowResolution: props.modelValue?.lowResolution ?? false,
  lowCpuPriority: props.modelValue?.lowCpuPriority ?? true,
  autoRestoreOnExit: props.modelValue?.autoRestoreOnExit ?? true
}))

const formatPointDate = (dateStr: string) => {
  try {
    const d = new Date(dateStr)
    return isNaN(d.getTime()) ? dateStr : d.toLocaleString()
  } catch {
    return dateStr
  }
}

const {
  configStatus,
  configStatusError,
  isRestoringConfig,
  isSavingBackup,
  restoreSquadConfig,
  saveCurrentSquadConfigAsBackup,
  fetchSquadConfigStatus,
  gpuInfo,
  restorePoints,
  isRestoringBaseline,
  isCreatingRestorePoint,
  restoreInitialBaseline,
  fetchGPUInfo,
  fetchRestorePoints,
  createRestorePoint,
  restoreRestorePoint,
  deleteRestorePoint
} = useSettings()
const { t } = useI18n()
const { isSquadRunning, status } = useBridge()

const restoreFeedback = ref<string | null>(null)
const restoreError = ref<string | null>(null)
const saveBackupFeedback = ref<string | null>(null)
const saveBackupError = ref<string | null>(null)
const baselineFeedback = ref<string | null>(null)
const baselineError = ref<string | null>(null)
const pointFeedback = ref<string | null>(null)
const pointError = ref<string | null>(null)
const newPointName = ref('')

const scheduleRestoreFeedback = useFeedbackTimer()
const scheduleBackupFeedback = useFeedbackTimer()
const scheduleBaselineFeedback = useFeedbackTimer()
const schedulePointFeedback = useFeedbackTimer()

const fpsPresets = [1, 5, 10, 15, 30]

const updateField = <K extends keyof PotatoSettings>(key: K, value: PotatoSettings[K]) => {
  emit('update:modelValue', {
    ...potato.value,
    [key]: value
  })
}

const handleRestoreConfig = async () => {
  restoreFeedback.value = null
  restoreError.value = null
  saveBackupFeedback.value = null
  saveBackupError.value = null
  try {
    const ok = await restoreSquadConfig()
    if (ok) {
      restoreFeedback.value = t('potato.restoredSuccess')
      scheduleRestoreFeedback(() => {
        restoreFeedback.value = null
      }, 4000)
    }
  } catch (err: any) {
    restoreError.value = err?.message || t('common.error')
  }
}

const handleRestoreBaseline = async () => {
  baselineFeedback.value = null
  baselineError.value = null
  try {
    const ok = await restoreInitialBaseline()
    if (ok) {
      baselineFeedback.value = t('potato.baselineRestoredSuccess')
      scheduleBaselineFeedback(() => {
        baselineFeedback.value = null
      }, 4000)
    }
  } catch (err: any) {
    baselineError.value = err?.message || t('common.error')
  }
}

const handleCreatePoint = async () => {
  pointFeedback.value = null
  pointError.value = null
  if (!newPointName.value.trim()) return
  try {
    await createRestorePoint(newPointName.value.trim(), '')
    pointFeedback.value = t('potato.pointCreatedSuccess')
    newPointName.value = ''
    schedulePointFeedback(() => {
      pointFeedback.value = null
    }, 4000)
  } catch (err: any) {
    pointError.value = err?.message || t('common.error')
  }
}

const handleRestorePoint = async (id: string) => {
  pointFeedback.value = null
  pointError.value = null
  try {
    const ok = await restoreRestorePoint(id)
    if (ok) {
      pointFeedback.value = t('potato.pointRestoredSuccess')
      schedulePointFeedback(() => {
        pointFeedback.value = null
      }, 4000)
    }
  } catch (err: any) {
    pointError.value = err?.message || t('common.error')
  }
}

const handleDeletePoint = async (id: string) => {
  pointFeedback.value = null
  pointError.value = null
  try {
    await deleteRestorePoint(id)
    pointFeedback.value = t('potato.pointDeletedSuccess')
    schedulePointFeedback(() => {
      pointFeedback.value = null
    }, 4000)
  } catch (err: any) {
    pointError.value = err?.message || t('common.error')
  }
}

const handleSaveBackup = async () => {
  restoreFeedback.value = null
  saveBackupFeedback.value = null
  saveBackupError.value = null

  if (configStatus.value?.isPotatoMode) {
    saveBackupError.value = t('potato.cannotSaveWhilePotato')
    return
  }

  try {
    const ok = await saveCurrentSquadConfigAsBackup()
    if (ok) {
      saveBackupFeedback.value = t('potato.saveBackupSuccess')
      scheduleBackupFeedback(() => {
        saveBackupFeedback.value = null
      }, 4000)
    }
  } catch (err: any) {
    saveBackupError.value = err?.message || 'Failed to save backup'
  }
}

const gpuUpscaleText = computed(() => {
  if (!gpuInfo.value || !gpuInfo.value.vendor) return t('potato.gpuUpscaleFsr')
  const vendor = String(gpuInfo.value.vendor).toLowerCase()
  if (vendor === 'nvidia' && gpuInfo.value.isRtx) {
    return t('potato.gpuUpscaleDlss')
  }
  if (vendor === 'amd') {
    return t('potato.gpuUpscaleFsrAmdFg')
  }
  return t('potato.gpuUpscaleFsr')
})

onMounted(() => {
  fetchSquadConfigStatus()
  fetchGPUInfo()
  fetchRestorePoints()
})
</script>

<template>
  <BaseCard variant="panel" padding="lg" class="space-y-5">
    <!-- Header -->
    <div class="flex items-center justify-between pb-3 border-b border-accent-secondary/15">
      <div class="flex items-center space-x-2.5">
        <div class="w-8 h-8 rounded-lg bg-accent/15 border border-accent/30 flex items-center justify-center text-accent">
          <Zap class="w-4 h-4" />
        </div>
        <div>
          <h3 class="text-sm font-semibold text-primary font-sans uppercase tracking-wide">
            {{ $t('potato.title') }}
          </h3>
          <p class="text-xs text-primary/60">{{ $t('potato.subtitle') }}</p>
        </div>
      </div>

      <BaseBadge
        :variant="potato.enabled ? 'brand' : 'neutral'"
        size="sm"
      >
        {{ potato.enabled ? $t('dashboard.enabled') : $t('dashboard.disabled') }}
      </BaseBadge>
    </div>

    <!-- Detected Hardware & GPU Upscaling Info -->
    <div v-if="gpuInfo && gpuInfo.model" class="p-3 bg-surface/70 border border-accent-secondary/20 rounded-xl space-y-2 font-mono text-xs">
      <div class="flex items-center justify-between">
        <div class="flex items-center space-x-2 text-primary font-sans font-semibold">
          <Cpu class="w-4 h-4 text-accent" />
          <span>{{ $t('potato.gpuDetected') }}</span>
          <span class="text-accent font-mono">{{ gpuInfo.model }}</span>
        </div>
        <BaseBadge :variant="gpuInfo?.isRtx ? 'brand' : 'neutral'" size="sm">
          {{ (gpuInfo?.vendor || 'GPU').toUpperCase() }}
        </BaseBadge>
      </div>
      <div class="text-[11px] text-primary/70 flex items-center justify-between">
        <span class="text-primary/50">{{ $t('potato.scalingProfile') }}:</span>
        <span class="text-emerald-400 font-sans font-medium">{{ gpuUpscaleText }}</span>
      </div>
    </div>

    <!-- Toggles & Configuration Parameters (Always Visible) -->
    <div class="space-y-4">
      <BaseToggle
        :model-value="potato.enabled"
        :label="$t('potato.enabled')"
        :description="$t('potato.enabledDesc')"
        @update:model-value="updateField('enabled', $event)"
      />

      <!-- Sub-options always visible so user can configure FPS and options anytime -->
      <div class="pl-3 border-l-2 border-accent/30 space-y-4 pt-1">
        <!-- FPS Limit Selection -->
        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <label class="block text-xs font-bold text-primary font-sans uppercase tracking-wide">
              {{ $t('potato.fpsLimit') }}
            </label>
            <span class="text-xs font-mono font-bold text-accent">
              {{ potato.fpsLimit }} FPS
            </span>
          </div>
          <p class="text-[11px] text-primary/60">{{ $t('potato.fpsLimitDesc') }}</p>

          <div class="flex items-center space-x-2 pt-1">
            <button
              v-for="fps in fpsPresets"
              :key="fps"
              type="button"
              :class="[
                'flex-1 py-1.5 rounded text-xs font-mono border transition-colors',
                potato.fpsLimit === fps
                  ? 'bg-accent/20 text-accent border-accent/50 font-bold'
                  : 'bg-surface text-primary/60 border-accent-secondary/20 hover:text-primary'
              ]"
              @click="updateField('fpsLimit', fps)"
            >
              {{ fps }} FPS
            </button>
          </div>
        </div>

        <BaseToggle
          :model-value="potato.lowResolution"
          :label="$t('potato.lowResolution')"
          :description="$t('potato.lowResolutionDesc')"
          @update:model-value="updateField('lowResolution', $event)"
        />

        <BaseToggle
          :model-value="potato.lowCpuPriority"
          :label="$t('potato.lowCpuPriority')"
          :description="$t('potato.lowCpuPriorityDesc')"
          :disabled="status?.canSetLowCpuPriority !== true && !potato.lowCpuPriority"
          @update:model-value="updateField('lowCpuPriority', $event)"
        />

        <p v-if="status?.canSetLowCpuPriority === false" class="text-xs text-amber-300">{{ $t('potato.lowCpuPriorityUnavailable') }}</p>

        <BaseToggle
          :model-value="potato.autoRestoreOnExit"
          :label="$t('potato.autoRestore')"
          :description="$t('potato.autoRestoreDesc')"
          @update:model-value="updateField('autoRestoreOnExit', $event)"
        />
      </div>

      <!-- Squad Configuration Telemetry & Manual Restore -->
      <div class="p-3.5 bg-surface/70 border border-accent-secondary/20 rounded-xl space-y-3 font-mono text-xs">
        <div class="flex items-center justify-between">
          <div class="flex items-center space-x-2 text-primary font-sans font-semibold">
            <FileCode class="w-4 h-4 text-accent" />
            <span>{{ $t('potato.configStatus') }}</span>
          </div>

          <BaseBadge
            v-if="configStatus"
            :variant="configStatus.hasPendingRestore ? 'warning' : 'success'"
            size="sm"
          >
            {{ configStatus.hasPendingRestore ? (configStatus.isPotatoMode ? $t('potato.statusActive', { fps: configStatus.currentFpsCap }) : $t('potato.statusPending')) : $t('potato.statusNormal') }}
          </BaseBadge>
        </div>

        <div v-if="configStatus" class="space-y-1.5 text-[11px] text-primary/70">
          <div class="flex items-center justify-between">
            <span class="text-primary/50">{{ $t('potato.configFound') }}:</span>
            <span :class="configStatus.configFound ? 'text-emerald-400' : 'text-rose-400'">
              {{ configStatus.configFound ? 'GameUserSettings.ini' : $t('potato.configNotFound') }}
            </span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-primary/50">{{ $t('potato.backupFound') }}:</span>
            <span :class="configStatus.hasBackup ? 'text-emerald-400' : 'text-primary/40'">
              {{ configStatus.hasBackup ? 'GameUserSettings.ini.original' : $t('potato.noBackup') }}
            </span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-primary/50">{{ $t('potato.baselineFound') }}:</span>
            <span :class="configStatus.hasBaseline ? 'text-emerald-400' : 'text-primary/40'">
              {{ configStatus.hasBaseline ? 'baseline.json (Зафиксировано)' : $t('potato.noBaseline') }}
            </span>
          </div>
        </div>

        <p class="text-[11px] text-primary/50 leading-relaxed">
          {{ $t('potato.backupExplainTip') }}
        </p>

        <div v-if="baselineFeedback" class="p-2.5 bg-emerald-500/10 border border-emerald-500/30 rounded-lg flex items-center space-x-2 text-xs text-emerald-300">
          <CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0" />
          <span>{{ baselineFeedback }}</span>
        </div>

        <div v-if="baselineError" class="p-2.5 bg-rose-500/10 border border-rose-500/30 rounded-lg flex items-center space-x-2 text-xs text-rose-300">
          <AlertCircle class="w-4 h-4 text-rose-400 shrink-0" />
          <span>{{ baselineError }}</span>
        </div>

        <div v-if="restoreFeedback" class="p-2.5 bg-emerald-500/10 border border-emerald-500/30 rounded-lg flex items-center space-x-2 text-xs text-emerald-300">
          <CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0" />
          <span>{{ restoreFeedback }}</span>
        </div>

        <div v-if="saveBackupFeedback" class="p-2.5 bg-emerald-500/10 border border-emerald-500/30 rounded-lg flex items-center space-x-2 text-xs text-emerald-300">
          <CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0" />
          <span>{{ saveBackupFeedback }}</span>
        </div>

        <div v-if="saveBackupError" class="p-2.5 bg-rose-500/10 border border-rose-500/30 rounded-lg flex items-center space-x-2 text-xs text-rose-300">
          <AlertCircle class="w-4 h-4 text-rose-400 shrink-0" />
          <span>{{ saveBackupError }}</span>
        </div>

        <div v-if="restoreError || configStatusError" role="alert" class="p-2.5 bg-rose-500/10 border border-rose-500/30 rounded-lg flex items-center space-x-2 text-xs text-rose-300">
          <AlertCircle class="w-4 h-4 text-rose-400 shrink-0" />
          <span>{{ restoreError || configStatusError }}</span>
          <BaseButton v-if="configStatusError" variant="ghost" size="sm" @click="fetchSquadConfigStatus">{{ $t('common.retry') }}</BaseButton>
        </div>

        <div v-if="configStatus?.isPotatoMode" class="p-2 bg-amber-500/10 border border-amber-500/20 rounded text-[11px] text-amber-200/90 flex items-center space-x-1.5">
          <AlertCircle class="w-3.5 h-3.5 text-amber-400 shrink-0" />
          <span>{{ $t('potato.cannotSaveWhilePotato') }}</span>
        </div>

        <p v-if="isSquadRunning === null" class="text-xs text-amber-300">{{ $t('potato.processUnknownNotice') }}</p>

        <!-- Actions -->
        <div class="pt-2 flex flex-wrap items-center justify-end gap-2">
          <!-- 1-Click Permanent Restore Initial Baseline Button -->
          <CiTooltip
            :title="$t('potato.restoreInitialBaselineTooltipTitle')"
            :description="$t('potato.restoreInitialBaselineTooltipDesc')"
          >
            <BaseButton
              variant="tactical"
              size="sm"
              :loading="isRestoringBaseline"
              :disabled="!configStatus?.hasBaseline || isSquadRunning !== false"
              @click="handleRestoreBaseline"
            >
              <History class="w-3.5 h-3.5 mr-1.5" />
              <span>{{ $t('potato.restoreInitialBaseline') }}</span>
            </BaseButton>
          </CiTooltip>

          <CiTooltip
            :title="$t('potato.saveBackupTooltipTitle')"
            :description="$t('potato.saveBackupTooltipDesc')"
          >
            <BaseButton
              variant="secondary"
              size="sm"
              :loading="isSavingBackup"
              :disabled="!configStatus?.configFound || configStatus?.hasPendingRestore || isSquadRunning !== false"
              @click="handleSaveBackup"
            >
              <Save class="w-3.5 h-3.5 mr-1.5 text-accent" />
              <span>{{ $t('potato.saveBackupButton') }}</span>
            </BaseButton>
          </CiTooltip>

          <CiTooltip
            :title="$t('potato.restoreTooltipTitle')"
            :description="$t('potato.restoreTooltipDesc')"
          >
            <BaseButton
              variant="secondary"
              size="sm"
              :loading="isRestoringConfig"
              :disabled="!configStatus?.hasPendingRestore || isSquadRunning !== false"
              @click="handleRestoreConfig"
            >
              <RotateCcw class="w-3.5 h-3.5 mr-1.5 text-accent" />
              <span>{{ $t('potato.restoreButton') }}</span>
            </BaseButton>
          </CiTooltip>
        </div>
      </div>

      <!-- Multi-Restore Points (Snapshots) Section -->
      <div class="p-3.5 bg-surface/70 border border-accent-secondary/20 rounded-xl space-y-3 font-mono text-xs">
        <div class="flex items-center justify-between">
          <div class="flex items-center space-x-2 text-primary font-sans font-semibold">
            <History class="w-4 h-4 text-accent" />
            <span>{{ $t('potato.restorePointsTitle') }}</span>
          </div>
          <BaseBadge variant="neutral" size="sm">
            {{ (restorePoints || []).length }}
          </BaseBadge>
        </div>
        <p class="text-[11px] text-primary/60 font-sans">
          {{ $t('potato.restorePointsSubtitle') }}
        </p>

        <!-- Create point input -->
        <div class="flex items-center space-x-2 pt-1">
          <input
            v-model="newPointName"
            type="text"
            :placeholder="$t('potato.restorePointNamePlaceholder')"
            class="flex-1 px-3 py-1.5 bg-surface border border-accent-secondary/30 rounded text-xs text-primary placeholder-primary/40 focus:outline-none focus:border-accent font-sans"
            :disabled="configStatus?.isPotatoMode || isSquadRunning !== false"
            @keyup.enter="handleCreatePoint"
          />
          <BaseButton
            variant="secondary"
            size="sm"
            :loading="isCreatingRestorePoint"
            :disabled="!newPointName.trim() || configStatus?.isPotatoMode || isSquadRunning !== false"
            @click="handleCreatePoint"
          >
            <Plus class="w-3.5 h-3.5 mr-1 text-accent" />
            <span>{{ $t('potato.createRestorePoint') }}</span>
          </BaseButton>
        </div>

        <div v-if="pointFeedback" class="p-2 bg-emerald-500/10 border border-emerald-500/30 rounded text-xs text-emerald-300 flex items-center space-x-2 font-sans">
          <CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0" />
          <span>{{ pointFeedback }}</span>
        </div>
        <div v-if="pointError" class="p-2 bg-rose-500/10 border border-rose-500/30 rounded text-xs text-rose-300 flex items-center space-x-2 font-sans">
          <AlertCircle class="w-4 h-4 text-rose-400 shrink-0" />
          <span>{{ pointError }}</span>
        </div>

        <!-- List of restore points -->
        <div v-if="restorePoints && restorePoints.length > 0" class="space-y-1.5 pt-1">
          <div
            v-for="point in (restorePoints || [])"
            :key="point.id"
            class="p-2 bg-surface/90 border border-accent-secondary/15 rounded-lg flex items-center justify-between text-xs font-sans"
          >
            <div class="space-y-0.5">
              <div class="font-medium text-primary">{{ point.name }}</div>
              <div class="text-[10px] text-primary/40 font-mono">
                {{ formatPointDate(point.createdAt) }}
              </div>
            </div>
            <div class="flex items-center space-x-1.5">
              <BaseButton
                variant="ghost"
                size="xs"
                :disabled="isSquadRunning !== false"
                @click="handleRestorePoint(point.id)"
              >
                <RotateCcw class="w-3 h-3 mr-1 text-accent" />
                <span>{{ $t('potato.restorePointAction') }}</span>
              </BaseButton>
              <BaseButton
                variant="ghost"
                size="xs"
                @click="handleDeletePoint(point.id)"
              >
                <Trash2 class="w-3 h-3 text-rose-400 hover:text-rose-300" />
              </BaseButton>
            </div>
          </div>
        </div>
        <div v-else class="text-[11px] text-primary/40 italic py-1 font-sans">
          {{ $t('potato.noRestorePoints') }}
        </div>
      </div>

      <!-- Safe guarantee notice -->
      <div class="p-3 bg-emerald-500/10 border border-emerald-500/20 rounded-lg flex items-center space-x-2.5 text-xs text-emerald-300">
        <ShieldCheck class="w-4 h-4 text-emerald-400 shrink-0" />
        <span>{{ $t('potato.safeNotice') }}</span>
      </div>
    </div>
  </BaseCard>
</template>
