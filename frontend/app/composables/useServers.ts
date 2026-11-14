import { storeToRefs } from 'pinia'
import { useServersStore } from '~/stores/servers'
import type { ServerFilterOptions } from '~/types'

export const useServers = () => {
  const serversStore = useServersStore()
  const {
    servers,
    selectedServerId,
    isLoading,
    error,
    lastUpdated,
    filters
  } = storeToRefs(serversStore)

  const fetchServers = () => serversStore.fetchServers()
  const selectServer = (id: string | null) => serversStore.selectServer(id)
  const updateFilters = (partial: Partial<ServerFilterOptions>) => serversStore.updateFilters(partial)
  const resetFilters = () => serversStore.resetFilters()

  return {
    servers,
    selectedServerId,
    isLoading,
    error,
    lastUpdated,
    filters,
    filteredServers: computed(() => serversStore.filteredServers),
    selectedServer: computed(() => serversStore.selectedServer),
    seedingServers: computed(() => serversStore.seedingServers),
    totalOnlinePlayers: computed(() => serversStore.totalOnlinePlayers),
    totalCapacity: computed(() => serversStore.totalCapacity),
    fetchServers,
    selectServer,
    updateFilters,
    resetFilters
  }
}
