import { apiClient } from '@/api/client'

export type MascotChatRole = 'user' | 'assistant'

export interface CommunityParticipation {
  user_id: number
  level: number
  is_admin: boolean
  reason: string
}

export interface MascotChatMessage {
  role: MascotChatRole
  content: string
}

export interface MascotChatRequest {
  mascot_key: string
  message: string
  locale?: string
  conversation?: MascotChatMessage[]
}

export interface MascotChatResponse {
  mascot_key: string
  name: string
  reply: string
  mode: 'ai-ready-fallback' | 'ai'
  prompt: string
  suggestions: string[]
  moderation_hint: string
}

export type CommunityPostKind = 'card' | 'token' | 'support' | 'pool' | 'feedback' | 'announcement'
export type CommunityScenario = 'resource_decision' | 'incident_support' | 'feedback_triage' | 'demand_match' | 'capability_showcase' | 'delivery_collaboration' | 'usage_intel' | 'announcement' | 'general'
export type CommunitySubjectType = 'shared_pool' | 'market_demand' | 'workbench_task' | 'capability_asset' | 'model' | 'user' | 'organization' | ''
export type CommunityActionType = 'discuss' | 'ask_help' | 'report' | 'share_signal' | 'recommend' | 'offer' | 'request' | 'accept' | 'deliver' | 'review' | 'announce'

export interface CommunityComment {
  id: number
  post_id: number
  user_id: number
  author: string
  body: string
  helper_role: string
  official: boolean
  status: string
  created_at: string
  updated_at: string
}

export interface CommunityPost {
  id: number
  user_id: number
  author: string
  kind: CommunityPostKind
  title: string
  body: string
  tags: string[]
  district?: string
  channel?: string
  private: boolean
  status: string
  pinned: boolean
  catches: number
  replies: number
  views: number
  source_type?: string
  source_id?: string
  scenario?: CommunityScenario | string
  subject_type?: CommunitySubjectType | string
  subject_id?: string
  subject_title?: string
  action_type?: CommunityActionType | string
  evidence?: unknown[]
  trust_signals?: Record<string, unknown>
  comments?: CommunityComment[]
  created_at: string
  updated_at: string
}

export interface CreateCommunityPostPayload {
  kind: CommunityPostKind
  title: string
  body: string
  tags?: string[]
  district?: string
  channel?: string
  private?: boolean
  source_type?: string
  source_id?: string
  scenario?: CommunityScenario | string
  subject_type?: CommunitySubjectType | string
  subject_id?: string
  subject_title?: string
  action_type?: CommunityActionType | string
  evidence?: unknown[]
  trust_signals?: Record<string, unknown>
}

export interface CreateCommunityCommentPayload {
  body: string
  helper_role?: string
}

export interface CommunityPostActionPayload {
  action: 'accept_comment' | 'set_status' | 'mark_resolved' | 'reopen'
  status?: string
  comment_id?: number
}

export interface AdminCommunityPostStatusPayload {
  status: string
  pinned?: boolean
}

export interface AdminCommunityCommentStatusPayload {
  status: string
  official?: boolean
}

export interface TokenPowerLeaderboardRow {
  rank: number
  user_id: number
  display_name: string
  effective_spend: number
  request_count: number
  token_count: number
  reward_cap: number
  reward_amount: number
  review_status: string
}

export interface TokenPowerLeaderboard {
  week_start: string
  week_end: string
  reward_cap: number
  rules: string[]
  rows: TokenPowerLeaderboardRow[]
}

export interface CommunityPostQuery {
  kind?: string
  district?: string
  channel?: string
  source_type?: string
  source_id?: string
  scenario?: CommunityScenario | string
  subject_type?: CommunitySubjectType | string
  subject_id?: string
  action_type?: CommunityActionType | string
  status?: string
  private?: boolean
  limit?: number
}

export interface SharedPoolCommunitySummary {
  pool_id: number
  total_posts: number
  discussion_posts: number
  feedback_posts: number
  incident_posts: number
  risk_signals: number
  last_post_title: string
  last_post_at?: string
  latest_posts?: CommunityPost[]
}

export interface CommunityPollOption {
  id: number
  label: string
  position: number
  vote_count: number
}

export interface CommunityPoll {
  proposal_kind?: 'general' | 'announcement' | 'activity' | 'improvement' | 'rule'
  minimum_votes?: number
  support_percent?: number
  display_days?: number
  decision?: 'voting' | 'published' | 'rejected' | 'expired' | 'withdrawn'
  published_at?: string
  expires_at?: string
  id: number
  post_id: number
  owner_user_id: number
  author: string
  title: string
  body: string
  status: 'open' | 'closed'
  options: CommunityPollOption[]
  total_votes: number
  viewer_option_id: number
  can_close: boolean
  closes_at?: string | null
  closed_at?: string | null
  created_at: string
}

export interface CreateCommunityPollPayload {
  proposal_kind?: CommunityPoll['proposal_kind']
  title: string
  body: string
  options: string[]
  closes_at?: string
}

export interface CommunityAnnouncementPolicy {
  minimum_votes: number
  support_percent: number
  voting_hours: number
  display_days: number
}

export const communityAPI = {
  listPolls(limit = 30): Promise<{ items: CommunityPoll[]; announcement_policy?: CommunityAnnouncementPolicy }> {
    return apiClient
      .get<{ items: CommunityPoll[]; announcement_policy?: CommunityAnnouncementPolicy }>('/biz/community/polls', { params: { limit } })
      .then((response) => response.data)
  },

  listPlayerAnnouncements(): Promise<{ items: CommunityPoll[] }> {
    return apiClient.get<{ items: CommunityPoll[] }>('/biz/community/player-announcements').then((response) => response.data)
  },

  getPoll(pollId: number): Promise<CommunityPoll> {
    return apiClient.get<CommunityPoll>(`/biz/community/polls/${pollId}`).then((response) => response.data)
  },

  createPoll(payload: CreateCommunityPollPayload): Promise<CommunityPoll> {
    return apiClient
      .post<CommunityPoll>('/biz/community/polls', payload)
      .then((response) => response.data)
  },

  votePoll(pollId: number, optionId: number): Promise<CommunityPoll> {
    return apiClient
      .post<CommunityPoll>(`/biz/community/polls/${pollId}/vote`, { option_id: optionId })
      .then((response) => response.data)
  },

  closePoll(pollId: number): Promise<CommunityPoll> {
    return apiClient
      .post<CommunityPoll>(`/biz/community/polls/${pollId}/close`)
      .then((response) => response.data)
  },

  listPosts(kindOrQuery: string | CommunityPostQuery = 'all', limit = 50): Promise<{ items: CommunityPost[] }> {
    const params: CommunityPostQuery = typeof kindOrQuery === 'string' ? { kind: kindOrQuery, limit } : kindOrQuery
    return apiClient
      .get<{ items: CommunityPost[] }>('/biz/community/posts', { params })
      .then((response) => response.data)
  },

  listMyPosts(kind = 'all', limit = 50): Promise<{ items: CommunityPost[] }> {
    return apiClient
      .get<{ items: CommunityPost[] }>('/biz/community/my-posts', { params: { kind, limit } })
      .then((response) => response.data)
  },

  listSharedPoolSummaries(poolIds: number[], limit = 3): Promise<{ items: SharedPoolCommunitySummary[] }> {
    const ids = Array.from(new Set(poolIds.filter((id) => Number.isFinite(id) && id > 0))).slice(0, 100)
    if (!ids.length) return Promise.resolve({ items: [] })
    return apiClient
      .get<{ items: SharedPoolCommunitySummary[] }>('/biz/community/shared-pool-summaries', { params: { pool_ids: ids.join(','), limit } })
      .then((response) => response.data)
  },

  createPost(payload: CreateCommunityPostPayload): Promise<CommunityPost> {
    return apiClient
      .post<CommunityPost>('/biz/community/posts', payload)
      .then((response) => response.data)
  },

  updatePostAction(postId: number, payload: CommunityPostActionPayload): Promise<CommunityPost> {
    return apiClient
      .patch<CommunityPost>(`/biz/community/posts/${postId}/action`, payload)
      .then((response) => response.data)
  },

  adminUpdatePostStatus(postId: number, payload: AdminCommunityPostStatusPayload): Promise<CommunityPost> {
    return apiClient
      .patch<CommunityPost>(`/admin/biz/community/posts/${postId}/status`, payload)
      .then((response) => response.data)
  },

  adminUpdateCommentStatus(commentId: number, payload: AdminCommunityCommentStatusPayload): Promise<CommunityComment> {
    return apiClient
      .patch<CommunityComment>(`/admin/biz/community/comments/${commentId}/status`, payload)
      .then((response) => response.data)
  },

  listComments(postId: number, limit = 50): Promise<{ items: CommunityComment[] }> {
    return apiClient
      .get<{ items: CommunityComment[] }>(`/biz/community/posts/${postId}/comments`, { params: { limit } })
      .then((response) => response.data)
  },

  createComment(postId: number, payload: CreateCommunityCommentPayload): Promise<CommunityComment> {
    return apiClient
      .post<CommunityComment>(`/biz/community/posts/${postId}/comments`, payload)
      .then((response) => response.data)
  },

  getTokenPower(limit = 10): Promise<TokenPowerLeaderboard> {
    return apiClient
      .get<TokenPowerLeaderboard>('/biz/community/token-power', { params: { limit } })
      .then((response) => response.data)
  },

  chatMascot(payload: MascotChatRequest): Promise<MascotChatResponse> {
    return apiClient
      .post<MascotChatResponse>('/biz/community/mascot-chat', payload)
      .then((response) => response.data)
  }
}

export default communityAPI

export async function getCommunityParticipation(): Promise<CommunityParticipation> {
  return (await apiClient.get<CommunityParticipation>('/biz/community/participation')).data
}
