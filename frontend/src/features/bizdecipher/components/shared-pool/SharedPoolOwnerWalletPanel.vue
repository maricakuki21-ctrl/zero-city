<template>
  <section class="spow-panel" aria-labelledby="spow-title">
    <header class="spow-head">
      <div>
        <p v-if="!compact" class="spow-kicker">经营与创作收益</p>
        <h2 id="spow-title">{{ compact ? '收益明细与转入' : '收益总钱包' }}</h2>
        <p v-if="poolId" data-testid="wallet-pool-scope">下方仅筛选共享池 #{{ poolId }} 的收益凭证；余额汇总及转入操作仍属于经营与创作的总钱包。</p>
        <p v-if="!compact">新收益先进入独立钱包，不会直接增加站点普通余额。只有你确认转入后，才会成为可消费的站内余额。</p>
      </div>
      <button class="spow-refresh" type="button" :disabled="loading || transferring" @click="loadLedger(false)">
        {{ loading ? '刷新中…' : '刷新账本' }}
      </button>
    </header>

    <div v-if="loadError" class="spow-alert spow-alert-error" role="alert">
      <span>{{ loadError }}</span>
      <button type="button" @click="loadLedger(false)">重新加载</button>
    </div>

    <details v-if="compact" class="spow-history-summary">
      <summary>累计与历史金额</summary>
      <p>累计经营与创作收益 {{ formatAmount(totalOperatingEarnings) }} · 已转站内余额 {{ formatAmount(ledger.wallet?.transferred_amount) }} · 历史已入余额 {{ formatAmount(legacyBalanceTotal) }}</p>
      <small>历史已入余额仅供核对，不可再次转入。</small>
    </details>
    <div v-else class="spow-metrics" :aria-busy="loading">
      <article>
        <span>累计经营与创作收益</span>
        <b data-testid="wallet-total-earned">{{ formatAmount(totalOperatingEarnings) }}</b>
        <small>包含新钱包和旧系统已结算收益</small>
      </article>
      <article class="spow-metric-primary">
        <span>可转入站内余额</span>
        <b data-testid="wallet-available">{{ formatAmount(ledger.wallet?.available_amount) }}</b>
        <small>当前可支配的经营与创作收益</small>
      </article>
      <article>
        <span>已转站点余额</span>
        <b data-testid="wallet-transferred">{{ formatAmount(ledger.wallet?.transferred_amount) }}</b>
        <small>通过本钱包确认转出的累计金额</small>
      </article>
      <article>
        <span>历史已入余额</span>
        <b data-testid="wallet-legacy">{{ formatAmount(legacyBalanceTotal) }}</b>
        <small>仅作历史展示，不能再次转入</small>
      </article>
    </div>

    <p v-if="ledger.wallet && (ledger.wallet.pending_amount > 0 || ledger.wallet.frozen_amount > 0)" class="spow-pending-note">
      另有待结算 {{ formatAmount(ledger.wallet?.pending_amount) }}、冻结 {{ formatAmount(ledger.wallet?.frozen_amount) }}，满足结算条件后才会进入“可转入站内余额”。
    </p>

    <div class="spow-transfer">
      <div class="spow-transfer-copy">
        <h3>将收益转入站内余额</h3>
        <p>这是仅转入站内余额的内部划转，不会转到银行卡或链上地址。转入成功后可用于站内消费，不能在这里撤回。</p>
      </div>
      <div class="spow-transfer-form">
        <label for="spow-transfer-amount">转入金额</label>
        <div class="spow-input-row">
          <input
            id="spow-transfer-amount"
            v-model="transferAmount"
            data-testid="wallet-transfer-input"
            type="number"
            inputmode="decimal"
            min="0"
            :max="ledger.wallet?.available_amount"
            step="0.000001"
            placeholder="输入本次要转入的金额"
            :disabled="transferring || !ledger.wallet"
            @input="handleAmountInput"
          />
          <button type="button" :disabled="transferring || !ledger.wallet || ledger.wallet.available_amount <= 0" @click="fillAvailableAmount">全部</button>
          <button
            data-testid="wallet-transfer-review"
            class="spow-primary-btn"
            type="button"
            :disabled="!canReviewTransfer || transferring || !ledger.wallet"
            @click="reviewTransfer"
          >
            核对转入
          </button>
        </div>
        <p v-if="amountError" class="spow-field-error">{{ amountError }}</p>
      </div>

      <div v-if="confirming" class="spow-confirm" role="group" aria-label="二次确认转入站点余额">
        <div>
          <b>请再确认一次：转入 {{ formatAmount(confirmAmount) }} 到站点余额</b>
          <p>系统会用同一个操作编号处理本次请求。即使网络中断后重试，也不会重复入账。</p>
          <small>操作编号：{{ operationId }}</small>
        </div>
        <div class="spow-confirm-actions">
          <button type="button" :disabled="transferring" @click="confirming = false">返回修改</button>
          <button
            data-testid="wallet-transfer-confirm"
            class="spow-primary-btn"
            type="button"
            :disabled="transferring"
            @click="confirmTransfer"
          >
            {{ transferring ? '正在转入，请勿重复点击…' : '确认转入站点余额' }}
          </button>
        </div>
      </div>

      <div v-if="transferMessage" class="spow-alert" :class="transferSucceeded ? 'spow-alert-success' : 'spow-alert-error'" role="status">
        {{ transferMessage }}
      </div>
    </div>

    <div class="spow-ledger-head">
      <div>
        <h3>{{ poolId ? '本池永久收益记录' : '永久收益总账' }}</h3>
        <p>共享池和专栏名称按收益发生时保存。内容以后归档，历史凭证和当时名称仍会保留。</p>
      </div>
      <span v-if="poolId && !loadError">已加载 {{ earningsRows.length }} 条 · 净变动 {{ formatSignedAmount(poolWalletDelta) }}{{ earningsHasMore ? '（非全部记录）' : '' }}</span>
    </div>

    <div v-if="earningsRows.length" class="spow-ledger-filters">
      <label>记录类型<select v-model="eventFilter" aria-label="筛选收益记录类型"><option value="">全部类型</option><option value="earning">经营收益</option><option value="transfer_to_balance">转入站内余额</option><option value="reversal">收益冲正</option><option value="adjustment">人工调整</option></select></label>
      <label>查找凭证<input v-model="receiptQuery" aria-label="查找收益凭证" placeholder="共享池、作品、模型或操作编号" type="search" /></label>
      <span>已加载 {{ earningsRows.length }} 条，匹配 {{ visibleEarnings.length }} 条{{ earningsHasMore ? ' · 还有更早记录' : '' }}</span>
    </div>
    <div v-if="loading && earningsRows.length === 0" class="spow-empty">正在读取收益总账…</div>
    <div v-else-if="loadError && earningsRows.length === 0" class="spow-empty">收益记录未能加载，当前不能确认是否有收益。</div>
    <div v-else-if="earningsRows.length === 0" class="spow-empty">
      暂无{{ poolId ? '这个池的' : '' }}新钱包收益记录。旧系统已经进入普通余额的记录仍保留在上方“历史旧余额”中。
    </div>
    <div v-else-if="!visibleEarnings.length" class="spow-empty">已加载记录中没有匹配项{{ earningsHasMore ? '，可继续加载更早记录。' : '。' }}</div>
    <div v-else class="spow-ledger-list" data-testid="wallet-earnings-ledger">
      <article v-for="entry in visibleEarnings" :key="entry.id" class="spow-ledger-row">
        <div class="spow-ledger-main">
          <div class="spow-ledger-title">
            <b data-testid="wallet-pool-snapshot">{{ earningsPoolLabel(entry) }}</b>
            <span>{{ earningsEventLabel(entry.event_type) }}</span>
          </div>
          <p>
            <span v-if="isColumnEarning(entry)">专栏{{ entry.metadata?.source_type === 'creator_column_refund' ? '退款' : '购买' }} · 订单 #{{ entry.metadata?.purchase_id }}</span>
            <span v-else-if="isAssetEarning(entry)">资产{{ entry.metadata?.source_type === 'capability_asset_refund' ? '退款' : '购买' }} · 订单 #{{ entry.metadata?.purchase_id }}</span>
            <span v-else-if="isMarketEarning(entry)">服务订单结算 · 订单 #{{ entry.metadata?.order_id }}</span>
            <span v-else-if="isTavernEarning(entry)">游戏入场收益 · 房间 #{{ entry.metadata?.room_id }} · 票据 #{{ entry.metadata?.ticket_id }}</span>
            <span v-if="entry.model_snapshot">模型 {{ entry.model_snapshot }}</span>
            <span v-if="entry.pricing_source_snapshot">{{ pricingSourceLabel(entry.pricing_source_snapshot) }}</span>
            <span v-if="entry.request_id">请求 {{ compactId(entry.request_id) }}</span>
          </p>
          <small>{{ formatDateTime(entry.posted_at || entry.created_at) }} · 名称按发生时快照保存</small>
          <details class="spow-receipt">
            <summary>账本凭证 #{{ entry.id }} · {{ postingStatus(entry.status) }}</summary>
            <dl>
              <div><dt>操作编号</dt><dd>{{ entry.operation_id || '未记录' }}</dd></div>
              <div><dt>来源请求</dt><dd>{{ entry.request_id || '未记录' }}</dd></div>
              <div v-if="isColumnEarning(entry)"><dt>专栏 / 购买订单</dt><dd>{{ entry.metadata?.column_id }} / {{ entry.metadata?.purchase_id }}</dd></div>
              <div v-else-if="isAssetEarning(entry)"><dt>资产 / 购买订单</dt><dd>{{ entry.metadata?.asset_id }} / {{ entry.metadata?.purchase_id }}</dd></div>
              <div v-else-if="isMarketEarning(entry)"><dt>服务订单</dt><dd>{{ entry.metadata?.order_id }}</dd></div>
              <template v-else-if="isTavernEarning(entry)">
                <div><dt>剧本 / 房间</dt><dd>{{ entry.metadata?.script_id }} / {{ entry.metadata?.room_id }}</dd></div>
                <div><dt>入场票据</dt><dd>#{{ entry.metadata?.ticket_id }}</dd></div>
              </template>
              <div v-else><dt>共享池 / 账号 / 价格版本</dt><dd>{{ entry.pool_id ?? '未记录' }} / {{ entry.account_id ?? '未记录' }} / {{ entry.price_version_id ?? '未记录' }}</dd></div>
              <template v-if="entry.event_type === 'earning'">
                <div><dt>用户实付</dt><dd>{{ formatAmount(entry.gross_amount) }}</dd></div>
                <div><dt>平台服务费</dt><dd>{{ formatAmount(entry.platform_fee_amount) }}</dd></div>
                <div><dt>净收益</dt><dd>{{ formatAmount(entry.net_amount) }}</dd></div>
              </template>
              <div><dt>收益余额变动</dt><dd>{{ formatSignedAmount(entry.wallet_delta) }}</dd></div>
              <div><dt>入账时间</dt><dd>{{ formatDateTime(entry.posted_at) }}</dd></div>
              <div v-if="entry.available_at"><dt>可用时间</dt><dd>{{ formatDateTime(entry.available_at) }}</dd></div>
            </dl>
            <p v-if="entry.metadata?.source_type === 'creator_column_refund'">专栏购买已退回买家的站内余额，阅读权限已撤回；不是外部银行卡退款。</p>
            <p v-else-if="entry.metadata?.source_type === 'capability_asset_refund'">资产购买已退回买家的站内余额，付费下载权限已撤回；不是外部银行卡退款。</p>
            <p v-else-if="isTavernEarning(entry)">本场已开局，入场费已结算至剧本作者收益钱包；不包含玩家模型调用费用。</p>
            <p v-else-if="entry.event_type === 'reversal'">这是收益账本冲正，不代表已向付款人退款。</p>
            <p v-if="entry.event_type === 'transfer_to_balance'">站内余额划转，不是外部提现。</p>
            <p v-if="entry.event_type === 'withdrawal' || entry.event_type === 'withdrawal_return'">
              提现申请 #{{ entry.metadata?.withdrawal_id ?? entry.operation_id }} · {{ entry.event_type === 'withdrawal' ? '申请扣款，外部到账状态请查看提现记录。' : '已退回可用收益，不计入站内余额或待结算收益。' }}
            </p>
          </details>
        </div>
        <div class="spow-ledger-amount" :class="entry.wallet_delta >= 0 ? 'is-positive' : 'is-negative'">
          <b>{{ formatSignedAmount(entry.wallet_delta) }}</b>
          <small v-if="entry.event_type === 'earning'">平台服务费 {{ formatAmount(entry.platform_fee_amount) }}</small>
          <small>可转余额结余 {{ formatAmount(entry.available_after) }}</small>
        </div>
      </article>
    </div>
    <div v-if="earningsHasMore" class="spow-load-more">
      <button
        data-testid="wallet-earnings-load-more"
        type="button"
        :disabled="earningsLoadingMore"
        @click="loadMoreEarnings"
      >
        {{ earningsLoadingMore ? '正在读取更早记录…' : '加载更早收益记录' }}
      </button>
    </div>

    <details v-if="ledger.legacy_withdrawable.length > 0" class="spow-legacy-details">
      <summary>查看旧系统已入普通余额记录（{{ ledger.legacy_withdrawable.length }} 条）</summary>
      <p>这些收益在旧系统中已经直接加入普通余额，只保留历史凭证，不会进入“现在可转”。</p>
      <div class="spow-legacy-list">
        <div v-for="entry in ledger.legacy_withdrawable" :key="entry.id">
          <span>{{ entry.note || `历史记录 #${entry.id}` }}</span>
          <b>{{ formatSignedAmount(entry.amount) }}</b>
        </div>
      </div>
    </details>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  listMySharedPoolLedger,
  transferSharedPoolOwnerEarnings,
  type SharedPoolLedgerView,
  type SharedPoolOwnerEarningsEntry,
} from '@/features/bizdecipher/api/bizdecipher'
import { listSharedPoolOwnerEarningsPage } from '@/features/bizdecipher/api/sharedPoolOwnerLedger'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  poolId?: number
  compact?: boolean
}>()

const emit = defineEmits<{
  refreshed: [ledger: SharedPoolLedgerView]
}>()

type TransferDraft = {
  ownerId: number
  amount: number
  operationId: string
}

const TRANSFER_DRAFT_KEY = 'shared-pool-owner-wallet-transfer-draft-v1'

function emptyLedger(): SharedPoolLedgerView {
  return {
    wallet: undefined,
    earnings: [],
    activity: [],
    withdrawable: [],
    legacy_withdrawable: [],
    incentives: [],
  }
}

const ledger = ref<SharedPoolLedgerView>(emptyLedger())
const walletReady = computed(() => Boolean(ledger.value.wallet))
const loading = ref(false)
const loadError = ref('')
const transferAmount = ref<string | number>('')
const confirmAmount = ref(0)
const operationId = ref('')
const confirming = ref(false)
const transferring = ref(false)
const transferMessage = ref('')
const transferSucceeded = ref(false)
const earningsHasMore = ref(false)
const earningsBeforeID = ref<number | undefined>()
const earningsLoadingMore = ref(false)
const eventFilter = ref('')
const receiptQuery = ref('')

const normalizedTransferAmount = computed(() => normalizeAmount(transferAmount.value))
const amountError = computed(() => {
  if (String(transferAmount.value).trim() === '') return ''
  if (normalizedTransferAmount.value <= 0) return '请输入大于 0 的金额。'
  if (!ledger.value.wallet) return '收益钱包数据暂不可用。'
  if (normalizedTransferAmount.value > ledger.value.wallet.available_amount + 1e-12) {
    return `本次最多可转 ${formatAmount(ledger.value.wallet.available_amount)}。`
  }
  return ''
})
const canReviewTransfer = computed(() => normalizedTransferAmount.value > 0 && amountError.value === '')
const legacyBalanceTotal = computed(() => ledger.value.wallet ? ledger.value.legacy_withdrawable.reduce((sum, entry) => sum + Number(entry.amount || 0), 0) : null)
const totalOperatingEarnings = computed(() => ledger.value.wallet ? Number(ledger.value.wallet.total_earned) + (legacyBalanceTotal.value ?? 0) : null)
const earningsRows = computed(() => {
  const rows = props.poolId
    ? ledger.value.earnings.filter((entry) => Number(entry.pool_id) === props.poolId)
    : ledger.value.earnings
  return [...rows].sort((left, right) => eventTime(right) - eventTime(left))
})
const poolWalletDelta = computed(() => earningsRows.value.reduce((sum, entry) => sum + Number(entry.wallet_delta || 0), 0))
const visibleEarnings = computed(() => {
  const query = receiptQuery.value.trim().toLocaleLowerCase()
  return earningsRows.value.filter(entry =>
    (!eventFilter.value || entry.event_type === eventFilter.value)
    && (!query || [entry.id, earningsPoolLabel(entry), entry.model_snapshot, entry.operation_id, entry.request_id]
      .some(value => String(value ?? '').toLocaleLowerCase().includes(query))))
})

function postingStatus(status: string): string {
  return ({ posted: '已入账', available: '已入账可用', settled: '已结算', pending: '待入账', reversed: '已冲正', failed: '失败', cancelled: '已取消' } as Record<string, string>)[status] || `状态：${status || '未记录'}`
}

function normalizeAmount(value: string | number): number {
  const parsed = Number(value)
  if (!Number.isFinite(parsed) || parsed <= 0) return 0
  return Number(parsed.toFixed(12))
}

function formatAmount(value?: number | null): string {
  if (!walletReady.value && value === undefined) return '暂不可用'
  if (value === undefined || value === null || !Number.isFinite(Number(value))) return '暂不可用'
  const amount = Number(value)
  return amount.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 8 })
}

function formatSignedAmount(value: number): string {
  const amount = Number(value || 0)
  return `${amount > 0 ? '+' : ''}${formatAmount(amount)}`
}

function eventTime(entry: SharedPoolOwnerEarningsEntry): number {
  const value = entry.posted_at || entry.created_at
  const parsed = Date.parse(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function formatDateTime(value?: string): string {
  if (!value) return '时间未记录'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value
  return parsed.toLocaleString('zh-CN', { hour12: false })
}

function compactId(value: string): string {
  return value.length > 16 ? `${value.slice(0, 8)}…${value.slice(-6)}` : value
}

function earningsPoolLabel(entry: SharedPoolOwnerEarningsEntry): string {
  if (isTavernEarning(entry)) {
    return typeof entry.metadata?.title === 'string' && entry.metadata.title.trim()
      ? entry.metadata.title : '酒馆游戏入场收益'
  }
  if (isAssetEarning(entry)) {
    return typeof entry.metadata?.asset_title === 'string' && entry.metadata.asset_title.trim()
      ? entry.metadata.asset_title : '历史能力资产'
  }
  if (isMarketEarning(entry)) {
    return typeof entry.metadata?.title === 'string' && entry.metadata.title.trim()
      ? entry.metadata.title : '服务订单'
  }
  if (isColumnEarning(entry)) {
    return typeof entry.metadata?.column_title === 'string' && entry.metadata.column_title.trim()
      ? entry.metadata.column_title
      : '历史专栏'
  }
  if (entry.pool_name_snapshot.trim()) return entry.pool_name_snapshot
  if (entry.pool_id) return `历史共享池 #${entry.pool_id}`
  return '收益钱包结算'
}

function isColumnEarning(entry: SharedPoolOwnerEarningsEntry): boolean {
  return entry.metadata?.source_type === 'creator_column_purchase' || entry.metadata?.source_type === 'creator_column_refund'
}

function isAssetEarning(entry: SharedPoolOwnerEarningsEntry): boolean {
  return entry.metadata?.source_type === 'capability_asset_purchase' || entry.metadata?.source_type === 'capability_asset_refund'
}

function isMarketEarning(entry: SharedPoolOwnerEarningsEntry): boolean {
  return entry.metadata?.source_type === 'marketplace_order_settlement'
}
function isTavernEarning(entry: SharedPoolOwnerEarningsEntry): boolean {
  return entry.metadata?.source_type === 'tavern_ticket_settlement'
}

function earningsEventLabel(eventType: string): string {
  const labels: Record<string, string> = {
    earning: '经营收益',
    transfer_to_balance: '转入站点余额',
    withdrawal: '提现申请扣款',
    withdrawal_return: '提现退回收益',
    reversal: '收益冲正',
    adjustment: '人工调整',
  }
  return labels[eventType] || '收益变动'
}

function pricingSourceLabel(source: string): string {
  const labels: Record<string, string> = {
    official: '官方定价',
    platform: '平台定价',
    custom: '池主自定义定价',
    catalog: '平台模型目录定价',
  }
  return labels[source] || `定价来源：${source}`
}

function createOperationId(): string {
  const randomId = typeof globalThis.crypto?.randomUUID === 'function'
    ? globalThis.crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(36).slice(2, 14)}`
  return `shared-pool-owner-transfer-${randomId}`
}

function readTransferDraft(): TransferDraft | null {
  try {
    const raw = globalThis.sessionStorage?.getItem(TRANSFER_DRAFT_KEY)
    if (!raw) return null
    const draft = JSON.parse(raw) as Partial<TransferDraft>
    const amount = normalizeAmount(Number(draft.amount || 0))
    if (!draft.ownerId || !draft.operationId || amount <= 0) return null
    return { ownerId: Number(draft.ownerId), amount, operationId: String(draft.operationId) }
  } catch {
    return null
  }
}

function storeTransferDraft(draft: TransferDraft): void {
  try {
    globalThis.sessionStorage?.setItem(TRANSFER_DRAFT_KEY, JSON.stringify(draft))
  } catch {
    // The operation id still remains stable for retries in the current page session.
  }
}

function clearTransferDraft(): void {
  try {
    globalThis.sessionStorage?.removeItem(TRANSFER_DRAFT_KEY)
  } catch {
    // Storage can be unavailable in strict privacy modes.
  }
}

function restoreTransferDraft(): void {
  const draft = readTransferDraft()
  if (!draft) return
  if (ledger.value.wallet && ledger.value.wallet.owner_id > 0 && draft.ownerId !== ledger.value.wallet.owner_id) {
    clearTransferDraft()
    return
  }
  if (!ledger.value.wallet || ledger.value.wallet.owner_id <= 0) return
  transferAmount.value = String(draft.amount)
  confirmAmount.value = draft.amount
  operationId.value = draft.operationId
  confirming.value = true
  transferSucceeded.value = false
  transferMessage.value = '检测到一笔尚未收到成功确认的转入请求。请核对金额后用原操作编号重试。'
}

async function loadLedger(restoreDraft: boolean): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const [view, earningsPage] = await Promise.all([
      listMySharedPoolLedger(),
      listSharedPoolOwnerEarningsPage({ poolId: props.poolId, limit: 50 }),
    ])
    ledger.value = {
      ...view,
      earnings: earningsPage.items,
    }
    earningsHasMore.value = earningsPage.has_more
    earningsBeforeID.value = earningsPage.next_before_id
    emit('refreshed', ledger.value)
    if (restoreDraft) restoreTransferDraft()
  } catch (error) {
    earningsHasMore.value = false
    earningsBeforeID.value = undefined
    loadError.value = extractApiErrorMessage(error, '收益账本加载失败，请重试。')
  } finally {
    loading.value = false
  }
}

async function loadMoreEarnings(): Promise<void> {
  if (earningsLoadingMore.value || !earningsHasMore.value || !earningsBeforeID.value) return
  earningsLoadingMore.value = true
  loadError.value = ''
  try {
    const page = await listSharedPoolOwnerEarningsPage({
      beforeId: earningsBeforeID.value,
      poolId: props.poolId,
      limit: 50,
    })
    const known = new Set(ledger.value.earnings.map(entry => entry.id))
    ledger.value.earnings.push(...page.items.filter(entry => !known.has(entry.id)))
    earningsHasMore.value = page.has_more
    earningsBeforeID.value = page.next_before_id
  } catch (error) {
    loadError.value = extractApiErrorMessage(error, '更早的收益记录读取失败，请重试。')
  } finally {
    earningsLoadingMore.value = false
  }
}

function handleAmountInput(): void {
  confirming.value = false
  confirmAmount.value = 0
  operationId.value = ''
  transferMessage.value = ''
  transferSucceeded.value = false
  clearTransferDraft()
}

function fillAvailableAmount(): void {
  if (!ledger.value.wallet) return
  transferAmount.value = String(normalizeAmount(ledger.value.wallet.available_amount))
  handleAmountInput()
}

function reviewTransfer(): void {
  if (!canReviewTransfer.value || transferring.value) return
  confirmAmount.value = normalizedTransferAmount.value
  operationId.value = operationId.value || createOperationId()
  confirming.value = true
  transferMessage.value = ''
  transferSucceeded.value = false
  if (ledger.value.wallet && ledger.value.wallet.owner_id > 0) {
    storeTransferDraft({
      ownerId: ledger.value.wallet.owner_id,
      amount: confirmAmount.value,
      operationId: operationId.value,
    })
  }
}

async function confirmTransfer(): Promise<void> {
  if (transferring.value || !confirming.value || confirmAmount.value <= 0 || !operationId.value) return
  transferring.value = true
  transferMessage.value = ''
  transferSucceeded.value = false
  try {
    const result = await transferSharedPoolOwnerEarnings(confirmAmount.value, operationId.value)
    transferSucceeded.value = true
    transferMessage.value = result.already_done
      ? `这笔转入此前已经完成，系统没有重复入账。当前站点余额 ${formatAmount(result.balance_after)}。`
      : `已转入 ${formatAmount(result.amount)}，当前站点余额 ${formatAmount(result.balance_after)}。`
    clearTransferDraft()
    confirming.value = false
    transferAmount.value = ''
    confirmAmount.value = 0
    operationId.value = ''
    await loadLedger(false)
  } catch (error) {
    transferMessage.value = extractApiErrorMessage(error, '转入失败，请保持当前操作编号后重试。')
    await loadLedger(false)
  } finally {
    transferring.value = false
  }
}

onMounted(() => loadLedger(true))
</script>

<style scoped>
.spow-ledger-filters { display:flex; flex-wrap:wrap; align-items:end; gap:12px; margin:16px 0; }
.spow-ledger-filters label { display:grid; gap:5px; min-width:0; font-size:12px; color:var(--bd-text-secondary); }
.spow-ledger-filters label:nth-child(2) { flex:1 1 220px; }
.spow-ledger-filters input,.spow-ledger-filters select { min-width:0; width:100%; min-height:36px; padding:7px 10px; border:1px solid var(--bd-ui-line); border-radius:6px; background:var(--bd-surface); color:var(--bd-text-primary); }
.spow-ledger-filters>span { font-size:12px; color:var(--bd-text-secondary); }
.spow-receipt { margin-top:10px; font-size:12px; }
.spow-receipt summary { cursor:pointer; color:var(--bd-accent-teal); padding:5px 0; }
.spow-receipt dl { display:grid; gap:8px; margin:10px 0; }
.spow-receipt dl>div { display:grid; grid-template-columns:minmax(100px,.6fr) minmax(0,1fr); gap:12px; }
.spow-receipt dt { color:var(--bd-text-secondary); }.spow-receipt dd { margin:0; overflow-wrap:anywhere; user-select:text; }
.spow-receipt summary:focus-visible { outline:2px solid var(--bd-accent-teal); outline-offset:2px; }
.spow-panel {
  --spow-border: color-mix(in srgb, var(--text, #172033) 12%, transparent);
  --spow-soft: color-mix(in srgb, var(--module-panel-raised, #fff) 72%, transparent);
  display: grid;
  gap: 18px;
  width: 100%;
  color: var(--text, #172033);
}
.spow-head,
.spow-ledger-head,
.spow-input-row,
.spow-confirm,
.spow-confirm-actions,
.spow-ledger-title,
.spow-ledger-row {
  display: flex;
  align-items: center;
}
.spow-head,
.spow-ledger-head,
.spow-confirm,
.spow-ledger-row { justify-content: space-between; }
.spow-head { gap: 18px; }
.spow-kicker {
  margin: 0 0 4px;
  color: var(--teal, #177e79);
  font-size: 11px;
  font-weight: 900;
}
.spow-head h2,
.spow-ledger-head h3,
.spow-transfer h3 { margin: 0; font-size: 18px; }
.spow-head p:not(.spow-kicker),
.spow-ledger-head p,
.spow-transfer-copy p,
.spow-confirm p,
.spow-legacy-details p {
  margin: 5px 0 0;
  color: var(--muted, #667085);
  font-size: 12px;
  line-height: 1.6;
}
.spow-refresh,
.spow-input-row button,
.spow-confirm-actions button,
.spow-alert button {
  min-height: 36px;
  border: 1px solid var(--spow-border);
  border-radius: 7px;
  padding: 0 13px;
  background: var(--module-panel, #fff);
  color: inherit;
  font-size: 12px;
  font-weight: 800;
  cursor: pointer;
}
button:disabled { cursor: not-allowed; opacity: .55; }
.spow-primary-btn {
  border-color: transparent !important;
  background: var(--teal, #177e79) !important;
  color: #fff !important;
}
.spow-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  border: 1px solid var(--spow-border);
  border-radius: 8px;
  overflow: hidden;
}
.spow-metrics article {
  min-width: 0;
  padding: 16px;
  border-right: 1px solid var(--spow-border);
  background: var(--spow-soft);
}
.spow-metrics article:last-child { border-right: 0; }
.spow-metrics span,
.spow-metrics small { display: block; color: var(--muted, #667085); }
.spow-metrics span { font-size: 12px; font-weight: 800; }
.spow-metrics b { display: block; margin: 7px 0 5px; font-size: 22px; overflow-wrap: anywhere; }
.spow-metrics small { min-height: 32px; font-size: 11px; line-height: 1.45; }
.spow-metric-primary b { color: var(--teal, #177e79); }
.spow-pending-note {
  margin: -6px 0 0;
  color: var(--muted, #667085);
  font-size: 12px;
}
.spow-transfer {
  display: grid;
  grid-template-columns: minmax(180px, .7fr) minmax(320px, 1.3fr);
  gap: 16px 22px;
  padding: 18px;
  border: 1px solid var(--spow-border);
  border-radius: 8px;
  background: var(--spow-soft);
}
.spow-transfer-form label { display: block; margin-bottom: 7px; font-size: 12px; font-weight: 800; }
.spow-input-row { gap: 8px; }
.spow-input-row input {
  min-width: 0;
  flex: 1;
  height: 38px;
  border: 1px solid var(--spow-border);
  border-radius: 7px;
  padding: 0 11px;
  background: var(--module-panel, #fff);
  color: inherit;
  font-size: 13px;
}
.spow-field-error { margin: 6px 0 0; color: #b42318; font-size: 11px; }
.spow-confirm,
.spow-alert { grid-column: 1 / -1; }
.spow-confirm {
  align-items: flex-end;
  gap: 18px;
  padding-top: 15px;
  border-top: 1px solid var(--spow-border);
}
.spow-confirm b { font-size: 13px; }
.spow-confirm small { color: var(--muted, #667085); font-size: 10px; overflow-wrap: anywhere; }
.spow-confirm-actions { flex: 0 0 auto; gap: 8px; }
.spow-alert {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  border-radius: 7px;
  padding: 11px 13px;
  font-size: 12px;
  line-height: 1.5;
}
.spow-alert-success { background: rgba(23, 126, 121, .1); color: #11645f; }
.spow-alert-error { background: rgba(180, 35, 24, .08); color: #9d2319; }
.spow-ledger-head { gap: 16px; }
.spow-ledger-head > span {
  flex: 0 0 auto;
  color: var(--teal, #177e79);
  font-size: 12px;
  font-weight: 900;
}
.spow-empty {
  border-block: 1px solid var(--spow-border);
  padding: 24px 4px;
  color: var(--muted, #667085);
  font-size: 12px;
  text-align: center;
}
.spow-ledger-list { border-top: 1px solid var(--spow-border); }
.spow-load-more { display: flex; justify-content: center; }
.spow-load-more button {
  min-height: 36px;
  border: 1px solid var(--spow-border);
  border-radius: 7px;
  padding: 0 16px;
  background: var(--module-panel, #fff);
  color: inherit;
  font-size: 12px;
  font-weight: 800;
  cursor: pointer;
}
.spow-ledger-row {
  gap: 18px;
  padding: 13px 2px;
  border-bottom: 1px solid var(--spow-border);
}
.spow-ledger-main { min-width: 0; flex: 1; }
.spow-ledger-title { gap: 8px; }
.spow-ledger-title b { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; }
.spow-ledger-title span {
  flex: 0 0 auto;
  border-radius: 999px;
  padding: 2px 7px;
  background: rgba(23, 126, 121, .1);
  color: var(--teal, #177e79);
  font-size: 10px;
  font-weight: 800;
}
.spow-ledger-main p { display: flex; flex-wrap: wrap; gap: 5px 12px; margin: 5px 0; color: var(--muted, #667085); font-size: 11px; }
.spow-ledger-main small { color: var(--muted, #667085); font-size: 10px; }
.spow-ledger-amount { flex: 0 0 auto; display: grid; justify-items: end; gap: 2px; text-align: right; }
.spow-ledger-amount b { font-size: 14px; }
.spow-ledger-amount small { color: var(--muted, #667085); font-size: 10px; }
.spow-ledger-amount.is-positive b { color: var(--teal, #177e79); }
.spow-ledger-amount.is-negative b { color: #b42318; }
.spow-legacy-details {
  border: 1px solid var(--spow-border);
  border-radius: 8px;
  padding: 12px 14px;
}
.spow-legacy-details summary { cursor: pointer; font-size: 12px; font-weight: 800; }
.spow-legacy-list { display: grid; gap: 7px; margin-top: 10px; }
.spow-legacy-list div { display: flex; justify-content: space-between; gap: 14px; font-size: 11px; }
.spow-legacy-list span { color: var(--muted, #667085); }

@media (max-width: 900px) {
  .spow-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .spow-metrics article:nth-child(2) { border-right: 0; }
  .spow-metrics article:nth-child(-n+2) { border-bottom: 1px solid var(--spow-border); }
  .spow-transfer { grid-template-columns: 1fr; }
  .spow-transfer-form,
  .spow-confirm,
  .spow-alert { grid-column: 1; }
}
@media (max-width: 620px) {
  .spow-head,
  .spow-ledger-head,
  .spow-confirm,
  .spow-ledger-row { align-items: stretch; flex-direction: column; }
  .spow-refresh { align-self: flex-start; }
  .spow-input-row { align-items: stretch; flex-wrap: wrap; }
  .spow-input-row input { flex-basis: 100%; }
  .spow-input-row button { flex: 1; }
  .spow-confirm-actions { width: 100%; }
  .spow-confirm-actions button { flex: 1; }
  .spow-ledger-amount { justify-items: start; text-align: left; }
}
</style>
