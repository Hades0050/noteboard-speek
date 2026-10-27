<script setup lang="ts">
import type { Card } from '~/types'

// Карточка без задачи: только заголовок и текст. Всё, что сложнее, живёт
// в задаче — карточку под это можно превратить кнопкой «Сделать задачей».
const props = defineProps<{ card: Card | null }>()
const emit = defineEmits<{ changed: [] }>()
const mobile = useIsMobile()

const { closeModal, openNote } = useModals()
const { create } = useNotes()
const toast = useToast()

function notify(title: string, icon = 'i-lucide-check') {
  toast.add({ title, icon, color: 'success' })
}

const title = ref('')
const body = ref('')
const busy = ref(false)

watch(() => props.card, (c) => {
  title.value = c?.title ?? ''
  body.value = c?.body ?? ''
}, { immediate: true })

const open = computed({
  get: () => !!props.card,
  set: (v: boolean) => { if (!v) closeModal() },
})

async function save() {
  if (!props.card || !title.value.trim()) return
  busy.value = true
  try {
    await boardRpc.updateCard({ id: props.card.id, title: title.value.trim(), body: body.value })
    emit('changed')
    notify('Карточка сохранена')
    closeModal()
  } finally {
    busy.value = false
  }
}

async function destroy() {
  if (!props.card) return
  await boardRpc.deleteCard({ id: props.card.id })
  emit('changed')
  notify('Карточка удалена', 'i-lucide-trash-2')
  closeModal()
}

// Карточка остаётся на своём месте, но получает задачу с текстом и аудио.
async function promote() {
  if (!props.card) return
  busy.value = true
  try {
    const note = await create(title.value.trim() || props.card.title, body.value)
    await boardRpc.updateCard({ id: props.card.id, noteId: note.id })
    emit('changed')
    notify('Карточка стала задачей', 'i-lucide-notebook-pen')
    await openNote(note.id)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <UModal
    v-model:open="open"
    :title="title || 'Карточка'"
    :fullscreen="mobile"
    :ui="{ content: 'sm:max-w-6xl sm:h-[85dvh]', body: 'flex flex-col', footer: 'flex-wrap' }"
  >
    <template #body>
      <div class="flex flex-1 flex-col gap-4">
        <UInput v-model="title" placeholder="Заголовок" size="xl" autofocus class="w-full" />
        <UTextarea v-model="body" placeholder="Описание" :rows="14" class="w-full flex-1" :ui="{ base: 'h-full resize-none' }" />
      </div>
    </template>

    <template #footer>
      <UButton label="Сохранить" icon="i-lucide-check" :loading="busy" :disabled="!title.trim()" @click="save" />
      <UButton
        label="Сделать задачей"
        icon="i-lucide-notebook-pen"
        color="neutral"
        variant="subtle"
        :loading="busy"
        @click="promote"
      />
      <div class="grow" />
      <UButton icon="i-lucide-trash-2" color="error" variant="ghost" aria-label="Удалить" @click="destroy" />
    </template>
  </UModal>
</template>
