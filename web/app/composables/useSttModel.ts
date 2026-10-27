import type { ModelProgress } from './useTauri'

// Модель распознавания живёт только в приложении: в вебе расшифровкой
// занимается сервер, поэтому там гейт всегда «готов».
export function useSttModel() {
  const ready = useState('model-ready', () => true)
  const busy = useState('model-busy', () => false)
  const error = useState('model-error', () => '')
  const progress = useState<ModelProgress | null>('model-progress', () => null)

  async function check() {
    if (!isTauri()) return
    ready.value = await modelReady()
  }

  async function download() {
    busy.value = true
    error.value = ''
    try {
      await modelDownload(p => { progress.value = p })
      ready.value = true
    } catch (e) {
      error.value = (e as { message?: string }).message ?? String(e)
    } finally {
      busy.value = false
      progress.value = null
    }
  }

  return { ready, busy, error, progress, check, download }
}
