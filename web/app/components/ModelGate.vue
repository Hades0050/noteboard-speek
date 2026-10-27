<script setup lang="ts">
const { ready, busy, error, progress, check, download } = useSttModel()
const dismissed = ref(false)

onMounted(check)

const open = computed({
  get: () => !ready.value && !dismissed.value,
  set: (v: boolean) => { dismissed.value = !v },
})

const percent = computed(() => {
  const p = progress.value
  return p && p.total > 0 ? Math.round((p.done / p.total) * 100) : 0
})
</script>

<template>
  <UModal v-model:open="open" title="Распознавание речи" :dismissible="!busy">
    <template #body>
      <div class="space-y-4">
        <p class="text-sm text-muted">
          Голосовые заметки расшифровываются на устройстве. Один раз нужно скачать
          модель GigaAM — около 330 МБ. До этого запись сохранится, но останется без текста.
        </p>

        <div v-if="busy" class="space-y-1">
          <UProgress :model-value="percent" />
          <p class="text-xs text-dimmed">
            {{ progress?.file }} — {{ percent }}%
          </p>
        </div>

        <UAlert v-if="error" color="error" variant="subtle" icon="i-lucide-triangle-alert" :description="error" />
      </div>
    </template>

    <template #footer>
      <div class="flex justify-end gap-2">
        <UButton color="neutral" variant="ghost" :disabled="busy" @click="dismissed = true">
          Позже
        </UButton>
        <UButton icon="i-lucide-download" :loading="busy" @click="download">
          Скачать
        </UButton>
      </div>
    </template>
  </UModal>
</template>
