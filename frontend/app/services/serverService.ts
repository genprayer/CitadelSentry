import { serverAdapter } from '~/adapters/wails/serverAdapter'
import type { Server, ServerFilterOptions } from '~/types'

export class ServerService {
  public async getServers(): Promise<Server[]> {
    return serverAdapter.fetchServers()
  }

  public async getServer(id: string): Promise<Server> {
    return serverAdapter.getServerDetails(id)
  }

  public filterServers(servers: Server[], options: ServerFilterOptions): Server[] {
    const searchLower = options.search.trim().toLowerCase()

    return servers.filter(server => {
      // Search text filter
      if (searchLower) {
        const matchesName = server.name.toLowerCase().includes(searchLower)
        const matchesMap = server.currentMap.toLowerCase().includes(searchLower)
        const matchesLayer = server.currentLayer.toLowerCase().includes(searchLower)
        if (!matchesName && !matchesMap && !matchesLayer) {
          return false
        }
      }

      // Seeding only filter
      if (options.onlySeeding && !server.isSeedingActive) {
        return false
      }

      // Hide full filter
      if (options.hideFull && server.currentPlayers >= server.maxPlayers) {
        return false
      }

      // Hide empty filter
      if (options.hideEmpty && server.currentPlayers === 0) {
        return false
      }

      return true
    })
  }

  public isServerSeedable(server: Server): boolean {
    if (server.status === 'offline') return false
    if (server.currentPlayers >= server.maxPlayers) return false
    return true
  }
}

export const serverService = new ServerService()
