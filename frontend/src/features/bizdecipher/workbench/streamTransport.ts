import { buildApiUrl } from '@/api/client'
import type {
  WorkbenchCapability,
  WorkbenchStreamInput,
  WorkbenchStreamMessage,
  WorkbenchStreamOutcome,
} from './contracts'
import { normalizeStreamMessage, normalizeStreamReset } from './normalizers'
import { isRecord, readBoolean, readField, unwrapApiData } from './contractValues'
import { WORKBENCH_ENDPOINTS } from './endpoints'
import { createServerSentEventParser, cursorIsNewer, type ServerSentEvent } from './sse'

const DEFAULT_MAX_RECONNECTS = 3
const MAX_RETRY_DELAY_MS = 2000

export type WorkbenchStreamDependencies = {
  readonly fetcher?: typeof fetch
  readonly getAuthToken?: () => string
  readonly isOnline?: () => boolean
  readonly onlineTarget?: EventTarget
  readonly wait?: (milliseconds: number, signal: AbortSignal) => Promise<void>
  readonly maxReconnects?: number
}

function defaultToken(): string {
  try {
    return localStorage.getItem('auth_token') ?? ''
  } catch {
    return ''
  }
}

function defaultOnline(): boolean {
  return typeof navigator === 'undefined' || navigator.onLine
}

function waitWithAbort(milliseconds: number, signal: AbortSignal): Promise<void> {
  if (signal.aborted) return Promise.resolve()
  return new Promise((resolve) => {
    const timer = globalThis.setTimeout(done, milliseconds)
    function done(): void {
      globalThis.clearTimeout(timer)
      signal.removeEventListener('abort', done)
      resolve()
    }
    signal.addEventListener('abort', done, { once: true })
  })
}

function waitUntilOnline(signal: AbortSignal, target: EventTarget, isOnline: () => boolean): Promise<void> {
  if (signal.aborted || isOnline()) return Promise.resolve()
  return new Promise((resolve) => {
    function done(): void {
      target.removeEventListener('online', onOnline)
      signal.removeEventListener('abort', done)
      resolve()
    }
    function onOnline(): void {
      if (isOnline()) done()
    }
    target.addEventListener('online', onOnline)
    signal.addEventListener('abort', done, { once: true })
  })
}

function isTerminal(message: WorkbenchStreamMessage): boolean {
  const status = message.run?.status
  return status === 'succeeded' || status === 'failed' || status === 'cancelled'
}

function isAbort(error: unknown, signal: AbortSignal): boolean {
  return signal.aborted || (error instanceof DOMException && error.name === 'AbortError')
}

async function readReset(response: Response, capabilities: readonly WorkbenchCapability[]): Promise<WorkbenchStreamMessage | null> {
  try {
    const value: unknown = await response.json()
    return normalizeStreamReset(value, capabilities)
  } catch {
    return null
  }
}

function streamHeaders(cursor: string, token: string): Readonly<Record<string, string>> {
  const headers: Record<string, string> = { Accept: 'text/event-stream', 'Cache-Control': 'no-cache' }
  if (cursor) headers['Last-Event-ID'] = cursor
  if (token) headers.Authorization = `Bearer ${token}`
  return headers
}

export function createRunStream(
  dependencies: WorkbenchStreamDependencies,
  capabilities: () => readonly WorkbenchCapability[],
): (input: WorkbenchStreamInput) => Promise<WorkbenchStreamOutcome> {
  const fetcher = dependencies.fetcher ?? globalThis.fetch
  const getAuthToken = dependencies.getAuthToken ?? defaultToken
  const isOnline = dependencies.isOnline ?? defaultOnline
  const onlineTarget = dependencies.onlineTarget ?? globalThis.window
  const wait = dependencies.wait ?? waitWithAbort
  const maxReconnects = dependencies.maxReconnects ?? DEFAULT_MAX_RECONNECTS

  return async (input: WorkbenchStreamInput): Promise<WorkbenchStreamOutcome> => {
    let cursor = input.cursor
    let reconnects = 0
    let retryDelay = 250
    while (!input.signal.aborted) {
      if (!isOnline()) {
        input.observer.onConnection('offline')
        await waitUntilOnline(input.signal, onlineTarget, isOnline)
        if (input.signal.aborted) break
      }
      input.observer.onConnection(reconnects === 0 ? 'connecting' : 'reconnecting')
      try {
        const response = await fetcher(buildApiUrl(WORKBENCH_ENDPOINTS.events(input.runId, cursor)), {
          method: 'GET',
          headers: streamHeaders(cursor, getAuthToken()),
          credentials: 'include',
          cache: 'no-store',
          signal: input.signal,
        })
        if (response.status === 410) {
          const reset = await readReset(response, capabilities())
          if (!reset || reset.kind !== 'reset') input.observer.onMalformed('服务端返回了无法解析的游标重置响应。')
          else {
            cursor = reset.cursor
            input.observer.onMessage(reset)
          }
        } else {
          if (!response.ok || !response.body) throw new Error(`workbench stream HTTP ${response.status}`)
          input.observer.onConnection('connected')
          const reader = response.body.getReader()
          const decoder = new TextDecoder()
          let terminal = false
          const parser = createServerSentEventParser((frame: ServerSentEvent) => {
            if (frame.retry !== null) retryDelay = Math.min(frame.retry, MAX_RETRY_DELAY_MS)
            let value: unknown
            try {
              value = JSON.parse(frame.data)
            } catch {
              input.observer.onMalformed('忽略了一条格式错误的工作台事件。')
              return
            }
            const message = normalizeStreamMessage(frame.event, frame.id, value, capabilities())
            if (!message) {
              input.observer.onMalformed('忽略了一条缺少游标或运行快照的工作台事件。')
              return
            }
            if (message.kind === 'event' && !cursorIsNewer(message.cursor, cursor)) return
            cursor = message.cursor
            input.observer.onMessage(message)
            const raw = unwrapApiData(value)
            const explicitTerminal = isRecord(raw) && readBoolean(readField(raw, 'terminal'))
            terminal = terminal || explicitTerminal || frame.event === 'terminal' || isTerminal(message)
          })
          try {
            while (!terminal && !input.signal.aborted) {
              const chunk = await reader.read()
              if (chunk.done) break
              parser.push(decoder.decode(chunk.value, { stream: true }))
            }
            parser.push(decoder.decode())
            parser.finish()
            if (terminal) {
              await reader.cancel()
              input.observer.onConnection('closed')
              return { kind: 'terminal', cursor }
            }
          } finally {
            reader.releaseLock()
          }
        }
      } catch (error) {
        if (isAbort(error, input.signal)) break
      }
      if (reconnects >= maxReconnects) {
        input.observer.onConnection('exhausted')
        return { kind: 'exhausted', cursor }
      }
      reconnects += 1
      input.observer.onConnection(isOnline() ? 'reconnecting' : 'offline')
      await wait(retryDelay * reconnects, input.signal)
    }
    input.observer.onConnection('closed')
    return { kind: 'aborted', cursor }
  }
}
