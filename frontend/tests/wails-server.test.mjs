import assert from 'node:assert/strict'
import test from 'node:test'
import { loadSource } from './helpers/load-source.mjs'

const adapterPath = 'adapters/wails/serverAdapter.ts'

const desktopServer = {
  id: 'one', name: 'One', serverIp: '', gamePort: 0, queryPort: 0,
  currentPlayers: 10, maxPlayers: 100, publicQueue: 0,
  currentMap: '', currentLayer: '', isSeedingActive: false, targetSeedCount: 50, status: 'online'
}

test('an empty desktop fleet is authoritative and does not trigger another data source', async () => {
  const fleet = []
  let extraCalls = 0
  const { WailsServerAdapter } = await loadSource(adapterPath, {
    window: { go: { main: { App: {
      FetchServers: async () => fleet,
      GetServers: async () => { extraCalls++; return [{ id: 'different-server' }] }
    } } } }
  })
  assert.equal(await new WailsServerAdapter().fetchServers(), fleet)
  assert.equal(extraCalls, 0)
})

test('desktop fleet errors are preserved instead of falling back to the browser API', async () => {
  const failure = new Error('Configured fleet endpoint is unavailable')
  let networkCalls = 0
  const { WailsServerAdapter } = await loadSource(adapterPath, {
    window: { go: { main: { App: { FetchServers: async () => { throw failure } } } } },
    useRuntimeConfig: () => ({ public: { citadelApiBaseUrl: 'https://preview.invalid' } }),
    fetch: async () => { networkCalls++; return { ok: true, json: async () => [] } }
  })
  await assert.rejects(new WailsServerAdapter().fetchServers(), error => error === failure)
  assert.equal(networkCalls, 0)
})

test('browser mapping preserves an explicitly unknown server capacity', async () => {
  const { WailsServerAdapter } = await loadSource(adapterPath, {
    window: {},
    localStorage: { getItem: () => null },
    useRuntimeConfig: () => ({ public: { citadelApiBaseUrl: 'https://preview.invalid' } }),
    fetch: async () => ({ ok: true, json: async () => [{ id: '1', name: 'One', status: 'unknown', players: 0, maxPlayers: 0, queue: 0 }] })
  })
  const fleet = await new WailsServerAdapter().fetchServers()
  assert.equal(fleet[0].maxPlayers, 0)
  assert.equal(fleet[0].status, 'unknown')
})

test('browser mapping does not turn a starting server into a ready full server', async () => {
  const { WailsServerAdapter } = await loadSource(adapterPath, {
    window: {}, localStorage: { getItem: () => null },
    useRuntimeConfig: () => ({ public: { citadelApiBaseUrl: 'https://preview.invalid' } }),
    fetch: async () => ({ ok: true, json: async () => [{ id: '1', name: 'One', status: 'starting', players: 100, maxPlayers: 100, queue: 0 }] })
  })
  const fleet = await new WailsServerAdapter().fetchServers()
  assert.equal(fleet[0].status, 'starting')
})

test('browser seeding targets come from the saved user settings', async () => {
  const { DEFAULT_APP_SETTINGS } = await loadSource('adapters/wails/settingsAdapter.ts')
  const saved = { ...DEFAULT_APP_SETTINGS, seed: { ...DEFAULT_APP_SETTINGS.seed, defaultTargetPlayers: 80 } }
  const { WailsServerAdapter } = await loadSource(adapterPath, {
    window: {}, localStorage: { getItem: () => JSON.stringify(saved) },
    useRuntimeConfig: () => ({ public: { citadelApiBaseUrl: 'https://preview.invalid' } }),
    fetch: async () => ({ ok: true, json: async () => [{ id: '1', name: 'One', status: 'online', players: 60, maxPlayers: 100, queue: 0 }] })
  })
  const fleet = await new WailsServerAdapter().fetchServers()
  assert.equal(fleet[0].targetSeedCount, 80)
  assert.equal(fleet[0].isSeedingActive, true)
})

test('malformed desktop fleets cannot replace confirmed server state', async () => {
  let payload
  const { WailsServerAdapter } = await loadSource(adapterPath, {
    window: { go: { main: { App: { FetchServers: async () => payload } } } }
  })
  for (payload of [null, [null], [{ id: 'one' }], [desktopServer, desktopServer], [{ ...desktopServer, currentPlayers: -1 }]]) {
    await assert.rejects(new WailsServerAdapter().fetchServers())
  }
})

test('browser requests use the explicitly saved API endpoint', async () => {
  const { DEFAULT_APP_SETTINGS } = await loadSource('adapters/wails/settingsAdapter.ts')
  const saved = { ...DEFAULT_APP_SETTINGS, webApiBaseUrl: 'https://configured.invalid/base/' }
  let requested
  const { WailsServerAdapter } = await loadSource(adapterPath, {
    window: {}, localStorage: { getItem: () => JSON.stringify(saved) },
    useRuntimeConfig: () => ({ public: { citadelApiBaseUrl: 'https://unselected.invalid' } }),
    fetch: async url => { requested = url; return { ok: true, json: async () => [] } }
  })
  await new WailsServerAdapter().fetchServers()
  assert.equal(requested, 'https://configured.invalid/base/api/servers/sentry')
})

test('incomplete browser telemetry is rejected instead of guessed', async () => {
  let payload
  const { WailsServerAdapter } = await loadSource(adapterPath, {
    window: {}, localStorage: { getItem: () => null },
    useRuntimeConfig: () => ({ public: { citadelApiBaseUrl: 'https://preview.invalid' } }),
    fetch: async () => ({ ok: true, json: async () => payload })
  })
  const valid = { id: 'one', name: 'One', players: 10, maxPlayers: 100, queue: 0, status: 'online' }
  for (const field of ['id', 'name', 'players', 'maxPlayers', 'queue', 'status']) {
    payload = [{ ...valid }]; delete payload[0][field]
    await assert.rejects(new WailsServerAdapter().fetchServers(), `missing ${field}`)
    payload = [{ ...valid, [field]: null }]
    await assert.rejects(new WailsServerAdapter().fetchServers(), `null ${field}`)
  }
})

test('full browser server remains full when the saved target exceeds capacity', async () => {
  const { WailsServerAdapter } = await loadSource(adapterPath, {
    window: {}, localStorage: { getItem: () => null },
    fetch: async () => ({ ok: true, json: async () => [{ id: 'one', name: '', status: 'online', players: 40, maxPlayers: 40, queue: 0, mapName: null }] })
  })
  const fleet = await new WailsServerAdapter().fetchServers()
  assert.equal(fleet[0].status, 'full')
  assert.equal(fleet[0].isSeedingActive, false)
  assert.equal(fleet[0].name, '')
  assert.equal(fleet[0].currentMap, '')
})
