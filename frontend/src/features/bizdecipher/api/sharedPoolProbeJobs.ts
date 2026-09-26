import { apiClient } from '@/api/client'
import type {
  SharedPoolFullCheckItem,
  SharedPoolUpstreamProbePayload,
  SharedPoolUpstreamProbeResult,
} from './bizdecipher'

export type SharedPoolProbeJobStatus =
  | 'queued'
  | 'running'
  | 'succeeded'
  | 'failed'
  | 'timed_out'
  | 'stale'
  | 'cancelled'

export type SharedPoolProbeJobItem = {
  index: number
  check_id: string
  title: string
  category: string
  required: boolean
  success: boolean
  http_status: number
  latency_ms: number
  error_type?: string
  error_message?: string
  evidence?: string
}

export type SharedPoolProbeJob = {
  id: string
  operation_id: string
  pool_id: number
  account_id?: number
  owner_id: number
  model_name: string
  upstream_model_name: string
  probe_type: string
  check_level: string
  config_version: number
  status: SharedPoolProbeJobStatus
  attempt: number
  max_attempts: number
  lease_expires_at?: string
  heartbeat_at?: string
  started_at?: string
  finished_at?: string
  result?: SharedPoolUpstreamProbeResult
  error_type?: string
  error_message?: string
  created_at: string
  updated_at: string
  items?: SharedPoolProbeJobItem[]
  deduplicated?: boolean
}

export type SharedPoolProbeJobWaitOptions = {
  signal?: AbortSignal
  pollIntervalMs?: number
  timeoutMs?: number
  onUpdate?: (job: SharedPoolProbeJob) => void
}

const terminalProbeJobStatuses = new Set<SharedPoolProbeJobStatus>([
  'succeeded',
  'failed',
  'timed_out',
  'stale',
  'cancelled',
])

export function createSharedPoolProbeOperationID(): string {
  if (typeof globalThis.crypto?.randomUUID === 'function') {
    return globalThis.crypto.randomUUID()
  }
  return `shared-pool-probe-${Date.now()}-${Math.random().toString(36).slice(2, 12)}`
}

export function isSharedPoolProbeJobTerminal(
  jobOrStatus: SharedPoolProbeJob | SharedPoolProbeJobStatus | string | null | undefined,
): boolean {
  const status = typeof jobOrStatus === 'object' && jobOrStatus
    ? jobOrStatus.status
    : jobOrStatus
  return terminalProbeJobStatuses.has(status as SharedPoolProbeJobStatus)
}

export async function probeSharedPoolUpstream(
  payload: SharedPoolUpstreamProbePayload,
): Promise<SharedPoolProbeJob> {
  const operationID = payload.operation_id?.trim() || createSharedPoolProbeOperationID()
  const requestPayload: SharedPoolUpstreamProbePayload = {
    ...payload,
    operation_id: operationID,
  }
  const { data } = await apiClient.post<SharedPoolProbeJob>(
    '/biz/upstream/probe',
    requestPayload,
    {
      timeout: 30000,
      headers: { 'Idempotency-Key': operationID },
    },
  )
  return data
}

export async function getSharedPoolProbeJob(jobID: string): Promise<SharedPoolProbeJob> {
  const { data } = await apiClient.get<SharedPoolProbeJob>(
    `/biz/upstream/probe-jobs/${encodeURIComponent(jobID)}`,
    { timeout: 15000 },
  )
  return data
}

function pollingDelay(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new DOMException('Probe polling aborted', 'AbortError'))
      return
    }
    const onAbort = () => {
      globalThis.clearTimeout(timer)
      reject(new DOMException('Probe polling aborted', 'AbortError'))
    }
    const timer = globalThis.setTimeout(() => {
      signal?.removeEventListener('abort', onAbort)
      resolve()
    }, ms)
    signal?.addEventListener('abort', onAbort, { once: true })
  })
}

function isRetryablePollingError(error: unknown): boolean {
  const status = Number((error as { status?: number })?.status || 0)
  return status === 0 || status === 408 || status === 429 || status >= 500
}

export async function waitForSharedPoolProbeJob(
  jobID: string,
  options: SharedPoolProbeJobWaitOptions = {},
): Promise<SharedPoolProbeJob> {
  const pollIntervalMs = Math.max(250, options.pollIntervalMs ?? 1500)
  const timeoutMs = Math.max(1000, options.timeoutMs ?? 12 * 60 * 1000)
  const deadline = Date.now() + timeoutMs
  let lastError: unknown

  while (Date.now() < deadline) {
    if (options.signal?.aborted) {
      throw new DOMException('Probe polling aborted', 'AbortError')
    }
    try {
      const job = await getSharedPoolProbeJob(jobID)
      if (options.signal?.aborted) {
        throw new DOMException('Probe polling aborted', 'AbortError')
      }
      lastError = undefined
      options.onUpdate?.(job)
      if (isSharedPoolProbeJobTerminal(job)) return job
    } catch (error) {
      if (options.signal?.aborted || (error as { name?: string })?.name === 'AbortError') {
        throw error
      }
      if (!isRetryablePollingError(error)) throw error
      lastError = error
    }
    await pollingDelay(pollIntervalMs, options.signal)
  }

  const suffix = lastError instanceof Error && lastError.message
    ? `: ${lastError.message}`
    : ''
  throw new Error(`等待检测任务结果超时，任务仍会在服务器后台继续${suffix}`)
}

export function sharedPoolProbeChecks(job: SharedPoolProbeJob): SharedPoolFullCheckItem[] {
  if (job.result?.checks?.length) return job.result.checks
  return (job.items || []).map((item) => ({
    id: item.check_id,
    title: item.title,
    category: item.category,
    required: item.required,
    success: item.success,
    http_status: item.http_status,
    latency_ms: item.latency_ms,
    evidence: item.evidence,
    error_type: item.error_type,
    error_message: item.error_message,
  }))
}
