import { apiClient } from '@/api/client'

export type SharedPoolUsageTraceStatus = 'pending' | 'succeeded' | 'failed'
export type SharedPoolSettlementOutcome = 'pending' | 'settled' | 'released' | 'failed' | 'not_required' | 'unknown'

export interface SharedPoolUsageTrace {
  id: number
  pool_id: number
  request_id: string
  pool_name_snapshot: string
  model: string
  endpoint: string
  account_alias: string
  status: SharedPoolUsageTraceStatus
  failure_stage?: string
  settlement_outcome: SharedPoolSettlementOutcome
  upstream_started: boolean
  usage_observed: boolean
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_creation_tokens: number
  auth_latency_ms?: number
  seat_latency_ms?: number
  routing_latency_ms?: number
  concurrency_latency_ms?: number
  reservation_latency_ms?: number
  upstream_latency_ms?: number
  first_token_ms?: number
  settlement_latency_ms?: number
  total_latency_ms?: number
  retry_count: number
  http_status?: number
  created_at: string
  completed_at?: string
}

export interface SharedPoolUsageTracePage {
  items: SharedPoolUsageTrace[]
  next_before_id?: number
  has_more: boolean
}

export async function listMySharedPoolUsageTraces(options: {
  beforeId?: number
  limit?: number
} = {}): Promise<SharedPoolUsageTracePage> {
  const { data } = await apiClient.get<SharedPoolUsageTracePage>('/biz/shared-pool/usage-traces', {
    params: {
      before_id: options.beforeId,
      limit: options.limit ?? 20,
    },
  })
  return data
}
