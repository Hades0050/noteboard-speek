import type { Note } from '~/types'

// Одно состояние задач на всё приложение: боковое меню и модалка
// смотрят в один список.
export function useNotes() {
  const notes = useState<Note[]>('notes', () => [])
  const query = useState('notes-query', () => '')
  const error = useState('notes-error', () => '')

  async function load() {
    try {
      notes.value = (await notesRpc.listNotes({ query: query.value })).notes
      error.value = ''
    } catch (e) {
      error.value = (e as Error).message
    }
  }

  async function create(title: string, body: string) {
    const { note } = await notesRpc.createNote({ title, body })
    await load()
    return note!
  }

  // Поля-optional в proto = presence: не переданное поле сервер не трогает.
  async function update(id: bigint, patch: Partial<Pick<Note, 'title' | 'body' | 'transcript' | 'pinned'>>) {
    const { note } = await notesRpc.updateNote({ id, ...patch })
    await load()
    return note!
  }

  async function remove(id: bigint) {
    await notesRpc.deleteNote({ id })
    await load()
  }

  async function retry(id: bigint) {
    await notesRpc.retranscribe({ id })
    await load()
  }

  // Загрузка файлов остаётся на REST: браузерный Connect не умеет стримить
  // тело запроса, а гнать мегабайты аудио в base64 внутри JSON — плохая идея.
  async function uploadAudio(id: bigint, blob: Blob) {
    if (isTauri()) {
      await attachAudioTauri(id, blob)
    } else {
      const form = new FormData()
      form.append('audio', blob, 'voice.webm')
      await api(`/notes/${id}/audio`, { method: 'POST', body: form })
    }
    await load()
  }

  const transcribing = computed(() =>
    notes.value.some(n => n.transcribeStatus === 'pending' || n.transcribeStatus === 'running'))

  return { notes, query, error, load, create, update, remove, retry, uploadAudio, transcribing }
}

export function statusMeta(status: string) {
  switch (status) {
    case 'pending': return { label: 'в очереди', color: 'neutral' as const, icon: 'i-lucide-clock' }
    case 'running': return { label: 'расшифровка', color: 'info' as const, icon: 'i-lucide-loader' }
    case 'done': return { label: 'расшифровано', color: 'success' as const, icon: 'i-lucide-check' }
    case 'error': return { label: 'ошибка', color: 'error' as const, icon: 'i-lucide-triangle-alert' }
    default: return null
  }
}
