<script setup lang="ts">
const { notes, query, load, transcribing } = useNotes()
const { noteId, openNote } = useModals()

let searchTimer: ReturnType<typeof setTimeout> | null = null
watch(query, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(load, 250)
})

// Пока воркер разбирает очередь расшифровки, подтягиваем список.
let poll: ReturnType<typeof setInterval> | null = null
onMounted(() => {
  load()
  poll = setInterval(() => { if (transcribing.value) load() }, 3000)
})
onBeforeUnmount(() => { if (poll) clearInterval(poll) })

function preview(text: string) {
  return stripHtml(text).replace(/\s+/g, ' ').trim().slice(0, 60)
}
</script>

<template>
  <UApp>
    <UDashboardGroup>
      <UDashboardSidebar
        id="main"
        collapsible
        resizable
        :min-size="14"
        :default-size="18"
        :max-size="26"
        :ui="{ footer: 'border-t border-default' }"
      >
        <template #header="{ collapsed }">
          <NuxtLink to="/" class="flex items-center gap-2 font-semibold text-highlighted">
            <UIcon name="i-lucide-kanban" class="size-5 text-primary shrink-0" />
            <span v-if="!collapsed">noteboard</span>
          </NuxtLink>
        </template>

        <template #default="{ collapsed }">
          <UInput
            v-if="!collapsed"
            v-model="query"
            icon="i-lucide-search"
            placeholder="Поиск по задачам"
            size="sm"
          />

          <div v-if="!collapsed" class="mt-2 flex-1 overflow-y-auto -mx-1 px-1">
            <p class="px-2 py-1 text-xs font-medium text-dimmed">
              Задачи
            </p>

            <button
              v-for="n in notes"
              :key="String(n.id)"
              type="button"
              class="group flex w-full items-start gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-elevated/60"
              :class="noteId === n.id ? 'bg-elevated text-highlighted' : 'text-muted'"
              @click="openNote(n.id)"
            >
              <UIcon
                :name="n.audioPath ? 'i-lucide-mic' : 'i-lucide-file-text'"
                class="size-4 mt-0.5 shrink-0"
                :class="n.transcribeStatus === 'running' ? 'text-info animate-pulse' : 'text-dimmed'"
              />
              <span class="min-w-0 flex-1">
                <span class="block truncate">{{ n.title || 'Без заголовка' }}</span>
                <span class="block truncate text-xs text-dimmed">{{ preview(n.body || n.transcript) }}</span>
              </span>
              <UIcon v-if="n.pinned" name="i-lucide-pin" class="size-3.5 shrink-0 text-primary" />
            </button>

            <p v-if="!notes.length" class="px-2 py-3 text-sm text-dimmed">
              Пока пусто
            </p>
          </div>
        </template>

        <template #footer="{ collapsed }">
          <UColorModeButton v-if="!collapsed" size="sm" />
          <span v-if="!collapsed" class="text-xs text-dimmed">{{ notes.length }} задач</span>
        </template>
      </UDashboardSidebar>

      <NuxtPage />
    </UDashboardGroup>

    <NoteModal />
    <ModelGate />
    <ConfirmDialog />
  </UApp>
</template>
