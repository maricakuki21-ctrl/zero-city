import { apiClient } from '@/api/client'
import type {
  CreatorWorkbenchService,
  WorkbenchLaunchInput,
  WorkbenchRunRecord,
  WorkbenchSaveInput,
  WorkbenchSavedReplay,
  WorkbenchStreamInput,
  WorkbenchStreamOutcome,
  WorkbenchWorkspace,
} from './contracts'
import { normalizeWorkbenchSavedReplay, normalizeWorkbenchWorkspace } from './normalizers'
import { normalizeWorkbenchRun } from './runNormalizer'
import { createRunStream, type WorkbenchStreamDependencies } from './streamTransport'
import { WORKBENCH_ENDPOINTS } from './endpoints'

export type WorkbenchRestTransport = {
  readonly get: (path: string) => Promise<unknown>
  readonly post: (path: string, body: unknown, options?: { readonly headers?: Readonly<Record<string, string>> }) => Promise<unknown>
}

export type WorkbenchServiceOptions = WorkbenchStreamDependencies & {
  readonly rest?: WorkbenchRestTransport
}

function defaultRest(): WorkbenchRestTransport {
  return {
    async get(path) {
      const response = await apiClient.get<unknown>(path)
      return response.data
    },
    async post(path, body, options) {
      const response = await apiClient.post<unknown>(path, body, options ? { headers: options.headers } : undefined)
      return response.data
    },
  }
}

function operationKey(operation: string, resourceId: string): string {
  return `workbench:${operation}:${resourceId}`
}

function requestId(prefix: string): string {
  const random = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`
  return `${prefix}-${random}`
}

function normalizeRunOrThrow(value: unknown, capabilities: WorkbenchWorkspace['capabilities']): WorkbenchRunRecord {
  const run = normalizeWorkbenchRun(value, capabilities)
  if (!run) throw new Error('工作台服务返回了无法解析的运行记录。')
  return run
}

function normalizeReplayOrThrow(value: unknown, capabilities: WorkbenchWorkspace['capabilities']): WorkbenchSavedReplay {
  const replay = normalizeWorkbenchSavedReplay(value, capabilities)
  if (!replay) throw new Error('工作台服务返回了无法解析的保存快照。')
  return replay
}

export function createHttpCreatorWorkbenchService(options: WorkbenchServiceOptions = {}): CreatorWorkbenchService {
  const rest = options.rest ?? defaultRest()
  let capabilities: WorkbenchWorkspace['capabilities'] = []
  const stream = createRunStream(options, () => capabilities)

  return {
    async loadWorkspace() {
      const workspace = normalizeWorkbenchWorkspace(await rest.get(WORKBENCH_ENDPOINTS.workspace))
      capabilities = workspace.capabilities
      return workspace
    },
    async createRun(input: WorkbenchLaunchInput) {
      const value = await rest.post(WORKBENCH_ENDPOINTS.runs, {
        workspace_token: input.workspaceToken,
        capability_id: input.capabilityId,
        capability_version: input.capabilityVersion,
        capability_digest: input.capabilityDigest,
        canonical_model_id: input.canonicalModelId,
        canonical_model_version: input.canonicalModelVersion,
        accepted_quote_id: input.acceptedQuoteId,
        accepted_quote_sha: input.acceptedQuoteSha,
        intent: input.intent,
        locale: input.locale,
        retry_of_run_id: input.retryOfRunId,
        request_id: requestId(input.retryOfRunId ? 'retry' : 'launch'),
      })
      return normalizeRunOrThrow(value, capabilities)
    },
    async refreshRun(runId: string) {
      return normalizeRunOrThrow(await rest.get(WORKBENCH_ENDPOINTS.run(runId)), capabilities)
    },
    async cancelRun(runId: string) {
      return normalizeRunOrThrow(
        await rest.post(WORKBENCH_ENDPOINTS.cancel(runId), undefined, { headers: { 'Idempotency-Key': operationKey('cancel', runId) } }),
        capabilities,
      )
    },
    async saveRun(runId: string, payload: WorkbenchSaveInput) {
      return normalizeReplayOrThrow(
        await rest.post(WORKBENCH_ENDPOINTS.save(runId), payload, { headers: { 'Idempotency-Key': operationKey('save', runId) } }),
        capabilities,
      )
    },
    async replayRun(savedReplayId: string) {
      return normalizeRunOrThrow(
        await rest.post(WORKBENCH_ENDPOINTS.replay(savedReplayId), undefined, { headers: { 'Idempotency-Key': operationKey('replay', savedReplayId) } }),
        capabilities,
      )
    },
    async forkRun(runId: string) {
      return normalizeRunOrThrow(
        await rest.post(WORKBENCH_ENDPOINTS.fork(runId), undefined, { headers: { 'Idempotency-Key': operationKey('fork', runId) } }),
        capabilities,
      )
    },
    async streamRun(input: WorkbenchStreamInput): Promise<WorkbenchStreamOutcome> {
      return stream(input)
    },
  }
}

export { WORKBENCH_ENDPOINTS, createRunStream }
