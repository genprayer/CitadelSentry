<script setup lang="ts">
interface Props {
  variant?: 'brand' | 'success' | 'warning' | 'danger' | 'info' | 'neutral'
  size?: 'sm' | 'md'
  dot?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'neutral',
  size: 'md',
  dot: false
})

const variantClass = computed(() => {
  switch (props.variant) {
    case 'brand':
      return 'bg-accent/15 text-accent border-accent/40'
    case 'success':
      return 'bg-ui-success/15 text-ui-success border-ui-success/40'
    case 'warning':
      return 'bg-squad-orange/15 text-squad-orange border-squad-orange/40'
    case 'danger':
      return 'bg-ui-danger/15 text-ui-danger border-ui-danger/40'
    case 'info':
      return 'bg-accent-secondary/15 text-accent-secondary border-accent-secondary/40'
    case 'neutral':
    default:
      return 'bg-surface/90 text-primary/70 border-accent-secondary/20'
  }
})

const dotColor = computed(() => {
  switch (props.variant) {
    case 'brand':
      return 'bg-accent'
    case 'success':
      return 'bg-ui-success'
    case 'warning':
      return 'bg-squad-orange'
    case 'danger':
      return 'bg-ui-danger'
    case 'info':
      return 'bg-accent-secondary'
    case 'neutral':
    default:
      return 'bg-primary/50'
  }
})
</script>

<template>
  <span
    :class="[
      'inline-flex items-center font-bold font-sans border rounded-md uppercase tracking-wider',
      size === 'sm' ? 'px-2 py-0.5 text-[10px]' : 'px-2.5 py-1 text-xs',
      variantClass
    ]"
  >
    <span v-if="dot" :class="['w-1.5 h-1.5 rounded-full mr-1.5 animate-pulse', dotColor]" />
    <slot />
  </span>
</template>
