<script setup lang="ts">
import { MapPin, Shield, Flame } from 'lucide-vue-next'
import type { Server } from '~/types'
import BaseModal from '~/components/ui/BaseModal.vue'
import BaseBadge from '~/components/ui/BaseBadge.vue'
import BaseButton from '~/components/ui/BaseButton.vue'
import BaseProgress from '~/components/ui/BaseProgress.vue'

interface Props {
  modelValue: boolean
  server: Server | null
  canSeed?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  canSeed: true
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  seed: [server: Server]
  join: [server: Server]
}>()

const playerPercent = computed(() => {
  if (!props.server || !props.server.maxPlayers) return 0
  return Math.round((props.server.currentPlayers / props.server.maxPlayers) * 100)
})
</script>

<template>
  <BaseModal
    :model-value="modelValue"
    :title="server?.name || $t('modal.detailsTitle')"
    :subtitle="$t('modal.detailsTitle')"
    max-width="lg"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div v-if="server" class="space-y-4 font-sans">
      <!-- Status & Seed Banner -->
      <div class="p-3 bg-surface rounded-lg border border-accent-secondary/20 flex items-center justify-between">
        <div class="flex items-center space-x-2 font-mono text-xs">
          <span class="text-primary/60 uppercase">Статус:</span>
          <BaseBadge :variant="server.status === 'online' ? 'success' : 'neutral'" size="sm">
            {{ server.status.toUpperCase() }}
          </BaseBadge>
          <BaseBadge v-if="server.isSeedingActive" variant="warning" size="sm" dot>
            {{ $t('fleet.statusSeeding') }}
          </BaseBadge>
        </div>

        <div class="text-xs font-mono text-primary/60">
          {{ $t('fleet.ping') }}: <strong class="text-accent">{{ server.pingMs ?? 'N/A' }} ms</strong>
        </div>
      </div>

      <!-- Map & Layer Card -->
      <div class="grid grid-cols-2 gap-3">
        <div class="p-3 bg-surface/80 rounded-lg border border-accent-secondary/15 space-y-1">
          <p class="text-[11px] font-mono text-primary/60 uppercase">{{ $t('modal.currentMap') }}</p>
          <p class="text-sm font-semibold text-primary flex items-center space-x-1.5">
            <MapPin class="w-4 h-4 text-accent" />
            <span>{{ server.currentMap }}</span>
          </p>
          <p class="text-xs text-primary/50 font-mono truncate">{{ server.currentLayer }}</p>
        </div>

        <div class="p-3 bg-surface/80 rounded-lg border border-accent-secondary/15 space-y-1">
          <p class="text-[11px] font-mono text-primary/60 uppercase">{{ server.gameMode || 'Squad AAS / RAAS' }}</p>
          <p class="text-sm font-semibold text-primary">
            {{ server.nextLayer || 'Next layer TBD' }}
          </p>
        </div>
      </div>

      <!-- Teams / Factions (if available) -->
      <div v-if="server.teamOne || server.teamTwo" class="p-3 bg-surface/80 rounded-lg border border-accent-secondary/15 space-y-2">
        <div class="flex items-center justify-between text-xs font-semibold text-primary font-mono">
          <div class="flex items-center space-x-2">
            <Shield class="w-4 h-4 text-sky-400" />
            <span>{{ server.teamOne || 'Faction 1' }}</span>
          </div>
          <span class="text-primary/40 font-normal">VS</span>
          <div class="flex items-center space-x-2">
            <span>{{ server.teamTwo || 'Faction 2' }}</span>
            <Shield class="w-4 h-4 text-squad-orange" />
          </div>
        </div>
      </div>

      <!-- Player Count & Queue Details -->
      <div class="p-3 bg-surface rounded-lg border border-accent-secondary/20 space-y-2">
        <div class="flex justify-between items-center text-xs font-mono">
          <span class="text-primary/70">{{ $t('modal.currentPlayers') }}:</span>
          <span class="text-accent font-bold">{{ server.currentPlayers }} / {{ server.maxPlayers }}</span>
        </div>
        <BaseProgress :value="playerPercent" :show-value="false" variant="brand" />

        <div class="grid grid-cols-2 gap-2 pt-1 text-xs font-mono">
          <div class="flex justify-between">
            <span class="text-primary/60">{{ $t('fleet.queueSlots') }}:</span>
            <span class="text-primary font-semibold">{{ server.publicQueue }}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-primary/60">{{ $t('fleet.target') }}:</span>
            <span class="text-accent font-semibold">{{ server.targetSeedCount > 0 ? server.targetSeedCount : $t('common.unknown') }}</span>
          </div>
        </div>
      </div>

      <!-- Network Endpoint -->
      <div class="p-3 bg-surface/80 rounded-lg border border-accent-secondary/15 flex items-center justify-between font-mono text-xs text-primary/60">
        <span>IP:</span>
        <span class="text-primary font-semibold select-all">{{ server.serverIp }}:{{ server.gamePort }}</span>
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-between w-full">
        <BaseButton variant="ghost" size="sm" @click="emit('update:modelValue', false)">
          {{ $t('common.close') }}
        </BaseButton>

        <div class="flex items-center space-x-2" v-if="server">
          <BaseButton
            variant="secondary"
            size="sm"
            @click="emit('join', server); emit('update:modelValue', false)"
          >
            {{ $t('fleet.join') }}
          </BaseButton>
          <BaseButton
            variant="tactical"
            size="sm"
            :disabled="!canSeed || server.status === 'offline'"
            @click="emit('seed', server); emit('update:modelValue', false)"
          >
            <Flame class="w-3.5 h-3.5 mr-1" />
            <span>{{ $t('fleet.seed') }}</span>
          </BaseButton>
        </div>
      </div>
    </template>
  </BaseModal>
</template>
