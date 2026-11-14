<script setup lang="ts">
import { useId } from 'vue'

interface Option {
  label: string
  value: string | number
  disabled?: boolean
}

interface Props {
  modelValue?: string | number
  label?: string
  hint?: string
  options: readonly Option[]
  disabled?: boolean
  error?: string | null
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: '',
  label: '',
  hint: '',
  disabled: false,
  error: null
})

const selectId = useId()
const feedbackId = `${selectId}-feedback`

const emit = defineEmits<{
  'update:modelValue': [value: any]
}>()

const onChange = (event: Event) => {
  const target = event.target as HTMLSelectElement
  emit('update:modelValue', target.value)
}
</script>

<template>
  <div class="space-y-1.5 w-full">
    <label v-if="label" :for="selectId" class="block text-xs font-medium text-primary/80 font-sans tracking-wide">
      {{ label }}
    </label>
    <select
      :id="selectId"
      :aria-describedby="error || hint ? feedbackId : undefined"
      :aria-invalid="error ? true : undefined"
      :value="modelValue"
      :disabled="disabled"
      @change="onChange"
      :class="[
        'w-full bg-background/80 text-primary border rounded-md px-3 py-2 text-xs font-mono transition-colors focus:outline-none focus:ring-1 disabled:opacity-50 disabled:cursor-not-allowed',
        error
          ? 'border-ui-danger/80 focus:border-ui-danger focus:ring-ui-danger/30'
          : 'border-accent-secondary/25 focus:border-accent focus:ring-accent/30'
      ]"
    >
      <option
        v-for="opt in options"
        :key="String(opt.value)"
        :value="opt.value"
        :disabled="opt.disabled"
        class="bg-surface text-primary py-1 disabled:text-primary/30"
      >
        {{ opt.label }}
      </option>
    </select>
    <p v-if="error" :id="feedbackId" class="text-xs text-ui-danger">{{ error }}</p>
    <p v-else-if="hint" :id="feedbackId" class="text-xs leading-relaxed text-primary/50">{{ hint }}</p>
  </div>
</template>
