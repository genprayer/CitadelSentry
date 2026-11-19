import type { AppSettings } from '~/types'
import { bridgeAdapter } from './bridgeAdapter'
import { BridgeUnavailableError } from './errors'
import { validateAppSettings, validateSquadConfigStatus } from '~/utils/settings-validation'

export const DEFAULT_APP_SETTINGS: AppSettings = {
  general: {
    launchOnStartup: false,
    minimizeToTray: true,
    closeToTray: true,
    notificationsEnabled: true,
    soundEnabled: true,
    language: 'ru',
    autoUpdateCheck: true
  },
  seed: {
    defaultTargetPlayers: 50,
    defaultMaxDurationMinutes: 60,
    defaultPostSeedAction: 'do_nothing'
  },
  power: {
    preventSleepDuringSeed: true
  },
  potato: {
    enabled: true,
    fpsLimit: 1,
    lowResolution: false,
    lowCpuPriority: true,
    autoRestoreOnExit: true
  },
  webApiBaseUrl: 'https://jointhectl.com'
}


const STORAGE_KEY = 'citadel_seeder_settings_fallback'

export class WailsSettingsAdapter {
  public async getSettings(): Promise<AppSettings> {
    if (bridgeAdapter.isAvailable()) {
      return validateAppSettings(await window.go!.main!.App!.GetSettings())
    }

    // Local fallback for standalone browser preview
    if (typeof window !== 'undefined') {
      const stored = localStorage.getItem(STORAGE_KEY)
      if (stored) {
        const parsed: unknown = JSON.parse(stored)
        return validateAppSettings(parsed)
      }
    }

    return {
      ...DEFAULT_APP_SETTINGS,
      general: { ...DEFAULT_APP_SETTINGS.general },
      seed: { ...DEFAULT_APP_SETTINGS.seed },
      power: { ...DEFAULT_APP_SETTINGS.power },
      potato: { ...DEFAULT_APP_SETTINGS.potato! }
    }
  }

  public async saveSettings(settings: AppSettings): Promise<boolean> {
    validateAppSettings(settings)
    if (bridgeAdapter.isAvailable()) {
      return await window.go!.main!.App!.SaveSettings(settings)
    }

    // Local fallback for standalone browser preview
    if (typeof window !== 'undefined') {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(settings))
      return true
    }

    return false
  }

  public async restoreSquadConfig(): Promise<boolean> {
    if (!bridgeAdapter.isAvailable()) throw new BridgeUnavailableError('Squad settings restoration requires the desktop runtime.')
    return await window.go!.main!.App!.RestoreSquadConfig()
  }

  public async saveCurrentSquadConfigAsBackup(): Promise<boolean> {
    if (!bridgeAdapter.isAvailable()) throw new BridgeUnavailableError('Squad settings backup requires the desktop runtime.')
    return await window.go!.main!.App!.SaveCurrentSquadConfigAsBackup()
  }

  public async getSquadConfigStatus(): Promise<import('~/types').SquadConfigStatus> {
    if (!bridgeAdapter.isAvailable()) throw new BridgeUnavailableError('Squad config status requires the desktop runtime.')
    return validateSquadConfigStatus(await window.go!.main!.App!.GetSquadConfigStatus())
  }

  public async getGPUInfo(): Promise<import('~/types').GPUInfo> {
    if (!bridgeAdapter.isAvailable()) {
      return { vendorId: 0, vendor: 'unknown', model: 'Standard Graphics Adapter', dedicatedVram: 0, isRtx: false }
    }
    return await window.go!.main!.App!.GetGPUInfo()
  }

  public async restoreInitialBaseline(): Promise<boolean> {
    if (!bridgeAdapter.isAvailable()) throw new BridgeUnavailableError('Squad settings restoration requires the desktop runtime.')
    return await window.go!.main!.App!.RestoreInitialBaseline()
  }

  public async hasInitialBaseline(): Promise<boolean> {
    if (!bridgeAdapter.isAvailable()) return false
    return await window.go!.main!.App!.HasInitialBaseline()
  }

  public async createRestorePoint(name: string, description: string): Promise<import('~/types').RestorePoint> {
    if (!bridgeAdapter.isAvailable()) throw new BridgeUnavailableError('Restore point creation requires the desktop runtime.')
    return await window.go!.main!.App!.CreateRestorePoint(name, description)
  }

  public async listRestorePoints(): Promise<import('~/types').RestorePoint[]> {
    if (!bridgeAdapter.isAvailable()) return []
    const points = await window.go!.main!.App!.ListRestorePoints()
    return Array.isArray(points) ? points : []
  }

  public async restoreRestorePoint(id: string): Promise<boolean> {
    if (!bridgeAdapter.isAvailable()) throw new BridgeUnavailableError('Restore point recovery requires the desktop runtime.')
    return await window.go!.main!.App!.RestoreRestorePoint(id)
  }

  public async deleteRestorePoint(id: string): Promise<boolean> {
    if (!bridgeAdapter.isAvailable()) throw new BridgeUnavailableError('Restore point deletion requires the desktop runtime.')
    return await window.go!.main!.App!.DeleteRestorePoint(id)
  }
}

export const settingsAdapter = new WailsSettingsAdapter()
