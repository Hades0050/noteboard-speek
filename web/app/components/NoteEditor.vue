<script setup lang="ts">
import { Editor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Image from '@tiptap/extension-image'
import Placeholder from '@tiptap/extension-placeholder'

const model = defineModel<string>({ default: '' })
const props = withDefaults(defineProps<{ placeholder?: string }>(), {
  placeholder: 'Описание задачи',
})

const editor = shallowRef<Editor>()
const uploading = ref(false)
const uploadError = ref('')

async function uploadImage(file: File) {
  uploading.value = true
  uploadError.value = ''
  try {
    let url: string
    if (isTauri()) {
      url = await uploadImageTauri(file)
    } else {
      const form = new FormData()
      form.append('file', file, file.name || 'image.png')
      url = (await api<{ url: string }>('/uploads', { method: 'POST', body: form })).url
    }
    editor.value?.chain().focus().setImage({ src: url }).run()
  } catch (e) {
    uploadError.value = (e as Error).message
  } finally {
    uploading.value = false
  }
}

function imagesFrom(list?: FileList | null): File[] {
  return Array.from(list ?? []).filter(f => f.type.startsWith('image/'))
}

onMounted(() => {
  editor.value = new Editor({
    content: model.value,
    extensions: [
      StarterKit.configure({ link: { openOnClick: false } }),
      Image.configure({ HTMLAttributes: { class: 'rounded-md' } }),
      Placeholder.configure({ placeholder: props.placeholder }),
    ],
    editorProps: {
      attributes: {
        class: 'prose prose-sm dark:prose-invert max-w-none min-h-64 focus:outline-none',
      },
      // Картинку можно вставить из буфера или перетащить в текст.
      handlePaste: (_view, event) => {
        const files = imagesFrom(event.clipboardData?.files)
        if (!files.length) return false
        files.forEach(uploadImage)
        return true
      },
      handleDrop: (_view, event) => {
        const files = imagesFrom((event as DragEvent).dataTransfer?.files)
        if (!files.length) return false
        event.preventDefault()
        files.forEach(uploadImage)
        return true
      },
    },
    onUpdate: ({ editor }) => { model.value = editor.getHTML() },
  })
})

// Приход свежего значения снаружи (загрузка заметки) не должен ломать курсор.
watch(model, (v) => {
  const ed = editor.value
  if (ed && v !== ed.getHTML()) ed.commands.setContent(v || '', { emitUpdate: false })
})

onBeforeUnmount(() => editor.value?.destroy())

const fileInput = useTemplateRef<HTMLInputElement>('fileInput')
function pickImage() { fileInput.value?.click() }
function onPick(e: Event) {
  const input = e.target as HTMLInputElement
  imagesFrom(input.files).forEach(uploadImage)
  input.value = ''
}

const linkOpen = ref(false)
const linkHref = ref('')

function setLink() {
  const ed = editor.value
  if (!ed) return
  linkHref.value = (ed.getAttributes('link').href as string | undefined) ?? 'https://'
  linkOpen.value = true
}

function applyLink() {
  const ed = editor.value
  linkOpen.value = false
  if (!ed) return
  const href = linkHref.value.trim()
  const chain = ed.chain().focus().extendMarkRange('link')
  if (href) chain.setLink({ href }).run()
  else chain.unsetLink().run()
}

type Tool = { icon: string, title: string, run: () => void, active?: () => boolean }

const tools = computed<Tool[]>(() => {
  const ed = editor.value
  if (!ed) return []
  return [
    { icon: 'i-lucide-bold', title: 'Жирный', run: () => ed.chain().focus().toggleBold().run(), active: () => ed.isActive('bold') },
    { icon: 'i-lucide-italic', title: 'Курсив', run: () => ed.chain().focus().toggleItalic().run(), active: () => ed.isActive('italic') },
    { icon: 'i-lucide-strikethrough', title: 'Зачёркнутый', run: () => ed.chain().focus().toggleStrike().run(), active: () => ed.isActive('strike') },
    { icon: 'i-lucide-heading-2', title: 'Заголовок', run: () => ed.chain().focus().toggleHeading({ level: 2 }).run(), active: () => ed.isActive('heading', { level: 2 }) },
    { icon: 'i-lucide-list', title: 'Список', run: () => ed.chain().focus().toggleBulletList().run(), active: () => ed.isActive('bulletList') },
    { icon: 'i-lucide-list-ordered', title: 'Нумерованный список', run: () => ed.chain().focus().toggleOrderedList().run(), active: () => ed.isActive('orderedList') },
    { icon: 'i-lucide-quote', title: 'Цитата', run: () => ed.chain().focus().toggleBlockquote().run(), active: () => ed.isActive('blockquote') },
    { icon: 'i-lucide-code', title: 'Код', run: () => ed.chain().focus().toggleCodeBlock().run(), active: () => ed.isActive('codeBlock') },
    { icon: 'i-lucide-link', title: 'Ссылка', run: setLink, active: () => ed.isActive('link') },
    { icon: 'i-lucide-image', title: 'Картинка', run: pickImage },
    { icon: 'i-lucide-undo-2', title: 'Отменить', run: () => ed.chain().focus().undo().run() },
    { icon: 'i-lucide-redo-2', title: 'Повторить', run: () => ed.chain().focus().redo().run() },
  ]
})

defineExpose({
  insertHTML(html: string) {
    editor.value?.chain().focus('end').insertContent(html).run()
  },
})
</script>

<template>
  <div class="rounded-md border border-default bg-default">
    <div class="flex flex-wrap items-center gap-0.5 border-b border-default p-1">
      <UButton
        v-for="t in tools"
        :key="t.icon"
        :icon="t.icon"
        :title="t.title"
        :aria-label="t.title"
        size="xs"
        color="neutral"
        :variant="t.active?.() ? 'soft' : 'ghost'"
        @mousedown.prevent
        @click="t.run()"
      />
      <UIcon v-if="uploading" name="i-lucide-loader-circle" class="ml-1 size-4 animate-spin text-dimmed" />
    </div>

    <EditorContent :editor="editor" class="px-3 py-2" />

    <input ref="fileInput" type="file" accept="image/*" multiple class="hidden" @change="onPick">

    <p v-if="uploadError" class="px-3 pb-2 text-xs text-error">
      {{ uploadError }}
    </p>

    <UModal v-model:open="linkOpen" title="Ссылка" description="Пустое поле убирает ссылку.">
      <template #body>
        <UInput
          v-model="linkHref"
          class="w-full"
          placeholder="https://"
          autofocus
          @keydown.enter.prevent="applyLink"
        />
      </template>
      <template #footer>
        <div class="flex justify-end gap-2">
          <UButton color="neutral" variant="ghost" @click="linkOpen = false">
            Отмена
          </UButton>
          <UButton @click="applyLink">
            Применить
          </UButton>
        </div>
      </template>
    </UModal>
  </div>
</template>

<style>
/* Плейсхолдер пустого документа: расширение вешает атрибут на первый узел. */
.tiptap p.is-editor-empty:first-child::before {
  content: attr(data-placeholder);
  float: left;
  height: 0;
  pointer-events: none;
  color: var(--ui-text-dimmed);
}

.tiptap img {
  max-width: 100%;
  height: auto;
}
</style>
