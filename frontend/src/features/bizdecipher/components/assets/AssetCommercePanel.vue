<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { saveAs } from 'file-saver'
import Icon from '@/components/icons/Icon.vue'
import AssetExecutionPanel from './AssetExecutionPanel.vue'
import { useAuthStore } from '@/stores/auth'
import { assetCommerce, type AssetCommerceInfo, type AssetCommercePolicy, type AssetPurchase } from '@/features/bizdecipher/api/assetCommerce'
import { downloadCapabilityAssetPackage, listCapabilityAssetVersions, type CapabilityAssetVersion } from '@/features/bizdecipher/api/bizdecipher'
import { extractActionableApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ assetId?: number }>()
const auth = useAuthStore()
const info = ref<AssetCommerceInfo | null>(null)
const policy = ref<AssetCommercePolicy | null>(null)
const orders = ref<AssetPurchase[]>([])
const versions = ref<CapabilityAssetVersion[]>([])
const cursor = ref(0)
const busy = ref(false)
const error = ref('')
const mode = ref('free')
const price = ref('0')
const reason = ref('')
const refundReasons = ref<Record<number, string>>({})
const selectedVersion = ref('')
const confirmed = ref(false)
const owner = computed(() => info.value?.owner_user_id === auth.user?.id)
let generation = 0

function operation(key: string) {
  const storedKey = `asset-commerce:${auth.user?.id}:${key}`
  let id = sessionStorage.getItem(storedKey)
  if (!id) {
    id = crypto.randomUUID()
    sessionStorage.setItem(storedKey, id)
  }
  return { id, clear: () => sessionStorage.removeItem(storedKey) }
}

async function load() {
  const current = ++generation
  error.value = ''
  try {
    const nextPolicy = await assetCommerce.policy()
    if (current !== generation) return
    policy.value = nextPolicy
    if (!props.assetId) return
    const id = props.assetId
    const nextInfo = await assetCommerce.info(id)
    if (current !== generation) return
    info.value = nextInfo
    mode.value = nextInfo.pricing_type === 'paid' ? 'paid' : 'free'
    price.value = nextInfo.price
    if (auth.user) {
      const history = await assetCommerce.history(id)
      if (current !== generation) return
      orders.value = history.items
      cursor.value = history.next_cursor
    }
    // Moderators can inspect receipts but do not gain package delivery rights.
    if (nextInfo.can_download || nextInfo.status === 'listed') {
      const nextVersions = await listCapabilityAssetVersions(id)
      if (current !== generation) return
      versions.value = nextVersions.filter(v => v.status === 'published')
      selectedVersion.value = versions.value[0]?.version || ''
    }
  } catch (e) {
    if (current === generation) error.value = extractActionableApiErrorMessage(e, '交易信息加载失败')
  }
}

async function act(work: () => Promise<void>) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try { await work() } catch (e) { error.value = extractActionableApiErrorMessage(e, '操作失败，请重试') }
  finally { busy.value = false }
}
function purchase() {
  if (!props.assetId || !info.value || !confirmed.value) return
  const id = props.assetId
  const amount = info.value.price
  const op = operation(`purchase:${id}:${amount}`)
  void act(async () => {
    await assetCommerce.purchase(id, op.id, amount)
    op.clear()
    confirmed.value = false
    await load()
  })
}
function refund(order: AssetPurchase) {
  const text = refundReasons.value[order.id]?.trim()
  if (!text || !window.confirm(`确认退还 USD ${order.amount}？`)) return
  const op = operation(`refund:${order.id}:${text}`)
  void act(async () => {
    await assetCommerce.refund(order.asset_id, order.id, op.id, text)
    op.clear()
    await load()
  })
}
function deliver(reuse: boolean) {
  const id = props.assetId
  const version = selectedVersion.value
  if (!id || !version) return
  void act(async () => {
    if (reuse) {
      const data = await assetCommerce.reuse(id, version, crypto.randomUUID())
      saveAs(new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' }), `asset-${id}-${version}.json`)
    } else saveAs(await downloadCapabilityAssetPackage(id, version), `asset-${id}-${version}.zip`)
  })
}
watch(() => [props.assetId, auth.user?.id], () => {
  info.value = null
  orders.value = []
  versions.value = []
  confirmed.value = false
  void load()
}, { immediate: true })
</script>

<template>
  <section class="asset-commerce" aria-label="资产交易">
    <p v-if="error" role="alert">{{ error }} <button type="button" :disabled="busy" @click="load">重试</button></p>
    <template v-if="!assetId && auth.isAdmin && policy">
      <h3>资产付费交易</h3>
      <label><input :checked="policy.enabled" type="checkbox" disabled />{{ policy.enabled ? '已开放' : '未开放' }}</label>
      <label>变更原因<input v-model="reason" maxlength="1000" /></label>
      <button type="button" :disabled="busy || !reason.trim()" @click="act(async () => { policy = await assetCommerce.setPolicy(!policy!.enabled, reason); reason = '' })">
        {{ policy.enabled ? '暂停新购买' : '开放购买' }}
      </button>
    </template>
    <template v-if="info && assetId">
      <h3>{{ info.pricing_type === 'paid' ? `USD ${info.price}` : '资产交付' }}</h3>
      <form v-if="owner" class="commerce-row" @submit.prevent="act(async () => { await assetCommerce.pricing(assetId!, mode, mode === 'free' ? '0' : price); await load() })">
        <label>定价<select v-model="mode"><option value="free">免费</option><option value="paid">付费</option></select></label>
        <label v-if="mode === 'paid'">USD<input v-model="price" inputmode="decimal" required pattern="\d+(\.\d{1,8})?" /></label>
        <button type="submit" :disabled="busy"><Icon name="check" size="sm" />保存定价</button>
      </form>
      <template v-else-if="info.pricing_type === 'paid' && !info.can_download">
        <label v-if="auth.user && policy?.enabled && info.has_package"><input v-model="confirmed" type="checkbox" />确认从余额支付 USD {{ info.price }}</label>
        <button type="button" :disabled="busy || !confirmed || !auth.user || !policy?.enabled || !info.has_package" @click="purchase">购买资产</button>
        <span v-if="!auth.user">请先登录</span>
        <span v-else-if="!policy?.enabled">购买暂未开放</span>
      </template>
      <div v-if="info.can_download && versions.length" class="commerce-row">
        <label>版本<select v-model="selectedVersion"><option v-for="v in versions" :key="v.id" :value="v.version">{{ v.version }}</option></select></label>
        <button type="button" :disabled="busy" @click="deliver(false)"><Icon name="download" size="sm" />下载 ZIP</button>
        <button type="button" :disabled="busy || !auth.user" @click="deliver(true)"><Icon name="download" size="sm" />导出复用包</button>
      </div>
      <div v-if="orders.length" class="receipt-list">
        <h4>交易记录</h4>
        <div v-for="order in orders" :key="order.id" class="receipt">
          <span>#{{ order.id }} · USD {{ order.amount }} · {{ order.status === 'refunded' ? '已退款' : '已购买' }}</span>
          <template v-if="auth.isAdmin && order.status === 'active'">
            <input v-model="refundReasons[order.id]" aria-label="退款原因" placeholder="退款原因" maxlength="1000" />
            <button type="button" :disabled="busy || !refundReasons[order.id]?.trim()" @click="refund(order)">退款并撤销交付权限</button>
          </template>
          <span v-if="order.refund_reason">{{ order.refund_reason }}</span>
        </div>
        <button v-if="cursor" type="button" :disabled="busy" @click="act(async () => { const page = await assetCommerce.history(assetId!, cursor); orders.push(...page.items); cursor = page.next_cursor })">更多记录</button>
      </div>
      <AssetExecutionPanel v-if="info.can_download && selectedVersion && auth.user" :asset-id="assetId" :version="selectedVersion" />
    </template>
  </section>
</template>

<style scoped>
.asset-commerce { padding-block: 16px; border-block: 1px solid #d1d5db; display: grid; gap: 12px; min-width: 0; color: inherit; }
h3, h4 { margin: 0; font-size: 16px; }
.commerce-row, .receipt { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; }
label { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
input:not([type=checkbox]), select { padding: 6px 8px; border: 1px solid #9ca3af; border-radius: 4px; min-width: 0; max-width: 100%; color: inherit; background: transparent; }
button { display: inline-flex; align-items: center; gap: 6px; padding: 6px 10px; border: 1px solid #9ca3af; border-radius: 4px; white-space: normal; }
button:disabled { opacity: .5; }
.receipt-list { display: grid; gap: 10px; }
.receipt { padding-block: 8px; border-bottom: 1px solid #e5e7eb; overflow-wrap: anywhere; }
[role=alert] { color: #b91c1c; }
</style>
