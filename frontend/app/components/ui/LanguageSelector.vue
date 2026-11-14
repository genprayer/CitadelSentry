<script setup lang="ts">
import { Globe } from 'lucide-vue-next'
import { useI18n, type LocaleCode } from '~/composables/useI18n'

const { locale, locales, setLocale } = useI18n()

const isOpen = ref(false)
const containerRef = ref<HTMLElement | null>(null)

const toggleOpen = () => {
  isOpen.value = !isOpen.value
}

const selectLocale = (code: LocaleCode) => {
  setLocale(code)
  isOpen.value = false
}

// Close on outside click
const handleClickOutside = (event: MouseEvent) => {
  if (containerRef.value && !containerRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  if (typeof window !== 'undefined') {
    window.addEventListener('click', handleClickOutside)
  }
})

onBeforeUnmount(() => {
  if (typeof window !== 'undefined') {
    window.removeEventListener('click', handleClickOutside)
  }
})
</script>

<template>
  <div ref="containerRef" class="relative inline-block text-left font-sans">
    <button
      type="button"
      class="inline-flex items-center space-x-1.5 px-2.5 py-1 text-xs font-semibold uppercase tracking-wider rounded border border-accent-secondary/25 bg-surface/80 hover:bg-surface text-primary hover:border-accent transition-all duration-200"
      @click.stop="toggleOpen"
    >
      <Globe class="w-3.5 h-3.5 text-accent" />
      <span>{{ locale.toUpperCase() }}</span>
    </button>

    <!-- Dropdown -->
    <transition
      enter-active-class="transition duration-100 ease-out"
      enter-from-class="transform scale-95 opacity-0"
      enter-to-class="transform scale-100 opacity-100"
      leave-active-class="transition duration-75 ease-in"
      leave-from-class="transform scale-100 opacity-100"
      leave-to-class="transform scale-95 opacity-0"
    >
      <div
        v-if="isOpen"
        class="absolute right-0 z-50 mt-1.5 w-36 rounded-md shadow-2xl bg-surface border border-accent-secondary/30 overflow-hidden"
      >
        <div class="py-1">
          <button
            v-for="item in locales"
            :key="item.code"
            type="button"
            class="w-full text-left px-3 py-2 text-xs flex items-center space-x-2.5 transition-colors"
            :class="[
              locale === item.code
                ? 'bg-accent/15 text-accent font-bold border-l-2 border-accent'
                : 'text-primary/80 hover:bg-surface/90 hover:text-primary'
            ]"
            @click="selectLocale(item.code)"
          >
            <span class="text-sm">{{ item.flag }}</span>
            <span class="tracking-wide">{{ item.name }}</span>
          </button>
        </div>
      </div>
    </transition>
  </div>
</template>
