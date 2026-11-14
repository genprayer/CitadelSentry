import assert from 'node:assert/strict'
import test from 'node:test'
import { loadSource } from './helpers/load-source.mjs'

test('power cancellation cannot report success after losing the desktop bridge', async () => {
  const { WailsSeedAdapter } = await loadSource('adapters/wails/seedAdapter.ts', { window: {} })
  await assert.rejects(new WailsSeedAdapter().cancelPowerAction(), error => error.name === 'BridgeUnavailableError')
})

test('power cancellation preserves the native failure', async () => {
  const failure = new Error('Scheduled shutdown could not be cancelled')
  const { WailsSeedAdapter } = await loadSource('adapters/wails/seedAdapter.ts', {
    window: { go: { main: { App: { CancelPowerAction: async () => { throw failure } } } } }
  })
  await assert.rejects(new WailsSeedAdapter().cancelPowerAction(), error => error === failure)
})

test('unavailable desktop status cannot be substituted with an idle session', async () => {
  const { WailsSeedAdapter } = await loadSource('adapters/wails/seedAdapter.ts', { window: {} })
  await assert.rejects(new WailsSeedAdapter().getSession(), error => error.name === 'BridgeUnavailableError')
})

test('native omitted optional snapshot fields remain absent', async () => {
  const snapshot = { revision: 0, state: 'idle', hasObservedGame: false,
    initialPlayerCount: 0, currentPlayerCount: 0, targetPlayerCount: 0,
    queuePosition: 0, elapsedSeconds: 0, remainingSeconds: null,
    currentQueueIndex: 0, totalQueueCount: 0 }
  const { WailsSeedAdapter } = await loadSource('adapters/wails/seedAdapter.ts', {
    window: { go: { main: { App: { GetSeedSession: async () => snapshot } } } }
  })
  const result = await new WailsSeedAdapter().getSession()
  assert.equal(result, snapshot)
  assert.equal('plan' in result, false)
  assert.equal('startedAt' in result, false)
})
