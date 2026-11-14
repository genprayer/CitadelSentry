import { defineStore } from 'pinia'
import { settingsService } from '~/services/settingsService'
import type { AppSettings, SquadConfigStatus } from '~/types'

const configStatusGeneration = new WeakMap<object, number>()

export const useSettingsStore = defineStore('settings', {
  state: () => ({
    settings: settingsService.getDefaultSettings(),
    draftSettings: settingsService.getDefaultSettings(),
    saveSuccess: false,
    isLoading: false,
    isSaving: false,
    isLoaded: false,
    error: null as string | null,
    configStatus: null as SquadConfigStatus | null,
    configStatusError: null as string | null,
    isRestoringConfig: false,
    isSavingBackup: false,
    gpuInfo: null as import('~/types').GPUInfo | null,
    restorePoints: [] as import('~/types').RestorePoint[],
    isRestoringBaseline: false,
    isCreatingRestorePoint: false
  }),

  getters: {
    hasUnsavedChanges: (state) => {
      return JSON.stringify(state.draftSettings) !== JSON.stringify(state.settings)
    }
  },

  actions: {
    async loadSettings() {
      this.isLoading = true
      this.isLoaded = false
      this.error = null

      try {
        this.settings = await settingsService.getSettings()
        this.draftSettings = JSON.parse(JSON.stringify(this.settings))
        this.isLoaded = true
        await Promise.all([
          this.fetchSquadConfigStatus(),
          this.fetchGPUInfo(),
          this.fetchRestorePoints()
        ])
      } catch (err: any) {
        this.error = err.message || 'Failed to load application settings'
      } finally {
        this.isLoading = false
      }
    },

    syncDraft() {
      this.draftSettings = JSON.parse(JSON.stringify(this.settings))
    },

    resetDraft() {
      this.draftSettings = settingsService.getDefaultSettings()
    },

    async saveDraft() {
      await this.saveSettings(this.draftSettings)
      this.saveSuccess = true
    },

    async saveSettings(updated: AppSettings) {
      if (!this.isLoaded) throw new Error('Load the saved application settings before changing them')
      this.isSaving = true
      this.error = null

      try {
        const success = await settingsService.saveSettings(updated)
        if (success) {
          this.settings = await settingsService.getSettings()
          this.draftSettings = JSON.parse(JSON.stringify(this.settings))
          await this.fetchSquadConfigStatus()
        } else {
          throw new Error('Could not persist settings')
        }
      } catch (err: any) {
        this.error = err.message || 'Failed to save settings'
        throw err
      } finally {
        this.isSaving = false
      }
    },

    async fetchSquadConfigStatus() {
      const generation = (configStatusGeneration.get(this) ?? 0) + 1
      configStatusGeneration.set(this, generation)
      try {
        const status = await settingsService.getSquadConfigStatus()
        if (generation !== configStatusGeneration.get(this)) return
        this.configStatus = status
        this.configStatusError = null
      } catch (err: unknown) {
        if (generation !== configStatusGeneration.get(this)) return
        this.configStatus = null
        this.configStatusError = err instanceof Error ? err.message : String(err)
      }
    },

    async restoreSquadConfig(): Promise<boolean> {
      this.isRestoringConfig = true
      try {
        const ok = await settingsService.restoreSquadConfig()
        await this.fetchSquadConfigStatus()
        return ok
      } finally {
        this.isRestoringConfig = false
      }
    },

    async saveCurrentSquadConfigAsBackup(): Promise<boolean> {
      this.isSavingBackup = true
      try {
        const ok = await settingsService.saveCurrentSquadConfigAsBackup()
        await this.fetchSquadConfigStatus()
        return ok
      } finally {
        this.isSavingBackup = false
      }
    },

    async fetchGPUInfo() {
      try {
        this.gpuInfo = await settingsService.getGPUInfo()
      } catch (err: unknown) {
        console.warn('Failed to fetch GPU info', err)
      }
    },

    async restoreInitialBaseline(): Promise<boolean> {
      this.isRestoringBaseline = true
      try {
        const ok = await settingsService.restoreInitialBaseline()
        await this.fetchSquadConfigStatus()
        return ok
      } finally {
        this.isRestoringBaseline = false
      }
    },

    async fetchRestorePoints() {
      try {
        const points = await settingsService.listRestorePoints()
        this.restorePoints = Array.isArray(points) ? points : []
      } catch (err: unknown) {
        console.warn('Failed to fetch restore points', err)
        this.restorePoints = []
      }
    },

    async createRestorePoint(name: string, description: string): Promise<import('~/types').RestorePoint> {
      this.isCreatingRestorePoint = true
      try {
        const rp = await settingsService.createRestorePoint(name, description)
        await this.fetchRestorePoints()
        return rp
      } finally {
        this.isCreatingRestorePoint = false
      }
    },

    async restoreRestorePoint(id: string): Promise<boolean> {
      this.isRestoringConfig = true
      try {
        const ok = await settingsService.restoreRestorePoint(id)
        await this.fetchSquadConfigStatus()
        return ok
      } finally {
        this.isRestoringConfig = false
      }
    },

    async deleteRestorePoint(id: string): Promise<boolean> {
      try {
        const ok = await settingsService.deleteRestorePoint(id)
        await this.fetchRestorePoints()
        return ok
      } catch (err: unknown) {
        console.warn('Failed to delete restore point', err)
        throw err
      }
    },

    resetToDefaults() {
      this.settings = settingsService.getDefaultSettings()
    }
  }
})
