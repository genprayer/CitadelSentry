import { storeToRefs } from 'pinia'
import { useBridgeStore } from '~/stores/bridge'

export const useBridge = () => {
  const bridgeStore = useBridgeStore()
  const { status, isChecking } = storeToRefs(bridgeStore)

  const checkStatus = () => bridgeStore.checkStatus()
  const minimize = () => bridgeStore.minimize()
  const maximize = () => bridgeStore.maximize()
  const close = () => bridgeStore.close()
  const openExternal = (url: string) => bridgeStore.openExternal(url)

  return {
    status,
    isChecking,
    isBridgeReady: computed(() => bridgeStore.isBridgeReady),
    isSquadRunning: computed(() => bridgeStore.isSquadRunning),
    isSteamRunning: computed(() => bridgeStore.isSteamRunning),
    platform: computed(() => bridgeStore.platform),
    checkStatus,
    initBridgeListeners: () => bridgeStore.initListeners(),
    disposeBridgeListeners: () => bridgeStore.disposeListeners(),
    minimize,
    maximize,
    close,
    openExternal,
    openLogsFolder: () => bridgeStore.openLogsFolder(),
    getLogFilePath: () => bridgeStore.getLogFilePath(),
    getRecentLogs: (limit?: number) => bridgeStore.getRecentLogs(limit),
    clearLogs: () => bridgeStore.clearLogs()
  }
}
