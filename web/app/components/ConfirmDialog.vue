<script setup lang="ts">
const pending = useConfirm()

const open = computed({
  get: () => pending.value !== null,
  set: (v: boolean) => { if (!v) answer(false) },
})

function answer(ok: boolean) {
  pending.value?.resolve(ok)
  pending.value = null
}
</script>

<template>
  <UModal v-model:open="open" :title="pending?.title ?? ''" :description="pending?.description">
    <template #footer>
      <div class="flex justify-end gap-2">
        <UButton color="neutral" variant="ghost" @click="answer(false)">
          Отмена
        </UButton>
        <UButton color="error" @click="answer(true)">
          {{ pending?.confirmLabel ?? 'Удалить' }}
        </UButton>
      </div>
    </template>
  </UModal>
</template>
