import type { Server } from './server'

// Canonical PostSeedAction — must match internal/seed/engine.go constants exactly.
// Frontend and Go backend share this as the single source of truth.
export type PostSeedAction =
  | 'do_nothing'    // stay in Squad after seed completes
  | 'close_squad'   // close SquadGame.exe after seed completes
  | 'sleep'         // put PC to sleep after seed completes
  | 'shutdown'      // shut down PC after seed completes

export type TimeoutAction =
  | 'abort'
  | 'close_squad'
  | 'shutdown'
  | 'sleep'
  | 'retry'
  | 'notify_only'
  | 'continue_next'

export type SeedState =
  | 'idle'
  | 'preparing'
  | 'transitioning'
  | 'connecting'
  | 'active'
  | 'completed'
  | 'timed_out'
  | 'stopped'
  | 'failed'

export interface SeedPlan {
  serverId: string
  serverIds?: string[]
  serverName: string
  serverIp?: string
  gamePort?: number
  queryPort?: number
  targetPlayerCount: number
  maxDurationMinutes: number
  serverLimit: number   // 1, 2, 3 or 0 = all servers in queue
  postSeedAction: PostSeedAction
  timeoutAction: TimeoutAction
  autoRecoverCrash?: boolean
  forceRunningSettings?: boolean
}

export interface ActiveSeedSession {
  hasObservedGame?: boolean
  revision?: number
  state: SeedState
  plan?: SeedPlan | null
  server?: Server | null
  initialPlayerCount: number
  currentPlayerCount: number
  targetPlayerCount: number
  queuePosition: number
  elapsedSeconds: number
  remainingSeconds: number | null
  currentQueueIndex?: number
  totalQueueCount?: number
  skippedServerCount?: number
  startedAt?: string | null
  completedAt?: string | null
  errorMessage?: string | null
}

export interface SeedQueueItem {
  id: string
  serverId: string
  serverName: string
  plan: SeedPlan
  priority: number
  status: 'queued' | 'active' | 'completed' | 'failed' | 'cancelled'
  createdAt: string
}
