import assert from 'node:assert/strict'
import test from 'node:test'
import { readFile } from 'node:fs/promises'
import { effectScope, reactive } from 'vue'
import { loadComponent } from './helpers/load-component.mjs'
import { loadSource } from './helpers/load-source.mjs'

async function fixture(t) {
  const ru = JSON.parse(await readFile(new URL('../app/locales/ru.json', import.meta.url), 'utf8'))
  const translate = (key, params = {}) => {
    const value = key.split('.').reduce((value, part) => value[part], ru)
    return value.replace(/\{(\w+)\}/g, (_, name) => String(params[name]))
  }
  const constants = await loadSource('constants/seed.ts')
  const component = await loadComponent('components/seed/SeedPolicySummary.vue', {
    '~/constants/seed': constants,
    '~/composables/useI18n': { useI18n: () => ({ t: translate }) }
  })
  const props = reactive({ serverCount: 2, targetPlayers: 40, maxDurationMinutes: 60, postSeedAction: 'do_nothing', timeoutAction: 'continue_next' })
  const scope = effectScope()
  t.after(() => scope.stop())
  const fields = scope.run(() => component.setup(props, { expose: () => undefined }))
  return { props, fields }
}

test('staying in Squad after success still explains automatic transitions between servers', async t => {
  const { props, fields } = await fixture(t)
  assert.match(fields.targetDescription.value, /перейти к следующему/)
  assert.match(fields.successDescription.value, /на всех выбранных серверах/)
  assert.match(fields.successDescription.value, /остаться в Squad/)
  assert.match(fields.timeoutDescription.value, /Если он последний, завершить очередь/)
  props.serverCount = 1
  assert.match(fields.targetDescription.value, /завершить миссию успешно/)
  assert.doesNotMatch(fields.targetDescription.value, /следующему/)
})

test('each timeout choice explains whether the rest of the queue will be cancelled', async t => {
  const { props, fields } = await fixture(t)
  for (const [action, expected] of [
    ['abort', /остановить всю очередь и остаться в Squad/],
    ['close_squad', /остановить всю очередь и закрыть Squad/],
    ['sleep', /остановить всю очередь и перевести ПК в сон/],
    ['shutdown', /остановить всю очередь и выключить ПК/],
    ['retry', /тому же серверу с новым таймером/]
  ]) {
    props.timeoutAction = action
    assert.match(fields.timeoutDescription.value, expected)
  }
  props.postSeedAction = 'shutdown'
  assert.match(fields.successDescription.value, /на всех выбранных серверах/)
  assert.match(fields.successDescription.value, /Выключить компьютер/)
})
