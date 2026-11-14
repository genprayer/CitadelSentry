import type { ActiveSeedSession, SeedPlan } from '~/types'
import { validateServer } from '~/utils/server-validation'

export const MAX_SEED_DURATION_MINUTES = Math.floor((2 ** 63 - 1) / (60 * 1e9))
const SEED_STATES = ['idle', 'preparing', 'transitioning', 'connecting', 'active', 'completed', 'timed_out', 'stopped', 'failed']
const ACTIVE_SEED_STATES = ['preparing', 'transitioning', 'connecting', 'active']
const POST_SEED_ACTIONS = ['do_nothing', 'close_squad', 'sleep', 'shutdown']
const TIMEOUT_ACTIONS = ['abort', 'retry', 'notify_only', 'continue_next', 'close_squad', 'sleep', 'shutdown']

function requireObject(value: unknown, name: string): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${name} must be an object`)
  return value as Record<string, unknown>
}

function requireCount(value: unknown, name: string, minimum = 0): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < minimum) {
    throw new Error(`${name} must be a safe integer of at least ${minimum}`)
  }
  return value
}

function requireString(value: unknown, name: string): string {
  if (typeof value !== 'string') throw new Error(`${name} must be a string`)
  return value
}

/** Validate an explicit plan without guessing server capacity or absent choices. */
export function validateSeedPlan(value: unknown): SeedPlan {
  const plan = requireObject(value, 'Seed plan')
  const serverId = requireString(plan.serverId, 'serverId')
  if (plan.serverIds !== undefined && (!Array.isArray(plan.serverIds) ||
      plan.serverIds.some(id => typeof id !== 'string' || id.length === 0))) {
    throw new Error('serverIds must contain explicit server identifiers')
  }
  if (!serverId && !(Array.isArray(plan.serverIds) && plan.serverIds.length > 0)) throw new Error('A target server is required')
  requireString(plan.serverName, 'serverName')
  requireCount(plan.targetPlayerCount, 'targetPlayerCount', 1)
  const duration = requireCount(plan.maxDurationMinutes, 'maxDurationMinutes', 1)
  if (duration > MAX_SEED_DURATION_MINUTES) throw new Error('maxDurationMinutes exceeds the supported duration')
  requireCount(plan.serverLimit, 'serverLimit')
  if (typeof plan.postSeedAction !== 'string' || !POST_SEED_ACTIONS.includes(plan.postSeedAction)) throw new Error('Unsupported postSeedAction')
  if (typeof plan.timeoutAction !== 'string' || !TIMEOUT_ACTIONS.includes(plan.timeoutAction)) throw new Error('Unsupported timeoutAction')
  if (plan.serverIp !== undefined) requireString(plan.serverIp, 'serverIp')
  for (const key of ['gamePort', 'queryPort']) {
    if (plan[key] !== undefined && requireCount(plan[key], key) > 65535) throw new Error(`${key} exceeds the supported port range`)
  }
  return value as SeedPlan
}

/** Check the producer's snapshot; optional Go omitempty fields remain absent. */
export function validateSeedSession(value: unknown): ActiveSeedSession {
  const session = requireObject(value, 'Seed session')
  if (typeof session.state !== 'string' || !SEED_STATES.includes(session.state)) throw new Error('Unknown seed session state')
  if (session.revision !== undefined) requireCount(session.revision, 'revision')
  for (const key of ['initialPlayerCount', 'currentPlayerCount', 'targetPlayerCount', 'queuePosition', 'elapsedSeconds']) {
    requireCount(session[key], key)
  }
  if (session.remainingSeconds !== null) requireCount(session.remainingSeconds, 'remainingSeconds')
  for (const key of ['currentQueueIndex', 'totalQueueCount', 'skippedServerCount']) {
    if (session[key] !== undefined) requireCount(session[key], key)
  }
  if (session.hasObservedGame !== undefined && typeof session.hasObservedGame !== 'boolean') throw new Error('hasObservedGame must be a boolean')
  for (const key of ['startedAt', 'completedAt']) {
    const timestamp = session[key]
    if (timestamp !== undefined && timestamp !== null &&
        (typeof timestamp !== 'string' || !Number.isFinite(Date.parse(timestamp)))) throw new Error(`${key} is invalid`)
  }
  if (session.errorMessage !== undefined && session.errorMessage !== null) requireString(session.errorMessage, 'errorMessage')
  if (session.plan !== undefined && session.plan !== null) validateSeedPlan(session.plan)
  if (ACTIVE_SEED_STATES.includes(session.state) && !session.plan) throw new Error('Active seed session has no recorded plan')
  if (session.server !== undefined && session.server !== null) {
    validateServer(session.server)
  }
  return value as ActiveSeedSession
}

export interface SeedProgress {
  revision?: number
  currentPlayerCount: number
  elapsedSeconds: number
  remainingSeconds?: number | null
  queuePosition?: number
}

/** Validate every progress field before any shared session mutation. */
export function validateSeedProgress(value: unknown): SeedProgress {
  const progress = requireObject(value, 'Seed progress')
  if (progress.revision !== undefined) requireCount(progress.revision, 'revision')
  requireCount(progress.currentPlayerCount, 'currentPlayerCount')
  requireCount(progress.elapsedSeconds, 'elapsedSeconds')
  if (progress.remainingSeconds !== undefined && progress.remainingSeconds !== null) requireCount(progress.remainingSeconds, 'remainingSeconds')
  if (progress.queuePosition !== undefined) requireCount(progress.queuePosition, 'queuePosition')
  return value as SeedProgress
}
