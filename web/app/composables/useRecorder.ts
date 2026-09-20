const MIME_CANDIDATES = ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4', 'audio/ogg;codecs=opus']

// Браузерная обработка настроена под голосовые звонки, а не под ASR:
// автоусиление качает громкость и смазывает окончания слов, шумодав выедает
// шипящие. Whisper обучен на сыром шумном звуке и без этих фильтров узнаёт
// слова лучше. Моно 48 кГц: дальше всё равно ffmpeg сведёт к 16 кГц.
const AUDIO_CONSTRAINTS: MediaTrackConstraints = {
  channelCount: 1,
  sampleRate: 48000,
  echoCancellation: false,
  noiseSuppression: false,
  autoGainControl: false,
}

export function useRecorder() {
  const recording = ref(false)
  const seconds = ref(0)
  const error = ref('')

  let recorder: MediaRecorder | null = null
  let stream: MediaStream | null = null
  let chunks: Blob[] = []
  let timer: ReturnType<typeof setInterval> | null = null

  function pickMime(): string {
    for (const m of MIME_CANDIDATES) {
      if (MediaRecorder.isTypeSupported(m)) return m
    }
    return ''
  }

  const supported = computed(() =>
    import.meta.client && !!navigator.mediaDevices?.getUserMedia && typeof MediaRecorder !== 'undefined',
  )

  async function start() {
    error.value = ''
    if (!supported.value) {
      error.value = 'Браузер не умеет запись. Нужен HTTPS или localhost.'
      return false
    }
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: AUDIO_CONSTRAINTS })
    } catch (e) {
      error.value = 'Нет доступа к микрофону: ' + (e as Error).message
      return false
    }
    chunks = []
    const mime = pickMime()
    recorder = new MediaRecorder(stream, {
      ...(mime ? { mimeType: mime } : {}),
      audioBitsPerSecond: 128000,
    })
    recorder.ondataavailable = (e) => { if (e.data.size) chunks.push(e.data) }
    recorder.start(1000)
    recording.value = true
    seconds.value = 0
    timer = setInterval(() => { seconds.value++ }, 1000)
    return true
  }

  function cleanup() {
    if (timer) { clearInterval(timer); timer = null }
    stream?.getTracks().forEach(t => t.stop())
    stream = null
    recording.value = false
  }

  function stop(): Promise<Blob | null> {
    return new Promise((resolve) => {
      if (!recorder || recorder.state === 'inactive') {
        cleanup()
        resolve(null)
        return
      }
      const type = recorder.mimeType || 'audio/webm'
      recorder.onstop = () => {
        const blob = chunks.length ? new Blob(chunks, { type }) : null
        recorder = null
        cleanup()
        resolve(blob)
      }
      recorder.stop()
    })
  }

  function cancel() {
    if (recorder && recorder.state !== 'inactive') {
      recorder.onstop = null
      recorder.stop()
    }
    recorder = null
    chunks = []
    cleanup()
  }

  onBeforeUnmount(cancel)

  return { recording, seconds, error, supported, start, stop, cancel }
}

export function mmss(total: number): string {
  const m = Math.floor(total / 60).toString().padStart(2, '0')
  const s = (total % 60).toString().padStart(2, '0')
  return `${m}:${s}`
}
