import {
  getZeroCityCardDefinition,
  zeroCityCardCatalogRarity,
  zeroCityCardImagePath,
  zeroCityCardRarityDisplayName,
  type ZeroCityCardRarity,
} from '@/constants/zeroCityCardManifest'

export interface SharedPoolCardPresentationInput {
  cardKey?: string | null
  cardRarity?: string | null
  fallbackCardKey?: string | null
  fallbackCardRarity?: string | null
}

export interface SharedPoolCardPresentation {
  key: string
  rarity: ZeroCityCardRarity
  rarityLabel: string
  badgeLabel: string
  name: string
  faction: string
  role: string
  line: string
  lore: string
  imageUrl: string
}

export interface SharedPoolServicePresentationInput {
  status?: string | null
  observation?: boolean
  todayAvailability?: number | null
  sevenDayAvailability?: number | null
  avgLatencyMs?: number | null
  hasEvidence?: boolean
}

export type SharedPoolServiceTone = 'excellent' | 'good' | 'watch' | 'risk' | 'pending' | 'offline'

export interface SharedPoolServiceGrade {
  code: 'S' | 'A' | 'B' | 'C' | 'OBSERVE' | 'OFFLINE'
  label: string
  tone: SharedPoolServiceTone
}

export interface SharedPoolDecision {
  title: string
  detail: string
  tone: SharedPoolServiceTone
}

const KNOWN_RARITIES = new Set<ZeroCityCardRarity>([
  'common', 'good', 'rare', 'epic', 'legendary', 'mythic', 'legacy',
])

function normalizeRarity(value: string | null | undefined, cardKey: string): ZeroCityCardRarity {
  const normalized = String(value || '').trim().toLowerCase() as ZeroCityCardRarity
  return KNOWN_RARITIES.has(normalized) ? normalized : zeroCityCardCatalogRarity(cardKey)
}

function finitePercent(value: number | null | undefined): number | null {
  const numeric = Number(value)
  if (!Number.isFinite(numeric) || numeric < 0) return null
  const percent = numeric <= 1 ? numeric * 100 : numeric
  return Math.min(100, percent)
}

function effectiveAvailability(input: SharedPoolServicePresentationInput): number | null {
  return finitePercent(input.sevenDayAvailability) ?? finitePercent(input.todayAvailability)
}

export function resolveSharedPoolCardPresentation(
  input: SharedPoolCardPresentationInput,
): SharedPoolCardPresentation | null {
  const key = String(input.cardKey || input.fallbackCardKey || '').trim()
  if (!key) return null

  const rarity = normalizeRarity(input.cardRarity || input.fallbackCardRarity, key)
  const definition = getZeroCityCardDefinition(key)
  const rarityLabel = zeroCityCardRarityDisplayName(rarity)

  return {
    key,
    rarity,
    rarityLabel,
    badgeLabel: `${rarityLabel}收藏卡`,
    name: definition?.name || '零点城收藏卡',
    faction: definition?.faction || '',
    role: definition?.role || '',
    line: definition?.line || '',
    lore: definition?.lore || '',
    imageUrl: zeroCityCardImagePath({ card_key: key, rarity }),
  }
}

export function sharedPoolServiceGrade(input: SharedPoolServicePresentationInput): SharedPoolServiceGrade {
  const status = String(input.status || '').trim().toLowerCase()
  if (status === 'offline' || status === 'maintenance') {
    return { code: 'OFFLINE', label: status === 'maintenance' ? '维护中' : '暂停服务', tone: 'offline' }
  }
  if (input.observation) {
    return { code: 'OBSERVE', label: '数据观察中', tone: 'pending' }
  }

  const availability = effectiveAvailability(input)
  if (input.hasEvidence === false || availability === null) {
    return { code: 'OBSERVE', label: '数据观察中', tone: 'pending' }
  }

  const latency = Number(input.avgLatencyMs)
  const latencyPenalty = Number.isFinite(latency) && latency > 5000 ? 1 : 0
  if (availability >= 99.5 && latencyPenalty === 0) {
    return { code: 'S', label: 'S级稳定', tone: 'excellent' }
  }
  if (availability >= 97 && latencyPenalty === 0) {
    return { code: 'A', label: 'A级良好', tone: 'good' }
  }
  if (availability >= 90) {
    return { code: 'B', label: 'B级波动', tone: 'watch' }
  }
  return { code: 'C', label: 'C级高波动', tone: 'risk' }
}

export function sharedPoolDecision(input: SharedPoolServicePresentationInput): SharedPoolDecision {
  const grade = sharedPoolServiceGrade(input)
  const availability = effectiveAvailability(input)
  const availabilityText = availability === null ? '近期数据不足' : `近7日可用率 ${availability.toFixed(2)}%`

  if (grade.code === 'OFFLINE') {
    return { title: '当前暂不适合加入', detail: '服务已暂停或维护，请等待恢复后再使用。', tone: grade.tone }
  }
  if (grade.code === 'OBSERVE') {
    return { title: '数据仍在观察，建议先等等', detail: '平台正在积累稳定性数据，暂时不要作为唯一主池。', tone: grade.tone }
  }
  if (grade.code === 'S') {
    return { title: '运行稳定，适合日常主力使用', detail: `${availabilityText}，仍建议保留一个备用池。`, tone: grade.tone }
  }
  if (grade.code === 'A') {
    return { title: '整体良好，适合日常使用', detail: `${availabilityText}，重要任务建议保留备用方案。`, tone: grade.tone }
  }
  if (grade.code === 'B') {
    return { title: '近期存在波动，更适合作为备用', detail: `${availabilityText}，不建议承担唯一主力流量。`, tone: grade.tone }
  }
  return { title: '近期波动较大，不建议作为唯一主池', detail: `${availabilityText}，请先查看讨论与检测记录。`, tone: grade.tone }
}
