import type { SharedPool } from '@/features/bizdecipher/api/bizdecipher'

export type SharedPoolLifecycleView = 'current' | 'attention' | 'archived' | 'all'
export type SharedPoolOpsBucket = 'supply' | 'pricing' | 'probe' | 'community' | 'attention' | 'current' | 'archived'

const ATTENTION_STATUSES = new Set(['offline', 'maintenance'])
const ATTENTION_GOVERNANCE_STATUSES = new Set(['watch', 'suppressed', 'banned'])

export function sharedPoolOpsBucket(pool: SharedPool): SharedPoolOpsBucket {
  if (isExplicitlyArchivedPool(pool)) return 'archived'
  if (['banned', 'suppressed'].includes(pool.governance_status || '')
    || ['offline', 'maintenance', 'limited'].includes(pool.status)
    || pool.lifecycle_state === 'suspended') return 'attention'
  if (Number(pool.complaint_count || 0) > 0 || pool.governance_status === 'watch') return 'community'
  const summary = sharedPoolCompatibilitySummary(pool)
  if (!summary.discovered) return 'supply'
  if (summary.probe !== 'passed') return 'probe'
  // A list snapshot is not the endpoint-pricing authority. Missing snapshots
  // require inspection, not a claim that the model is free or unpriced.
  if (summary.priced < summary.discovered) return 'pricing'
  if (pool.listed !== true || pool.billing_activation_required === true) return 'attention'
  return 'current'
}

export type SharedPoolCompatibilitySummary = {
  discovered: number
  priced: number
  probe: 'passed' | 'failed' | 'pending'
  label: string
}

export function sharedPoolCompatibilitySummary(pool: SharedPool): SharedPoolCompatibilitySummary {
  const configs = (pool.model_configs || []).filter((model) => model.enabled !== false && model.model_open !== false)
  const disabled = new Set((pool.model_configs || [])
    .filter(model => model.enabled === false || model.model_open === false)
    .map(model => model.model_name))
  const names = new Set([...(pool.models || []), ...configs.map(model => model.model_name)]
    .filter(name => name.trim() && !disabled.has(name)))
  const discovered = names.size
  const priced = new Set(configs.filter(model => Boolean(model.pricing)).map(model => model.model_name)).size
  const hasProbe = Boolean(pool.last_probe_at && Number.isFinite(Date.parse(pool.last_probe_at)))
  const probe = !hasProbe ? 'pending'
    : pool.last_probe_success === false || (pool.last_probe_gate_required === true && pool.last_probe_gate_passed === false) ? 'failed'
      : pool.last_probe_success === true && (!pool.last_probe_gate_required || pool.last_probe_gate_passed === true) ? 'passed'
        : 'pending'
  const label = discovered === 0
    ? '待发现模型'
    : probe === 'failed'
      ? '最近池级检测失败'
      : probe === 'pending'
        ? '池级检测待核'
        : priced < discovered
          ? `模型 ${discovered} · 报价待核`
          : `模型 ${discovered} · 池级检测通过`
  return { discovered, priced, probe, label }
}

export function isExplicitlyArchivedPool(pool: SharedPool): boolean {
  return String(pool.lifecycle_state || '').toLowerCase() === 'archived' || Boolean(pool.archived_at)
}

export function needsSharedPoolAttention(pool: SharedPool): boolean {
  if (isExplicitlyArchivedPool(pool)) return false

  const status = String(pool.status || '').toLowerCase()
  const governanceStatus = String(pool.governance_status || '').toLowerCase()
  return pool.listed === false
    || ATTENTION_STATUSES.has(status)
    || ATTENTION_GOVERNANCE_STATUSES.has(governanceStatus)
    || Number(pool.consecutive_probe_failures || 0) >= 3
}

export function filterSharedPoolsByLifecycle(
  pools: readonly SharedPool[],
  view: SharedPoolLifecycleView,
): SharedPool[] {
  if (view === 'all') return [...pools]
  if (view === 'archived') return pools.filter(isExplicitlyArchivedPool)
  if (view === 'attention') return pools.filter(needsSharedPoolAttention)
  return pools.filter((pool) => !isExplicitlyArchivedPool(pool) && !needsSharedPoolAttention(pool))
}

export function hasSharedPoolArchiveContract(pools: readonly SharedPool[]): boolean {
  return pools.some((pool) =>
    Object.prototype.hasOwnProperty.call(pool, 'lifecycle_state')
      || Object.prototype.hasOwnProperty.call(pool, 'archived_at')
      || Object.prototype.hasOwnProperty.call(pool, 'archive_reason'),
  )
}
