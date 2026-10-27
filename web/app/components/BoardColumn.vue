<script setup lang="ts">
import { animations } from '@formkit/drag-and-drop'
import { useDragAndDrop } from '@formkit/drag-and-drop/vue'
import type { Card, Column } from '~/types'

const props = defineProps<{ column: Column }>()
const emit = defineEmits<{
  order: [columnId: bigint, cards: Card[]]
  drop: []
  addCard: [columnId: bigint, title: string]
  newNote: [columnId: bigint]
  pickNote: [columnId: bigint]
  rename: [columnId: bigint, title: string]
  removeColumn: [columnId: bigint]
  open: [card: Card]
}>()

// longPress: на телефоне обычное перетаскивание отбирает скролл у колонки,
// поэтому карточка берётся удержанием.
const [dropzone, cards] = useDragAndDrop<Card>([...props.column.cards], {
  group: 'kanban',
  dropZoneClass: 'dragging',
  longPress: true,
  longPressDuration: 300,
  longPressClass: 'long-press',
  plugins: [animations()],
  onDragend: async () => {
    // Дождаться order от обеих колонок; перечитывание во время drag создаёт дубли.
    await nextTick()
    emit('drop')
  },
})

watch(() => props.column.cards, (v) => { cards.value = [...v] })
watch(cards, (v) => emit('order', props.column.id, v))

const draft = ref('')
function addCard() {
  const t = draft.value.trim()
  if (!t) return
  emit('addCard', props.column.id, t)
  draft.value = ''
}

const renaming = ref(false)
const name = ref(props.column.title)
watch(() => props.column.title, (v) => { name.value = v })
function commitName() {
  renaming.value = false
  if (name.value.trim() && name.value !== props.column.title) emit('rename', props.column.id, name.value.trim())
  else name.value = props.column.title
}

// Двойного клика на тач-экране нет, поэтому переименование и удаление — в меню.
const menu = [[
  { label: 'Переименовать', icon: 'i-lucide-pencil', onSelect: () => { renaming.value = true } },
  { label: 'Удалить колонку', icon: 'i-lucide-trash-2', color: 'error' as const, onSelect: () => emit('removeColumn', props.column.id) },
]]
</script>

<template>
  <section class="flex h-full min-h-0 w-[82vw] max-w-72 shrink-0 snap-start flex-col gap-2 rounded-lg border border-default bg-elevated/40 p-2.5 sm:w-72">
    <header class="flex min-w-0 shrink-0 items-center gap-1.5">
      <UInput
        v-if="renaming"
        v-model="name"
        size="xs"
        autofocus
        class="grow"
        @keyup.enter="commitName"
        @blur="commitName"
      />
      <h2
        v-else
        class="min-w-0 grow truncate text-sm font-semibold text-highlighted"
        title="Двойной клик — переименовать"
        @dblclick="renaming = true"
      >
        {{ column.title }}
      </h2>
      <UBadge :label="String(cards.length)" color="neutral" variant="subtle" size="sm" />
      <UDropdownMenu :items="menu">
        <UButton icon="i-lucide-ellipsis" size="xs" color="neutral" variant="ghost" aria-label="Меню колонки" />
      </UDropdownMenu>
    </header>

    <!-- Скролл у каждой колонки свой: длинная колонка не тянет вниз соседей. -->
    <div ref="dropzone" class="dropzone -mx-1 flex min-h-6 flex-1 flex-col gap-2 overflow-y-auto px-1">
      <article
        v-for="card in cards"
        :key="String(card.id)"
        class="kcard group flex min-w-0 cursor-grab select-none flex-col gap-3 rounded-lg border border-default bg-default p-3.5 shadow-xs transition-[border-color,box-shadow] hover:border-primary/50 hover:shadow-md active:cursor-grabbing"
        @click="emit('open', card)"
      >
        <div class="flex min-w-0 items-start gap-2.5">
          <span class="flex size-7 shrink-0 items-center justify-center rounded-md bg-elevated text-muted">
            <UIcon :name="card.noteId ? 'i-lucide-notebook-pen' : 'i-lucide-square-check'" class="size-4" />
          </span>
          <span class="min-w-0 grow break-words pt-0.5 text-sm font-medium leading-5 text-highlighted">
            {{ card.noteTitle || card.title || 'Без названия' }}
          </span>
        </div>
        <p v-if="card.body" class="line-clamp-3 break-words text-sm leading-5 text-muted">
          {{ card.body }}
        </p>
        <div class="flex flex-wrap items-center justify-between gap-2 border-t border-default/60 pt-2">
          <span class="text-xs text-dimmed">{{ card.noteId ? 'Задача' : 'Карточка' }}</span>
          <UBadge
            v-if="card.noteStatus && statusMeta(card.noteStatus)"
            v-bind="statusMeta(card.noteStatus)!"
            variant="subtle"
            size="sm"
          />
        </div>
      </article>
    </div>

    <div class="flex min-w-0 shrink-0 items-center gap-1.5">
      <UInput
        v-model="draft"
        size="sm"
        variant="soft"
        placeholder="+ карточка"
        class="grow"
        @keyup.enter="addCard"
      />
      <UButton
        icon="i-lucide-square-pen"
        size="sm"
        color="neutral"
        variant="soft"
        title="Новая задача в эту колонку"
        aria-label="Новая задача"
        @click="emit('newNote', column.id)"
      />
      <UButton
        icon="i-lucide-link"
        size="sm"
        color="neutral"
        variant="soft"
        title="Добавить существующую задачу"
        aria-label="Добавить существующую задачу"
        @click="emit('pickNote', column.id)"
      />
    </div>
  </section>
</template>
