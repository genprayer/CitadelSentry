<script setup lang="ts">
import { X } from 'lucide-vue-next'

interface Props {
  modelValue: boolean
  title?: string
  subtitle?: string
  maxWidth?: 'sm' | 'md' | 'lg' | 'xl'
  closeOnBackdrop?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  title: '',
  subtitle: '',
  maxWidth: 'md',
  closeOnBackdrop: false
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  close: []
}>()

const close = () => {
  emit('update:modelValue', false)
  emit('close')
}

const maxWidthClass = computed(() => {
  switch (props.maxWidth) {
    case 'sm':
      return 'max-w-sm'
    case 'lg':
      return 'max-w-2xl'
    case 'xl':
      return 'max-w-4xl'
    case 'md':
    default:
      return 'max-w-lg'
  }
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="modelValue"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-background/90"
      @click.self="closeOnBackdrop ? close() : undefined"
    >
      <div
        :class="[
          'w-full bg-surface border border-accent-secondary/20 rounded-xl shadow-xl overflow-hidden flex flex-col max-h-[90vh] animate-in fade-in zoom-in-95 duration-150',
          maxWidthClass
        ]"
      >
        <!-- Header -->
        <div class="px-5 py-4 border-b border-accent-secondary/15 flex items-center justify-between bg-surface">
          <div>
            <h3 v-if="title" class="text-sm font-bold font-sans tracking-wider uppercase text-primary">{{ title }}</h3>
            <p v-if="subtitle" class="text-xs text-primary/60 font-mono mt-0.5">{{ subtitle }}</p>
          </div>
          <button
            type="button"
            class="p-1 rounded-md text-primary/50 hover:text-primary hover:bg-surface/80 transition-colors"
            @click="close"
          >
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Content -->
        <div
          class="p-5 overflow-y-auto flex-1 space-y-4 text-primary/90"
          style="-webkit-overflow-scrolling: touch; transform: translateZ(0); will-change: scroll-position;"
        >
          <slot />
        </div>

        <!-- Footer -->
        <div v-if="$slots.footer" class="px-5 py-3 border-t border-accent-secondary/15 bg-background/50 flex items-center justify-end space-x-3">
          <slot name="footer" />
        </div>
      </div>
    </div>
  </Teleport>
</template>
