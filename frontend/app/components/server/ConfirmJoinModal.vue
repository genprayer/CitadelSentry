<script setup lang="ts">
import { AlertTriangle, LogIn } from 'lucide-vue-next'
import type { Server } from '~/types'
import BaseModal from '~/components/ui/BaseModal.vue'
import BaseButton from '~/components/ui/BaseButton.vue'

interface Props {
  modelValue: boolean
  activeServerName: string
  targetServer: Server | null
  isLoading?: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  confirm: []
}>()
</script>

<template>
  <BaseModal
    :model-value="modelValue"
    :title="$t('confirmJoin.title')"
    max-width="sm"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div v-if="targetServer" class="space-y-4 font-sans text-xs">
      <div class="p-3 bg-amber-500/10 border border-amber-500/30 rounded-xl flex items-start space-x-3 text-amber-200">
        <AlertTriangle class="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5" />
        <p class="leading-relaxed">
          {{ $t('confirmJoin.description', { activeServer: activeServerName, targetServer: targetServer.name }) }}
        </p>
      </div>

      <div class="p-3 bg-surface rounded-xl border border-accent-secondary/15 font-mono space-y-1">
        <div class="flex justify-between text-primary/60">
          <span>Целевой сервер:</span>
          <span class="text-primary font-bold truncate max-w-[180px]">{{ targetServer.name }}</span>
        </div>
        <div class="flex justify-between text-primary/60">
          <span>Карта:</span>
          <span class="text-primary truncate max-w-[180px]">{{ targetServer.currentMap }}</span>
        </div>
        <div class="flex justify-between text-primary/60">
          <span>Игроки:</span>
          <span class="text-accent font-bold">{{ targetServer.currentPlayers }} / {{ targetServer.maxPlayers }}</span>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-end space-x-2 w-full">
        <BaseButton
          variant="ghost"
          size="sm"
          @click="emit('update:modelValue', false)"
        >
          {{ $t('confirmJoin.cancelButton') }}
        </BaseButton>
        <BaseButton
          variant="danger"
          size="sm"
          :loading="isLoading"
          @click="emit('confirm')"
        >
          <LogIn class="w-3.5 h-3.5 mr-1" />
          <span>{{ $t('confirmJoin.confirmButton') }}</span>
        </BaseButton>
      </div>
    </template>
  </BaseModal>
</template>
