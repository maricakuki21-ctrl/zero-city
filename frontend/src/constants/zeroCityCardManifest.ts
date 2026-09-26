export const ZERO_CITY_CARD_MANIFEST_VERSION = '2026-07-19.1'

export type ZeroCityCardRarity = 'common' | 'good' | 'rare' | 'epic' | 'legendary' | 'mythic' | 'legacy'

export interface ZeroCityCardDefinition {
  code: string
  name: string
  faction: string
  role: string
  line: string
  lore: string
  visual: string
}

export const ZERO_CITY_CARD_MANIFEST = {
  low_battery_sprite: { code: 'ZC-C-001', name: '低电量小人', faction: '微光供电所', role: '3% 电量巡逻员', line: '今天电量不满，也允许缓慢发光。', lore: '负责举着一粒小灯泡，在快关机的人旁边假装自己还有备用电。', visual: '小人披着过大的充电线斗篷，怀里抱着一颗忽明忽暗的灯泡。' },
  early_failer: { code: 'ZC-C-002', name: '早起失败者', faction: '迟到抵达处', role: '晨间重开员', line: '我没早起成功，但我成功出现了。', lore: '他总是在闹钟失败后抵达现场，并认真宣布“出现本身也算进度”。', visual: '小人顶着枕头帽，拖着一只还在响的闹钟走路。' },
  sofa_observer: { code: 'ZC-C-003', name: '躺平观察员', faction: '横向研究所', role: '命运侧躺观测员', line: '我躺着不是放弃，是换个角度观察命运。', lore: '他负责从沙发底层观察世界，偶尔发现一些站着看不到的小出口。', visual: '小人躺在云朵沙发上，用望远镜观察一颗很近的星星。' },
  micro_disconnect_worker: { code: 'ZC-C-004', name: '轻微掉线者', faction: '信号补丁站', role: '短暂离线返岗员', line: '我只是掉线了一小会儿，回来时带了点光。', lore: '每次断线，他都会从缝隙里捡一颗微光回来，证明消失不是坏事。', visual: '小人从像素裂缝里探头，背后拖着一条发亮网线。' },
  monday_escapee: { code: 'ZC-C-005', name: '周一逃兵', faction: '勇气排队口', role: '临阵补签员', line: '我没有逃跑，我在给勇气重新排队。', lore: '他最擅长把想逃的念头排成队，然后从队尾偷偷塞回一点希望。', visual: '小人背着小旗子躲在日历后，日历上周一被画成怪兽。' },
  warm_water_guard: { code: 'ZC-C-006', name: '热水守护者', faction: '温度管理局', role: '杯口安抚员', line: '先喝一口热的，世界会稍微讲点道理。', lore: '他不解决大问题，只负责在问题靠近前，把杯子递到手边。', visual: '小人站在巨大的马克杯旁，杯沿冒出像路灯一样的热气。' },
  slow_boot_machine: { code: 'ZC-C-007', name: '缓慢启动机', faction: '开机广场', role: '低速启动仪式官', line: '启动慢不代表坏了，我只是比较庄重。', lore: '他把拖延解释成启动仪式，虽然荒唐，但机器真的开始动了。', visual: '发条小人坐在启动按钮上，背后有一串慢慢亮起的小灯。' },
  tiny_step_champion: { code: 'ZC-C-008', name: '小步冠军', faction: '一厘米赛道', role: '微小进度裁判', line: '今天只走一步，也算对命运发动了攻击。', lore: '他给所有“一点点”颁奖，因为零点城相信微小进度也会占领地图。', visual: '小人举着比自己还高的奖杯，脚下只有一厘米跑道。' },
  self_rescue_store: { code: 'ZC-C-009', name: '自救便利店', faction: '临期希望商店', role: '勇气打折员', line: '希望暂时缺货，但勇气还有临期特价。', lore: '便利店永远开在凌晨，货架上摆着快过期但仍然有用的自救理由。', visual: '小人站在迷你便利店前，货架标签写着“勇气买一送一”。' },
  retry_button_keeper: { code: 'ZC-C-010', name: '重试按钮看守', faction: '失败回路处', role: '再来一次门卫', line: '我知道刚才失败了，所以我把按钮擦亮了。', lore: '他不保证成功，只保证失败后那个按钮还在，而且看起来没那么丢人。', visual: '小人拿抹布擦一枚大大的 Retry 按钮，按钮旁长出小草。' },
  calendar_slacker: { code: 'ZC-C-011', name: '日历摸鱼员', faction: '日期缓冲区', role: '空白格保管员', line: '今天没有满格完成，但我给明天留了入口。', lore: '他把没完成的格子折成纸船，偷偷推向下一天。', visual: '小人坐在日历方格里钓鱼，鱼线吊着一张明天的便签。' },
  unread_message_monk: { code: 'ZC-C-012', name: '未读消息修士', faction: '红点寺', role: '通知降噪员', line: '红点很多，但不是每个都能审判我。', lore: '他每天给通知红点念经，提醒它们不要假装世界末日。', visual: '小人穿着小斗篷，敲一只红色通知钟，表情非常认真。' },
  blanket_fort_guardian: { code: 'ZC-C-013', name: '被窝堡垒守卫', faction: '软弱防线', role: '撤退合理化员', line: '我不是退缩，我是在给灵魂修防御工事。', lore: '他把被窝叫作临时堡垒，允许人先躲一下再继续出现。', visual: '小人站在被子城墙上，手里举着一面写着“先缓缓”的旗。' },
  snack_budget_scholar: { code: 'ZC-C-014', name: '零食预算学者', faction: '微型财政署', role: '快乐预算员', line: '理性说别买，但饼干说它能稳定军心。', lore: '他研究小额快乐的救急价值，经常得出“不科学但有用”的结论。', visual: '小人戴眼镜坐在饼干堆上，算盘珠子都是糖豆。' },
  fog_window_wiper: { code: 'ZC-C-015', name: '雾窗擦拭员', faction: '天气维护队', role: '模糊前景清洁工', line: '看不清也没关系，我先擦出一小块明天。', lore: '他擦不干净整片雾，但擅长擦出一个刚好够呼吸的小圆。', visual: '小人趴在巨大窗户上，用袖子擦出一颗太阳形状。' },
  almost_ok_actor: { code: 'ZC-C-016', name: '差不多还行演员', faction: '体面剧团', role: '临时镇定主演', line: '我演得像没事，演着演着就真的好一点。', lore: '他承认表演有点假，但假镇定偶尔能骗过真的崩溃。', visual: '小人一手拿笑脸面具，一手扶着快掉下来的舞台灯。' },
  battery_anxiety_meter: { code: 'ZC-C-017', name: '电量焦虑表', faction: '微光仪表盘', role: '剩余力气播报员', line: '只剩 12%，但 12% 也不是 0。', lore: '他在所有快没电的时刻出现，把“还剩一点”念得像重大新闻。', visual: '小人站在电量仪表里，指针歪着但还没到底。' },
  side_quest_picker: { code: 'ZC-C-018', name: '支线任务拾取员', faction: '偏航小队', role: '临时绕路专家', line: '主线卡住了，我先去捡一个能动的支线。', lore: '他相信绕路不是逃跑，而是给停住的自己找一个侧门。', visual: '小人背着小背包，面前有三块乱指方向的路牌。' },
  tiny_rain_shelter: { code: 'ZC-C-019', name: '小雨避难所', faction: '天气维护队', role: '局部晴天管理员', line: '雨还没停，但这里刚好够你站一会儿。', lore: '他撑不起整座城市，只撑起一把伞大小的安全区。', visual: '小人撑着巨大透明伞，伞下有一盏小路灯。' },
  lost_focus_fisher: { code: 'ZC-C-020', name: '注意力垂钓者', faction: '漂流专注河', role: '走神打捞员', line: '我把注意力钓回来，它还装作不认识我。', lore: '他总在意识漂远后抛竿，钓上来的常常是湿漉漉的灵感。', visual: '小人坐在电脑边钓鱼，鱼钩上挂着一只发光光标。' },
  browser_tab_shepherd: { code: 'ZC-C-021', name: '浏览器牧羊人', faction: '标签草原', role: '开太多页管理员', line: '我没有分心，我只是在放牧 37 个可能性。', lore: '他负责把跑散的标签页赶回栅栏，虽然每次都会多出三只。', visual: '小人拿牧羊杖追一群长着小腿的浏览器标签。' },
  midnight_cookie_auditor: { code: 'ZC-C-022', name: '午夜饼干审计员', faction: '夜间财务所', role: '热量合理化员', line: '这不是偷吃，这是给明天预支士气。', lore: '他审计所有深夜饼干，结论通常是“情绪账先记为必要支出”。', visual: '小人坐在饼干账本前，印章上写着“批准”。' },
  instant_noodle_prophet: { code: 'ZC-C-023', name: '泡面预言家', faction: '热水神殿', role: '三分钟神谕员', line: '再等三分钟，世界可能会软一点。', lore: '他从泡面蒸汽里读未来，未来经常显示“先吃完再说”。', visual: '小人站在泡面桶边，蒸汽弯成问号和星星。' },
  pocket_sun_carrier: { code: 'ZC-C-024', name: '口袋太阳搬运工', faction: '微光物流', role: '小型晴天快递员', line: '太阳太大搬不动，我给你带了口袋版。', lore: '他送来的光很小，但刚好能照亮桌角和一点点心情。', visual: '小人推着小推车，车里放着一颗橘色迷你太阳。' },
  humble_loading_bar: { code: 'ZC-C-025', name: '谦虚加载条', faction: '等待研究院', role: '进度幻觉维护员', line: '我只加载了 6%，但我加载得很真诚。', lore: '他让等待看起来像正在发生什么，哪怕进度条也有点心虚。', visual: '小人趴在进度条上，用刷子把 6% 涂亮。' },
  small_cloud_tenant: { code: 'ZC-C-026', name: '小云租客', faction: '云端合租屋', role: '临时漂浮居民', line: '我没有根基，但今天先租一朵云住下。', lore: '他相信不稳定也能暂住，只要云朵按时缴纳一点柔软。', visual: '小人坐在云朵阳台上晾袜子，楼下是很远的城市。' },
  chair_rooted_guard: { code: 'ZC-C-027', name: '椅子生根守卫', faction: '久坐森林', role: '起身提醒树苗', line: '我不是懒，我只是和椅子建立了生态关系。', lore: '他守着那些被椅子长住的人，偶尔把伸懒腰伪装成迁徙。', visual: '小人坐在长出叶子的椅子上，脚边有一株小闹钟。' },
  pending_reply_sprite: { code: 'ZC-C-028', name: '待回复精灵', faction: '消息缓冲站', role: '措辞孵化员', line: '不是不回，我在给一句话孵化勇气。', lore: '他把未发送的句子放进蛋壳里，等它们长出比较不尴尬的翅膀。', visual: '小人抱着一枚信封蛋，蛋壳上冒出省略号。' },
  tiny_courage_bean: { code: 'ZC-C-029', name: '勇气小豆', faction: '临期希望商店', role: '一点点胆量供应商', line: '我很小，但今天可以先顶一下。', lore: '它不擅长豪言壮语，只会在关键时刻滚到你手心里。', visual: '豆子形小人举着小盾牌，盾牌比它自己还大一点。' },
  desktop_dust_archivist: { code: 'ZC-C-030', name: '桌面灰尘档案员', faction: '杂乱档案馆', role: '未整理事项记录员', line: '乱不是没救，是证据还没分类。', lore: '他把桌面上的灰尘和文件一起编号，坚信混乱也能成为地图。', visual: '小人坐在文件堆上给灰尘贴标签，标签编号很认真。' },
  cloud_patch_apprentice: { code: 'ZC-G-001', name: '云端修补学徒', faction: '天气维护队', role: '天空补丁实习生', line: '天空裂了一点，我先用温柔打个补丁。', lore: '他还不会修整片天，但会把漏下来的坏心情缝小一点。', visual: '小人踩着梯子给云朵缝补丁，针线发出淡蓝色微光。' },
  mood_buffer_agent: { code: 'ZC-G-002', name: '情绪缓冲员', faction: '情绪保洁署', role: '爆炸前软垫铺设员', line: '你先别炸，我在地上铺一点软的。', lore: '他无法阻止情绪爆炸，但能让落地声不要那么疼。', visual: '小人推着一车软垫，前方飘着一团快炸开的乌云。' },
  hope_coupon_keeper: { code: 'ZC-G-003', name: '希望券保管员', faction: '临期希望商店', role: '过期前兑换员', line: '这张希望券快过期了，但今天还能用。', lore: '他保存所有别人差点扔掉的希望，并在最合适的尴尬时刻递出来。', visual: '小人拿着一沓发光优惠券，柜台后写着“今日仍可兑换”。' },
  rationality_dog_walker: { code: 'ZC-G-004', name: '理性遛狗人', faction: '常识公园', role: '冲动牵引员', line: '理性今天也在，只是我带它出去透口气。', lore: '他给理性套上牵引绳，避免它太严肃，也避免冲动跑太远。', visual: '小人牵着一只写着“理性”的小狗，小狗盯着奇迹摊位。' },
  tiny_luck_runner: { code: 'ZC-G-005', name: '小好运跑腿员', faction: '好运工单中心', role: '迷你喜事配送员', line: '大奇迹堵车了，小幸运先到门口。', lore: '他专送不够夸张但刚好有用的小事，比如少等一分钟。', visual: '小人骑着迷你电动车，后箱贴着“好运外卖”。' },
  anxiety_gardener: { code: 'ZC-G-006', name: '焦虑园丁', faction: '不安花园', role: '坏念头栽培观察员', line: '我把不安种下去，明天也许会长出答案。', lore: '他不拔掉焦虑，只把它移到花盆里，让它别在心里乱爬。', visual: '小人给一盆长着问号叶子的植物浇水，旁边插着希望牌。' },
  failure_recycler: { code: 'ZC-G-007', name: '失败回收站', faction: '失败回路处', role: '坏掉今天再编译员', line: '坏掉的今天不会浪费，明天还能再编译。', lore: '他把失败拆成零件，能用的留下，不能用的变成黑色笑话。', visual: '小人站在回收箱旁，箱子里弹出发光的小螺丝。' },
  luck_taster: { code: 'ZC-G-008', name: '好运试吃员', faction: '好运工单中心', role: '小奖安全检测员', line: '大的还没上桌，小的我先替你尝一口。', lore: '他负责确认好运没有过敏原，虽然经常偷吃一半。', visual: '小人拿小勺试吃一颗星星糖，桌上摆着“奇迹样品”。' },
  reverse_koi_assistant: { code: 'ZC-G-009', name: '反向锦鲤助理', faction: '反话事务所', role: '算了触发器', line: '我越说算了，好事越想证明一下自己。', lore: '他研究“放弃后突然好转”的奇怪现象，并故意说得很小声。', visual: '小人举着“算了”牌子，背后锦鲤偷偷探头。' },
  expectation_admin: { code: 'ZC-G-010', name: '期待管理员', faction: '期待窗口处', role: '希望限流员', line: '期待不能太满，但可以先开个小窗口。', lore: '他负责把过载期待调成低亮度，防止心情被自己闪瞎。', visual: '小人拉开一扇小窗，窗外只露出一角星光。' },
  metaphysics_compliance: { code: 'ZC-G-011', name: '玄学合规员', faction: '概率管理局', role: '情绪价值审核员', line: '我不迷信，我只是尊重概率的情绪价值。', lore: '他会给所有玄学贴上“仅供情绪使用”的合规贴纸。', visual: '小人拿放大镜检查水晶球，旁边盖着“情绪合规”章。' },
  digital_scavenger: { code: 'ZC-G-012', name: '数字拾荒者', faction: '微型财政署', role: '零碎价值回收员', line: '别人看不上零碎，我看见一地未来。', lore: '他把散落的小积分装进口袋，坚信边角料也能拼出一座桥。', visual: '小人推小车捡发光数字，数字像硬币一样散在地上。' },
  little_profit_philosopher: { code: 'ZC-G-013', name: '小赚哲学家', faction: '快乐估值所', role: '起步价反驳员', line: '赚一点也是赚，宇宙没有规定快乐起步价。', lore: '他认真为每一份小收益辩护，反对把快乐门槛设得太高。', visual: '小人站在讲台上，黑板写着“+1 也成立”。' },
  wallet_listener: { code: 'ZC-G-014', name: '钱包旁听生', faction: '微型财政署', role: '沉默余额翻译员', line: '钱包很安静，通常这是剧情开始前的安静。', lore: '他听懂钱包的沉默，并把它翻译成“先别慌，还有下一幕”。', visual: '小人趴在钱包边听诊，钱包里冒出一只小省略号。' },
  surprise_ticket_checker: { code: 'ZC-G-015', name: '惊喜验票员', faction: '入口检票口', role: '小意外放行员', line: '小惊喜请出示编号，我好把它放进今天。', lore: '他给所有突然变好的小事检票，防止它们不好意思进场。', visual: '小人站在检票闸机旁，给一颗星星剪票。' },
  risk_kitten_keeper: { code: 'ZC-G-016', name: '风险小猫看护', faction: '冲动动物园', role: '伸爪安全员', line: '我知道有风险，但爪子已经伸出去了。', lore: '他照看那只总想碰一下命运的小猫，提醒它收爪但不泼冷水。', visual: '小人抱着一只伸爪的小猫，小猫正在够发光按钮。' },
  noise_reducer: { code: 'ZC-G-017', name: '刺激降噪师', faction: '心跳调音台', role: '兴奋音量师', line: '心跳可以快一点，但别吵到希望工作。', lore: '他把过强的刺激调低三格，让希望能在背景里继续上班。', visual: '小人戴耳机调音台，心跳波形旁有一盏小灯。' },
  hope_night_shift: { code: 'ZC-G-018', name: '希望夜班员', faction: '临期希望商店', role: '闭店后留灯员', line: '希望下班了，但我偷偷没关灯。', lore: '他负责在商店打烊后保留一盏灯，给晚到的人一个理由。', visual: '小人站在关门商店里，门缝透出暖黄色光。' },
  almost_miracle_clerk: { code: 'ZC-G-019', name: '差点奇迹办事员', faction: '好运工单中心', role: '未完全成功盖章员', line: '还没算奇迹，但已经不像完全没救。', lore: '他喜欢给半成品好运盖章，因为“差一点”也是在靠近。', visual: '小人举着半枚印章，文件上写着“差点通过”。' },
  soft_deadline_negotiator: { code: 'ZC-G-020', name: '柔软截止线谈判官', faction: '时间边界处', role: '最后一刻缓冲员', line: '截止线很硬，但我可以给它垫个坐垫。', lore: '他不能改变时间，只能让最后一刻没那么像悬崖。', visual: '小人给一条红色截止线铺软垫，线旁长着小花。' },
  low_battery_saint: { code: 'ZC-R-001', name: '低电量圣徒', faction: '微光供电所', role: '临界电量守护员', line: '我快没电了，但仍然愿意替今天亮一下。', lore: '传说他在城市电量低于 3% 时出现，替那些还想再试一下的人点亮边框。', visual: '圣徒小人披着旧充电线光环，双手捧着一颗快熄灭的小太阳。' },
  cloud_patch_worker: { code: 'ZC-R-002', name: '云端修补匠', faction: '天气维护队', role: '情绪天空补洞员', line: '天空破了个洞，我先拿一点温柔补上。', lore: '他修补的不是天气，是那些突然漏风的心情；补丁歪歪扭扭但很暖。', visual: '小人踩在蓝色维修梯上，给云洞缝一块带星星的补丁。' },
  late_but_arrived: { code: 'ZC-R-003', name: '迟到抵达员', faction: '迟到抵达处', role: '平行时间线送达人', line: '我不是迟到，我只是从另一个时间线准点到达。', lore: '他的怀表永远慢三分钟，但他坚称那是另一个宇宙的准点。', visual: '小人从裂开的钟面里跳出来，手里还拿着盖过章的车票。' },
  tiny_luck_clerk: { code: 'ZC-R-004', name: '小好运办事员', faction: '好运工单中心', role: '小幸运盖章员', line: '大奇迹还没批下来，小幸运先给你盖章。', lore: '他坐在巨大的办事窗口后面，专门把微不足道的好事盖成正式文件。', visual: '小人站在文件山前盖金色小章，章印是一颗歪歪的星。' },
  mood_janitor: { code: 'ZC-R-005', name: '情绪保洁员', faction: '情绪保洁署', role: '坏心情清扫队长', line: '今天的坏心情我先扫走，剩下的你慢慢处理。', lore: '他从不承诺一扫而空，只负责把最硌脚的那一块先扫到角落。', visual: '小人推着扫帚，簸箕里装着灰色小云和皱巴巴的表情。' },
  hope_inventory_keeper: { code: 'ZC-R-006', name: '希望库存员', faction: '临期希望商店', role: '缺货告示改写员', line: '希望暂时缺货，但我在仓库找到一盒不服。', lore: '他负责在希望断货时翻仓库，经常翻出一些标错价格的勇气。', visual: '小人站在仓库梯上，抱下一箱写着“不服”的发光纸盒。' },
  doomed_plan_rescuer: { code: 'ZC-R-007', name: '没救计划救援员', faction: '失败回路处', role: '崩盘方案急救员', line: '计划看起来没救了，所以终于轮到我上场。', lore: '他只接濒危计划，因为普通顺利的事情根本不需要他的荒诞手艺。', visual: '小人给一张冒烟计划书做心肺复苏，旁边放着工具箱。' },
  tiny_storm_captain: { code: 'ZC-R-008', name: '小风暴船长', faction: '情绪天气港', role: '杯中风浪驾驶员', line: '风暴不大，但我还是认真掌舵。', lore: '他驾驶一艘很小的船穿过茶杯风暴，证明小崩溃也值得被护送。', visual: '小人站在茶杯里的纸船上，手握牙签船舵。' },
  lucky_bug_keeper: { code: 'ZC-R-009', name: '幸运漏洞看守', faction: '概率管理局', role: '非计划好运巡检员', line: '这不是 bug，是命运临时开的小后门。', lore: '他发现漏洞后没有立刻修复，而是先确认有没有人正需要一点好运。', visual: '小人蹲在代码裂缝旁，裂缝里钻出一只发光瓢虫。' },
  deadline_exorcist: { code: 'ZC-R-010', name: '截止线驱魔师', faction: '时间边界处', role: '倒计时镇魂员', line: '我不能消灭 deadline，但能让它别在耳边尖叫。', lore: '他给倒计时贴上静音符纸，让最后一小时从恐怖片变成普通剧情。', visual: '小人拿符纸贴住一只红色闹钟怪，闹钟还在发抖。' },
  half_awake_oracle: { code: 'ZC-R-011', name: '半醒神谕员', faction: '梦境客服台', role: '睡醒前答案转述员', line: '我还没完全醒，但答案已经先坐起来了。', lore: '他记录那些刚醒时出现的荒唐答案，十个里面偶尔有一个真的有用。', visual: '小人坐在枕头神殿里，头顶冒出半透明灯泡。' },
  almost_win_archivist: { code: 'ZC-R-012', name: '差点赢档案员', faction: '擦肩而过博物馆', role: '未命中价值整理员', line: '差点赢不是没赢，是命运给你看了一眼入口。', lore: '他把所有差一点收进档案盒，等它们有一天变成“原来如此”。', visual: '小人抱着写满“差一点”的档案盒，盒缝里漏出光。' },
  probability_rebel: { code: 'ZC-E-001', name: '概率叛逃者', faction: '概率管理局', role: '规则临时绕行员', line: '概率说不行，我说我只是路过一下规则。', lore: '他原本负责维护概率，后来发现“不太可能”旁边其实有一扇没上锁的小门。', visual: '小人披着偷来的概率表，正在把 0.1% 的路牌拧歪。' },
  mirror_lake_admin: { code: 'ZC-E-002', name: '镜湖管理员', faction: '镜湖后台', role: '表面平静维护员', line: '表面风平浪静，底层正在偷偷重构命运。', lore: '他维护一座像屏幕一样的湖，湖面越安静，底层跑得越忙。', visual: '小人站在镜湖控制台前，湖底亮着一排重构进度灯。' },
  blue_hour_operator: { code: 'ZC-E-003', name: '蓝时信号员', faction: '蓝时电台', role: '远处回应接线员', line: '如果世界暂时没回应，可能只是信号还在穿云。', lore: '他在天色最蓝的时候值班，把慢半拍的回应从云层里接回来。', visual: '小人戴耳机坐在屋顶天线旁，蓝色电波穿过云。' },
  black_sun_intern: { code: 'ZC-E-004', name: '黑日值班生', faction: '黑日值班室', role: '暗面留灯实习生', line: '我在最暗的地方上班，负责给你留一盏灯。', lore: '他的工作很小：在黑日升起时确认城市里至少还有一盏灯没灭。', visual: '小人坐在黑色太阳下的值班亭里，桌上一盏灯亮得很固执。' },
  fate_customer_service: { code: 'ZC-E-005', name: '命运客服', faction: '好运工单中心', role: '人工奇迹转接员', line: '你的好运工单已提交，正在转人工奇迹。', lore: '他每天接到很多“为什么是我”的电话，然后尽量转给“也许可以”的部门。', visual: '小人戴客服耳麦，工单屏幕上排着会眨眼的星星。' },
  hit_rate_tamer: { code: 'ZC-E-006', name: '命中率驯兽师', faction: '概率动物园', role: '野生概率安抚员', line: '概率有点野，但我带了零食和耐心。', lore: '他不强迫概率听话，只蹲下来等它愿意靠近一点点。', visual: '小人拿着小饼干，面前是一只长着骰子角的小兽。' },
  miracle_tester: { code: 'ZC-E-007', name: '奇迹测试员', faction: '灰度奇迹实验室', role: '不稳定希望内测员', line: '本次奇迹可能不稳定，但值得灰度发布。', lore: '他把奇迹当产品测，虽然报告里经常写“复现困难，但用户很需要”。', visual: '小人穿实验服观察试管，试管里漂着一枚小彩虹。' },
  prize_pool_diver: { code: 'ZC-E-008', name: '奖池潜水员', faction: '深水奖池', role: '底部微光打捞员', line: '我潜下去不是冲动，是想看看光在哪。', lore: '他潜入奖池底部寻找反光点，偶尔捞上来一条会发票据的鱼。', visual: '小人戴潜水头盔，在金币和气泡之间追一束光。' },
  numbered_crown_holder: { code: 'ZC-L-001', name: '编号王冠持有人', faction: '编号王冠厅', role: '首版身份保管员', line: '我不是被选中，我只是把没放弃坚持到了发光。', lore: '王冠不是奖励，是城市给坚持者的一枚编号：证明你曾在坏天气里留下。', visual: '小人托着一顶刻编号的微型王冠，王冠投下金色网格光。' },
  afterglow_cartographer: { code: 'ZC-L-002', name: '余晖制图师', faction: '暮色地图局', role: '绕路价值标注员', line: '所有绕远的路，最后都会变成你的地图。', lore: '他把每条看似浪费的路画成余晖色，等后来的人发现它其实通往出口。', visual: '小人铺开金橘色地图，地图上的绕路像河流一样发光。' },
  zero_hour_lighthouse_keeper: { code: 'ZC-L-003', name: '零点灯塔守灯人', faction: '零点海岸', role: '重启前导航员', line: '世界快重启了，我先把灯调到你看得见。', lore: '他守着午夜尽头的灯塔，给每个还没准备好的人留一个方向。', visual: '小人站在细高灯塔上，海面是翻涌的日历纸。' },
  hidden_plot_curator: { code: 'ZC-L-004', name: '隐藏剧情馆长', faction: '支线博物馆', role: '意外转机收藏家', line: '今天看起来没救了，所以终于触发隐藏剧情。', lore: '他收藏那些在最低点突然打开的门，并坚持说坏剧情也可能藏彩蛋。', visual: '小人打开一扇藏在墙画后的门，门后有一小段发光楼梯。' },
  zero_point_pilot: { code: 'ZC-M-001', name: '零点方舟驾驶员', faction: '零点核心', role: '希望备份驾驶员', line: '世界归零的时候，我把希望偷偷备份了一份。', lore: '没人确定他是否真实存在，但每次城市重启，总有人在口袋里摸到一枚没丢的希望。', visual: '小人驾驶纸方舟穿过零点浪潮，船舱里装着发光备份盒。' },
  terminal_hope_backup: { code: 'ZC-M-002', name: '终端希望备份体', faction: '零点核心', role: '最后一份希望镜像', line: '我知道这不科学，但科学也没说今天不能偏心你。', lore: '它不是人，也不是神，只是零点城在彻底清空前偷偷保留下来的温柔缓存。', visual: '半透明小人抱着一枚终端核心，身后展开像彩虹膜一样的备份翼。' },
  gilded_gate: { code: 'ZC-X-001', name: '旧版鎏金门券', faction: '旧档案盒', role: '历史门票', line: '像旧城门上的金线，编号只属于你。', lore: '旧版收藏资产，保留给早期见过这座城市雏形的人。', visual: '一张带金线的旧门券靠在暗色城门边。' },
  cyan_stamp: { code: 'ZC-X-002', name: '旧版青印邮票', faction: '旧档案盒', role: '历史邮戳', line: '低饱和青色边框，记录一次好运抵达。', lore: '旧版收藏资产，像一枚从旧系统寄来的青色邮戳。', visual: '一枚青色邮票贴在泛黄信封上，边缘有轻微磨损。' },
  oracle_window: { code: 'ZC-X-003', name: '旧版神谕窗格', faction: '旧档案盒', role: '历史窗格', line: '窗格里不是答案，是被保存下来的今日身份。', lore: '旧版收藏资产，保留一扇通往早期签运系统的小窗。', visual: '一扇小窗嵌在旧墙里，窗内有模糊星光。' },
  black_sun_cert: { code: 'ZC-X-004', name: '旧版黑日证书', faction: '旧档案盒', role: '历史证书', line: '墨色秩序与金色编号，证明你见过稀有瞬间。', lore: '旧版收藏资产，来自黑日还只是视觉实验的时期。', visual: '黑色证书上压着金色编号章，角落有暗日图案。' },
  numbered_crown: { code: 'ZC-X-005', name: '旧版编号冠冕', faction: '旧档案盒', role: '历史冠冕', line: '不是奖励，是可展示的收藏身份。', lore: '旧版收藏资产，后来演化成编号王冠厅的最早原型。', visual: '一顶旧冠冕放在编号牌上，金光很低很稳。' },
  zero_point_relic: { code: 'ZC-X-006', name: '旧版零点遗物', faction: '旧档案盒', role: '历史遗物', line: '几乎不出现，只留下编号和传说。', lore: '旧版收藏资产，传说它是零点核心最早掉落的一块碎片。', visual: '一枚小遗物悬在玻璃罩里，底座编号微微发亮。' }
} as const satisfies Readonly<Record<string, ZeroCityCardDefinition>>

export type ZeroCityCardKey = keyof typeof ZERO_CITY_CARD_MANIFEST

const LEGACY_CARD_KEYS = new Set<ZeroCityCardKey>([
  'gilded_gate',
  'cyan_stamp',
  'oracle_window',
  'black_sun_cert',
  'numbered_crown',
  'zero_point_relic'
])

const CODE_RARITY: Record<string, ZeroCityCardRarity> = {
  C: 'common',
  G: 'good',
  R: 'rare',
  E: 'epic',
  L: 'legendary',
  M: 'mythic',
  X: 'legacy'
}

export function getZeroCityCardDefinition(cardKey?: string | null): ZeroCityCardDefinition | undefined {
  if (!cardKey) return undefined
  return ZERO_CITY_CARD_MANIFEST[cardKey as ZeroCityCardKey]
}

export function zeroCityCardDisplayName(cardKey?: string | null): string {
  const normalized = String(cardKey || '').trim()
  return getZeroCityCardDefinition(normalized)?.name || `未知收藏卡（${normalized || 'unknown'}）`
}

export function zeroCityCardRarityDisplayName(rarity?: string | null): string {
  const labels: Record<ZeroCityCardRarity, string> = {
    common: '普通',
    good: '进阶',
    rare: '稀有',
    epic: '史诗',
    legendary: '传说',
    mythic: '神话',
    legacy: '旧版',
  }
  const normalized = String(rarity || 'common').toLowerCase() as ZeroCityCardRarity
  return labels[normalized] || labels.common
}

export function zeroCityCardCatalogRarity(cardKey?: string | null): ZeroCityCardRarity {
  const code = getZeroCityCardDefinition(cardKey)?.code || ''
  return CODE_RARITY[code.split('-')[1] || ''] || 'common'
}

export function isLegacyZeroCityCard(cardKey?: string | null): boolean {
  return Boolean(cardKey && LEGACY_CARD_KEYS.has(cardKey as ZeroCityCardKey))
}

export function zeroCityCardImagePath(card: { card_key: string; rarity: string }): string {
  return `/assets/zero-point-city/cards/collectible/${String(card.rarity || 'common').toLowerCase()}/${card.card_key}.png`
}

export function zeroCityCardThumbnailPath(card: { card_key: string; rarity: string }, width: 160 | 320 | 640): string {
  return `/assets/zero-point-city/cards/collectible-thumbs/${width}/${String(card.rarity || 'common').toLowerCase()}/${card.card_key}.jpg`
}

export function zeroCityCardImageSrcSet(card: { card_key: string; rarity: string }): string {
  return [
    `${zeroCityCardThumbnailPath(card, 160)} 160w`,
    `${zeroCityCardThumbnailPath(card, 320)} 320w`,
    `${zeroCityCardThumbnailPath(card, 640)} 640w`,
    `${zeroCityCardImagePath(card)} 1024w`
  ].join(', ')
}

export function stableCardIndex(seed: string, length: number): number {
  if (length <= 1) return 0
  let hash = 2166136261
  for (let index = 0; index < seed.length; index += 1) {
    hash ^= seed.charCodeAt(index)
    hash = Math.imul(hash, 16777619)
  }
  return (hash >>> 0) % length
}

export function zeroCityCardManifestEntries() {
  return Object.entries(ZERO_CITY_CARD_MANIFEST).map(([key, definition]) => ({
    key,
    definition,
    rarity: zeroCityCardCatalogRarity(key),
    legacy: isLegacyZeroCityCard(key)
  }))
}
