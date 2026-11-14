<script setup lang="ts">
import AppSidebar from '~/components/app/AppSidebar.vue'
import AppHeader from '~/components/app/AppHeader.vue'
import AppStatusBar from '~/components/app/AppStatusBar.vue'
import BridgeStatusBanner from '~/components/app/BridgeStatusBanner.vue'
</script>

<template>
  <div class="h-screen w-screen flex flex-col bg-background text-primary overflow-hidden font-sans">
    <!-- Main Application Shell -->
    <div class="flex-1 flex overflow-hidden">
      <!-- Left Sidebar Navigation -->
      <AppSidebar />

      <!-- Right Main Content Area -->
      <div class="flex-1 flex flex-col min-w-0 overflow-hidden bg-background">
        <!-- Bridge Standby Warning Banner -->
        <BridgeStatusBanner />

        <!-- Top Fleet Telemetry Header -->
        <AppHeader />

        <!-- Scrollable Page Content -->
        <main
          class="flex-1 overflow-y-auto p-6 bg-background"
          style="-webkit-overflow-scrolling: touch; transform: translateZ(0); will-change: scroll-position;"
        >
          <slot />
        </main>
      </div>
    </div>

    <!-- Bottom Status Bar -->
    <AppStatusBar />
  </div>
</template>
