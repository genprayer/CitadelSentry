<script setup lang="ts">
interface Props {
  value: number // 0 to 100
  label?: string
  showValue?: boolean
  variant?: 'brand' | 'success' | 'warning' | 'danger'
  animated?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  value: 0,
  label: '',
  showValue: true,
  variant: 'brand',
  animated: false
})

const clampedValue = computed(() => Math.min(100, Math.max(0, props.value)))

const barColor = computed(() => {
  switch (props.variant) {
    case 'success':
      return 'bg-ui-success'
    case 'warning':
      return 'bg-gradient-to-r from-squad-orange to-squad-yellow'
    case 'danger':
      return 'bg-ui-danger'
    case 'brand':
    default:
      return 'bg-gradient-to-r from-accent-secondary to-accent'
  }
})
</script>

<template>
  <div class="w-full space-y-1.5">
    <div v-if="label || showValue" class="flex justify-between items-center text-xs font-mono text-primary/80">
      <span v-if="label">{{ label }}</span>
      <span v-if="showValue" class="font-bold text-primary">{{ clampedValue }}%</span>
    </div>
    <div class="h-2 w-full bg-surface/90 rounded-full overflow-hidden border border-accent-secondary/20">
      <div
        :class="[
          'h-full transition-all duration-300 rounded-full',
          barColor,
          animated ? 'animate-pulse' : ''
        ]"
        :style="{ width: `${clampedValue}%` }"
      />
    </div>
  </div>
</template>
