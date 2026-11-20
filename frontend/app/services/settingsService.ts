import { DEFAULT_APP_SETTINGS, settingsAdapter } from '~/adapters/wails/settingsAdapter'
import type { AppSettings } from '~/types'

export class SettingsService {
  public async getSettings(): Promise<AppSettings> {
    return settingsAdapter.getSettings()
  }

  public async saveSettings(settings: AppSettings): Promise<boolean> {
    return settingsAdapter.saveSettings(settings)
  }

  public async setTelemetryOptIn(optIn: boolean): Promise<boolean> {
    return settingsAdapter.setTelemetryOptIn(optIn)
  }

  public async restoreSquadConfig(): Promise<boolean> {
    return settingsAdapter.restoreSquadConfig()
  }

  public async saveCurrentSquadConfigAsBackup(): Promise<boolean> {
    return settingsAdapter.saveCurrentSquadConfigAsBackup()
  }

  public async getSquadConfigStatus(): Promise<import('~/types').SquadConfigStatus> {
    return settingsAdapter.getSquadConfigStatus()
  }

  public async getGPUInfo(): Promise<import('~/types').GPUInfo> {
    return settingsAdapter.getGPUInfo()
  }

  public async restoreInitialBaseline(): Promise<boolean> {
    return settingsAdapter.restoreInitialBaseline()
  }

  public async hasInitialBaseline(): Promise<boolean> {
    return settingsAdapter.hasInitialBaseline()
  }

  public async createRestorePoint(name: string, description: string): Promise<import('~/types').RestorePoint> {
    return settingsAdapter.createRestorePoint(name, description)
  }

  public async listRestorePoints(): Promise<import('~/types').RestorePoint[]> {
    return settingsAdapter.listRestorePoints()
  }

  public async restoreRestorePoint(id: string): Promise<boolean> {
    return settingsAdapter.restoreRestorePoint(id)
  }

  public async deleteRestorePoint(id: string): Promise<boolean> {
    return settingsAdapter.deleteRestorePoint(id)
  }

  public getDefaultSettings(): AppSettings {
    return JSON.parse(JSON.stringify(DEFAULT_APP_SETTINGS))
  }
}

export const settingsService = new SettingsService()
