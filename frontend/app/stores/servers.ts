import { defineStore } from 'pinia'
import { serverService } from '~/services/serverService'
import type { Server, ServerFilterOptions } from '~/types'

const requestGeneration = new WeakMap<object, number>()

export const useServersStore = defineStore('servers', {
  state: () => ({
    servers: [] as Server[],
    selectedServerId: null as string | null,
    isLoading: false,
    error: null as string | null,
    lastUpdated: null as string | null,
    filters: {
      search: '',
      onlySeeding: false,
      hideFull: false,
      hideEmpty: false
    } as ServerFilterOptions
  }),

  getters: {
    filteredServers: (state) => {
      return serverService.filterServers(state.servers, state.filters)
    },
    selectedServer: (state) => {
      if (!state.selectedServerId) return null
      return state.servers.find(s => s.id === state.selectedServerId) ?? null
    },
    seedingServers: (state) => {
      return state.servers.filter(s => s.isSeedingActive)
    },
    totalOnlinePlayers: (state) => {
      return state.servers.reduce((acc, s) => acc + s.currentPlayers, 0)
    },
    totalCapacity: (state) => {
      return state.servers.reduce((acc, s) => acc + s.maxPlayers, 0)
    }
  },

  actions: {
    async fetchServers() {
      const generation = (requestGeneration.get(this) ?? 0) + 1
      requestGeneration.set(this, generation)
      this.isLoading = true
      this.error = null

      try {
        const servers = await serverService.getServers()
        if (generation !== requestGeneration.get(this)) return
        this.servers = servers
        this.lastUpdated = new Date().toISOString()
      } catch (error: unknown) {
        if (generation === requestGeneration.get(this)) this.error = error instanceof Error ? error.message : String(error)
      } finally {
        if (generation === requestGeneration.get(this)) this.isLoading = false
      }
    },

    selectServer(id: string | null) {
      this.selectedServerId = id
    },

    updateFilters(partial: Partial<ServerFilterOptions>) {
      this.filters = { ...this.filters, ...partial }
    },

    resetFilters() {
      this.filters = {
        search: '',
        onlySeeding: false,
        hideFull: false,
        hideEmpty: false
      }
    }
  }
})
