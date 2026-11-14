<script setup lang="ts">
interface Props {
  modelValue: boolean
  label?: string
  description?: string
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: false,
  label: '',
  description: '',
  disabled: false
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const toggle = () => {
  if (!props.disabled) {
    emit('update:modelValue', !props.modelValue)
  }
}
</script>

<template>
  <div
    class="flex items-center justify-between cursor-pointer py-1.5 select-none"
    :class="{ 'opacity-50 cursor-not-allowed': disabled }"
    @click="toggle"
  >
    <div class="space-y-0.5 pr-4">
      <p v-if="label" class="text-xs font-bold text-primary font-sans uppercase tracking-wide">{{ label }}</p>
      <p v-if="description" class="text-[11px] text-primary/60 leading-relaxed">{{ description }}</p>
    </div>
    <button
      type="button"
      role="switch"
      :aria-checked="modelValue"
      :disabled="disabled"
      :class="[
        'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent/40',
        modelValue ? 'bg-accent border-accent' : 'bg-surface border-accent-secondary/30'
      ]"
    >
      <span
        :class="[
          'pointer-events-none inline-block h-5 w-5 transform rounded-full shadow ring-0 transition duration-200 ease-in-out',
          modelValue ? 'translate-x-5 bg-background' : 'translate-x-0 bg-primary/50'
        ]"
      />
    </button>
  </div>
</template>
