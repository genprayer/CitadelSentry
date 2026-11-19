<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { ArrowUpCircle, Sparkles, AlertCircle, LoaderCircle, Download, X } from 'lucide-vue-next'
import { useUpdater } from '~/composables/useUpdater'
import BaseButton from '~/components/ui/BaseButton.vue'
import BaseBadge from '~/components/ui/BaseBadge.vue'

const { updateInfo, isUpdating, isModalOpen, updateError, applyUpdate, dismissModal } = useUpdater()

const assetSizeMB = computed(() => {
  if (!updateInfo.value?.assetSize) return ''
  return (updateInfo.value.assetSize / (1024 * 1024)).toFixed(1) + ' MB'
})

const onKeyDown = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && isModalOpen.value && !isUpdating.value) {
    dismissModal()
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKeyDown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeyDown)
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="isModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-background/95 animate-in fade-in duration-200"
    >
      <div
        class="w-full max-w-md bg-surface border-2 border-accent/40 rounded-2xl shadow-2xl shadow-accent/15 overflow-hidden flex flex-col animate-in zoom-in-95 duration-200"
      >
        <!-- Top accent banner -->
        <div class="bg-gradient-to-r from-accent/80 via-accent to-accent/80 px-4 py-2 flex items-center justify-between text-background font-mono text-xs font-bold uppercase tracking-wider">
          <div class="flex items-center space-x-2">
            <Sparkles class="w-4 h-4 shrink-0" />
            <span>{{ $t('updates.modalTitle') }}</span>
          </div>
          <button
            v-if="!isUpdating"
            type="button"
            class="text-background/80 hover:text-background transition-colors focus:outline-none"
            @click="dismissModal"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="p-6 space-y-5">
          <!-- Icon & Header Info -->
          <div class="flex items-center space-x-4">
            <div class="w-13 h-13 rounded-2xl bg-accent/15 border border-accent/30 flex items-center justify-center text-accent shrink-0">
              <ArrowUpCircle class="w-7 h-7" />
            </div>
            <div>
              <h2 class="text-lg font-bold font-sans tracking-wide text-primary uppercase">
                {{ $t('updates.modalSubtitle') }}
              </h2>
              <div class="flex items-center space-x-2 mt-1">
                <span class="text-xs text-primary/60 font-mono">{{ $t('updates.currentVersion') }}:</span>
                <BaseBadge variant="neutral" size="sm">
                  v{{ updateInfo?.currentVersion }}
                </BaseBadge>
                <span class="text-xs text-primary/40 font-mono">→</span>
                <BaseBadge variant="brand" size="sm">
                  v{{ updateInfo?.latestVersion }}
                </BaseBadge>
              </div>
            </div>
          </div>

          <!-- Description Box -->
          <div class="bg-surface-secondary/70 border border-accent-secondary/20 rounded-xl p-4 space-y-2 text-sm text-primary/80">
            <p>{{ $t('updates.updatePrompt') }}</p>
            <div v-if="assetSizeMB" class="text-xs text-primary/50 font-mono">
              Размер пакета: {{ assetSizeMB }}
            </div>
          </div>

          <!-- Updating Status / Spinner -->
          <div v-if="isUpdating" class="bg-accent/10 border border-accent/30 rounded-xl p-4 text-center space-y-2 animate-pulse">
            <div class="flex items-center justify-center space-x-2 text-accent font-semibold text-sm">
              <LoaderCircle class="w-4 h-4 animate-spin" />
              <span>{{ $t('updates.downloading') }}</span>
            </div>
            <p class="text-xs text-primary/60">
              {{ $t('updates.restarting') }}
            </p>
          </div>

          <!-- Error Alert -->
          <div
            v-if="updateError"
            class="bg-ui-danger/15 border border-ui-danger/30 rounded-xl p-3 flex items-start space-x-2.5 text-xs text-ui-danger"
          >
            <AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
            <div class="flex-1">
              <p class="font-semibold">{{ $t('updates.error') }}</p>
              <p class="font-mono mt-0.5 opacity-90">{{ updateError }}</p>
            </div>
          </div>

          <!-- Action Buttons -->
          <div v-if="!isUpdating" class="flex items-center justify-end space-x-3 pt-2">
            <BaseButton
              variant="secondary"
              size="md"
              @click="dismissModal"
            >
              {{ $t('updates.remindLater') }}
            </BaseButton>
            <BaseButton
              variant="tactical"
              size="md"
              class="space-x-1.5"
              @click="applyUpdate"
            >
              <Download class="w-4 h-4 inline mr-1" />
              <span>{{ $t('updates.updateNow') }}</span>
            </BaseButton>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
