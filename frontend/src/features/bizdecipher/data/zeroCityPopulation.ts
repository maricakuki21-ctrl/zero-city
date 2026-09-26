import {
  ZERO_CITY_CARD_MANIFEST,
  zeroCityCardManifestEntries,
  type ZeroCityCardKey,
  type ZeroCityCardRarity,
} from '@/constants/zeroCityCardManifest'
import type { ChronicleDistrict } from './zeroCityChronicle'

export type ZeroCityFaction = (typeof ZERO_CITY_CARD_MANIFEST)[ZeroCityCardKey]['faction']
export type ZeroCityNarrativeLayer = 'main' | 'recurring' | 'resident' | 'myth' | 'legacy'

export interface ZeroCityFactionProfile {
  district: ChronicleDistrict
  duty: string
  pressure: string
}

export interface ZeroCityCharacterAtlasEntry {
  key: ZeroCityCardKey
  name: string
  faction: ZeroCityFaction
  district: ChronicleDistrict
  role: string
  line: string
  duty: string
  pressure: string
  rarity: ZeroCityCardRarity
  narrativeLayer: ZeroCityNarrativeLayer
}

/**
 * The city has many named desks and societies, but only five physical districts.
 * Every collectible identity must inherit a home, a public duty and a conflict.
 */
export const zeroCityFactionProfiles = {
  微光供电所: { district: 'power', duty: '分配城市的有限电量，并保留交班记录。', pressure: '每一次多留一格电，都意味着另一个地方先暗下去。' },
  微光仪表盘: { district: 'power', duty: '播报剩余力气，不把临界值误写成归零。', pressure: '数字越精确，人们越容易忘记数字后面是正在生活的人。' },
  微光物流: { district: 'power', duty: '把小型光源和备用部件送到仍有人等待的地方。', pressure: '运力永远不够，配送顺序本身就是一次选择。' },
  开机广场: { district: 'power', duty: '帮助停摆的设备和居民重新开始。', pressure: '城市崇拜快速启动，却没人愿意承认有些人需要更久。' },
  等待研究院: { district: 'power', duty: '记录加载、排队和尚未完成的过程。', pressure: '等待一旦太久，研究就会被怀疑只是在替拖延找理由。' },
  日期缓冲区: { district: 'power', duty: '保管没有按时完成、但仍值得继续的日期。', pressure: '给今天留退路，可能会把代价全部推给明天。' },
  时间边界处: { district: 'power', duty: '协调截止线、延期和最后一刻的缓冲。', pressure: '每一次宽限都可能救一个人，也可能拖累另一个人的承诺。' },
  一厘米赛道: { district: 'power', duty: '确认微小进度确实发生过。', pressure: '鼓励小步不能变成拒绝面对真正终点的借口。' },
  勇气排队口: { district: 'power', duty: '让临阵退缩的人仍能重新排队。', pressure: '队伍允许重来，却不能保证机会会一直等在原地。' },
  久坐森林: { district: 'power', duty: '照看与座椅、屏幕和工作绑定太久的人。', pressure: '休息和停滞看起来太像，连本人也常常分不清。' },
  横向研究所: { district: 'power', duty: '从非标准姿势观察城市遗漏的出口。', pressure: '换个角度可能发现真相，也可能只是避开必须站起来的时刻。' },

  临期希望商店: { district: 'shop', duty: '保存快过期、仍然能用的勇气与希望。', pressure: '商店越擅长救急，值夜的人越难真正关门。' },
  好运工单中心: { district: 'shop', duty: '受理那些无法承诺、但值得尝试的小请求。', pressure: '把偶然写成流程以后，人们会开始要求奇迹按时交付。' },
  微型财政署: { district: 'shop', duty: '替零碎价值、小额快乐和边角收益记账。', pressure: '承认小价值不能变成替不公平分配粉饰太平。' },
  夜间财务所: { district: 'shop', duty: '记录深夜发生的临时支出和情绪账。', pressure: '被解释为必要的支出，仍然可能在天亮后付出代价。' },
  快乐估值所: { district: 'shop', duty: '反对给快乐设过高的起步价。', pressure: '所有东西都被估值以后，快乐也可能只剩一个价格。' },
  温度管理局: { district: 'shop', duty: '维护热水、炉火和能够让人停留的温度。', pressure: '照顾眼前的冷，不能代替解决让人一直受冻的原因。' },
  热水神殿: { district: 'shop', duty: '把三分钟的等待变成可以喘息的仪式。', pressure: '仪式可以让世界软一点，却不能假装问题已经消失。' },
  入口检票口: { district: 'shop', duty: '给意外、惊喜和新来者一个正式入口。', pressure: '检查能保护城市，也可能把没有证明的人永远挡在门外。' },
  期待窗口处: { district: 'shop', duty: '调节期待的亮度和开放范围。', pressure: '降低期待能避免受伤，也可能让人不再敢真正想要什么。' },
  反话事务所: { district: 'shop', duty: '保管那些不敢直接说出口的愿望。', pressure: '反话说久了，连最亲近的人也会把真心当成玩笑。' },
  常识公园: { district: 'shop', duty: '让理性和冲动在公共空间里同时出现。', pressure: '常识能防止失控，也最容易被拿来嘲笑尚未验证的可能。' },
  不安花园: { district: 'shop', duty: '把无处安放的焦虑移到可观察的位置。', pressure: '照看不安不等于让它无限生长。' },
  软弱防线: { district: 'shop', duty: '提供短暂撤退和恢复体力的安全区。', pressure: '堡垒保护人，也可能让里面的人再也不愿打开门。' },
  体面剧团: { district: 'shop', duty: '在人还没准备好时借给他一副暂时能用的表情。', pressure: '表演能撑过现场，却可能让真正的崩溃无人看见。' },

  天气维护队: { district: 'workshop', duty: '修补会漏进生活里的天气裂缝。', pressure: '修补得越漂亮，越可能掩盖裂缝为什么出现。' },
  失败回路处: { district: 'workshop', duty: '保管失败记录、重试入口和可复用零件。', pressure: '鼓励重试不能变成强迫每个人无止境地再来一次。' },
  情绪保洁署: { district: 'workshop', duty: '清理最妨碍行动的情绪碎片。', pressure: '把地面扫干净，不能把人的情绪也当作垃圾。' },
  信号补丁站: { district: 'workshop', duty: '修复短暂掉线和断裂连接。', pressure: '重新上线不代表离线期间发生的事可以被跳过。' },
  标签草原: { district: 'workshop', duty: '管理过量打开、四处跑散的可能性。', pressure: '整理标签能恢复秩序，也可能过早关掉唯一有用的那一页。' },
  漂流专注河: { district: 'workshop', duty: '打捞走神以后仍然值得保留的灵感。', pressure: '把注意力找回来，不代表它还愿意回到原来的任务。' },
  心跳调音台: { district: 'workshop', duty: '降低过强刺激，让希望仍能在背景里工作。', pressure: '降噪过度会把真正的危险也一起静音。' },
  灰度奇迹实验室: { district: 'workshop', duty: '小范围验证那些暂时无法稳定复现的好事。', pressure: '人们急着需要结果时，实验最容易越过证据边界。' },
  冲动动物园: { district: 'workshop', duty: '观察并约束会伸手碰按钮的冲动。', pressure: '安全围栏保护人，也会让探索只剩围观。' },

  蓝时电台: { district: 'harbour', duty: '接收迟到的回应，并为沉默保留空白。', pressure: '不替沉默下结论，也意味着必须承受长时间的不确定。' },
  零点海岸: { district: 'harbour', duty: '给尚未准备好回来的人保留方向。', pressure: '灯一直亮着，会让守灯的人误以为自己永远不能离开。' },
  迟到抵达处: { district: 'harbour', duty: '登记没有准时、但确实抵达的人。', pressure: '承认迟到的价值，不能抹掉等待者实际付出的时间。' },
  黑日值班室: { district: 'harbour', duty: '在城市最暗的时段确认至少还有一盏灯。', pressure: '长期凝视黑暗的人，最容易忘记自己也需要被看见。' },
  梦境客服台: { district: 'harbour', duty: '记录刚醒时出现、尚未被现实验证的答案。', pressure: '梦能提供线索，也能成为逃避证据的便利借口。' },
  偏航小队: { district: 'harbour', duty: '在主线堵死时寻找可行动的支路。', pressure: '支线能救人，也可能让最重要的问题永远没人回来处理。' },
  情绪天气港: { district: 'harbour', duty: '护送处在局部风暴中的人靠岸。', pressure: '小风暴同样值得救援，但港口不可能同时接住所有船。' },
  深水奖池: { district: 'harbour', duty: '打捞被沉到概率底部的微小反光。', pressure: '追逐深处的光最容易让人忘记自己已经下潜多远。' },
  消息缓冲站: { district: 'harbour', duty: '给尚未发送的回复留出孵化时间。', pressure: '等待更好的措辞，可能最终变成永远不回复。' },
  云端合租屋: { district: 'harbour', duty: '给暂时没有根基的人提供短期住处。', pressure: '允许漂泊不能变成随时被赶走的另一种不稳定。' },

  暮色地图局: { district: 'archive', duty: '标记可走的路和曾经无法通过的边界。', pressure: '地图为了清晰删掉的细节，往往正是某个人全部的经历。' },
  杂乱档案馆: { district: 'archive', duty: '保存未分类、未确认和互相矛盾的证据。', pressure: '什么都保存会淹没真相，替证据排序又会制造新的偏见。' },
  支线博物馆: { district: 'archive', duty: '收藏最低点以后意外打开的门。', pressure: '人们开始期待隐藏剧情时，会轻视真实发生过的坏事。' },
  擦肩而过博物馆: { district: 'archive', duty: '保存差一点命中的入口与未完成结果。', pressure: '纪念差一点，不能把真正的失败改写成胜利。' },
  概率管理局: { district: 'archive', duty: '记录概率规则、异常和被临时绕开的边界。', pressure: '维护规则的人最清楚规则何时不公平，也最难承认自己曾经放行。' },
  概率动物园: { district: 'archive', duty: '观察无法被命令、只能被耐心接近的概率。', pressure: '驯服的幻觉会让人低估随机性真正的野性。' },
  编号王冠厅: { district: 'archive', duty: '保管坚持者的编号和早期城市身份。', pressure: '编号证明一个人来过，也可能慢慢变成排斥后来者的门槛。' },
  镜湖后台: { district: 'archive', duty: '维护城市表面与底层记录之间的映照。', pressure: '湖面越平静，越可能有人正在水下改写结构。' },
  红点寺: { district: 'archive', duty: '区分真正需要回应的通知和制造焦虑的噪音。', pressure: '降低噪音时，最安静的求救也可能被一并忽略。' },
  零点核心: { district: 'archive', duty: '保管重启以前留下的备份和不可验证的原始记录。', pressure: '完整备份也许能恢复城市，却未必能恢复同一个人。' },
  旧档案盒: { district: 'archive', duty: '保存早期版本的门票、邮戳、证书与遗物。', pressure: '尊重历史不能让旧版本永久支配仍在生活的城市。' },
} as const satisfies Record<ZeroCityFaction, ZeroCityFactionProfile>

export const zeroCityMainCastKeys = [
  'low_battery_sprite',
  'hope_night_shift',
  'retry_button_keeper',
  'cloud_patch_worker',
  'blue_hour_operator',
  'zero_hour_lighthouse_keeper',
  'afterglow_cartographer',
  'desktop_dust_archivist',
] as const satisfies readonly ZeroCityCardKey[]

const mainCast = new Set<ZeroCityCardKey>(zeroCityMainCastKeys)

function narrativeLayer(key: ZeroCityCardKey, rarity: ZeroCityCardRarity, legacy: boolean): ZeroCityNarrativeLayer {
  if (mainCast.has(key)) return 'main'
  if (legacy) return 'legacy'
  if (rarity === 'mythic') return 'myth'
  if (rarity === 'rare' || rarity === 'epic' || rarity === 'legendary') return 'recurring'
  return 'resident'
}

export const zeroCityCharacterAtlas: readonly ZeroCityCharacterAtlasEntry[] = zeroCityCardManifestEntries().map((entry) => {
  const key = entry.key as ZeroCityCardKey
  const faction = entry.definition.faction as ZeroCityFaction
  const profile = zeroCityFactionProfiles[faction]
  return {
    key,
    name: entry.definition.name,
    faction,
    district: profile.district,
    role: entry.definition.role,
    line: entry.definition.line,
    duty: profile.duty,
    pressure: profile.pressure,
    rarity: entry.rarity,
    narrativeLayer: narrativeLayer(key, entry.rarity, entry.legacy),
  }
})

export function zeroCityCharacterEntry(key: string | undefined): ZeroCityCharacterAtlasEntry | undefined {
  return zeroCityCharacterAtlas.find(entry => entry.key === key)
}

export function zeroCityCharactersInDistrict(district: ChronicleDistrict): ZeroCityCharacterAtlasEntry[] {
  return zeroCityCharacterAtlas.filter(entry => entry.district === district)
}
