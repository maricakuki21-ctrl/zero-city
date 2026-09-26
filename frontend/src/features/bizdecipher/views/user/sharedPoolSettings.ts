import type { SharedPool, UpdateSharedPoolPayload } from '@/features/bizdecipher/api/bizdecipher'
import { parseSharedPoolMultiplier } from '@/features/bizdecipher/components/shared-pool/sharedPoolPricing'

export const SHARED_POOL_REVERIFY_MESSAGE = '设置已保存，门槛配置已变化，池子已自动下架；请重新满血检测，达标后自动上架。'
export const SHARED_POOL_SAVE_FAILED_MESSAGE = '保存失败，请重试'

export type SharedPoolSettingsDraft = {
  name: string
  description: string
  upstream_base_url: string
  upstream_api_key: string
  account_mode_enabled: boolean
  rate_multiplier: number
  max_users: number
  min_balance_admission: number
  hourly_seat_fee: number
  hourly_min_usage_waiver: number
  account_concurrency: number
  user_concurrency: number
  verification_mode: 'full_check' | 'professional_review'
  verification_exemption_reason: string
}

export type SharedPoolSettingsSaveOptions = {
  pool: SharedPool
  draft: SharedPoolSettingsDraft
  updatePool: (poolID: number, payload: UpdateSharedPoolPayload) => Promise<SharedPool>
  formatError: (error: unknown, fallback: string) => string
  showError: (message: string) => unknown
  formatEffectiveAt: (value: string) => string
}

export class SharedPoolSettingsSaveError extends Error {
  readonly originalError: unknown

  constructor(message: string, originalError: unknown) {
    super(message)
    this.name = 'SharedPoolSettingsSaveError'
    this.originalError = originalError
  }
}

export function buildSharedPoolSettingsPayload(
  pool: SharedPool,
  draft: SharedPoolSettingsDraft,
): UpdateSharedPoolPayload {
  const payload: UpdateSharedPoolPayload = {
    expected_config_version: Number(pool.config_version || 0),
  }

  const assignChanged = <K extends keyof UpdateSharedPoolPayload>(
    key: K,
    value: UpdateSharedPoolPayload[K],
    current: unknown,
  ): void => {
    if (value !== current) payload[key] = value
  }

  assignChanged('name', draft.name.trim() || pool.name, pool.name)
  assignChanged('description', draft.description.trim(), pool.description || '')
  assignChanged('upstream_base_url', draft.upstream_base_url.trim() || pool.upstream_base_url || '', pool.upstream_base_url || '')
  const rateMultiplier = parseSharedPoolMultiplier(draft.rate_multiplier)
  if (rateMultiplier == null) throw new Error('调用倍率必须是有限数字，且不能小于 0.0001。')
  const currentRateMultiplier = parseSharedPoolMultiplier(pool.rate_multiplier) ?? 1
  if (rateMultiplier !== currentRateMultiplier) {
    payload.rate_multiplier = rateMultiplier
    payload.sync_model_rates = true
  }
  assignChanged('max_users', Number(draft.max_users) || 20, Number(pool.max_users) || 20)
  assignChanged('min_balance_admission', Math.max(0, Number(draft.min_balance_admission) || 0), Math.max(0, Number(pool.min_balance_admission) || 0))
  assignChanged('hourly_seat_fee', Math.max(0, Number(draft.hourly_seat_fee) || 0), Math.max(0, Number(pool.hourly_seat_fee) || 0))
  assignChanged('hourly_min_usage_waiver', Math.max(0, Number(draft.hourly_min_usage_waiver) || 0), Math.max(0, Number(pool.hourly_min_usage_waiver) || 0))
  assignChanged('account_concurrency', Math.max(1, Number(draft.account_concurrency) || 1), Math.max(1, Number(pool.account_concurrency) || 1))
  assignChanged('user_concurrency', Math.max(1, Number(draft.user_concurrency) || 1), Math.max(1, Number(pool.user_concurrency) || 1))
  assignChanged('account_mode_enabled', draft.account_mode_enabled, Boolean(pool.account_mode_enabled))
  assignChanged('verification_mode', draft.verification_mode, pool.verification_mode || 'full_check')
  assignChanged('verification_exemption_reason', draft.verification_exemption_reason.trim(), pool.verification_exemption_reason || '')

  const upstreamAPIKey = draft.upstream_api_key.trim()
  if (upstreamAPIKey) payload.upstream_api_key = upstreamAPIKey
  return payload
}

export function sharedPoolSettingsSuccessMessage(
  wasListed: boolean,
  updated: SharedPool,
  formatEffectiveAt: (value: string) => string,
): string {
  if (wasListed && !updated.listed) return SHARED_POOL_REVERIFY_MESSAGE
  if (updated.pending_settlement_rule_effective_from) {
    return `设置已保存，将于 ${formatEffectiveAt(updated.pending_settlement_rule_effective_from)} 生效`
  }
  return '设置已保存'
}

export async function persistSharedPoolSettings(
  options: SharedPoolSettingsSaveOptions,
): Promise<{ updated: SharedPool; message: string }> {
  const wasListed = Boolean(options.pool.listed)
  try {
    const updated = await options.updatePool(
      options.pool.id,
      buildSharedPoolSettingsPayload(options.pool, options.draft),
    )
    return {
      updated,
      message: sharedPoolSettingsSuccessMessage(wasListed, updated, options.formatEffectiveAt),
    }
  } catch (error) {
    const message = options.formatError(error, SHARED_POOL_SAVE_FAILED_MESSAGE)
    options.showError(message)
    throw new SharedPoolSettingsSaveError(message, error)
  }
}
