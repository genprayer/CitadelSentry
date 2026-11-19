import { ref } from 'vue'
import { bridgeAdapter } from '~/adapters/wails/bridgeAdapter'
import type { UpdateInfo } from '~/types'

const updateInfo = ref<UpdateInfo | null>(null)
const isChecking = ref(false)
const isUpdating = ref(false)
const isModalOpen = ref(false)
const updateError = ref<string | null>(null)
const checkMessage = ref<string | null>(null)

export const useUpdater = () => {
  const checkForUpdates = async (silent = false): Promise<UpdateInfo | null> => {
    if (isChecking.value || !bridgeAdapter.isAvailable()) return null
    isChecking.value = true
    updateError.value = null
    checkMessage.value = null

    try {
      const info = await bridgeAdapter.checkForUpdates()
      updateInfo.value = info

      if (info && info.available) {
        isModalOpen.value = true
      } else if (!silent) {
        checkMessage.value = 'upToDate'
      }
      return info
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err)
      if (!silent) {
        updateError.value = msg
      }
      return null
    } finally {
      isChecking.value = false
    }
  }

  const applyUpdate = async (): Promise<boolean> => {
    if (!updateInfo.value?.latestVersion || isUpdating.value) return false
    isUpdating.value = true
    updateError.value = null

    try {
      await bridgeAdapter.installUpdate(updateInfo.value.latestVersion)
      return true
    } catch (err: unknown) {
      updateError.value = err instanceof Error ? err.message : String(err)
      isUpdating.value = false
      return false
    }
  }

  const dismissModal = () => {
    isModalOpen.value = false
  }

  return {
    updateInfo,
    isChecking,
    isUpdating,
    isModalOpen,
    updateError,
    checkMessage,
    checkForUpdates,
    applyUpdate,
    dismissModal
  }
}
