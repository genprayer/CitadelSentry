<script setup lang="ts">
interface Props {
  variant?: 'default' | 'panel' | 'elevated' | 'glass'
  padding?: 'none' | 'sm' | 'md' | 'lg'
  bordered?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'default',
  padding: 'md',
  bordered: true
})

const variantClass = computed(() => {
  switch (props.variant) {
    case 'panel':
      return 'bg-surface'
    case 'elevated':
      return 'bg-surface shadow-lg shadow-black/50'
    case 'glass':
      return 'citadel-card shadow-lg shadow-black/40'
    case 'default':
    default:
      return 'bg-surface shadow-md shadow-black/30'
  }
})

const paddingClass = computed(() => {
  switch (props.padding) {
    case 'none':
      return 'p-0'
    case 'sm':
      return 'p-3'
    case 'lg':
      return 'p-6'
    case 'md':
    default:
      return 'p-4'
  }
})
</script>

<template>
  <div
    :class="[
      'rounded-xl transition-colors duration-150',
      variantClass,
      paddingClass,
      bordered ? 'border border-accent-secondary/15 hover:border-accent-secondary/30' : ''
    ]"
  >
    <slot />
  </div>
</template>
