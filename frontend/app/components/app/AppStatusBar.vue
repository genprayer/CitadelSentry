<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Radio, Gamepad2, Layers, Save, RotateCcw } from 'lucide-vue-next'
import { useBridge } from '~/composables/useBridge'
import { useSettings } from '~/composables/useSettings'
import BaseButton from '~/components/ui/BaseButton.vue'

const route = useRoute()
const { isBridgeReady, isSquadRunning, isSteamRunning, status } = useBridge()
const { hasUnsavedChanges, saveDraft, resetDraft, isSaving, isLoaded } = useSettings()

const isSettingsPage = computed(() => route.path === '/settings')

const handleSave = async () => {
  try {
    await saveDraft()
  } catch (err) {
    console.error('Failed to save settings from status bar', err)
  }
}

const handleReset = () => {
  resetDraft()
}
</script>

<template>
  <div class="h-9 w-full bg-surface border-t border-accent-secondary/15 px-4 flex items-center justify-between text-xs font-mono select-none text-primary/70">
    <!-- Left: Core Services Status -->
    <div class="flex items-center space-x-5">
      <!-- Bridge -->
      <div class="flex items-center space-x-1.5">
        <Radio :class="['w-3.5 h-3.5', isBridgeReady ? 'text-emerald-400 animate-pulse' : 'text-primary/40']" />
        <span>{{ $t('settings.bridgeLabel') }}:</span>
        <span :class="isBridgeReady ? 'text-emerald-400 font-semibold' : 'text-amber-400'">
          {{ isBridgeReady ? $t('settings.bridgeConnected') : $t('settings.bridgeDisconnected') }}
        </span>
      </div>

      <!-- Steam Process -->
      <div class="flex items-center space-x-1.5">
        <Layers :class="['w-3.5 h-3.5', isSteamRunning ? 'text-emerald-400' : 'text-primary/40']" />
        <span>{{ $t('settings.steamProcess') }}:</span>
        <span :class="isSteamRunning ? 'text-emerald-400 font-semibold' : 'text-primary/40'">
          {{ isSteamRunning === null ? $t('settings.unknown') : isSteamRunning ? $t('settings.active') : $t('settings.inactive') }}
        </span>
      </div>

      <!-- Squad Process -->
      <div class="flex items-center space-x-1.5">
        <Gamepad2 :class="['w-3.5 h-3.5', isSquadRunning ? 'text-emerald-400' : 'text-primary/40']" />
        <span>{{ $t('settings.squadProcess') }}:</span>
        <span :class="isSquadRunning ? 'text-emerald-400 font-semibold' : 'text-primary/40'">
          {{ isSquadRunning === null ? $t('settings.unknown') : isSquadRunning ? $t('settings.active') : $t('settings.inactive') }}
        </span>
      </div>
    </div>

    <!-- Right: In Settings page, replace brand with Save / Reset controls -->
    <div v-if="isSettingsPage" class="flex items-center space-x-2.5">
      <div v-if="hasUnsavedChanges" class="flex items-center space-x-1.5 text-[11px] text-amber-400 font-sans font-bold mr-1">
        <span class="w-2 h-2 rounded-full bg-amber-400 animate-pulse" />
        <span>{{ $t('settings.unsavedChanges') }}</span>
      </div>

      <BaseButton
        variant="ghost"
        size="xs"
        :disabled="!isLoaded || isSaving"
        @click="handleReset"
      >
        <RotateCcw class="w-3 h-3 mr-1" />
        <span>{{ $t('settings.resetDefaults') }}</span>
      </BaseButton>

      <BaseButton
        variant="tactical"
        size="xs"
        :loading="isSaving"
        :disabled="!isLoaded"
        @click="handleSave"
      >
        <Save class="w-3 h-3 mr-1" />
        <span>{{ $t('settings.saveChanges') }}</span>
      </BaseButton>
    </div>

    <!-- Right: Default Brand & Timestamp when not in settings -->
    <div v-else class="flex items-center space-x-2 text-xs">
      <span class="text-accent font-semibold tracking-wider uppercase">Citadel Sentry</span>
      <span class="text-primary/30">•</span>
      <span class="text-primary/50 font-mono">v{{ status.appVersion || 'dev' }}</span>
      <span class="text-primary/30">•</span>
      <span v-if="status.lastPingAt" class="text-emerald-400/80">{{ new Date(status.lastPingAt).toLocaleTimeString() }}</span>
      <span v-else class="text-emerald-400/80">{{ $t('settings.ready') }}</span>
    </div>
  </div>
</template>
