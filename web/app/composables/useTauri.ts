import { Code, ConnectError, type Transport } from '@connectrpc/connect'
import { create, fromBinary, toBinary } from '@bufbuild/protobuf'
import { Channel, convertFileSrc, invoke, isTauri as inTauri } from '@tauri-apps/api/core'

// В приложении сервера нет: тот же protobuf уходит в Rust через invoke.
// Единственное, что отличает сборку под Android и Windows от веба, — этот файл.
export function isTauri(): boolean {
  return import.meta.client && inTauri()
}

const errorCodes: Record<string, Code> = {
  not_found: Code.NotFound,
  invalid_argument: Code.InvalidArgument,
  internal: Code.Internal,
}

function toConnectError(e: unknown): ConnectError {
  const { code, message } = (e ?? {}) as { code?: string, message?: string }
  return new ConnectError(message ?? String(e), errorCodes[code ?? ''] ?? Code.Internal)
}

// На Android Tauri не умеет читать сырое тело запроса и шлёт IPC через
// postMessage, где Uint8Array превращается в массив чисел (втрое дороже
// и в Rust приезжает не как байты). Поэтому все команды принимают base64.
function toBase64(bytes: Uint8Array): string {
  let binary = ''
  for (let i = 0; i < bytes.length; i += 0x8000)
    binary += String.fromCharCode(...bytes.subarray(i, i + 0x8000))
  return btoa(binary)
}

export function tauriTransport(): Transport {
  return {
    async unary(method, _signal, _timeoutMs, _headers, input) {
      const request = toBinary(method.input, create(method.input, input))
      const path = `${method.parent.typeName}/${method.name}`
      let response: ArrayBuffer
      try {
        response = await invoke<ArrayBuffer>('rpc', toBase64(request), { headers: { method: path } })
      } catch (e) {
        throw toConnectError(e)
      }
      return {
        stream: false,
        service: method.parent,
        method,
        header: new Headers(),
        trailer: new Headers(),
        message: fromBinary(method.output, new Uint8Array(response)),
      }
    },
    stream() {
      throw new ConnectError('стриминг не используется', Code.Unimplemented)
    },
  } as Transport
}

/// Путь к файлу на диске превращается в ссылку, которую понимает WebView.
export function mediaSrc(path: string): string {
  return convertFileSrc(path)
}

export async function uploadImageTauri(file: File): Promise<string> {
  const ext = (file.name.split('.').pop() || 'png').toLowerCase()
  const bytes = new Uint8Array(await file.arrayBuffer())
  const path = await invoke<string>('upload_image', toBase64(bytes), { headers: { ext } })
  return convertFileSrc(path)
}

export async function attachAudioTauri(id: bigint, blob: Blob): Promise<void> {
  const wav = await toWav16k(blob)
  await invoke('attach_audio', toBase64(wav), { headers: { 'note-id': String(id) } })
}

export type ModelProgress = { file: string, done: number, total: number }

export async function modelReady(): Promise<boolean> {
  return invoke<boolean>('model_ready')
}

export async function modelDownload(onProgress: (p: ModelProgress) => void): Promise<void> {
  const progress = new Channel<ModelProgress>()
  progress.onmessage = onProgress
  await invoke('model_download', { progress })
}

// В Rust нет декодера Opus, поэтому запись MediaRecorder приводим к WAV
// 16 kHz mono здесь — у WebView декодер уже есть.
export async function toWav16k(blob: Blob): Promise<Uint8Array> {
  const ctx = new AudioContext()
  const decoded = await ctx.decodeAudioData(await blob.arrayBuffer())
  await ctx.close()

  const rate = 16000
  const offline = new OfflineAudioContext(1, Math.ceil(decoded.duration * rate), rate)
  const source = offline.createBufferSource()
  source.buffer = decoded
  source.connect(offline.destination)
  source.start()
  const rendered = await offline.startRendering()
  return encodeWav(rendered.getChannelData(0), rate)
}

function encodeWav(samples: Float32Array, rate: number): Uint8Array {
  const bytes = new Uint8Array(44 + samples.length * 2)
  const view = new DataView(bytes.buffer)
  const ascii = (offset: number, text: string) => {
    for (let i = 0; i < text.length; i++) view.setUint8(offset + i, text.charCodeAt(i))
  }
  ascii(0, 'RIFF')
  view.setUint32(4, 36 + samples.length * 2, true)
  ascii(8, 'WAVEfmt ')
  view.setUint32(16, 16, true)
  view.setUint16(20, 1, true)
  view.setUint16(22, 1, true)
  view.setUint32(24, rate, true)
  view.setUint32(28, rate * 2, true)
  view.setUint16(32, 2, true)
  view.setUint16(34, 16, true)
  ascii(36, 'data')
  view.setUint32(40, samples.length * 2, true)
  for (let i = 0; i < samples.length; i++) {
    const s = Math.max(-1, Math.min(1, samples[i]!))
    view.setInt16(44 + i * 2, s < 0 ? s * 0x8000 : s * 0x7fff, true)
  }
  return bytes
}
