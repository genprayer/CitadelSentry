<script setup lang="ts">
import { Power } from 'lucide-vue-next'
import type { PowerSettings } from '~/types'
import BaseCard from '~/components/ui/BaseCard.vue'
import BaseToggle from '~/components/ui/BaseToggle.vue'

interface Props {
  modelValue: PowerSettings
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: PowerSettings]
}>()

const updateField = <K extends keyof PowerSettings>(key: K, value: PowerSettings[K]) => {
  emit('update:modelValue', {
    ...props.modelValue,
    [key]: value
  })
}

</script>

<template>
  <BaseCard variant="panel" padding="lg" class="space-y-4">
    <div class="flex items-center space-x-2.5 pb-3 border-b border-accent-secondary/15">
      <Power class="w-4 h-4 text-accent" />
      <div>
        <h3 class="text-sm font-semibold text-primary font-sans uppercase tracking-wide">
          {{ $t('settings.powerTitle') }}
        </h3>
        <p class="text-xs text-primary/60">{{ $t('settings.powerDesc') }}</p>
      </div>
    </div>

    <div class="space-y-4">
      <BaseToggle
        :model-value="modelValue.preventSleepDuringSeed"
        :label="$t('settings.preventSleep')"
        :description="$t('settings.preventSleepDesc')"
        @update:model-value="updateField('preventSleepDuringSeed', $event)"
      />

    </div>
  </BaseCard>
</template>
