<script setup lang="ts">
import { Server as ServerIcon, RefreshCw, AlertCircle, X } from 'lucide-vue-next'
import type { Server } from '~/types'
import { useSeed } from '~/composables/useSeed'
import { useServers } from '~/composables/useServers'
import ServerList from '~/components/server/ServerList.vue'
import SeedConfigModal from '~/components/seed/SeedConfigModal.vue'
import ConfirmJoinModal from '~/components/server/ConfirmJoinModal.vue'

const { joinServerDirect, isSeedingActive, session } = useSeed()
const { fetchServers, isLoading } = useServers()

const selectedConfigServer = ref<Server | null>(null)
const showConfigModal = ref(false)
const joinError = ref<string | null>(null)
const isJoining = ref(false)
const pendingJoinServer = ref<Server | null>(null)
const showConfirmJoinModal = ref(false)

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
    joinError.value = err?.message || 'Не удалось подключиться к серверу'
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
</script>

<template>
  <div class="space-y-6 max-w-7xl mx-auto">
    <!-- Join Error Alert -->
    <div
      v-if="joinError"
      class="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-start justify-between space-x-3 text-rose-300 text-sm font-mono animate-fade-in"
    >
      <div class="flex items-start space-x-2.5">
        <AlertCircle class="w-5 h-5 text-rose-400 flex-shrink-0 mt-0.5" />
        <div>
          <p class="font-bold font-sans">Ошибка подключения</p>
          <p class="text-xs text-rose-300/80 mt-0.5">{{ joinError }}</p>
        </div>
      </div>
      <button
        type="button"
        class="p-1 hover:bg-rose-500/20 rounded-lg transition-colors text-rose-400"
        @click="joinError = null"
      >
        <X class="w-4 h-4" />
      </button>
    </div>

    <!-- Page Header -->
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-lg font-bold font-sans tracking-wide uppercase text-primary flex items-center space-x-2">
          <ServerIcon class="w-5 h-5 text-accent" />
          <span>{{ $t('fleet.title') }}</span>
        </h2>
        <p class="text-xs text-primary/60 mt-0.5">
          {{ $t('fleet.subtitle') }}
        </p>
      </div>

      <button
        type="button"
        :disabled="isLoading"
        class="px-3 py-1.5 rounded-lg bg-surface hover:bg-surface/80 text-primary hover:text-accent text-xs font-sans uppercase border border-accent-secondary/20 transition-colors flex items-center space-x-1.5 disabled:opacity-50"
        @click="fetchServers"
      >
        <RefreshCw :class="['w-3.5 h-3.5', isLoading ? 'animate-spin text-accent' : '']" />
        <span>{{ $t('fleet.refreshFleet') }}</span>
      </button>
    </div>

    <!-- Server List & Filters -->
    <ServerList
      @seed="handleOpenSeed"
      @join="handleJoin"
    />

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
