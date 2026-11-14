import { defineStore } from 'pinia'
import { bridgeAdapter } from '~/adapters/wails/bridgeAdapter'
import { seedService } from '~/services/seedService'
import { BRIDGE_EVENTS } from '~/types/bridge'
import type { ActiveSeedSession, SeedPlan, SeedQueueItem, Server } from '~/types'

import { formatSecondsToHms } from '~/utils/time'
import { validateSeedSession, validateSeedProgress } from '~/utils/seed-validation'

const listenerCleanup = new WeakMap<object, Array<() => void>>()
const sessionGeneration = new WeakMap<object, number>()
let lastDirectJoinTime = 0
let lastDirectJoinServerId = ''

const initialSession: ActiveSeedSession = {
  revision: 0,
  state: 'idle',
  plan: null,
  server: null,
  initialPlayerCount: 0,
  currentPlayerCount: 0,
  targetPlayerCount: 0,
  queuePosition: 0,
  elapsedSeconds: 0,
  remainingSeconds: null,
  startedAt: null,
  completedAt: null,
  errorMessage: null
}

export const useSeedStore = defineStore('seed', {
  state: () => ({
    session: { ...initialSession } as ActiveSeedSession,
    queue: [] as SeedQueueItem[],
    isLoading: false,
    error: null as string | null,
    sessionStatusError: null as string | null,
    isSessionStatusKnown: false,
    isListenerInitialized: false
  }),

  getters: {
    isSeedingActive: (state) => ['preparing', 'transitioning', 'connecting', 'active'].includes(state.session.state),
    isSeedingPreparing: (state) => state.session.state === 'preparing',
    isSeedingConnecting: (state) => state.session.state === 'connecting',
    isSeedingIdle: (state) => state.isSessionStatusKnown && state.session.state === 'idle',
    isSeedingFinished: (state) => ['completed', 'timed_out', 'stopped', 'failed'].includes(state.session.state),
    progressPercent: (state) => {
      const { initialPlayerCount, currentPlayerCount, targetPlayerCount } = state.session
      if (!targetPlayerCount) return 0
      if (currentPlayerCount >= targetPlayerCount) return 100
      if (targetPlayerCount <= initialPlayerCount) return 0
      const currentGain = Math.max(0, currentPlayerCount - initialPlayerCount)
      const targetGain = targetPlayerCount - initialPlayerCount
      return Math.min(100, Math.round((currentGain / targetGain) * 100))
    },
    formattedElapsedTime: (state) => {
      return formatSecondsToHms(state.session.elapsedSeconds)
    },
    formattedRemainingTime: (state) => {
      if (state.session.remainingSeconds === null || state.session.remainingSeconds === undefined) {
        return null
      }
      return formatSecondsToHms(state.session.remainingSeconds)
    }
  },

  actions: {
    applySession(value: unknown) {
      const currentRevision = this.session.revision
      if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Seed session must be an object')
      const incomingRevision = value && typeof value === 'object' && !Array.isArray(value)
        ? (value as Record<string, unknown>).revision : undefined
      if (currentRevision !== undefined && currentRevision > 0 &&
        (incomingRevision === undefined || (typeof incomingRevision === 'number' && Number.isSafeInteger(incomingRevision) && incomingRevision >= 0 && incomingRevision < currentRevision))) return false
      const updatedSession = validateSeedSession(value)
      this.session = { ...updatedSession }
      const previousError = this.sessionStatusError
      this.sessionStatusError = null
      if (this.error === previousError) this.error = null
      this.isSessionStatusKnown = true
      sessionGeneration.set(this, (sessionGeneration.get(this) ?? 0) + 1)
      return true
    },

    recordSessionFailure(error: unknown) {
      this.sessionStatusError = String(error)
      this.error = this.sessionStatusError
      this.isSessionStatusKnown = false
      sessionGeneration.set(this, (sessionGeneration.get(this) ?? 0) + 1)
    },

    initBridgeListeners() {
      if (this.isListenerInitialized) return

      const disposeState = bridgeAdapter.onEvent<ActiveSeedSession>(BRIDGE_EVENTS.SEED_STATE_CHANGED, (updatedSession) => {
        try {
          this.applySession(updatedSession)
        } catch (error) {
          this.recordSessionFailure(error)
        }
      })
      listenerCleanup.set(this, [disposeState])

      try {
        const disposeProgress = bridgeAdapter.onEvent<{ revision?: number; currentPlayerCount: number; elapsedSeconds: number; remainingSeconds?: number; queuePosition?: number }>(
          BRIDGE_EVENTS.SEED_PROGRESS,
          (progress) => {
            try {
              if (this.session.revision !== undefined && this.session.revision > 0 && progress &&
                  (progress.revision === undefined || (Number.isSafeInteger(progress.revision) && progress.revision! >= 0 && progress.revision! < this.session.revision))) return
              const checked = validateSeedProgress(progress)
              // A progress-only event cannot establish a newer run or its state.
              if (!this.isSessionStatusKnown || !this.isSeedingActive || checked.revision !== this.session.revision) return
              this.session.currentPlayerCount = checked.currentPlayerCount
              this.session.elapsedSeconds = checked.elapsedSeconds
              if (checked.remainingSeconds !== undefined) {
                this.session.remainingSeconds = checked.remainingSeconds
              }
              if (checked.queuePosition !== undefined) {
                this.session.queuePosition = checked.queuePosition
              }
              sessionGeneration.set(this, (sessionGeneration.get(this) ?? 0) + 1)
            } catch (error) {
              this.recordSessionFailure(error)
            }
          }
        )
        listenerCleanup.set(this, [disposeState, disposeProgress])
        this.isListenerInitialized = true
      } catch (error) {
        this.disposeBridgeListeners()
        throw error
      }
    },

    disposeBridgeListeners() {
      sessionGeneration.set(this, (sessionGeneration.get(this) ?? 0) + 1)
      const dispose = listenerCleanup.get(this)
      listenerCleanup.delete(this)
      this.isListenerInitialized = false
      dispose?.forEach(unsubscribe => unsubscribe())
    },

    async fetchCurrentSession() {
      const generation = sessionGeneration.get(this) ?? 0
      this.isLoading = true
      try {
        const updated = await seedService.getSession()
        if (generation !== (sessionGeneration.get(this) ?? 0) &&
            (updated.revision === undefined || this.session.revision === undefined || updated.revision <= this.session.revision)) return this.isSessionStatusKnown
        this.applySession(updated)
        return this.isSessionStatusKnown
      } catch (error) {
        if (generation === (sessionGeneration.get(this) ?? 0)) this.recordSessionFailure(error)
        return false
      } finally {
        this.isLoading = false
      }
    },

    async startSeed(plan: SeedPlan, server?: Server | null) {
      const generation = sessionGeneration.get(this) ?? 0
      this.isLoading = true
      this.error = null

      try {
        if (!this.isSessionStatusKnown) throw new Error(this.sessionStatusError || 'Seed session status is unavailable')
        this.applySession(await seedService.startSeed(plan, server))
      } catch (err: any) {
        this.error = err.message || 'Failed to start seeding'
        if (generation === (sessionGeneration.get(this) ?? 0)) this.recordSessionFailure(err)
        throw err
      } finally {
        this.isLoading = false
      }
    },

    async stopSeed() {
      const generation = sessionGeneration.get(this) ?? 0
      this.isLoading = true
      this.error = null

      try {
        this.applySession(await seedService.stopSeed())
      } catch (err: any) {
        this.error = err.message || 'Failed to stop seeding'
        if (generation === (sessionGeneration.get(this) ?? 0)) this.recordSessionFailure(err)
        throw err
      } finally {
        this.isLoading = false
      }
    },

    async joinServerDirect(serverId: string) {
      const now = Date.now()
      if (this.isLoading) {
        return false
      }
      if ((now - lastDirectJoinTime) < 2000) {
        // Debounce rapid repeated join triggers from UI
        return false
      }
      lastDirectJoinTime = now
      lastDirectJoinServerId = serverId
      this.isLoading = true
      this.error = null

      try {
        return await seedService.joinServerDirect(serverId)
      } catch (err: any) {
        this.error = err.message || 'Failed to join server'
        throw err
      } finally {
        this.isLoading = false
      }
    },

    addQueueItem(item: SeedQueueItem) {
      this.queue.push(item)
    },

    removeQueueItem(id: string) {
      this.queue = this.queue.filter(q => q.id !== id)
    },

    clearError() {
      this.error = null
    },

    async cancelPowerAction() {
      try {
        await seedService.cancelPowerAction()
      } catch (err: any) {
        console.warn('Failed to cancel power action', err)
        throw err
      }
    },

    async skipToNextServer() {
      this.isLoading = true
      this.error = null
      try {
        return await seedService.skipToNextServer()
      } catch (err: any) {
        this.error = err.message || 'Failed to skip server'
        throw err
      } finally {
        this.isLoading = false
      }
    },

    async restartSquadWithPotato(plan: SeedPlan, server?: Server | null) {
      const generation = sessionGeneration.get(this) ?? 0
      this.isLoading = true
      this.error = null
      try {
        if (!this.isSessionStatusKnown) throw new Error(this.sessionStatusError || 'Seed session status is unavailable')
        this.applySession(await seedService.restartSquadWithPotato(plan, server))
      } catch (err: any) {
        this.error = err.message || 'Failed to restart Squad with Potato Mode'
        if (generation === (sessionGeneration.get(this) ?? 0)) this.recordSessionFailure(err)
        throw err
      } finally {
        this.isLoading = false
      }
    }
  }
})
