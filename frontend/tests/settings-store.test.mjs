import assert from 'node:assert/strict'
import test from 'node:test'
import { createPinia, setActivePinia } from 'pinia'
import { loadSource } from './helpers/load-source.mjs'

const validStatus = {
  configFound: true, configPath: '/fixture/GameUserSettings.ini',
  hasBackup: true, hasPendingRestore: false, backupPath: '/fixture/backup/GameUserSettings.ini.original',
  isPotatoMode: false, currentFpsCap: 144.5
}

async function fixture() {
  setActivePinia(createPinia())
  const requests = []
  const { useSettingsStore } = await loadSource('stores/settings.ts', {
    window: { go: { main: { App: {
      GetSquadConfigStatus: () => new Promise((resolve, reject) => requests.push({ resolve, reject }))
    } } } }
  })
  return { store: useSettingsStore(), requests }
}

test('late valid config status cannot hide a newer backup integrity failure', async () => {
  const { store, requests } = await fixture()
  const first = store.fetchSquadConfigStatus(), second = store.fetchSquadConfigStatus()
  requests[1].resolve({ ...validStatus, errorMessage: 'Backup checksum mismatch' }); await second
  requests[0].resolve(validStatus); await first
  assert.equal(store.configStatus, null)
  assert.match(store.configStatusError, /checksum mismatch/)
})

test('late config failure cannot discard a newer verified status', async () => {
  const { store, requests } = await fixture()
  const first = store.fetchSquadConfigStatus(), second = store.fetchSquadConfigStatus()
  requests[1].resolve(validStatus); await second
  requests[0].reject(new Error('Older read failed')); await first
  assert.equal(store.configStatus?.currentFpsCap, 144.5)
  assert.equal(store.configStatusError, null)
})

test('malformed config status is unavailable until a validated retry succeeds', async () => {
  const { store, requests } = await fixture()
  const first = store.fetchSquadConfigStatus(); requests[0].resolve({ hasBackup: true }); await first
  assert.equal(store.configStatus, null)
  assert.ok(store.configStatusError)
  const second = store.fetchSquadConfigStatus(); requests[1].resolve(validStatus); await second
  assert.equal(store.configStatus?.currentFpsCap, 144.5)
  assert.equal(store.configStatusError, null)
})
