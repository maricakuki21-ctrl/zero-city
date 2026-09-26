import { type Ref } from 'vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import type {
  CreatorWorkbenchService,
  WorkbenchConnectionState,
  WorkbenchRunRecord,
  WorkbenchStreamMessage,
} from './contracts'

type StreamControllerOptions = {
  readonly ensureService: () => Promise<CreatorWorkbenchService>
  readonly currentRun: Ref<WorkbenchRunRecord | null>
  readonly streamCursor: Ref<string>
  readonly connectionState: Ref<WorkbenchConnectionState>
  readonly streamError: Ref<string>
  readonly statusMessage: Ref<string>
}

function isActive(run: WorkbenchRunRecord | null): boolean {
  return run?.status === 'queued' || run?.status === 'running' || run?.status === 'cancel_requested'
}

export function createStreamController(options: StreamControllerOptions) {
  let controller: AbortController | null = null
  let generation = 0

  function stopStream(): void {
    generation += 1
    controller?.abort()
    controller = null
  }

  function applyMessage(message: WorkbenchStreamMessage): void {
    options.streamCursor.value = message.cursor
    options.currentRun.value = message.run
    if (message.kind === 'reset') options.statusMessage.value = `连接已按服务端快照重置（${message.reason}）。`
    if (message.run && !isActive(message.run)) options.connectionState.value = 'closed'
  }

  function applyRun(nextRun: WorkbenchRunRecord | null, restartStream: boolean): void {
    options.currentRun.value = nextRun
    options.streamCursor.value = nextRun?.cursor ?? ''
    if (!nextRun || !isActive(nextRun)) {
      stopStream()
      options.connectionState.value = nextRun ? 'closed' : 'idle'
      return
    }
    if (restartStream) startStream(nextRun)
  }

  function startStream(run: WorkbenchRunRecord): void {
    stopStream()
    const nextController = new AbortController()
    controller = nextController
    const currentGeneration = generation
    options.connectionState.value = 'connecting'
    void (async () => {
      try {
        const service = await options.ensureService()
        const outcome = await service.streamRun({
          runId: run.id,
          cursor: options.streamCursor.value,
          signal: nextController.signal,
          observer: {
            onMessage: applyMessage,
            onConnection: (state) => {
              if (currentGeneration === generation) options.connectionState.value = state
            },
            onMalformed: (message) => {
              if (currentGeneration === generation) options.streamError.value = message
            },
          },
        })
        if (currentGeneration !== generation || nextController.signal.aborted) return
        if (outcome.kind === 'exhausted') {
          options.connectionState.value = 'exhausted'
          const snapshot = await service.refreshRun(run.id)
          if (currentGeneration !== generation || nextController.signal.aborted) return
          options.currentRun.value = snapshot
          options.streamCursor.value = snapshot.cursor
          options.statusMessage.value = '实时连接已暂时中断，已回退到一次性快照。可手动重新连接。'
        }
      } catch (error) {
        if (currentGeneration === generation && !nextController.signal.aborted) {
          options.streamError.value = extractApiErrorMessage(error, '实时运行连接失败')
          options.connectionState.value = 'exhausted'
        }
      }
    })()
  }

  function reconnect(): void {
    if (options.currentRun.value && isActive(options.currentRun.value)) startStream(options.currentRun.value)
  }

  return { applyMessage, applyRun, reconnect, startStream, stopStream }
}
