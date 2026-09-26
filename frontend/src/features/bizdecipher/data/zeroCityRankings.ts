export type ZeroCityRankingSource = 'spending' | 'community' | 'answers' | 'assets'
export type ZeroCityRankingWindow = 'day' | 'week' | 'month' | 'all'

export interface ZeroCityRankingDefinition {
  key: string
  title: string
  description: string
  behaviorLabel: string
  weeklyRule: string
  monthlyRule: string
  source: ZeroCityRankingSource
  window: ZeroCityRankingWindow
  windowLabel: string
  valueUnit: string
  limit: number
  order: number
  enabled: boolean
  route?: string
}

export interface ZeroCityRankingConfig {
  version: number
  items: ZeroCityRankingDefinition[]
}

export interface ZeroCityRankingPost {
  id?: number
  userId?: number
  createdAt?: string
  author: string
  card?: string
  kind?: string
  district?: string
  scenario?: string
  status?: string
  tags?: string[]
  catches: number
  replies: number
  views: number
  evidenceCount: number
  official: boolean
  accepted: boolean
}

export interface ZeroCityRankingRow {
  rank: number
  label: string
  value: string
  secondary: string
  userId?: number
  postId?: number
}

export const defaultZeroCityRankingConfig: ZeroCityRankingConfig = {
  version: 2,
  items: [
    {
      key: 'spending',
      title: '有效消费',
      description: '真实有效消费形成排名，失败、退款与赠送额度不计入。',
      behaviorLabel: '多使用真实服务',
      weeklyRule: '统计口径与资格待权威数据源确认',
      monthlyRule: '未接入奖励结算，不预告奖励',
      source: 'spending',
      window: 'week',
      windowLabel: '本周',
      valueUnit: '余额',
      limit: 10,
      order: 10,
      enabled: true,
    },
    {
      key: 'contribution',
      title: '城市贡献',
      description: '采纳、确认、证据与真实交付共同形成贡献。',
      behaviorLabel: '留下可信贡献',
      weeklyRule: '只展示当前可见内容的贡献线索',
      monthlyRule: '待权威贡献与资格服务接入',
      source: 'community',
      window: 'week',
      windowLabel: '本周',
      valueUnit: '贡献',
      limit: 5,
      order: 20,
      enabled: true,
      route: '/community?district=governance&channel=badges',
    },
    {
      key: 'answers',
      title: '答疑互助',
      description: '按采纳、解决与有效证据计分，不按回复数量灌水。',
      behaviorLabel: '接住真实问题',
      weeklyRule: '仅按真实采纳、解决与证据信号整理',
      monthlyRule: '统计范围以页面标注的数据源为准',
      source: 'answers',
      window: 'week',
      windowLabel: '本周',
      valueUnit: '回答',
      limit: 5,
      order: 30,
      enabled: true,
      route: '/community?district=workshop&channel=help-desk',
    },
    {
      key: 'assets',
      title: '作品能力',
      description: '按真实使用、收藏、复用与交付证明形成排名。',
      behaviorLabel: '做出可复用作品',
      weeklyRule: '作品复用与交付证明需由原始系统确认',
      monthlyRule: '未实现的推荐或奖励不在此承诺',
      source: 'assets',
      window: 'month',
      windowLabel: '本月',
      valueUnit: '复用',
      limit: 5,
      order: 40,
      enabled: true,
      route: '/assets',
    },
  ],
}

export async function loadZeroCityRankingConfig(): Promise<ZeroCityRankingConfig> {
  const response = await fetch('/config/zero-city-rankings.json', { cache: 'no-store' })
  if (!response.ok) throw new Error(`ranking config request failed: ${response.status}`)
  return parseZeroCityRankingConfig(await response.json())
}

export function parseZeroCityRankingConfig(value: unknown): ZeroCityRankingConfig {
  if (!value || typeof value !== 'object') return { version: defaultZeroCityRankingConfig.version, items: [] }
  const config = value as Record<string, unknown>
  const rawItems = Array.isArray(config.items) ? config.items : []
  return normalizeRankingConfig({
    version: typeof config.version === 'number' ? config.version : defaultZeroCityRankingConfig.version,
    items: rawItems.map(migrateRankingDefinition).filter((item): item is ZeroCityRankingDefinition => Boolean(item)),
  })
}

export function isRankingDefinition(value: unknown): value is ZeroCityRankingDefinition {
  if (!value || typeof value !== 'object') return false
  const item = value as Record<string, unknown>
  return typeof item.key === 'string' &&
    typeof item.title === 'string' &&
    typeof item.description === 'string' &&
    typeof item.behaviorLabel === 'string' &&
    typeof item.weeklyRule === 'string' &&
    typeof item.monthlyRule === 'string' &&
    ['spending', 'community', 'answers', 'assets'].includes(String(item.source)) &&
    ['day', 'week', 'month', 'all'].includes(String(item.window)) &&
    typeof item.windowLabel === 'string' &&
    typeof item.valueUnit === 'string' &&
    Number.isFinite(item.limit) &&
    Number.isFinite(item.order) &&
    typeof item.enabled === 'boolean'
}

function migrateRankingDefinition(value: unknown): ZeroCityRankingDefinition | undefined {
  if (!value || typeof value !== 'object') return undefined
  const item = value as Record<string, unknown>
  const source = String(item.source)
  const window = String(item.window)
  if (
    typeof item.key !== 'string' ||
    typeof item.title !== 'string' ||
    typeof item.description !== 'string' ||
    typeof item.behaviorLabel !== 'string' ||
    !['spending', 'community', 'answers', 'assets'].includes(source) ||
    !['day', 'week', 'month', 'all'].includes(window) ||
    typeof item.windowLabel !== 'string' ||
    typeof item.valueUnit !== 'string' ||
    !Number.isFinite(item.limit) ||
    !Number.isFinite(item.order) ||
    typeof item.enabled !== 'boolean'
  ) return undefined

  const safeRules = defaultZeroCityRankingConfig.items.find((definition) => definition.key === item.key) ??
    defaultZeroCityRankingConfig.items.find((definition) => definition.source === source)

  return {
    key: item.key,
    title: item.title,
    description: item.description,
    behaviorLabel: item.behaviorLabel,
    weeklyRule: typeof item.weeklyRule === 'string' ? item.weeklyRule : safeRules?.weeklyRule || '仅展示可验证记录',
    monthlyRule: typeof item.monthlyRule === 'string' ? item.monthlyRule : safeRules?.monthlyRule || '未接入的奖励与结算不在此承诺',
    source: source as ZeroCityRankingSource,
    window: window as ZeroCityRankingWindow,
    windowLabel: item.windowLabel,
    valueUnit: item.valueUnit,
    limit: Number(item.limit),
    order: Number(item.order),
    enabled: item.enabled,
    route: typeof item.route === 'string' && item.route !== '/community#community-token-intel' ? item.route : undefined,
  }
}

export function normalizeRankingConfig(value: ZeroCityRankingConfig): ZeroCityRankingConfig {
  const items = value.items
    .filter(isRankingDefinition)
    .map((item) => ({
      ...item,
      limit: Math.max(1, Math.min(10, Math.floor(item.limit))),
      order: Number(item.order),
    }))
    .sort((left, right) => left.order - right.order)
  return { version: value.version, items }
}

export function buildZeroCityRankingRows(
  definition: ZeroCityRankingDefinition,
  posts: readonly ZeroCityRankingPost[],
): ZeroCityRankingRow[] {
  if (definition.source === 'spending') return []
  const scoped = posts.filter((post) => isWithinRankingWindow(post, definition) && (() => {
    if (definition.source === 'answers') return post.kind === 'support' || post.scenario === 'incident_support' || post.accepted
    if (definition.source === 'assets') return post.scenario === 'capability_showcase' || post.tags?.some((tag) => /能力|资产|作品|asset/i.test(tag))
    return true
  })())

  const groups = new Map<string, { label: string; userId?: number; posts: ZeroCityRankingPost[] }>()
  for (const post of scoped) {
    const label = post.card || post.author || '居民'
    const key = post.userId ? `user:${post.userId}` : `name:${label}`
    const group = groups.get(key)
    if (group) group.posts.push(post)
    else groups.set(key, { label, userId: post.userId, posts: [post] })
  }

  return [...groups.values()]
    .map((group) => {
      const score = group.posts.reduce((total, post) => {
        const base = definition.source === 'assets'
          ? post.catches + post.views * 0.02
          : post.replies * 2 + post.catches + post.evidenceCount * 4
        return total + base + (post.accepted ? 12 : 0) + (post.official ? 8 : 0)
      }, 0)
      const roundedScore = Math.max(1, Math.round(score))
      const evidence = group.posts.reduce((total, post) => total + post.evidenceCount, 0)
      const replies = group.posts.reduce((total, post) => total + post.replies, 0)
      const latest = group.posts[0]
      return {
        label: group.label,
        value: `${roundedScore} ${definition.valueUnit}`,
        secondary: `${group.posts.length} 条动态 · ${replies} 回复 · ${evidence} 条证据`,
        userId: group.userId,
        postId: latest?.id,
        score,
      }
    })
    .sort((left, right) => right.score - left.score || left.label.localeCompare(right.label, 'zh-CN'))
    .slice(0, definition.limit)
    .map((row, index) => ({
      rank: index + 1,
      label: row.label,
      value: row.value,
      secondary: row.secondary,
      userId: row.userId,
      postId: row.postId,
    }))
}

function isWithinRankingWindow(post: ZeroCityRankingPost, definition: ZeroCityRankingDefinition): boolean {
  if (definition.window === 'all' || !post.createdAt) return true
  const createdAt = Date.parse(post.createdAt)
  if (!Number.isFinite(createdAt)) return true
  const days = definition.window === 'day' ? 1 : definition.window === 'week' ? 7 : 30
  return createdAt >= Date.now() - days * 24 * 60 * 60 * 1000
}
