<script setup lang="ts">
import { Users, Server, Flame, RefreshCw } from 'lucide-vue-next'
import { useServers } from '~/composables/useServers'
import { useSeed } from '~/composables/useSeed'
import LanguageSelector from '~/components/ui/LanguageSelector.vue'

const { servers, seedingServers, totalOnlinePlayers, isLoading, fetchServers } = useServers()
const { isSeedingActive, isSessionStatusKnown, session } = useSeed()
</script>

<template>
  <header class="h-14 bg-surface border-b border-accent-secondary/15 px-6 flex items-center justify-between select-none shadow-sm">
    <!-- Left: Clean Citadel Icon + Status / Active Seeding Summary -->
    <div class="flex items-center space-x-3.5">
      <img
        src="/img/logo_citadel.png"
        alt="Citadel"
        class="w-7 h-7 object-contain drop-shadow-[0_0_8px_rgba(235,225,200,0.4)]"
      />
      <div class="h-4 w-px bg-accent-secondary/20" />

      <div v-if="!isSessionStatusKnown" class="text-sm text-amber-300">
        {{ $t('dashboard.sessionUnavailable') }}
      </div>

      <div v-else-if="isSeedingActive && session.state === 'active'" class="flex items-center space-x-2.5 bg-emerald-500/10 border border-emerald-500/30 px-3 py-1.5 rounded-lg">
        <span class="w-2.5 h-2.5 rounded-full bg-emerald-400 animate-ping" />
        <span class="text-sm font-sans font-semibold text-emerald-300 uppercase tracking-wide">
          {{ $t('dashboard.activeSeed') }}: {{ session.server?.name || session.plan?.serverName || $t('common.unknown') }}
        </span>
        <span class="text-sm font-mono text-emerald-400/80">
          ({{ session.currentPlayerCount }}/{{ session.targetPlayerCount }})
        </span>
      </div>

      <div v-else-if="isSeedingActive" class="flex items-center space-x-2 text-sm text-amber-200 font-sans tracking-wide">
        <span>{{ $t(session.state === 'preparing' ? 'dashboard.statePreparing' : session.state === 'transitioning' ? 'dashboard.stateTransitioning' : 'dashboard.stateConnecting') }}: {{ session.server?.name || session.plan?.serverName }}</span>
      </div>

      <div v-else class="flex items-center space-x-2 text-sm text-primary/70 font-sans tracking-wide">
        <span class="w-2 h-2 rounded-full bg-accent-secondary/50" />
        <span class="uppercase font-medium">{{ $t('dashboard.idleStatus') }}</span>
      </div>
    </div>

    <!-- Right: Network / Fleet Telemetry & Language -->
    <div class="flex items-center space-x-5">
      <div class="flex items-center space-x-4 text-sm font-sans text-primary/80">
        <!-- Online Servers -->
        <div class="flex items-center space-x-1.5">
          <Server class="w-4 h-4 text-primary/50" />
          <span>{{ servers.length }} {{ $t('nav.fleet') }}</span>
        </div>

        <!-- Seeding Servers -->
        <div class="flex items-center space-x-1.5 text-squad-orange font-medium">
          <Flame class="w-4 h-4 text-squad-orange" />
          <span>{{ seedingServers.length }} {{ $t('fleet.onlySeeding') }}</span>
        </div>

        <!-- Online Players -->
        <div class="flex items-center space-x-1.5 text-accent font-medium">
          <Users class="w-4 h-4 text-accent" />
          <span>{{ totalOnlinePlayers }} {{ $t('common.players') }}</span>
        </div>
      </div>

      <!-- Language Selector -->
      <LanguageSelector />

      <!-- Refresh Servers Button -->
      <button
        type="button"
        :disabled="isLoading"
        class="p-1.5 rounded-md hover:bg-surface border border-accent-secondary/20 hover:border-accent text-primary/70 hover:text-accent transition-colors disabled:opacity-50"
        :title="$t('fleet.refreshFleet')"
        @click="fetchServers"
      >
        <RefreshCw :class="['w-4 h-4', isLoading ? 'animate-spin text-accent' : '']" />
      </button>
    </div>
  </header>
</template>
