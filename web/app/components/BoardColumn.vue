<script setup lang="ts">
import { useDragAndDrop } from '@formkit/drag-and-drop/vue'
import type { Card, Column } from '~/types'

const props = defineProps<{ column: Column }>()
const emit = defineEmits<{
  order: [columnId: bigint, cards: Card[]]
  addCard: [columnId: bigint, title: string]
  newNote: [columnId: bigint]
  pickNote: [columnId: bigint]
  rename: [columnId: bigint, title: string]
  removeColumn: [columnId: bigint]
  open: [card: Card]
}>()

const [dropzone, cards] = useDragAndDrop<Card>([...props.column.cards], {
  group: 'kanban',
  dragHandle: '.handle',
  dropZoneClass: 'dragging',
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
</script>

<template>
  <section class="flex h-full min-h-0 w-72 shrink-0 flex-col gap-2 rounded-lg border border-default bg-elevated/40 p-2.5">
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
      <UButton
        icon="i-lucide-x"
        size="xs"
        color="neutral"
        variant="ghost"
        aria-label="Удалить колонку"
        @click="emit('removeColumn', column.id)"
      />
    </header>

    <!-- Скролл у каждой колонки свой: длинная колонка не тянет вниз соседей. -->
    <div ref="dropzone" class="dropzone -mx-1 flex min-h-6 flex-1 flex-col gap-2 overflow-y-auto px-1">
      <article
        v-for="card in cards"
        :key="String(card.id)"
        class="kcard group flex min-w-0 cursor-pointer flex-col gap-1 rounded-md border border-default bg-default p-2 shadow-xs hover:border-primary/60"
        @click="emit('open', card)"
      >
        <div class="flex min-w-0 items-center gap-1.5">
          <UIcon
            name="i-lucide-grip-vertical"
            class="handle size-4 shrink-0 cursor-grab text-dimmed active:cursor-grabbing"
            @click.stop
          />
          <UIcon
            v-if="card.noteId"
            name="i-lucide-notebook-pen"
            class="size-3.5 shrink-0 text-dimmed"
          />
          <span class="min-w-0 grow break-words text-sm">
            {{ card.noteTitle || card.title || 'Без названия' }}
          </span>
        </div>
        <p v-if="card.body" class="line-clamp-3 break-words text-xs text-muted">
          {{ card.body }}
        </p>
        <UBadge
          v-if="card.noteStatus && statusMeta(card.noteStatus)"
          v-bind="statusMeta(card.noteStatus)!"
          variant="subtle"
          size="sm"
          class="self-start"
        />
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
