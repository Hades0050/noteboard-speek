import { createClient } from '@connectrpc/connect'
import { createConnectTransport } from '@connectrpc/connect-web'
import { BoardService, NotesService } from '~/gen/noteboard/v1/noteboard_pb'

// Один транспорт на приложение. baseUrl относительный: в dev его проксирует
// Nuxt на Go (:8080), в проде фронт отдаёт сам Go — значит, один origin.
// useBinaryFormat: connect-web по умолчанию шлёт JSON — читаемо в DevTools,
// но втрое тяжелее. Бинарный protobuf тот же Connect-протокол, тот же URL.
const transport = createConnectTransport({ baseUrl: '/api/rpc', useBinaryFormat: true })

export const notesRpc = createClient(NotesService, transport)
export const boardRpc = createClient(BoardService, transport)

// id в proto — int64, в TypeScript это bigint. Хелперы для мест, где id
// приходит строкой из URL или уходит в :key.
export function toID(v: unknown): bigint {
  return BigInt(String(v ?? 0))
}
