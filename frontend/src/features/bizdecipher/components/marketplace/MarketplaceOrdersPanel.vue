<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { CheckCircle2, CircleAlert, ClipboardList, RefreshCw, Send, Star } from '@lucide/vue'
import {
  marketplaceAPI,
  type MarketplaceOrder,
  type MarketplaceOrderAction,
  type MarketplaceOrderStatus,
} from '@/features/bizdecipher/api/marketplace'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
import MarketplaceOrderFunds from './MarketplaceOrderFunds.vue'
import type { OrderFunds } from '../../api/marketplaceFunds'

const statusLabels: Record<MarketplaceOrderStatus, string> = {
  quoted: '待确认',
  confirmed: '已确认',
  delivered: '待验收',
  accepted: '已验收',
  settled: '已完成',
  canceled: '已取消',
  disputed: '争议中',
}
const actionLabels: Record<MarketplaceOrderAction, string> = {
  confirm: '确认合作',
  deliver: '提交交付',
  accept: '验收通过',
  settle: '结束订单',
  cancel: '取消订单',
  dispute: '登记争议',
  review: '提交评价',
}
const noteActions = new Set<MarketplaceOrderAction>(['deliver', 'cancel', 'dispute'])
const orders = ref<MarketplaceOrder[]>([])
const activeOrder = ref<MarketplaceOrder | null>(null)
const nextCursor = ref<number | null>(null)
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')
const pendingAction = ref<MarketplaceOrderAction | null>(null)
const actionNote = ref('')
const reviewRating = ref(5)
const reviewBody = ref('')
let listRequestId = 0
let detailRequestId = 0

const orderFunds = ref<OrderFunds | null>(null)
const actionable = computed(() => activeOrder.value?.available_actions.filter(action => action !== 'review' && !(orderFunds.value?.status === 'unpaid' && action === 'confirm') && !(orderFunds.value?.status === 'released' && action === 'dispute')) ?? [])
const canReview = computed(() => Boolean(activeOrder.value?.available_actions.includes('review')))
const resolutions = computed(() => activeOrder.value?.events?.filter(event => event.event === 'admin_resolve_dispute') ?? [])

const toMessage = (value: unknown, fallback: string) => extractActionableApiErrorMessage(value, fallback)
const otherParty = (order: MarketplaceOrder) => order.viewer_role === 'buyer' ? order.seller_display_name : order.buyer_display_name
const formatTime = (value: string) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : ''
const statusLabel = (status: MarketplaceOrderStatus) => statusLabels[status] ?? status
const actionLabel = (action: MarketplaceOrderAction) => actionLabels[action] ?? action

async function loadOrders(append = false): Promise<void> {
  const requestId = ++listRequestId
  loading.value = true
  error.value = ''
  try {
    const page = await marketplaceAPI.listOrders(append ? nextCursor.value ?? undefined : undefined)
    if (requestId !== listRequestId) return
    orders.value = append ? [...orders.value, ...page.items] : page.items
    nextCursor.value = page.next_cursor ?? null
    const refreshOrder = orders.value.find(order => order.id === activeOrder.value?.id) ?? (!activeOrder.value ? orders.value[0] : undefined)
    if (refreshOrder) await openOrder(refreshOrder)
    else if (!append && activeOrder.value) await openOrder(activeOrder.value)
  } catch (value) {
    if (requestId === listRequestId) error.value = toMessage(value, '订单暂时无法读取，请重试。')
  } finally {
    if (requestId === listRequestId) loading.value = false
  }
}

async function openOrder(order: MarketplaceOrder): Promise<void> {
  const requestId = ++detailRequestId
  activeOrder.value = order
  orderFunds.value = null
  pendingAction.value = null
  actionNote.value = ''
  notice.value = ''
  try {
    const detail = await marketplaceAPI.getOrder(order.id)
    if (requestId === detailRequestId) {
      activeOrder.value = detail
      orders.value = orders.value.map(item => item.id === detail.id ? detail : item)
    }
  } catch (value) {
    if (requestId === detailRequestId) error.value = toMessage(value, '订单详情暂时无法读取，请重试。')
  }
}

async function beginAction(action: MarketplaceOrderAction): Promise<void> {
  if (!activeOrder.value || saving.value) return
  if (!noteActions.has(action)) {
    await applyAction(action)
    return
  }
  pendingAction.value = action
  actionNote.value = ''
}

async function applyAction(action: MarketplaceOrderAction, note = ''): Promise<void> {
  if (!activeOrder.value || saving.value) return
  const requestId = ++detailRequestId
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    const updated = await marketplaceAPI.applyOrderAction(activeOrder.value.id, action === 'review'
      ? { action, note, rating: reviewRating.value }
      : { action, note })
    if (requestId !== detailRequestId) return
    activeOrder.value = updated
    orders.value = orders.value.map(item => item.id === updated.id ? updated : item)
    pendingAction.value = null
    actionNote.value = ''
    if (action === 'review') reviewBody.value = ''
    notice.value = `${actionLabel(action)}已记录。`
  } catch (value) {
    if (requestId === detailRequestId) error.value = toMessage(value, '订单操作失败，请重试。')
  } finally {
    if (requestId === detailRequestId) saving.value = false
  }
}

async function submitActionNote(): Promise<void> {
  if (!pendingAction.value || !actionNote.value.trim()) return
  await applyAction(pendingAction.value, actionNote.value.trim())
}

async function submitReview(): Promise<void> {
  if (!activeOrder.value || !canReview.value || saving.value) return
  await applyAction('review', reviewBody.value.trim())
}

onMounted(() => { void loadOrders() })
</script>

<template>
  <section class="market-orders">
    <header>
      <div><p>合作订单</p><h2>我的订单</h2></div>
      <button class="order-icon" type="button" aria-label="刷新订单" :disabled="loading" @click="loadOrders()"><RefreshCw :size="17" /></button>
    </header>
    <p v-if="error" class="order-notice error" role="alert">{{ error }}</p>
    <p v-else-if="notice" class="order-notice" role="status">{{ notice }}</p>
    <div class="order-layout">
      <aside aria-label="订单列表">
        <button
          v-for="order in orders"
          :key="order.id"
          class="order-list-item"
          :class="{ active: activeOrder?.id === order.id }"
          type="button"
          @click="openOrder(order)"
        >
          <span>#{{ order.id }} · {{ statusLabel(order.status) }}</span>
          <strong>{{ order.listing_title }}</strong>
          <small>{{ order.viewer_role === 'buyer' ? '向' : '来自' }} {{ otherParty(order) }} · {{ formatTime(order.updated_at) }}</small>
        </button>
        <div v-if="!loading && !orders.length" class="order-empty compact"><ClipboardList :size="24" /><p>卖家报价后，订单会出现在这里。</p></div>
        <button v-if="nextCursor" class="order-load-more" type="button" :disabled="loading" @click="loadOrders(true)">加载更多</button>
      </aside>

      <article v-if="activeOrder" class="order-detail">
        <div class="order-detail-head">
          <div><span class="order-status" :data-status="activeOrder.status">{{ statusLabel(activeOrder.status) }}</span><h3>{{ activeOrder.listing_title }}</h3></div>
          <span>#{{ activeOrder.id }}</span>
        </div>
        <dl class="order-facts">
          <div><dt>{{ activeOrder.viewer_role === 'buyer' ? '服务方' : '需求方' }}</dt><dd>{{ otherParty(activeOrder) }}</dd></div>
          <div><dt>金额约定</dt><dd>{{ activeOrder.amount_text || '待双方约定' }}</dd></div>
          <div><dt>交付周期</dt><dd>{{ activeOrder.delivery_text || '待约定' }}</dd></div>
          <div><dt>可修改次数</dt><dd>{{ activeOrder.revision_limit }} 次</dd></div>
        </dl>
        <section class="order-scope"><h4>合作范围</h4><p>{{ activeOrder.scope_text }}</p></section>
        <section v-if="activeOrder.delivery_note || activeOrder.accept_note || activeOrder.cancel_reason || activeOrder.dispute_note" class="order-notes">
          <h4>处理说明</h4>
          <p v-if="activeOrder.delivery_note"><strong>{{ activeOrder.delivered_at ? '交付：' : '历史交付：' }}</strong>{{ activeOrder.delivery_note }}</p>
          <p v-if="activeOrder.accept_note"><strong>{{ activeOrder.accepted_at ? '验收：' : '历史验收：' }}</strong>{{ activeOrder.accept_note }}</p>
          <p v-if="activeOrder.cancel_reason"><strong>取消：</strong>{{ activeOrder.cancel_reason }}</p>
          <p v-if="activeOrder.dispute_note"><strong>争议：</strong>{{ activeOrder.dispute_note }}</p>
        </section>
        <MarketplaceOrderFunds :key="activeOrder.id" :order="activeOrder" @loaded="orderFunds = $event" @changed="loadOrders()" />
        <p v-if="orderFunds?.status === 'held'" class="order-boundary"><CircleAlert :size="15" />验收通过会将本订单款项放入卖家收益钱包，请确认交付物后操作。</p>
        <section v-if="resolutions.length" class="order-notes" aria-label="争议处理结果">
          <h4>争议处理结果</h4>
          <p v-for="event in resolutions" :key="event.id"><strong>管理员 {{ event.actor_name }} · {{ event.to_status === 'confirmed' ? '继续履约，重新交付与验收' : '终止合作' }}：</strong>{{ event.note }}<br />{{ formatTime(event.created_at) }}</p>
        </section>

        <div v-if="actionable.length" class="order-actions">
          <button v-for="action in actionable" :key="action" class="order-button" type="button" :disabled="saving" @click="beginAction(action)">
            <Send v-if="action === 'deliver'" :size="15" /><CheckCircle2 v-else :size="15" />{{ actionLabel(action) }}
          </button>
        </div>
        <form v-if="pendingAction" class="order-action-note" @submit.prevent="submitActionNote">
          <label>{{ actionLabel(pendingAction) }}说明<textarea v-model="actionNote" required rows="3" maxlength="2000" placeholder="写清交付物、取消原因或争议事实" /></label>
          <div><button class="order-button" type="button" @click="pendingAction = null">返回</button><button class="order-button primary" type="submit" :disabled="saving || !actionNote.trim()">确认记录</button></div>
        </form>

        <form v-if="canReview" class="order-review" @submit.prevent="submitReview">
          <div><Star :size="16" /><strong>完成评价</strong><select v-model.number="reviewRating" aria-label="评分"><option v-for="score in [5, 4, 3, 2, 1]" :key="score" :value="score">{{ score }} 星</option></select></div>
          <textarea v-model="reviewBody" rows="3" maxlength="2000" placeholder="评价合作过程、交付质量和沟通情况" />
          <button class="order-button primary" type="submit" :disabled="saving"><Star :size="15" />提交评价</button>
        </form>

        <section class="order-timeline">
          <h4>状态记录</h4>
          <ol>
            <li v-for="event in activeOrder.events" :key="event.id">
              <span>{{ formatTime(event.created_at) }}</span>
              <strong>{{ event.actor_name }} · {{ event.event === 'admin_resolve_dispute' ? '管理员处理争议' : event.event === 'quote' ? '提交报价' : actionLabel(event.event as MarketplaceOrderAction) }}</strong>
              <p>{{ event.note || `${statusLabel(event.from_status || 'quoted')} → ${statusLabel(event.to_status)}` }}</p>
            </li>
          </ol>
        </section>
        <section v-if="activeOrder.reviews?.length" class="order-reviews">
          <h4>双方评价</h4>
          <div v-for="review in activeOrder.reviews" :key="review.id">
            <strong>{{ review.author_name }} · {{ review.rating }} 星</strong>
            <p>{{ review.body || '未填写文字评价' }}</p>
          </div>
        </section>
      </article>

      <div v-else class="order-empty"><ClipboardList :size="30" /><h3>选择一笔订单</h3><p>订单详情只对买卖双方可见。</p></div>
    </div>
  </section>
</template>

<style scoped>
.market-orders{display:grid;gap:14px}.market-orders>header{display:flex;align-items:flex-start;justify-content:space-between;gap:12px;border-bottom:1px solid var(--zc-line);padding-bottom:14px}.market-orders header p,.market-orders header span{margin:0;color:var(--zc-muted);font-size:.75rem}.market-orders h2,.market-orders h3,.market-orders h4,.market-orders p{margin:0}.market-orders h2{margin-top:2px;color:var(--zc-text-strong);font-size:1.1rem}.market-orders h3{color:var(--zc-text-strong);font-size:.9375rem}.market-orders h4{color:var(--zc-text-strong);font-size:.8125rem}.order-icon{width:36px;height:36px;display:grid;place-items:center;border:1px solid var(--zc-line);border-radius:6px;background:var(--zc-surface);color:var(--zc-text);cursor:pointer}.order-icon:disabled{opacity:.6}.order-notice{border-left:3px solid var(--zc-accent);background:color-mix(in srgb,var(--zc-accent) 8%,transparent);padding:9px 11px;font-size:.8125rem}.order-notice.error{border-color:var(--zc-danger);color:var(--zc-danger)}.order-layout{min-height:520px;display:grid;grid-template-columns:280px minmax(0,1fr);border:1px solid var(--zc-line);border-radius:8px;overflow:hidden;background:var(--zc-surface)}.order-layout>aside{display:grid;align-content:start;gap:6px;border-right:1px solid var(--zc-line);padding:12px}.order-list-item{display:grid;gap:4px;border:1px solid var(--zc-line);border-radius:6px;background:var(--zc-bg);color:var(--zc-text);padding:10px;text-align:left;cursor:pointer}.order-list-item.active{border-color:var(--zc-accent);background:color-mix(in srgb,var(--zc-accent) 8%,var(--zc-surface))}.order-list-item span,.order-meta{color:var(--zc-muted);font-size:.6875rem}.order-list-item strong{overflow:hidden;color:var(--zc-text-strong);font-size:.8125rem;text-overflow:ellipsis;white-space:nowrap}.order-list-item small{color:var(--zc-muted);font-size:.6875rem}.order-load-more{border:0;background:transparent;color:var(--zc-accent);padding:8px;font-size:.75rem;cursor:pointer}.order-detail{display:grid;gap:16px;align-content:start;padding:18px}.order-detail-head{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}.order-detail-head>span{color:var(--zc-muted);font-size:.75rem}.order-status{display:inline-block;margin-bottom:6px;border-radius:999px;background:var(--zc-bg);padding:3px 8px;color:var(--zc-accent);font-size:.6875rem;font-weight:700}.order-status[data-status="canceled"],.order-status[data-status="disputed"]{color:var(--zc-danger)}.order-facts{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:8px;margin:0}.order-facts div{display:grid;gap:4px;border-top:1px solid var(--zc-line);padding-top:9px}.order-facts dt{color:var(--zc-muted);font-size:.6875rem}.order-facts dd{margin:0;color:var(--zc-text-strong);font-size:.75rem;overflow-wrap:anywhere}.order-scope,.order-notes,.order-timeline,.order-reviews{display:grid;gap:8px}.order-scope p,.order-notes p,.order-boundary,.order-review textarea,.order-action-note textarea,.order-timeline p,.order-reviews p{color:var(--zc-text);font-size:.8125rem;line-height:1.6;white-space:pre-wrap}.order-notes strong{color:var(--zc-text-strong)}.order-boundary{display:flex;align-items:center;gap:7px;border-top:1px solid var(--zc-line);border-bottom:1px solid var(--zc-line);padding:9px 0;color:var(--zc-muted)!important}.order-actions{display:flex;flex-wrap:wrap;gap:7px}.order-button{min-height:34px;display:inline-flex;align-items:center;justify-content:center;gap:6px;border:1px solid var(--zc-line);border-radius:6px;background:var(--zc-surface);color:var(--zc-text);padding:0 11px;font:650 .75rem/1 inherit;cursor:pointer}.order-button.primary{border-color:var(--zc-accent);background:var(--zc-accent);color:var(--zc-accent-contrast,#fff)}.order-button:disabled{opacity:.6;cursor:wait}.order-action-note,.order-review{display:grid;gap:9px;border:1px solid var(--zc-line);border-radius:8px;padding:12px}.order-action-note label,.order-review{color:var(--zc-text-strong);font-size:.75rem}.order-action-note label{display:grid;gap:7px}.order-action-note textarea,.order-review textarea{width:100%;border:1px solid var(--zc-line);border-radius:6px;background:var(--zc-bg);color:var(--zc-text);padding:9px 10px;font:inherit;outline:none;resize:vertical}.order-action-note>div,.order-review>div{display:flex;align-items:center;justify-content:flex-end;gap:7px}.order-review>div{justify-content:flex-start}.order-review select{border:1px solid var(--zc-line);border-radius:5px;background:var(--zc-bg);color:var(--zc-text);padding:4px 7px}.order-timeline ol{display:grid;gap:0;margin:0;padding:0;list-style:none}.order-timeline li{position:relative;display:grid;gap:3px;border-left:1px solid var(--zc-line);padding:0 0 13px 14px}.order-timeline li::before{position:absolute;top:4px;left:-4px;width:7px;height:7px;border-radius:50%;background:var(--zc-accent);content:""}.order-timeline li span{color:var(--zc-muted);font-size:.6875rem}.order-timeline li strong{color:var(--zc-text-strong);font-size:.75rem}.order-timeline li p{margin:0;color:var(--zc-muted)}.order-reviews>div{display:grid;gap:3px;border-top:1px solid var(--zc-line);padding-top:8px}.order-reviews strong{color:var(--zc-text-strong);font-size:.75rem}.order-empty{grid-column:1/-1;min-height:220px;display:grid;place-items:center;align-content:center;gap:8px;color:var(--zc-muted);text-align:center;padding:24px}.order-empty.compact{min-height:150px}.order-empty h3{color:var(--zc-text-strong)}.order-empty p{max-width:340px;color:var(--zc-muted);font-size:.8125rem}@media(max-width:820px){.order-layout{grid-template-columns:1fr}.order-layout>aside{border-right:0;border-bottom:1px solid var(--zc-line)}.order-facts{grid-template-columns:1fr 1fr}}@media(max-width:520px){.market-orders>header,.order-detail-head{align-items:flex-start}.order-detail{padding:14px}.order-facts{grid-template-columns:1fr}.order-actions .order-button{flex:1}.order-action-note>div{justify-content:stretch}.order-action-note .order-button{flex:1}}
</style>
