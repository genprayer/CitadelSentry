<script setup lang="ts">
import { Loader2 } from 'lucide-vue-next'

interface Props {
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost' | 'tactical'
  size?: 'xs' | 'sm' | 'md' | 'lg'
  disabled?: boolean
  loading?: boolean
  type?: 'button' | 'submit' | 'reset'
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'primary',
  size: 'md',
  disabled: false,
  loading: false,
  type: 'button'
})

const variantClasses = computed(() => {
  switch (props.variant) {
    case 'primary':
      return 'bg-accent hover:bg-accent/90 text-background font-bold tracking-wider uppercase shadow-md shadow-black/40 border border-accent/40 active:scale-[0.98]'
    case 'tactical':
      return 'bg-gradient-to-r from-squad-orange to-squad-yellow hover:brightness-105 text-squad-dark font-bold tracking-wider uppercase shadow-lg shadow-squad-orange/25 border border-squad-yellow/50 active:scale-[0.98]'
    case 'secondary':
      return 'bg-surface/80 hover:bg-surface text-primary border border-accent-secondary/30 hover:border-accent active:scale-[0.98]'
    case 'danger':
      return 'bg-ui-danger hover:bg-ui-danger/90 text-white font-bold tracking-wider uppercase shadow-md shadow-ui-danger/30 border border-ui-danger/40 active:scale-[0.98]'
    case 'ghost':
      return 'bg-transparent hover:bg-surface/60 text-primary/70 hover:text-accent'
  }
})

const sizeClasses = computed(() => {
  switch (props.size) {
    case 'xs':
      return 'px-2.5 py-1 text-[11px] rounded'
    case 'sm':
      return 'px-3 py-1.5 text-xs rounded-md'
    case 'lg':
      return 'px-6 py-3 text-sm rounded-lg tracking-wider'
    case 'md':
    default:
      return 'px-4 py-2 text-xs rounded-md tracking-wider'
  }
})
</script>

<template>
  <button
    :type="type"
    :disabled="disabled || loading"
    :class="[
      'inline-flex items-center justify-center font-sans uppercase transition-all duration-150 focus:outline-none focus:ring-2 focus:ring-accent/40 select-none disabled:opacity-50 disabled:cursor-not-allowed disabled:active:scale-100',
      variantClasses,
      sizeClasses
    ]"
  >
    <Loader2 v-if="loading" class="w-4 h-4 mr-2 animate-spin text-current" />
    <slot />
  </button>
</template>
