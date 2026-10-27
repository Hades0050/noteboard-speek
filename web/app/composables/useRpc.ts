import { createClient, type Transport } from '@connectrpc/connect'
import { createConnectTransport } from '@connectrpc/connect-web'
import { BoardService, NotesService } from '~/gen/noteboard/v1/noteboard_pb'

// В вебе RPC уходят в Go по HTTP, в приложении — в Rust через invoke.
// Выбор откладываем до первого вызова: isTauri() отвечает только на клиенте.
let impl: Transport | undefined

function pick(): Transport {
  impl ??= isTauri()
    ? tauriTransport()
    // baseUrl относительный: в dev его проксирует Nuxt на Go (:8080),
    // в проде фронт отдаёт сам Go — значит, один origin.
    // useBinaryFormat: connect-web по умолчанию шлёт JSON — читаемо в DevTools,
    // но втрое тяжелее. Бинарный protobuf тот же Connect-протокол, тот же URL.
    : createConnectTransport({ baseUrl: '/api/rpc', useBinaryFormat: true })
  return impl
}

const transport: Transport = {
  unary: (...args) => pick().unary(...args),
  stream: (...args) => pick().stream(...args),
}

export const notesRpc = createClient(NotesService, transport)
export const boardRpc = createClient(BoardService, transport)

// id в proto — int64, в TypeScript это bigint. Хелперы для мест, где id
// приходит строкой из URL или уходит в :key.
export function toID(v: unknown): bigint {
  return BigInt(String(v ?? 0))
}
