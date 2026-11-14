import type { Server } from '~/types'
import { bridgeAdapter } from './bridgeAdapter'
import { BridgeUnavailableError } from './errors'
import { settingsAdapter } from './settingsAdapter'
import { mapPublicServerFleet, validateServerFleet } from '~/utils/server-validation'

export class WailsServerAdapter {
  public async fetchServers(): Promise<Server[]> {
    if (bridgeAdapter.isAvailable() && window.go?.main?.App) {
      if (typeof window.go.main.App.FetchServers === 'function') {
        return validateServerFleet(await window.go.main.App.FetchServers())
      }
      if (typeof window.go.main.App.GetServers === 'function') {
        return validateServerFleet(await window.go.main.App.GetServers())
      }
      throw new BridgeUnavailableError('The desktop server API is unavailable.')
    }

    try {
      const settings = await settingsAdapter.getSettings()
      const seedTarget = settings.seed.defaultTargetPlayers
      const apiBaseUrl = settings.webApiBaseUrl.replace(/\/$/, '')
      const response = await fetch(`${apiBaseUrl}/api/servers/sentry`, {
        headers: { Accept: 'application/json' }
      })
      if (!response.ok) throw new Error(`HTTP ${response.status}`)
      return mapPublicServerFleet(await response.json(), seedTarget)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      throw new Error(`Failed to load server fleet: ${message}`)
    }
  }

  public async getServerDetails(serverId: string): Promise<Server> {
    const servers = await this.fetchServers()
    const server = servers.find(s => s.id === serverId)
    if (!server) {
      throw new Error(`Server with ID ${serverId} not found.`)
    }
    return server
  }
}

export const serverAdapter = new WailsServerAdapter()
