<template>
  <AppLayout>
    <div class="asmy-page shared-experience">
      <header class="asmy-header">
        <div class="asmy-header-copy">
          <h1 class="asmy-title">我的资源</h1>
          <div class="asmy-header-actions">
            <RouterLink to="/keys" class="asmy-btn"><KeyRound :size="15" />我的密钥</RouterLink>
            <RouterLink to="/account-square" class="asmy-btn asmy-btn-primary">发现资源</RouterLink>
          </div>
        </div>
      </header>

      <div v-if="loadErrors.length" class="asmy-error-banner" role="alert">
        <b>部分数据暂未加载</b>
        <span>{{ loadErrors.join('；') }}</span>
        <button class="asmy-btn asmy-btn-sm" type="button" @click="refreshAll">重新加载</button>
      </div>

      <nav class="asmy-tabs" role="tablist" aria-label="用户页面分区">
        <button
          v-for="tab in primaryTabs"
          :key="tab.key"
          type="button"
          class="asmy-tab"
          :class="{ active: activeGroup === tab.key }"
          :id="`member-tab-${tab.key}`"
          role="tab"
          :aria-selected="activeGroup === tab.key"
          :aria-controls="`member-panel-${tab.key}`"
          :tabindex="activeGroup === tab.key ? 0 : -1"
          @click="activeTab = tab.key"
          @keydown="handleMemberTabKeydown($event, tab.key)"
        >
          {{ tab.label }}
        </button>
      </nav>
      <div :id="`member-panel-${activeGroup}`" role="tabpanel" :aria-labelledby="`member-tab-${activeGroup}`">
      <nav v-if="activeGroup === 'ledger'" class="member-subtabs" aria-label="账单分区">
        <button :class="{ active: activeTab === 'ledger' }" :aria-pressed="activeTab === 'ledger'" @click="activeTab = 'ledger'">消费明细</button>
        <button :class="{ active: activeTab === 'seats' }" :aria-pressed="activeTab === 'seats'" @click="activeTab = 'seats'">席位费用与退出记录</button>
      </nav>

      <!-- 已加入池 -->
      <section v-if="activeTab === 'pools'" class="asmy-section">
        <div class="asmy-section-head">
          <div class="member-resource-toolbar">
          <label class="member-resource-search"><Search :size="16" /><input v-model="resourceQuery" type="search" aria-label="搜索已加入资源" placeholder="搜索共享池名称或编号" /></label>
          <label class="member-state-filter">状态<select v-model="resourceStatus" aria-label="筛选资源状态"><option value="all">全部未退出</option><option value="available">有效席位</option><option value="unavailable">暂不可用</option></select></label>
          </div>
          <button class="asmy-btn asmy-btn-sm" type="button" :disabled="loading" title="刷新资源" aria-label="刷新资源" @click="refreshAll"><RefreshCw :size="15" /></button>
        </div>

        <div v-if="loading" class="asmy-loading"><div class="asmy-spinner" /><span>加载中…</span></div>
        <div v-else-if="currentSeats.length === 0" class="asmy-empty">
          <img :src="zeroCityMascots.keyKeeper" width="80" height="80" alt="" />
          <p class="asmy-empty-title">还没有加入任何共享池</p>
          <p class="asmy-empty-sub">选一个适合的池，加入后就能在这里管理。</p>
          <RouterLink to="/account-square" class="asmy-btn asmy-btn-primary">前往市场</RouterLink>
        </div>
        <div v-else-if="!visibleSeats.length" class="asmy-empty"><p>没有匹配的共享池</p><button class="asmy-btn" @click="resourceQuery = ''; resourceStatus = 'all'">重置筛选</button></div>
        <div v-else class="member-resource-table-wrap" role="region" aria-label="已开通资源" tabindex="0">
          <table class="member-resource-table">
            <thead><tr><th scope="col">资源</th><th scope="col">状态</th><th scope="col">本窗消费</th><th scope="col">累计席位扣费</th><th scope="col">密钥与操作</th></tr></thead>
            <tbody>
              <tr v-for="seat in visibleSeats" :key="seat.id" class="member-resource-row">
                <td>
                  <RouterLink :to="`/account-square/pool/${seat.pool_id}`" class="member-resource-name">{{ seat.pool_name || `池 #${seat.pool_id}` }}</RouterLink>
                  <small>#{{ seat.pool_id }} · 席位费 {{ formatSeatFee(seat.hourly_seat_fee) }}</small>
                  <details class="member-details">
                    <summary>费用与席位详情</summary>
                    <p>{{ seatWindowText(seat) }}</p><p>{{ seatUsageGuidance(seat) }}</p>
                    <dl class="member-seat-facts">
                      <dt>减免线</dt><dd>{{ formatMoney(seat.hourly_min_usage_waiver) }}/h</dd>
                      <dt>预计席位费</dt><dd>{{ formatMoney(estimateFinalSeatFee(seat)) }}</dd>
                      <dt>加入时间</dt><dd>{{ formatDateTime(seat.joined_at) }}</dd>
                      <dt>最近活跃</dt><dd>{{ formatDateTime(seat.last_activity_at || seat.joined_at) }}</dd>
                      <dt>预计自动释放</dt><dd>{{ seatIdleReleaseText(seat) }}</dd>
                    </dl>
                  </details>
                </td>
                <td>
                  <span class="asmy-chip" :class="seatStatusClass(seat)">{{ seatStatusLabel(seat) }}</span>
                  <template v-if="!isSeatAvailable(seat)">
                    <small>{{ seat.release_reason ? releaseReasonLabel(seat.release_reason) : '席位暂不可用于路由，请查看池详情确认原因' }}</small>
                    <RouterLink to="/account-square" class="asmy-link">换个资源</RouterLink>
                  </template>
                </td>
                <td class="member-money">{{ formatMoney(seat.current_hour_usage_amount) }}</td>
                <td class="member-money">{{ formatMoney(seat.total_charged) }}</td>
                <td>
                  <small>{{ boundPoolIds.has(seat.pool_id) ? '共享密钥已绑定' : '尚未绑定密钥' }}</small>
                  <div class="member-row-actions">
                    <button class="asmy-btn asmy-btn-sm" type="button" :disabled="creatingKeyPoolId === seat.pool_id || !isSeatAvailable(seat)" @click="openPoolKey(seat)"><KeyRound :size="15" />{{ creatingKeyPoolId === seat.pool_id ? '处理中…' : boundPoolIds.has(seat.pool_id) ? '查看 Key' : '生成 Key' }}</button>
                    <button class="asmy-btn asmy-btn-sm" type="button" :aria-label="`查看 ${seat.pool_name || seat.pool_id} 的消费记录`" @click="openPoolLedger(seat.pool_id)">消费记录</button>
                    <button v-if="isSeatAvailable(seat)" class="asmy-btn asmy-btn-sm asmy-btn-danger" type="button" :disabled="leavingPoolId === seat.pool_id" @click="leavePool(seat)">{{ leavingPoolId === seat.pool_id ? '退出中…' : '退出' }}</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <!-- 席位费用 -->
      <section v-if="activeTab === 'seats'" class="asmy-section">
        <div class="asmy-section-head">
          <div>
            <h2>席位与费用</h2>
            <p>连续消费抵扣、小时最终结算；当前窗口的费率和门槛冻结，修改只影响后续窗口。</p>
          </div>
        </div>

        <details class="asmy-rule-card member-details">
          <summary>席位费怎么算</summary>
          <ul>
            <li>加入后每满 1 小时结算一次；即使没有调用，也可能产生席位费。</li>
            <li>本小时调用越多，席位费抵扣越多；达到页面显示的减免线，本小时席位费全免。</li>
            <li>页面会持续显示本小时消费、还差多少全免和预计最终席位费。</li>
            <li>余额不足不会扣成负数，系统会停止席位和对应共享池 Key。</li>
          </ul>
        </details>

        <div v-if="activeJoinedPools.length === 0" class="asmy-empty">
          <p class="asmy-empty-title">暂无席位</p>
          <p class="asmy-empty-sub">加入共享池后，这里会显示每个席位的窗口进度与预计费用。</p>
        </div>
        <div v-else class="asmy-seat-list">
          <article v-for="seat in activeJoinedPools" :key="`seat-${seat.id}`" class="asmy-seat-card">
            <div class="asmy-seat-head">
              <b>{{ seat.pool_name || `池 #${seat.pool_id}` }}</b>
              <span>{{ seat.current_hour_waiver_met ? '本窗已达全免' : '连续抵扣进行中' }}</span>
            </div>
            <div class="asmy-progress">
              <div class="asmy-progress-bar" :style="{ width: `${seatProgress(seat)}%` }" />
            </div>
            <div class="asmy-metric-grid">
              <span><small>本小时基础席位费</small><strong>{{ formatMoney(seat.hourly_seat_fee) }}</strong></span>
              <span><small>全免消费门槛</small><strong>{{ formatMoney(seat.hourly_min_usage_waiver) }}</strong></span>
              <span><small>本小时有效消费</small><strong>{{ formatMoney(seat.current_hour_usage_amount) }}</strong></span>
              <span><small>已抵扣席位费</small><strong>{{ formatMoney(estimateWaiver(seat)) }}</strong></span>
              <span><small>预计最终席位费</small><strong>{{ formatMoney(estimateFinalSeatFee(seat)) }}</strong></span>
              <span><small>距全免还差</small><strong>{{ formatMoney(remainingToFullWaiver(seat)) }}</strong></span>
            </div>
          </article>
        </div>
        <div v-if="historicalSeats.length" class="asmy-history">
          <div class="asmy-section-head">
            <div>
              <h3>退出历史</h3>
              <p>历史席位只用于核对，不再参与扣费、路由或组合 Key 绑定。</p>
            </div>
          </div>
          <div class="asmy-history-list">
            <article v-for="seat in historicalSeats" :key="`history-${seat.id}`">
              <div><b>{{ seat.pool_name || `池 #${seat.pool_id}` }}</b><span>{{ releaseReasonLabel(seat.release_reason) }}</span></div>
              <small>加入 {{ formatDateTime(seat.joined_at) }} · 退出 {{ formatDateTime(seat.released_at || seat.last_activity_at || seat.joined_at) }}</small>
            </article>
          </div>
        </div>
      </section>

      <!-- 组合 Key -->
      <section v-if="activeTab === 'unified'" class="asmy-section">
        <div class="asmy-section-head">
          <div>
            <h2>资源绑定</h2>
            <p>一个 <code>sk-share</code> 管理多个已加入共享池。调用时按模型与池状态命中具体池，费用与分成只记到成功命中的池。</p>
          </div>
          <button class="asmy-btn asmy-btn-sm" type="button" :disabled="keysLoading || loading" @click="refreshAll">
            {{ keysLoading || loading ? '刷新中…' : '刷新' }}
          </button>
        </div>

        <details class="asmy-rule-card member-details">
          <summary>组合路由规则</summary>
          <ul>
            <li>所有密钥均可在“我的密钥”查看；此处管理已加入资源的绑定。</li>
            <li>首次为任一已加入池生成 Key 时创建组合入口；后续池绑定到同一把 Key。</li>
            <li>请求带模型名时，在已绑定且席位有效的池中选择可服务该模型的池。</li>
            <li>每次成功调用记录最终命中池与扣费归属；失败切换时只对成功命中池计费。</li>
          </ul>
        </details>

        <div v-if="activeJoinedPools.length === 0" class="asmy-empty">
          <p class="asmy-empty-title">还没有可纳入组合的池</p>
          <p class="asmy-empty-sub">先加入至少一个共享池，才能创建组合 Key 并绑定池。</p>
          <RouterLink to="/account-square" class="asmy-btn asmy-btn-primary">前往市场</RouterLink>
        </div>

        <template v-else>
          <div class="asmy-unified-card">
            <div class="asmy-unified-copy">
              <h3>组合 Key 入口</h3>
              <div class="asmy-key-value-row" style="margin-top: 12px">
                <code>{{ unifiedKeyDisplay }}</code>
                <button
                  class="asmy-btn asmy-btn-sm"
                  type="button"
                  :disabled="!lastCreatedKey && !unifiedKeyValue"
                  @click="copyUnifiedKey"
                >
                  复制完整 Key
                </button>
              </div>
              <p class="asmy-muted" style="margin-top: 10px">
                已绑定 {{ boundPoolIds.size }} / {{ activeJoinedPools.length }} 个有效席位
                · 绑定记录 {{ accessKeys.length }} 条
                · 可路由模型约 {{ unifiedModelCount }} 个
              </p>
              <p class="asmy-muted" style="margin-top: 8px">完整 Key 仅向登录本人显示，可随时返回本页复制或删除。</p>
            </div>
            <div class="asmy-unified-side">
              <div class="asmy-unified-status">
                <b>绑定进度</b>
                <p>未绑定池：{{ unboundPools.length }} · 已绑定池：{{ boundPools.length }}</p>
                <div class="asmy-unified-pools">
                  <span v-for="seat in boundPools.slice(0, 6)" :key="`b-${seat.id}`">{{ seat.pool_name || `#${seat.pool_id}` }}</span>
                  <span v-if="boundPools.length === 0">尚未绑定</span>
                </div>
              </div>
              <button
                class="asmy-btn asmy-btn-primary"
                type="button"
                :disabled="bindingAll || unboundPools.length === 0"
                @click="bindAllJoinedPools"
              >
                {{ bindingAll ? '绑定中…' : unboundPools.length === 0 ? '已绑定全部已加入池' : `绑定全部未绑定池（${unboundPools.length}）` }}
              </button>
              <RouterLink class="asmy-btn asmy-btn-sm" to="/keys">查看全部密钥</RouterLink>
            </div>
          </div>

          <div class="asmy-section-head" style="margin-top: 4px">
            <div>
              <h2>池绑定管理</h2>
              <p>为每个已加入池生成或确认组合绑定。已绑定池参与同一把 Key 的模型路由。</p>
            </div>
          </div>

          <div class="asmy-pool-list">
            <article v-for="seat in activeJoinedPools" :key="`bind-${seat.id}`" class="asmy-pool-card">
              <div class="asmy-pool-main">
                <div class="asmy-pool-avatar">{{ poolInitial(seat.pool_name) }}</div>
                <div class="asmy-pool-info">
                  <div class="asmy-pool-title-row">
                    <b>{{ seat.pool_name || `池 #${seat.pool_id}` }}</b>
                    <span v-if="boundPoolIds.has(seat.pool_id)" class="asmy-chip asmy-chip-teal">已绑定</span>
                    <span v-else class="asmy-chip">未绑定</span>
                    <span class="asmy-chip">{{ seat.status || 'unknown' }}</span>
                  </div>
                  <div class="asmy-metric-grid">
                    <span><small>席位费</small><strong>{{ formatSeatFee(seat.hourly_seat_fee) }}</strong></span>
                    <span><small>本窗消费</small><strong>{{ formatMoney(seat.current_hour_usage_amount) }}</strong></span>
                    <span><small>绑定模型</small><strong>{{ modelsForPool(seat.pool_id) }}</strong></span>
                    <span><small>累计 Key 消费</small><strong>{{ formatMoney(usageForPool(seat.pool_id)) }}</strong></span>
                  </div>
                </div>
              </div>
              <div class="asmy-pool-actions">
                <RouterLink :to="`/account-square/pool/${seat.pool_id}`" class="asmy-btn asmy-btn-sm">池详情</RouterLink>
                <button
                  class="asmy-btn asmy-btn-sm asmy-btn-primary"
                  type="button"
                  :disabled="creatingKeyPoolId === seat.pool_id || bindingAll"
                  @click="bindPoolToUnifiedKey(seat)"
                >
                  {{ creatingKeyPoolId === seat.pool_id ? '处理中…' : boundPoolIds.has(seat.pool_id) ? '刷新绑定' : '加入组合 Key' }}
                </button>
              </div>
            </article>
          </div>
        </template>
      </section>

      <!-- 账本 -->
      <section v-if="activeTab === 'ledger'" class="asmy-section">
        <div v-if="ledgerPoolId !== null" class="member-ledger-filter" role="status">当前池 #{{ ledgerPoolId }}<button class="asmy-btn asmy-btn-sm" @click="ledgerPoolId = null">查看全部账单</button></div>
        <div class="asmy-section-head">
          <div>
            <h2>消费账本</h2>
            <p>这里记录共享池 API 调用、席位费、退款与冲正。席位费按整小时结算，不会出现在「使用记录」页。</p>
          </div>
          <button class="asmy-btn asmy-btn-sm" type="button" :disabled="ledgerLoading" @click="loadLedger">
            {{ ledgerLoading ? '加载中…' : '刷新' }}
          </button>
        </div>

        <details class="asmy-rule-card member-details" style="margin-bottom: 12px">
          <summary>如何理解扣费</summary>
          <ul>
            <li>「使用记录」只展示普通网关 API 请求，不含共享池席位费。</li>
            <li>共享池 API 扣费与席位费都在本页账本；标题会标明类型、池编号与结算窗口。</li>
            <li>加入后满 1 小时才会结算席位费；若不想继续占用，请及时退出。</li>
          </ul>
        </details>
        <details class="member-details"><summary>查询单次调用与费用归属</summary><SharedPoolUsageTracePanel /></details>
        <div v-if="ledgerLoading" class="asmy-loading"><div class="asmy-spinner" /><span>加载账本…</span></div>
        <div v-else-if="visibleActivity.length + visibleIncentives.length === 0" class="asmy-empty">
          <p class="asmy-empty-title">暂无账单记录</p>
          <p class="asmy-empty-sub">加入并实际使用共享池后，扣款与相关调整会自动出现在这里。</p>
        </div>
        <div v-else class="asmy-ledger-wrap">
          <template v-if="visibleActivity.length > 0">
            <p class="asmy-ledger-label">消费 / 扣款</p>
            <article v-for="entry in visibleActivity" :key="`a-${entry.id}`" class="asmy-ledger-row">
              <div>
                <b>{{ ledgerTitle(entry) }}</b>
                <small>{{ ledgerAttribution(entry) }}</small>
                <small>余额后：{{ formatMoney(entry.balance_after) }} · {{ entry.status || 'posted' }}</small>
              </div>
              <strong class="neg">{{ formatSigned(entry.amount) }}</strong>
              <span>{{ formatDateTime(entry.created_at) }}</span>
            </article>
          </template>
          <template v-if="visibleIncentives.length > 0">
            <p class="asmy-ledger-label">奖励 / 调整</p>
            <article v-for="entry in visibleIncentives" :key="`i-${entry.id}`" class="asmy-ledger-row">
              <div>
                <b>{{ ledgerTitle(entry) }}</b>
                <small>{{ ledgerAttribution(entry) }}</small>
                <small>余额后：{{ formatMoney(entry.balance_after) }} · {{ entry.status || 'posted' }}</small>
              </div>
              <strong class="pos">{{ formatSigned(entry.amount) }}</strong>
              <span>{{ formatDateTime(entry.created_at) }}</span>
            </article>
          </template>
        </div>
      </section>
      </div>
    </div>
    <BaseDialog :show="keyDialogPoolId !== null" title="资源密钥" @close="keyDialogPoolId = null">
      <MemberKeyList v-if="contextKeys.length" :keys="contextKeys" :deleting-id="deletingAPIKeyId" @copy="copyText($event, '密钥已复制')" @delete="deleteUnifiedKey" />
      <p v-else>此资源暂无绑定密钥。</p>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { KeyRound, RefreshCw, Search } from '@lucide/vue'
import MemberKeyList from '@/features/bizdecipher/components/shared-pool/MemberKeyList.vue'
import { zeroCityMascots } from '@/features/bizdecipher/constants/zeroCityMascots'
import { useClipboard } from '@/composables/useClipboard'
import '@/features/bizdecipher/components/shared-pool/shared-market-experience.css'
import SharedPoolUsageTracePanel from '@/features/bizdecipher/components/shared-pool/SharedPoolUsageTracePanel.vue'
import {
  createSharedPoolAccessKey,
  deleteSharedPoolAccessKey,
  leaveSharedPool,
  listMySeats,
  listMySharedPoolAccessKeys,
  listMySharedPoolLedger,
  type PoolSeat,
  type SharedPoolAccessKey,
  type SharedPoolLedgerEntry,
} from '@/features/bizdecipher/api/bizdecipher'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
const route = useRoute()
const { copyToClipboard } = useClipboard()
const resourceQuery = ref('')
const resourceStatus = ref<'all' | 'available' | 'unavailable'>('all')
const ledgerPoolId = ref<number | null>(null)
function isSeatAvailable(seat: PoolSeat): boolean { return seat.status === 'active' || seat.status === 'held' }
function openPoolLedger(poolId: number) { ledgerPoolId.value = poolId; activeTab.value = 'ledger' }

const tabs = [
  { key: 'pools', label: '已加入池' },
  { key: 'seats', label: '席位与费用' },
  { key: 'keys', label: 'Key 绑定' },
  { key: 'unified', label: '组合 Key' },
  { key: 'ledger', label: '消费账本' },
] as const

type TabKey = (typeof tabs)[number]['key']
const activeTab = ref<TabKey>('pools')
const primaryTabs = [{ key: 'pools', label: '已加入' }, { key: 'unified', label: '资源绑定' }, { key: 'ledger', label: '消费账单' }] as const
const activeGroup = computed(() => activeTab.value === 'seats' ? 'ledger' : activeTab.value)
async function handleMemberTabKeydown(event: KeyboardEvent, current: string) {
  const index = primaryTabs.findIndex(tab => tab.key === current)
  const target = event.key === 'ArrowRight' ? (index + 1) % 3 : event.key === 'ArrowLeft' ? (index + 2) % 3 : event.key === 'Home' ? 0 : event.key === 'End' ? 2 : -1
  if (target < 0) return
  event.preventDefault()
  activeTab.value = primaryTabs[target].key
  await nextTick()
  document.getElementById(`member-tab-${activeTab.value}`)?.focus()
}

const loading = ref(false)
const keysLoading = ref(false)
const ledgerLoading = ref(false)
const joinedPools = ref<PoolSeat[]>([])
const activeJoinedPools = computed(() =>
  joinedPools.value.filter((seat) => seat.status === 'active' || seat.status === 'held')
)
const currentSeats = computed(() => joinedPools.value.filter(seat => !['left', 'released'].includes(seat.status)))
const visibleSeats = computed(() => currentSeats.value.filter(seat => {
  const matches = `${seat.pool_name || ''} ${seat.pool_id}`.toLowerCase().includes(resourceQuery.value.trim().toLowerCase())
  return matches && (resourceStatus.value === 'all' || (resourceStatus.value === 'available' ? isSeatAvailable(seat) : !isSeatAvailable(seat)))
}))
const historicalSeats = computed(() =>
  joinedPools.value.filter((seat) => seat.status === 'left' || seat.status === 'released')
)

const accessKeys = ref<SharedPoolAccessKey[]>([])
const keyDialogPoolId = ref<number | null>(null)
const contextKeys = computed(() => {
  const ids = new Set(accessKeys.value.filter(k => k.pool_id === keyDialogPoolId.value).map(k => k.api_key_id))
  return accessKeys.value.filter(k => ids.has(k.api_key_id))
})
const activity = ref<SharedPoolLedgerEntry[]>([])
const incentives = ref<SharedPoolLedgerEntry[]>([])
const visibleActivity = computed(() => activity.value.filter(entry => ledgerPoolId.value === null || entry.pool_id === ledgerPoolId.value))
const visibleIncentives = computed(() => incentives.value.filter(entry => ledgerPoolId.value === null || entry.pool_id === ledgerPoolId.value))
const creatingKeyPoolId = ref<number | null>(null)
const leavingPoolId = ref<number | null>(null)
const deletingAPIKeyId = ref<number | null>(null)
const lastCreatedKey = ref<string>('')
const bindingAll = ref(false)
const seatsError = ref('')
const keysError = ref('')
const ledgerError = ref('')

const loadErrors = computed(() => [seatsError.value, keysError.value, ledgerError.value].filter(Boolean))

const boundPoolIds = computed(() => {
  const ids = new Set<number>()
  for (const key of accessKeys.value) {
    if (key.status === 'disabled') continue
    ids.add(key.pool_id)
  }
  return ids
})

const boundPools = computed(() => activeJoinedPools.value.filter((seat) => boundPoolIds.value.has(seat.pool_id)))
const unboundPools = computed(() => activeJoinedPools.value.filter((seat) => !boundPoolIds.value.has(seat.pool_id)))

const unifiedKeyValue = computed(() => {
  const active = accessKeys.value.find((key) => key.status !== 'disabled' && key.key)
  return active?.key || ''
})

const unifiedKeyDisplay = computed(() => {
  if (lastCreatedKey.value || unifiedKeyValue.value) return 'sk-share-••••••••'
  const preview = accessKeys.value.find(key => key.status !== 'disabled')?.key_preview
  if (preview) return preview
  return '尚未创建 · 绑定任一池后生成 sk-share-…'
})

const unifiedModelCount = computed(() => {
  const models = new Set<string>()
  for (const key of accessKeys.value) {
    if (key.status === 'disabled') continue
    for (const model of key.allowed_models || []) {
      if (model) models.add(model)
    }
  }
  return models.size
})

function poolInitial(name?: string): string {
  return (name || 'P').slice(0, 1).toUpperCase()
}

function seatStatusLabel(seat: PoolSeat): string {
  const labels: Record<string, string> = {
    active: '席位有效',
    held: '席位保留',
    inactive: '席位失效',
    left: '已退出',
    released: '已退出',
    suspended: '已暂停',
  }
  return labels[seat.status] || seat.status || '状态未知'
}

function releaseReasonLabel(reason?: string): string {
  const labels: Record<string, string> = {
    user_leave: '主动退出',
    owner_removed: '池主移除',
    idle_timeout: '连续 2 小时无调用，系统自动释放',
    insufficient_balance: '余额不足，系统停止席位',
    pool_unavailable: '池已下架或暂不可用',
    pool_archived: '池已归档',
    admin_release: '管理员释放',
    legacy_release: '历史退出记录',
  }
  return labels[String(reason || '')] || '席位已结束'
}

function seatStatusClass(seat: PoolSeat): string {
  if (seat.status === 'active' || seat.status === 'held') return 'asmy-chip-teal'
  if (seat.status === 'suspended' || seat.status === 'inactive') return 'asmy-chip-warn'
  return ''
}

function seatUsageGuidance(seat: PoolSeat): string {
  if (seat.status !== 'active' && seat.status !== 'held') return '此席位当前不参与组合 Key 路由'
  if (!boundPoolIds.value.has(seat.pool_id)) return '席位有效，但尚未绑定共享池组合 Key；连续 2 小时无调用会自动释放'
  if (seat.hourly_seat_fee <= 0) return '席位有效且已绑定，可参与模型路由；当前席位免费，连续 2 小时无调用会自动释放'
  if (seat.current_hour_waiver_met) return '本窗口消费已达到全额减免线；连续 2 小时无调用会自动释放'
  return `席位有效且已绑定；继续消费可抵扣本窗口席位费，连续 2 小时无调用会自动释放`
}

function seatIdleReleaseText(seat: PoolSeat): string {
  if (seat.status !== 'active' && seat.status !== 'held') return '—'
  const raw = seat.idle_release_at || ''
  if (!raw) return '最近活跃后 2 小时'
  const ts = Date.parse(raw)
  if (Number.isNaN(ts)) return formatDateTime(raw)
  const remainMs = ts - Date.now()
  if (remainMs <= 0) return '即将自动释放'
  const mins = Math.ceil(remainMs / 60000)
  if (mins < 60) return `${mins} 分钟后`
  const hours = Math.floor(mins / 60)
  const rem = mins % 60
  return rem > 0 ? `${hours} 小时 ${rem} 分钟后` : `${hours} 小时后`
}

function seatWindowText(seat: PoolSeat): string {
  if (seat.hourly_seat_fee <= 0) return '本窗口无席位费'
  const remaining = remainingToFullWaiver(seat)
  if (seat.current_hour_waiver_met || remaining <= 0) return '本窗口预计席位费 0.0000'
  return `距全额减免还差 ${formatMoney(remaining)}；当前预计席位费 ${formatMoney(estimateFinalSeatFee(seat))}`
}

function ledgerSourceLabel(source: string): string {
  const labels: Record<string, string> = {
    share_pool_usage: 'API 调用扣费',
    pool_seat_fee: '席位费结算',
    share_pool_payout: 'API 调用分润',
    pool_owner_payout: '席位费分润',
    shared_pool_stability_reward: '稳定运行奖励',
    refund: '退款',
    reversal: '冲正',
    adjustment: '账务调整',
  }
  return labels[source] || source || '账本事件'
}

function ledgerTitle(entry: SharedPoolLedgerEntry): string {
  const note = String(entry.note || '').trim()
  if (note && !/^pool seat fee/i.test(note) && !/^shared pool/i.test(note) && !/^pool owner payout/i.test(note)) {
    return note
  }
  const poolText = entry.pool_id ? ` · 池 #${entry.pool_id}` : ''
  switch (entry.source_type) {
    case 'pool_seat_fee':
      return `共享池席位费结算${poolText}`
    case 'share_pool_usage':
      return `共享池 API 调用扣费${poolText}`
    case 'share_pool_payout':
    case 'pool_owner_payout':
      return `共享池收益入账${poolText}`
    default:
      return note || ledgerSourceLabel(entry.source_type)
  }
}

function ledgerAttribution(entry: SharedPoolLedgerEntry): string {
  const poolText = entry.pool_id ? `最终归属池 #${entry.pool_id}` : '未关联具体共享池'
  const sourceText = entry.source_id ? ` · 事件 ${entry.source_id}` : ''
  return `${poolText} · ${ledgerSourceLabel(entry.source_type)}${sourceText}`
}

function formatMoney(value?: number | null): string {
  const n = Number(value || 0)
  if (!Number.isFinite(n)) return '0.0000'
  return n.toFixed(4)
}

function formatSeatFee(value?: number): string {
  const n = Number(value || 0)
  return n > 0 ? `${n.toFixed(4)}/h` : '免费'
}

function formatDateTime(value?: string): string {
  if (!value) return '—'
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? '—' : d.toLocaleString()
}

function formatSigned(value?: number): string {
  const n = Number(value || 0)
  const abs = Math.abs(n).toFixed(6)
  return n >= 0 ? `+${abs}` : `-${abs}`
}

/** Continuous waiver: C = min(F, U * F / T); if T<=0, no waiver path. */
function estimateWaiver(seat: PoolSeat): number {
  const F = Math.max(0, Number(seat.hourly_seat_fee || 0))
  const T = Math.max(0, Number(seat.hourly_min_usage_waiver || 0))
  const U = Math.max(0, Number(seat.current_hour_usage_amount || 0))
  if (F <= 0) return 0
  if (T <= 0) return 0
  return Math.min(F, (U * F) / T)
}

function estimateFinalSeatFee(seat: PoolSeat): number {
  const F = Math.max(0, Number(seat.hourly_seat_fee || 0))
  return Math.max(0, F - estimateWaiver(seat))
}

function remainingToFullWaiver(seat: PoolSeat): number {
  const T = Math.max(0, Number(seat.hourly_min_usage_waiver || 0))
  const U = Math.max(0, Number(seat.current_hour_usage_amount || 0))
  if (T <= 0) return 0
  return Math.max(0, T - U)
}

function seatProgress(seat: PoolSeat): number {
  const T = Math.max(0, Number(seat.hourly_min_usage_waiver || 0))
  const U = Math.max(0, Number(seat.current_hour_usage_amount || 0))
  if (T <= 0) return seat.hourly_seat_fee > 0 ? 0 : 100
  return Math.max(0, Math.min(100, (U / T) * 100))
}

async function copyText(value: string | undefined, successMessage: string): Promise<void> {
  if (!value) {
    appStore.showError('没有可复制内容')
    return
  }
  await copyToClipboard(value, successMessage)
}

async function loadJoinedPools(): Promise<void> {
  loading.value = true
  seatsError.value = ''
  try {
    joinedPools.value = await listMySeats()
  } catch {
    joinedPools.value = []
    seatsError.value = '席位数据加载失败'
  } finally {
    loading.value = false
  }
}

async function loadKeys(): Promise<void> {
  keysLoading.value = true
  keysError.value = ''
  try {
    accessKeys.value = await listMySharedPoolAccessKeys()
  } catch {
    accessKeys.value = []
    keysError.value = '组合 Key 数据加载失败'
  } finally {
    keysLoading.value = false
  }
}

async function loadLedger(): Promise<void> {
  ledgerLoading.value = true
  ledgerError.value = ''
  try {
    const view = await listMySharedPoolLedger()
    activity.value = view.activity ?? []
    incentives.value = view.incentives ?? []
  } catch {
    activity.value = []
    incentives.value = []
    ledgerError.value = '消费账本加载失败'
  } finally {
    ledgerLoading.value = false
  }
}

async function refreshAll(): Promise<void> {
  await Promise.all([loadJoinedPools(), loadKeys(), loadLedger()])
}

function modelsForPool(poolId: number): string {
  const key = accessKeys.value.find((item) => item.pool_id === poolId && item.status !== 'disabled')
  if (!key) return '未绑定'
  if (!key.allowed_models?.length) return '池内开放模型'
  return key.allowed_models.slice(0, 3).join(', ')
}

function usageForPool(poolId: number): number {
  const key = accessKeys.value.find((item) => item.pool_id === poolId && item.status !== 'disabled')
  return Number(key?.total_used || 0)
}

async function copyUnifiedKey(): Promise<void> {
  if (lastCreatedKey.value) {
    await copyText(lastCreatedKey.value, '组合 Key 已复制')
    return
  }
  if (unifiedKeyValue.value) {
    await copyText(unifiedKeyValue.value, '完整组合 Key 已复制')
    return
  }
  appStore.showError('还没有组合 Key，请先绑定至少一个池')
}

async function deleteUnifiedKey(key: SharedPoolAccessKey): Promise<void> {
  if (!window.confirm('确认删除这把共享池组合 Key？删除后它会立即失效，所有池绑定需要重新生成；席位、消费账本和历史记录不会删除。')) return
  deletingAPIKeyId.value = key.api_key_id
  try {
    await deleteSharedPoolAccessKey(key.api_key_id)
    accessKeys.value = accessKeys.value.filter((item) => item.api_key_id !== key.api_key_id)
    lastCreatedKey.value = ''
    await loadKeys()
    appStore.showSuccess('组合 Key 已删除并立即失效')
  } catch (error: any) {
    appStore.showError(error?.response?.data?.message || error?.message || '删除组合 Key 失败')
  } finally {
    deletingAPIKeyId.value = null
  }
}

async function createKeyForPool(seat: PoolSeat, options: { stayOnUnified?: boolean } = {}): Promise<void> {
  if (creatingKeyPoolId.value !== null || bindingAll.value) return
  creatingKeyPoolId.value = seat.pool_id
  try {
    const result = await createSharedPoolAccessKey(seat.pool_id, `${seat.pool_name || 'pool'}-shared`)
    if (result?.access_key?.key) {
      lastCreatedKey.value = result.access_key.key
    }
    await loadKeys()
    if (!options.stayOnUnified) {
      keyDialogPoolId.value = seat.pool_id
    }
    appStore.showSuccess(result?.already_held ? '此资源已绑定' : '密钥已就绪')
  } catch (error: any) {
    appStore.showError(error?.response?.data?.message || error?.message || '绑定组合 Key 失败')
  } finally {
    creatingKeyPoolId.value = null
  }
}

async function openPoolKey(seat: PoolSeat) {
  if (boundPoolIds.value.has(seat.pool_id)) keyDialogPoolId.value = seat.pool_id
  else await createKeyForPool(seat)
}

async function bindPoolToUnifiedKey(seat: PoolSeat): Promise<void> {
  await createKeyForPool(seat, { stayOnUnified: true })
  activeTab.value = 'unified'
}

async function bindAllJoinedPools(): Promise<void> {
  if (unboundPools.value.length === 0) {
    appStore.showSuccess('所有已加入池都已绑定')
    return
  }
  bindingAll.value = true
  let ok = 0
  let fail = 0
  try {
    for (const seat of unboundPools.value) {
      creatingKeyPoolId.value = seat.pool_id
      try {
        const result = await createSharedPoolAccessKey(seat.pool_id, `${seat.pool_name || 'pool'}-shared`)
        if (result?.access_key?.key) {
          lastCreatedKey.value = result.access_key.key
        }
        ok += 1
      } catch {
        fail += 1
      }
    }
    await loadKeys()
    if (ok > 0 && fail === 0) {
      appStore.showSuccess(`已绑定 ${ok} 个池到组合 Key`)
    } else if (ok > 0) {
      appStore.showSuccess(`已绑定 ${ok} 个池，${fail} 个失败`)
    } else {
      appStore.showError('绑定失败，请检查席位状态后重试')
    }
    if (lastCreatedKey.value) {
      await copyText(lastCreatedKey.value, '组合 Key 明文已复制，请妥善保存')
    }
  } finally {
    creatingKeyPoolId.value = null
    bindingAll.value = false
  }
}

async function leavePool(seat: PoolSeat): Promise<void> {
  if (seat.status !== 'active' && seat.status !== 'held') {
    appStore.showError('该席位已不在加入状态')
    await refreshAll()
    return
  }
  if (!window.confirm(`确认退出「${seat.pool_name || `池 #${seat.pool_id}`}」？退出会结算应计席位费。`)) return
  leavingPoolId.value = seat.pool_id
  try {
    await leaveSharedPool(seat.pool_id)
    // optimistic local drop so UI never looks like a fake leave
    joinedPools.value = joinedPools.value.filter((item) => !(item.pool_id === seat.pool_id && (item.status === 'active' || item.status === 'held')))
    await refreshAll()
    const stillActive = joinedPools.value.some((item) => item.pool_id === seat.pool_id && (item.status === 'active' || item.status === 'held'))
    if (stillActive) {
      appStore.showError('退出请求已发送，但席位仍显示为已加入，请刷新后重试')
      return
    }
    appStore.showSuccess('已退出共享池')
  } catch (error: any) {
    appStore.showError(error?.response?.data?.message || error?.message || '退出失败')
    await refreshAll()
  } finally {
    leavingPoolId.value = null
  }
}

onMounted(() => {
  const tab = String(route.query.tab || '')
  if ((tabs as readonly { key: string }[]).some((item) => item.key === tab)) {
    activeTab.value = tab === 'keys' ? 'unified' : tab as TabKey
  }
  void refreshAll()
})
</script>

<style scoped>
.asmy-page .asmy-header{display:block;padding:0 0 16px}.asmy-header-copy{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:12px}.asmy-page .asmy-header-actions{margin-top:0}.asmy-page .asmy-title{font-size:22px;font-weight:600}.asmy-section-head .member-resource-toolbar{flex:1;margin:0}.asmy-page{gap:18px}
.asmy-page {
  --bg: var(--zc-bg, #edf2f8);
  --surface: var(--zc-surface-raised, #f3f6fb);
  --card: var(--zc-card, #f7f9fc);
  --text: var(--zc-text-strong, #0c1524);
  --muted: var(--zc-muted, #667286);
  --line: var(--zc-line, rgba(20, 34, 56, 0.1));
  --teal: var(--zc-accent, #2b8f8a);
  --blue: var(--zc-accent-2, #4978e8);
  --danger: var(--zc-danger, #c23d3d);
  max-width: 1100px;
  margin: 0 auto;
  padding: 8px 0 80px;
  display: grid;
  gap: 18px;
  color: var(--text);
}

.asmy-header {
  display: grid;
  gap: 16px;
  border: 1px solid var(--line);
  border-radius: 24px;
  padding: 24px;
  background: var(--surface);
}
@media (min-width: 960px) {
  .asmy-header { grid-template-columns: minmax(0, 1.3fr) minmax(240px, 0.7fr); align-items: stretch; }
}
.asmy-eyebrow { margin: 0; font-size: 11px; font-weight: 900; letter-spacing: .14em; text-transform: uppercase; color: var(--blue); }
.asmy-title { margin: 10px 0 0; font-size: clamp(26px, 4vw, 36px); font-weight: 950; letter-spacing: -.03em; }
.asmy-subtitle { margin: 10px 0 0; max-width: 46rem; color: var(--muted); font-size: 14px; line-height: 1.7; font-weight: 650; }
.asmy-header-actions, .asmy-pool-actions, .asmy-tabs { display: flex; flex-wrap: wrap; gap: 10px; }
.asmy-header-actions { margin-top: 18px; }
.asmy-stats { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; }
.asmy-stats article {
  display: grid; align-content: center; gap: 6px; border: 1px solid var(--line); border-radius: 16px; padding: 14px; background: var(--card);
}
.asmy-stats span { color: var(--muted); font-size: 12px; font-weight: 800; }
.asmy-stats b { font-size: 28px; font-weight: 950; }

.asmy-tabs { border-bottom: 1px solid var(--line); padding-bottom: 4px; }
.asmy-operations { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 16px; align-items: center; border: 1px solid var(--line); border-radius: 20px; padding: 20px; background: var(--surface); box-shadow: 0 18px 42px color-mix(in srgb, var(--text) 7%, transparent); }
.asmy-operations-kicker { display: block; margin-bottom: 5px; color: var(--blue); font-size: 10px; font-weight: 950; letter-spacing: .14em; text-transform: uppercase; }
.asmy-operations h2 { margin: 0; font-size: 19px; font-weight: 950; }
.asmy-operations p { margin: 6px 0 0; color: var(--muted); font-size: 12px; line-height: 1.6; }
.asmy-operations-progress { display: flex; align-items: center; gap: 10px; min-width: 180px; }
.asmy-operations-progress > div { width: 132px; height: 8px; overflow: hidden; border-radius: 999px; background: color-mix(in srgb, var(--line) 84%, transparent); }
.asmy-operations-progress span { display: block; height: 100%; border-radius: inherit; background: linear-gradient(90deg, var(--teal), var(--blue)); }
.asmy-operations-progress b { color: var(--teal); font-size: 16px; }
.asmy-operations-steps { grid-column: 1 / -1; display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; }
.asmy-operations-steps button { min-width: 0; display: flex; gap: 9px; align-items: flex-start; border: 1px solid var(--line); border-radius: 14px; padding: 11px; background: var(--card); color: var(--muted); text-align: left; cursor: pointer; }
.asmy-operations-steps button > span { width: 22px; height: 22px; flex: 0 0 22px; display: grid; place-items: center; border-radius: 8px; background: color-mix(in srgb, var(--muted) 10%, transparent); font-size: 9px; font-weight: 950; }
.asmy-operations-steps button div { min-width: 0; display: grid; gap: 3px; }
.asmy-operations-steps button b { color: var(--text); font-size: 12px; }
.asmy-operations-steps button small { color: var(--muted); font-size: 10px; line-height: 1.45; }
.asmy-operations-steps button.done > span { color: var(--zc-accent-ink); background: var(--teal); }
.asmy-operations-attention .asmy-operations-kicker { color: var(--blue); }
.asmy-operations-ready .asmy-operations-kicker { color: var(--teal); }
.asmy-error-banner { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; border: 1px solid color-mix(in srgb, var(--danger) 28%, var(--line)); border-radius: 14px; padding: 12px 14px; color: var(--danger); background: color-mix(in srgb, var(--danger) 7%, var(--card)); }
.asmy-error-banner b { font-size: 12px; }
.asmy-error-banner span { flex: 1; min-width: 180px; color: var(--muted); font-size: 12px; }
.asmy-tab, .asmy-btn {
  border: 1px solid var(--line); border-radius: 12px; background: var(--card); color: var(--text);
  font: inherit; font-weight: 850; cursor: pointer; text-decoration: none;
}
.asmy-tab { min-height: 40px; padding: 0 14px; color: var(--muted); }
.asmy-tab.active { border-color: color-mix(in srgb, var(--teal) 35%, var(--line)); background: color-mix(in srgb, var(--teal) 10%, var(--card)); color: var(--teal); }
.asmy-btn { display: inline-flex; align-items: center; justify-content: center; min-height: 38px; padding: 0 14px; }
.asmy-btn-primary { border-color: color-mix(in srgb, var(--teal) 40%, transparent); background: color-mix(in srgb, var(--teal) 14%, var(--card)); color: var(--teal); }
.asmy-btn-sm { min-height: 32px; padding: 0 12px; font-size: 12px; }
.asmy-btn-danger { color: var(--danger); border-color: color-mix(in srgb, var(--danger) 28%, var(--line)); }
.asmy-btn:disabled { opacity: .55; cursor: not-allowed; }

.asmy-section { display: grid; gap: 14px; }
.asmy-section-head { display: flex; justify-content: space-between; gap: 12px; flex-wrap: wrap; align-items: flex-start; }
.asmy-section-head h2 { margin: 0 0 4px; font-size: 20px; font-weight: 950; }
.asmy-section-head p, .asmy-muted { margin: 0; color: var(--muted); font-size: 13px; line-height: 1.6; }

.asmy-loading, .asmy-empty {
  display: grid; justify-items: start; gap: 10px; border: 1px solid var(--line); border-radius: 18px; padding: 28px; background: var(--card);
}
.asmy-loading { grid-auto-flow: column; justify-content: start; align-items: center; color: var(--muted); }
.asmy-spinner { width: 18px; height: 18px; border-radius: 999px; border: 2px solid color-mix(in srgb, var(--teal) 20%, transparent); border-top-color: var(--teal); animation: spin .8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.asmy-empty { justify-items: center; text-align: center; }
.asmy-empty-title { margin: 0; font-size: 16px; font-weight: 900; }
.asmy-empty-sub { margin: 0; color: var(--muted); font-size: 13px; }

.asmy-pool-list, .asmy-seat-list, .asmy-key-list, .asmy-ledger-wrap { display: grid; gap: 12px; }
.asmy-pool-card, .asmy-seat-card, .asmy-key-card, .asmy-rule-card, .asmy-unified-card, .asmy-explain-grid article, .asmy-ledger-row {
  border: 1px solid var(--line); border-radius: 16px; background: var(--card); padding: 16px;
}
.asmy-pool-card { display: grid; gap: 14px; }
@media (min-width: 900px) {
  .asmy-pool-card { grid-template-columns: minmax(0, 1fr) auto; align-items: center; }
}
.asmy-pool-main { display: flex; gap: 12px; min-width: 0; }
.asmy-pool-avatar { width: 40px; height: 40px; border-radius: 12px; display: grid; place-items: center; background: color-mix(in srgb, var(--teal) 12%, transparent); color: var(--teal); font-weight: 950; flex-shrink: 0; }
.asmy-pool-info { min-width: 0; flex: 1; display: grid; gap: 10px; }
.asmy-pool-title-row { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
.asmy-pool-title-row b { font-size: 16px; font-weight: 950; }
.asmy-chip { border-radius: 999px; border: 1px solid var(--line); padding: 3px 8px; font-size: 11px; font-weight: 850; color: var(--muted); }

.asmy-chip-teal { color: var(--teal); border-color: color-mix(in srgb, var(--teal) 30%, var(--line)); background: color-mix(in srgb, var(--teal) 8%, transparent); }
.asmy-chip-warn { color: #a66a17; border-color: color-mix(in srgb, #a66a17 30%, var(--line)); background: color-mix(in srgb, #a66a17 8%, transparent); }
.asmy-pool-guidance { display: grid; gap: 3px; border-left: 3px solid color-mix(in srgb, var(--teal) 50%, transparent); padding-left: 10px; }
.asmy-pool-guidance span { color: var(--text); font-size: 12px; font-weight: 850; line-height: 1.5; }
.asmy-pool-guidance small { color: var(--muted); font-size: 11px; line-height: 1.45; }
.asmy-warn { margin: 8px 0 0; color: var(--danger); font-size: 12px; font-weight: 750; line-height: 1.5; }
.asmy-metric-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 8px; }
.asmy-metric-grid span { display: grid; gap: 3px; border-radius: 12px; border: 1px solid var(--line); padding: 8px 10px; background: color-mix(in srgb, var(--surface) 80%, transparent); }
.asmy-metric-grid small { color: var(--muted); font-size: 11px; font-weight: 800; }
.asmy-metric-grid strong { font-size: 12px; font-weight: 900; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.asmy-rule-card ul { margin: 10px 0 0; padding-left: 18px; color: var(--muted); font-size: 13px; line-height: 1.7; }
.asmy-rule-card h3 { margin: 0; font-size: 15px; font-weight: 950; }
.asmy-seat-head { display: flex; justify-content: space-between; gap: 10px; flex-wrap: wrap; margin-bottom: 10px; }
.asmy-seat-head b { font-size: 15px; font-weight: 950; }
.asmy-seat-head span { color: var(--teal); font-size: 12px; font-weight: 850; }
.asmy-progress { height: 8px; border-radius: 999px; background: color-mix(in srgb, var(--line) 80%, transparent); overflow: hidden; margin-bottom: 12px; }
.asmy-progress-bar { height: 100%; background: linear-gradient(90deg, var(--teal), var(--blue)); }
.asmy-history { margin-top: 22px; }
.asmy-history-list { border-top: 1px solid var(--line); }
.asmy-history-list article { display: flex; justify-content: space-between; gap: 12px; padding: 12px 2px; border-bottom: 1px solid var(--line); align-items: center; flex-wrap: wrap; }
.asmy-history-list article > div { display: grid; gap: 3px; }
.asmy-history-list b { font-size: 13px; font-weight: 900; }
.asmy-history-list span, .asmy-history-list small { color: var(--muted); font-size: 11px; line-height: 1.5; }

.asmy-explain-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 10px; }
.asmy-explain-grid p { margin: 8px 0; color: var(--muted); font-size: 13px; line-height: 1.6; }
.asmy-badge { display: inline-flex; border-radius: 999px; padding: 4px 10px; font-size: 11px; font-weight: 900; background: color-mix(in srgb, var(--blue) 12%, transparent); color: var(--blue); }
.asmy-badge-teal { background: color-mix(in srgb, var(--teal) 12%, transparent); color: var(--teal); }
.asmy-badge-blue { background: color-mix(in srgb, var(--blue) 12%, transparent); color: var(--blue); }
.asmy-link { color: var(--teal); font-size: 13px; font-weight: 850; text-decoration: none; }

.asmy-key-head, .asmy-key-value-row, .asmy-key-meta { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; }
.asmy-key-value-row { margin-top: 10px; }
.asmy-key-value-row code {
  flex: 1; min-width: 180px; border-radius: 10px; border: 1px solid var(--line); padding: 8px 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace; color: var(--teal); background: color-mix(in srgb, var(--surface) 85%, transparent);
}
.asmy-key-meta { margin-top: 8px; color: var(--muted); font-size: 12px; }

.asmy-unified-card { display: grid; gap: 16px; }
@media (min-width: 900px) { .asmy-unified-card { grid-template-columns: minmax(0, 1.2fr) minmax(240px, 0.8fr); } }
.asmy-unified-copy h3 { margin: 10px 0; font-size: 18px; font-weight: 950; }
.asmy-unified-copy ul { margin: 0; padding-left: 18px; color: var(--muted); font-size: 13px; line-height: 1.7; }
.asmy-unified-side { display: grid; gap: 10px; align-content: start; }
.asmy-unified-status { display: grid; gap: 8px; border: 1px solid var(--line); border-radius: 14px; padding: 12px; background: color-mix(in srgb, var(--surface) 80%, transparent); }
.asmy-unified-status b { font-size: 13px; font-weight: 950; }
.asmy-unified-status p { margin: 0; color: var(--muted); font-size: 12px; font-weight: 750; }
.asmy-unified-pools { display: flex; flex-wrap: wrap; gap: 6px; }
.asmy-unified-pools span { border-radius: 999px; border: 1px solid var(--line); padding: 4px 10px; font-size: 12px; font-weight: 850; color: var(--teal); }

.asmy-ledger-label { margin: 4px 0; color: var(--muted); font-size: 11px; font-weight: 900; letter-spacing: .08em; text-transform: uppercase; }
.asmy-ledger-row { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; gap: 10px; align-items: center; }
.asmy-ledger-row b { display: block; font-size: 13px; }
.asmy-ledger-row small, .asmy-ledger-row span { color: var(--muted); font-size: 12px; }
.asmy-ledger-row .pos { color: var(--teal); }
.asmy-ledger-row .neg { color: var(--danger); }

@media (max-width: 900px) {
  .asmy-operations-steps { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 720px) {
  .asmy-stats, .asmy-operations-steps { grid-template-columns: 1fr; }
  .asmy-operations { grid-template-columns: 1fr; }
  .asmy-operations-progress { min-width: 0; width: 100%; }
  .asmy-operations-progress > div { width: auto; flex: 1; }
  .asmy-ledger-row { grid-template-columns: 1fr; }
  .asmy-pool-actions { width: 100%; }
  .asmy-pool-actions .asmy-btn { flex: 1 1 140px; }
}
</style>
