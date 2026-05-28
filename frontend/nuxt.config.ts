export default defineNuxtConfig({
  ssr: false,

  modules: [
    '@nuxt/ui',
    '@pinia/nuxt',
    '@nuxtjs/i18n',
  ],

  i18n: {
    restructureDir: false,
    locales: [
      { code: 'en', file: 'en.json' },
      { code: 'de', file: 'de.json' },
    ],
    defaultLocale: 'en',
    langDir: 'i18n/locales/',
    strategy: 'no_prefix',
  },

  devtools: { enabled: false },

  typescript: {
    strict: true,
  },

  compatibilityDate: '2024-11-01',
})
