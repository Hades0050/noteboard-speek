<script setup lang="ts">
import type { Card, Column, Note } from '~/types'

const { notes, load: loadNotes, create } = useNotes()
const { noteId, cardId, openNote, openCard } = useModals()
const toast = useToast()
const mobile = useIsMobile()

const columns = ref<Column[]>([])
const loadError = ref('')
const newColumn = ref('')

// Порядок, который колонки сообщают во время перетаскивания.
// Ключ — id колонки строкой: id в proto это int64, то есть bigint,
// а ключами объекта bigint быть не может.
const local = reactive<Record<string, bigint[]>>({})
let serverOrder = ''

function signature(order: Record<string, bigint[]>): string {
  return Object.keys(order)
    .sort()
    .map(id => `${id}:${(order[id] ?? []).join(',')}`)
    .join('|')
}

async function load() {
  try {
    columns.value = (await boardRpc.getBoard({})).columns
    for (const key of Object.keys(local)) delete local[key]
    for (const c of columns.value) local[String(c.id)] = c.cards.map(card => card.id)
    serverOrder = signature(local)
    loadError.value = ''
  } catch (e) {
    loadError.value = (e as Error).message
  }
}

// Модалка меняет и задачу, и карточку — после её закрытия доска перечитывается.
watch([noteId, cardId], ([n, c]) => {
  if (n === null && c === null) load()
})

function onOrder(columnId: bigint, cards: Card[]) {
  local[String(columnId)] = cards.map(c => c.id)
}

async function commit() {
  if (signature(local) === serverOrder) return
  const payload = Object.keys(local).map(key => ({ columnId: BigInt(key), cardIds: local[key] ?? [] }))
  await boardRpc.reorder({ columns: payload })
  await load()
}

const openedCard = computed<Card | null>(() => {
  if (cardId.value === null) return null
  for (const c of columns.value) {
    const found = c.cards.find(card => card.id === cardId.value)
    if (found) return found
  }
  return null
})

function openFromBoard(card: Card) {
  if (card.noteId) openNote(card.noteId)
  else openCard(card.id)
}

// Новая задача сразу в колонке: заголовком служит время, дальше её
// заполняют в модалке. Кнопка в шапке кладёт задачу в первую колонку —
// доска без колонок бессмысленна, поэтому иначе кнопки просто нет.
async function newNoteIn(columnId: bigint) {
  const note = await create(autoTitle(), '')
  await boardRpc.attachNote({ noteId: note.id, columnId })
  await openNote(note.id)
}

// Выбор задачи для колонки: модалка с поиском по уже загруженному списку.
const pickerColumn = ref<bigint | null>(null)
const pickerQuery = ref('')
const pickerOpen = computed({
  get: () => pickerColumn.value !== null,
  set: (v: boolean) => { if (!v) pickerColumn.value = null },
})
const pickerNotes = computed<Note[]>(() => {
  const q = pickerQuery.value.trim().toLowerCase()
  if (!q) return notes.value
  return notes.value.filter(n =>
    `${n.title} ${n.body} ${n.transcript}`.toLowerCase().includes(q))
})

async function openPicker(columnId: bigint) {
  pickerQuery.value = ''
  pickerColumn.value = columnId
  if (!notes.value.length) await loadNotes()
}

async function attachNote(id: bigint) {
  const columnId = pickerColumn.value
  pickerColumn.value = null
  if (!columnId) return
  await boardRpc.attachNote({ noteId: id, columnId })
  await load()
  toast.add({ title: 'Задача на доске', icon: 'i-lucide-kanban', color: 'success' })
}

async function addCard(columnId: bigint, title: string) {
  await boardRpc.createCard({ columnId, title })
  await load()
}

async function renameColumn(id: bigint, title: string) {
  await boardRpc.renameColumn({ id, title })
  await load()
}

async function removeColumn(id: bigint) {
  if (!await askConfirm({ title: 'Удалить колонку?', description: 'Карточки в ней тоже исчезнут.' })) return
  await boardRpc.deleteColumn({ id })
  await load()
  toast.add({ title: 'Колонка удалена', icon: 'i-lucide-trash-2', color: 'success' })
}

async function addColumn() {
  const t = newColumn.value.trim()
  if (!t) return
  await boardRpc.createColumn({ title: t })
  newColumn.value = ''
  await load()
}

onMounted(load)
</script>

<template>
  <UDashboardPanel id="board" :ui="{ body: 'p-0 sm:p-0 gap-0 overflow-hidden' }">
    <template #header>
      <UDashboardNavbar title="Доска" icon="i-lucide-kanban">
        <template #right>
          <UButton
            v-if="columns[0]"
            label="Создать задачу"
            icon="i-lucide-plus"
            @click="newNoteIn(columns[0]!.id)"
          />
        </template>
      </UDashboardNavbar>
    </template>

    <template #body>
      <UAlert v-if="loadError" color="error" variant="subtle" :description="loadError" class="m-4" />

      <!-- Колонки во всю высоту панели, лишние уезжают вбок. -->
      <div class="flex h-full min-h-0 snap-x snap-mandatory items-stretch gap-3 overflow-x-auto p-4 sm:snap-none sm:p-6">
        <BoardColumn
          v-for="c in columns"
          :key="String(c.id)"
          :column="c"
          @order="onOrder"
          @drop="commit"
          @add-card="addCard"
          @new-note="newNoteIn"
          @pick-note="openPicker"
          @open="openFromBoard"
          @rename="renameColumn"
          @remove-column="removeColumn"
        />

        <div class="h-fit w-[82vw] max-w-56 shrink-0 snap-start rounded-lg border border-dashed border-default p-2.5 sm:w-56">
          <UInput
            v-model="newColumn"
            size="sm"
            variant="soft"
            placeholder="+ колонка"
            class="w-full"
            @keyup.enter="addColumn"
          />
        </div>
      </div>

      <CardModal :card="openedCard" @changed="load" />

      <UModal v-model:open="pickerOpen" title="Задачу на доску" :fullscreen="mobile">
        <template #body>
          <div class="flex flex-col gap-3">
            <UInput
              v-model="pickerQuery"
              icon="i-lucide-search"
              placeholder="Поиск по задачам"
              autofocus
            />
            <p v-if="!pickerNotes.length" class="text-sm text-muted">
              Задач нет
            </p>
            <div v-else class="flex max-h-80 flex-col gap-1 overflow-y-auto">
              <button
                v-for="n in pickerNotes"
                :key="String(n.id)"
                type="button"
                class="flex min-w-0 flex-col gap-0.5 rounded-md px-2 py-1.5 text-left hover:bg-elevated"
                @click="attachNote(n.id)"
              >
                <span class="truncate text-sm text-highlighted">{{ n.title || 'Без названия' }}</span>
                <span v-if="n.body || n.transcript" class="truncate text-xs text-muted">
                  {{ n.body || n.transcript }}
                </span>
              </button>
            </div>
          </div>
        </template>
      </UModal>
    </template>
  </UDashboardPanel>
</template>
