import assert from 'node:assert/strict'
import test from 'node:test'
import { createPinia, setActivePinia } from 'pinia'
import { loadSource } from './helpers/load-source.mjs'

const server = {
  id: 'one', name: 'One', serverIp: '', gamePort: 0, queryPort: 0,
  currentPlayers: 10, maxPlayers: 100, publicQueue: 0,
  currentMap: '', currentLayer: '', isSeedingActive: false, targetSeedCount: 50, status: 'online'
}

async function fixture() {
  setActivePinia(createPinia())
  const replies = []
  const { useServersStore } = await loadSource('stores/servers.ts', {
    window: { go: { main: { App: { FetchServers: () => new Promise((resolve, reject) => replies.push({ resolve, reject })) } } } }
  })
  return { store: useServersStore(), replies }
}

test('a late fleet response cannot overwrite a newer confirmed list', async () => {
  const { store, replies } = await fixture()
  const first = store.fetchServers(), second = store.fetchServers()
  replies[1].resolve([{ ...server, currentPlayers: 20 }]); await second
  replies[0].resolve([server]); await first
  assert.equal(store.servers[0].currentPlayers, 20)
})

test('a late fleet failure cannot invalidate a newer confirmed list', async () => {
  const { store, replies } = await fixture()
  const first = store.fetchServers(), second = store.fetchServers()
  replies[1].resolve([server]); await second
  replies[0].reject(new Error('Older request failed')); await first
  assert.equal(store.error, null)
})

test('an older completion cannot stop the newest loading indicator', async () => {
  const { store, replies } = await fixture()
  const first = store.fetchServers(), second = store.fetchServers()
  replies[0].resolve([server]); await first
  assert.equal(store.isLoading, true)
  replies[1].resolve([server]); await second
  assert.equal(store.isLoading, false)
})

test('malformed telemetry preserves the last confirmed fleet and timestamp', async () => {
  const { store, replies } = await fixture()
  const initial = store.fetchServers(); replies[0].resolve([server]); await initial
  const timestamp = store.lastUpdated
  const next = store.fetchServers(); replies[1].resolve([{ id: 'one' }]); await next
  assert.equal(store.servers[0].name, 'One')
  assert.equal(store.lastUpdated, timestamp)
  assert.ok(store.error)
})
