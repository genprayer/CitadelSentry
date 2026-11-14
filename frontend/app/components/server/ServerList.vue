<script setup lang="ts">
import { Flame, RefreshCw, X } from 'lucide-vue-next'
import type { Server } from '~/types'
import { useServers } from '~/composables/useServers'
import { useSeed } from '~/composables/useSeed'
import ServerCard from '~/components/server/ServerCard.vue'
import ServerDetailsModal from '~/components/server/ServerDetailsModal.vue'
import BaseInput from '~/components/ui/BaseInput.vue'

const emit = defineEmits<{
  seed: [server: Server]
  join: [server: Server]
}>()

const {
  servers,
  filteredServers,
  filters,
  isLoading,
  error,
  updateFilters,
  resetFilters,
  fetchServers
} = useServers()

const { session, isSeedingActive, stopSeed, error: seedError } = useSeed()
const isStopping = ref(false)

const handleStop = async () => {
  if (isStopping.value) return
  isStopping.value = true
  try { await stopSeed() } catch (error) { console.error('Could not stop seed mission', error) }
  finally { isStopping.value = false }
}

const activeDetailsServer = ref<Server | null>(null)
const showDetailsModal = ref(false)

const openDetails = (server: Server) => {
  activeDetailsServer.value = server
  showDetailsModal.value = true
}

const onSearchInput = (val: string | number) => {
  updateFilters({ search: String(val) })
}
</script>

<template>
  <div class="space-y-4 font-sans">
    <div v-if="seedError" role="alert" class="p-3 rounded-lg border border-rose-500/30 bg-rose-500/10 text-xs text-rose-300">{{ seedError }}</div>
    <div v-if="error" role="alert" class="p-3 rounded-lg border border-rose-500/30 bg-rose-500/10 text-xs text-rose-300">
      {{ error }}
    </div>
    <!-- Filter Toolbar -->
    <div class="bg-surface border border-accent-secondary/20 p-3 rounded-xl flex flex-wrap items-center justify-between gap-3">
      <!-- Search input -->
      <div class="w-full sm:w-72">
        <BaseInput
          :model-value="filters.search"
          :placeholder="$t('fleet.searchPlaceholder')"
          @update:model-value="onSearchInput"
        />
      </div>

      <!-- Filter Toggles & Actions -->
      <div class="flex items-center flex-wrap gap-2 text-xs font-mono">
        <button
          type="button"
          :class="[
            'px-3 py-1.5 rounded-lg border transition-colors flex items-center space-x-1.5',
            filters.onlySeeding
              ? 'bg-accent/20 text-accent border-accent/40 font-semibold'
              : 'bg-surface text-primary/60 border-accent-secondary/20 hover:text-primary'
          ]"
          @click="updateFilters({ onlySeeding: !filters.onlySeeding })"
        >
          <Flame class="w-3.5 h-3.5 text-squad-orange" />
          <span>{{ $t('fleet.onlySeeding') }}</span>
        </button>

        <button
          type="button"
          :class="[
            'px-3 py-1.5 rounded-lg border transition-colors',
            filters.hideFull
              ? 'bg-accent/20 text-accent border-accent/40 font-semibold'
              : 'bg-surface text-primary/60 border-accent-secondary/20 hover:text-primary'
          ]"
          @click="updateFilters({ hideFull: !filters.hideFull })"
        >
          {{ $t('fleet.hideFull') }}
        </button>

        <button
          type="button"
          :class="[
            'px-3 py-1.5 rounded-lg border transition-colors',
            filters.hideEmpty
              ? 'bg-accent/20 text-accent border-accent/40 font-semibold'
              : 'bg-surface text-primary/60 border-accent-secondary/20 hover:text-primary'
          ]"
          @click="updateFilters({ hideEmpty: !filters.hideEmpty })"
        >
          {{ $t('fleet.hideEmpty') }}
        </button>

        <button
          v-if="filters.search || filters.onlySeeding || filters.hideFull || filters.hideEmpty"
          type="button"
          class="px-2.5 py-1.5 rounded-lg text-primary/60 hover:text-rose-400 hover:bg-surface transition-colors flex items-center space-x-1"
          @click="resetFilters"
        >
          <X class="w-3.5 h-3.5" />
          <span>{{ $t('fleet.clearFilters') }}</span>
        </button>
      </div>
    </div>

    <!-- Server Cards Grid -->
    <div v-if="filteredServers.length > 0" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      <ServerCard
        v-for="server in filteredServers"
        :key="server.id"
        :server="server"
        :can-seed="true"
        :is-seeding-this-server="isSeedingActive && session.server?.id === server.id"
        :mission-state="session.state"
        :is-stopping="isStopping"
        :is-queued="isSeedingActive && !!session.plan?.serverIds?.includes(server.id) && session.server?.id !== server.id"
        :queue-order-number="(session.plan?.serverIds?.indexOf(server.id) ?? -1) + 1"
        :is-any-seeding-active="isSeedingActive"
        :mission-target="session.plan?.targetPlayerCount || session.targetPlayerCount"
        @seed="emit('seed', $event)"
        @join="emit('join', $event)"
        @stop="handleStop"
        @details="openDetails"
      />
    </div>

    <!-- Empty State -->
    <div
      v-else
      class="p-12 text-center bg-surface rounded-xl border border-accent-secondary/20 space-y-3"
    >
      <p class="text-sm font-semibold text-primary font-sans">
        {{ $t('fleet.noServersFound') }}
      </p>
      <button
        type="button"
        class="text-xs font-mono text-accent hover:underline inline-flex items-center space-x-1"
        @click="resetFilters(); fetchServers()"
      >
        <RefreshCw class="w-3 h-3 mr-1" />
        <span>{{ $t('fleet.refreshFleet') }}</span>
      </button>
    </div>

    <!-- Server Details Modal -->
    <ServerDetailsModal
      v-model="showDetailsModal"
      :server="activeDetailsServer"
      :can-seed="true"
      @seed="emit('seed', $event)"
      @join="emit('join', $event)"
    />
  </div>
</template>
