<script setup lang="ts">
import type { Column, Note } from '~/types'

// Единственный экран задачи. Новая задача создаётся сразу на доске пустой,
// поэтому здесь всегда есть id и отдельного режима «черновик» не нужно.
const { noteId, closeModal } = useModals()
const { update, remove, retry, uploadAudio, load } = useNotes()
const { recording, seconds, error: recError, supported, start, stop, cancel } = useRecorder()
const toast = useToast()
const mobile = useIsMobile()

function notify(title: string, icon = 'i-lucide-check', color: 'success' | 'error' | 'info' = 'success') {
  toast.add({ title, icon, color })
}

const note = ref<Note | null>(null)
const title = ref('')
const body = ref('')
const dirty = ref(false)
const busy = ref(false)
const loadError = ref('')

// Приём с сервера не считается правкой пользователя, иначе «Сохранить» горит
// всегда, а автообновление после расшифровки перестаёт работать.
let syncing = false

const canSave = computed(() => !!title.value.trim() || !!stripHtml(body.value).trim())
const status = computed(() => note.value ? statusMeta(note.value.transcribeStatus) : null)

function reset() {
  note.value = null
  title.value = ''
  body.value = ''
  dirty.value = false
  loadError.value = ''
}

async function fill(fresh: Note) {
  note.value = fresh
  if (dirty.value) return
  syncing = true
  title.value = fresh.title
  body.value = fresh.body
  await nextTick()
  syncing = false
}

async function fetchNote() {
  const id = noteId.value
  if (id === null) return
  try {
    const { note: fresh } = await notesRpc.getNote({ id })
    if (fresh) await fill(fresh)
    loadError.value = ''
  } catch (e) {
    loadError.value = (e as Error).message
  }
}

watch(noteId, (id) => {
  if (id === null) {
    reset()
    return
  }
  if (note.value?.id !== id) reset()
  fetchNote()
}, { immediate: true })

watch([title, body], () => { if (!syncing) dirty.value = true })

// Пока идёт расшифровка — тянем задачу сами, не дожидаясь списка в сайдбаре.
let poll: ReturnType<typeof setInterval> | null = null
watch(() => note.value?.transcribeStatus, (s) => {
  const running = s === 'pending' || s === 'running'
  if (running && !poll) poll = setInterval(fetchNote, 2000)
  if (!running && poll) {
    clearInterval(poll)
    poll = null
    load()
    if (s === 'done') notify('Расшифровка готова', 'i-lucide-audio-lines')
    if (s === 'error') notify('Расшифровка не удалась', 'i-lucide-triangle-alert', 'error')
  }
})
onBeforeUnmount(() => {
  if (poll) clearInterval(poll)
  cancel()
})

function persist(): Promise<Note> {
  return update(noteId.value!, { title: title.value, body: body.value })
}

async function save() {
  if (!canSave.value) return
  busy.value = true
  try {
    await persist()
    dirty.value = false
    notify('Задача сохранена')
    closeModal()
  } catch (e) {
    notify((e as Error).message, 'i-lucide-triangle-alert', 'error')
  } finally {
    busy.value = false
  }
}

// Закрытие не должно съедать набранный текст: несохранённое дописываем молча.
function requestClose() {
  if (dirty.value && canSave.value) persist()
  closeModal()
}

const open = computed({
  get: () => noteId.value !== null,
  set: (v: boolean) => { if (!v) requestClose() },
})

async function togglePin() {
  if (!note.value) return
  const saved = await update(note.value.id, { pinned: !note.value.pinned })
  await fill(saved)
  notify(saved.pinned ? 'Закреплено' : 'Откреплено', 'i-lucide-pin')
}

async function destroy() {
  if (!note.value) return
  await remove(note.value.id)
  notify('Задача удалена', 'i-lucide-trash-2')
  closeModal()
}

async function toggleRecord() {
  if (!recording.value) {
    await start()
    return
  }
  const blob = await stop()
  if (!blob) return
  const id = noteId.value
  if (id === null) return
  busy.value = true
  try {
    await uploadAudio(id, blob)
    await fetchNote()
    notify('Запись отправлена на расшифровку', 'i-lucide-mic')
  } finally {
    busy.value = false
  }
}

// «На доску»: список колонок подтягивается лениво, при открытии меню.
const boardColumns = ref<Column[]>([])

async function loadColumns() {
  if (boardColumns.value.length) return
  boardColumns.value = (await boardRpc.getBoard({})).columns
}

const boardItems = computed(() => [boardColumns.value.map(c => ({
  label: c.title,
  icon: 'i-lucide-columns-3',
  onSelect: async () => {
    if (!note.value) return
    await boardRpc.attachNote({ noteId: note.value.id, columnId: c.id })
    notify('Задача на доске', 'i-lucide-kanban')
  },
}))])

const editor = useTemplateRef<{ insertHTML: (html: string) => void }>('editor')

function insertTranscript() {
  const text = note.value?.transcript
  if (!text) return
  editor.value?.insertHTML(text
    .split(/\n+/)
    .map(line => `<p>${line.replace(/&/g, '&amp;').replace(/</g, '&lt;')}</p>`)
    .join(''))
}
</script>

<template>
  <UModal
    v-model:open="open"
    :title="title || 'Новая задача'"
    :fullscreen="mobile"
    :ui="{ content: 'max-w-3xl' }"
  >
    <template #body>
      <UAlert v-if="loadError" color="error" variant="subtle" :description="loadError" />

      <div v-else class="space-y-4">
        <UInput
          v-model="title"
          placeholder="Заголовок"
          size="xl"
          variant="none"
          autofocus
          class="w-full"
          :ui="{ base: 'px-0 text-2xl font-semibold' }"
        />

        <NoteEditor
          ref="editor"
          v-model="body"
          placeholder="Описание задачи. Картинку можно вставить из буфера или перетащить. Или нажми «Записать голос»."
        />

        <UAlert v-if="recError" color="error" variant="subtle" icon="i-lucide-mic-off" :description="recError" />
        <p v-else-if="!supported" class="text-sm text-dimmed">
          Запись микрофона доступна только на localhost или по HTTPS.
        </p>

        <!-- Аудио в одну строку: плеер и так съедает высоту, ещё и карточкой
             вокруг него обкладывать нечего. -->
        <div v-if="note?.audioPath" class="space-y-2 rounded-md border border-default p-2">
          <div class="flex min-w-0 items-center gap-2">
            <audio
              controls
              preload="none"
              class="h-9 min-w-0 grow"
              :src="audioSrc(note)"
            />
            <UBadge v-if="status" :color="status.color" variant="subtle" :icon="status.icon" size="sm">
              {{ status.label }}
            </UBadge>
            <UButton
              icon="i-lucide-rotate-ccw"
              size="xs"
              color="neutral"
              variant="ghost"
              :title="note.transcribeStatus === 'error' ? 'Повторить' : 'Расшифровать заново'"
              aria-label="Расшифровать заново"
              @click="retry(note.id).then(fetchNote)"
            />
          </div>

          <p v-if="note.transcribeError" class="text-sm text-error">
            {{ note.transcribeError }}
          </p>

          <div v-if="note.transcript" class="flex items-start gap-2">
            <p class="max-h-40 grow overflow-y-auto rounded-md bg-elevated/60 p-2 text-sm whitespace-pre-wrap">
              {{ note.transcript }}
            </p>
            <UButton
              icon="i-lucide-corner-down-left"
              size="xs"
              color="neutral"
              variant="subtle"
              title="Вставить в текст"
              aria-label="Вставить в текст"
              @click="insertTranscript"
            />
          </div>
        </div>
      </div>
    </template>

    <template #footer>
      <UButton
        label="Сохранить"
        icon="i-lucide-check"
        :loading="busy"
        :disabled="!canSave || recording"
        @click="save"
      />

      <UButton
        :label="recording ? `Стоп ${mmss(seconds)}` : note?.audioPath ? 'Перезаписать голос' : 'Записать голос'"
        :icon="recording ? 'i-lucide-square' : 'i-lucide-mic'"
        :color="recording ? 'error' : 'neutral'"
        variant="subtle"
        :disabled="busy || !supported"
        @click="toggleRecord"
      />

      <UButton v-if="recording" label="Отменить" color="neutral" variant="ghost" @click="cancel" />

      <div class="grow" />

      <template v-if="note">
        <span class="text-xs text-dimmed">изменено {{ formatDate(note.updatedAt) }}</span>

        <UDropdownMenu :items="boardItems" @update:open="loadColumns">
          <UButton
            label="На доску"
            icon="i-lucide-kanban"
            color="neutral"
            variant="subtle"
            trailing-icon="i-lucide-chevron-down"
          />
        </UDropdownMenu>

        <UButton
          :icon="note.pinned ? 'i-lucide-pin-off' : 'i-lucide-pin'"
          color="neutral"
          variant="ghost"
          :aria-label="note.pinned ? 'Открепить' : 'Закрепить'"
          @click="togglePin"
        />

        <UButton
          icon="i-lucide-trash-2"
          color="error"
          variant="ghost"
          aria-label="Удалить"
          @click="destroy"
        />
      </template>
    </template>
  </UModal>
</template>
