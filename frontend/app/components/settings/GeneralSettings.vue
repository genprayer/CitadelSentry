<script setup lang="ts">
import { Sliders, Globe } from 'lucide-vue-next'
import type { GeneralSettings } from '~/types'
import { useI18n, type LocaleCode } from '~/composables/useI18n'
import BaseCard from '~/components/ui/BaseCard.vue'
import BaseToggle from '~/components/ui/BaseToggle.vue'
import BaseSelect from '~/components/ui/BaseSelect.vue'

interface Props {
  modelValue: GeneralSettings
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: GeneralSettings]
}>()

const { locale, locales, setLocale } = useI18n()

const languageOptions = locales.map(l => ({
  value: l.code,
  label: `${l.flag} ${l.name}`
}))

const updateField = <K extends keyof GeneralSettings>(key: K, value: GeneralSettings[K]) => {
  emit('update:modelValue', {
    ...props.modelValue,
    [key]: value
  })
}

const handleLanguageChange = (val: string | number) => {
  const code = String(val) as LocaleCode
  setLocale(code)
  updateField('language', code)
}
</script>

<template>
  <BaseCard variant="panel" padding="lg" class="space-y-4">
    <div class="flex items-center space-x-2.5 pb-3 border-b border-accent-secondary/15">
      <Sliders class="w-4 h-4 text-accent" />
      <div>
        <h3 class="text-sm font-semibold text-primary font-sans uppercase tracking-wide">
          {{ $t('settings.generalTitle') }}
        </h3>
        <p class="text-xs text-primary/60">{{ $t('settings.generalDesc') }}</p>
      </div>
    </div>

    <!-- Language Selector inside Settings -->
    <div class="pb-3 border-b border-accent-secondary/15 max-w-sm">
      <BaseSelect
        :model-value="modelValue.language || locale"
        :options="languageOptions"
        :label="$t('settings.language')"
        @update:model-value="handleLanguageChange"
      />
      <p class="text-[11px] text-primary/50 mt-1">{{ $t('settings.languageDesc') }}</p>
    </div>

    <div class="space-y-4 divide-y divide-accent-secondary/15">
      <BaseToggle
        :model-value="modelValue.launchOnStartup"
        :label="$t('settings.launchOnStartup')"
        :description="$t('settings.launchOnStartupDesc')"
        @update:model-value="updateField('launchOnStartup', $event)"
      />

      <div class="pt-3">
        <BaseToggle
          :model-value="modelValue.minimizeToTray"
          :label="$t('settings.minimizeToTray')"
          :description="$t('settings.minimizeToTrayDesc')"
          @update:model-value="updateField('minimizeToTray', $event)"
        />
      </div>

      <div class="pt-3">
        <BaseToggle
          :model-value="modelValue.closeToTray"
          :label="$t('settings.closeToTray')"
          :description="$t('settings.closeToTrayDesc')"
          @update:model-value="updateField('closeToTray', $event)"
        />
      </div>

      <div class="pt-3">
        <BaseToggle
          :model-value="modelValue.notificationsEnabled"
          :label="$t('settings.notifications')"
          :description="$t('settings.notificationsDesc')"
          @update:model-value="updateField('notificationsEnabled', $event)"
        />
      </div>

      <div class="pt-3">
        <BaseToggle
          :model-value="modelValue.soundEnabled"
          :label="$t('settings.sound')"
          :description="$t('settings.soundDesc')"
          @update:model-value="updateField('soundEnabled', $event)"
        />
      </div>
    </div>
  </BaseCard>
</template>
