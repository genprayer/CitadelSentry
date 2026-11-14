import assert from 'node:assert/strict'
import test from 'node:test'
import { createPinia, setActivePinia } from 'pinia'
import { loadSource } from './helpers/load-source.mjs'

test('process status stays unknown without a desktop bridge', async () => {
  const { WailsBridgeAdapter } = await loadSource('adapters/wails/bridgeAdapter.ts')
  const status = await new WailsBridgeAdapter().getStatus()
  assert.equal(status.squadRunning, null)
  assert.equal(status.steamRunning, null)
})

test('failed desktop status does not claim the game is stopped', async () => {
  const { WailsBridgeAdapter } = await loadSource('adapters/wails/bridgeAdapter.ts', {
    window: { go: { main: { App: { GetBridgeStatus: async () => { throw new Error('process enumeration failed') } } } } }
  })
  const status = await new WailsBridgeAdapter().getStatus()
  assert.equal(status.squadRunning, null)
  assert.match(status.processStatusError, /process enumeration failed/)
})

test('bridge status events preserve unknown process states', async () => {
  setActivePinia(createPinia())
  let receiveStatus
  const { useBridgeStore } = await loadSource('stores/bridge.ts', {
    window: { runtime: { EventsOn: (_name, callback) => { receiveStatus = callback; return () => {} } } }
  })
  const store = useBridgeStore()
  store.initListeners()
  receiveStatus({ squadRunning: null, steamRunning: true, processStatusError: 'Squad inspection failed' })
  assert.equal(store.isSquadRunning, null)
  assert.equal(store.isSteamRunning, true)
  assert.equal(store.status.processStatusError, 'Squad inspection failed')
})

test('native priority capability is accepted only as an explicit boolean', async () => {
  setActivePinia(createPinia())
  let receiveStatus
  const { useBridgeStore } = await loadSource('stores/bridge.ts', {
    window: { runtime: { EventsOn: (_name, callback) => { receiveStatus = callback; return () => {} } } }
  })
  const store = useBridgeStore()
  store.initListeners()
  receiveStatus({ canSetLowCpuPriority: false })
  assert.equal(store.status.canSetLowCpuPriority, false)
  receiveStatus({ canSetLowCpuPriority: 'true' })
  assert.equal(store.status.canSetLowCpuPriority, undefined)
  store.disposeListeners()
})

test('bridge listener initialization and teardown own exactly one subscription', async () => {
  setActivePinia(createPinia())
  let subscriptions = 0
  let removals = 0
  let receiveStatus
  const { useBridgeStore } = await loadSource('stores/bridge.ts', {
    window: { runtime: { EventsOn: (_name, callback) => {
      subscriptions++
      receiveStatus = callback
      return () => { removals++ }
    } } }
  })
  const store = useBridgeStore()
  store.initListeners()
  store.initListeners()
  assert.equal(subscriptions, 1)
  store.disposeListeners()
  store.disposeListeners()
  assert.equal(removals, 1)
  store.initListeners()
  receiveStatus({ squadRunning: false, steamRunning: true })
  assert.equal(store.isSquadRunning, false)
  assert.equal(subscriptions, 2)
  store.disposeListeners()
})

test('late bridge API response cannot overwrite a newer unknown process event', async () => {
  setActivePinia(createPinia())
  let finishRead
  let receiveStatus
  const read = new Promise(resolve => { finishRead = resolve })
  const { useBridgeStore } = await loadSource('stores/bridge.ts', {
    window: {
      runtime: { EventsOn: (_name, callback) => { receiveStatus = callback; return () => {} } },
      go: { main: { App: { GetBridgeStatus: () => read } } }
    }
  })
  const store = useBridgeStore()
  store.initListeners()
  const loading = store.checkStatus()
  receiveStatus({ squadRunning: null, steamRunning: null, processStatusError: 'Inspection failed' })
  finishRead({ squadRunning: false, steamRunning: false, platform: 'windows', appVersion: 'test' })
  await loading
  assert.equal(store.isSquadRunning, null)
  assert.equal(store.status.processStatusError, 'Inspection failed')
  store.disposeListeners()
})
