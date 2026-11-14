import { defineStore } from 'pinia'
import { bridgeAdapter } from '~/adapters/wails/bridgeAdapter'
import type { WailsBridgeStatus } from '~/types'

const listenerCleanup = new WeakMap<object, () => void>()
const statusGeneration = new WeakMap<object, number>()

export const useBridgeStore = defineStore('bridge', {
  state: () => ({
    status: {
      isAvailable: false,
      platform: 'unknown' as const,
      appVersion: 'unknown',
      squadRunning: null,
      steamRunning: null,
      lastPingAt: null as string | null
    } as WailsBridgeStatus,
    isChecking: false
  }),

  getters: {
    isBridgeReady: (state) => state.status.isAvailable,
    isSquadRunning: (state) => state.status.squadRunning,
    isSteamRunning: (state) => state.status.steamRunning,
    platform: (state) => state.status.platform
  },

  actions: {
    async checkStatus() {
      if (this.isChecking) return
      const generation = statusGeneration.get(this) ?? 0
      this.isChecking = true
      try {
        const status = await bridgeAdapter.getStatus()
        if (generation === (statusGeneration.get(this) ?? 0)) this.status = status
      } catch (err) {
        console.error('Failed to check bridge status', err)
      } finally {
        this.isChecking = false
      }
    },

    initListeners() {
      if (listenerCleanup.has(this)) return
      const dispose = bridgeAdapter.onEvent<Record<string, any>>('bridge:status', (data) => {
        if (data) {
          statusGeneration.set(this, (statusGeneration.get(this) ?? 0) + 1)
          this.status = {
            ...this.status,
            isAvailable: true,
            platform: data.platform || this.status.platform,
            appVersion: String(data.appVersion || this.status.appVersion),
            squadRunning: typeof data.squadRunning === 'boolean' ? data.squadRunning : null,
            steamRunning: typeof data.steamRunning === 'boolean' ? data.steamRunning : null,
            processStatusError: typeof data.processStatusError === 'string' ? data.processStatusError : undefined,
            canSetLowCpuPriority: typeof data.canSetLowCpuPriority === 'boolean' ? data.canSetLowCpuPriority : undefined,
            lastPingAt: data.lastPingAt || new Date().toISOString(),
            logFilePath: data.logFilePath || this.status.logFilePath
          }
        }
      })
      listenerCleanup.set(this, dispose)
    },

    disposeListeners() {
      statusGeneration.set(this, (statusGeneration.get(this) ?? 0) + 1)
      const dispose = listenerCleanup.get(this)
      listenerCleanup.delete(this)
      dispose?.()
    },

    async openLogsFolder() {
      return await bridgeAdapter.openLogsFolder()
    },

    async getLogFilePath() {
      return await bridgeAdapter.getLogFilePath()
    },

    async getRecentLogs(limit = 100) {
      return await bridgeAdapter.getRecentLogs(limit)
    },

    async clearLogs() {
      await bridgeAdapter.clearLogs()
    },

    async openExternal(url: string) {
      await bridgeAdapter.openExternalUrl(url)
    },

    async minimize() {
      await bridgeAdapter.minimizeWindow()
    },

    async maximize() {
      await bridgeAdapter.maximizeWindow()
    },

    async close() {
      await bridgeAdapter.closeWindow()
    }
  }
})
