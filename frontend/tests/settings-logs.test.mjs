import assert from 'node:assert/strict'
import test from 'node:test'
import { effectScope, ref } from 'vue'
import { loadComponent } from './helpers/load-component.mjs'
import { loadSource } from './helpers/load-source.mjs'

async function fixture(operations = {}) {
  const component = await loadComponent('pages/settings.vue', {
    'vue-router': {
      useRoute: () => ({ query: {} }),
      useRouter: () => ({ replace: () => {}, push: () => {} })
    },
    '~/composables/useSettings': { useSettings: () => ({
      settings: ref({ general: {}, seed: {}, power: {}, potato: {}, webApiBaseUrl: 'https://fixture.invalid' }),
      isSaving: ref(false), isLoading: ref(false), isLoaded: ref(true), error: ref(null)
    }) },
    '~/composables/useBridge': { useBridge: () => ({
      status: ref({ logFilePath: '/fixture/citadel-sentry.log' }), isBridgeReady: ref(true), platform: ref('linux'),
      getRecentLogs: operations.getRecentLogs ?? (async () => []),
      clearLogs: operations.clearLogs ?? (async () => undefined)
    }) },
    '~/composables/useI18n': { useI18n: () => ({ t: key => key }) },
    '~/composables/use-feedback-timer': { useFeedbackTimer: () => () => undefined },
    '~/adapters/wails': { bridgeAdapter: {} }
  }, { onMounted: () => undefined, console: { ...console, error: () => undefined } })
  const scope = effectScope()
  const fields = scope.run(() => component.setup({}, { emit: () => undefined, expose: () => undefined }))
  return { fields, dispose: () => scope.stop() }
}

test('failed log reads are visible and a successful retry clears the error', async t => {
  let shouldFail = true
  const f = await fixture({ getRecentLogs: async () => {
    if (shouldFail) throw new Error('Fixture log read denied')
    return ['verified fixture line']
  } }); t.after(f.dispose)
  await f.fields.loadLogs()
  assert.match(f.fields.logsError?.value ?? '', /Fixture log read denied/)
  assert.equal(f.fields.hasLoadedLogs?.value, false)
  shouldFail = false
  await f.fields.loadLogs()
  assert.equal(f.fields.logsError.value, null)
  assert.deepEqual(Array.from(f.fields.logsList.value), ['verified fixture line'])
})

test('failed clearing preserves confirmed lines and reports the failure', async t => {
  const f = await fixture({
    getRecentLogs: async () => ['verified fixture line'],
    clearLogs: async () => { throw new Error('Fixture log truncate denied') }
  }); t.after(f.dispose)
  await f.fields.loadLogs()
  await f.fields.handleClearLogs()
  assert.match(f.fields.logsError?.value ?? '', /Fixture log truncate denied/)
  assert.deepEqual(Array.from(f.fields.logsList.value), ['verified fixture line'])
})

test('a read started before successful clearing cannot restore deleted log lines', async t => {
  let finishRead
  const f = await fixture({ getRecentLogs: () => new Promise(resolve => { finishRead = resolve }) }); t.after(f.dispose)
  const reading = f.fields.loadLogs()
  await f.fields.handleClearLogs()
  finishRead(['line captured before clearing'])
  await reading
  assert.deepEqual(Array.from(f.fields.logsList.value), [])
  assert.equal(f.fields.isLoadingLogs.value, false)
})

test('a late read error cannot replace the confirmed result of clearing', async t => {
  let rejectRead
  const f = await fixture({ getRecentLogs: () => new Promise((_resolve, reject) => { rejectRead = reject }) }); t.after(f.dispose)
  const reading = f.fields.loadLogs()
  await f.fields.handleClearLogs()
  rejectRead(new Error('Old read failed after clear'))
  await reading
  assert.equal(f.fields.logsError.value, null)
  assert.equal(f.fields.hasLoadedLogs.value, true)
})

test('clearing in progress does not dispatch duplicate clears or a new read', async t => {
  let finishClear
  let clearCalls = 0
  let readCalls = 0
  const f = await fixture({
    clearLogs: () => { clearCalls++; return new Promise(resolve => { finishClear = resolve }) },
    getRecentLogs: async () => { readCalls++; return [] }
  }); t.after(f.dispose)
  const clearing = f.fields.handleClearLogs()
  await f.fields.handleClearLogs()
  await f.fields.loadLogs()
  assert.equal(clearCalls, 1)
  assert.equal(readCalls, 0)
  finishClear()
  await clearing
  assert.equal(f.fields.isClearingLogs.value, false)
})

test('log read and clear cannot report an empty or cleared journal without native methods', async () => {
  for (const globals of [{}, { window: {} }, { window: { go: { main: { App: {} } } } }]) {
    const { WailsBridgeAdapter } = await loadSource('adapters/wails/bridgeAdapter.ts', globals)
    const adapter = new WailsBridgeAdapter()
    await assert.rejects(adapter.getRecentLogs(), error => error.name === 'BridgeUnavailableError')
    await assert.rejects(adapter.clearLogs(), error => error.name === 'BridgeUnavailableError')
  }
})

test('native nil log slices remain empty while malformed line payloads are rejected', async () => {
  let payload = null
  const { WailsBridgeAdapter } = await loadSource('adapters/wails/bridgeAdapter.ts', {
    window: { go: { main: { App: { GetRecentLogs: async () => payload } } } }
  })
  const adapter = new WailsBridgeAdapter()
  assert.equal((await adapter.getRecentLogs()).length, 0)
  for (payload of [{ lines: [] }, 'a line', [null], [42]]) {
    await assert.rejects(adapter.getRecentLogs(), /log lines/i)
  }
})
