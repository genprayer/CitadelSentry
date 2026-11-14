import assert from 'node:assert/strict'
import test from 'node:test'
import { loadSource } from './helpers/load-source.mjs'

const adapterPath = 'adapters/wails/settingsAdapter.ts'

test('incomplete and coerced config status cannot claim a healthy backup', async () => {
  let payload
  const valid = { configFound: true, configPath: '/fixture/GameUserSettings.ini', hasBackup: true, hasPendingRestore: false, backupPath: '/fixture/backup', isPotatoMode: false, currentFpsCap: 144.5 }
  const { WailsSettingsAdapter } = await loadSource(adapterPath, {
    window: { go: { main: { App: { GetSquadConfigStatus: async () => payload } } } }
  })
  for (payload of [null, [], { hasBackup: true }, { ...valid, hasBackup: 'true' }, { ...valid, currentFpsCap: Number.NaN }, { ...valid, configPath: 1 }]) {
    await assert.rejects(new WailsSettingsAdapter().getSquadConfigStatus())
  }
  payload = valid
  assert.equal((await new WailsSettingsAdapter().getSquadConfigStatus()).currentFpsCap, 144.5)
})

test('desktop settings failures preserve the backend error instead of loading defaults', async () => {
  const failure = new Error('User settings could not be read')
  const { WailsSettingsAdapter } = await loadSource(adapterPath, {
    window: { go: { main: { App: { GetSettings: async () => { throw failure } } } } }
  })
  await assert.rejects(new WailsSettingsAdapter().getSettings(), error => error === failure)
})

test('config operations cannot report success without the desktop bridge', async () => {
  const { WailsSettingsAdapter } = await loadSource(adapterPath, { window: {} })
  const adapter = new WailsSettingsAdapter()
  for (const operation of ['restoreSquadConfig', 'saveCurrentSquadConfigAsBackup', 'getSquadConfigStatus']) {
    await assert.rejects(adapter[operation](), error => error.name === 'BridgeUnavailableError')
  }
})

test('desktop config status failures are visible instead of fabricating missing files', async () => {
  const failure = new Error('Config backup is unreadable')
  const { WailsSettingsAdapter } = await loadSource(adapterPath, {
    window: { go: { main: { App: { GetSquadConfigStatus: async () => { throw failure } } } } }
  })
  await assert.rejects(new WailsSettingsAdapter().getSquadConfigStatus(), error => error === failure)
})

test('invalid recovery metadata cannot be shown as a normal config status', async () => {
  const { WailsSettingsAdapter } = await loadSource(adapterPath, {
    window: { go: { main: { App: { GetSquadConfigStatus: async () => ({ hasPendingRestore: false, errorMessage: 'Squad backup active flag is null' }) } } } }
  })
  await assert.rejects(new WailsSettingsAdapter().getSquadConfigStatus(), /active flag is null/)
})

test('browser defaults cannot be changed by editing a previously returned settings object', async () => {
  const { WailsSettingsAdapter } = await loadSource(adapterPath, {
    window: {}, localStorage: { getItem: () => null }
  })
  const adapter = new WailsSettingsAdapter()
  const first = await adapter.getSettings()
  const original = first.seed.defaultTargetPlayers
  first.seed.defaultTargetPlayers = original + 1
  const second = await adapter.getSettings()
  assert.equal(second.seed.defaultTargetPlayers, original)
})

test('invalid stored preview settings are preserved and reported', async () => {
  const { WailsSettingsAdapter } = await loadSource(adapterPath, {
    window: {}, localStorage: { getItem: () => '{invalid-json' }
  })
  await assert.rejects(new WailsSettingsAdapter().getSettings())
})

test('incomplete saved preview settings are not filled with guessed values', async () => {
  let writes = 0
  const { WailsSettingsAdapter } = await loadSource(adapterPath, {
    window: {}, localStorage: { getItem: () => '{"seed":{"defaultTargetPlayers":25}}', setItem: () => { writes++ } }
  })
  await assert.rejects(new WailsSettingsAdapter().getSettings())
  assert.equal(writes, 0)
})

test('invalid settings cannot overwrite saved preview data or reach native save', async () => {
  let writes = 0
  const { WailsSettingsAdapter, DEFAULT_APP_SETTINGS } = await loadSource(adapterPath, {
    window: {}, localStorage: { setItem: () => { writes++ } }
  })
  const settings = { ...DEFAULT_APP_SETTINGS, potato: { ...DEFAULT_APP_SETTINGS.potato, fpsLimit: 0 } }
  await assert.rejects(new WailsSettingsAdapter().saveSettings(settings))
  assert.equal(writes, 0)
  let nativeWrites = 0
  const desktop = await loadSource(adapterPath, {
    window: { go: { main: { App: { SaveSettings: async () => { nativeWrites++; return true } } } } }
  })
  await assert.rejects(new desktop.WailsSettingsAdapter().saveSettings(settings))
  assert.equal(nativeWrites, 0)
})

test('malformed native settings and coerced actions cannot be used as saved choices', async () => {
  const initial = await loadSource(adapterPath)
  const invalid = { ...initial.DEFAULT_APP_SETTINGS, seed: { ...initial.DEFAULT_APP_SETTINGS.seed, defaultPostSeedAction: ['shutdown'] } }
  const { WailsSettingsAdapter } = await loadSource(adapterPath, {
    window: { go: { main: { App: { GetSettings: async () => invalid } } } }
  })
  await assert.rejects(new WailsSettingsAdapter().getSettings(), /defaultPostSeedAction/)
})

test('listRestorePoints normalizes null native return to empty array', async () => {
  const { WailsSettingsAdapter } = await loadSource(adapterPath, {
    window: { go: { main: { App: { ListRestorePoints: async () => null } } } }
  })
  const points = await new WailsSettingsAdapter().listRestorePoints()
  assert.ok(Array.isArray(points))
  assert.equal(points.length, 0)
})
