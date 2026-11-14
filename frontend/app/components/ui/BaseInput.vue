<script setup lang="ts">
interface Props {
  modelValue?: string | number
  label?: string
  placeholder?: string
  type?: string
  disabled?: boolean
  error?: string | null
  hint?: string
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: '',
  label: '',
  placeholder: '',
  type: 'text',
  disabled: false,
  error: null,
  hint: ''
})

const emit = defineEmits<{
  'update:modelValue': [value: any]
}>()

const onInput = (event: Event) => {
  const target = event.target as HTMLInputElement
  emit('update:modelValue', props.type === 'number' ? Number(target.value) : target.value)
}
</script>

<template>
  <div class="space-y-1.5 w-full">
    <label v-if="label" class="block text-xs font-medium text-primary/80 font-sans tracking-wide">
      {{ label }}
    </label>
    <div class="relative">
      <input
        :type="type"
        :value="modelValue"
        :placeholder="placeholder"
        :disabled="disabled"
        @input="onInput"
        :class="[
          'w-full bg-background/60 text-primary placeholder:text-primary/40 border rounded-md px-3 py-2 text-xs font-mono transition-colors focus:outline-none focus:ring-1 disabled:opacity-50 disabled:cursor-not-allowed',
          error
            ? 'border-ui-danger/80 focus:border-ui-danger focus:ring-ui-danger/30'
            : 'border-accent-secondary/25 focus:border-accent focus:ring-accent/30'
        ]"
      />
    </div>
    <p v-if="error" class="text-xs text-ui-danger">{{ error }}</p>
    <p v-else-if="hint" class="text-[11px] text-primary/50">{{ hint }}</p>
  </div>
</template>
