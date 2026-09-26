import type { ZeroCityWorkspaceKey } from './zeroCityModules'

export type ZeroCitySurfaceKind = 'plaza' | 'workshop' | 'collaboration' | 'activity'

export interface ZeroCitySurfaceDefinition {
  readonly workspace: Exclude<ZeroCityWorkspaceKey, 'home' | 'mine'>
  readonly kind: ZeroCitySurfaceKind
  readonly eyebrow: string
  readonly title: string
  readonly description: string
  readonly primaryLabel: string
  readonly objectLabel: string
  readonly emptyTitle: string
  readonly emptyNote: string
  readonly cues: readonly string[]
}

export const zeroCitySurfaceDefinitions: readonly ZeroCitySurfaceDefinition[] = [
  {
    workspace: 'tavern',
    kind: 'plaza',
    eyebrow: '近况、问题和随手发现',
    title: '闲聊广场',
    description: '留下一条大家可以继续回复的广场帖，也可以从旧话题里接着聊。',
    primaryLabel: '写一条广场帖',
    objectLabel: '话题与回复',
    emptyTitle: '广场还没有真实帖子',
    emptyNote: '登录后可以从一个问题、近况或随手发现开始。',
    cues: ['最新话题', '等待回复', '生活与灵感'],
  },
  {
    workspace: 'workshop',
    kind: 'workshop',
    eyebrow: '问题 / 教程 / 情报',
    title: '技术工坊',
    description: '问题要有可采纳结论，教程要有步骤，情报要标出来源与时间。',
    primaryLabel: '提交技术内容',
    objectLabel: '可复用结论',
    emptyTitle: '这个工位还没有内容',
    emptyNote: '发布真实问题、可复现教程或带日期的情报。',
    cues: ['答疑可采纳', '教程强调步骤', '情报强调来源'],
  },
  {
    workspace: 'market',
    kind: 'collaboration',
    eyebrow: '共创进度与寻找伙伴',
    title: '协作交流',
    description: '找搭档，分享进展，一起把事情做完。作品和交付记录仍然回到原始项目查看。',
    primaryLabel: '发起协作讨论',
    objectLabel: '协作线索',
    emptyTitle: '还没有可跟进的协作线索',
    emptyNote: '可以说明正在做什么、缺少什么，实际委托请进入正式业务模块。',
    cues: ['找搭档', '分享进展', '成果复盘'],
  },
  {
    workspace: 'governance',
    kind: 'activity',
    eyebrow: '活动 / 公示 / 反馈',
    title: '城市活动',
    description: '先看活动时间与参与方式，再查看公示、结果与处理中的反馈。',
    primaryLabel: '发布活动或反馈',
    objectLabel: '活动与公示',
    emptyTitle: '暂无进行中的活动',
    emptyNote: '可以查看公示与过往结果，或留下一条城市反馈。',
    cues: ['进行中', '公示与结果', '反馈进度'],
  },
]

export function getZeroCitySurface(workspace: ZeroCityWorkspaceKey): ZeroCitySurfaceDefinition | undefined {
  return zeroCitySurfaceDefinitions.find((item) => item.workspace === workspace)
}
