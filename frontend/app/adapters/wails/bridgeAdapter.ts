import type { UpdateInfo, WailsBridgeStatus } from '~/types'
import { BridgeUnavailableError } from './errors'

export class WailsBridgeAdapter {
  public isAvailable(): boolean {
    if (typeof window === 'undefined') return false
    return Boolean(window.go?.main?.App)
  }

  public async checkForUpdates(): Promise<UpdateInfo> {
    if (!this.isAvailable()) throw new Error('Desktop bridge is unavailable')
    return await window.go!.main!.App!.CheckForUpdates()
  }

  public async installUpdate(version: string): Promise<void> {
    if (!this.isAvailable()) throw new Error('Desktop bridge is unavailable')
    await window.go!.main!.App!.InstallUpdate(version)
  }

  public async getUpdateLastError(): Promise<string> {
    if (!this.isAvailable()) return ''
    return await window.go!.main!.App!.GetUpdateLastError()
  }

  public async getStatus(): Promise<WailsBridgeStatus> {
    if (!this.isAvailable()) {
      return {
        isAvailable: false,
        platform: 'browser',
        appVersion: 'browser-preview',
        squadRunning: null,
        steamRunning: null,
        lastPingAt: new Date().toISOString()
      }
    }

    try {
      const app = window.go?.main?.App
      let status: any = null
      if (app?.GetBridgeStatus) {
        status = await app.GetBridgeStatus()
      } else if (app?.GetSystemStatus) {
        status = await app.GetSystemStatus()
      }

      if (status) {
        return {
          ...status,
          isAvailable: true,
          squadRunning: typeof status.squadRunning === 'boolean' ? status.squadRunning : null,
          steamRunning: typeof status.steamRunning === 'boolean' ? status.steamRunning : null,
          canSetLowCpuPriority: typeof status.canSetLowCpuPriority === 'boolean' ? status.canSetLowCpuPriority : undefined,
          lastPingAt: new Date().toISOString()
        }
      }
      throw new Error('Empty bridge status')
    } catch (error) {
      return {
        isAvailable: false,
        platform: 'unknown',
        appVersion: 'unknown',
        squadRunning: null,
        steamRunning: null,
        processStatusError: String(error),
        lastPingAt: null
      }
    }
  }

  public async openLogsFolder(): Promise<string> {
    if (typeof window === 'undefined') return ''
    const app = window.go?.main?.App
    if (app?.OpenLogsFolder) {
      return await app.OpenLogsFolder()
    }
    return ''
  }

  public async getLogFilePath(): Promise<string> {
    if (typeof window === 'undefined') return ''
    const app = window.go?.main?.App
    if (app?.GetLogFilePath) {
      return await app.GetLogFilePath()
    }
    return ''
  }

  public async getRecentLogs(limit = 100): Promise<string[]> {
    const app = typeof window === 'undefined' ? undefined : window.go?.main?.App
    if (!app?.GetRecentLogs) throw new BridgeUnavailableError('Log reading requires the desktop runtime.')
    const lines: unknown = await app.GetRecentLogs(limit)
    // Go's nil []string is serialized as null for an empty native journal.
    if (lines === null) return []
    if (!Array.isArray(lines) || lines.some(line => typeof line !== 'string')) {
      throw new Error('Native log lines must be an array of strings')
    }
    return lines
  }

  public async clearLogs(): Promise<void> {
    const app = typeof window === 'undefined' ? undefined : window.go?.main?.App
    if (!app?.ClearLogs) throw new BridgeUnavailableError('Log clearing requires the desktop runtime.')
    await app.ClearLogs()
  }

  public async openExternalUrl(url: string): Promise<void> {
    if (typeof window === 'undefined') return

    if (window.runtime?.BrowserOpenURL) {
      window.runtime.BrowserOpenURL(url)
      return
    }

    const app = window.go?.main?.App
    if (app?.OpenExternal) {
      await app.OpenExternal(url)
      return
    } else if (app?.OpenExternalUrl) {
      await app.OpenExternalUrl(url)
      return
    }

    // Browser fallback
    window.open(url, '_blank', 'noopener,noreferrer')
  }

  public async minimizeWindow(): Promise<void> {

    if (window.runtime?.WindowMinimise) {
      window.runtime.WindowMinimise()
    } else if (window.go?.main?.App?.MinimizeWindow) {
      await window.go.main.App.MinimizeWindow()
    }
  }

  public async maximizeWindow(): Promise<void> {
    if (window.runtime?.WindowToggleMaximise) {
      window.runtime.WindowToggleMaximise()
    } else if (window.go?.main?.App?.MaximizeWindow) {
      await window.go.main.App.MaximizeWindow()
    }
  }

  public async closeWindow(): Promise<void> {
    if (window.runtime?.Quit) {
      window.runtime.Quit()
    } else if (window.go?.main?.App?.CloseWindow) {
      await window.go.main.App.CloseWindow()
    }
  }

  public onEvent<T = any>(eventName: string, callback: (data: T) => void): () => void {
    if (typeof window !== 'undefined' && window.runtime?.EventsOn) {
      return window.runtime.EventsOn(eventName, callback)
    }
    if (typeof window !== 'undefined') {
      const handler = (e: any) => callback(e.detail)
      window.addEventListener(eventName, handler)
      return () => window.removeEventListener(eventName, handler)
    }
    return () => {}
  }
}

export const bridgeAdapter = new WailsBridgeAdapter()
