// Открытая модалка живёт в query-параметре, а не в своём сторе: перезагрузка,
// кнопка «назад» и ссылка на заметку продолжают работать.
function parseId(v: unknown): bigint | null {
  return typeof v === 'string' && /^\d+$/.test(v) ? BigInt(v) : null
}

export function useModals() {
  const route = useRoute()
  const router = useRouter()

  const noteId = computed(() => parseId(route.query.note))
  const cardId = computed(() => parseId(route.query.card))

  const openNote = (id: bigint) => router.push({ query: { note: String(id) } })
  const openCard = (id: bigint) => router.push({ query: { card: String(id) } })
  const closeModal = () => router.push({ query: {} })

  return { noteId, cardId, openNote, openCard, closeModal }
}
