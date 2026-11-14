import en from '~/locales/en.json'
import ru from '~/locales/ru.json'

export type LocaleCode = 'ru' | 'en'

export interface LocaleOption {
  code: LocaleCode
  name: string
  flag: string
}

export const SUPPORTED_LOCALES: LocaleOption[] = [
  { code: 'ru', name: 'Русский', flag: '🇷🇺' },
  { code: 'en', name: 'English', flag: '🇬🇧' }
]

const translations: Record<LocaleCode, any> = {
  ru,
  en
}

const STORAGE_LOCALE_KEY = 'citadel_seeder_locale'

export function useI18n() {
  const locale = useState<LocaleCode>('citadel_locale', () => {
    if (typeof window !== 'undefined') {
      try {
        const saved = localStorage.getItem(STORAGE_LOCALE_KEY) as LocaleCode
        if (saved && (saved === 'ru' || saved === 'en')) {
          return saved
        }
      } catch {}
    }
    return 'ru'
  })

  const setLocale = (newLocale: LocaleCode) => {
    locale.value = newLocale
    if (typeof window !== 'undefined') {
      try {
        localStorage.setItem(STORAGE_LOCALE_KEY, newLocale)
      } catch {}
    }
  }

  const t = (path: string, params?: Record<string, string | number>): string => {
    const activeLocale = locale.value || 'ru'
    const dict = translations[activeLocale] || translations.ru

    const segments = path.split('.')
    let current: any = dict

    for (const segment of segments) {
      if (current && typeof current === 'object' && segment in current) {
        current = current[segment]
      } else {
        // Fallback to English if key is missing in Russian
        let fallback: any = translations.en
        for (const fbSegment of segments) {
          if (fallback && typeof fallback === 'object' && fbSegment in fallback) {
            fallback = fallback[fbSegment]
          } else {
            return path
          }
        }
        current = fallback
        break
      }
    }

    if (typeof current !== 'string') {
      return path
    }

    if (!params) {
      return current
    }

    return current.replace(/\{(\w+)\}/g, (_, key) => {
      return params[key] !== undefined ? String(params[key]) : `{${key}}`
    })
  }

  return {
    locale: readonly(locale),
    locales: SUPPORTED_LOCALES,
    setLocale,
    t
  }
}
