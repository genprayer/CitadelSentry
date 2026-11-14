export type ServerOperationalStatus =
  | 'online'
  | 'seeding'
  | 'full'
  | 'offline'
  | 'dead'
  | 'removed'
  | 'starting'
  | 'restarting'
  | 'stopping'
  | 'resolving'
  | 'stale'
  | 'pending'
  | 'unknown'

export interface Server {
  id: string
  name: string
  serverIp: string
  queryPort: number
  gamePort: number
  currentPlayers: number
  maxPlayers: number
  publicQueue: number
  currentMap: string
  currentLayer: string
  gameMode?: string
  nextLayer?: string
  teamOne?: string
  teamTwo?: string
  isSeedingActive: boolean
  targetSeedCount: number
  status: ServerOperationalStatus
  pingMs?: number
  updatedAt?: string
}

export interface ServerFilterOptions {
  search: string
  onlySeeding: boolean
  hideFull: boolean
  hideEmpty: boolean
}
