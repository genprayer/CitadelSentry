import assert from 'node:assert/strict'
import test from 'node:test'
import { computed, effectScope, ref } from 'vue'
import { loadComponent } from './helpers/load-component.mjs'

test('mission clock waits for connection and uses confirmed remaining time after a pause', async t => {
  const session = ref({ state: 'connecting', startedAt: null, remainingSeconds: 600 })
  const component = await loadComponent('components/sentry/SentryChronometer.vue', {
    'lucide-vue-next': { Power: {}, Radio: {} },
    '~/composables/useSeed': { useSeed: () => ({
      session, isSeedingActive: computed(() => true), isSessionStatusKnown: ref(true), formattedRemainingTime: ref('00:10:00')
    }) },
    '~/composables/useI18n': { useI18n: () => ({ t: key => key }) }
  })
  const scope = effectScope()
  t.after(() => scope.stop())
  const fields = scope.run(() => component.setup({ size: 'sm', interactive: true, showDigitalTime: true }, { emit: () => undefined, expose: () => undefined }))
  assert.equal(fields.targetEndTime.value, null)
  session.value = { state: 'active', startedAt: '2026-10-23T12:00:00Z', remainingSeconds: 500 }
  assert.equal(fields.targetEndTime.value.getTime(), fields.now.value.getTime() + 500_000)
  session.value.state = 'connecting'
  assert.equal(fields.targetEndTime.value, null)
  session.value.state = 'active'
  session.value.remainingSeconds = 400
  assert.equal(fields.targetEndTime.value.getTime(), fields.now.value.getTime() + 400_000)
})
