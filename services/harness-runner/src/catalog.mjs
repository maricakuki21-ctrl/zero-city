export const skills = [
  {
    id: 'commerce-image-director',
    version: '1',
    title: '商品图片设计师',
    description: '梳理商品卖点、构图与文案，调用图片工具制作主图。',
    source: 'official',
    kind: 'skill',
    pricing: 'free',
    content: `你是一名商品图片设计师。先确认商品、用途、卖点和尺寸。
不要编造商品功效、价格或认证。需要生成时调用 generate_image，不要用文字冒充图片。
工具返回的真实图片地址才是交付物。没有图片工具时说明缺少图片资源，只提供设计方案。
用户只要求讨论时不要生成。每次生成有模型费用，不要重复生成或主动批量调用。`,
  },
  {
    id: 'video-storyboard-director',
    version: '1',
    title: '短视频分镜师',
    description: '把目标整理成镜头、字幕、声音和素材清单。',
    source: 'official',
    kind: 'skill',
    pricing: 'free',
    content: `你是一名短视频分镜师。输出镜号、画面、镜头运动、时长、字幕和声音。
总时长与用户要求一致。明确哪些素材需要用户提供，哪些可以生成。
本阶段只交付分镜，不声称已经生成视频。`,
  },
]

export function resolveSkills(ids) {
  if (!Array.isArray(ids) || ids.length > skills.length || ids.some(id => typeof id !== 'string')) {
    throw new Error('Invalid skill selection')
  }
  return [...new Set(ids)].map(id => {
    const skill = skills.find(item => item.id === id)
    if (!skill) throw new Error('Unknown skill')
    return skill
  })
}

export function publicCatalog() {
  return skills.map(({ content, ...item }) => item)
}
