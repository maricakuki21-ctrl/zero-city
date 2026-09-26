import { apiClient } from '@/api/client'

export function newRewardRequestID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  // Also work on HTTP test hosts, where randomUUID is not exposed.
  const bytes = new Uint32Array(4)
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    crypto.getRandomValues(bytes)
    return `reward-${Array.from(bytes, n => n.toString(16)).join('-')}`
  }
  return `reward-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

export interface RewardResource { id: string; label: string; model: string }
export interface TokenGrant {
  id: number; packet_id: number; resource_id: string; model: string
  tokens: number; used: number; reserved: number; expires_at: string
}
export interface TokenPacket {
  id: number; message_id: number; sender_id: number; resource_id: string; model: string
  total_tokens: number; portions: number; claimed: number; mode: 'equal' | 'random'
  blessing: string; opens_at: string; closes_at: string; use_hours: number
  server_time: string; mine?: TokenGrant
}
export interface TokenPacketInput {
  client_id: string; resource_id: string; total_tokens: number; portions: number
  mode: 'equal' | 'random'; blessing: string; delay_seconds: number; claim_hours: number; use_hours: number
}
export interface RewardRun {
  id: number; client_id: string; resource_id: string; model: string; reserved: number
  actual: number; covered: number; status: 'pending' | 'succeeded' | 'failed' | 'review'
  result: string; note: string; created_at: string
}
export interface RewardWallet { grants: TokenGrant[]; runs: RewardRun[]; server_time: string }
export interface RewardReview extends RewardRun { user_id: number; gateway_request_id: string }
export const tokenRewardsAPI = {
  resources: () => apiClient.get<{ items: RewardResource[] }>('/biz/token-rewards/resources').then(r => r.data.items),
  create: (slug: string, input: TokenPacketInput) => apiClient.post<TokenPacket>(`/biz/chat/channels/${encodeURIComponent(slug)}/token-packets`, input).then(r => r.data),
  get: (id: number) => apiClient.get<TokenPacket>(`/biz/chat/token-packets/${id}`).then(r => r.data),
  claim: (id: number) => apiClient.post<TokenGrant>(`/biz/chat/token-packets/${id}/claim`).then(r => r.data),
  wallet: () => apiClient.get<RewardWallet>('/biz/token-rewards/wallet').then(r => r.data),
  reviews: () => apiClient.get<{ items: RewardReview[] }>('/biz/token-rewards/reviews').then(r => r.data.items),
  resolve: (id: number, actual_tokens: number, evidence: string) => apiClient.post<RewardRun>(`/biz/token-rewards/reviews/${id}/resolve`, { actual_tokens, evidence }).then(r => r.data),
  run: (input: { client_id: string; resource_id: string; prompt: string; max_output_tokens: number }) =>
    apiClient.post<RewardRun>('/biz/token-rewards/runs', input, { timeout: 175000 }).then(r => r.data),
}
export function availableReward(grant: TokenGrant, now: number): number {
  return Date.parse(grant.expires_at) > now ? Math.max(0, grant.tokens - grant.used - grant.reserved) : 0
}
