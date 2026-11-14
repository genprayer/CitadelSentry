<script setup lang="ts">
await navigateTo('/settings?tab=potato', { replace: true })
</script>

<template>
  <div class="h-full flex items-center justify-center text-primary/40 font-mono text-xs">
    {{ $t('common.loading') }}
  </div>
</template>
