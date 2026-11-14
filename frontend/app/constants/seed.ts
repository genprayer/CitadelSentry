import type { ServerOperationalStatus } from '~/types/server'

export const NON_SEEDABLE_SERVER_STATUSES = new Set<ServerOperationalStatus>([
  'dead',
  'offline',
  'removed',
  'starting',
  'restarting',
  'stopping',
  'resolving',
  'stale',
  'pending'
])

export const isSeedableServerStatus = (status: ServerOperationalStatus): boolean => {
  return status === 'online' || status === 'seeding' || status === 'full'
}

export const SEED_SERVER_LIMIT_PRESETS = [
  { labelKey: 'modal.serverLimitOne', value: 1 },
  { labelKey: 'modal.serverLimitTwo', value: 2 },
  { labelKey: 'modal.serverLimitThree', value: 3 },
  { labelKey: 'modal.serverLimitAll', value: 0 }
] as const

export const SEED_TARGET_PLAYER_PRESETS = [35, 45, 55, 65, 75] as const

export const SEED_DURATION_PRESETS = [30, 45, 60, 90, 120] as const

// PostSeedAction — canonical values matching internal/seed/engine.go constants
export const POST_SEED_ACTION_OPTIONS = [
  { labelKey: 'modal.actionTargetStay', value: 'do_nothing' },
  { labelKey: 'modal.actionTargetCloseGame', value: 'close_squad' },
  { labelKey: 'modal.actionTargetSleep', value: 'sleep' },
  { labelKey: 'modal.actionTargetShutdown', value: 'shutdown' }
] as const

export const TIMEOUT_ACTION_OPTIONS = [
  { labelKey: 'modal.actionTimeoutAbort', value: 'abort' },
  { labelKey: 'modal.actionTimeoutNext', value: 'continue_next' },
  { labelKey: 'modal.actionTimeoutCloseGame', value: 'close_squad' },
  { labelKey: 'modal.actionTimeoutSleep', value: 'sleep' },
  { labelKey: 'modal.actionTimeoutShutdown', value: 'shutdown' }
] as const
