<script setup lang="ts">
import { ref, computed, onBeforeUnmount } from 'vue'
import { Info } from 'lucide-vue-next'

interface Props {
  title?: string
  content?: string
  description?: string
  example?: string
  hotkey?: string
  position?: 'top' | 'bottom' | 'left' | 'right'
  customClass?: string
  delay?: number
}

const props = withDefaults(defineProps<Props>(), {
  title: '',
  content: '',
  description: '',
  example: '',
  hotkey: '',
  position: 'top',
  customClass: '',
  delay: 100
})

const effectiveContent = computed(() => props.content || props.description || '')
const isVisible = ref(false)
let timer: ReturnType<typeof setTimeout> | null = null

const show = () => {
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => {
    isVisible.value = true
  }, props.delay)
}

const hide = () => {
  if (timer) clearTimeout(timer)
  isVisible.value = false
}

onBeforeUnmount(() => {
  if (timer) clearTimeout(timer)
})

const positionClasses = computed(() => {
  switch (props.position) {
    case 'bottom':
      return 'top-full left-1/2 -translate-x-1/2 mt-2'
    case 'left':
      return 'right-full top-1/2 -translate-y-1/2 mr-2'
    case 'right':
      return 'left-full top-1/2 -translate-y-1/2 ml-2'
    case 'top':
    default:
      return 'bottom-full left-1/2 -translate-x-1/2 mb-2'
  }
})
</script>

<template>
  <div
    class="relative inline-flex items-center text-left"
    @mouseenter="show"
    @mouseleave="hide"
    @focusin="show"
    @focusout="hide"
  >
    <!-- Trigger Slot or Default Info Icon -->
    <div
      tabindex="0"
      class="inline-flex items-center focus:outline-none rounded transition-colors group"
      aria-haspopup="true"
      :aria-expanded="isVisible"
    >
      <slot name="trigger">
        <slot>
          <span class="inline-flex items-center justify-center w-4 h-4 rounded-full bg-surface-alt/70 text-accent/80 hover:text-accent hover:bg-surface-alt transition-all border border-accent/30 hover:border-accent shadow-sm cursor-help">
            <Info class="w-3 h-3" />
          </span>
        </slot>
      </slot>
    </div>

    <!-- Rich Tooltip Popup -->
    <transition
      enter-active-class="transition duration-150 ease-out"
      enter-from-class="opacity-0 translate-y-1 scale-95"
      enter-to-class="opacity-100 translate-y-0 scale-100"
      leave-active-class="transition duration-100 ease-in"
      leave-from-class="opacity-100 translate-y-0 scale-100"
      leave-to-class="opacity-0 translate-y-1 scale-95"
    >
      <div
        v-if="isVisible && (title || effectiveContent)"
        role="tooltip"
        :class="[
          'ci-rich-tooltip absolute z-50 pointer-events-none w-72 max-w-[calc(100vw-2rem)] p-3 rounded-xl shadow-2xl',
          positionClasses,
          customClass
        ]"
      >
        <!-- Title & Hotkey -->
        <div v-if="title || hotkey" class="font-sans font-bold text-white text-xs tracking-wider uppercase mb-1.5 flex items-center justify-between border-b border-accent/20 pb-1.5">
          <div class="flex items-center gap-1.5 min-w-0">
            <span class="text-accent text-xs">◆</span>
            <span class="truncate text-accent">{{ title }}</span>
          </div>
          <span v-if="hotkey" class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-surface/80 border border-accent/30 text-accent/90 shrink-0">
            {{ hotkey }}
          </span>
        </div>

        <!-- Content -->
        <div v-if="effectiveContent" class="text-xs text-primary/85 leading-relaxed font-sans whitespace-pre-line">
          {{ effectiveContent }}
        </div>

        <!-- Optional Example -->
        <div v-if="example" class="mt-2 pt-1.5 border-t border-accent-secondary/15 text-[11px] font-mono text-accent/90 bg-black/40 px-2 py-1 rounded">
          <span class="text-primary/40 select-none">Пример: </span>
          <span>{{ example }}</span>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.ci-rich-tooltip {
  border: 1px solid rgba(226, 178, 87, 0.35);
  background: linear-gradient(180deg, rgba(22, 25, 34, 0.98), rgba(9, 11, 17, 0.98));
  box-shadow:
    0 18px 44px rgba(0, 0, 0, 0.75),
    0 0 0 1px rgba(4, 5, 10, 0.8),
    0 0 16px rgba(226, 178, 87, 0.12);
}
</style>
