<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { Wallet, CheckCircle2 } from '@lucide/vue'
import { useAuthStore } from '@/stores/auth'
import { marketplaceFundsAPI, type OrderFunds } from '../../api/marketplaceFunds'
import { type MarketplaceOrder } from '../../api/marketplace'
import { extractActionableApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ order: MarketplaceOrder }>()
const emit = defineEmits<{ changed: []; loaded: [funds: OrderFunds | null] }>()
const auth = useAuthStore()
const funds = ref<OrderFunds | null>(null)
const enabled = ref(false)
const amount = ref('')
const agreed = ref(false)
const saving = ref(false)
const loading = ref(false)
const error = ref('')
let epoch = 0
let operation = ''
const labels = { unpaid: '待付款', held: '待交付与验收', released: '已入卖家收益钱包', refunded: '已原路退回站内余额' }
const canPrice = computed(() => !funds.value && props.order.status === 'quoted' && props.order.viewer_role === 'seller')
function token() {
  const bytes = new Uint8Array(20)
  globalThis.crypto.getRandomValues(bytes)
  return `market_${Array.from(bytes, x => x.toString(16).padStart(2, '0')).join('')}`
}
async function load() {
  const current = ++epoch
  funds.value = null; agreed.value = false; amount.value = ''; operation = ''; error.value = ''
  loading.value = true; saving.value = false
  try {
    const [f, p] = await Promise.all([marketplaceFundsAPI.get(props.order.id), marketplaceFundsAPI.policy()])
    if (current !== epoch) return
    funds.value = f; enabled.value = p.enabled; emit('loaded', f)
  } catch (e) { if (current === epoch) error.value = extractActionableApiErrorMessage(e, '资金记录读取失败') }
  finally { if (current === epoch) loading.value = false }
}
async function submit(pay: boolean) {
  if (saving.value || loading.value || pay && (!agreed.value || !funds.value || !enabled.value)) return
  const current = epoch
  saving.value = true; error.value = ''
  try {
    let f: OrderFunds
    if (pay) {
      operation ||= token()
      f = await marketplaceFundsAPI.pay(props.order.id, funds.value!.amount, operation)
    } else f = await marketplaceFundsAPI.price(props.order.id, amount.value)
    if (current !== epoch) return
    funds.value = f; agreed.value = false; emit('loaded', f); emit('changed')
  } catch (e) { if (current === epoch) error.value = extractActionableApiErrorMessage(e, '操作未完成，可重试') }
  finally { if (current === epoch) saving.value = false }
}
watch(() => [props.order.id, props.order.status, auth.user?.id], load, { immediate: true })
onBeforeUnmount(() => { ++epoch })
</script>
<template>
  <section class="funds" aria-label="订单资金">
    <h4><Wallet :size="16" />订单资金</h4>
    <p v-if="loading" role="status">正在读取</p>
    <p v-if="error" class="funds-error" role="alert">{{ error }}</p>
    <template v-if="funds">
      <div class="funds-summary"><strong>USD {{ funds.amount }}</strong><span>{{ order.status === 'canceled' && funds.status === 'unpaid' ? '已取消 · 未付款' : labels[funds.status] }}</span></div>
      <p v-if="funds.status === 'held'">本订单款项尚未入卖家钱包。验收通过即放款；取消或争议裁定取消时退回买家站内余额。</p>
      <form v-if="funds.status === 'unpaid' && order.status === 'quoted' && order.viewer_role === 'buyer'" @submit.prevent="submit(true)">
        <label><input v-model="agreed" type="checkbox" :disabled="!enabled || saving" />确认从站内余额支付 USD {{ funds.amount }}，验收后放款。</label>
        <button :disabled="!enabled || !agreed || saving || loading"><Wallet :size="15" />付款并确认合作</button>
        <p v-if="!enabled">平台暂未开放站内付款。</p>
      </form>
    </template>
    <form v-else-if="canPrice && !loading" @submit.prevent="submit(false)">
      <label>站内结算金额 · USD<input v-model="amount" required inputmode="decimal" pattern="(0|[1-9][0-9]{0,11})(\.[0-9]{1,8})?" placeholder="例如 25.00" :disabled="saving" /></label>
      <p>文字报价不会自动换算。确认后金额不可修改，买家需单独确认付款。</p>
      <button :disabled="saving || !amount || loading"><CheckCircle2 :size="15" />确认结算金额</button>
    </form>
    <p v-else-if="!loading && !error">{{ order.legacy_cooperation ? '历史文字合作约定，未启用站内收款。' : '等待服务方确认 USD 结算金额，付款后开始合作。' }}</p>
  </section>
</template>
<style scoped>
.funds{display:grid;gap:9px;border-block:1px solid var(--zc-line);padding:14px 0}.funds h4{display:flex;gap:7px;align-items:center;margin:0;font-size:.8125rem}.funds-summary{display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap}.funds p,.funds span,.funds label{font-size:.75rem;color:var(--zc-muted);line-height:1.6;margin:0}.funds form{display:grid;gap:9px}.funds input:not([type=checkbox]){display:block;width:min(100%,240px);margin-top:6px;padding:8px;border:1px solid var(--zc-line);border-radius:5px;background:var(--zc-bg);color:var(--zc-text)}.funds button{display:inline-flex;align-items:center;justify-content:center;gap:7px;justify-self:start;padding:8px 12px;border:1px solid var(--zc-accent);border-radius:6px;background:var(--zc-accent);color:var(--zc-accent-contrast,#fff);font-size:.75rem}.funds button:disabled{opacity:.5}.funds .funds-error{color:var(--zc-danger)}
</style>
