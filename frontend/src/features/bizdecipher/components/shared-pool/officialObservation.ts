import type { Group } from '@/types'
import type { UserMonitorView, MonitorTimelinePoint } from '@/api/channelMonitor'

// Discovery policy only: never disables a group, its keys or its billing.
export function officialDiscoveryState(
  status: string,
  monitor?: UserMonitorView,
  now = Date.now(),
): { hidden: boolean; label: string } {
  if (status !== 'active') return { hidden: true, label: '已停用' }
  if (monitor?.observation_mode === 'unavailable') return { hidden: true, label: '上游不可用' }
  const samples = observationSamples(monitor).filter((p): p is MonitorTimelinePoint => p !== null)
  const latest = samples.at(-1)
  const fresh = latest && now - Date.parse(latest.checked_at) >= 0 && now - Date.parse(latest.checked_at) <= 30 * 60_000
  if (!fresh) return { hidden: false, label: latest ? '检测待更新' : '尚待检测' }
  const anotherAvailable = monitor?.extra_models.some(m => m.status === 'operational' || m.status === 'degraded')
  if (latest.status === 'operational') return { hidden: false, label: '检测模型可用' }
  if (anotherAvailable) return { hidden: false, label: '部分模型可用' }
  const failures = samples.slice(-3)
  const sustainedFailure = failures.length === 3
    && failures.every(p => p.status === 'failed' && now - Date.parse(p.checked_at) <= 2 * 60 * 60_000)
    && new Set(failures.map(p => p.checked_at)).size === 3
  if (sustainedFailure) return { hidden: true, label: '连续检测失败' }
  return { hidden: false, label: latest.status === 'error' ? '检测异常' : '检测有波动' }
}

export function resolveOfficialObservation(
  group: Pick<Group, 'id' | 'name' | 'platform'>,
  groups: Pick<Group, 'id' | 'name' | 'platform'>[],
  monitors: UserMonitorView[],
): UserMonitorView | undefined {
  const sameGroup = (item: Pick<Group, 'name' | 'platform'>) => (
    item.name.trim() === group.name.trim() && item.platform === group.platform
  )
  // Persisted monitors expose a name, not a group ID. Never guess between duplicates.
  if (groups.filter(sameGroup).length !== 1) return undefined
  const candidates = monitors.filter(item => (
    item.provider === group.platform
    && item.group_name.trim() === group.name.trim()
    && (!item.synthetic || item.id === -group.id)
  ))
  return candidates.length === 1 ? candidates[0] : undefined
}

export function observationSamples(monitor?: UserMonitorView): Array<MonitorTimelinePoint | null> {
  const samples = monitor?.synthetic && monitor.observation_mode !== 'active' ? [] : (monitor?.timeline || [])
    .filter(item => Number.isFinite(Date.parse(item.checked_at)))
    .slice()
    .sort((a, b) => Date.parse(a.checked_at) - Date.parse(b.checked_at))
    .slice(-24)
  return [...Array<null>(24 - samples.length).fill(null), ...samples]
}

export function observedModelNames(monitor?: UserMonitorView): string[] {
  if (!monitor) return []
  return [...new Set([monitor.primary_model, ...(monitor.extra_models || []).map(item => item.model)]
    .map(item => item.trim()).filter(Boolean))]
}
