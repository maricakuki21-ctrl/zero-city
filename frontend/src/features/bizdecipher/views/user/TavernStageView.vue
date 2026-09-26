<template>
  <AppLayout>
    <main class="tavern-stage-page">
      <section class="stage-hero">
        <div>
          <p class="stage-kicker">零号城 · 酒馆舞台</p>
          <h1>{{ runtimeConfig?.room.title || '酒馆舞台' }}</h1>
          <p>
            {{ runtimeConfig?.script.title || '房间舞台' }}
          </p>
        </div>
        <div class="stage-actions">
          <button type="button" class="stage-secondary-button" @click="goBack">
            <Icon name="arrowLeft" size="sm" />
            返回我的房间
          </button>
          <button type="button" class="stage-primary-button" :disabled="loading" @click="loadRuntimeSession">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            {{ loading ? '同步中...' : '同步配置' }}
          </button>
        </div>
      </section>

      <section v-if="missingToken" class="stage-state stage-state-warning">
        <Icon name="key" size="xl" />
        <div>
          <h2>舞台凭证不存在或已离开当前浏览器会话</h2>
          <p>请从“我的房间”重新点击进入舞台。短期 token 不会写入地址栏，避免被日志和复制链接泄露。</p>
        </div>
      </section>

      <section v-else-if="loading && !runtimeConfig" class="stage-grid">
        <article v-for="item in 4" :key="item" class="stage-card stage-skeleton">
          <span></span>
          <strong></strong>
          <p></p>
        </article>
      </section>

      <section v-else-if="loadError" class="stage-state stage-state-error">
        <Icon name="exclamationCircle" size="xl" />
        <div>
          <h2>舞台配置加载失败</h2>
          <p>{{ loadError }}</p>
        </div>
      </section>

      <section v-else-if="runtimeConfig" class="stage-shell">
        <aside class="stage-left">
          <article class="stage-room-card">
            <div class="stage-card-head">
              <span class="stage-pill">{{ roomStatusLabel(runtimeConfig.room.status) }}</span>
              <span class="stage-pill muted">{{ roleLabel(runtimeConfig.participant.role) }}</span>
            </div>
            <h2>{{ runtimeConfig.room.title }}</h2>
            <p>{{ runtimeConfig.script.summary || runtimeConfig.room.script_title }}</p>
            <div class="stage-room-stats">
              <span>
                <strong>{{ runtimeConfig.room.current_players }}/{{ runtimeConfig.room.max_players }}</strong>
                玩家
              </span>
              <span>
                <strong>{{ pricingLabel(runtimeConfig.budget.billing_mode, runtimeConfig.budget.entry_credit_cost, runtimeConfig.budget.entry_balance_cost) }}</strong>
                入场
              </span>
              <span>
                <strong>{{ runtimeConfig.budget.turn_budget }}</strong>
                回合预算
              </span>
            </div>
          </article>

          <article class="stage-card">
            <div class="stage-card-title">
              <Icon name="book" size="sm" />
              <h3>剧本档案</h3>
            </div>
            <dl class="stage-definition-list">
              <div>
                <dt>剧本</dt>
                <dd>{{ runtimeConfig.script.title }}</dd>
              </div>
              <div>
                <dt>作者</dt>
                <dd>{{ runtimeConfig.script.author || '未知' }}</dd>
              </div>
              <div>
                <dt>难度</dt>
                <dd>{{ difficultyLabel(runtimeConfig.script.difficulty) }}</dd>
              </div>
              <div>
                <dt>预计时长</dt>
                <dd>{{ runtimeConfig.script.estimated_minutes }} 分钟</dd>
              </div>
              <div>
                <dt>游戏包</dt>
                <dd>{{ runtimeConfig.package ? `v${runtimeConfig.package.version} · ${packageStatusLabel(runtimeConfig.package.status)}` : '未绑定' }}</dd>
              </div>
            </dl>
          </article>

          <article class="stage-card">
            <div class="stage-card-title">
              <Icon name="users" size="sm" />
              <h3>NPC / 角色卡</h3>
            </div>
            <div v-if="runtimeConfig.prompts.npc_cards.length" class="stage-chip-list">
              <span v-for="npc in runtimeConfig.prompts.npc_cards" :key="npc" class="stage-chip">{{ npc }}</span>
            </div>
            <p v-else class="stage-muted">当前剧本还没有配置 NPC 卡。后续外部运行时子应用可从这里接收角色卡映射。</p>
          </article>
        </aside>

        <section class="stage-main">
          <article class="stage-runtime-board">
            <div class="stage-runtime-topline">
              <Icon name="document" size="sm" />
              <span>房间配置已读取 · 非实时连接</span>
            </div>
            <h2>{{ runtimeConfig.room.status === 'running' ? '故事进行中' : '开场准备' }}</h2>
            <p>
              {{ runtimeConfig.prompts.opening_prompt || runtimeConfig.script.summary }}
            </p>
            <div class="stage-runtime-grid">
              <div>
                <Icon name="shield" size="sm" />
                <span>运行沙箱</span>
                <strong>{{ runtimeConfig.bridge.sandbox_mode || '未声明' }}</strong>
              </div>
              <div>
                <Icon name="document" size="sm" />
                <span>游戏协议</span>
                <strong>{{ runtimeConfig.bridge.protocol_version || runtimeConfig.bridge.config_version || '未声明' }}</strong>
              </div>
              <div>
                <Icon name="server" size="sm" />
                <span>包运行方式</span>
                <strong>{{ runtimeConfig.package?.runtime_kind || runtimeConfig.bridge.provider }}</strong>
              </div>
              <div>
                <Icon name="key" size="sm" />
                <span>session</span>
                <strong>#{{ sessionID }}</strong>
              </div>
            </div>
          </article>

          <TavernAIHost
            v-if="runtimeConfig.participant.role === 'owner' && runtimeConfig.room.status === 'running'"
            :room-id="runtimeConfig.room.id"
            :through-turn="turns.reduce((last, turn) => Math.max(last, turn.turn_index), 0)"
            @recorded="loadTurns()"
          />

          <article class="stage-card stage-transcript-card">
            <div class="stage-card-title">
              <Icon name="chat" size="sm" />
              <h3>房间回合</h3>
            </div>
            <div v-if="turnsLoading && turns.length === 0" class="stage-muted" role="status">正在读取房间回合…</div>
            <div v-else-if="turnsError && turns.length === 0" class="stage-turn-error" role="alert">
              <span>{{ turnsError }}</span>
              <button type="button" class="stage-secondary-button compact" @click="loadTurns()">重新读取</button>
            </div>
            <div v-else-if="turns.length === 0" class="stage-muted">
              还没有成员留下回合。房间开始后，已加入成员可以在这里记录行动与发言。
            </div>
            <ol v-else class="stage-turn-list" aria-live="polite">
              <li v-for="turn in turns" :key="turn.id" class="stage-turn">
                <div class="stage-turn-head">
                  <strong>{{ turn.author_name || `用户 #${turn.author_user_id}` }}</strong>
                  <span>{{ turn.author_role === 'owner' ? '房主' : '玩家' }} · 第 {{ turn.turn_index }} 回合</span>
                </div>
                <p>{{ turn.body }}</p>
                <time>{{ formatTurnTime(turn.created_at) }}</time>
              </li>
            </ol>
            <form v-if="canWriteTurns" class="stage-turn-composer" @submit.prevent="sendTurn">
              <textarea
                v-model="turnBody"
                rows="3"
                maxlength="4000"
                placeholder="记录你的行动、发言或判断"
                aria-label="房间回合内容"
              />
              <div>
                <span>{{ turnBody.trim().length }}/4000</span>
                <button type="submit" class="stage-primary-button compact" :disabled="sendingTurn || !turnBody.trim()">
                  {{ sendingTurn ? '写入中...' : '写入回合' }}
                </button>
              </div>
              <p v-if="turnsError" class="stage-turn-error" role="alert">{{ turnsError }}</p>
            </form>
            <p v-else class="stage-muted">
              {{ runtimeConfig.room.status === 'running' ? '只有房间成员可以写入回合。' : '房间结束后，回合记录保持只读。' }}
            </p>
          </article>

          <div class="stage-prompt-grid">
            <article class="stage-card stage-prompt-card">
              <div class="stage-card-title">
                <Icon name="sparkles" size="sm" />
                <h3>主持人提示</h3>
              </div>
              <p>{{ runtimeConfig.prompts.host_brief || '暂无主持提示。建议在剧本投稿时补充节奏、线索释放规则和禁区。' }}</p>
            </article>
            <article class="stage-card stage-prompt-card">
              <div class="stage-card-title">
                <Icon name="chat" size="sm" />
                <h3>开场提示</h3>
              </div>
              <p>{{ runtimeConfig.prompts.opening_prompt || '暂无开场提示。后续 AI 主持会优先读取这里作为第一幕启动内容。' }}</p>
            </article>
          </div>

          <article class="stage-card">
            <div class="stage-card-title">
              <Icon name="infoCircle" size="sm" />
              <h3>上线边界</h3>
            </div>
            <ul class="stage-boundary-list">
              <li>已闭合：短期舞台凭证、房间成员权限、状态硬闸、runtime 配置读取。</li>
              <li>已闭合：成员多轮记录、幂等重试、完成后回看和跨账号响应隔离。</li>
              <li>AI 主持由房主确认报价后调用，费用沿用工作台用量结算；不向玩家自动分摊费用。</li>
              <li>游戏作者分成和任意脚本运行暂未开放。</li>
              <li>隔离要求：外部运行时独立构建、独立部署、独立声明，BizDecipher 主仓只保留桥接配置和业务治理。</li>
            </ul>
          </article>
        </section>
      </section>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import TavernAIHost from '@/features/bizdecipher/components/tavern/TavernAIHost.vue'
import {
  appendTavernRoomTurn,
  getTavernRuntimeSession,
  listTavernRoomTurns,
  type TavernRoomTurn,
  type TavernRuntimeConfig,
} from '@/features/bizdecipher/api/bizdecipher'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const loading = ref(false)
const loadError = ref('')
const missingToken = ref(false)
const runtimeConfig = ref<TavernRuntimeConfig | null>(null)
const turns = ref<TavernRoomTurn[]>([])
const turnsLoading = ref(false)
const turnsError = ref('')
const sendingTurn = ref(false)
const turnBody = ref('')
let loadGeneration = 0
let turnsGeneration = 0
let turnsTimer: ReturnType<typeof setInterval> | null = null

const sessionID = computed(() => String(route.query.session || ''))
const roomID = computed(() => String(route.query.room || ''))
const canWriteTurns = computed(() => runtimeConfig.value?.room.status === 'running' && !!runtimeConfig.value?.participant.role)

function tavernRuntimeSessionStorageKey(id: string): string {
  return `zero-city:tavern-runtime:${id}`
}

function readRuntimeToken(): string {
  if (!sessionID.value) return ''
  return sessionStorage.getItem(tavernRuntimeSessionStorageKey(sessionID.value)) || ''
}

async function loadRuntimeSession() {
  const generation = ++loadGeneration
  runtimeConfig.value = null
  missingToken.value = false
  loading.value = true
  loadError.value = ''
  try {
    const token = readRuntimeToken()
    if (!token) {
      missingToken.value = true
      return
    }
    const session = await getTavernRuntimeSession(token)
    if (generation !== loadGeneration) return
    if (!session.config) {
      throw new Error('服务器未返回舞台配置，请返回房间重新进入。')
    }
    runtimeConfig.value = session.config
    await loadTurns()
    startTurnPolling()
  } catch (error) {
    if (generation !== loadGeneration) return
    loadError.value = extractApiErrorMessage(error, '舞台配置加载失败')
    appStore.showError(loadError.value)
  } finally {
    if (generation === loadGeneration) loading.value = false
  }
}

async function loadTurns(reset = false) {
  const room = runtimeConfig.value?.room.id ?? Number(roomID.value)
  if (!room) return
  const generation = ++turnsGeneration
  turnsLoading.value = true
  try {
    const after = reset ? 0 : turns.value.at(-1)?.turn_index ?? 0
    const page = await listTavernRoomTurns(room, after, 100)
    if (generation !== turnsGeneration) return
    if (reset) turns.value = page
    else if (page.length) {
      const known = new Set(turns.value.map(turn => turn.id))
      turns.value = [...turns.value, ...page.filter(turn => !known.has(turn.id))]
    }
    turnsError.value = ''
  } catch (error) {
    if (generation !== turnsGeneration) return
    turnsError.value = extractApiErrorMessage(error, '房间回合读取失败')
  } finally {
    if (generation === turnsGeneration) turnsLoading.value = false
  }
}

function startTurnPolling() {
  stopTurnPolling()
  if (runtimeConfig.value?.room.status !== 'running') return
  turnsTimer = setInterval(() => { void loadTurns() }, 5000)
}

function stopTurnPolling() {
  if (turnsTimer) {
    clearInterval(turnsTimer)
    turnsTimer = null
  }
}

function nextTurnMessageID(): string {
  return globalThis.crypto?.randomUUID?.() ?? `turn-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

async function sendTurn() {
  const room = runtimeConfig.value?.room
  const body = turnBody.value.trim()
  const targetRoomID = room?.id ?? Number(roomID.value)
  if (!room || !targetRoomID || room.status !== 'running' || !body || sendingTurn.value) return
  sendingTurn.value = true
  turnsError.value = ''
  try {
    const saved = await appendTavernRoomTurn(targetRoomID, {
      client_message_id: nextTurnMessageID(),
      body,
    })
    if (!turns.value.some(turn => turn.id === saved.id)) turns.value.push(saved)
    turnBody.value = ''
  } catch (error) {
    turnsError.value = extractApiErrorMessage(error, '回合写入失败，内容已保留，可直接重试')
  } finally {
    sendingTurn.value = false
  }
}

function formatTurnTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '时间未记录' : date.toLocaleString('zh-CN', { hour12: false })
}

function goBack() {
  router.push({ name: 'ZeroCityTavern', query: { view: 'rooms', ...(roomID.value ? { room: roomID.value } : {}) } })
}

function roomStatusLabel(value: string): string {
  const map: Record<string, string> = {
    draft: '草稿',
    lobby: '大厅',
    running: '进行中',
    paused: '暂停',
    completed: '已完成',
    cancelled: '已取消',
  }
  return map[value] ?? value
}

function roleLabel(value: string): string {
  const map: Record<string, string> = { owner: '房主', player: '玩家' }
  return map[value] ?? value
}

function difficultyLabel(value: string): string {
  const map: Record<string, string> = { easy: '入门', normal: '标准', hard: '进阶', expert: '专家' }
  return map[value] ?? '标准'
}

function packageStatusLabel(value: string): string {
  const map: Record<string, string> = { draft: '草稿', published: '已发布', revoked: '已撤销' }
  return map[value] ?? value
}

function pricingLabel(mode: string, credit: number, balance: number): string {
  if (mode === 'credit') return `${credit} 积分`
  if (mode === 'balance') return `¥${balance.toFixed(2)}`
  if (mode === 'hybrid') return `${credit} 积分 + ¥${balance.toFixed(2)}`
  return '免费'
}

watch([sessionID, roomID], () => {
  turnsGeneration++
  turns.value = []
  turnsError.value = ''
  turnBody.value = ''
  stopTurnPolling()
  void loadRuntimeSession()
}, { immediate: true })
onBeforeUnmount(() => {
  loadGeneration++
  turnsGeneration++
  stopTurnPolling()
})
</script>

<style scoped>
.tavern-stage-page {
  min-height: 100vh;
  padding: 32px clamp(18px, 4vw, 56px) 56px;
  background: var(--bd-canvas);
  color: var(--bd-text-primary);
}

.stage-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  max-width: 1280px;
  margin: 0 auto 24px;
}

.stage-hero h1 {
  margin: 8px 0 10px;
  font-size: 24px;
  line-height: 1.3;
  overflow-wrap: anywhere;
  letter-spacing: 0;
}

.stage-hero p {
  max-width: 760px;
  margin: 0;
  color: #5f6d66;
  line-height: 1.8;
}

.stage-kicker {
  margin: 0;
  color: #1d8b7b;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0;
  text-transform: uppercase;
}

.stage-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.stage-primary-button,
.stage-secondary-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 40px;
  padding: 0 16px;
  border-radius: 6px;
  border: 1px solid transparent;
  font-weight: 750;
  cursor: pointer;
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease;
}

.stage-primary-button {
  background: #10231d;
  color: #f8fffb;
}

.stage-secondary-button {
  background: rgba(255, 255, 255, 0.78);
  color: #20312b;
  border-color: rgba(33, 56, 47, 0.12);
}

.stage-primary-button:hover:not(:disabled),
.stage-secondary-button:hover:not(:disabled) {
  transform: translateY(-1px);
}

.stage-primary-button:disabled,
.stage-secondary-button:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}

.stage-shell,
.stage-grid {
  max-width: 1280px;
  margin: 0 auto;
}

.stage-shell {
  display: grid;
  grid-template-columns: minmax(280px, 380px) minmax(0, 1fr);
  gap: 18px;
}

.stage-left,
.stage-main {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.stage-card,
.stage-room-card,
.stage-runtime-board,
.stage-state {
  border-bottom: 1px solid var(--bd-ui-line);
  background: var(--bd-surface);
}

.stage-card,
.stage-room-card,
.stage-runtime-board {
  border-radius: 0;
  padding: 18px;
}

.stage-room-card {
  background: #203c35;
  color: #f9fff9;
  min-height: 260px;
}

.stage-card-head,
.stage-card-title,
.stage-runtime-topline {
  display: flex;
  align-items: center;
  gap: 10px;
}

.stage-card-head {
  justify-content: space-between;
  margin-bottom: 22px;
}

.stage-pill,
.stage-chip {
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  border-radius: 999px;
  padding: 0 10px;
  background: rgba(42, 157, 143, 0.14);
  color: #126b5f;
  font-size: 12px;
  font-weight: 750;
}

.stage-room-card .stage-pill {
  background: rgba(255, 255, 255, 0.16);
  color: #effff9;
}

.stage-pill.muted {
  background: rgba(112, 124, 117, 0.12);
  color: #5f6d66;
}

.stage-room-card .stage-pill.muted {
  background: rgba(255, 255, 255, 0.12);
  color: #d8eee6;
}

.stage-room-card h2,
.stage-runtime-board h2,
.stage-state h2 {
  margin: 0 0 10px;
  font-size: 20px;
  line-height: 1.12;
  letter-spacing: 0;
}

.stage-room-card p,
.stage-runtime-board p,
.stage-card p,
.stage-state p {
  margin: 0;
  color: #61716a;
  line-height: 1.75;
}

.stage-room-card p {
  color: #d9eee6;
}

.stage-room-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin-top: 28px;
}

.stage-room-stats span {
  min-width: 0;
  border-radius: 4px;
  padding: 12px;
  background: rgba(255, 255, 255, 0.12);
  color: #d8eee6;
  font-size: 12px;
}

.stage-room-stats strong {
  display: block;
  color: #ffffff;
  font-size: 18px;
  line-height: 1.2;
  word-break: break-word;
}

.stage-card-title {
  margin-bottom: 14px;
  color: #1d8b7b;
}

.stage-card-title h3 {
  margin: 0;
  color: #1b2f28;
  font-size: 16px;
  letter-spacing: 0;
}

.stage-definition-list {
  display: grid;
  gap: 12px;
  margin: 0;
}

.stage-definition-list div {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  padding-bottom: 10px;
  border-bottom: 1px solid rgba(23, 32, 28, 0.08);
}

.stage-definition-list div:last-child {
  border-bottom: 0;
  padding-bottom: 0;
}

.stage-definition-list dt {
  color: #77827d;
  font-size: 13px;
}

.stage-definition-list dd {
  margin: 0;
  color: #1b2f28;
  font-weight: 750;
  text-align: right;
}

.stage-chip-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.stage-muted {
  color: #75827b !important;
}

.stage-runtime-board {
  min-height: 320px;
  background: var(--bd-surface);
}

.stage-runtime-topline {
  color: #1d8b7b;
  font-weight: 800;
  font-size: 13px;
  margin-bottom: 18px;
}

.stage-runtime-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-top: 28px;
}

.stage-runtime-grid div {
  min-width: 0;
  border-radius: 0;
  padding: 14px;
  background: rgba(255, 255, 255, 0.75);
  border: 1px solid rgba(29, 139, 123, 0.1);
}

.stage-runtime-grid span,
.stage-runtime-grid strong {
  display: block;
}

.stage-runtime-grid span {
  margin: 8px 0 4px;
  color: #748078;
  font-size: 12px;
}

.stage-runtime-grid strong {
  color: #1b2f28;
  font-size: 13px;
  word-break: break-word;
}

.stage-prompt-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.stage-prompt-card {
  min-height: 180px;
}

.stage-boundary-list {
  display: grid;
  gap: 10px;
  margin: 0;
  padding-left: 18px;
  color: #5f6d66;
  line-height: 1.7;
}

.stage-transcript-card {
  display: grid;
  gap: 14px;
}

.stage-turn-list {
  display: grid;
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.stage-turn {
  display: grid;
  gap: 7px;
  border-left: 3px solid var(--bd-accent-teal);
  padding: 10px 12px;
  background: var(--bd-canvas);
}

.stage-turn-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}

.stage-turn-head strong {
  color: var(--bd-text-primary);
  font-size: 13px;
}

.stage-turn-head span,
.stage-turn time {
  color: var(--bd-text-secondary);
  font-size: 11px;
}

.stage-turn p {
  margin: 0;
  color: var(--bd-text-primary);
  line-height: 1.7;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.stage-turn-composer {
  display: grid;
  gap: 8px;
  border-top: 1px solid var(--bd-ui-line);
  padding-top: 14px;
}

.stage-turn-composer textarea {
  width: 100%;
  min-height: 84px;
  resize: vertical;
  border: 1px solid var(--bd-ui-line);
  border-radius: 6px;
  padding: 10px 12px;
  background: var(--bd-surface);
  color: var(--bd-text-primary);
  font: inherit;
  line-height: 1.6;
}

.stage-turn-composer > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.stage-turn-composer > div > span {
  color: var(--bd-text-secondary);
  font-size: 11px;
}

.stage-turn-error {
  margin: 0;
  color: var(--bd-status-danger);
  font-size: 12px;
  line-height: 1.6;
}

.stage-state {
  display: flex;
  align-items: center;
  gap: 18px;
  max-width: 960px;
  margin: 80px auto 0;
  border-radius: 6px;
  padding: 22px;
}

.stage-state-warning {
  color: #8a5b13;
  background: rgba(255, 249, 230, 0.92);
}

.stage-state-error {
  color: #9f2d2d;
  background: rgba(255, 242, 242, 0.92);
}

.stage-skeleton {
  min-height: 180px;
  overflow: hidden;
}

.stage-skeleton span,
.stage-skeleton strong,
.stage-skeleton p {
  display: block;
  border-radius: 999px;
  background: linear-gradient(90deg, rgba(220, 231, 225, 0.7), rgba(245, 249, 246, 0.9), rgba(220, 231, 225, 0.7));
  background-size: 200% 100%;
  animation: stageShimmer 1.4s ease-in-out infinite;
}

.stage-skeleton span {
  width: 90px;
  height: 22px;
}

.stage-skeleton strong {
  width: 70%;
  height: 28px;
  margin: 22px 0 16px;
}

.stage-skeleton p {
  width: 95%;
  height: 16px;
}

.animate-spin {
  animation: stageSpin 0.8s linear infinite;
}

@keyframes stageSpin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@keyframes stageShimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

@media (max-width: 960px) {
  .stage-hero,
  .stage-shell {
    grid-template-columns: 1fr;
  }

  .stage-hero {
    flex-direction: column;
  }

  .stage-actions {
    justify-content: flex-start;
  }

  .stage-runtime-grid,
  .stage-prompt-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .tavern-stage-page {
    padding: 20px 14px 40px;
  }

  .stage-room-stats,
  .stage-runtime-grid,
  .stage-prompt-grid {
    grid-template-columns: 1fr;
  }

  .stage-primary-button,
  .stage-secondary-button {
    width: 100%;
  }
}

.stage-left,
.stage-main {
  min-width: 0;
  overflow-wrap: anywhere;
}

.stage-primary-button:focus-visible,
.stage-secondary-button:focus-visible {
  outline: 2px solid var(--bd-accent-teal);
  outline-offset: 3px;
}

@media (prefers-reduced-motion: reduce) {
  .animate-spin,
  .stage-skeleton span,
  .stage-skeleton strong,
  .stage-skeleton p {
    animation: none;
  }
}
</style>
