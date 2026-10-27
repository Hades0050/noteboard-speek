export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.hook('app:error', (e: unknown) => {
    console.error('APP_ERROR', (e as Error)?.stack || String(e))
  })
  nuxtApp.hook('vue:error', (e: unknown) => {
    console.error('VUE_ERROR', (e as Error)?.stack || String(e))
  })
  window.addEventListener('unhandledrejection', (ev) => {
    console.error('REJECTION', (ev.reason as Error)?.stack || String(ev.reason))
  })
})
