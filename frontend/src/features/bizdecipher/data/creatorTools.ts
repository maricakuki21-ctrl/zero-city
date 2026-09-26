export const CREATOR_RESOURCE_SOURCES = ['all', 'official', 'shared-pool', 'creator'] as const

export type CreatorResourceSource = Exclude<(typeof CREATOR_RESOURCE_SOURCES)[number], 'all'>
export type CreatorResourceKind = 'model' | 'tool' | 'workflow' | 'template'
export type CreatorResourceBilling = 'official' | 'free' | 'points' | 'balance' | 'hybrid'

export type CreatorToolResource = {
  readonly id: string
  readonly title: string
  readonly subtitle: string
  readonly source: CreatorResourceSource
  readonly sourceLabel: string
  readonly kind: CreatorResourceKind
  readonly kindLabel: string
  readonly summary: string
  readonly billing: CreatorResourceBilling
  readonly billingLabel: string
  readonly ownerLabel: string
  readonly evidenceLabel: string
  readonly linkedCapabilityId?: string
  readonly tags: readonly string[]
}

export const creatorToolResources: readonly CreatorToolResource[] = [
  {
    id: 'official-language-planner',
    title: '官方 · 语言规划器',
    subtitle: '目标拆解、提示词与执行计划',
    source: 'official',
    sourceLabel: '官方资源',
    kind: 'model',
    kindLabel: '语言模型',
    summary: '把自然语言目标整理成可执行步骤，再交给图片、视频、代码或工作流能力执行。',
    billing: 'official',
    billingLabel: '按官方报价',
    ownerLabel: 'BizDecipher 官方',
    evidenceLabel: '接入状态以工作区能力合同为准',
    tags: ['规划', '语言模型', '官方'],
  },
  {
    id: 'official-image-direction',
    title: '官方 · 图片方向',
    subtitle: '构图、风格与电商视觉规划',
    source: 'official',
    sourceLabel: '官方资源',
    kind: 'workflow',
    kindLabel: '工作流',
    summary: '适合先确定图片方向、尺寸、画面层级，再交给图像模型生成或交付。',
    billing: 'official',
    billingLabel: '按官方报价',
    ownerLabel: 'BizDecipher 官方',
    evidenceLabel: '等待工作区能力映射',
    tags: ['图片', '电商', '工作流'],
  },
  {
    id: 'shared-pool-media-route',
    title: '共享池 · 媒体路由',
    subtitle: '共享池中的图片与视频模型组合',
    source: 'shared-pool',
    sourceLabel: '共享池',
    kind: 'model',
    kindLabel: '模型组合',
    summary: '按共享池当前的可用模型、倍率与实时报价选择媒体执行路径。',
    billing: 'balance',
    billingLabel: '余额结算',
    ownerLabel: '由池主维护',
    evidenceLabel: '价格与可用性需要服务端实时返回',
    tags: ['共享池', '图片', '视频'],
  },
  {
    id: 'shared-pool-code-route',
    title: '共享池 · 代码执行',
    subtitle: '代码生成、检查与打包',
    source: 'shared-pool',
    sourceLabel: '共享池',
    kind: 'model',
    kindLabel: '代码模型',
    summary: '让语言模型负责规格和验收，再把实现、测试与打包交给代码执行能力。',
    billing: 'hybrid',
    billingLabel: '积分或余额',
    ownerLabel: '由池主维护',
    evidenceLabel: '未连接时只保存选择偏好',
    tags: ['共享池', '代码', '验收'],
  },
  {
    id: 'creator-commerce-kit',
    title: '示例 · 电商视觉套件',
    subtitle: '主图、卖点图与详情页工作流',
    source: 'creator',
    sourceLabel: '创作者工具',
    kind: 'workflow',
    kindLabel: '工作流',
    summary: '创作者发布的可复用工作流，包含输入字段、图像步骤、文案步骤和交付模板。',
    billing: 'points',
    billingLabel: '示例计价 · 积分',
    ownerLabel: '作者可自行定价',
    evidenceLabel: '待接入真实调用与分成账本',
    tags: ['创作者', '电商', '图片'],
  },
  {
    id: 'creator-storyboard-kit',
    title: '示例 · 短视频分镜包',
    subtitle: '主题拆解、镜头、字幕与配音清单',
    source: 'creator',
    sourceLabel: '创作者工具',
    kind: 'tool',
    kindLabel: '技能工具',
    summary: '以结构化分镜作为输出，方便继续交给视频模型，也方便人工修改和复用。',
    billing: 'free',
    billingLabel: '示例计价 · 免费',
    ownerLabel: '作者开放使用',
    evidenceLabel: '暂无真实使用数据',
    tags: ['创作者', '视频', '技能'],
  },
  {
    id: 'creator-tavern-game-kit',
    title: '示例 · 酒馆剧本引擎',
    subtitle: '角色卡、线索、回合与结局状态',
    source: 'creator',
    sourceLabel: '创作者工具',
    kind: 'template',
    kindLabel: '游戏模板',
    summary: '为酒馆里的剧本杀、跑团和互动故事准备可导入的状态结构与主持流程。',
    billing: 'hybrid',
    billingLabel: '积分或余额',
    ownerLabel: '作者可自行定价',
    evidenceLabel: '游戏运行需要平台网关托管 Key',
    tags: ['创作者', '游戏', '酒馆'],
  },
] as const

export function filterCreatorToolResources(
  resources: readonly CreatorToolResource[],
  query: string,
  source: (typeof CREATOR_RESOURCE_SOURCES)[number],
  mode: string,
): readonly CreatorToolResource[] {
  const normalizedQuery = query.trim().toLocaleLowerCase()
  return resources.filter((resource) => {
    const sourceMatches = source === 'all' || resource.source === source
    const modeMatches = mode === 'general'
      || resource.tags.some((tag) => tag.includes(mode === 'commerce' ? '电商' : mode === 'image' ? '图片' : mode === 'video' ? '视频' : mode === 'game' ? '游戏' : mode === 'code' ? '代码' : ''))
    const queryMatches = !normalizedQuery || [resource.title, resource.subtitle, resource.summary, resource.sourceLabel, ...resource.tags]
      .some((value) => value.toLocaleLowerCase().includes(normalizedQuery))
    return sourceMatches && modeMatches && queryMatches
  })
}

export function resourceBillingTone(billing: CreatorResourceBilling): 'official' | 'points' | 'balance' | 'free' {
  if (billing === 'official') return 'official'
  if (billing === 'points') return 'points'
  if (billing === 'balance' || billing === 'hybrid') return 'balance'
  return 'free'
}
