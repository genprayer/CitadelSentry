import type { Server } from '~/types'

function requireObject(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Server must be an object')
  return value as Record<string, unknown>
}

function requireString(value: unknown, key: string): string {
  if (typeof value !== 'string') throw new Error(`${key} must be a string`)
  return value
}

function requireCount(value: unknown, key: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) throw new Error(`${key} must be a nonnegative safe integer`)
  return value
}

function requireId(value: unknown): string {
  const id = requireString(value, 'id')
  if (!id || id !== id.trim()) throw new Error('Server id must be explicit and unambiguous')
  return id
}

function requirePort(value: unknown, key: string): number {
  const port = requireCount(value, key)
  if (port > 65535) throw new Error(`${key} exceeds the supported port range`)
  return port
}

/** Validate the complete native DTO without filling missing telemetry. */
export function validateServer(value: unknown): Server {
  const server = requireObject(value)
  requireId(server.id)
  for (const key of ['name', 'serverIp', 'currentMap', 'currentLayer', 'status']) requireString(server[key], key)
  for (const key of ['currentPlayers', 'maxPlayers', 'publicQueue', 'targetSeedCount']) requireCount(server[key], key)
  for (const key of ['gamePort', 'queryPort']) requirePort(server[key], key)
  if (typeof server.isSeedingActive !== 'boolean') throw new Error('isSeedingActive must be a boolean')
  for (const key of ['gameMode', 'nextLayer', 'teamOne', 'teamTwo']) {
    if (server[key] !== undefined) requireString(server[key], key)
  }
  if (server.pingMs !== undefined) requireCount(server.pingMs, 'pingMs')
  if (server.updatedAt !== undefined && (typeof server.updatedAt !== 'string' || !Number.isFinite(Date.parse(server.updatedAt)))) throw new Error('updatedAt is invalid')
  return value as Server
}

/** Check the entire fleet before shared state is replaced; an empty array is valid. */
export function validateServerFleet(value: unknown): Server[] {
  if (!Array.isArray(value)) throw new Error('Server fleet must be an array')
  const ids = new Set<string>()
  for (const item of value) {
    const server = validateServer(item)
    if (ids.has(server.id)) throw new Error(`Repeated server id: ${server.id}`)
    ids.add(server.id)
  }
  return value as Server[]
}

/** Map the public producer schema; only its explicitly nullable metadata may be absent. */
export function mapPublicServerFleet(value: unknown, seedTarget: number): Server[] {
  if (!Array.isArray(value)) throw new Error('Server fleet must be an array')
  const fleet = value.map(item => {
    const row = requireObject(item)
    const id = requireId(row.id)
    const name = requireString(row.name, 'name')
    const players = requireCount(row.players, 'players')
    const maxPlayers = requireCount(row.maxPlayers, 'maxPlayers')
    const queue = requireCount(row.queue, 'queue')
    let status = requireString(row.status, 'status').toLowerCase() || 'unknown'
    for (const key of ['ip', 'mapName', 'layer', 'gameMode', 'team1', 'team2', 'localMapImage']) {
      if (row[key] !== undefined && row[key] !== null) requireString(row[key], key)
    }
    const gamePort = row.port == null ? 0 : requirePort(row.port, 'port')
    const queryPort = row.queryPort == null ? 0 : requirePort(row.queryPort, 'queryPort')
    if ((status === 'online' || status === 'seeding') && maxPlayers > 0 && players >= maxPlayers) status = 'full'
    const isSeeding = (status === 'online' || status === 'seeding') && players < seedTarget
    if (isSeeding) status = 'seeding'
    return {
      id, name, serverIp: row.ip ?? '', gamePort, queryPort,
      currentPlayers: players, maxPlayers, publicQueue: queue,
      currentMap: row.mapName ?? '', currentLayer: row.layer ?? '',
      gameMode: row.gameMode ?? '', teamOne: row.team1 ?? '', teamTwo: row.team2 ?? '',
      isSeedingActive: isSeeding, targetSeedCount: seedTarget, status,
      updatedAt: new Date().toISOString()
    } as Server
  })
  return validateServerFleet(fleet)
}
