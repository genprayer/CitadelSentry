import { useI18n } from '~/composables/useI18n'

export default defineNuxtPlugin(() => {
  const { t, locale, setLocale, locales } = useI18n()

  return {
    provide: {
      t,
      locale,
      setLocale,
      locales
    }
  }
})
