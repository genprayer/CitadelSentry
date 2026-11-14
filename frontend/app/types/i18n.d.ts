import type { useI18n } from '~/composables/useI18n'

declare module 'vue' {
  interface ComponentCustomProperties {
    $t: (path: string, params?: Record<string, string | number>) => string
    $locale: string
    $setLocale: (code: string) => void
  }
}

declare module '@vue/runtime-core' {
  interface ComponentCustomProperties {
    $t: (path: string, params?: Record<string, string | number>) => string
    $locale: string
    $setLocale: (code: string) => void
  }
}

declare module '#app' {
  interface NuxtApp {
    $t: (path: string, params?: Record<string, string | number>) => string
    $locale: string
    $setLocale: (code: string) => void
  }
}

export {}
