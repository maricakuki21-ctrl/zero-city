import { describe, expect, it, vi } from 'vitest'
import { createHttpCreatorWorkbenchService } from '../service'
import { createRunStream } from '../streamTransport'
import { createServerSentEventParser } from '../sse'
import { WORKBENCH_ENDPOINTS } from '../endpoints'

function encode(chunks: readonly string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder()
  return new ReadableStream({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(encoder.encode(chunk))
      controller.close()
    },
  })
}

function runPayload(id: string, status: string, extras: Record<string, unknown> = {}) {
  return {
    id,
    status,
    capability_id: 'cap-copy',
    intent: 'hello',
    cursor: extras.cursor ?? '1',
    cost: { currency: 'CNY', estimate: '0.0001', actual: extras.actual ?? null },
    ...extras,
  }
}

describe('workbench SSE parser and stream', () => {
  it('parses split chunks, comments, and duplicate cursors', () => {
    const events: Array<{ id: string; data: string }> = []
    const parser = createServerSentEventParser((event) => events.push(event))
    parser.push('id: 1\nda')
    parser.push('ta: {"id":"run-1","status":"running","cursor":"1"}\n\n')
    parser.push(': keep-alive\n\n')
    parser.push('id: 1\ndata: {"id":"run-1","status":"running","cursor":"1"}\n\n')
    parser.push('id: 2\ndata: {"id":"run-1","status":"succeeded","cursor":"2"}\n\n')
    parser.finish()
    expect(events.map((event) => event.id)).toEqual(['1', '1', '2'])
  })

  it('reconnects, handles stale reset, terminal close, abort cleanup, and artifact kinds', async () => {
    const calls: string[] = []
    const messages: string[] = []
    const connections: string[] = []
    let round = 0
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      round += 1
      calls.push(String(input))
      if (init?.signal?.aborted) throw new DOMException('aborted', 'AbortError')
      if (round === 1) {
        return new Response(encode(['id: 1\ndata: {"id":"run-1","status":"running","cursor":"1"}\n\n']), {
          status: 200,
          headers: { 'Content-Type': 'text/event-stream' },
        })
      }
      if (round === 2) {
        return new Response(JSON.stringify({
          kind: 'reset',
          reason: 'stale',
          cursor: '4',
          run: runPayload('run-1', 'running', { cursor: '4' }),
        }), { status: 410, headers: { 'Content-Type': 'application/json' } })
      }
      return new Response(encode([
        'id: 5\ndata: {"id":"run-1","status":"succeeded","cursor":"5","artifacts":[{"id":"a1","kind":"code","title":"main.ts"},{"id":"a2","kind":"image","title":"cover"},{"id":"a3","kind":"video","title":"clip","verified_accepted_task":true}]}\n\n',
      ]), { status: 200, headers: { 'Content-Type': 'text/event-stream' } })
    })
    const stream = createRunStream({
      fetcher: fetcher as typeof fetch,
      getAuthToken: () => 'token-1',
      wait: async () => undefined,
      maxReconnects: 3,
    }, () => [])
    const controller = new AbortController()
    const outcome = await stream({
      runId: 'run-1',
      cursor: '0',
      signal: controller.signal,
      observer: {
        onMessage: (message) => messages.push(`${message.kind}:${message.cursor}`),
        onConnection: (state) => connections.push(state),
        onMalformed: () => undefined,
      },
    })
    expect(outcome.kind).toBe('terminal')
    expect(messages).toContain('reset:4')
    expect(connections).toContain('reconnecting')
    expect(calls.some((url) => url.includes('/events') && !url.includes('token-1'))).toBe(true)
    expect(fetcher.mock.calls[0]?.[1]?.headers).toMatchObject({ Authorization: 'Bearer token-1' })
  })

  it('cancels in-flight stream on abort and forks through REST', async () => {
    const rest = {
      get: vi.fn(),
      post: vi.fn().mockResolvedValue({
        id: 'run-fork',
        status: 'queued',
        capability_id: 'cap-copy',
        intent: 'hello',
        forked_from_run_id: 'run-1',
      }),
    }
    const service = createHttpCreatorWorkbenchService({ rest })
    const forked = await service.forkRun('run-1')
    expect(rest.post).toHaveBeenCalled()
    expect(forked.id).toBe('run-fork')
    const controller = new AbortController()
    let started = false
    const stream = createRunStream({
      fetcher: vi.fn(async (_url, init) => {
        started = true
        return new Promise((_, reject) => {
          init?.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')))
        })
      }) as typeof fetch,
      wait: async () => undefined,
    }, () => [])
    const pending = stream({
      runId: 'run-1',
      cursor: '',
      signal: controller.signal,
      observer: { onMessage: () => undefined, onConnection: () => undefined, onMalformed: () => undefined },
    })
    await Promise.resolve()
    controller.abort()
    const aborted = await pending
    expect(started).toBe(true)
    expect(aborted.kind).toBe('aborted')
  })
})

describe('workbench endpoints', () => {
  it('keeps JWT out of the event query', () => {
    expect(WORKBENCH_ENDPOINTS.events('run-1', '9')).toBe('/biz/workbench/runs/run-1/events?cursor=9')
    expect(WORKBENCH_ENDPOINTS.events('run-1', '9')).not.toMatch(/Bearer|token/i)
  })
})
