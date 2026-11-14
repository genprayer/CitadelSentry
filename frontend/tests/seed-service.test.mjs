import assert from 'node:assert/strict'
import test from 'node:test'
import { loadSource } from './helpers/load-source.mjs'

const validPlan = {
  serverId: 'one', serverIds: ['one'], serverName: 'One',
  targetPlayerCount: 150, maxDurationMinutes: 60, serverLimit: 1,
  postSeedAction: 'do_nothing', timeoutAction: 'abort'
}

test('missing frontend server data does not invent capacity 100', async () => {
  const { SeedService } = await loadSource('services/seedService.ts')
  assert.equal(new SeedService().validatePlan(validPlan).valid, true)
})

test('already reached primary target does not reject a queue that the backend can skip', async () => {
  const { SeedService } = await loadSource('services/seedService.ts')
  const plan = { ...validPlan, targetPlayerCount: 50, serverIds: ['one', 'two'], serverLimit: 2 }
  assert.equal(new SeedService().validatePlan(plan, { maxPlayers: 100, currentPlayers: 60 }).valid, true)
})

test('nonfinite/fractional choices and unsupported actions cannot reach native start', async () => {
  let starts = 0
  const { SeedService } = await loadSource('services/seedService.ts', {
    window: { go: { main: { App: { StartSeed: async () => { starts++; return {} } } } } }
  })
  for (const changes of [
    { targetPlayerCount: Number.NaN }, { targetPlayerCount: 50.5 },
    { maxDurationMinutes: Number.POSITIVE_INFINITY }, { maxDurationMinutes: 0.5 },
    { serverLimit: 1.5 }, { postSeedAction: ['shutdown'] }, { timeoutAction: 'unknown' }
  ]) {
    await assert.rejects(new SeedService().startSeed({ ...validPlan, ...changes }))
  }
  assert.equal(starts, 0)
})
