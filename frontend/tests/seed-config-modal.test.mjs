import assert from 'node:assert/strict'
import test from 'node:test'
import { effectScope, ref } from 'vue'
import { loadComponent } from './helpers/load-component.mjs'
import { loadSource } from './helpers/load-source.mjs'

async function fixture(hour = 15, minute = 0, options = {}) {
  const clock = { now: new Date(2026, 9, 13, hour, minute).getTime() }
  class FixtureDate extends Date {
    constructor(...arguments_) { super(...(arguments_.length ? arguments_ : [clock.now])) }
    static now() { return clock.now }
  }
  const plans = []
  const isSquadRunning = ref(options.isSquadRunning ?? false)
  const savedSettings = ref({ seed: { defaultTargetPlayers: 62, defaultMaxDurationMinutes: 45, defaultPostSeedAction: 'do_nothing' }, potato: { enabled: options.isPotatoEnabled ?? false, fpsLimit: 1 } })
  const server = { id: 'fixture-one', name: 'Fixture Server', serverIp: '127.0.0.1', gamePort: 7787, queryPort: 27165, currentPlayers: 50, maxPlayers: 100, publicQueue: 0, status: 'seeding' }
  const constants = await loadSource('constants/seed.ts')
  const { validateSeedPlan } = await loadSource('utils/seed-validation.ts')
  const component = await loadComponent('components/seed/SeedConfigModal.vue', {
    '~/constants/seed': constants,
    '~/composables/useSeed': { useSeed: () => ({ startSeed: async plan => plans.push(validateSeedPlan(plan)), isLoading: ref(false), isSessionStatusKnown: ref(true), sessionStatusError: ref(null) }) },
    '~/composables/useSettings': { useSettings: () => ({ settings: savedSettings, isLoaded: ref(true), isSaving: ref(false), error: ref(null) }) },
    '~/composables/useServers': { useServers: () => ({ servers: ref([server]) }) },
    '~/composables/useBridge': { useBridge: () => ({ isSquadRunning }) },
    '~/composables/useI18n': { useI18n: () => ({ t: key => key }) }
  }, { Date: FixtureDate })
  const scope = effectScope()
  const fields = scope.run(() => component.setup({ modelValue: true, server }, { emit: () => undefined, expose: () => undefined }))
  return { fields, plans, isSquadRunning, savedSettings, advance: minutes => { clock.now += minutes * 60_000 }, dispose: () => scope.stop() }
}

test('running Squad explains the Potato Mode launch block and closing it clears the block', async t => {
  const f = await fixture(15, 0, { isSquadRunning: true, isPotatoEnabled: true }); t.after(f.dispose)
  assert.equal(f.fields.startBlockedReason.value, 'modal.potatoStartBlocked')
  await f.fields.handleStart()
  assert.equal(f.plans.length, 0)
  f.isSquadRunning.value = false
  assert.equal(f.fields.startBlockedReason.value, null)
  await f.fields.handleStart()
  assert.equal(f.plans.length, 1)
})

test('turning off Potato Mode permits seeding with an already running game', async t => {
  const f = await fixture(15, 0, { isSquadRunning: true, isPotatoEnabled: true }); t.after(f.dispose)
  f.savedSettings.value.potato.enabled = false
  assert.equal(f.fields.startBlockedReason.value, null)
  await f.fields.handleStart()
  assert.equal(f.plans.length, 1)
})

test('unknown process status remains blocked and explains why', async t => {
  const f = await fixture(15, 0, { isPotatoEnabled: true }); t.after(f.dispose)
  f.isSquadRunning.value = null
  assert.equal(f.fields.startBlockedReason.value, 'potato.processUnknownNotice')
  await f.fields.handleStart()
  assert.equal(f.plans.length, 0)
})

test('explicit finish time is recalculated when launch is delayed', async t => {
  const f = await fixture(); t.after(f.dispose)
  f.fields.localEndTime.value = '15:10'
  f.advance(2)
  await f.fields.handleStart()
  assert.equal(f.plans[0].maxDurationMinutes, 8)
})

test('a passed finish time blocks launch and preserves the chosen clock time', async t => {
  const f = await fixture(); t.after(f.dispose)
  f.fields.localEndTime.value = '15:10'
  f.advance(11)
  await f.fields.handleStart()
  assert.equal(f.plans.length, 0)
  assert.equal(f.fields.formError.value, 'modal.minDurationWarning')
  assert.equal(f.fields.localEndTime.value, '15:10')
})

test('an explicit finish after midnight retains its chosen next-day date', async t => {
  const f = await fixture(23, 50); t.after(f.dispose)
  f.fields.localEndTime.value = '00:10'
  f.advance(2)
  await f.fields.handleStart()
  assert.equal(f.plans[0].maxDurationMinutes, 18)
})

test('editing duration clears the earlier clock deadline', async t => {
  const f = await fixture(); t.after(f.dispose)
  f.fields.localEndTime.value = '15:10'
  f.fields.durationInput.value = 45
  f.advance(2)
  await f.fields.handleStart()
  assert.equal(f.plans[0].maxDurationMinutes, 45)
})

test('saved relative duration remains the saved choice before launch', async t => {
  const f = await fixture(); t.after(f.dispose)
  f.advance(2)
  await f.fields.handleStart()
  assert.equal(f.plans[0].maxDurationMinutes, 45)
})

test('missing duration cannot display an invented one-minute finish time', async t => {
  const f = await fixture(); t.after(f.dispose)
  f.fields.durationInput.value = 0
  assert.equal(f.fields.localEndTime.value, '')
  await f.fields.handleStart()
  assert.equal(f.plans.length, 0)
})
