<script setup lang="ts">
import { useBridge } from '~/composables/useBridge'
import { useSeed } from '~/composables/useSeed'
import { useSettings } from '~/composables/useSettings'
import { useServers } from '~/composables/useServers'
import { useUpdater } from '~/composables/useUpdater'
import PowerActionModal from '~/components/app/PowerActionModal.vue'
import UpdateAvailableModal from '~/components/app/UpdateAvailableModal.vue'
import TelemetryConsentModal from '~/components/app/TelemetryConsentModal.vue'

const { checkStatus, initBridgeListeners: initBridgeStatusListeners, disposeBridgeListeners: disposeBridgeStatusListeners } = useBridge()
const { initBridgeListeners, disposeBridgeListeners, fetchCurrentSession } = useSeed()
const { settings, loadSettings } = useSettings()
const { fetchServers } = useServers()
const { checkForUpdates } = useUpdater()

let isMounted = false

onMounted(async () => {
  isMounted = true
  try {
    // 1. Initialize desktop bridge status and telemetry listener
    initBridgeStatusListeners()
    await checkStatus()
    if (!isMounted) return

    // 2. Initialize seed listeners and restore any ongoing seed state
    initBridgeListeners()
    await fetchCurrentSession()
    if (!isMounted) return

    // 3. Load app settings
    await loadSettings()
    if (!isMounted) return

    // 4. Check for updates on startup if enabled
    if (settings.value.general?.autoUpdateCheck !== false) {
      checkForUpdates(true).catch(err => console.warn('Background update check error', err))
    }

    // 5. Fetch initial server fleet telemetry in the background (non-blocking)
    fetchServers().catch(err => console.warn('Background server fetch error', err))
  } catch (err) {
    console.error('Initialization error during startup sequence', err)
  }
})

onUnmounted(() => {
  isMounted = false
  disposeBridgeStatusListeners()
  disposeBridgeListeners()
})
</script>

<template>
  <div class="relative min-h-screen">
    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>

    <PowerActionModal />
    <UpdateAvailableModal />
    <TelemetryConsentModal />
  </div>
</template>
