<script setup lang="ts">
import { Flame } from 'lucide-vue-next'
import type { SeedSettings, PostSeedAction } from '~/types'
import { POST_SEED_ACTION_OPTIONS } from '~/constants/seed'
import { useI18n } from '~/composables/useI18n'
import BaseCard from '~/components/ui/BaseCard.vue'
import BaseInput from '~/components/ui/BaseInput.vue'
import BaseSelect from '~/components/ui/BaseSelect.vue'

interface Props {
  modelValue: SeedSettings
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: SeedSettings]
}>()

const { t } = useI18n()

const postSeedOptions = computed(() => POST_SEED_ACTION_OPTIONS.map(option => ({
  label: t(option.labelKey),
  value: option.value
})))

const updateField = <K extends keyof SeedSettings>(key: K, value: SeedSettings[K]) => {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}
</script>

<template>
  <BaseCard variant="panel" padding="lg" class="space-y-4">
    <div class="flex items-center space-x-2.5 pb-3 border-b border-accent-secondary/15">
      <Flame class="w-4 h-4 text-squad-orange" />
      <div>
        <h3 class="text-sm font-semibold text-primary font-sans uppercase tracking-wide">
          {{ $t('settings.seedTitle') }}
        </h3>
        <p class="text-xs text-primary/60">{{ $t('settings.seedDesc') }}</p>
      </div>
    </div>

    <div class="space-y-4">
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <BaseInput
          :model-value="modelValue.defaultTargetPlayers"
          type="number"
          :label="$t('settings.defaultTargetPlayers')"
          :hint="$t('settings.defaultTargetPlayersDesc')"
          @update:model-value="updateField('defaultTargetPlayers', Number($event))"
        />
        <BaseInput
          :model-value="modelValue.defaultMaxDurationMinutes"
          type="number"
          :label="$t('settings.defaultMaxDuration')"
          :hint="$t('settings.defaultMaxDurationDesc')"
          @update:model-value="updateField('defaultMaxDurationMinutes', Number($event))"
        />
      </div>

      <div class="pt-2">
        <BaseSelect
          :model-value="modelValue.defaultPostSeedAction"
          :label="$t('settings.postSeedAction')"
          :hint="$t('settings.postSeedActionDesc')"
          :options="postSeedOptions"
          @update:model-value="updateField('defaultPostSeedAction', $event as PostSeedAction)"
        />
      </div>
    </div>
  </BaseCard>
</template>
