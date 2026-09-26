import { reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import {
  createSharedPool,
  updateSharedPool,
  deleteSharedPool,
  createSharedPoolAccount,
  updateSharedPoolAccount,
  deleteSharedPoolAccount,
  importSharedPoolAccounts,
  listMySharedPools,
  listMySharedPoolLedger,
  listSharedPoolAccounts,
  listSharedPoolAccountProbeHistories,
  listSharedPoolMembers,
  removeSharedPoolMember,
  probeSharedPoolUpstream,
  fetchSharedPoolUpstreamModels,
  type SharedPool as ApiSharedPool,
  type SharedPoolAccount,
  type SharedPoolLedgerView,
  type SharedPoolProbeHistory,
  type CreateSharedPoolPayload,
  type UpdateSharedPoolPayload,
  type PoolSeat,
} from '@/api/bizdecipher'

// ── Local types ──────────────────────────────────────────────
export interface PoolFormState {
  name: string
  description: string
  avatar_url: string
  status_note: string
  disabled_reason: string
  upstream_base_url: string
  upstream_api_key: string
  rate_multiplier: number
  max_users: number
  min_balance_admission: number
  hourly_seat_fee: number
  hourly_min_usage_waiver: number
  proxy_id: number | null
  proxy_url: string
  proxy_region: string
  proxy_status: string
  account_concurrency: number
  user_concurrency: number
  account_mode_enabled: boolean
  oauth_provider: string
  probe_model: string
  listed: boolean
  status: 'healthy' | 'limited' | 'offline' | 'maintenance'
}

export interface PoolAccountFormState {
  name: string
  description: string
  provider: string
  auth_type: string
  upstream_base_url: string
  upstream_api_key: string
  status: string
  status_note: string
  disabled_reason: string
  group_name: string
  proxy_id: number | null
  proxy_url: string
  proxy_region: string
  proxy_status: string
  account_weight: number
  priority: number
  rpm_limit: number
  account_concurrency: number
  user_concurrency: number
  gate_required: boolean
  ttl: number
  cache_enabled: boolean
  routing_policy: string
}

export function emptyPoolForm(): PoolFormState {
  return {
    name: '', description: '', avatar_url: '', status_note: '', disabled_reason: '',
    upstream_base_url: '', upstream_api_key: '', rate_multiplier: 1.0, max_users: 100,
    min_balance_admission: 0, hourly_seat_fee: 0, hourly_min_usage_waiver: 0,
    proxy_id: null, proxy_url: '', proxy_region: '', proxy_status: '',
    account_concurrency: 10, user_concurrency: 3, account_mode_enabled: false,
    oauth_provider: '', probe_model: '', listed: false, status: 'healthy',
  }
}

export function emptyAccountForm(): PoolAccountFormState {
  return {
    name: '', description: '', provider: 'openai', auth_type: 'api_key',
    upstream_base_url: '', upstream_api_key: '', status: 'active',
    status_note: '', disabled_reason: '', group_name: '',
    proxy_id: null, proxy_url: '', proxy_region: '', proxy_status: '',
    account_weight: 1, priority: 0, rpm_limit: 0, account_concurrency: 5,
    user_concurrency: 2, gate_required: true, ttl: 3600, cache_enabled: false,
    routing_policy: 'round_robin',
  }
}

// ── Composable ───────────────────────────────────────────────
export function usePoolOwner() {
  const { t } = useI18n()
  const appStore = useAppStore()

  // ── State ──
  const myPools = ref<ApiSharedPool[]>([])
  const sharedPoolLedger = ref<SharedPoolLedgerView>({
    wallet: {
      owner_id: 0,
      available_amount: 0,
      pending_amount: 0,
      frozen_amount: 0,
      transferred_amount: 0,
      total_earned: 0,
      version: 0,
    },
    earnings: [],
    activity: [],
    withdrawable: [],
    legacy_withdrawable: [],
    incentives: [],
  })
  const poolMembers = reactive<Record<number, PoolSeat[]>>({})
  const poolAccounts = reactive<Record<number, SharedPoolAccount[]>>({})
  const poolForms = reactive<Record<number, PoolFormState>>({})
  const poolAccountForms = reactive<Record<string, PoolAccountFormState>>({})
  const accountImportText = reactive<Record<number, string>>({})
  const accountProbeHistories = reactive<Record<string, SharedPoolProbeHistory[]>>({})
  const ownerProbeHistories = reactive<Record<number, SharedPoolProbeHistory[]>>({})
  const bulkAccountSelection = reactive<Record<number, number[]>>({})

  // Loading states
  const creatingPool = ref(false)
  const deletingPoolId = ref<number | null>(null)
  const savingPoolId = ref<number | null>(null)
  const probingPoolKey = ref<string | null>(null)
  const expandedOwnerProbePoolId = ref<number | null>(null)
  const expandedAccountProbeKey = ref<string | null>(null)
  const expandedAccountPoolId = ref<number | null>(null)
  const accountLoadingPoolId = ref<number | null>(null)
  const savingAccountKey = ref<string | null>(null)
  const deletingAccountId = ref<number | null>(null)
  const importingAccountPoolId = ref<number | null>(null)
  const memberLoadingPoolId = ref<number | null>(null)
  const removingSeatId = ref<number | null>(null)
  const syncingModelsKey = ref<string | null>(null)
  const syncedModels = reactive<Record<string, string[]>>({})

  // ── Load functions ──
  async function loadMyPools(): Promise<void> {
    try {
      myPools.value = await listMySharedPools()
    } catch {
      myPools.value = []
    }
  }

  async function loadSharedPoolLedger(): Promise<void> {
    try {
      sharedPoolLedger.value = await listMySharedPoolLedger()
    } catch {
      sharedPoolLedger.value = {
        wallet: {
          owner_id: 0,
          available_amount: 0,
          pending_amount: 0,
          frozen_amount: 0,
          transferred_amount: 0,
          total_earned: 0,
          version: 0,
        },
        earnings: [],
        activity: [],
        withdrawable: [],
        legacy_withdrawable: [],
        incentives: [],
      }
    }
  }

  async function loadPoolAccounts(poolId: number): Promise<void> {
    accountLoadingPoolId.value = poolId
    try {
      poolAccounts[poolId] = await listSharedPoolAccounts(poolId)
    } catch {
      poolAccounts[poolId] = []
    } finally {
      accountLoadingPoolId.value = null
    }
  }

  async function loadPoolMembers(poolId: number): Promise<void> {
    memberLoadingPoolId.value = poolId
    try {
      poolMembers[poolId] = await listSharedPoolMembers(poolId)
    } catch {
      poolMembers[poolId] = []
    } finally {
      memberLoadingPoolId.value = null
    }
  }

  // ── Pool CRUD ──
  async function createPool(payload: CreateSharedPoolPayload): Promise<void> {
    creatingPool.value = true
    try {
      await createSharedPool(payload)
      await loadMyPools()
      appStore.showSuccess(t('accountSquare.poolCreated') || '共享池已创建')
    } catch (e: unknown) {
      const msg = (e as { message?: string })?.message || t('common.error')
      appStore.showError(msg)
    } finally {
      creatingPool.value = false
    }
  }

  async function updatePool(id: number, payload: UpdateSharedPoolPayload): Promise<void> {
    savingPoolId.value = id
    try {
      await updateSharedPool(id, payload)
      await loadMyPools()
      appStore.showSuccess(t('accountSquare.poolSaved') || '已保存')
    } catch (e: unknown) {
      const msg = (e as { message?: string })?.message || t('common.error')
      appStore.showError(msg)
    } finally {
      savingPoolId.value = null
    }
  }

  async function deletePool(id: number): Promise<void> {
    deletingPoolId.value = id
    try {
      await deleteSharedPool(id)
      await loadMyPools()
      appStore.showSuccess(t('accountSquare.poolDeleted') || '已删除')
    } catch (e: unknown) {
      const msg = (e as { message?: string })?.message || t('common.error')
      appStore.showError(msg)
    } finally {
      deletingPoolId.value = null
    }
  }

  // ── Account CRUD ──
  async function createAccount(poolId: number, payload: Parameters<typeof createSharedPoolAccount>[1]): Promise<boolean> {
    try {
      await createSharedPoolAccount(poolId, payload)
      await loadPoolAccounts(poolId)
      appStore.showSuccess(t('accountSquare.accountAdded') || '账号已添加')
      return true
    } catch (e: unknown) {
      const msg = (e as { message?: string })?.message || t('common.error')
      appStore.showError(msg)
      return false
    }
  }

  async function updateAccount(
    poolId: number,
    accountId: number,
    payload: Parameters<typeof updateSharedPoolAccount>[2]
  ): Promise<void> {
    savingAccountKey.value = `${poolId}-${accountId}`
    try {
      await updateSharedPoolAccount(poolId, accountId, payload)
      await loadPoolAccounts(poolId)
    } catch (e: unknown) {
      const msg = (e as { message?: string })?.message || t('common.error')
      appStore.showError(msg)
    } finally {
      savingAccountKey.value = null
    }
  }

  async function deleteAccount(poolId: number, accountId: number): Promise<void> {
    deletingAccountId.value = accountId
    try {
      await deleteSharedPoolAccount(poolId, accountId)
      await loadPoolAccounts(poolId)
      appStore.showSuccess(t('accountSquare.accountDeleted') || '已删除')
    } catch (e: unknown) {
      const msg = (e as { message?: string })?.message || t('common.error')
      appStore.showError(msg)
    } finally {
      deletingAccountId.value = null
    }
  }

  function defaultAccountModelConfig(modelName?: string) {
    return {
      provider: 'openai',
      model_name: (modelName || '').trim() || 'gpt-3.5-turbo',
      rate_multiplier: 1,
      five_hour_protection_percent: 100,
      seven_day_protection_percent: 100,
      daily_protection_percent: 100,
      max_concurrency: 0,
      model_open: true,
    }
  }

  async function importAccounts(poolId: number, csvText: string): Promise<boolean> {
    importingAccountPoolId.value = poolId
    try {
      const lines = csvText.trim().split('\n').filter(Boolean)
      const items = lines.map(line => {
        const [name, upstream_base_url, upstream_api_key, model_name] = line.split(',').map(s => s.trim())
        return {
          name: name || '',
          upstream_base_url: upstream_base_url || '',
          upstream_api_key: upstream_api_key || '',
          model_configs: [defaultAccountModelConfig(model_name)],
        }
      })
      await importSharedPoolAccounts(poolId, items)
      await loadPoolAccounts(poolId)
      appStore.showSuccess(t('accountSquare.imported') || '已导入')
      return true
    } catch (e: unknown) {
      const msg = (e as { message?: string })?.message || t('common.error')
      appStore.showError(msg)
      return false
    } finally {
      importingAccountPoolId.value = null
    }
  }

  // ── Probe ──
  async function probePool(payload: { poolId: number; accountId?: number; baseUrl?: string; apiKey?: string; probeModel?: string }): Promise<void> {
    const key = payload.accountId ? `${payload.poolId}-${payload.accountId}` : `${payload.poolId}`
    probingPoolKey.value = key
    try {
      await probeSharedPoolUpstream({
        pool_id: payload.poolId,
        account_id: payload.accountId,
        upstream_base_url: payload.baseUrl || '',
        upstream_api_key: payload.apiKey || '',
        probe_model: payload.probeModel || 'gpt-3.5-turbo',
        probe_type: 'manual',
      })
      appStore.showSuccess(t('accountSquare.probeStarted') || '检测已提交')
    } catch (e: unknown) {
      const msg = (e as { message?: string })?.message || t('common.error')
      appStore.showError(msg)
    } finally {
      probingPoolKey.value = null
    }
  }

  async function syncUpstreamModels(poolId: number, accountId?: number): Promise<string[]> {
    const key = accountId ? `${poolId}-${accountId}` : `${poolId}`
    syncingModelsKey.value = key
    try {
      const result = await fetchSharedPoolUpstreamModels({
        pool_id: poolId,
        account_id: accountId,
      })
      const models = result.models || []
      syncedModels[key] = models
      appStore.showSuccess(models.length > 0 ? `已拉取 ${models.length} 个模型` : '上游暂未返回模型')
      return models
    } catch (e: unknown) {
      syncedModels[key] = []
      const msg = (e as { message?: string })?.message || t('common.error')
      appStore.showError(msg)
      return []
    } finally {
      syncingModelsKey.value = null
    }
  }

  async function loadAccountProbeHistory(poolId: number, accountId: number): Promise<void> {
    const key = `${poolId}-${accountId}`
    try {
      accountProbeHistories[key] = await listSharedPoolAccountProbeHistories(poolId, accountId, { limit: 20 })
    } catch {
      accountProbeHistories[key] = []
    }
  }

  // ── Member management ──
  async function removePoolMember(poolId: number, seatId: number): Promise<void> {
    removingSeatId.value = seatId
    try {
      await removeSharedPoolMember(poolId, seatId)
      await loadPoolMembers(poolId)
    } catch (e: unknown) {
      const msg = (e as { message?: string })?.message || t('common.error')
      appStore.showError(msg)
    } finally {
      removingSeatId.value = null
    }
  }

  // ── UI toggles ──
  function toggleAccountPanel(poolId: number): void {
    if (expandedAccountPoolId.value === poolId) {
      expandedAccountPoolId.value = null
    } else {
      expandedAccountPoolId.value = poolId
      if (!poolAccounts[poolId]) loadPoolAccounts(poolId)
    }
  }

  function toggleOwnerProbeHistory(poolId: number): void {
    expandedOwnerProbePoolId.value = expandedOwnerProbePoolId.value === poolId ? null : poolId
  }

  function toggleAccountProbeHistory(key: string, poolId: number, accountId: number): void {
    if (expandedAccountProbeKey.value === key) {
      expandedAccountProbeKey.value = null
    } else {
      expandedAccountProbeKey.value = key
      loadAccountProbeHistory(poolId, accountId)
    }
  }

  // ── Form helpers ──
  function initPoolForm(pool: ApiSharedPool): void {
    poolForms[pool.id] = {
      name: pool.name || '',
      description: pool.description || '',
      avatar_url: pool.avatar_url || '',
      status_note: pool.status_note || '',
      disabled_reason: pool.disabled_reason || '',
      upstream_base_url: pool.upstream_base_url || '',
      upstream_api_key: '',
      rate_multiplier: pool.rate_multiplier ?? 1,
      max_users: pool.max_users ?? 100,
      min_balance_admission: pool.min_balance_admission ?? 0,
      hourly_seat_fee: pool.pending_hourly_seat_fee ?? pool.hourly_seat_fee ?? 0,
      hourly_min_usage_waiver: pool.pending_hourly_min_usage_waiver ?? pool.hourly_min_usage_waiver ?? 0,
      proxy_id: pool.proxy_id ?? null,
      proxy_url: pool.proxy_url || '',
      proxy_region: pool.proxy_region || '',
      proxy_status: pool.proxy_status || '',
      account_concurrency: pool.account_concurrency ?? 10,
      user_concurrency: pool.user_concurrency ?? 3,
      account_mode_enabled: pool.account_mode_enabled ?? false,
      oauth_provider: pool.oauth_provider || '',
      probe_model: '',
      listed: pool.listed ?? false,
      status: (pool.status as 'healthy' | 'limited' | 'offline' | 'maintenance') ?? 'healthy',
    }
  }

  return {
    // state
    myPools, sharedPoolLedger, poolMembers, poolAccounts,
    poolForms, poolAccountForms, accountImportText,
    accountProbeHistories, ownerProbeHistories, bulkAccountSelection,
    // loading
    creatingPool, deletingPoolId, savingPoolId, probingPoolKey,
    expandedOwnerProbePoolId, expandedAccountProbeKey,
    expandedAccountPoolId, accountLoadingPoolId,
    savingAccountKey, deletingAccountId, importingAccountPoolId,
    memberLoadingPoolId, removingSeatId, syncingModelsKey, syncedModels,
    // functions
    loadMyPools, loadSharedPoolLedger, loadPoolAccounts, loadPoolMembers,
    createPool, updatePool, deletePool,
    createAccount, updateAccount, deleteAccount, importAccounts,
    probePool, syncUpstreamModels, loadAccountProbeHistory, removePoolMember,
    toggleAccountPanel, toggleOwnerProbeHistory, toggleAccountProbeHistory,
    initPoolForm, emptyPoolForm, emptyAccountForm,
  }
}

// ── Card Skin ──────────────────────────────────────────────────────────────
import { setPoolCardSkin as apiSetSkin, clearPoolCardSkin as apiClearSkin } from '@/api/bizdecipher'
import { getCheckinCards } from '@/api/user'
import type { CheckinCollectibleCard } from '@/types'

export function usePoolCardSkin(
  appStore: ReturnType<typeof import('@/stores/app').useAppStore>,
  afterChange?: () => void | Promise<void>,
) {
  const settingSkinPoolId = ref<number | null>(null)
  const clearingSkinPoolId = ref<number | null>(null)
  const skinSelectorPoolId = ref<number | null>(null)
  const availableCards = ref<CheckinCollectibleCard[]>([])
  const loadingCards = ref(false)

  async function loadMyCards(): Promise<void> {
    loadingCards.value = true
    try {
      availableCards.value = await getCheckinCards()
    } catch {
      availableCards.value = []
    } finally {
      loadingCards.value = false
    }
  }

  async function setCardSkin(poolId: number, cardKey: string, cardRarity: string): Promise<void> {
    settingSkinPoolId.value = poolId
    try {
      await apiSetSkin(poolId, cardKey, cardRarity)
      await afterChange?.()
      appStore.showSuccess('底色已设置')
      skinSelectorPoolId.value = null
    } catch (e: unknown) {
      appStore.showError((e as {message?:string})?.message || '设置失败')
    } finally {
      settingSkinPoolId.value = null
    }
  }

  async function clearCardSkin(poolId: number): Promise<void> {
    clearingSkinPoolId.value = poolId
    try {
      await apiClearSkin(poolId)
      await afterChange?.()
      appStore.showSuccess('底色已清除')
    } catch (e: unknown) {
      appStore.showError((e as {message?:string})?.message || '清除失败')
    } finally {
      clearingSkinPoolId.value = null
    }
  }

  function openSkinSelector(poolId: number): void {
    skinSelectorPoolId.value = poolId
    if (availableCards.value.length === 0) loadMyCards()
  }

  function closeSkinSelector(): void {
    skinSelectorPoolId.value = null
  }

  return {
    settingSkinPoolId, clearingSkinPoolId, skinSelectorPoolId,
    availableCards, loadingCards,
    setCardSkin, clearCardSkin, openSkinSelector, closeSkinSelector,
  }
}
