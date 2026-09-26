export type ServerSentEvent = {
  readonly event: string
  readonly id: string
  readonly data: string
  readonly retry: number | null
}

export type ServerSentEventParser = {
  readonly push: (chunk: string) => void
  readonly finish: () => void
}

function nextBoundary(buffer: string): { readonly index: number; readonly length: number } | null {
  const candidates = [
    { index: buffer.indexOf('\r\n\r\n'), length: 4 },
    { index: buffer.indexOf('\n\n'), length: 2 },
    { index: buffer.indexOf('\r\r'), length: 2 },
  ].filter((candidate) => candidate.index >= 0)
  if (candidates.length === 0) return null
  return candidates.reduce((earliest, candidate) => candidate.index < earliest.index ? candidate : earliest)
}

function parseFrame(raw: string): ServerSentEvent | null {
  let event = ''
  let id = ''
  let retry: number | null = null
  const data: string[] = []
  for (const line of raw.replace(/\r\n|\r/g, '\n').split('\n')) {
    if (!line || line.startsWith(':')) continue
    const separator = line.indexOf(':')
    const field = separator < 0 ? line : line.slice(0, separator)
    const rawValue = separator < 0 ? '' : line.slice(separator + 1)
    const value = rawValue.startsWith(' ') ? rawValue.slice(1) : rawValue
    if (field === 'event') event = value
    if (field === 'id' && !value.includes('\u0000')) id = value
    if (field === 'data') data.push(value)
    if (field === 'retry' && /^\d+$/.test(value)) retry = Number(value)
  }
  if (data.length === 0) return null
  return { event, id, data: data.join('\n'), retry }
}

export function createServerSentEventParser(onEvent: (event: ServerSentEvent) => void): ServerSentEventParser {
  let buffer = ''

  function drain(final: boolean): void {
    let boundary = nextBoundary(buffer)
    while (boundary) {
      const raw = buffer.slice(0, boundary.index)
      buffer = buffer.slice(boundary.index + boundary.length)
      const frame = parseFrame(raw)
      if (frame) onEvent(frame)
      boundary = nextBoundary(buffer)
    }
    if (final && buffer) {
      const frame = parseFrame(buffer)
      buffer = ''
      if (frame) onEvent(frame)
    }
  }

  return {
    push(chunk: string) {
      buffer += chunk
      drain(false)
    },
    finish() {
      drain(true)
    },
  }
}

export function cursorIsNewer(candidate: string, current: string): boolean {
  if (!candidate) return false
  if (!current) return true
  if (candidate === current) return false
  if (!/^\d+$/.test(candidate) || !/^\d+$/.test(current)) return true
  return BigInt(candidate) > BigInt(current)
}
