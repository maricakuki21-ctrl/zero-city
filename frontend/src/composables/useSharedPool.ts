import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'
import {
  createSharedPoolAccessKey, joinSharedPool, leaveSharedPool,
  likeSharedPool, unlikeSharedPool,
  listModelCatalog, listMySeats, listMySharedPoolAccessKeys,
  listSharedPoolProbeHistories, listSharedPools, reportSharedPool,
  type ModelCatalogEntry, type SharedPool as ApiSharedPool,
  type SharedPoolAccessKey, type SharedPoolAccountSummary as ApiSharedPoolAccountSummary,
  type SharedPoolListView, type SharedPoolQuery, type PoolSeat,
  type SharedPoolProbeHistory,
  type SharedMarketPoolProjection,
} from '@/api/bizdecipher'
import { communityAPI, type SharedPoolCommunitySummary } from '@/api/community'
import { extractActionableApiErrorMessage } from '@/utils/apiError'

// ── Types ──────────────────────────────────────────────────────────────────

export type PoolStatus = 'healthy' | 'limited' | 'offline' | 'maintenance' | 'unknown'

export interface PoolVM {
  id: number
  mark: string
  name: string
  owner: string
  ownerId?: number
  ownerCardAsset: NonNullable<ApiSharedPool['owner_card_asset']>
  cardSkinKey: string
  cardSkinRarity: string
  tier: string
  status: PoolStatus
  models: string[]
  todayAvailability: number
  sevenDayAvailability: number
  rate: number
  currentUsers: number
  maxUsers: number
  minBalance: number
  hourlyFee: number
  hourlyMinUsageWaiver: number
  latency: number
  complaints: number
  likes: number
  likedByMe: boolean
  successRate: string
  throughputRPM?: string
  rateDecimal?: string
  sourceFreshness?: string
  sourceConfidence?: string
  avatarUrl: string
  statusNote: string
  disabledReason: string
  governanceStatus: string
  governanceNote: string
  observation: boolean
  lastProbeAt: string
  lastProbeSuccess?: boolean
  lastProbeErrorType: string
  lastProbeErrorMessage: string
  consecutiveProbeFailures: number
  lastSuccessfulProbeAt: string
  lastProbeCheckLevel: string
  lastProbeGateRequired: boolean
  lastProbeGatePassed: boolean
  lastProbeFullCheckPassed: number
  lastProbeFullCheckTotal: number
  lastProbeFullCheckScore: number
  verificationMode: string
  verificationExemptionReason: string
  verificationBadgeLabel: string
  verificationTrusted: boolean
  accountSummary: ApiSharedPoolAccountSummary
  modelConfigs: NonNullable<ApiSharedPool['model_configs']>
}

// ── Utility helpers ────────────────────────────────────────────────────────

export function emptyAccountSummary(): ApiSharedPoolAccountSummary {
  return {
    total_accounts: 0, configured_accounts: 0, schedulable_accounts: 0,
    gate_blocked_accounts: 0, disabled_accounts: 0, total_account_concurrency: 0,
    total_user_concurrency: 0, total_rpm_limit: 0, average_full_check_score: 0,
    full_check_passed_accounts: 0, full_check_total_accounts: 0,
    total_calls: 0, successful_calls: 0, failed_calls: 0,
    success_rate: 0, model_coverage: [], last_probe_at: null
  }
}

export function normalizeAccountSummary(summary?: ApiSharedPoolAccountSummary): ApiSharedPoolAccountSummary {
  const base = emptyAccountSummary()
  if (!summary) return base
  return { ...base, ...summary, model_coverage: Array.isArray(summary.model_coverage) ? summary.model_coverage : [] }
}

export function emptyOwnerCardAsset(): NonNullable<ApiSharedPool['owner_card_asset']> {
  return {
    collectible_count: 0, highest_rarity: '', featured_card_key: '',
    featured_card_rarity: '', featured_card_serial_no: null,
    featured_card_edition_no: null, featured_card_supply: null,
    profile_background_ready: false
  }
}

export function normalizeOwnerCardAsset(
  asset?: ApiSharedPool['owner_card_asset']
): NonNullable<ApiSharedPool['owner_card_asset']> {
  return { ...emptyOwnerCardAsset(), ...(asset || {}) }
}

export function emptyPoolCommunitySummary(poolId: number): SharedPoolCommunitySummary {
  return {
    pool_id: poolId, total_posts: 0, discussion_posts: 0, feedback_posts: 0,
    incident_posts: 0, risk_signals: 0, last_post_title: '', latest_posts: []
  }
}

function runtimeNumber(value: string | undefined): number {
  if (!value?.trim()) return Number.NaN
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : Number.NaN
}

function poolStatus(value: string | undefined): PoolStatus {
  return value === 'healthy' || value === 'limited' || value === 'offline' || value === 'maintenance'
    ? value
    : 'unknown'
}

export function toVM(p: ApiSharedPool, candidate?: SharedMarketPoolProjection): PoolVM {
  const projection = candidate?.pool_id === p.id ? candidate : undefined
  const product = projection?.product
  const runtime = projection?.runtime
  const hasLegacyRuntimeEvidence = !projection && Boolean(p.account_summary || p.last_probe_at)
  const name = product?.name.value ?? p.name
  const canonicalSuccessRate = p.account_summary?.success_rate
  const verificationMode = String(p.verification_mode || '').trim() || 'full_check'
  const verificationExemptionReason = String(p.verification_exemption_reason || '').trim()
  const verificationTrusted = verificationMode === 'full_check'
    ? Boolean(p.last_probe_gate_passed) && Number(p.last_probe_full_check_score || 0) >= 70 && Number(p.last_probe_full_check_total || 0) > 0
    : verificationMode === 'professional_review'
  const verificationBadgeLabel = verificationMode === 'professional_review'
    ? '专业核验'
    : verificationTrusted ? '满血验证' : '待满血验证'
  return {
    id: p.id,
    mark: (name || '?').trim().charAt(0).toUpperCase() || '?',
    name,
    owner: product?.owner_label.value ?? p.owner_label,
    ownerId: p.owner_id,
    ownerCardAsset: normalizeOwnerCardAsset(p.owner_card_asset),
    cardSkinKey: String(p.card_skin_key || '').trim(),
    cardSkinRarity: String(p.card_skin_rarity || '').trim(),
    tier: p.tier,
    status: projection
      ? projection.stale || !runtime || runtime.health.value.confidence === 'unknown' ? 'unknown' : poolStatus(runtime.health.value.state)
      : hasLegacyRuntimeEvidence ? poolStatus(p.status) : 'unknown',
    models: projection ? (runtime?.models.value.filter(model => model.executable).map(model => model.name) ?? []) : p.models || [],
    todayAvailability: projection ? runtimeNumber(runtime?.today_availability_percent.value) : hasLegacyRuntimeEvidence ? p.today_availability : Number.NaN,
    sevenDayAvailability: projection ? runtimeNumber(runtime?.seven_day_availability_percent.value) : hasLegacyRuntimeEvidence ? p.seven_day_availability : Number.NaN,
    rate: product ? runtimeNumber(product.pricing.rate_multiplier.value) : p.rate_multiplier,
    currentUsers: product?.membership.current_users.value ?? p.current_users,
    maxUsers: product?.membership.maximum_users.value ?? p.max_users,
    minBalance: product ? runtimeNumber(product.pricing.minimum_balance.value) : p.min_balance_admission,
    hourlyFee: product ? runtimeNumber(product.pricing.hourly_seat_fee.value) : p.hourly_seat_fee,
    hourlyMinUsageWaiver: product ? runtimeNumber(product.pricing.hourly_usage_waiver.value) : p.hourly_min_usage_waiver || 0,
    latency: projection ? runtimeNumber(runtime?.latency_ms.value) : hasLegacyRuntimeEvidence ? p.avg_latency_ms : Number.NaN,
    complaints: product?.community.complaints.value ?? p.complaint_count ?? 0,
    likes: product?.community.likes.value ?? p.like_count ?? 0,
    likedByMe: Boolean(p.liked_by_me),
    successRate: projection ? runtime?.success_rate_percent.value || '-' : Number.isFinite(canonicalSuccessRate) ? Number(canonicalSuccessRate).toFixed(1) : '-',
    throughputRPM: runtime?.throughput_rpm.value || '-',
    rateDecimal: product?.pricing.rate_multiplier.value ?? String(p.rate_multiplier),
    sourceFreshness: projection?.stale ? 'last_verified' : runtime?.health.evidence.freshness ?? 'last_verified',
    sourceConfidence: runtime?.health.value.confidence ?? (hasLegacyRuntimeEvidence ? 'medium' : 'unknown'),
    avatarUrl: p.avatar_url || '',
    statusNote: p.status_note || '',
    disabledReason: p.disabled_reason || '',
    governanceStatus: String(p.governance_status || 'normal'),
    governanceNote: String(p.governance_note || ''),
    observation: String(p.governance_status || '') === 'watch' && Number(p.consecutive_probe_failures || 0) >= 3,
    lastProbeAt: p.last_probe_at || '',
    lastProbeSuccess: p.last_probe_success,
    lastProbeErrorType: p.last_probe_error_type || '',
    lastProbeErrorMessage: p.last_probe_error_message || '',
    consecutiveProbeFailures: p.consecutive_probe_failures || 0,
    lastSuccessfulProbeAt: p.last_successful_probe_at || '',
    lastProbeCheckLevel: p.last_probe_check_level || '',
    lastProbeGateRequired: Boolean(p.last_probe_gate_required),
    lastProbeGatePassed: Boolean(p.last_probe_gate_passed),
    lastProbeFullCheckPassed: Number(p.last_probe_full_check_passed || 0),
    lastProbeFullCheckTotal: Number(p.last_probe_full_check_total || 0),
    lastProbeFullCheckScore: Number(p.last_probe_full_check_score || 0),
    verificationMode,
    verificationExemptionReason,
    verificationBadgeLabel,
    verificationTrusted,
    accountSummary: normalizeAccountSummary(p.account_summary),
    modelConfigs: p.model_configs || []
  }
}

// ── Composable ─────────────────────────────────────────────────────────────

export function useSharedPool() {
  const { t } = useI18n()
  const appStore = useAppStore()
  const { copyToClipboard } = useClipboard()

  // ── State ────────────────────────────────────────────────────────────────

  const keyword = ref('')
  const selectedStatus = ref('all')
  const selectedModel = ref('all')
  const selectedAvailability = ref('all')
  const marketView = ref<'public' | 'observation'>('public')
  const sortBy = ref('recommended')
  const loading = ref(false)
  const loadError = ref('')
  const pools = ref<PoolVM[]>([])
  const modelCatalog = ref<ModelCatalogEntry[]>([])
  const serverStats = ref({ total: 0, online: 0, limited: 0, avgAvailability: 0 })
  const joinedPoolIds = ref<Set<number>>(new Set())
  const mySeats = ref<PoolSeat[]>([])
  const actingPoolId = ref<number | null>(null)
  const reportingPoolId = ref<number | null>(null)
  const myAccessKeys = ref<SharedPoolAccessKey[]>([])
  const latestGeneratedKey = ref<SharedPoolAccessKey | null>(null)
  const creatingAccessKey = ref(false)
  const pricingDrawerPool = ref<PoolVM | null>(null)
  const expandedMarketDetailPoolId = ref<number | null>(null)
  const expandedMarketProbePoolId = ref<number | null>(null)
  const probeHistories = reactive<Record<number, SharedPoolProbeHistory[]>>({})
  const poolCommunitySummaries = reactive<Record<number, SharedPoolCommunitySummary>>({})

  // ── Internal helpers ─────────────────────────────────────────────────────

  function accountModelCoverage(pool: PoolVM): string[] {
    const summary = pool.accountSummary || emptyAccountSummary()
    return summary.model_coverage?.length ? summary.model_coverage : pool.models || []
  }

  // ── Computed ─────────────────────────────────────────────────────────────

  const filteredPools = computed(() => {
    let result = [...pools.value]
    if (keyword.value) {
      const kw = keyword.value.toLowerCase()
      result = result.filter(p =>
        p.name.toLowerCase().includes(kw) ||
        p.owner.toLowerCase().includes(kw) ||
        accountModelCoverage(p).some(m => m.toLowerCase().includes(kw))
      )
    }
    if (selectedModel.value !== 'all') {
      const model = selectedModel.value.toLowerCase()
      result = result.filter(p => accountModelCoverage(p).some(m => m.toLowerCase().includes(model)))
    }
    if (selectedStatus.value !== 'all') {
      result = result.filter(p => p.status === selectedStatus.value)
    }
    if (selectedAvailability.value !== 'all') {
      const min = parseInt(selectedAvailability.value)
      result = result.filter(p => p.todayAvailability >= min)
    }
    switch (sortBy.value) {
      case 'availability': result.sort((a, b) => b.todayAvailability - a.todayAvailability); break
      case 'rate':         result.sort((a, b) => a.rate - b.rate); break
      case 'latency':      result.sort((a, b) => a.latency - b.latency); break
      case 'users':        result.sort((a, b) => b.currentUsers - a.currentUsers); break
      case 'newest':       result.sort((a, b) => b.id - a.id); break
    }
    return result
  })

  const stats = computed(() => ({
    total: serverStats.value.total,
    online: serverStats.value.online,
    limited: serverStats.value.limited,
    avgAvailability: serverStats.value.avgAvailability.toFixed(1)
  }))

  // ── Data loading ──────────────────────────────────────────────────────────

  async function loadModelCatalog(): Promise<void> {
    try { modelCatalog.value = await listModelCatalog() }
    catch { modelCatalog.value = [] }
  }

  async function loadPoolCommunitySummaries(poolIds: number[], request = poolRequestGeneration): Promise<void> {
    try {
      const response = await communityAPI.listSharedPoolSummaries(poolIds, 3)
      if (request !== poolRequestGeneration) return
      const received = new Set<number>()
      for (const summary of response.items || []) {
        poolCommunitySummaries[summary.pool_id] = summary
        received.add(summary.pool_id)
      }
      for (const poolId of poolIds) {
        if (!received.has(poolId)) poolCommunitySummaries[poolId] = emptyPoolCommunitySummary(poolId)
      }
    } catch {
      if (request !== poolRequestGeneration) return
      for (const poolId of poolIds) poolCommunitySummaries[poolId] = emptyPoolCommunitySummary(poolId)
    }
  }

  let poolRequestGeneration = 0
  async function loadPools(): Promise<void> {
    const request = ++poolRequestGeneration
    loading.value = true
    loadError.value = ''
    try {
      const query: SharedPoolQuery = {
        keyword: keyword.value.trim() || undefined,
        model: selectedModel.value,
        status: selectedStatus.value,
        view: marketView.value,
        min_availability: selectedAvailability.value === 'all' ? undefined : Number(selectedAvailability.value),
        sort: sortBy.value,
        limit: 100
      }
      const view: SharedPoolListView = await listSharedPools(query)
      if (request !== poolRequestGeneration) return
      const projections = new Map((view.projections || []).map(projection => [projection.pool_id, projection]))
      pools.value = (view.pools || []).map(pool => toVM(pool, projections.get(pool.id)))
      serverStats.value = {
        total: view.total, online: view.online,
        limited: view.limited, avgAvailability: view.avg_availability
      }
      await loadPoolCommunitySummaries(pools.value.map(p => p.id), request)
    } catch (error) {
      if (request === poolRequestGeneration) loadError.value = extractActionableApiErrorMessage(error, '共享池市场加载失败，请重试。')
    } finally {
      if (request === poolRequestGeneration) loading.value = false
    }
  }

  async function loadMySeats(): Promise<void> {
    try {
      const seats = await listMySeats()
      mySeats.value = seats
      joinedPoolIds.value = new Set(seats.filter(s => s.status === 'active').map(s => s.pool_id))
    } catch {
      mySeats.value = []
      joinedPoolIds.value = new Set()
    }
  }

  async function loadMyAccessKeys(): Promise<void> {
    try { myAccessKeys.value = await listMySharedPoolAccessKeys() }
    catch { myAccessKeys.value = [] }
  }

  // ── Pool actions ──────────────────────────────────────────────────────────

  async function joinPool(pool: PoolVM): Promise<void> {
    if (actingPoolId.value !== null) return
    const fee = Number(pool.hourlyFee || 0)
    const waiver = Number(pool.hourlyMinUsageWaiver || 0)
    const feeText = fee > 0 ? `${fee.toFixed(4)}/小时` : '免费'
    const waiverText = waiver > 0 ? `；当小时 API 消费达到 ${waiver.toFixed(4)} 可全额抵扣席位费` : ''
    const ok = window.confirm(
      `确认加入「${pool.name}」？

` +
      `1) 席位费按整小时结算：${feeText}${waiverText}
` +
      `2) 加入后至少满 1 小时才会结算一笔席位费；未调用 API 也可能产生席位费
` +
      `3) 席位费与共享池 API 扣费记在「共享市场 → 我的共享池 → 消费账本」，不会出现在「使用记录」页
` +
      `4) 连续 2 小时无真实 API 调用，系统会自动释放席位
` +
      `5) 若不想继续占用席位，请及时退出，避免空占扣费`
    )
    if (!ok) return
    actingPoolId.value = pool.id
    try {
      const res = await joinSharedPool(pool.id)
      joinedPoolIds.value = new Set(joinedPoolIds.value).add(pool.id)
      appStore.showSuccess(res.already_held ? t('accountSquare.alreadyJoined') : t('accountSquare.joinSuccess'))
      await Promise.all([loadPools(), loadMySeats()])
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message
      appStore.showError(msg || t('accountSquare.joinFailed'))
    } finally {
      actingPoolId.value = null
    }
  }

  async function leavePool(pool: PoolVM): Promise<void> {
    if (actingPoolId.value !== null) return
    actingPoolId.value = pool.id
    try {
      await leaveSharedPool(pool.id)
      const next = new Set(joinedPoolIds.value)
      next.delete(pool.id)
      joinedPoolIds.value = next
      appStore.showSuccess(t('accountSquare.leaveSuccess'))
      await Promise.all([loadPools(), loadMySeats()])
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message
      appStore.showError(msg || t('accountSquare.leaveFailed'))
    } finally {
      actingPoolId.value = null
    }
  }

  async function createKeyFromPool(pool: PoolVM): Promise<void> {
    if (creatingAccessKey.value) return
    if (!joinedPoolIds.value.has(pool.id)) {
      appStore.showError(t('accountSquare.joinRequiredForKey'))
      return
    }
    creatingAccessKey.value = true
    try {
      const result = await createSharedPoolAccessKey(pool.id, `${pool.name} ${t('accountSquare.dedicatedKeySuffix')}`)
      latestGeneratedKey.value = result.access_key
      myAccessKeys.value = [result.access_key, ...myAccessKeys.value]
      appStore.showSuccess(t('accountSquare.keyCreatedOneTime'))
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message
      appStore.showError(msg || t('accountSquare.joinFailed'))
    } finally {
      creatingAccessKey.value = false
    }
  }

  async function copyKey(key: SharedPoolAccessKey): Promise<void> {
    if (!key.key) return
    await copyToClipboard(key.key, t('accountSquare.generatedKeyCopied'))
  }

  async function reportPool(pool: PoolVM): Promise<void> {
    if (reportingPoolId.value !== null) return
    reportingPoolId.value = pool.id
    try {
      const reason = t('accountSquare.reportReason', { name: pool.name })
      const updated = await reportSharedPool(pool.id, reason)
      pools.value = pools.value.map(item => item.id === pool.id ? toVM(updated) : item)
      appStore.showSuccess(t('accountSquare.reportSuccess'))
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message
      appStore.showError(msg || t('accountSquare.reportFailed'))
    } finally {
      reportingPoolId.value = null
    }
  }

  async function likePool(pool: PoolVM): Promise<void> {
    if (!pool?.id || actingPoolId.value === pool.id) return
    actingPoolId.value = pool.id
    try {
      const updated = pool.likedByMe
        ? await unlikeSharedPool(pool.id)
        : await likeSharedPool(pool.id)
      const target = pools.value.find((item) => item.id === pool.id)
      if (target) {
        target.likes = updated.like_count ?? (pool.likedByMe ? Math.max(0, pool.likes - 1) : pool.likes + 1)
        target.likedByMe = Boolean(updated.liked_by_me)
      }
      appStore.showSuccess(updated.liked_by_me ? '已认可该共享池' : '已取消认可')
    } catch (e: unknown) {
      appStore.showError((e as { message?: string })?.message || '操作失败')
    } finally {
      actingPoolId.value = null
    }
  }


  // ── UI toggles ────────────────────────────────────────────────────────────

  function toggleMarketDetail(pool: PoolVM): void {
    expandedMarketDetailPoolId.value = expandedMarketDetailPoolId.value === pool.id ? null : pool.id
  }

  async function toggleMarketProbeHistory(pool: PoolVM): Promise<void> {
    if (expandedMarketProbePoolId.value === pool.id) {
      expandedMarketProbePoolId.value = null
      return
    }
    expandedMarketProbePoolId.value = pool.id
    if (!probeHistories[pool.id]?.length) {
      try {
        probeHistories[pool.id] = await listSharedPoolProbeHistories(pool.id, { limit: 10 })
      } catch {
        probeHistories[pool.id] = []
      }
    }
  }

  // ── Public API ────────────────────────────────────────────────────────────

  return {
    // state
    keyword, selectedStatus, selectedModel, selectedAvailability, marketView, sortBy,
    loading, loadError, pools, modelCatalog, serverStats, joinedPoolIds, mySeats,
    actingPoolId, reportingPoolId, myAccessKeys, latestGeneratedKey,
    creatingAccessKey, pricingDrawerPool,
    expandedMarketDetailPoolId, expandedMarketProbePoolId,
    probeHistories, poolCommunitySummaries,
    // computed
    filteredPools, stats,
    // functions
    loadPools, loadModelCatalog, loadMySeats, loadMyAccessKeys,
    loadPoolCommunitySummaries,
    joinPool, leavePool, createKeyFromPool, copyKey, reportPool, likePool,
    toggleMarketDetail, toggleMarketProbeHistory,
  }
}
