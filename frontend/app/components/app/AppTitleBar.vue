<script setup lang="ts">
import { Minus, Square, X, Shield } from 'lucide-vue-next'
import { useBridge } from '~/composables/useBridge'

const { isBridgeReady, platform, status, minimize, maximize, close } = useBridge()
</script>

<template>
  <div class="h-9 w-full bg-surface border-b border-accent-secondary/15 flex items-center justify-between px-3 select-none wails-drag z-40">
    <!-- Left: App Brand & Platform -->
    <div class="flex items-center space-x-2.5 wails-no-drag">
      <div class="flex items-center space-x-2">
        <img src="/img/logo_citadel.png" alt="Citadel" class="w-4 h-4 object-contain drop-shadow-[0_0_6px_rgba(235,225,200,0.4)]" />
        <span class="text-xs font-bold tracking-wider uppercase font-sans text-primary">
          Citadel <span class="text-accent">Sentry</span>
        </span>
      </div>

      <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-surface border border-accent-secondary/20 text-accent">
        v{{ status.appVersion }}
      </span>

      <span
        v-if="!isBridgeReady"
        class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20"
      >
        Browser Preview
      </span>
      <span
        v-else
        class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 uppercase"
      >
        {{ platform }}
      </span>
    </div>

    <!-- Center: Empty drag region -->
    <div class="flex-1 h-full wails-drag" />

    <!-- Right: Window Controls (Wails Desktop Only) -->
    <div class="flex items-center space-x-1 wails-no-drag">
      <button
        type="button"
        class="w-7 h-6 flex items-center justify-center rounded hover:bg-surface/80 text-primary/60 hover:text-accent transition-colors"
        title="Minimize"
        @click="minimize"
      >
        <Minus class="w-3.5 h-3.5" />
      </button>
      <button
        type="button"
        class="w-7 h-6 flex items-center justify-center rounded hover:bg-surface/80 text-primary/60 hover:text-accent transition-colors"
        title="Maximize"
        @click="maximize"
      >
        <Square class="w-3 h-3" />
      </button>
      <button
        type="button"
        class="w-7 h-6 flex items-center justify-center rounded hover:bg-rose-600/80 text-primary/60 hover:text-white transition-colors"
        title="Close"
        @click="close"
      >
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  </div>
</template>
