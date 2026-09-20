export function api<T>(path: string, opts: Record<string, unknown> = {}): Promise<T> {
  return $fetch<T>(`/api${path}`, opts as never)
}

export function formatDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })
}

// Новая задача создаётся пустой, а без подписи она неотличима от соседей
// на доске: пока заголовок не набрали, им служит время создания.
export function autoTitle(): string {
  return `Задача ${formatDate(new Date().toISOString())}`
}

// Тело заметки хранится как HTML — для превью и подписей нужен голый текст.
export function stripHtml(html: string): string {
  return html
    .replace(/<br\s*\/?>/gi, ' ')
    .replace(/<\/(p|div|li|h[1-6])>/gi, ' ')
    .replace(/<[^>]+>/g, '')
    .replace(/&nbsp;/g, ' ')
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
}
