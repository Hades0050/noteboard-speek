type Ask = { title: string, description?: string, confirmLabel?: string }
type Pending = Ask & { resolve: (ok: boolean) => void }

// Браузерный confirm() в WebView на Android выглядит чужеродно и блокирует
// поток, поэтому вопросы задаёт модалка — одна на всё приложение.
export function useConfirm() {
  return useState<Pending | null>('confirm', () => null)
}

export function askConfirm(ask: Ask): Promise<boolean> {
  const pending = useConfirm()
  return new Promise((resolve) => { pending.value = { ...ask, resolve } })
}
