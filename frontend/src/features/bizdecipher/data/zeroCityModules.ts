export type ZeroCityWorkspaceKey = 'home' | 'tavern' | 'workshop' | 'market' | 'governance' | 'mine'

export const zeroCityWorkspaceKeys = [
  'home',
  'tavern',
  'workshop',
  'market',
  'governance',
  'mine',
] as const satisfies readonly ZeroCityWorkspaceKey[]

export type ZeroCityModuleKey = 'home' | 'tavern' | 'forum' | 'market' | 'mine'

export type ZeroCityWorkflowOwnership = 'community-only' | 'deep-link-only'

export type ZeroCityModuleDefinition = {
  readonly key: ZeroCityModuleKey
  readonly entryWorkspace: ZeroCityWorkspaceKey
  readonly workspaces: readonly ZeroCityWorkspaceKey[]
  readonly mark: string
  readonly label: string
  readonly note: string
  readonly purpose: string
  readonly scene: string
  readonly primaryObject: string
  readonly primaryAction: string
  readonly returnTrigger: string
  readonly shareTrigger: string
  readonly boundary: string
  readonly workflowOwnership: ZeroCityWorkflowOwnership
  readonly secondarySurfaces: readonly string[]
}

export const zeroCityModuleDefinitions = [
  {
    key: 'home',
    entryWorkspace: 'home',
    workspaces: ['home'],
    mark: '城',
    label: '全城动态',
    note: '看看现在',
    purpose: '让居民快速知道城里正在发生什么，并找到一件值得参与的事。',
    scene: '进城后的第一眼，以及想确认新变化、回复和官方信号时。',
    primaryObject: '动态、问题、回应与证据摘要',
    primaryAction: '浏览、发布、回应',
    returnTrigger: '关注话题的新回应、被采纳结论和城市事件进展。',
    shareTrigger: '可独立传播的问题结论、证据摘要和事件进展。',
    boundary: '只呈现公共讨论和业务结果摘要，不承载业务表单、交付或结算流程。',
    workflowOwnership: 'community-only',
    secondarySurfaces: ['城市排行榜', '城市档案'],
  },
  {
    key: 'tavern',
    entryWorkspace: 'tavern',
    workspaces: ['tavern'],
    mark: '酒',
    label: '酒馆',
    note: '水聊和房间',
    purpose: '用低门槛的即时交流、房间和城市故事建立熟悉感。',
    scene: '想随便聊聊、围观现场、加入一段临时话题或故事时。',
    primaryObject: '房间、现场话题和城市故事',
    primaryAction: '进入、聊天、主持',
    returnTrigger: '房间续场、故事推进和熟人再次出现。',
    shareTrigger: '有场景感的片段、金句和活动纪念卡。',
    boundary: '娱乐和叙事行为不得改变真实资产、权限、财务或业务状态。',
    workflowOwnership: 'community-only',
    secondarySurfaces: ['小游戏', '每日事件'],
  },
  {
    key: 'forum',
    entryWorkspace: 'workshop',
    workspaces: ['workshop', 'governance'],
    mark: '论',
    label: '论坛',
    note: '讨论和共建',
    purpose: '沉淀可查找、可复用、可形成城市共识的长期讨论。',
    scene: '需要提问、答疑、复盘、提交提案或共同决定城市规则时。',
    primaryObject: '主题、回答、提案和证据',
    primaryAction: '提问、回答、采纳、提案',
    returnTrigger: '新答案、被采纳结果、官方回应和提案进展。',
    shareTrigger: '经验证的解决方案、清晰立场和共建结论。',
    boundary: '讨论可以指向业务模块，但不在论坛内完成资产生产、交付或结算。',
    workflowOwnership: 'community-only',
    secondarySurfaces: ['可信讨论榜', '共建记录'],
  },
  {
    key: 'market',
    entryWorkspace: 'market',
    workspaces: ['market'],
    mark: '协',
    label: '协作集市',
    note: '能力和协作',
    purpose: '帮助居民发现可靠能力、真实需求和已经发生的交付。',
    scene: '想找人、找能力、找需求，或判断一项协作是否值得继续时。',
    primaryObject: '能力、需求和交付证明预览',
    primaryAction: '浏览、匹配、进入正式协作',
    returnTrigger: '匹配反馈、协作状态变化和新的可信案例。',
    shareTrigger: '可验证的能力案例、需求卡和交付证明。',
    boundary: '只负责发现和可信预览；合同、执行、验收、结算必须进入正式业务模块。',
    workflowOwnership: 'deep-link-only',
    secondarySurfaces: ['能力榜', '交付案例'],
  },
  {
    key: 'mine',
    entryWorkspace: 'mine',
    workspaces: ['mine'],
    mark: '我',
    label: '我的零号城',
    note: '资料和作品',
    purpose: '把居民身份、贡献、作品和城市记忆组织成可持续积累的个人空间。',
    scene: '查看自己的成长、整理公开身份，或回顾在城里留下的记录时。',
    primaryObject: '居民档案、贡献、作品与收藏卡',
    primaryAction: '查看、整理、展示',
    returnTrigger: '成长进度、回应通知、贡献确认和新收藏。',
    shareTrigger: '有辨识度的居民主页、作品和纪念卡。',
    boundary: '这里只展示可信投影，不作为资金、权限或业务状态的事实来源。',
    workflowOwnership: 'community-only',
    secondarySurfaces: ['收藏卡册', '个人档案'],
  },
] as const satisfies readonly ZeroCityModuleDefinition[]

export function isZeroCityModuleActive(
  module: ZeroCityModuleDefinition,
  workspace: ZeroCityWorkspaceKey,
): boolean {
  return module.workspaces.includes(workspace)
}
