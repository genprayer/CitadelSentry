<script setup lang="ts">
import { AlertTriangle, Info } from 'lucide-vue-next'
import { useBridge } from '~/composables/useBridge'

const { isBridgeReady } = useBridge()
</script>

<template>
  <div
    v-if="!isBridgeReady"
    class="bg-amber-500/10 border-b border-amber-500/20 px-4 py-2 flex items-center justify-between text-xs text-amber-300 font-sans"
  >
    <div class="flex items-center space-x-2">
      <AlertTriangle class="w-4 h-4 text-amber-400 flex-shrink-0" />
      <span>
        <strong>Browser Preview Mode:</strong> Wails desktop bridge is disconnected. Game execution and Steam hooks are running in standby mode.
      </span>
    </div>
    <span class="text-[11px] font-mono text-amber-400/80 bg-amber-500/20 px-2 py-0.5 rounded">
      Offline Bridge
    </span>
  </div>
</template>
