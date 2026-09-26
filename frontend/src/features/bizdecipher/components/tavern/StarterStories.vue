<script setup lang="ts">
import { computed, effectScope, onScopeDispose, ref, shallowRef, watch, type EffectScope, type Ref } from 'vue'
import { useStorage } from '@vueuse/core'
import { BookOpen, ChevronRight, Flag, Gamepad2, RotateCcw, Save, Users } from '@lucide/vue'
import { useAuthStore } from '@/stores/auth'
import { isDraftRecord, isStringList, localDraftSerializer } from '@/utils/localDraftStorage'
import { storyChoiceResponse, storyEnding } from './starterStoryOutcomes'
type Scene = { readonly title: string; readonly text: string; readonly clue: string; readonly choices: readonly string[] }
type Story = { readonly id: string; readonly title: string; readonly kind: string; readonly summary: string; readonly cover: string; readonly roles: readonly string[]; readonly scenes: readonly Scene[]; readonly ending: string }
const stories: readonly Story[] = [
  {
    id: 'last-letter', title: '最后一封来信', kind: '剧本杀 · 初始体验',
    summary: '钟声响起，信箱里多了一封明天的信。三位城民，都有一件不愿公开的往事。',
    cover: '/assets/zero-point-city/cards/collectible/rare/half_awake_oracle.png',
    roles: ['档案员', '修钟师', '夜班邮差'],
    scenes: [
      { title: '第一幕 · 不该出现的邮戳', text: '停业多年的旧邮局里，档案柜被人打开。信封写着明天的日期，但值班簿上只有三个人的名字。你决定先找出这封信来自哪里。', clue: '邮戳油墨尚未干透，信纸却已经泛黄。', choices: ['检查邮戳与印章', '询问夜班邮差', '整理三人的时间线'] },
      { title: '第二幕 · 少走的十二分钟', text: '修钟师承认昨夜将大厅时钟拨慢了十二分钟。档案员保管的交接记录中，恰好少了这段时间。邮差说他听到了两次钟声。', clue: '维修单背面写着：不要销毁，等她回来。', choices: ['追问维修单上的留言', '比对真实时间与值班记录', '公开第一幕的发现'] },
      { title: '第三幕 · 谁寄出了信', text: '三人都承认认识收信人。邮差守着一封退回多年的信，修钟师曾替她修过怀表，而档案员正在整理她未公开的手稿。你要向大家提出最后的判断。', clue: '旧信封内侧有档案柜编号，新邮戳盖在旧邮戳上。', choices: ['档案员重新投递了旧信', '修钟师设计了钟声', '邮差带来了未来的信'] },
    ],
    ending: '档案员重新投递了那封旧信，希望让一段被搁置的告别终于抵达。所谓明天的来信，是一次笨拙但真诚的邀请。钟声与时间差，只是让三人终于在同一张桌前坐下。'
  },
  {
    id: 'fog-station', title: '雾线末班车', kind: '跑团 · 开场体验',
    summary: '一辆没有终点站的列车，停在城市边界。带上你的角色，决定第一段旅程的方向。',
    cover: '/assets/zero-point-city/cards/collectible/rare/tiny_storm_captain.png',
    roles: ['探路者', '机械师', '记录者'],
    scenes: [
      { title: '出发 · 消失的站牌', text: '末班车驶入薄雾，车票上的终点渐渐褪色。列车员留下了一盏灯、一张路线图和一句话：终点不会主动找到你。', clue: '路线图标着三个地点：旧塔、水闸、信号站。', choices: ['前往旧塔寻找高处视野', '沿水闸寻找人类活动', '进入信号站尝试联络'] },
      { title: '探索 · 雾中的信号', text: '废弃的设备里传来断续的敲击声。你发现城市的信号被困在同一段循环，必须决定优先修复什么。', clue: '手册记录：信号和时钟必须来自同一个电源。', choices: ['检修电源和线路', '记录敲击声寻找规律', '请同行者检查时钟'] },
      { title: '抉择 · 最后一盏灯', text: '电源恢复了，但仅够点亮一段轨道。返回城市的路清晰起来，另一侧却传来求助声。你握着决定列车去向的道岔钥匙。', clue: '救援道岔可切换一次，之后必须手动复位。', choices: ['先返回城市召集救援', '把灯留给求助者', '分工复位道岔再一起出发'] },
    ],
    ending: '列车重新获得了方向。你们把这次探索的路线和决定记入城民档案，下一次旅程将从这些选择继续。'
  },
]
type LocalRoom = { storyId: string; title: string; role: string; payer: string; resource: string; unit: string; limit: number; scene: number; choices: string[]; finished: boolean }
const auth = useAuthStore()
const error = ref('')
function isRoom(value: unknown): value is LocalRoom | null {
  if (value === null) return true
  if (!isDraftRecord(value)) return false
  return ['storyId', 'title', 'role', 'payer', 'resource', 'unit'].every(key => typeof value[key] === 'string')
    && stories.some(item => item.id === value.storyId)
    && typeof value.scene === 'number' && Number.isInteger(value.scene) && value.scene >= 0 && value.scene < 3
    && typeof value.limit === 'number' && Number.isFinite(value.limit) && value.limit >= 0
    && isStringList(value.choices) && typeof value.finished === 'boolean'
}
const storedRoom = shallowRef<{ room: Ref<LocalRoom | null> }>({ room: ref(null) })
let storageScope: EffectScope | undefined
const room = computed({
  get: () => storedRoom.value.room.value,
  set: (value: LocalRoom | null) => { storedRoom.value.room.value = value },
})
const selectedId = ref(room.value?.storyId ?? stories[0]?.id ?? '')
const story = computed(() => stories.find(item => item.id === selectedId.value) ?? stories[0])
const activeStory = computed(() => stories.find(item => item.id === room.value?.storyId))
const scene = computed(() => activeStory.value?.scenes[room.value?.scene ?? 0])
const ending = computed(() => storyEnding(room.value?.storyId ?? '', room.value?.choices ?? []))
const lastResponse = computed(() => {
  if (!room.value || !activeStory.value || !room.value.choices.length) return ''
  const step = room.value.choices.length - 1
  const choice = activeStory.value.scenes[step]?.choices.indexOf(room.value.choices[step] ?? '') ?? -1
  return storyChoiceResponse(activeStory.value.id, step, choice)
})
const roomName = ref('')
const role = ref('')
const payer = ref('host')
const resource = ref('official')
const unit = ref('credit')
const limit = ref(0)
const clueVisible = ref(false)
const tab = ref<'catalog' | 'room'>(room.value ? 'room' : 'catalog')
watch(() => auth.user?.id, () => {
  storageScope?.stop()
  error.value = ''
  storageScope = effectScope()
  storageScope.run(() => {
    storedRoom.value = { room: useStorage<LocalRoom | null>(`tavern-local-room-v1-${auth.user?.id ?? 'local'}`, null, undefined, {
      serializer: localDraftSerializer(isRoom),
      flush: 'sync',
      onError: () => { error.value = '本地房间无法读取或保存，原记录未被自动删除。' },
    }) }
  })
  selectedId.value = room.value?.storyId ?? stories[0]?.id ?? ''
  tab.value = room.value ? 'room' : 'catalog'
  roomName.value = ''
  role.value = ''
  payer.value = 'host'
  resource.value = 'official'
  unit.value = 'credit'
  limit.value = 0
  clueVisible.value = false
}, { immediate: true, flush: 'sync' })
onScopeDispose(() => storageScope?.stop())
function start(): void {
  if (!story.value) return
  room.value = { storyId: story.value.id, title: roomName.value.trim() || story.value.title, role: role.value || story.value.roles[0] || '城民', payer: payer.value, resource: resource.value, unit: unit.value, limit: limit.value, scene: 0, choices: [], finished: false }
  clueVisible.value = false
  tab.value = 'room'
}
function choose(choice: string): void {
  if (!room.value || !activeStory.value || room.value.finished || !scene.value?.choices.includes(choice)) return
  const finished = room.value.scene + 1 >= activeStory.value.scenes.length
  room.value = {
    ...room.value,
    choices: [...room.value.choices, choice],
    scene: finished ? room.value.scene : room.value.scene + 1,
    finished,
  }
  clueVisible.value = false
}
function exportRecord(): void {
  if (!room.value) return
  const blob = new Blob([JSON.stringify({ schema: 'tavern.local-preview.v1', mode: 'local-scripted', ...room.value }, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url; link.download = 'tavern-room-record.json'; link.click()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <section class="starter-tavern">
    <header class="tavern-start-head"><div><p class="tavern-start-meta">零号城 · 故事与相遇</p><h2>选一个故事，留下你的选择</h2></div><RouterLink class="tavern-start-link" to="/community">回到全城动态 <ChevronRight :size="16" /></RouterLink></header>
    <nav class="tavern-start-tabs" aria-label="酒馆体验栏目"><button :class="{ active: tab === 'catalog' }" @click="tab = 'catalog'"><BookOpen :size="16" />初始故事</button><button :class="{ active: tab === 'room' }" @click="tab = 'room'"><Gamepad2 :size="16" />我的体验房间</button></nav>
    <p class="tavern-start-mode">本地剧情体验 · 固定剧本，无 AI 调用、无在线玩家、不扣积分或余额</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <div v-if="tab === 'catalog'" class="tavern-start-grid">
      <div class="tavern-stories">
        <article v-for="item in stories" :key="item.id" class="tavern-story" :class="{ selected: selectedId === item.id }">
          <div class="tavern-story-art"><img :src="item.cover" :alt="item.title" width="320" height="220" /></div>
          <div class="tavern-story-copy"><span class="tavern-start-meta">{{ item.kind }}</span><h2>{{ item.title }}</h2><p>{{ item.summary }}</p><div class="tavern-start-meta"><Users :size="14" />{{ item.roles.join(' / ') }}</div><button class="tavern-start-button" :aria-pressed="selectedId === item.id" @click="selectedId = item.id; role = ''">{{ selectedId === item.id ? '已选择' : '选择故事' }}<ChevronRight :size="15" /></button></div>
        </article>
      </div>
      <form class="tavern-room-config" @submit.prevent="start">
        <h2>房间设置</h2><p class="tavern-start-meta">{{ story?.title }}</p>
        <label>房间名称<input v-model="roomName" maxlength="100" :placeholder="story?.title" /></label>
        <label>我的角色<select v-model="role"><option value="">默认角色</option><option v-for="item in story?.roles" :key="item">{{ item }}</option></select></label>
        <div class="tavern-config-divider"><h3>这一局怎么玩</h3><p class="tavern-start-meta">读剧情 → 查看线索 → 每幕选择一次 → 查看结局与复盘。预计 5–10 分钟；角色用于代入，不附带隐藏数值加成。探查路线、设备准备和最后决定会改变回应或结局。</p></div>
        <p class="tavern-start-meta">免费单人体验，自动保存在当前浏览器。无需密钥；切换设备不会同步，可随时导出记录。</p>
        <p v-if="room" class="tavern-start-meta">开始新体验会替换此浏览器的体验房间。可先在房间内导出记录。</p>
        <button class="tavern-start-button primary" type="submit"><Gamepad2 :size="16" />开始本地体验</button>
      </form>
    </div>
    <section v-else-if="room && activeStory" class="tavern-play-room">
      <aside class="tavern-room-roster"><img :src="activeStory.cover" alt="" width="160" height="180" /><h2>{{ room.title }}</h2><p>{{ room.role }} · 我</p><p class="tavern-start-meta">1 位本地体验者</p><hr /><h3>房间记录</h3><ol><li v-for="(choice, index) in room.choices" :key="index">{{ choice }}</li></ol><button class="tavern-start-button" @click="exportRecord"><Save :size="15" />导出记录</button></aside>
      <main class="tavern-story-stage">
        <div class="tavern-scene-progress"><span v-for="(item, index) in activeStory.scenes" :key="item.title" :class="{ passed: room.finished || index <= room.scene }" /></div>
        <aside v-if="lastResponse" class="tavern-choice-response" aria-live="polite"><strong>你的行动带来了什么</strong><p>{{ lastResponse }}</p></aside>
        <template v-if="!room.finished && scene"><p class="tavern-start-meta">第 {{ room.scene + 1 }} / {{ activeStory.scenes.length }} 幕</p><h2>{{ scene.title }}</h2><p class="tavern-narrative">{{ scene.text }}</p><button class="tavern-start-link" :aria-expanded="clueVisible" @click="clueVisible = !clueVisible"><BookOpen :size="16" />{{ clueVisible ? '收起线索' : '查看本幕线索' }}</button><blockquote v-if="clueVisible">{{ scene.clue }}</blockquote><h3>你的选择</h3><div class="tavern-choices"><button v-for="choice in scene.choices" :key="choice" @click="choose(choice)">{{ choice }}<ChevronRight :size="16" /></button></div></template>
        <template v-else><Flag :size="28" /><h2>本次体验结束</h2><h3>{{ ending.title }}</h3><p class="tavern-narrative">{{ ending.text }}</p><p v-if="activeStory.id === 'last-letter'" class="tavern-start-meta">{{ room.choices[2] === '档案员重新投递了旧信' ? '你的判断找到了真相。' : '揭晓：真正重新寄出信的是档案员。' }}</p><p class="tavern-start-meta">你的选择已保存在本机。可以导出复盘，或重新开始探索另一种结局。</p><button class="tavern-start-button" @click="tab = 'catalog'"><RotateCcw :size="16" />选择下一段故事</button></template>
      </main>
    </section>
    <div v-else class="tavern-no-room"><Gamepad2 :size="32" /><h2>还没有体验房间</h2><button class="tavern-start-button" @click="tab = 'catalog'">选择故事</button></div>
  </section>
</template>

<style scoped>
.tavern-choice-response { padding: 12px 16px; border-left: 3px solid var(--bd-accent-teal); background: var(--bd-surface); border-radius: 0 8px 8px 0; line-height: 1.8; margin-bottom: 20px; }
.tavern-choice-response strong { color: var(--bd-accent-teal); font-size: 12px; }
.tavern-choice-response p { margin-top: 4px; font-size: 13px; color: var(--bd-text-secondary); }
.starter-tavern{color:var(--bd-text-primary);font-size:14px}.tavern-start-head{display:flex;justify-content:space-between;align-items:center;gap:16px;margin-bottom:20px}.starter-tavern h1{font-size:24px;font-weight:700}.starter-tavern h2{font-size:18px;font-weight:650}.starter-tavern h3{font-size:14px;font-weight:650}.tavern-start-meta{color:var(--bd-text-secondary);font-size:12px;line-height:1.7}.tavern-start-tabs{display:flex;gap:24px;border-bottom:1px solid var(--bd-ui-line)}.tavern-start-tabs button{display:flex;gap:8px;align-items:center;padding:12px 0;border-bottom:2px solid transparent;font-size:13px}.tavern-start-tabs .active{border-color:var(--bd-accent-teal);color:var(--bd-accent-teal)}.tavern-start-mode{font-size:12px;color:var(--bd-text-secondary);margin:16px 0 24px}.tavern-start-grid{display:grid;grid-template-columns:minmax(0,1fr) 280px;gap:24px;align-items:start}.tavern-stories{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(250px,100%),1fr));gap:16px}.tavern-story{border:1px solid var(--bd-ui-line);border-radius:8px;background:var(--bd-surface);overflow:hidden;min-width:0}.tavern-story.selected{border-color:var(--bd-accent-teal)}.tavern-story-art{height:220px;background:var(--bd-canvas);overflow:hidden}.tavern-story-art img{width:100%;height:100%;object-fit:contain}.tavern-story-copy{padding:20px}.tavern-story-copy h2{margin:8px 0 12px}.tavern-story-copy p{color:var(--bd-text-secondary);font-size:13px;line-height:1.8;min-height:72px;margin-bottom:16px}.tavern-story-copy .tavern-start-meta{display:flex;align-items:center;gap:6px;flex-wrap:wrap}.tavern-start-button{display:inline-flex;align-items:center;justify-content:center;gap:8px;min-height:40px;padding:8px 12px;border:1px solid var(--bd-ui-line);border-radius:6px;background:var(--bd-surface);font-size:13px;margin-top:16px}.tavern-start-button.primary{background:var(--bd-accent-teal);color:var(--bd-surface)}.tavern-start-link{display:inline-flex;align-items:center;gap:6px;color:var(--bd-accent-teal);font-size:12px}.tavern-room-config{display:grid;gap:12px;border-left:1px solid var(--bd-ui-line);padding-left:24px}.tavern-room-config label{display:grid;gap:8px;font-size:12px}.tavern-room-config input,.tavern-room-config select{min-width:0;width:100%;min-height:40px;padding:8px;border:1px solid var(--bd-ui-line);background:var(--bd-surface);color:var(--bd-text-primary);border-radius:6px}.tavern-config-divider{border-top:1px solid var(--bd-ui-line);padding-top:20px;margin-top:8px}.tavern-config-row{display:grid;grid-template-columns:1fr 1fr;gap:12px}.tavern-play-room{display:grid;grid-template-columns:220px minmax(0,1fr);gap:32px;min-height:560px}.tavern-room-roster{border-right:1px solid var(--bd-ui-line);padding-right:24px}.tavern-room-roster img{width:100%;height:180px;object-fit:contain}.tavern-room-roster p{margin:12px 0}.tavern-room-roster hr{margin:24px 0;border-color:var(--bd-ui-line)}.tavern-room-roster ol{padding:12px 0 12px 16px;list-style:decimal;color:var(--bd-text-secondary);font-size:12px;line-height:1.8}.tavern-room-roster li{margin-bottom:8px;overflow-wrap:anywhere}.tavern-story-stage{max-width:760px;min-width:0;padding-top:12px}.tavern-story-stage h2{font-size:24px;margin:16px 0}.tavern-narrative{font-size:16px;line-height:2.1;margin:20px 0 28px;white-space:pre-wrap}.tavern-story-stage blockquote{padding:16px;border-left:3px solid var(--bd-accent-gold);background:var(--bd-surface);margin:20px 0;font-size:13px;line-height:1.8}.tavern-story-stage h3{margin:28px 0 16px}.tavern-scene-progress{display:flex;gap:8px;margin-bottom:24px}.tavern-scene-progress span{height:4px;flex:1;background:var(--bd-ui-line)}.tavern-scene-progress .passed{background:var(--bd-accent-teal)}.tavern-choices{display:grid;gap:12px}.tavern-choices button{display:flex;justify-content:space-between;align-items:center;gap:12px;text-align:left;padding:16px;background:var(--bd-surface);border:1px solid var(--bd-ui-line);border-radius:6px;min-height:52px}.tavern-choices button:hover{border-color:var(--bd-accent-teal);color:var(--bd-accent-teal)}.tavern-choices svg{flex-shrink:0}.tavern-no-room{display:flex;flex-direction:column;gap:16px;align-items:center;padding:64px 20px}.starter-tavern :focus-visible{outline:2px solid var(--bd-accent-teal);outline-offset:3px}
@media(max-width:1100px){.tavern-start-grid{grid-template-columns:1fr}.tavern-room-config{border-left:0;border-top:1px solid var(--bd-ui-line);padding:24px 0 0;grid-template-columns:1fr 1fr}.tavern-room-config>h2,.tavern-room-config>p,.tavern-config-divider{grid-column:1/-1}.tavern-play-room{grid-template-columns:170px minmax(0,1fr);gap:20px}}
@media(max-width:600px){.tavern-play-room{grid-template-columns:1fr}.tavern-room-roster{border-right:0;border-bottom:1px solid var(--bd-ui-line);padding:0 0 20px}.tavern-room-roster img{width:100px;height:100px;float:right}.tavern-room-config{grid-template-columns:1fr}.tavern-story-stage h2{font-size:20px}.tavern-narrative{font-size:14px}.tavern-start-head{align-items:flex-start}.tavern-start-head>a{margin-top:12px}.tavern-story-copy p{min-height:0}}
</style>
