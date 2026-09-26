import { computed, ref, watch, type Ref } from 'vue'
import {
  activateSharedPoolNativeBilling,
  createSharedPoolNativeOperationID,
  listMySharedPools,
  type ActivateNativeSharedPoolBillingPayload,
  type SharedPool,
} from '@/features/bizdecipher/api/bizdecipher'

type PublicationState = 'idle' | 'pending' | 'uncertain' | 'success' | 'error'

function isConfigVersion(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value > 0
}

export function useNativePoolPublication(pool: Ref<SharedPool | null>) {
  const state = ref<PublicationState>('idle')
  const message = ref('')
  const operation = ref<ActivateNativeSharedPoolBillingPayload | null>(null)
  const blockedVersion = ref<number | null>(null)
  const pending = computed(() => state.value === 'pending')
  const unresolved = computed(() => operation.value !== null)
  const canPublish = computed(() => {
    const current = pool.value
    if (!current || current.owner_paused || pending.value) return false
    if (operation.value) return true
    return !current.listed && current.native_onboarding_state === 'supply_ready_billing_blocked'
      && isConfigVersion(current.config_version)
      && current.config_version !== blockedVersion.value
  })
  const storageKey = (id: number) => `shared-pool-native-publication:${id}`

  function persist(id: number, value: ActivateNativeSharedPoolBillingPayload | null) {
    operation.value = value
    try {
      if (value) sessionStorage.setItem(storageKey(id), JSON.stringify(value))
      else sessionStorage.removeItem(storageKey(id))
    } catch {
      // Same-tab retries still use the in-memory operation if storage is unavailable.
    }
  }

  watch(() => pool.value?.id, id => {
    operation.value = null
    state.value = 'idle'
    message.value = ''
    blockedVersion.value = null
    if (!id) return
    try {
      const stored = JSON.parse(sessionStorage.getItem(storageKey(id)) || 'null')
      if (stored && typeof stored.operation_id === 'string' && stored.operation_id.length > 0
        && stored.operation_id.length <= 160 && isConfigVersion(stored.expected_config_version)) {
        operation.value = { operation_id: stored.operation_id, expected_config_version: stored.expected_config_version }
        state.value = 'uncertain'
        message.value = '上次发布结果尚未确认，请重试确认；不会新建发布请求。'
      }
    } catch {
      // Invalid browser data is not an activation command.
    }
  }, { immediate: true, flush: 'sync' })

  async function publish() {
    const current = pool.value
    if (!current || !canPublish.value) return
    const version = current.config_version
    let payload = operation.value
    if (!payload) {
      if (!isConfigVersion(version)) return
      payload = {
        operation_id: createSharedPoolNativeOperationID(),
        expected_config_version: version,
      }
    }
    persist(current.id, payload)
    state.value = 'pending'
    message.value = ''

    let result
    try {
      result = await activateSharedPoolNativeBilling(current.id, payload)
      if (!result || result.pool_id !== current.id || result.state !== 'billing_active'
        || !isConfigVersion(result.config_version) || result.config_version <= payload.expected_config_version
        || !isConfigVersion(result.activated_config_version)
        || result.activated_config_version <= payload.expected_config_version
        || result.activated_config_version > result.config_version) {
        throw new Error('Unconfirmed activation response')
      }
    } catch (error) {
      if (pool.value?.id !== current.id) return
      const status = (error as { status?: number } | null)?.status
      if ([400, 401, 403, 404, 409, 422].includes(status || 0)) {
        persist(current.id, null)
        state.value = 'error'
        if (status === 409) {
          blockedVersion.value = payload.expected_config_version
          message.value = '配置已变化，本次未发布。正在获取新版本，请核对后重新发布。'
          try {
            const latest = (await listMySharedPools()).find(item => item.id === current.id)
            if (!latest || pool.value?.id !== current.id) throw new Error('Missing pool')
            pool.value = latest
            message.value = latest.listed ? '服务器显示池子已上架，已同步最新状态。' : '配置已更新，请核对资源和价格后重新发布。'
          } catch {
            message.value = '配置已变化，刷新失败。请先刷新页面获取最新配置，再发布。'
          }
        } else if (status === 422) {
          message.value = '暂不能发布：请检查资源连接、模型报价和结算配置；未修改池子上架状态。'
        } else if (status === 404) {
          message.value = '服务器未提供该池的发布入口，请刷新或联系管理员核对服务版本。'
        } else if (status === 401 || status === 403) {
          message.value = '当前登录身份无权发布该池，请重新登录池主账号。'
        } else {
          message.value = '发布参数无效，请刷新后重新提交。'
        }
      } else {
        state.value = 'uncertain'
        message.value = '未能确认发布结果。请重试确认，系统会沿用同一请求，不重复开通。'
      }
      return
    }

    if (pool.value?.id !== current.id) return
    persist(current.id, null)
    state.value = 'success'
    // Activation is confirmed; a subsequent list failure must not be shown as a failed write.
    pool.value = { ...pool.value, native_onboarding_state: result.state, config_version: result.config_version }
    message.value = '服务器已确认计费开通，正在同步上架状态。'
    try {
      const latest = (await listMySharedPools()).find(item => item.id === current.id)
      if (!latest || !isConfigVersion(latest.config_version) || latest.config_version < result.config_version) throw new Error('Pool view not refreshed')
      if (pool.value?.id !== current.id) return
      pool.value = latest
      message.value = latest.listed
        ? '已上架。计费开通不代表每个模型的实际推理均已验证。'
        : '计费开通已确认；服务器当前显示未上架，请检查最新资源状态。'
    } catch {
      message.value = '计费开通已确认，但上架状态刷新失败。请刷新页面，无需重复提交。'
    }
  }

  return { state, message, pending, unresolved, canPublish, publish }
}
