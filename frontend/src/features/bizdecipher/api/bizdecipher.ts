import { apiClient } from '@/api/client'
import type { CommunityPost, CommunityPostQuery } from './community'

export {
  createSharedPoolProbeOperationID,
  getSharedPoolProbeJob,
  isSharedPoolProbeJobTerminal,
  probeSharedPoolUpstream,
  sharedPoolProbeChecks,
  waitForSharedPoolProbeJob,
} from './sharedPoolProbeJobs'
export type {
  SharedPoolProbeJob,
  SharedPoolProbeJobItem,
  SharedPoolProbeJobStatus,
  SharedPoolProbeJobWaitOptions,
} from './sharedPoolProbeJobs'

export type BizPoolStatus = {
  health: string
  active_resources: number
  testing_resources: number
  pending_contributors: number
  open_requests: number
}

// PlatformStatus reflects the health of the underlying API gateway (channels
// & upstream accounts), deliberately separate from the marketplace pool status.
export type PlatformStatus = {
  health: string
  active_channels: number
  total_channels: number
  active_accounts: number
  error_accounts: number
  total_accounts: number
}

export type BizListResponse<T> = {
  items: T[]
}

export type BizContributorPayload = {
  capacity_types: string[]
  settlement_method: string
  capacity_hint: string
  label: string
}

export type BizOperatorPayload = {
  skills: string[]
}

export type BizCustomRequestPayload = {
  goal: string
  context: string
  budget_range: string
  deadline?: string
  references: string[]
}

export type BizCreditLedgerEntry = {
  id: number
  user_id: number
  source_type: string
  source_id?: string
  amount: number
  balance_after: number
  status?: string
  note?: string
  created_at?: string
  posted_at?: string
}

export type BizCreditGrantPayload = {
  user_id: number
  source_type?: string
  source_id?: string
  amount: number
  note?: string
}

export async function getBizPoolStatus(): Promise<BizPoolStatus> {
  const { data } = await apiClient.get<BizPoolStatus>('/biz/pool/status')
  return data
}

// GET /biz/platform/status — gateway-level health, independent of the pool.
export async function getBizPlatformStatus(): Promise<PlatformStatus> {
  const { data } = await apiClient.get<PlatformStatus>('/biz/platform/status')
  return data
}

export async function applyBizContributor(payload: BizContributorPayload) {
  const { data } = await apiClient.post('/biz/contributor/apply', payload)
  return data
}

export async function applyBizOperator(payload: BizOperatorPayload) {
  const { data } = await apiClient.post('/biz/operator/apply', payload)
  return data
}

export async function createBizCustomRequest(payload: BizCustomRequestPayload) {
  const { data } = await apiClient.post('/biz/custom-requests', payload)
  return data
}

export async function listMyBizCredits() {
  const { data } = await apiClient.get<BizListResponse<BizCreditLedgerEntry>>('/biz/credits')
  return data
}

export async function adminListBizContributors() {
  const { data } = await apiClient.get<BizListResponse<unknown>>('/admin/biz/contributors')
  return data
}

export async function adminListBizOperators() {
  const { data } = await apiClient.get<BizListResponse<unknown>>('/admin/biz/operators')
  return data
}

export async function adminListBizRequests() {
  const { data } = await apiClient.get<BizListResponse<unknown>>('/admin/biz/requests')
  return data
}

export async function adminListBizCredits() {
  const { data } = await apiClient.get<BizListResponse<unknown>>('/admin/biz/credits')
  return data
}

export async function adminGrantBizCredit(payload: BizCreditGrantPayload) {
  const { data } = await apiClient.post('/admin/biz/credits/grant', payload)
  return data
}

// ==================== Promo campaigns ====================

export type PromoCampaign = {
  id: number
  name: string
  type: string
  title: string
  description: string
  enabled: boolean
  credit_amount: number
  start_at: string
  end_at: string
  target: string
  auto_hide: boolean
  created_at: string
  updated_at: string
}

export type PromoActiveView = {
  campaign: PromoCampaign | null
  claimed: boolean
  claimable: boolean
}

export type PromoClaimResult = {
  promo_id: number
  amount: number
  balance_after: number
  already_claimed: boolean
}

export type PromoCampaignPayload = {
  name: string
  type?: string
  title?: string
  description?: string
  enabled: boolean
  credit_amount: number
  start_at?: string
  end_at?: string
  target?: string
  auto_hide?: boolean
}

// GET /biz/promo/active — active campaign + current user's claim state.
export async function getActivePromo(): Promise<PromoActiveView> {
  const { data } = await apiClient.get<PromoActiveView>('/biz/promo/active')
  return data
}

// POST /biz/promo/:id/claim — claim the campaign reward (idempotent).
export async function claimPromo(promoID: number): Promise<PromoClaimResult> {
  const { data } = await apiClient.post<PromoClaimResult>(`/biz/promo/${promoID}/claim`)
  return data
}

export async function adminListPromos(): Promise<BizListResponse<PromoCampaign>> {
  const { data } = await apiClient.get<BizListResponse<PromoCampaign>>('/admin/biz/promos')
  return data
}

export async function adminCreatePromo(payload: PromoCampaignPayload): Promise<PromoCampaign> {
  const { data } = await apiClient.post<PromoCampaign>('/admin/biz/promos', payload)
  return data
}

export async function adminUpdatePromo(id: number, payload: PromoCampaignPayload): Promise<PromoCampaign> {
  const { data } = await apiClient.patch<PromoCampaign>(`/admin/biz/promos/${id}`, payload)
  return data
}

// ==================== Shared pools (Account Square, read-only) ====================

export type SharedPoolPriceSnapshot = {
  billing_mode: string
  input_price?: number | null
  output_price?: number | null
  cache_write_price?: number | null
  cache_read_price?: number | null
  image_output_price?: number | null
  per_request_price?: number | null
  price_source?: string
}

export type SharedPoolEndpointType = 'chat' | 'responses' | 'image_generation' | 'image_edit' | 'video'

export type SharedPoolBillingMode = 'token' | 'per_request' | 'image' | 'video'

export type SharedPoolPriceComponents = {
  billing_mode: SharedPoolBillingMode
  currency: 'USD' | string
  input_price?: number | null
  output_price?: number | null
  cache_read_price?: number | null
  cache_write_price?: number | null
  image_item_price?: number | null
  video_second_price?: number | null
  per_request_price?: number | null
  minimum_charge?: number | null
  maximum_charge?: number | null
}

export type SharedPoolPriceQuote = {
  price_version_id: number
  pool_id: number
  pool_model_id: number
  endpoint_id: number
  model_name: string
  endpoint_type: SharedPoolEndpointType
  pricing_source: 'official_catalog' | 'owner_custom'
  pricing_status: string
  config_version: number
  base_price: SharedPoolPriceComponents
  multiplier: number
  fee_mode?: 'buyer_surcharge_v1'
  platform_fee_percent?: number
  owner_price?: SharedPoolPriceComponents
  user_price: SharedPoolPriceComponents
  effective_from: string
  explicit_free: boolean
  example_cost: number
}

export type SharedPoolModelEndpointPricing = {
  pool_model_id: number
  provider: string
  model_name: string
  display_name: string
  pricing_source: 'official_catalog' | 'owner_custom'
  pricing_status: string
  pricing_config_version: number
  endpoint_id: number
  endpoint_type: SharedPoolEndpointType
  enabled: boolean
  gate_status: string
  endpoint_pricing_status: string
  config_version: number
  current_price?: SharedPoolPriceQuote | null
}

export type SaveSharedPoolCustomPricePayload = {
  model_name: string
  endpoint_type: SharedPoolEndpointType
  operation_id: string
  billing_mode: SharedPoolBillingMode
  input_price?: number | null
  output_price?: number | null
  cache_read_price?: number | null
  cache_write_price?: number | null
  image_item_price?: number | null
  video_second_price?: number | null
  per_request_price?: number | null
  multiplier: number
  minimum_charge?: number | null
  maximum_charge?: number | null
}

export type ModelCatalogEntry = {
  id: number
  provider: string
  model_name: string
  display_name: string
  family: string
  tier_label: string
  capability_tags: string[]
  aliases: string[]
  default_rate_multiplier: number
  default_rank_weight: number
  mainstream: boolean
  enabled: boolean
  sort_order: number
  modalities_in: string[]
  modalities_out: string[]
  adapter_kind: string
  context_window: number
  orchestrator: boolean
  runtime_role: string
  pricing?: SharedPoolPriceSnapshot | null
}

export type SharedPoolModelConfig = {
  provider: string
  model_name: string
  upstream_model_name?: string
  display_name?: string
  aliases?: string[]
  rate_multiplier: number
  rank_weight?: number
  five_hour_protection_percent: number
  seven_day_protection_percent: number
  daily_protection_percent?: number
  min_balance_admission?: number
  hourly_seat_fee?: number
  hourly_min_usage_waiver?: number
  max_concurrency: number
  enabled?: boolean
  model_open?: boolean
  tags?: string[]
  pricing?: SharedPoolPriceSnapshot | null
}


export type SharedPoolAccount = {
  id: number
  pool_id: number
  owner_id: number
  name: string
  description?: string
  provider: string
  auth_type: string
  upstream_base_url: string
  has_upstream_key: boolean
  has_oauth_credentials: boolean
  key_preview: string
  expires_at?: string | null
  auto_pause_on_expired: boolean
  schedulable: boolean
  status: string
  status_note?: string
  disabled_reason?: string
  group_name?: string
  proxy_id?: number | null
  proxy_url?: string
  proxy_region?: string
  proxy_status?: string
  account_weight: number
  priority: number
  rpm_limit: number
  account_concurrency: number
  user_concurrency: number
  tls_profile_id?: number | null
  ttl_seconds: number
  cache_policy?: Record<string, unknown>
  routing_policy?: Record<string, unknown>
  model_configs: SharedPoolModelConfig[]
  last_probe_at?: string | null
  last_probe_success?: boolean | null
  last_probe_error_type?: string
  last_probe_error_message?: string
  last_successful_probe_at?: string | null
  full_check_score: number
  full_check_passed: number
  full_check_total: number
  gate_required: boolean
  gate_passed: boolean
  total_calls: number
  successful_calls: number
  failed_calls: number
  last_used_at?: string | null
  native_operation_id?: string
  native_binding_state?: SharedPoolNativeAccountBindingState
  native_binding_step?: string
  native_error_code?: string
  native_models?: string[]
  native_connection_status?: string
  native_connection_verified_at?: string | null
  native_evidence_stale?: boolean
  native_models_verified_at?: string | null
  billing_activation_required?: boolean
  created_at: string
  updated_at: string
}

export type SharedPoolNativeOnboardingState =
  | 'draft'
  | 'supply_configuring'
  | 'supply_needs_attention'
  | 'supply_ready_billing_blocked'
  | 'billing_active'
  | 'legacy_existing'

export type SharedPoolNativeAccountBindingState =
  | 'pending'
  | 'creating'
  | 'attached'
  | 'verifying'
  | 'ready'
  | 'needs_attention'
  | 'detached'

export type CreateNativeSharedPoolDraftPayload = {
  name: string
  description?: string
  operation_id: string
}

export type CreateNativeSharedPoolAccountPayload = {
  operation_id: string
  name: string
  description?: string
  provider: string
  auth_type: 'api_key' | 'oauth'
  upstream_base_url: string
  upstream_api_key?: string
  credentials?: Record<string, unknown>
}

export type RepairNativeSharedPoolAccountPayload = {
  repair_operation_id: string
  expected_config_version: number
  provider: string
  auth_type: 'api_key' | 'oauth'
  upstream_base_url: string
  upstream_api_key?: string
  credentials?: Record<string, unknown>
}

export type VerifyNativeSharedPoolReadinessPayload = {
  expected_config_version: number
}

export type ActivateNativeSharedPoolBillingPayload = {
  operation_id: string
  expected_config_version: number
}

export type SharedPoolNativeActivation = {
  pool_id: number
  state: SharedPoolNativeOnboardingState
  config_version: number
  activated_config_version: number
  activated_at?: string
  already_active: boolean
}

export type SharedPoolAccountPayload = Partial<Omit<SharedPoolAccount, 'id' | 'pool_id' | 'owner_id' | 'has_upstream_key' | 'key_preview' | 'created_at' | 'updated_at' | 'last_used_at'>> & {
  name: string
  upstream_base_url: string
  upstream_api_key?: string
  model_configs: SharedPoolModelConfig[]
}

export type SharedPoolAccountImportResult = {
  total: number
  created: number
  failed: number
  items: Array<{
    index: number
    name: string
    created: boolean
    account?: SharedPoolAccount
    error?: string
  }>
}

export type SharedPoolOAuthDataPackage = {
  type?: string
  version?: string
  accounts: Array<{
    name?: string
    platform: string
    type: string
    credentials: Record<string, unknown>
    extra?: Record<string, unknown>
    concurrency?: number
    priority?: number
    expires_at?: number
    auto_pause_on_expired?: boolean
  }>
  proxies?: unknown[]
}

export type SharedPoolOAuthImportResult = {
  total: number
  created: number
  updated: number
  skipped: number
  failed: number
  items: Array<{
    index: number
    name: string
    action: 'created' | 'updated' | 'skipped' | 'failed'
    account_id?: number
    message?: string
  }>
  warnings?: Array<{ index: number; name: string; message: string }>
  errors?: Array<{ index: number; name: string; message: string }>
}

export type SharedPoolAccountSummary = {
  total_accounts: number
  configured_accounts: number
  schedulable_accounts: number
  gate_blocked_accounts: number
  disabled_accounts: number
  total_account_concurrency: number
  total_user_concurrency: number
  total_rpm_limit: number
  average_full_check_score: number
  full_check_passed_accounts: number
  full_check_total_accounts: number
  total_calls: number
  successful_calls: number
  failed_calls: number
  success_rate: number
  model_coverage: string[]
  last_probe_at?: string | null
}

export type SharedPoolOwnerCardAsset = {
  collectible_count: number
  highest_rarity: string
  featured_card_key: string
  featured_card_rarity: string
  featured_card_serial_no?: number | null
  featured_card_edition_no?: number | null
  featured_card_supply?: number | null
  profile_background_ready: boolean
}

export type SharedPool = {

  id: number
  name: string
  description: string
  avatar_url?: string
  status_note?: string
  disabled_reason?: string
  featured_score?: number
  owner_id?: number
  owner_label: string
  owner_card_asset?: SharedPoolOwnerCardAsset
  tier: string
  status: string
  listed?: boolean
  supply_mode?: 'native' | 'legacy' | string
  native_onboarding_state?: SharedPoolNativeOnboardingState
  billing_activation_required?: boolean
  models: string[]
  model_configs?: SharedPoolModelConfig[]
  account_summary?: SharedPoolAccountSummary
  rate_multiplier: number

  max_users: number
  current_users: number
  min_balance_admission: number
  hourly_seat_fee: number
  hourly_min_usage_waiver?: number
  today_availability: number
  seven_day_availability: number
  avg_latency_ms: number
  last_probe_at?: string
  last_probe_success?: boolean
  last_probe_error_type?: string
  last_probe_error_message?: string
  consecutive_probe_failures?: number
  last_successful_probe_at?: string
  last_probe_check_level?: string
  last_probe_gate_required?: boolean
  last_probe_gate_passed?: boolean
  last_probe_full_check_passed?: number
  last_probe_full_check_total?: number
  last_probe_full_check_score?: number
  upstream_base_url?: string
  has_upstream_key?: boolean
  proxy_id?: number | null
  proxy_url?: string
  proxy_region?: string
  proxy_status?: string
  account_concurrency?: number
  user_concurrency?: number
  account_mode_enabled?: boolean
  oauth_provider?: string
  verification_mode?: 'full_check' | 'professional_review' | string
  verification_exemption_reason?: string
  settlement_rule_effective_from?: string
  pending_hourly_seat_fee?: number
  pending_hourly_min_usage_waiver?: number
  pending_platform_fee_percent?: number
  pending_settlement_rule_effective_from?: string
  owner_share_percent?: number
  platform_fee_percent?: number
  quality_score?: number

  rank_weight?: number
  reward_score?: number
  penalty_score?: number
  market_score?: number
  governance_status?: string
  governance_note?: string
  admin_note?: string
  lifecycle_state?: string
  archived_at?: string | null
  archive_reason?: string
  complaint_count?: number
  like_count?: number
  liked_by_me?: boolean
  total_calls?: number
  successful_calls?: number
  failed_calls?: number
  card_skin_key?: string | null
  owner_paused?: boolean
  card_skin_rarity?: string | null
  config_version?: number
}

export type SharedPoolGovernanceSummary = {
  snapshot_at: string
  today_gross_charges: number
  today_owner_payout: number
  today_platform_fee: number
  today_usage_count: number
  today_active_pools: number
  open_complaints: number
  listed_pools: number
  healthy_pools: number
  limited_pools: number
  offline_pools: number
  watch_pools: number
  suppressed_pools: number
  banned_pools: number
  total_calls: number
  successful_calls: number
  failed_calls: number
  average_availability: number
  platform_fee_basis: string
  auto_governance_summary: string
}

export type SharedPoolListView = {
  pools: SharedPool[]
  total: number
  online: number
  limited: number
  avg_availability: number
  governance_summary?: SharedPoolGovernanceSummary
  projections?: readonly SharedMarketPoolProjection[]
}

export type SharedMarketSource =
  | 'sub2.model_catalog'
  | 'sub2.group'
  | 'sub2.account'
  | 'sub2.health'
  | 'sub2.usage_log'
  | 'bizdecipher.pool'
  | 'bizdecipher.membership'
  | 'bizdecipher.price_snapshot'
  | 'bizdecipher.canonical_ledger'
  | 'bizdecipher.community'
  | 'official.service_status'

export type SharedMarketEvidence = {
  readonly source: SharedMarketSource
  readonly observed_at: string
  readonly freshness: 'live' | 'recent' | 'last_verified'
  readonly confidence: 'high' | 'medium' | 'unknown'
}

export type SharedMarketFact<T> = {
  readonly value: T
  readonly evidence: SharedMarketEvidence
}

export type SharedMarketRuntimeFacts = {
  readonly group_id: SharedMarketFact<string>
  readonly account_ids: SharedMarketFact<readonly string[]>
  readonly models: SharedMarketFact<readonly { readonly name: string; readonly executable: boolean }[]>
  readonly group_availability: SharedMarketFact<{ readonly state: string; readonly available: number; readonly total: number; readonly explanation?: string }>
  readonly account_availability: SharedMarketFact<{ readonly state: string; readonly available: number; readonly total: number; readonly explanation?: string }>
  readonly health: SharedMarketFact<{ readonly state: string; readonly confidence: 'high' | 'medium' | 'unknown'; readonly reason?: string }>
  readonly today_availability_percent: SharedMarketFact<string>
  readonly seven_day_availability_percent: SharedMarketFact<string>
  readonly latency_ms: SharedMarketFact<string>
  readonly throughput_rpm: SharedMarketFact<string>
  readonly success_rate_percent: SharedMarketFact<string>
  readonly canonical_usage: SharedMarketFact<{
    readonly requests_succeeded: string
    readonly requests_failed: string
    readonly input_tokens: string
    readonly output_tokens: string
  }>
}

export type SharedMarketProductFacts = {
  readonly name: SharedMarketFact<string>
  readonly description: SharedMarketFact<string>
  readonly owner_label: SharedMarketFact<string>
  readonly membership: {
    readonly current_users: SharedMarketFact<number>
    readonly maximum_users: SharedMarketFact<number>
  }
  readonly pricing: {
    readonly rate_multiplier: SharedMarketFact<string>
    readonly minimum_balance: SharedMarketFact<string>
    readonly hourly_seat_fee: SharedMarketFact<string>
    readonly hourly_usage_waiver: SharedMarketFact<string>
  }
  readonly financial?: {
    readonly available_balance: SharedMarketFact<string>
    readonly owner_gross: SharedMarketFact<string>
    readonly owner_net: SharedMarketFact<string>
    readonly platform_fee: SharedMarketFact<string>
    readonly processor_fee: SharedMarketFact<string>
    readonly residue: SharedMarketFact<string>
  }
  readonly community: {
    readonly likes: SharedMarketFact<number>
    readonly complaints: SharedMarketFact<number>
    readonly discussions: SharedMarketFact<number>
  }
}

export type SharedMarketPoolProjection = {
  readonly pool_id: number
  readonly product: SharedMarketProductFacts
  readonly runtime?: SharedMarketRuntimeFacts
  readonly official_service_status: SharedMarketFact<string>
  readonly stale: boolean
  readonly errors: readonly {
    readonly dependency: string
    readonly code: string
    readonly message: string
    readonly retryable: boolean
    readonly action: string
  }[]
}

export type BizProfile = {
  user_id: number
  handle: string
  display_name: string
  avatar_url: string
  bio: string
  profile_visibility?: 'public' | 'private' | string
  profile_theme?: string
  background_card_key?: string
  background_card_rarity?: string
  background_card_serial_no?: number | null
  background_card_edition_no?: number | null
  background_card_edition_supply?: number | null
  created_at: string
  updated_at: string
}

export type ZeroCityProfileStats = {
  followers: number
  following: number
  community_posts: number
  shared_pools: number
  collectible_cards: number
}

export type ZeroCityFollowState = {
  is_following: boolean
  followers: number
  following: number
}

export type ZeroCityPublicProfile = {
  profile: BizProfile
  stats: ZeroCityProfileStats
  follow_state: ZeroCityFollowState
  shared_pools: SharedPool[]
  posts: CommunityPost[]
}

export type SharedPoolQuery = {
  keyword?: string
  model?: string
  status?: string
  view?: 'public' | 'observation' | string
  lifecycle?: 'current' | 'attention' | 'archived' | 'all'
  min_availability?: number
  sort?: string
  limit?: number
}


export type CreateSharedPoolPayload = {
  name: string
  description?: string
  avatar_url?: string
  status_note?: string
  disabled_reason?: string
  upstream_base_url?: string
  upstream_api_key?: string
  models: string[]
  model_configs?: SharedPoolModelConfig[]
  rate_multiplier?: number
  max_users?: number
  min_balance_admission?: number
  hourly_seat_fee?: number
  hourly_min_usage_waiver?: number
  proxy_id?: number | null
  proxy_url?: string
  proxy_region?: string
  proxy_status?: string
  account_concurrency?: number
  user_concurrency?: number
  account_mode_enabled?: boolean
  oauth_provider?: string
  verification_mode?: 'full_check' | 'professional_review' | string
  verification_exemption_reason?: string
  probe_model?: string
  listed?: boolean
  status?: string
}


export type UpdateSharedPoolPayload = Partial<Omit<CreateSharedPoolPayload, 'upstream_api_key'>> & {
  upstream_api_key?: string
  sync_model_rates?: boolean
  expected_config_version: number
}

export type SharedPoolImportItemPayload = CreateSharedPoolPayload

export type SharedPoolImportItemResult = {
  index: number
  name: string
  created: boolean
  pool?: SharedPool
  error?: string
}

export type SharedPoolImportResult = {
  total: number
  created: number
  failed: number
  items: SharedPoolImportItemResult[]
}

export type SharedPoolUpstreamProbePayload = {
  pool_id?: number
  account_id?: number
  upstream_base_url?: string
  upstream_api_key?: string
  probe_model?: string
  probe_type?: 'manual' | 'publish_gate' | 'scheduled' | 'scheduled_full'
  proxy_url?: string
  operation_id?: string
}

export type SharedPoolUpstreamModelsPayload = {
  pool_id?: number
  account_id?: number
  upstream_base_url?: string
  upstream_api_key?: string
  proxy_url?: string
}

export type SharedPoolUpstreamModelsResult = {
  models: string[]
  checked_at: string
  http_status: number
}

export type SharedPoolFullCheckItem = {
  id: string
  title: string
  category: string
  required: boolean
  success: boolean
  http_status?: number
  latency_ms?: number
  evidence?: string
  error_type?: string
  error_message?: string
}

/** Public full-check report from GET /biz/pools/:id/full-check-report */
export type SharedPoolFullCheckReport = {
  pool_id: number
  report_id?: string
  checked_at?: string
  model?: string
  score?: number
  passed?: number
  total?: number
  gate_passed?: boolean
  probe_type?: string
  checks?: SharedPoolFullCheckItem[]
  has_full_check: boolean
  message?: string
}

export type SharedPoolProbeMetadata = {
  check_level: string
  gate_required: boolean
  gate_passed: boolean
  full_check_passed: number
  full_check_total: number
  full_check_score: number
  checks?: SharedPoolFullCheckItem[]
}

export type SharedPoolUpstreamProbeResult = {
  ok: boolean
  model: string
  models: string[]
  message: string
  checked_at: string
  latency_ms?: number
  http_status?: number
  error_type?: string
  error_message?: string
  check_level?: string
  gate_required?: boolean
  gate_passed?: boolean
  full_check_passed?: number
  full_check_total?: number
  full_check_score?: number
  checks?: SharedPoolFullCheckItem[]
}

export type SharedPoolProbeHistory = {
  id: number
  pool_id: number
  account_id?: number
  owner_id?: number
  model_name: string
  upstream_model_name: string
  probe_type: 'manual' | 'publish_gate' | 'scheduled' | 'scheduled_full' | string
  success: boolean
  http_status: number
  error_type: string
  error_message: string
  latency_ms: number
  checked_at: string
  created_at: string
  metadata?: SharedPoolProbeMetadata
}

export type SharedPoolGovernanceLog = {
  id: number
  pool_id: number
  admin_user_id?: number
  action: string
  before_value: unknown
  after_value: unknown
  reason: string
  created_at: string
}

export type SharedPoolProbeAggregationSummary = {
  pools_checked: number
  pool_level_checked?: number
  accounts_checked?: number
  succeeded: number
  failed: number
  limited: number
  offlined: number
  candidates_found?: number
  skipped_cooldown?: boolean
  message?: string
}


export type SharedPoolAccessKey = {
  id: number
  pool_id: number
  pool_name: string
  user_id: number
  api_key_id: number
  name: string
  key?: string
  key_preview: string
  status: string
  allowed_models: string[]
  total_used: number
  last_used_at?: string
  created_at: string
  /** true when this binding participates in multi-pool unified routing */
  account_mode?: boolean
}

export type SharedPoolLedgerEntry = {
  id: number
  user_id: number
  pool_id?: number
  source_type: string
  source_id: string
  asset_type: 'balance' | 'balance_legacy' | 'credit_balance' | string
  amount: number
  balance_after: number
  status: string
  note: string
  created_at: string
  posted_at?: string
}

export type SharedPoolOwnerWallet = {
  owner_id: number
  available_amount: number
  pending_amount: number
  frozen_amount: number
  transferred_amount: number
  total_earned: number
  version: number
  updated_at?: string
}

export type SharedPoolOwnerEarningsEntry = {
  id: number
  owner_id: number
  pool_id?: number
  account_id?: number
  price_version_id?: number
  event_type: 'earning' | 'transfer_to_balance' | 'reversal' | 'adjustment' | string
  operation_id: string
  request_id: string
  pool_name_snapshot: string
  owner_label_snapshot: string
  model_snapshot: string
  pricing_source_snapshot: string
  gross_amount: number
  platform_fee_amount: number
  net_amount: number
  wallet_delta: number
  available_after: number
  status: string
  available_at?: string
  metadata?: Record<string, unknown>
  created_at: string
  posted_at?: string
}

export type AdminSharedPoolOwnerEarningsEntry = SharedPoolOwnerEarningsEntry & {
  owner_email: string
  owner_username: string
}

export type AdminSharedPoolOwnerEarningsPage = {
  items: AdminSharedPoolOwnerEarningsEntry[]
  next_before_id?: number
  has_more: boolean
  matching_entries: number
  matching_owners: number
  matching_gross_amount: number
  matching_platform_fee: number
  matching_net_amount: number
}

export type AdminSharedPoolOwnerEarningsQuery = {
  owner_id?: number
  pool_id?: number
  before_id?: number
  kind?: '' | 'earning' | 'api' | 'seat' | 'transfer' | 'adjustment'
  status?: '' | 'pending' | 'available' | 'settled' | 'reversed'
  limit?: number
}

export type SharedPoolLedgerView = {
  wallet?: SharedPoolOwnerWallet
  earnings: SharedPoolOwnerEarningsEntry[]
  activity: SharedPoolLedgerEntry[]
  withdrawable: SharedPoolLedgerEntry[]
  legacy_withdrawable: SharedPoolLedgerEntry[]
  incentives: SharedPoolLedgerEntry[]
}

export type SharedPoolOwnerWalletTransferResult = {
  operation_id: string
  amount: number
  wallet_after: number
  balance_after: number
  already_done: boolean
}

export type SharedPoolAccessKeyResult = {
  access_key: SharedPoolAccessKey
  already_held: boolean
}

export async function listModelCatalog(): Promise<ModelCatalogEntry[]> {
  const { data } = await apiClient.get<{ models: ModelCatalogEntry[] }>('/biz/models')
  return data?.models ?? []
}

export type ModelCapabilityProfileInput = {
  modalities_in?: string[]
  modalities_out?: string[]
  adapter_kind?: string
  context_window?: number
  orchestrator?: boolean
  runtime_role?: string
}

export async function updateModelCatalogProfile(
  id: number,
  payload: ModelCapabilityProfileInput,
): Promise<ModelCatalogEntry> {
  const { data } = await apiClient.patch<ModelCatalogEntry>(`/admin/biz/models/${id}/profile`, payload)
  return data
}

export async function getZeroCityPublicProfile(target: string | number): Promise<ZeroCityPublicProfile> {
  const { data } = await apiClient.get<ZeroCityPublicProfile>(`/biz/zero-city/profiles/${encodeURIComponent(String(target))}`)
  return data
}

export async function getMyZeroCityProfile(): Promise<BizProfile> {
  const { data } = await apiClient.get<BizProfile>('/biz/profile')
  return data
}

export async function updateMyZeroCityProfile(payload: { display_name: string; avatar_url: string }): Promise<BizProfile> {
  const { data } = await apiClient.put<BizProfile>('/biz/profile', payload)
  return data
}

export async function followZeroCityProfile(userId: number): Promise<ZeroCityFollowState> {
  const { data } = await apiClient.post<ZeroCityFollowState>(`/biz/zero-city/profiles/${userId}/follow`)
  return data
}

export async function unfollowZeroCityProfile(userId: number): Promise<ZeroCityFollowState> {
  const { data } = await apiClient.delete<ZeroCityFollowState>(`/biz/zero-city/profiles/${userId}/follow`)
  return data
}

export async function updateZeroCityProfileBackground(payload: { card_key: string; serial_no?: number | null }): Promise<BizProfile> {
  const { data } = await apiClient.put<BizProfile>('/biz/zero-city/profile/background', payload)
  return data
}

// GET /biz/pools — public marketplace catalog + aggregate header stats.
export async function listSharedPools(query: SharedPoolQuery = {}): Promise<SharedPoolListView> {

  const params: Record<string, string | number> = {}
  if (query.keyword) params.keyword = query.keyword
  if (query.model && query.model !== 'all') params.model = query.model
  if (query.status && query.status !== 'all') params.status = query.status
  if (query.view && query.view !== 'public') params.view = query.view

  if (query.min_availability) params.min_availability = query.min_availability
  if (query.sort) params.sort = query.sort
  if (query.limit) params.limit = query.limit
  const { data } = await apiClient.get<SharedPoolListView>('/biz/pools', { params })
  return data
}

export type SharedPoolGovernancePayload = {
  platform_fee_percent?: number
  featured_score?: number
  reward_score?: number
  penalty_score?: number
  governance_status?: 'normal' | 'boosted' | 'watch' | 'suppressed' | 'banned'
  governance_note?: string
  admin_note?: string
  listed?: boolean
  status?: 'healthy' | 'limited' | 'offline' | 'maintenance'
}

export type SharedPoolRestorePayload = {
  reason?: string
  operation_id?: string
}

export type SharedPoolUsageReviewState = 'pending' | 'processing' | 'resolved' | 'all'

export type SharedPoolUsageReviewAction = 'release' | 'capture_hold' | 'settle_amount'

export type SharedPoolUsageReview = {
  reservation_id: number
  request_id: string
  pool_id: number
  pool_name: string
  user_id: number
  user_label: string
  model: string
  endpoint_type: string
  pricing_source: string
  hold_amount: number
  reported_amount: number
  settled_amount: number
  reservation_status: string
  trigger_reason: string
  reserved_at: string
  forward_started_at?: string
  expires_at: string
  finalized_at?: string
  resolution_id?: number
  resolution_admin_id?: number
  resolution_action?: SharedPoolUsageReviewAction
  resolution_amount?: number
  resolution_note?: string
  resolution_operation_id?: string
  resolved_at?: string
}

export type SharedPoolUsageReviewPage = {
  items: SharedPoolUsageReview[]
  next_before_id?: number
  has_more: boolean
  summary?: SharedPoolUsageReviewQueueSummary
}

export type SharedPoolUsageReviewGroupSummary = {
  pool_id: number
  pool_name: string
  model: string
  trigger_reason: string
  pending_count: number
  pending_hold: number
  oldest_reserved_at?: string
}

export type SharedPoolUsageReviewQueueSummary = {
  pending_count: number
  pending_hold: number
  oldest_reserved_at?: string
  groups: SharedPoolUsageReviewGroupSummary[]
}

export type SharedPoolUsageReviewPolicy = {
  auto_release_enabled: boolean
  auto_release_minutes: number
  auto_release_max_hold: number
}

export type SharedPoolUsageReviewQuery = {
  state?: SharedPoolUsageReviewState
  before_id?: number
  limit?: number
}

export type ResolveSharedPoolUsageReviewPayload = {
  action: 'release'
  note: string
  operation_id: string
}

export type ResolveSharedPoolUsageReviewResult = {
  review: SharedPoolUsageReview
  replay?: boolean
}

export type BatchResolveSharedPoolUsageReviewsPayload = {
  reservation_ids: number[]
  action: 'release'
  note: string
  operation_id: string
}

export type BatchResolveSharedPoolUsageReviewItem = {
  reservation_id: number
  success: boolean
  review?: SharedPoolUsageReview
  replay?: boolean
  error?: string
}

export type BatchResolveSharedPoolUsageReviewsResult = {
  items: BatchResolveSharedPoolUsageReviewItem[]
  succeeded: number
  failed: number
}

export async function adminListSharedPools(query: SharedPoolQuery = {}): Promise<SharedPoolListView> {
  const params: Record<string, string | number> = {}
  if (query.keyword) params.keyword = query.keyword
  if (query.model && query.model !== 'all') params.model = query.model
  if (query.status && query.status !== 'all') params.status = query.status
  if (query.lifecycle) params.lifecycle = query.lifecycle
  if (query.min_availability) params.min_availability = query.min_availability
  if (query.sort) params.sort = query.sort
  if (query.limit) params.limit = query.limit
  const { data } = await apiClient.get<SharedPoolListView>('/admin/biz/shared-pools', { params })
  return data
}

export async function adminListSharedPoolOwnerEarnings(
  query: AdminSharedPoolOwnerEarningsQuery = {},
): Promise<AdminSharedPoolOwnerEarningsPage> {
  const params: Record<string, string | number> = {}
  if (query.owner_id) params.owner_id = query.owner_id
  if (query.pool_id) params.pool_id = query.pool_id
  if (query.before_id) params.before_id = query.before_id
  if (query.kind) params.kind = query.kind
  if (query.status) params.status = query.status
  if (query.limit) params.limit = query.limit
  const { data } = await apiClient.get<AdminSharedPoolOwnerEarningsPage>('/admin/biz/shared-pool-owner-earnings', { params })
  return data
}

export async function adminUpdateSharedPoolGovernance(id: number, payload: SharedPoolGovernancePayload): Promise<SharedPool> {
  const { data } = await apiClient.patch<SharedPool>(`/admin/biz/shared-pools/${id}/governance`, payload)
  return data
}

export async function adminRestoreSharedPool(id: number, payload: SharedPoolRestorePayload): Promise<SharedPool> {
  const config = payload.operation_id
    ? { headers: { 'Idempotency-Key': payload.operation_id } }
    : undefined
  const { data } = await apiClient.post<SharedPool>(`/admin/biz/shared-pools/${id}/restore`, payload, config)
  return data
}

export async function adminListSharedPoolUsageReviews(
  query: SharedPoolUsageReviewQuery = {},
): Promise<SharedPoolUsageReviewPage> {
  const params: Record<string, string | number> = {
    state: query.state ?? 'pending',
    limit: query.limit ?? 20,
  }
  if (query.before_id && query.before_id > 0) params.before_id = query.before_id
  const { data } = await apiClient.get<SharedPoolUsageReviewPage>('/admin/biz/shared-pool-usage-reviews', { params })
  return data
}

export async function adminResolveSharedPoolUsageReview(
  id: number,
  payload: ResolveSharedPoolUsageReviewPayload,
): Promise<ResolveSharedPoolUsageReviewResult> {
  const { data } = await apiClient.post<ResolveSharedPoolUsageReviewResult>(
    `/admin/biz/shared-pool-usage-reviews/${id}/resolve`,
    payload,
    { headers: { 'Idempotency-Key': payload.operation_id } },
  )
  return data
}

export async function adminGetSharedPoolUsageReviewPolicy(): Promise<SharedPoolUsageReviewPolicy> {
  const { data } = await apiClient.get<SharedPoolUsageReviewPolicy>('/admin/biz/shared-pool-usage-reviews/policy')
  return data
}

export async function adminUpdateSharedPoolUsageReviewPolicy(
  payload: SharedPoolUsageReviewPolicy,
): Promise<SharedPoolUsageReviewPolicy> {
  const { data } = await apiClient.put<SharedPoolUsageReviewPolicy>('/admin/biz/shared-pool-usage-reviews/policy', payload)
  return data
}

export async function adminBatchResolveSharedPoolUsageReviews(
  payload: BatchResolveSharedPoolUsageReviewsPayload,
): Promise<BatchResolveSharedPoolUsageReviewsResult> {
  const { data } = await apiClient.post<BatchResolveSharedPoolUsageReviewsResult>(
    '/admin/biz/shared-pool-usage-reviews/batch-resolve',
    payload,
    { headers: { 'Idempotency-Key': payload.operation_id } },
  )
  return data
}

export async function adminRunSharedPoolProbeAggregation(limit = 50): Promise<SharedPoolProbeAggregationSummary> {
  const { data } = await apiClient.post<SharedPoolProbeAggregationSummary>('/admin/biz/shared-pools/probe-aggregation', null, { params: { limit } })
  return data
}

export type SharedPoolProbeHistoryQuery = {
  limit?: number
  account_id?: number
}

export async function listSharedPoolProbeHistories(id: number, query: SharedPoolProbeHistoryQuery = {}): Promise<SharedPoolProbeHistory[]> {
  const { data } = await apiClient.get<BizListResponse<SharedPoolProbeHistory>>(`/biz/pools/${id}/probe-histories`, {
    params: { limit: query.limit ?? 20 }
  })
  return data?.items ?? []
}

export async function listOwnerSharedPoolProbeHistories(id: number, query: SharedPoolProbeHistoryQuery = {}): Promise<SharedPoolProbeHistory[]> {
  const { data } = await apiClient.get<BizListResponse<SharedPoolProbeHistory>>(`/biz/owner/pools/${id}/probe-histories`, {
    params: {
      limit: query.limit ?? 20,
      ...(query.account_id ? { account_id: query.account_id } : {})
    }
  })
  return data?.items ?? []
}

export async function listSharedPoolAccountProbeHistories(poolId: number, accountId: number, query: SharedPoolProbeHistoryQuery = {}): Promise<SharedPoolProbeHistory[]> {
  const { data } = await apiClient.get<BizListResponse<SharedPoolProbeHistory>>(`/biz/pools/${poolId}/accounts/${accountId}/probe-histories`, {
    params: { limit: query.limit ?? 20 }
  })
  return data?.items ?? []
}

export async function adminListSharedPoolProbeHistories(id: number, query: SharedPoolProbeHistoryQuery = {}): Promise<SharedPoolProbeHistory[]> {
  const { data } = await apiClient.get<BizListResponse<SharedPoolProbeHistory>>(`/admin/biz/shared-pools/${id}/probe-histories`, {
    params: {
      limit: query.limit ?? 50,
      ...(query.account_id ? { account_id: query.account_id } : {})
    }
  })
  return data?.items ?? []
}

export async function adminListSharedPoolGovernanceLogs(id: number, limit = 50): Promise<SharedPoolGovernanceLog[]> {
  const { data } = await apiClient.get<BizListResponse<SharedPoolGovernanceLog>>(`/admin/biz/shared-pools/${id}/governance-logs`, { params: { limit } })
  return data?.items ?? []
}

export async function adminListCommunityPosts(query: CommunityPostQuery = {}): Promise<CommunityPost[]> {
  const { data } = await apiClient.get<BizListResponse<CommunityPost>>('/admin/biz/community/posts', { params: query })
  return data?.items ?? []
}

// ==================== Tavern scripts and rooms ====================

export type TavernScriptStatus = 'draft' | 'pending' | 'listed' | 'rejected' | 'archived' | 'delisted'
export type TavernRoomStatus = 'draft' | 'lobby' | 'running' | 'paused' | 'completed' | 'cancelled'
export type TavernGenre = 'mystery' | 'sci_fi' | 'fantasy' | 'horror' | 'workplace' | 'historical' | 'open_world' | 'other'
export type TavernDifficulty = 'easy' | 'normal' | 'hard' | 'expert'
export type TavernPricingMode = 'free' | 'credit' | 'balance' | 'hybrid'
export type TavernVisibility = 'public' | 'private'
export type TavernHostMode = 'ai_host' | 'human_host' | 'mixed'

export type TavernScript = {
  entry_price_usd?: string
  id: number
  user_id: number
  author: string
  title: string
  slug: string
  summary: string
  description: string
  genre: TavernGenre | string
  status: TavernScriptStatus | string
  visibility: TavernVisibility | string
  player_min: number
  player_max: number
  estimated_minutes: number
  difficulty: TavernDifficulty | string
  tags: string[]
  npc_cards: string[]
  host_brief: string
  opening_prompt: string
  safety_notes: string
  pricing_mode: TavernPricingMode | string
  entry_credit_cost: number
  entry_balance_cost: number
  author_revenue_share: number
  quality_score: number
  review_note: string
  reviewed_by?: number | null
  reviewed_at?: string | null
  published_at?: string | null
  archived_at?: string | null
  created_at: string
  updated_at: string
}

export type TavernRoom = {
  ticket_price_usd?: string
  id: number
  script_id: number
  package_id?: number | null
  package_version?: string
  package_runtime_kind?: string
  package_status?: string
  owner_id: number
  owner: string
  script_title: string
  title: string
  status: TavernRoomStatus | string
  visibility: TavernVisibility | string
  host_mode: TavernHostMode | string
  billing_mode: TavernPricingMode | string
  entry_credit_cost: number
  entry_balance_cost: number
  max_players: number
  current_players: number
  current_user_joined: boolean
  current_phase: string
  room_config: Record<string, unknown>
  started_at?: string | null
  ended_at?: string | null
  created_at: string
  updated_at: string
}

export type TavernRuntimeConfig = {
  runtime_id: string
  room: TavernRoom
  script: TavernScript
  package?: TavernGamePackage
  participant: {
    user_id: number
    role: string
  }
  bridge: {
    provider: string
    mode: string
    config_version: string
    protocol_version: string
    sandbox_mode: string
    launch_url: string
    external_runtime_isolated: boolean
  }
  gateway: {
    source: string
    model_strategy: string
    proxy_path: string
  }
  budget: {
    billing_mode: string
    entry_credit_cost: number
    entry_balance_cost: number
    turn_budget: number
  }
  prompts: {
    host_brief: string
    opening_prompt: string
    safety_notes: string
    npc_cards: string[]
  }
}

export type TavernGamePackageStatus = 'draft' | 'published' | 'revoked'
export type TavernGamePackageRuntimeKind = 'declarative'
export type TavernGamePackageEntryKind = 'prompt_flow' | 'scene_graph'

export type TavernGamePackagePermissions = {
  ai_gateway: boolean
  save: boolean
  score: boolean
  purchases: boolean
  presence: boolean
}

export type TavernGamePackageLimits = {
  max_turns: number
  max_scenes: number
}

export type TavernGamePackageManifest = {
  schema_version: 'tavern.package.v1' | string
  runtime_kind: TavernGamePackageRuntimeKind | string
  protocol_version: '2026-09-13.package.v1' | string
  entry: {
    kind: TavernGamePackageEntryKind | string
    ref: string
  }
  permissions: TavernGamePackagePermissions
  content: Record<string, unknown>
  limits: TavernGamePackageLimits
}

export type TavernGamePackage = {
  id: number
  script_id: number
  owner_user_id: number
  version: string
  schema_version: string
  runtime_kind: TavernGamePackageRuntimeKind | string
  protocol_version: string
  manifest: TavernGamePackageManifest
  package_digest?: string
  status: TavernGamePackageStatus | string
  published_at?: string | null
  created_at: string
  updated_at: string
}

export type TavernGamePackagePayload = {
  version: string
  manifest: TavernGamePackageManifest
}

export type TavernRuntimeSession = {
  id: number
  token?: string
  room_id: number
  user_id: number
  runtime_id: string
  status: string
  expires_at: string
  created_at: string
  updated_at: string
  config?: TavernRuntimeConfig
}

export type TavernRoomTurn = {
  id: number
  room_id: number
  author_user_id: number
  author_name: string
  author_role: 'owner' | 'player' | 'system' | string
  turn_index: number
  kind: 'player' | 'host' | 'system' | string
  client_message_id: string
  body: string
  created_at: string
}

export type TavernRoomTurnPayload = {
  client_message_id: string
  body: string
}

export type TavernScriptQuery = {
  keyword?: string
  genre?: TavernGenre | 'all' | string
  status?: TavernScriptStatus | 'all' | string
  sort?: 'latest' | 'popular' | 'updated' | 'quality' | string
  limit?: number
}

export type TavernScriptPayload = {
  title: string
  summary: string
  description: string
  genre: TavernGenre | string
  status: 'draft' | 'pending'
  visibility?: TavernVisibility | string
  player_min?: number
  player_max?: number
  estimated_minutes?: number
  difficulty?: TavernDifficulty | string
  tags?: string[]
  npc_cards?: string[]
  host_brief?: string
  opening_prompt?: string
  safety_notes?: string
  pricing_mode?: TavernPricingMode | string
  entry_credit_cost?: number
  entry_balance_cost?: number
  author_revenue_share?: number
}

export type TavernRoomPayload = {
  script_id: number
  title: string
  visibility?: TavernVisibility | string
  host_mode?: TavernHostMode | string
  billing_mode?: TavernPricingMode | string
  entry_credit_cost?: number
  entry_balance_cost?: number
  max_players?: number
  room_config?: Record<string, unknown>
}

export type TavernScriptReviewPayload = {
  status: 'listed' | 'rejected' | 'delisted'
  review_note?: string
  quality_score?: number
}

function tavernScriptParams(query: TavernScriptQuery = {}) {
  const params: Record<string, string | number> = {}
  if (query.keyword) params.keyword = query.keyword
  if (query.genre && query.genre !== 'all') params.genre = query.genre
  if (query.status && query.status !== 'all') params.status = query.status
  if (query.sort) params.sort = query.sort
  if (query.limit) params.limit = query.limit
  return params
}

export async function listTavernScripts(query: TavernScriptQuery = {}): Promise<TavernScript[]> {
  const { data } = await apiClient.get<BizListResponse<TavernScript>>('/biz/tavern/scripts', { params: tavernScriptParams(query) })
  return data?.items ?? []
}

export async function getTavernScript(id: number): Promise<TavernScript> {
  const { data } = await apiClient.get<TavernScript>(`/biz/tavern/scripts/${id}`)
  return data
}

export async function listMyTavernScripts(query: Pick<TavernScriptQuery, 'status' | 'limit'> = {}): Promise<TavernScript[]> {
  const { data } = await apiClient.get<BizListResponse<TavernScript>>('/biz/tavern/my-scripts', { params: tavernScriptParams(query) })
  return data?.items ?? []
}

export async function createTavernScript(payload: TavernScriptPayload): Promise<TavernScript> {
  const { data } = await apiClient.post<TavernScript>('/biz/tavern/scripts', payload)
  return data
}

export async function listTavernGamePackages(scriptID: number): Promise<TavernGamePackage[]> {
  const { data } = await apiClient.get<BizListResponse<TavernGamePackage>>(`/biz/tavern/scripts/${scriptID}/packages`)
  return data?.items ?? []
}

export async function createTavernGamePackage(scriptID: number, payload: TavernGamePackagePayload): Promise<TavernGamePackage> {
  const { data } = await apiClient.post<TavernGamePackage>(`/biz/tavern/scripts/${scriptID}/packages`, payload)
  return data
}

export async function publishTavernGamePackage(scriptID: number, version: string): Promise<TavernGamePackage> {
  const { data } = await apiClient.post<TavernGamePackage>(`/biz/tavern/scripts/${scriptID}/packages/${encodeURIComponent(version)}/publish`)
  return data
}

export async function revokeTavernGamePackage(scriptID: number, version: string): Promise<TavernGamePackage> {
  const { data } = await apiClient.post<TavernGamePackage>(`/biz/tavern/scripts/${scriptID}/packages/${encodeURIComponent(version)}/revoke`)
  return data
}

export async function listMyTavernRooms(query: { status?: TavernRoomStatus | 'all' | string; limit?: number } = {}): Promise<TavernRoom[]> {
  const params: Record<string, string | number> = {}
  if (query.status && query.status !== 'all') params.status = query.status
  if (query.limit) params.limit = query.limit
  const { data } = await apiClient.get<BizListResponse<TavernRoom>>('/biz/tavern/my-rooms', { params })
  return data?.items ?? []
}

export async function createTavernRoom(payload: TavernRoomPayload): Promise<TavernRoom> {
  const { data } = await apiClient.post<TavernRoom>('/biz/tavern/rooms', payload)
  return data
}

export async function openTavernRoom(id: number): Promise<TavernRoom> {
  const { data } = await apiClient.post<TavernRoom>(`/biz/tavern/rooms/${id}/open`)
  return data
}

export async function joinTavernRoom(id: number): Promise<TavernRoom> {
  const { data } = await apiClient.post<TavernRoom>(`/biz/tavern/rooms/${id}/join`)
  return data
}

export async function getTavernRoomRuntime(id: number): Promise<TavernRuntimeConfig> {
  const { data } = await apiClient.get<TavernRuntimeConfig>(`/biz/tavern/rooms/${id}/runtime`)
  return data
}

export async function createTavernRuntimeSession(id: number): Promise<TavernRuntimeSession> {
  const { data } = await apiClient.post<TavernRuntimeSession>(`/biz/tavern/rooms/${id}/runtime-session`)
  return data
}

export async function getTavernRuntimeSession(token: string): Promise<TavernRuntimeSession> {
  const { data } = await apiClient.post<TavernRuntimeSession>('/biz/tavern/runtime/session', { token })
  return data
}

export async function listTavernRoomTurns(id: number, after = 0, limit = 100): Promise<TavernRoomTurn[]> {
  const { data } = await apiClient.get<BizListResponse<TavernRoomTurn>>(`/biz/tavern/rooms/${id}/turns`, { params: { after, limit } })
  return data?.items ?? []
}

export async function appendTavernRoomTurn(id: number, payload: TavernRoomTurnPayload): Promise<TavernRoomTurn> {
  const { data } = await apiClient.post<TavernRoomTurn>(`/biz/tavern/rooms/${id}/turns`, payload)
  return data
}

export async function startTavernRoom(id: number): Promise<TavernRoom> {
  const { data } = await apiClient.post<TavernRoom>(`/biz/tavern/rooms/${id}/start`)
  return data
}

export async function completeTavernRoom(id: number): Promise<TavernRoom> {
  const { data } = await apiClient.post<TavernRoom>(`/biz/tavern/rooms/${id}/complete`)
  return data
}

export async function cancelTavernRoom(id: number): Promise<TavernRoom> {
  const { data } = await apiClient.post<TavernRoom>(`/biz/tavern/rooms/${id}/cancel`)
  return data
}

export async function adminListTavernScripts(query: TavernScriptQuery = {}): Promise<TavernScript[]> {
  const { data } = await apiClient.get<BizListResponse<TavernScript>>('/admin/biz/tavern/scripts', { params: tavernScriptParams(query) })
  return data?.items ?? []
}

export async function adminReviewTavernScript(id: number, payload: TavernScriptReviewPayload): Promise<TavernScript> {
  const { data } = await apiClient.patch<TavernScript>(`/admin/biz/tavern/scripts/${id}/review`, payload)
  return data
}

// ==================== Capability assets ====================

export type CapabilityAssetStatus = 'draft' | 'pending' | 'listed' | 'rejected' | 'archived' | 'delisted'

export type CapabilityAssetType =
  | 'product_app'
  | 'game'
  | 'workflow'
  | 'agent'
  | 'api_tool'
  | 'plugin_template'
  | 'prompt_solution'
  | 'dataset_report'
  | 'other'

export type CapabilityAssetPricingType = 'free' | 'paid' | 'contact' | 'open_source'

export type CapabilityAssetActionType =
  | 'visit_product'
  | 'open_demo'
  | 'view_workflow'
  | 'open_agent'
  | 'view_docs'
  | 'copy_prompt'
  | 'view_report'
  | 'download_template'
  | 'contact_author'
  | 'view_detail'

export type CapabilityAsset = {
  id: number
  user_id: number
  author: string
  title: string
  slug: string
  summary: string
  description: string
  asset_type: CapabilityAssetType | string
  status: CapabilityAssetStatus | string
  tags: string[]
  scenario_tags: string[]
  integration_tags: string[]
  cover_url: string
  screenshot_urls: string[]
  video_url: string
  demo_url: string
  doc_url: string
  source_url: string
  template_url: string
  primary_action_type: CapabilityAssetActionType | string
  pricing_type: CapabilityAssetPricingType | string
  contact_enabled: boolean
  is_featured: boolean
  featured_weight: number
  view_count: number
  like_count: number
  favorite_count: number
  download_count: number
  use_count: number
  liked_by_me: boolean
  favorited_by_me: boolean
  downloaded_by_me: boolean
  viewed_today: boolean
  comment_count: number
  rating_avg: number
  rating_count: number
  review_note: string
  reviewed_by?: number | null
  reviewed_at?: string | null
  published_at?: string | null
  archived_at?: string | null
  created_at: string
  updated_at: string
}

export type CapabilityAssetQuery = {
  keyword?: string
  asset_type?: CapabilityAssetType | 'all' | string
  status?: CapabilityAssetStatus | 'all' | string
  featured?: boolean
  sort?: 'featured' | 'latest' | 'popular' | 'updated' | string
  limit?: number
}

export type CapabilityAssetPayload = {
  title: string
  summary: string
  description: string
  asset_type: CapabilityAssetType | string
  status: 'draft' | 'pending'
  tags: string[]
  scenario_tags: string[]
  integration_tags: string[]
  cover_url?: string
  screenshot_urls?: string[]
  video_url?: string
  demo_url?: string
  doc_url?: string
  source_url?: string
  template_url?: string
  primary_action_type?: CapabilityAssetActionType | string
  pricing_type?: CapabilityAssetPricingType | string
  contact_enabled?: boolean
}

export type CapabilityAssetReviewPayload = {
  status: 'listed' | 'rejected' | 'delisted'
  review_note?: string
  is_featured?: boolean
  featured_weight?: number
}

export type CapabilityAssetVersionStatus = 'draft' | 'ready' | 'published' | 'revoked'

export type CapabilityAssetVersion = {
  id: number
  asset_id: number
  version: string
  runtime_kind: string
  manifest: Record<string, unknown>
  package_digest?: string
  status: CapabilityAssetVersionStatus | string
  file_count: number
  total_bytes: number
  published_at?: string | null
  created_at: string
  updated_at: string
}

export type CapabilityAssetFile = {
  path: string
  content_type: string
  byte_size: number
  sha256: string
}

export type CreateCapabilityAssetVersionPayload = {
  version: string
  runtime_kind: string
  manifest: Record<string, unknown>
}

export type CapabilityAssetFilePayload = {
  path: string
  content_type: string
  content_base64: string
}

export type CapabilityAssetStats = {
  asset_id: number
  title: string
  status: CapabilityAssetStatus | string
  view_count: number
  like_count: number
  favorite_count: number
  download_count: number
  use_count: number
  unique_viewers: number
  unique_downloaders: number
  unique_users: number
  revenue_connected: boolean
  last_download_at?: string | null
  last_use_at?: string | null
}

function capabilityAssetParams(query: CapabilityAssetQuery = {}) {
  const params: Record<string, string | number | boolean> = {}
  if (query.keyword) params.keyword = query.keyword
  if (query.asset_type && query.asset_type !== 'all') params.asset_type = query.asset_type
  if (query.status && query.status !== 'all') params.status = query.status
  if (query.featured) params.featured = true
  if (query.sort) params.sort = query.sort
  if (query.limit) params.limit = query.limit
  return params
}

export async function listCapabilityAssets(query: CapabilityAssetQuery = {}): Promise<CapabilityAsset[]> {
  const { data } = await apiClient.get<BizListResponse<CapabilityAsset>>('/biz/assets', { params: capabilityAssetParams(query) })
  return data?.items ?? []
}

export async function getCapabilityAsset(id: number): Promise<CapabilityAsset> {
  const { data } = await apiClient.get<CapabilityAsset>(`/biz/assets/${id}`)
  return data
}

export async function listMyCapabilityAssets(query: Pick<CapabilityAssetQuery, 'status' | 'limit'> = {}): Promise<CapabilityAsset[]> {
  const { data } = await apiClient.get<BizListResponse<CapabilityAsset>>('/biz/my/assets', { params: capabilityAssetParams(query) })
  return data?.items ?? []
}

export async function getMyCapabilityAssetStats(): Promise<CapabilityAssetStats[]> {
  const { data } = await apiClient.get<BizListResponse<CapabilityAssetStats>>('/biz/my/assets/stats')
  return data?.items ?? []
}

export async function setCapabilityAssetLike(id: number, liked: boolean): Promise<CapabilityAsset> {
  const request = liked
    ? apiClient.post<CapabilityAsset>(`/biz/assets/${id}/like`)
    : apiClient.delete<CapabilityAsset>(`/biz/assets/${id}/like`)
  const { data } = await request
  return data
}

export async function setCapabilityAssetFavorite(id: number, favorited: boolean): Promise<CapabilityAsset> {
  const request = favorited
    ? apiClient.post<CapabilityAsset>(`/biz/assets/${id}/favorite`)
    : apiClient.delete<CapabilityAsset>(`/biz/assets/${id}/favorite`)
  const { data } = await request
  return data
}

export async function createCapabilityAsset(payload: CapabilityAssetPayload): Promise<CapabilityAsset> {
  const { data } = await apiClient.post<CapabilityAsset>('/biz/assets', payload)
  return data
}

export async function updateCapabilityAsset(id: number, payload: CapabilityAssetPayload): Promise<CapabilityAsset> {
  const { data } = await apiClient.patch<CapabilityAsset>(`/biz/assets/${id}`, payload)
  return data
}

export async function adminListCapabilityAssets(query: CapabilityAssetQuery = {}): Promise<CapabilityAsset[]> {
  const { data } = await apiClient.get<BizListResponse<CapabilityAsset>>('/admin/biz/assets', { params: capabilityAssetParams(query) })
  return data?.items ?? []
}

export async function adminReviewCapabilityAsset(id: number, payload: CapabilityAssetReviewPayload): Promise<CapabilityAsset> {
  const { data } = await apiClient.patch<CapabilityAsset>(`/admin/biz/assets/${id}/review`, payload)
  return data
}

export async function listCapabilityAssetVersions(assetID: number): Promise<CapabilityAssetVersion[]> {
  const { data } = await apiClient.get<BizListResponse<CapabilityAssetVersion>>(`/biz/assets/${assetID}/versions`)
  return data?.items ?? []
}

export async function createCapabilityAssetVersion(
  assetID: number,
  payload: CreateCapabilityAssetVersionPayload,
): Promise<CapabilityAssetVersion> {
  const { data } = await apiClient.post<CapabilityAssetVersion>(`/biz/assets/${assetID}/versions`, payload)
  return data
}

export async function putCapabilityAssetFile(
  assetID: number,
  version: string,
  payload: CapabilityAssetFilePayload,
): Promise<CapabilityAssetVersion> {
  const { data } = await apiClient.put<CapabilityAssetVersion>(
    `/biz/assets/${assetID}/versions/${encodeURIComponent(version)}/files`,
    payload,
    { timeout: 120000 },
  )
  return data
}

export async function finalizeCapabilityAssetVersion(assetID: number, version: string): Promise<CapabilityAssetVersion> {
  const { data } = await apiClient.post<CapabilityAssetVersion>(
    `/biz/assets/${assetID}/versions/${encodeURIComponent(version)}/finalize`,
  )
  return data
}

export async function publishCapabilityAssetVersion(assetID: number, version: string): Promise<CapabilityAssetVersion> {
  const { data } = await apiClient.post<CapabilityAssetVersion>(
    `/biz/assets/${assetID}/versions/${encodeURIComponent(version)}/publish`,
  )
  return data
}

export async function revokeCapabilityAssetVersion(assetID: number, version: string): Promise<CapabilityAssetVersion> {
  const { data } = await apiClient.post<CapabilityAssetVersion>(
    `/biz/assets/${assetID}/versions/${encodeURIComponent(version)}/revoke`,
  )
  return data
}

export async function downloadCapabilityAssetPackage(assetID: number, version: string): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(
    `/biz/assets/${assetID}/versions/${encodeURIComponent(version)}/download`,
    { responseType: 'blob', timeout: 120000 },
  )
  return data
}

// GET /biz/pools/:id — single pool detail.
export async function getSharedPool(id: number): Promise<SharedPool> {
  const { data } = await apiClient.get<SharedPool>(`/biz/pools/${id}`)
  return data
}

export async function listSharedPoolModelPricing(id: number): Promise<SharedPoolModelEndpointPricing[]> {
  const { data } = await apiClient.get<{ items: SharedPoolModelEndpointPricing[] }>(`/biz/pools/${id}/pricing`)
  return data?.items ?? []
}

export async function adminInspectSharedPoolPricing(id: number): Promise<SharedPoolModelEndpointPricing[]> {
  const { data } = await apiClient.get<{ items: SharedPoolModelEndpointPricing[] }>(`/admin/biz/shared-pools/${id}/pricing`)
  return data.items
}

export async function saveSharedPoolCustomPrice(id: number, payload: SaveSharedPoolCustomPricePayload): Promise<SharedPoolPriceQuote> {
  const { data } = await apiClient.put<{ price: SharedPoolPriceQuote }>(`/biz/pools/${id}/pricing`, payload)
  return data.price
}

// GET /biz/pools/:id/full-check-report — latest full-capability detection report.
export async function getSharedPoolFullCheckReport(id: number): Promise<SharedPoolFullCheckReport> {
  const { data } = await apiClient.get<SharedPoolFullCheckReport>(`/biz/pools/${id}/full-check-report`)
  return data
}

export async function createSharedPool(payload: CreateSharedPoolPayload): Promise<SharedPool> {
  const { data } = await apiClient.post<SharedPool>('/biz/pools', payload)
  return data
}

export function createSharedPoolNativeOperationID(): string {
  if (typeof globalThis.crypto?.randomUUID === 'function') {
    return globalThis.crypto.randomUUID()
  }
  return `native-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export async function createNativeSharedPoolDraft(payload: CreateNativeSharedPoolDraftPayload): Promise<SharedPool> {
  const { data } = await apiClient.post<SharedPool>(
    '/biz/pools',
    {
      name: payload.name,
      ...(payload.description ? { description: payload.description } : {}),
      supply_mode: 'native',
      operation_id: payload.operation_id,
    },
    { headers: { 'Idempotency-Key': payload.operation_id } },
  )
  return data
}

export async function importSharedPools(items: SharedPoolImportItemPayload[]): Promise<SharedPoolImportResult> {
  const { data } = await apiClient.post<SharedPoolImportResult>('/biz/pool-imports', { items })
  return data
}

export async function updateSharedPool(id: number, payload: UpdateSharedPoolPayload): Promise<SharedPool> {
  const { data } = await apiClient.patch<SharedPool>(`/biz/pools/${id}`, payload)
  return data
}

export async function fetchSharedPoolUpstreamModels(payload: SharedPoolUpstreamModelsPayload): Promise<SharedPoolUpstreamModelsResult> {
  const { data } = await apiClient.post<SharedPoolUpstreamModelsResult>('/biz/upstream/models', payload)
  return data
}

export async function deleteSharedPool(id: number): Promise<void> {

  await apiClient.delete(`/biz/pools/${id}`)
}

export async function listMySharedPools(): Promise<SharedPool[]> {
  const { data } = await apiClient.get<{ pools: SharedPool[] }>('/biz/my-pools')
  return data?.pools ?? []
}

export async function listSharedPoolAccounts(poolId: number): Promise<SharedPoolAccount[]> {
  const { data } = await apiClient.get<{ accounts: SharedPoolAccount[] }>(`/biz/pools/${poolId}/accounts`)
  return data?.accounts ?? []
}

export async function createSharedPoolAccount(poolId: number, payload: SharedPoolAccountPayload): Promise<SharedPoolAccount> {
  const { data } = await apiClient.post<SharedPoolAccount>(`/biz/pools/${poolId}/accounts`, payload)
  return data
}

export async function createNativeSharedPoolAccount(
  poolId: number,
  payload: CreateNativeSharedPoolAccountPayload,
): Promise<SharedPoolAccount> {
  const { data } = await apiClient.post<SharedPoolAccount>(
    `/biz/pools/${poolId}/accounts`,
    {
      operation_id: payload.operation_id,
      name: payload.name,
      ...(payload.description ? { description: payload.description } : {}),
      provider: payload.provider,
      auth_type: payload.auth_type,
      upstream_base_url: payload.upstream_base_url,
      ...(payload.upstream_api_key ? { upstream_api_key: payload.upstream_api_key } : {}),
      ...(payload.credentials ? { credentials: payload.credentials } : {}),
    },
    { headers: { 'Idempotency-Key': payload.operation_id } },
  )
  return data
}

export async function importSharedPoolAccounts(poolId: number, items: SharedPoolAccountPayload[]): Promise<SharedPoolAccountImportResult> {
  const { data } = await apiClient.post<SharedPoolAccountImportResult>(`/biz/pools/${poolId}/accounts/import`, { items })
  return data
}

export async function importSharedPoolOAuthPackage(
  poolId: number,
  dataPackage: SharedPoolOAuthDataPackage,
  updateExisting = true,
): Promise<SharedPoolOAuthImportResult> {
  const { data } = await apiClient.post<SharedPoolOAuthImportResult>(`/biz/pools/${poolId}/accounts/import-oauth-package`, {
    data: dataPackage,
    update_existing: updateExisting,
  })
  return data
}

export async function updateSharedPoolAccount(poolId: number, accountId: number, payload: SharedPoolAccountPayload): Promise<SharedPoolAccount> {
  const { data } = await apiClient.patch<SharedPoolAccount>(`/biz/pools/${poolId}/accounts/${accountId}`, payload)
  return data
}

export async function repairSharedPoolNativeAccount(
  poolId: number,
  accountId: number,
  payload: RepairNativeSharedPoolAccountPayload,
): Promise<SharedPoolAccount> {
  const {
    repair_operation_id,
    expected_config_version,
    provider,
    auth_type,
    upstream_base_url,
    upstream_api_key,
    credentials,
  } = payload
  const { data } = await apiClient.patch<SharedPoolAccount>(
    `/biz/pools/${poolId}/accounts/${accountId}`,
    {
      repair_operation_id,
      expected_config_version,
      provider,
      auth_type,
      upstream_base_url,
      ...(upstream_api_key ? { upstream_api_key } : {}),
      ...(credentials ? { credentials } : {}),
    },
    { headers: { 'Idempotency-Key': repair_operation_id } },
  )
  return data
}

export async function verifySharedPoolNativeReadiness(
  poolId: number,
  accountId: number,
  payload: VerifyNativeSharedPoolReadinessPayload,
): Promise<SharedPoolAccount> {
  const { data } = await apiClient.post<SharedPoolAccount>(
    `/biz/pools/${poolId}/accounts/${accountId}/native-readiness`,
    { expected_config_version: payload.expected_config_version },
  )
  return data
}

export async function deleteSharedPoolAccount(poolId: number, accountId: number): Promise<void> {
  await apiClient.delete(`/biz/pools/${poolId}/accounts/${accountId}`)
}

export async function activateSharedPoolNativeBilling(
  poolId: number,
  payload: ActivateNativeSharedPoolBillingPayload,
): Promise<SharedPoolNativeActivation> {
  const { data } = await apiClient.post<SharedPoolNativeActivation>(
    `/biz/pools/${poolId}/native-activation`,
    { operation_id: payload.operation_id, expected_config_version: payload.expected_config_version },
    { headers: { 'Idempotency-Key': payload.operation_id } },
  )
  return data
}

export async function listMySharedPoolAccessKeys(): Promise<SharedPoolAccessKey[]> {
  const { data } = await apiClient.get<{ keys: SharedPoolAccessKey[] }>('/biz/share-keys')
  return data?.keys ?? []
}

export async function deleteSharedPoolAccessKey(apiKeyId: number): Promise<void> {
  await apiClient.delete(`/biz/share-keys/${apiKeyId}`)
}

export async function listMySharedPoolLedger(): Promise<SharedPoolLedgerView> {
  const { data } = await apiClient.get<SharedPoolLedgerView>('/biz/shared-pool/ledger')
  return {
    // Older responses predate the owner-wallet migration and omit `wallet`;
    // callers still need a readable zero so they never render `undefined`.
    wallet: data?.wallet ?? {
      owner_id: 0,
      available_amount: 0,
      pending_amount: 0,
      frozen_amount: 0,
      transferred_amount: 0,
      total_earned: 0,
      version: 0,
    },
    earnings: data?.earnings ?? [],
    activity: data?.activity ?? [],
    withdrawable: data?.withdrawable ?? [],
    legacy_withdrawable: data?.legacy_withdrawable ?? [],
    incentives: data?.incentives ?? []
  }
}

export async function transferSharedPoolOwnerEarnings(
  amount: number,
  operationId: string,
): Promise<SharedPoolOwnerWalletTransferResult> {
  const { data } = await apiClient.post<SharedPoolOwnerWalletTransferResult>(
    '/biz/shared-pool/earnings/transfer',
    { amount, operation_id: operationId },
    { headers: { 'Idempotency-Key': operationId } },
  )
  return data
}

export async function createSharedPoolAccessKey(id: number, name?: string): Promise<SharedPoolAccessKeyResult> {
  const { data } = await apiClient.post<SharedPoolAccessKeyResult>(`/biz/pools/${id}/access-key`, { name })
  return data
}

export async function reportSharedPool(id: number, reason?: string): Promise<SharedPool> {
  const { data } = await apiClient.post<SharedPool>(`/biz/pools/${id}/report`, { reason })
  return data
}

export async function likeSharedPool(id: number): Promise<SharedPool> {
  const { data } = await apiClient.post<SharedPool>(`/biz/pools/${id}/like`)
  return data
}

export async function unlikeSharedPool(id: number): Promise<SharedPool> {
  const { data } = await apiClient.delete<SharedPool>(`/biz/pools/${id}/like`)
  return data
}

// ==================== Pool seats (money-sensitive: join / leave / billing) ====================

export type PoolSeat = {
  id: number
  pool_id: number
  pool_name: string
  user_id: number
  status: string
  hourly_seat_fee: number
  hourly_min_usage_waiver?: number
  current_hour_usage_amount?: number
  current_hour_waiver_remaining?: number
  current_hour_waiver_met?: boolean
  joined_at: string
  last_charged_at: string
  last_activity_at?: string
  idle_release_at?: string
  released_at?: string
  release_reason?: string
  total_charged: number
}

export type JoinPoolResult = {
  seat: PoolSeat
  already_held: boolean
}

// GET /biz/seats — the current user's pool seats (active first).
export async function listMySeats(): Promise<PoolSeat[]> {
  const { data } = await apiClient.get<{ seats: PoolSeat[] }>('/biz/seats')
  return data?.seats ?? []
}

// POST /biz/pools/:id/join — take a seat in a pool (charges hourly seat fee).
export async function joinSharedPool(id: number): Promise<JoinPoolResult> {
  const { data } = await apiClient.post<JoinPoolResult>(`/biz/pools/${id}/join`)
  return data
}

// POST /biz/pools/:id/leave — release the current user's seat (settles due hours).
export async function leaveSharedPool(id: number): Promise<void> {
  await apiClient.post(`/biz/pools/${id}/leave`)
}

// GET /biz/pools/:id/members — owner-only member list for a shared pool.
export async function listSharedPoolMembers(id: number, status: 'active' | 'released' | 'all' = 'active'): Promise<PoolSeat[]> {
  const { data } = await apiClient.get<{ members: PoolSeat[] }>(`/biz/pools/${id}/members`, {
    params: { status },
  })
  return data?.members ?? []
}

// DELETE /biz/pools/:id/members/:seatId — owner-only member removal.
export async function removeSharedPoolMember(id: number, seatId: number): Promise<void> {
  await apiClient.delete(`/biz/pools/${id}/members/${seatId}`)
}

// PATCH /biz/pools/:id/card-skin — 池主设置收藏卡底色
export async function setPoolCardSkin(id: number, cardKey: string, cardRarity: string): Promise<void> {
  await apiClient.patch(`/biz/pools/${id}/card-skin`, { card_key: cardKey, card_rarity: cardRarity })
}

// DELETE /biz/pools/:id/card-skin — 池主清除收藏卡底色
export async function clearPoolCardSkin(id: number): Promise<void> {
  await apiClient.delete(`/biz/pools/${id}/card-skin`)
}

export async function setSharedPoolOwnerPause(id: number, paused: boolean, version: number): Promise<SharedPool> {
  const { data } = await apiClient.put<SharedPool>(`/biz/pools/${id}/owner-pause`, { paused, expected_config_version: version })
  return data
}
