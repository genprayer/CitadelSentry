<script setup lang="ts">
import { Flame, ArrowRight, AlertCircle, X } from 'lucide-vue-next'
import type { Server } from '~/types'
import { useServers } from '~/composables/useServers'
import { useSeed } from '~/composables/useSeed'
import ActiveSeedCard from '~/components/seed/ActiveSeedCard.vue'
import SeedConfigModal from '~/components/seed/SeedConfigModal.vue'
import ServerCard from '~/components/server/ServerCard.vue'
import ConfirmJoinModal from '~/components/server/ConfirmJoinModal.vue'
import { useI18n } from '~/composables/useI18n'

const router = useRouter()
const { servers, seedingServers, error: fleetError, isLoading: isFleetLoading, fetchServers } = useServers()
const { isSeedingActive, session, joinServerDirect, stopSeed } = useSeed()
const { t } = useI18n()

const selectedConfigServer = ref<Server | null>(null)
const showConfigModal = ref(false)
const joinError = ref<string | null>(null)
const isJoining = ref(false)
const pendingJoinServer = ref<Server | null>(null)
const showConfirmJoinModal = ref(false)

// Priority targets: servers needing seeding first, then servers with low player counts
const priorityTargets = computed(() => {
  if (seedingServers.value.length > 0) {
    return seedingServers.value.slice(0, 6)
  }
  return servers.value.slice(0, 6)
})

const handleOpenSeed = (server: Server) => {
  selectedConfigServer.value = server
  showConfigModal.value = true
}

const executeDirectJoin = async (server: Server) => {
  joinError.value = null
  isJoining.value = true
  try {
    await joinServerDirect(server.id)
    showConfirmJoinModal.value = false
  } catch (err: any) {
    console.error('Failed to direct join server', err)
    joinError.value = err?.message || t('dashboard.joinErrorFallback')
  } finally {
    isJoining.value = false
  }
}

const handleJoin = async (server: Server) => {
  if (isSeedingActive.value) {
    pendingJoinServer.value = server
    showConfirmJoinModal.value = true
    return
  }
  await executeDirectJoin(server)
}

const navigateToServers = () => {
  router.push('/servers')
}
</script>

<template>
  <div class="space-y-6 max-w-6xl mx-auto">
    <div
      v-if="joinError"
      class="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-start justify-between space-x-3 text-rose-300 text-sm font-mono animate-fade-in"
    >
      <div class="flex items-start space-x-2.5">
        <AlertCircle class="w-5 h-5 text-rose-400 flex-shrink-0 mt-0.5" />
        <div>
          <p class="font-bold font-sans">{{ $t('dashboard.joinErrorTitle') }}</p>
          <p class="text-xs text-rose-300/80 mt-0.5">{{ joinError }}</p>
        </div>
      </div>
      <button
        type="button"
        class="p-1 hover:bg-rose-500/20 rounded-lg transition-colors text-rose-400"
        :aria-label="$t('common.close')"
        @click="joinError = null"
      >
        <X class="w-4 h-4" />
      </button>
    </div>

    <!-- Active Seeding Operational HUD -->
    <ActiveSeedCard @select-server="navigateToServers" />

    <!-- Priority Seeding Fleet Targets -->
    <div class="space-y-4">
      <div v-if="fleetError" role="alert" class="p-3 rounded-lg border border-rose-500/30 bg-rose-500/10 text-xs text-rose-300 space-y-2">
        <p>{{ fleetError }}</p>
        <button type="button" :disabled="isFleetLoading" class="px-3 py-1 rounded border border-rose-500/30 hover:bg-rose-500/10 disabled:opacity-50" @click="fetchServers">
          {{ $t('common.retry') }}
        </button>
      </div>
      <div class="flex items-center justify-between pb-1 border-b border-accent-secondary/15">
        <div class="flex items-center space-x-2 font-sans">
          <Flame class="w-4 h-4 text-squad-orange" />
          <h3 class="text-xs font-bold uppercase tracking-wider text-primary">
            {{ $t('dashboard.priorityTargets') }}
          </h3>
        </div>

        <NuxtLink
          to="/servers"
          class="text-xs font-sans text-accent hover:text-white flex items-center space-x-1 uppercase tracking-wider transition-colors"
        >
          <span>{{ $t('dashboard.viewAllServers') }}</span>
          <ArrowRight class="w-3.5 h-3.5" />
        </NuxtLink>
      </div>

      <div v-if="priorityTargets.length > 0" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <ServerCard
          v-for="server in priorityTargets"
          :key="server.id"
          :server="server"
          :can-seed="true"
          :is-seeding-this-server="isSeedingActive && session.server?.id === server.id"
          :mission-state="session.state"
          :is-queued="isSeedingActive && !!session.plan?.serverIds?.includes(server.id) && session.server?.id !== server.id"
          :queue-order-number="(session.plan?.serverIds?.indexOf(server.id) ?? -1) + 1"
          :is-any-seeding-active="isSeedingActive"
          :mission-target="session.plan?.targetPlayerCount || session.targetPlayerCount"
          @seed="handleOpenSeed"
          @join="handleJoin"
          @stop="stopSeed"
          @details="handleOpenSeed"
        />
      </div>

      <div
        v-else
        class="p-8 bg-surface rounded-xl border border-accent-secondary/15 text-center text-xs font-sans text-primary/50"
      >
        {{ $t('fleet.noServersFound') }}
      </div>
    </div>

    <!-- Seed Configuration Modal -->
    <SeedConfigModal
      v-model="showConfigModal"
      :server="selectedConfigServer"
    />

    <!-- Safe Join Confirmation Modal -->
    <ConfirmJoinModal
      v-model="showConfirmJoinModal"
      :active-server-name="session.server?.name || 'текущий'"
      :target-server="pendingJoinServer"
      :is-loading="isJoining"
      @confirm="pendingJoinServer && executeDirectJoin(pendingJoinServer)"
    />
  </div>
</template>
