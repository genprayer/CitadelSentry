// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2026-09-18',
  ssr: false,
  devtools: { enabled: false },
  telemetry: false,

  runtimeConfig: {
    public: {
      citadelApiBaseUrl: 'https://jointhectl.com'
    }
  },

  future: {
    compatibilityVersion: 4
  },
  modules: [
    '@nuxtjs/tailwindcss',
    '@pinia/nuxt'
  ],

  css: ['~/assets/css/main.css'],

  app: {
    head: {
      title: 'Citadel Sentry',
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        { name: 'description', content: 'Official Citadel Sentry Desktop Application' }
      ],
      link: [
        { rel: 'icon', type: 'image/png', href: '/img/logo_citadel.png' },
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
        { rel: 'stylesheet', href: 'https://fonts.googleapis.com/css2?family=Orbitron:wght@400;500;600;700;800;900&family=Tektur:wght@400;500;600;700;800;900&display=swap' }
      ]
    }
  },

  typescript: {
    strict: true
  }
})
