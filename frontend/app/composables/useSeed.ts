import { storeToRefs } from 'pinia'
import { useSeedStore } from '~/stores/seed'
import { seedService } from '~/services/seedService'
import type { SeedPlan, SeedQueueItem, Server } from '~/types'

export const useSeed = () => {
  const seedStore = useSeedStore()
  const { session, queue, isLoading, error, sessionStatusError, isSessionStatusKnown } = storeToRefs(seedStore)

  const initBridgeListeners = () => seedStore.initBridgeListeners()
  const fetchCurrentSession = () => seedStore.fetchCurrentSession()
  const startSeed = (plan: SeedPlan, server?: Server | null) => seedStore.startSeed(plan, server)
  const stopSeed = () => seedStore.stopSeed()
  const joinServerDirect = (serverId: string) => seedStore.joinServerDirect(serverId)
  const addQueueItem = (item: SeedQueueItem) => seedStore.addQueueItem(item)
  const removeQueueItem = (id: string) => seedStore.removeQueueItem(id)
  const clearError = () => seedStore.clearError()
  const cancelPowerAction = () => seedStore.cancelPowerAction()
  const skipToNextServer = () => seedStore.skipToNextServer()
  const checkRunningSquadPotatoState = () => seedService.checkRunningSquadPotatoState()
  const restartSquadWithPotato = (plan: SeedPlan, server?: Server | null) => seedStore.restartSquadWithPotato(plan, server)

  return {
    session,
    queue,
    isLoading,
    error,
    sessionStatusError,
    isSessionStatusKnown,
    isSeedingActive: computed(() => seedStore.isSeedingActive),
    isSeedingPreparing: computed(() => seedStore.isSeedingPreparing),
    isSeedingConnecting: computed(() => seedStore.isSeedingConnecting),
    isSeedingIdle: computed(() => seedStore.isSeedingIdle),
    isSeedingFinished: computed(() => seedStore.isSeedingFinished),
    progressPercent: computed(() => seedStore.progressPercent),
    formattedElapsedTime: computed(() => seedStore.formattedElapsedTime),
    formattedRemainingTime: computed(() => seedStore.formattedRemainingTime),
    initBridgeListeners,
    disposeBridgeListeners: () => seedStore.disposeBridgeListeners(),
    fetchCurrentSession,
    startSeed,
    stopSeed,
    joinServerDirect,
    cancelPowerAction,
    skipToNextServer,
    checkRunningSquadPotatoState,
    restartSquadWithPotato,
    addQueueItem,
    removeQueueItem,
    clearError
  }
}
