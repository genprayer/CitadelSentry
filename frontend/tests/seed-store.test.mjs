import assert from 'node:assert/strict'
import test from 'node:test'
import { createPinia, setActivePinia } from 'pinia'
import { loadSource } from './helpers/load-source.mjs'

function session(state, revision) {
  return {
    state, revision, plan: state === 'active' ? {
      serverId: 'one', serverName: 'One', targetPlayerCount: 50,
      maxDurationMinutes: 60, serverLimit: 1, postSeedAction: 'do_nothing', timeoutAction: 'abort'
    } : null, server: null, initialPlayerCount: 0,
    currentPlayerCount: 0, targetPlayerCount: 0, queuePosition: 0,
    elapsedSeconds: 0, remainingSeconds: null, startedAt: null,
    completedAt: null, errorMessage: null
  }
}

test('an already reached target shows complete progress', async () => {
  setActivePinia(createPinia())
  const { useSeedStore } = await loadSource('stores/seed.ts')
  const store = useSeedStore()
  store.applySession({ ...session('completed', 1), initialPlayerCount: 60, currentPlayerCount: 60, targetPlayerCount: 50 })
  assert.equal(store.progressPercent, 100)
})

test('late state events cannot resume a session after a newer stop', async () => {
  setActivePinia(createPinia())
  const listeners = new Map()
  const { useSeedStore } = await loadSource('stores/seed.ts', {
    window: { runtime: { EventsOn: (name, callback) => { listeners.set(name, callback); return () => listeners.delete(name) } } }
  })
  const store = useSeedStore()
  store.initBridgeListeners()
  const changed = listeners.get('citadel:seed:state_changed')
  changed(session('stopped', 3))
  changed(session('active', 2))
  assert.equal(store.session.state, 'stopped')
})

test('late API responses cannot overwrite a newer state event', async () => {
  setActivePinia(createPinia())
  let finishRead
  const read = new Promise(resolve => { finishRead = resolve })
  const listeners = new Map()
  const { useSeedStore } = await loadSource('stores/seed.ts', {
    window: {
      runtime: { EventsOn: (name, callback) => { listeners.set(name, callback); return () => listeners.delete(name) } },
      go: { main: { App: { GetSeedSession: () => read } } }
    }
  })
  const store = useSeedStore()
  store.initBridgeListeners()
  const loading = store.fetchCurrentSession()
  listeners.get('citadel:seed:state_changed')(session('stopped', 3))
  finishRead(session('active', 2))
  await loading
  assert.equal(store.session.state, 'stopped')
})

test('seed listener teardown allows one fresh subscription on remount', async () => {
  setActivePinia(createPinia())
  const counts = new Map()
  const removals = new Map()
  const { useSeedStore } = await loadSource('stores/seed.ts', {
    window: { runtime: { EventsOn: (name) => {
      counts.set(name, (counts.get(name) || 0) + 1)
      return () => removals.set(name, (removals.get(name) || 0) + 1)
    } } }
  })
  const store = useSeedStore()
  store.initBridgeListeners()
  store.initBridgeListeners()
  assert.equal(counts.get('citadel:seed:state_changed'), 1)
  store.disposeBridgeListeners()
  store.disposeBridgeListeners()
  assert.equal(removals.get('citadel:seed:state_changed'), 1)
  assert.equal(removals.get('citadel:seed:progress'), 1)
  store.initBridgeListeners()
  assert.equal(counts.get('citadel:seed:state_changed'), 2)
  store.disposeBridgeListeners()
})

test('failed listener registration releases the partial subscription and can retry', async () => {
  setActivePinia(createPinia())
  let shouldFailProgress = true
  let removals = 0
  const { useSeedStore } = await loadSource('stores/seed.ts', {
    window: { runtime: { EventsOn: name => {
      if (name === 'citadel:seed:progress' && shouldFailProgress) throw new Error('registration failed')
      return () => { removals++ }
    } } }
  })
  const store = useSeedStore()
  assert.throws(() => store.initBridgeListeners(), /registration failed/)
  assert.equal(removals, 1)
  assert.equal(store.isListenerInitialized, false)
  shouldFailProgress = false
  store.initBridgeListeners()
  assert.equal(store.isListenerInitialized, true)
  store.disposeBridgeListeners()
  assert.equal(removals, 3)
})

test('malformed newer state events retain the last confirmed session and show an error', async () => {
  setActivePinia(createPinia())
  const listeners = new Map()
  const { useSeedStore } = await loadSource('stores/seed.ts', {
    window: { runtime: { EventsOn: (name, callback) => { listeners.set(name, callback); return () => listeners.delete(name) } } }
  })
  const store = useSeedStore()
  store.initBridgeListeners()
  const changed = listeners.get('citadel:seed:state_changed')
  changed(session('active', 3))
  changed({ revision: 4 })
  assert.equal(store.session.state, 'active')
  assert.equal(store.session.revision, 3)
  assert.ok(store.error)
})

test('invalid progress is rejected before any counters or revision change', async () => {
  setActivePinia(createPinia())
  const listeners = new Map()
  const { useSeedStore } = await loadSource('stores/seed.ts', {
    window: { runtime: { EventsOn: (name, callback) => { listeners.set(name, callback); return () => listeners.delete(name) } } }
  })
  const store = useSeedStore()
  store.initBridgeListeners()
  listeners.get('citadel:seed:state_changed')(session('active', 3))
  listeners.get('citadel:seed:progress')({ revision: 3, currentPlayerCount: -1, elapsedSeconds: Number.NaN })
  assert.equal(store.session.currentPlayerCount, 0)
  assert.equal(store.session.elapsedSeconds, 0)
  assert.equal(store.session.revision, 3)
  assert.ok(store.error)
})

test('failed API session read remains visible and preserves the confirmed state', async () => {
  setActivePinia(createPinia())
  const { useSeedStore } = await loadSource('stores/seed.ts', {
    window: { go: { main: { App: { GetSeedSession: async () => { throw new Error('Session read failed') } } } } }
  })
  const store = useSeedStore()
  store.applySession(session('active', 3))
  await store.fetchCurrentSession()
  assert.equal(store.session.state, 'active')
  assert.match(store.error, /Session read failed/)
  assert.equal(store.isSessionStatusKnown, false)
})

test('late API failure cannot invalidate a newer confirmed event', async () => {
  setActivePinia(createPinia())
  let rejectRead
  const read = new Promise((_resolve, reject) => { rejectRead = reject })
  const listeners = new Map()
  const { useSeedStore } = await loadSource('stores/seed.ts', {
    window: {
      runtime: { EventsOn: (name, callback) => { listeners.set(name, callback); return () => listeners.delete(name) } },
      go: { main: { App: { GetSeedSession: () => read } } }
    }
  })
  const store = useSeedStore()
  store.initBridgeListeners()
  const loading = store.fetchCurrentSession()
  listeners.get('citadel:seed:state_changed')(session('stopped', 3))
  rejectRead(new Error('Older request failed'))
  await loading
  assert.equal(store.session.state, 'stopped')
  assert.equal(store.isSessionStatusKnown, true)
  assert.equal(store.error, null)
})

test('progress cannot establish a newer run or alter a terminal snapshot', async () => {
  setActivePinia(createPinia())
  const listeners = new Map()
  const { useSeedStore } = await loadSource('stores/seed.ts', {
    window: { runtime: { EventsOn: (name, callback) => { listeners.set(name, callback); return () => listeners.delete(name) } } }
  })
  const store = useSeedStore()
  store.initBridgeListeners()
  listeners.get('citadel:seed:state_changed')(session('stopped', 3))
  const progress = listeners.get('citadel:seed:progress')
  progress({ revision: 4, currentPlayerCount: 40, elapsedSeconds: 30 })
  progress({ revision: 3, currentPlayerCount: 40, elapsedSeconds: 30 })
  assert.equal(store.session.state, 'stopped')
  assert.equal(store.session.revision, 3)
  assert.equal(store.session.currentPlayerCount, 0)
})

test('stale malformed progress cannot invalidate a newer known snapshot', async () => {
  setActivePinia(createPinia())
  const listeners = new Map()
  const { useSeedStore } = await loadSource('stores/seed.ts', {
    window: { runtime: { EventsOn: (name, callback) => { listeners.set(name, callback); return () => listeners.delete(name) } } }
  })
  const store = useSeedStore()
  store.initBridgeListeners()
  listeners.get('citadel:seed:state_changed')(session('active', 3))
  listeners.get('citadel:seed:progress')({ revision: 2, currentPlayerCount: Number.NaN })
  assert.equal(store.isSessionStatusKnown, true)
  assert.equal(store.error, null)
  assert.equal(store.session.revision, 3)
})
