<script setup lang="ts">
import { Play, Server, Settings, ShieldCheck, BookOpen, Save, Zap, Sliders, Radio, Gamepad2 } from 'lucide-vue-next'
import { useSeed } from '~/composables/useSeed'
import { useSettings } from '~/composables/useSettings'
import { useI18n } from '~/composables/useI18n'
import { useRoute, useRouter } from 'vue-router'
import BaseButton from '~/components/ui/BaseButton.vue'

const route = useRoute()
const router = useRouter()
const { isSeedingActive } = useSeed()
const { hasUnsavedChanges, saveDraft, isSaving } = useSettings()
const { t } = useI18n()

const isActive = (path: string) => {
  if (path === '/' && route.path === '/') return true
  if (path !== '/' && route.path.startsWith(path)) return true
  return false
}

const currentSettingTab = computed(() => (route.query.tab as string) || 'general')

const settingSections = computed(() => [
  { id: 'general', label: t('settings.tabGeneral') || 'Основные параметры', icon: Sliders },
  { id: 'seed', label: t('settings.tabSeed') || 'Автосидинг', icon: Play },
  { id: 'power', label: t('settings.tabPower') || 'Электропитание', icon: Zap },
  { id: 'potato', label: t('settings.tabPotato') || 'Мод картошки', icon: Gamepad2 },
  { id: 'diagnostics', label: t('settings.tabDiagnostics') || 'Диагностика', icon: Radio }
])

const navigateSettingTab = (tabId: string) => {
  router.push({ path: '/settings', query: { tab: tabId } })
}
</script>

<template>
  <aside class="w-64 bg-surface border-r border-accent-secondary/15 flex flex-col justify-between select-none z-30">
    <!-- Top Navigation Section -->
    <div class="p-3.5 space-y-6">
      <!-- App Header / Logo -->
      <div class="px-2 py-1.5 flex items-center space-x-3">
        <div class="w-10 h-10 rounded-xl bg-surface border border-accent/40 flex items-center justify-center shadow-lg shadow-black/40 overflow-hidden">
          <img src="/img/logo_citadel.png" alt="Citadel" class="w-8 h-8 object-contain drop-shadow-[0_0_8px_rgba(235,225,200,0.4)]" />
        </div>
        <div>
          <h1 class="text-base font-bold text-primary font-sans tracking-wide">CITADEL</h1>
          <p class="text-xs font-sans text-accent font-semibold tracking-widest uppercase">SENTRY</p>
        </div>
      </div>

      <!-- Navigation Links -->
      <nav class="space-y-1.5">
        <!-- Dashboard -->
        <NuxtLink
          to="/"
          :class="[
            'flex items-center justify-between px-3.5 py-3 rounded-xl text-sm font-sans uppercase tracking-wider transition-all group',
            isActive('/')
              ? 'bg-accent/15 text-accent border border-accent/30 font-bold'
              : 'text-primary/80 hover:bg-surface/90 hover:text-primary hover:border-accent-secondary/20'
          ]"
        >
          <div class="flex items-center space-x-3.5">
            <Play :class="['w-5 h-5', isActive('/') ? 'text-accent' : 'text-primary/50 group-hover:text-primary']" />
            <span>{{ $t('nav.dashboard') }}</span>
          </div>
          <span
            v-if="isSeedingActive"
            class="inline-block w-2.5 h-2.5 rounded-full bg-emerald-400 animate-ping"
            title="Seeding active"
          />
        </NuxtLink>

        <!-- Fleet Servers -->
        <NuxtLink
          to="/servers"
          :class="[
            'flex items-center justify-between px-3.5 py-3 rounded-xl text-sm font-sans uppercase tracking-wider transition-all group',
            isActive('/servers')
              ? 'bg-accent/15 text-accent border border-accent/30 font-bold'
              : 'text-primary/80 hover:bg-surface/90 hover:text-primary hover:border-accent-secondary/20'
          ]"
        >
          <div class="flex items-center space-x-3.5">
            <Server :class="['w-5 h-5', isActive('/servers') ? 'text-accent' : 'text-primary/50 group-hover:text-primary']" />
            <span>{{ $t('nav.fleet') }}</span>
          </div>
        </NuxtLink>

        <!-- Settings with Sub-sections -->
        <div class="space-y-1">
          <NuxtLink
            to="/settings"
            :class="[
              'flex items-center justify-between px-3.5 py-3 rounded-xl text-sm font-sans uppercase tracking-wider transition-all group',
              isActive('/settings')
                ? 'bg-accent/15 text-accent border border-accent/30 font-bold'
                : 'text-primary/80 hover:bg-surface/90 hover:text-primary hover:border-accent-secondary/20'
            ]"
          >
            <div class="flex items-center space-x-3.5">
              <Settings :class="['w-5 h-5', isActive('/settings') ? 'text-accent' : 'text-primary/50 group-hover:text-primary']" />
              <span>{{ $t('nav.settings') }}</span>
            </div>
          </NuxtLink>

          <!-- Settings Sub-items when on settings -->
          <div v-if="isActive('/settings')" class="ml-4 pl-3 border-l border-accent-secondary/20 space-y-1 py-1 animate-in fade-in duration-150">
            <button
              v-for="sub in settingSections"
              :key="sub.id"
              type="button"
              :class="[
                'w-full text-left px-2.5 py-1.5 rounded-lg text-xs font-sans tracking-wide transition-colors flex items-center space-x-2.5',
                currentSettingTab === sub.id
                  ? 'text-accent font-bold bg-accent/10'
                  : 'text-primary/60 hover:text-primary hover:bg-surface/60'
              ]"
              @click="navigateSettingTab(sub.id)"
            >
              <component :is="sub.icon" class="w-3.5 h-3.5 flex-shrink-0" />
              <span class="truncate">{{ sub.label }}</span>
            </button>
          </div>
        </div>

        <!-- Guide / Manual -->
        <NuxtLink
          to="/guide"
          :class="[
            'flex items-center justify-between px-3.5 py-3 rounded-xl text-sm font-sans uppercase tracking-wider transition-all group',
            isActive('/guide')
              ? 'bg-accent/15 text-accent border border-accent/30 font-bold'
              : 'text-primary/80 hover:bg-surface/90 hover:text-primary hover:border-accent-secondary/20'
          ]"
        >
          <div class="flex items-center space-x-3.5">
            <BookOpen :class="['w-5 h-5', isActive('/guide') ? 'text-accent' : 'text-primary/50 group-hover:text-primary']" />
            <span>{{ $t('nav.guide') }}</span>
          </div>
        </NuxtLink>
      </nav>
    </div>

    <!-- Bottom Status / System Badge -->
    <div class="p-3.5 border-t border-accent-secondary/15 bg-surface/60">
      <div v-if="isActive('/settings') && hasUnsavedChanges" class="animate-in fade-in duration-150">
        <BaseButton
          variant="tactical"
          size="sm"
          class="w-full justify-center"
          :loading="isSaving"
          @click="saveDraft"
        >
          <Save class="w-3.5 h-3.5 mr-1.5" />
          <span>{{ $t('settings.saveChanges') }}</span>
        </BaseButton>
      </div>
      <div v-else class="flex items-center space-x-2.5 px-3 py-2 rounded-xl bg-surface/80 border border-accent-secondary/15 text-xs font-mono">
        <ShieldCheck class="w-4 h-4 text-accent flex-shrink-0" />
        <div class="truncate">
          <p class="text-primary font-semibold truncate font-sans">Citadel Sentry</p>
          <p class="text-[11px] text-emerald-400 font-mono">{{ $t('dashboard.standbyReady') }}</p>
        </div>
      </div>
    </div>
  </aside>
</template>