<script setup lang="ts">
import { Sliders, ArrowUpCircle, CheckCircle2, AlertCircle } from 'lucide-vue-next'
import type { GeneralSettings } from '~/types'
import { useI18n, type LocaleCode } from '~/composables/useI18n'
import { useUpdater } from '~/composables/useUpdater'
import { useBridge } from '~/composables/useBridge'
import BaseCard from '~/components/ui/BaseCard.vue'
import BaseToggle from '~/components/ui/BaseToggle.vue'
import BaseSelect from '~/components/ui/BaseSelect.vue'
import BaseButton from '~/components/ui/BaseButton.vue'

interface Props {
  modelValue: GeneralSettings
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: GeneralSettings]
}>()

const { locale, locales, setLocale } = useI18n()
const { isChecking, updateInfo, updateError, checkMessage, checkForUpdates } = useUpdater()
const { status } = useBridge()

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

      <div class="pt-3">
        <BaseToggle
          :model-value="modelValue.autoUpdateCheck !== false"
          :label="$t('settings.autoUpdateCheck')"
          :description="$t('settings.autoUpdateCheckDesc')"
          @update:model-value="updateField('autoUpdateCheck', $event)"
        />
      </div>
    </div>

    <!-- Manual Update Check Section -->
    <div class="pt-4 border-t border-accent-secondary/15 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
      <div>
        <div class="flex items-center space-x-2 text-xs font-semibold text-primary uppercase font-sans">
          <ArrowUpCircle class="w-3.5 h-3.5 text-accent" />
          <span>{{ $t('settings.checkUpdatesTitle') }}</span>
        </div>
        <p class="text-[11px] text-primary/50 mt-0.5">
          {{ $t('settings.checkUpdatesDesc') }}
          <span v-if="status?.appVersion" class="font-mono text-primary/70">
            (v{{ status.appVersion }})
          </span>
        </p>

        <!-- Inline Status Feedback -->
        <div v-if="checkMessage === 'upToDate'" class="flex items-center space-x-1.5 text-xs text-accent mt-2 font-mono">
          <CheckCircle2 class="w-3.5 h-3.5" />
          <span>{{ $t('settings.upToDate') }}</span>
        </div>

        <div v-if="updateError" class="flex items-center space-x-1.5 text-xs text-ui-danger mt-2 font-mono">
          <AlertCircle class="w-3.5 h-3.5" />
          <span>{{ updateError }}</span>
        </div>

        <div v-if="updateInfo?.available" class="flex items-center space-x-1.5 text-xs text-amber-400 mt-2 font-mono">
          <ArrowUpCircle class="w-3.5 h-3.5" />
          <span>{{ $t('settings.updateAvailable', { version: updateInfo.latestVersion }) }}</span>
        </div>
      </div>

      <BaseButton
        variant="secondary"
        size="sm"
        :loading="isChecking"
        @click="checkForUpdates(false)"
      >
        {{ $t('settings.checkNow') }}
      </BaseButton>
    </div>
  </BaseCard>
</template>
