import {
  ZERO_CITY_CARD_MANIFEST,
  zeroCityCardCatalogRarity,
  zeroCityCardThumbnailPath,
  type ZeroCityCardKey,
} from '@/constants/zeroCityCardManifest'

export const CHRONICLE_EDITION = '第一卷 · 留灯的人'
export type ChronicleEra = 'before' | 'lamplight' | 'bridge' | 'open'
export type ChronicleDistrict = 'power' | 'shop' | 'workshop' | 'harbour' | 'archive'

export const chronicleEras: ReadonlyArray<{ id: ChronicleEra; name: string; years: string; note: string }> = [
  { id: 'before', name: '断光以前', years: '零历前夜', note: '有人先把真相折起来，城市才有了第一张完整的地图。' },
  { id: 'lamplight', name: '留灯年代', years: '零历 01—02 年', note: '只剩 3% 的时候，最难分清的是电和责任。' },
  { id: 'bridge', name: '渡桥年代', years: '零历 03 年', note: '桥不是把两岸接上，而是让人有地方承认自己走不动。' },
  { id: 'open', name: '开城以后', years: '零历 04 年', note: '门打开以后，救场的人终于得面对自己的生活。' },
]

export const chronicleDistricts: ReadonlyArray<{
  id: ChronicleDistrict; name: string; address: string; detail: string; everyday: string; landmark: string
}> = [
  { id: 'power', name: '微光街', address: '东岸 · 旧供电环线', detail: '街灯不是同时亮起的。每一盏旁边都有一张手写维修单，写着谁还在等它。微光供电所的门一直开着，里面没有“电量不足者禁止入内”的牌子。', everyday: '黄昏后，巡逻员挨家确认灯罩。住户可以说今天不想亮灯，他会在本子上写“已休息”，而不是“故障”。', landmark: '永远差一格的电量钟' },
  { id: 'shop', name: '未打烊巷', address: '城心 · 临期希望商店后街', detail: '巷口原本只有一家便利店。后来出现了热水桌、修伞摊和一个没有最低消费的酒馆。关门的店会把凳子留在外面，方便晚到的人坐一会儿。', everyday: '夜班员每天擦两遍窗。第一遍擦灰，第二遍擦掉别人误以为这里已经打烊的倒影。', landmark: '门牌写着“今日仍然营业”的小店' },
  { id: 'workshop', name: '重试工坊', address: '北坡 · 失败回路处', detail: '这里收留没做完的机器，也收留不愿解释为什么没做完的人。墙上展示修好的东西，抽屉里认真保存修坏过的零件。天空维修梯就停在窗外。', everyday: '每天清晨，重试按钮看守给按钮擦灰。云端修补匠修完屋顶，会把剩下的布料裁成邻居雨衣上的小口袋。', landmark: '没有被拆掉的第一架维修梯' },
  { id: 'harbour', name: '蓝时海岸', address: '西岸 · 电台与零点灯塔之间', detail: '海面漂着被划掉的日历纸。电台记录每一封没有及时抵达的信，灯塔照向岸边而不是远方：它首先要让还在水里的人看见台阶。', everyday: '信号员在蓝时校准天线，守灯人负责送饭。两人约好，接不到回应的时候也不能擅自替对方宣布结局。', landmark: '只向回家方向亮起的灯塔' },
  { id: 'archive', name: '余晖档案街', address: '南坡 · 暮色地图局旁', detail: '地图局和档案馆隔着一张很长的桌子。地图标记可走的路，档案标记当时为什么没有走过去。两种记录不互相纠正。', everyday: '制图师午后出门测路，档案员在桌边收拾便签。桌上有一只空盒，专门装尚未想好如何命名的事情。', landmark: '始终给后来者留白的城市地图' },
]

export interface ChronicleResident {
  key: ZeroCityCardKey
  district: ChronicleDistrict
  home: string
  routine: string
  wish: string
  fear: string
  secret: string
  relationship: { key: ZeroCityCardKey; detail: string }
}

export const chronicleResidents: readonly ChronicleResident[] = [
  {
    key: 'low_battery_sprite', district: 'power', home: '微光供电所楼下，第三间小值班室',
    routine: '每天只巡一段街，交班前却总会绕去旧备用环线，确认凌晨 00:03 那盏红灯还会不会亮。',
    wish: '查清最后 3% 的电量是谁改过，又究竟送到了谁手里。',
    fear: '害怕自己一旦不再有用，就不会再有人等他回来。',
    secret: '断光那晚，他故意把计量盘多报了 0.7%，才为旧回路留下三秒钟。',
    relationship: { key: 'hope_night_shift', detail: '他欠夜班员一个真正的休息日，也欠他在值班表上写回真名。' },
  },
  {
    key: 'hope_night_shift', district: 'shop', home: '临期希望商店的阁楼，窗正对巷口',
    routine: '每晚关门后等三分钟；如果有人敲门，就假装自己只是忘了关灯。',
    wish: '想把店彻底关上一晚，同时确认没有人会因为他休息而被留在门外。',
    fear: '害怕灯一旦熄灭，大家就会发现所谓希望只是他一个人硬撑出来的。',
    secret: '最后 3% 事件的正式值班表是他伪造的；被烧掉的一页一直藏在阁楼地板下。',
    relationship: { key: 'low_battery_sprite', detail: '他把钥匙交给巡逻员，又在城里开始责怪巡逻员时保持了沉默。' },
  },
  {
    key: 'retry_button_keeper', district: 'workshop', home: '失败回路处一层，靠近公共工作桌',
    routine: '给按钮擦灰、登记故障，也记录每一个站在按钮前却决定不按的人。',
    wish: '弄清墙上的按钮究竟会救回谁，而不是永远替别人保管这个选择。',
    fear: '害怕所有人终有一天都看着他，要求他按下那枚可能毁掉城市的按钮。',
    secret: '按钮第一次自行亮起以前，他已经悄悄拔掉了安全锁；他至今不知道两件事是否有关。',
    relationship: { key: 'cloud_patch_worker', detail: '他曾漏检那枚软金属铆钉。修补匠没有揭发他，只要求他以后别再把“能运行”叫作“修好了”。' },
  },
  {
    key: 'cloud_patch_worker', district: 'workshop', home: '工坊顶楼，屋檐上晾着不同颜色的补丁',
    routine: '先补会漏到床边的洞，再把每块旧补丁掀起一角，确认下面的天空还和昨天一样。',
    wish: '找到一种不会顺手抹掉过去的修补方法。',
    fear: '害怕自己修得越好，城市忘掉的人就越多。',
    secret: '他早就知道天空补丁下面藏着被剪掉的录音，却一直把那当成修复工作的正常损耗。',
    relationship: { key: 'retry_button_keeper', detail: '他替看守隐瞒漏检的铆钉，看守则替他保留了第一块会说话的天空碎片。' },
  },
  {
    key: 'blue_hour_operator', district: 'harbour', home: '蓝时电台的屋顶小屋，天线旁第二扇窗',
    routine: '整理迟到的消息，不替沉默编答案；每次公共广播结束后，会多发一遍自己的私人呼号。',
    wish: '收到一封不需要代转、只写给自己的回信。',
    fear: '害怕有一天把真正的告别误写成“信号仍在路上”。',
    secret: '第八夜听到的第一个名字其实是他的名字；他在公开档案里删掉了这一句。',
    relationship: { key: 'zero_hour_lighthouse_keeper', detail: '守灯人替他藏过私人呼号，他则必须决定是否原谅守灯人藏起整座城的第一封回信。' },
  },
  {
    key: 'zero_hour_lighthouse_keeper', district: 'harbour', home: '零点灯塔下的小厨房，炉边有一张折叠床',
    routine: '天黑前擦灯，零点前煮汤；大雾时把光压得很低，只照旧码头最后一级台阶。',
    wish: '在某个晴天睡过零点，不再靠“总有人需要我”证明自己应该留在这里。',
    fear: '害怕桥真的修好以后，灯塔和自己都变得多余。',
    secret: '断光后的第一夜他就收到过对岸回信，却把录音藏了两年。',
    relationship: { key: 'blue_hour_operator', detail: '他欠信号员那两年的真相，也欠对岸一个没有被自己替他们决定的回答。' },
  },
  {
    key: 'afterglow_cartographer', district: 'archive', home: '暮色地图局后院，一间朝西的窄屋',
    routine: '走错路后记下沿途的店、台阶和封条；只在傍晚画被官方擦掉的部分。',
    wish: '把那条被删掉的路画到真正的终点，让地图第一次承认自己曾经撒谎。',
    fear: '害怕自己最后只剩一张没人敢打开的退稿。',
    secret: '第一张红线地图不是他发现的；原图上被刮掉的签名和他的笔迹一模一样。',
    relationship: { key: 'desktop_dust_archivist', detail: '档案员替他藏住原图，他则承诺公开时不会把全部责任推给那个保管证据的人。' },
  },
  {
    key: 'desktop_dust_archivist', district: 'archive', home: '杂乱档案馆二层，编号还没贴完的小房间',
    routine: '给碎纸编号，不替写字的人改结论；每周留一天不整理，让新证据有地方落下。',
    wish: '让“尚未完成”和“仍有争议”成为正式分类，而不是被系统自动清理。',
    fear: '害怕真相公开以后，受伤的人先来找保管证据的人算账。',
    secret: '十二张维修单里有十一张曾被他亲手归为“不存在”；他只来得及偷回最后一张。',
    relationship: { key: 'afterglow_cartographer', detail: '制图师以为他只是胆小；实际上，档案员一直在等一个愿意和他共同署名的人。' },
  },
]

export interface ChronicleEvent {
  id: string
  era: ChronicleEra
  date: string
  title: string
  summary: string
  district: ChronicleDistrict
  residents: readonly ZeroCityCardKey[]
  paragraphs: readonly string[]
  trace: string
}

export const chronicleEvents: readonly ChronicleEvent[] = [
  {
    id: 'the-unfinished-map', era: 'before', date: '零历前夜 · 傍晚', title: '地图上没有的一条路',
    summary: '地图局把一条路删成了空白。余晖制图师偏要沿着空白走一遍，结果在尽头看见一盏本不该亮的红灯。',
    district: 'archive', residents: ['afterglow_cartographer', 'desktop_dust_archivist'],
    paragraphs: [
      '断光前一晚，余晖制图师在地图局的废纸篓里捡到一张被撕掉一半的图。图上没有街名，只有一条红线绕过东岸，落在“旧备用环线”。地图局的规矩是：走不通的路不配有名字。制图师知道这条规矩，因为他曾经把一条被洪水冲断的路画上去，也因此丢过一次资格。',
      '他拿图去找桌面灰尘档案员。档案员先说不知道，随后从编号未完的抽屉里取出十二张被撕过的维修单。每一张都有同一个时间：凌晨 00:03。档案员说，若把它们归档，地图局会把“未核实”改成“从未发生”。制图师想把证据带走；档案员不让——离开档案馆的纸，最容易被说成伪造。他们只好一起沿着红线走，穿过封死的排水渠，来到一扇没有门牌的铁门。门后确实有电，红灯一闪一闪，像有人在里面用最后一点力气敲桌面。制图师敲了三下，里面也回了三下。档案员却把手按在他腕上：“别问是谁。先记住这扇门的位置。”',
      '第二天，地图局把红线从纸上彻底刮掉，制图师的编号也被撤下。档案员没有把原图放回抽屉，而是压在一张空白登记表下面。凌晨 00:03，红灯准时熄灭。再过几个小时，整座城就会断光。',
    ],
    trace: '第八张退稿的背面多了一行不是两人笔迹的字：“别把桥修到这里。”',
  },
  {
    id: 'the-last-three-percent', era: 'lamplight', date: '零历 01 年 · 第一个长夜', title: '最后的 3% 留给谁',
    summary: '断光那夜，整座城只剩 3% 的电。值班表要求保住总闸，门外的敲门声却来自那条被删掉的路。',
    district: 'power', residents: ['low_battery_sprite', 'hope_night_shift'],
    paragraphs: [
      '断光以后，供电所的计量盘停在 3%。低电量小人被派去守住总闸，手里的小灯却一直朝着地图上没有的方向偏。希望夜班员把商店门锁了两次，第二次是因为门外有人敲门。敲门声是三短一长，和余晖制图师在铁门上敲出的节奏一样。值班长说旧备用环线不存在，要求他们把最后的电留给城市名单和总闸。夜班员看着门缝下渗进来的水，问：“如果名单里没有他，电还算救到人吗？”没人回答。',
      '低电量小人把自己的剩余电量接进旧回路。商店熄了灯，街上的人开始骂他们把希望浪费在一个不存在的地方。红灯亮了三秒，铁门后传来一个喘息声：“我们不是在城外。”随后线路烧断，最后的 3% 变成了黑。',
      '天亮时，门外没有人，只有一枚湿透的铆钉和一张被烧掉一角的维修单。维修单的背面写着：失败回路处，顶楼，别走直梯。夜班员把铆钉收进口袋，却没有把那晚写进正式值班表。',
    ],
    trace: '官方记录写着“电量自然耗尽”。夜班员的签名是假的，真正的值班表少了一整页。',
  },
  {
    id: 'a-button-left-in-place', era: 'lamplight', date: '零历 02 年 · 雨季', title: '那枚没有被拆掉的按钮',
    summary: '他们收到一枚烧黑的铆钉和一只旧继电器。重试按钮可以让旧线路重启，也可能把整座工坊送进雨里。',
    district: 'workshop', residents: ['retry_button_keeper', 'cloud_patch_worker'],
    paragraphs: [
      '重试按钮看守在工坊顶楼找到那枚湿铆钉。云端修补匠认出它不是被雨泡坏的，而是有人用软金属替换了原来的安全件。有人想让那架梯子在最需要的时候断掉。他们可以把铆钉交上去，工坊会被封；也可以换掉它，继续假装这只是一次事故。修补匠已经收拾好工具包，准备离开这座总把裂缝藏起来的城。按钮看守没有拦他，只把墙上的红色按钮擦得更亮。',
      '午夜暴雨时，按钮自己亮了。看守按下去，旧回路短暂重启，整座工坊的屋顶被掀开一角。修补匠没有去关电，而是踩着摇晃的梯子把一块蓝色补丁钉到天空上。里面露出的不是星星，是一段被剪掉的电台录音。',
      '录音只说了一句：“如果你们还在，别回信。”随后传来海浪声和七次断续的电流。修补匠想把按钮拆掉，看守却把它留在墙上：“现在拆，才像是我们从没听见过。”',
    ],
    trace: '旧继电器每天凌晨 00:03 自己响一声。没人知道它在给谁报时。',
  },
  {
    id: 'the-answer-across-water', era: 'bridge', date: '零历 03 年 · 蓝时', title: '没有回音的另一岸',
    summary: '蓝时电台连续七夜没有回应。守灯人不是收不到，他曾经收到过一条不敢让全城听见的回话。',
    district: 'harbour', residents: ['blue_hour_operator', 'zero_hour_lighthouse_keeper'],
    paragraphs: [
      '蓝时信号员连续七夜发送同一个呼号。第八夜，他在噪音里听见自己的名字，只有两个字，后面跟着一阵像人在水下说话的喘息。守灯人立刻把灯转向海面，不让那段声音进入公共频道。信号员追到灯塔底层，发现守灯人一直在给旧码头留一束窄光。守灯人承认，断光后的第一夜他收到过回信：“别叫我们的名字。有人在你们这一岸数灯。”他害怕城市听见后会立刻派人过桥，于是把回应藏了起来。',
      '他们用工坊送来的补丁重新包住天线，公开播放那段未经剪辑的录音。城里的人第一次听见对岸的海，也第一次听见守灯人承认自己撒过谎。回应终于回来：“桥可以开，但不是所有人都会回来。”',
      '信号员没有追问对岸是谁，只把七夜的空白页和第八夜的录音一起封存。守灯人把灯调到最下面一级台阶。天亮前，台阶上出现了一串湿脚印，走到一半就消失了。',
    ],
    trace: '电台的第八夜记录没有署名。最后一行只有一句：“别把桥修成一条只准回来的人走的路。”',
  },
  {
    id: 'a-bridge-with-benches', era: 'bridge', date: '零历 03 年 · 入秋', title: '一座允许坐下的桥',
    summary: '桥的最后三十米没有出现在施工图里。制图师坚持把它画回来，官方却要求在潮水上涨前签字开通。',
    district: 'archive', residents: ['afterglow_cartographer', 'low_battery_sprite', 'desktop_dust_archivist'],
    paragraphs: [
      '官方方案把桥画成一条直线，没有座椅，也没有最后三十米。余晖制图师把旧地图摊在施工台上，发现三处休息台正好对着灯塔、工坊和那扇没有门牌的铁门。有人不是在修桥，是在拼一条信号线。档案员拿出藏了两年的碳纸，要求把被删掉的路写进桥的底稿。工程负责人说，潮水只给他们一天，等所有证据核实完，对岸的人可能先被水带走。低电量小人测完每一段电量，发现中段只能承受一个人和一张椅子的重量。',
      '他们还是开了桥，只在入口挂上一块刺眼的牌子：未完工。第一位走到中段的人停了五次，后面的人开始催促。桥身突然往下沉了一寸，所有人只能坐下。没有人因此被救成英雄，但桥没有塌。',
      '第三把椅子的背面被刻了一行新字： “空白不是没有名字，是名字还没决定回来。”走到对岸的人带回一张空白收藏卡，卡面没有图，只有一条从桥外伸进来的线。',
    ],
    trace: '桥的验收文件至今少最后一页。那一页上，应该写着第一位过桥者的名字。',
  },
  {
    id: 'a-city-that-can-take-shifts', era: 'open', date: '零历 04 年 · 开城日', title: '今天轮到谁去生活',
    summary: '开城典礼准备把四年的救场写成一张英雄名单。希望夜班员先说不：如果名单上没有休息，他不签自己的名字。',
    district: 'shop', residents: ['hope_night_shift', 'zero_hour_lighthouse_keeper', 'blue_hour_operator', 'cloud_patch_worker'],
    paragraphs: [
      '开城前一天，管理者把所有人的名字排在一张金色名单上，准备宣布“零号城从未失去任何人”。希望夜班员看完，把钥匙放回桌上，说自己今晚不值班。厨房里没人动，连水壶都像在等他改口。他不想再用一次休息换一盏灯，也不想让别人把他的疲惫写成奉献。蓝时信号员接过电台，修补匠接过屋顶，守灯人把灯调低。城市第一次没有立刻崩坏，只是每个人都做得比夜班员慢。',
      '档案员把原图、假签名、七夜空白页和空白收藏卡摆在开城桌上。有人要求删掉这些证据，否则新居民会不信任这座城。制图师回答：“他们本来就不该只相信一张漂亮的名单。”于是第一份公开档案留下了争议，也留下了修订入口。',
      '城门打开那晚，没有所有灯一起亮。夜班员睡到第二天中午，门外有人替他留了一盏小灯。档案馆里却出现一张没有署名的新卡： “第十二个人还在桥的另一头。他不肯回来。他说，等你们承认他不是故障。”',
    ],
    trace: '开城日的菜单背面仍挂着那张值班表。休假栏里多了一个陌生人的名字，字迹和第一章那条红线一样。',
  },
]

export function chronicleResident(key: string | undefined): ChronicleResident | undefined {
  return chronicleResidents.find(resident => resident.key === key)
}

export function chroniclePortrait(key: ZeroCityCardKey): string {
  return zeroCityCardThumbnailPath({ card_key: key, rarity: zeroCityCardCatalogRarity(key) }, 320)
}

export function chronicleCardLink(key: ZeroCityCardKey): string {
  return `/zero-city/cards?card=${encodeURIComponent(key)}`
}

export function filterChronicleEvents(query = '', era: ChronicleEra | 'all' = 'all'): ChronicleEvent[] {
  const term = query.trim().toLocaleLowerCase()
  return chronicleEvents.filter(event => {
    if (era !== 'all' && event.era !== era) return false
    const district = chronicleDistricts.find(item => item.id === event.district)
    const text = [event.title, event.summary, ...event.paragraphs, event.trace, event.date, district?.name,
      ...event.residents.flatMap(key => [ZERO_CITY_CARD_MANIFEST[key].name, ZERO_CITY_CARD_MANIFEST[key].faction])]
    return !term || text.join(' ').toLocaleLowerCase().includes(term)
  })
}

export function eventsForResident(key: ZeroCityCardKey): ChronicleEvent[] {
  return chronicleEvents.filter(event => event.residents.includes(key))
}
