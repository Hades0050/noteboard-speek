export default defineNuxtConfig({
  compatibilityDate: '2026-09-20',
  ssr: false,
  modules: ['@nuxt/ui'],
  // Приложение работает офлайн: иконки должны лежать в бандле, а не
  // подгружаться из Iconify API.
  icon: {
    provider: 'none',
    clientBundle: { scan: true, sizeLimitKb: 2048 },
  },
  devtools: { enabled: false },
  css: ['~/assets/css/main.css'],
  nitro: {
    devProxy: {
      '/api': { target: 'http://localhost:8080/api', changeOrigin: true },
    },
  },
  app: {
    head: {
      title: 'noteboard',
      meta: [
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        { name: 'color-scheme', content: 'light dark' },
      ],
    },
  },
})
