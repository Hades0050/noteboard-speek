// Узкий экран: модалки разворачиваются на весь экран, колонки листаются по одной.
export function useIsMobile() {
  const mobile = ref(false)
  onMounted(() => {
    const mq = window.matchMedia('(max-width: 640px)')
    const sync = () => { mobile.value = mq.matches }
    sync()
    mq.addEventListener('change', sync)
    onBeforeUnmount(() => mq.removeEventListener('change', sync))
  })
  return mobile
}
