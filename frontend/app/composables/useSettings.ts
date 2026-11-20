import { storeToRefs } from 'pinia'
import { useSettingsStore } from '~/stores/settings'
import type { AppSettings } from '~/types'

export const useSettings = () => {
  const settingsStore = useSettingsStore()
  const {
    settings,
    draftSettings,
    hasUnsavedChanges,
    saveSuccess,
    isLoading,
    isSaving,
    isLoaded,
    error,
    configStatus,
    configStatusError,
    isRestoringConfig,
    isSavingBackup,
    gpuInfo,
    restorePoints,
    isRestoringBaseline,
    isCreatingRestorePoint
  } = storeToRefs(settingsStore)

  const loadSettings = () => settingsStore.loadSettings()
  const saveSettings = (updated: AppSettings) => settingsStore.saveSettings(updated)
  const saveDraft = () => settingsStore.saveDraft()
  const resetDraft = () => settingsStore.resetDraft()
  const syncDraft = () => settingsStore.syncDraft()
  const resetToDefaults = () => settingsStore.resetToDefaults()
  const restoreSquadConfig = () => settingsStore.restoreSquadConfig()
  const saveCurrentSquadConfigAsBackup = () => settingsStore.saveCurrentSquadConfigAsBackup()
  const fetchSquadConfigStatus = () => settingsStore.fetchSquadConfigStatus()
  const fetchGPUInfo = () => settingsStore.fetchGPUInfo()
  const restoreInitialBaseline = () => settingsStore.restoreInitialBaseline()
  const fetchRestorePoints = () => settingsStore.fetchRestorePoints()
  const createRestorePoint = (name: string, description: string) => settingsStore.createRestorePoint(name, description)
  const restoreRestorePoint = (id: string) => settingsStore.restoreRestorePoint(id)
  const deleteRestorePoint = (id: string) => settingsStore.deleteRestorePoint(id)
  const setTelemetryOptIn = (optIn: boolean) => settingsStore.setTelemetryOptIn(optIn)

  return {
    settings,
    draftSettings,
    hasUnsavedChanges,
    saveSuccess,
    isLoading,
    isSaving,
    isLoaded,
    error,
    configStatus,
    configStatusError,
    isRestoringConfig,
    isSavingBackup,
    gpuInfo,
    restorePoints,
    isRestoringBaseline,
    isCreatingRestorePoint,
    loadSettings,
    saveSettings,
    saveDraft,
    resetDraft,
    syncDraft,
    resetToDefaults,
    restoreSquadConfig,
    saveCurrentSquadConfigAsBackup,
    fetchSquadConfigStatus,
    fetchGPUInfo,
    restoreInitialBaseline,
    fetchRestorePoints,
    createRestorePoint,
    restoreRestorePoint,
    deleteRestorePoint,
    setTelemetryOptIn
  }
}
