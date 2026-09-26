<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RefreshCw, Gavel } from '@lucide/vue'
import { marketplaceAPI, type MarketplaceOrder } from '@/features/bizdecipher/api/marketplace'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
import MarketplaceFundsPolicy from './MarketplaceFundsPolicy.vue'
import MarketplaceOrderFunds from './MarketplaceOrderFunds.vue'

const orders = ref<MarketplaceOrder[]>([])
const selected = ref<MarketplaceOrder | null>(null)
const cursor = ref<number>()
const loading = ref(false)
const detailLoading = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')
const reason = ref('')
const outcome = ref<'confirmed' | 'canceled'>('confirmed')
let detailRequest = 0
async function load(append = false) {
  if (loading.value || saving.value) return
  loading.value = true
  error.value = ''
  try {
    const page = await marketplaceAPI.listDisputes(append ? cursor.value : undefined)
    orders.value = append ? [...orders.value, ...page.items] : page.items
    cursor.value = page.next_cursor ?? undefined
  } catch (err) {
    error.value = extractActionableApiErrorMessage(err, '争议列表读取失败，请重试。')
  } finally { loading.value = false }
}
async function open(id: number) {
  if (saving.value) return
  const request = ++detailRequest
  selected.value = null
  detailLoading.value = true
  error.value = ''
  notice.value = ''
  reason.value = ''
  outcome.value = 'confirmed'
  try {
    const order = await marketplaceAPI.getDispute(id)
    if (request === detailRequest) selected.value = order
  } catch (err) {
    if (request === detailRequest) error.value = extractActionableApiErrorMessage(err, '争议详情读取失败，请重试。')
  } finally { if (request === detailRequest) detailLoading.value = false }
}
async function resolve() {
  if (!selected.value || selected.value.status !== 'disputed' || saving.value || !reason.value.trim()) return
  saving.value = true
  error.value = ''
  try {
    const updated = await marketplaceAPI.resolveDispute(selected.value.id, { outcome: outcome.value, reason: reason.value.trim() })
    selected.value = updated
    orders.value = orders.value.filter(order => order.id !== updated.id)
    reason.value = ''
    notice.value = '争议处理已记录，买卖双方可在订单中查看处理结果和资金状态。'
  } catch (err) {
    error.value = extractActionableApiErrorMessage(err, '处理失败；订单可能已被其他管理员处理，请刷新详情。')
  } finally { saving.value = false }
}
onMounted(() => { void load() })
</script>

<template>
  <section class="disputes" aria-label="市场争议管理">
    <header><h2>订单争议</h2><button type="button" title="刷新争议" aria-label="刷新争议" :disabled="loading || saving" @click="load()"><RefreshCw :size="18" /></button></header>
    <MarketplaceFundsPolicy />
    <p v-if="error" role="alert">{{ error }}</p><p v-if="notice" role="status">{{ notice }}</p>
    <p v-if="loading" role="status">正在读取争议…</p>
    <p v-else-if="!orders.length">暂无待处理争议</p>
    <div class="dispute-layout">
      <aside aria-label="待处理争议">
        <button v-for="order in orders" :key="order.id" type="button" :disabled="saving" :aria-pressed="selected?.id === order.id" @click="open(order.id)">
          <strong>#{{ order.id }} {{ order.listing_title }}</strong><span>{{ order.buyer_display_name }} / {{ order.seller_display_name }}</span>
        </button>
        <button v-if="cursor" type="button" :disabled="loading || saving" @click="load(true)">加载更多</button>
      </aside>
      <p v-if="detailLoading" role="status">正在读取详情…</p>
      <article v-else-if="selected">
        <header><h3>#{{ selected.id }} {{ selected.listing_title }}</h3><button type="button" title="刷新争议详情" aria-label="刷新争议详情" :disabled="saving" @click="open(selected.id)"><RefreshCw :size="16" /></button></header>
        <dl><dt>需求方 / 服务方</dt><dd>{{ selected.buyer_display_name }} / {{ selected.seller_display_name }}</dd><dt>合作范围</dt><dd>{{ selected.scope_text }}</dd><dt>金额文字约定</dt><dd>{{ selected.amount_text || '未约定' }}</dd><dt>争议说明</dt><dd>{{ selected.dispute_note }}</dd></dl>
        <MarketplaceOrderFunds :key="selected.id" :order="selected" />
        <p>终止合作将把本订单尚未放款的款项退回买家站内余额；旧的文字订单不产生退款。</p>
        <form v-if="selected.status === 'disputed'" @submit.prevent="resolve">
          <label>处理结果<select v-model="outcome" :disabled="saving"><option value="confirmed">继续履约（重新交付与验收）</option><option value="canceled">终止合作</option></select></label>
          <label>处理理由<textarea v-model="reason" required maxlength="1000" rows="4" :disabled="saving" /></label>
          <button type="submit" :disabled="saving || !reason.trim()"><Gavel :size="16" />{{ saving ? '提交中…' : '确认处理' }}</button>
        </form>
        <h4>订单记录</h4>
        <ol><li v-for="event in selected.events" :key="event.id"><strong>{{ event.event === 'admin_resolve_dispute' ? '管理员处理' : event.actor_name }} · {{ event.to_status }}</strong><time>{{ new Date(event.created_at).toLocaleString() }}</time><p>{{ event.note }}</p></li></ol>
      </article>
    </div>
  </section>
</template>

<style scoped>
.disputes{display:grid;gap:12px;color:var(--zc-text)}header{display:flex;align-items:center;justify-content:space-between;gap:12px}h2,h3,h4,p{margin:0}h2{font-size:1.1rem}h3{font-size:1rem}p,dt,time{font-size:.8125rem;color:var(--zc-muted)}button,select,textarea{border:1px solid var(--zc-line);border-radius:6px;background:var(--zc-surface);color:var(--zc-text);font:inherit;padding:8px}button{display:flex;align-items:center;gap:6px;cursor:pointer}button:disabled{opacity:.6;cursor:default}header button{width:36px;height:36px;justify-content:center}button[aria-pressed=true]{border-color:var(--zc-accent)}[role=alert]{color:var(--zc-danger)}.dispute-layout{display:grid;grid-template-columns:260px minmax(0,1fr);gap:18px}aside,article,form,label{display:grid;align-content:start;gap:10px}aside button{display:grid;text-align:left;overflow-wrap:anywhere}aside span{font-size:.75rem}article{min-width:0;border-left:1px solid var(--zc-line);padding-left:18px}dl{margin:0;display:grid;gap:6px}dd{margin:0;white-space:pre-wrap;overflow-wrap:anywhere}form{border-block:1px solid var(--zc-line);padding-block:14px}label{font-size:.8125rem}textarea{width:100%;resize:vertical}select{width:100%;min-width:0}ol{margin:0;padding-left:20px}li{padding-bottom:12px;overflow-wrap:anywhere}li time{display:block}li p{white-space:pre-wrap}@media(max-width:720px){.dispute-layout{grid-template-columns:1fr}article{border-left:0;border-top:1px solid var(--zc-line);padding:14px 0 0}}
</style>
