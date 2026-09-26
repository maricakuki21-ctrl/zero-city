<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { RefreshCw } from '@lucide/vue'
import { createWithdrawal, cancelWithdrawal, getWithdrawalPolicy, listWithdrawals, withdrawalStatus, type Withdrawal, type WithdrawalInput, type WithdrawalPolicy } from '../../api/withdrawals'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
import { getAuthGeneration } from '@/utils/authSession'
const session = getAuthGeneration()
let mounted = true
onBeforeUnmount(() => { mounted = false })
const active = () => mounted && session === getAuthGeneration()
const emit = defineEmits<{ changed: [] }>()
const props = defineProps<{ ownerId: number }>()
const draftKey = `withdrawal-token:${props.ownerId}`
const draftToken = ref(sessionStorage.getItem(draftKey) || '')
const policy = ref<WithdrawalPolicy>()
const rows = ref<Withdrawal[]>([])
const busy = ref(false)
const error = ref('')
const amount = ref('')
const channel = ref('')
const recipient = ref('')
const pendingInput = ref<WithdrawalInput>()
const more = ref(false)
async function load(append = false) {
  if (!active()) return
  busy.value = true; error.value = ''
  try {
    const config = await getWithdrawalPolicy()
    if (!active()) return
    policy.value = config
    const page = await listWithdrawals(false, append ? rows.value.at(-1)?.id : 0)
    if (!active()) return
    rows.value = append ? [...rows.value, ...page] : page
    if (draftToken.value && page.some(row => row.operation_id === draftToken.value)) {
      sessionStorage.removeItem(draftKey); draftToken.value = ''; pendingInput.value = undefined
      amount.value = ''; recipient.value = ''; emit('changed')
    }
    more.value = page.length === 100
    if (!channel.value) channel.value = policy.value.channels[0] || ''
  } catch (e) { if (active()) error.value = extractActionableApiErrorMessage(e, '提现信息加载失败') }
  finally { if (active()) busy.value = false }
}
async function submit() {
  if (busy.value || !active()) return
  busy.value = true; error.value = ''
  // Keep the same payload/token after an ambiguous network result, never resubmit as a new payment.
  try {
    draftToken.value ||= typeof globalThis.crypto.randomUUID === 'function'
      ? globalThis.crypto.randomUUID()
      : Array.from(globalThis.crypto.getRandomValues(new Uint8Array(16)), n => n.toString(16).padStart(2, '0')).join('')
    sessionStorage.setItem(draftKey, draftToken.value)
    pendingInput.value ??= { operation_id: draftToken.value, amount: amount.value, channel: channel.value, recipient: recipient.value }
    await createWithdrawal(pendingInput.value)
    if (!active()) return
    sessionStorage.removeItem(draftKey); draftToken.value = ''
    pendingInput.value = undefined; amount.value = ''; recipient.value = ''
    emit('changed'); await load()
  } catch (e) {
    if (!active()) return
    const status = (e as { response?: { status?: number } })?.response?.status
    if (status === 400 || status === 409) {
      // Let the owner correct/re-enter the original payload, but never replace
      // a token whose earlier result may already have committed.
      pendingInput.value = undefined
    }
    error.value = extractActionableApiErrorMessage(e, '提交结果未确认，请使用原申请重试并检查记录。')
  }
  finally { if (active()) busy.value = false }
}
async function cancel(id: number) {
  if (!active() || busy.value) return
  busy.value = true; error.value = ''
  try { await cancelWithdrawal(id); if (!active()) return; emit('changed'); await load() }
  catch (e) { if (active()) error.value = extractActionableApiErrorMessage(e, '撤销失败，请刷新确认状态') }
  finally { if (active()) busy.value = false }
}
onMounted(() => load())
</script>

<template>
  <section class="withdrawal-panel" aria-label="收益提现">
    <header><h2>收益提现</h2><button title="刷新提现记录" aria-label="刷新提现记录" :disabled="busy" @click="load()"><RefreshCw :size="16" /></button></header>
    <p>仅可提取已入账的经营与创作可用收益，金额单位 USD（账本美元）；站内余额、充值和积分不可提现。外部到账币种与费用以人工打款说明为准。</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="policy && !policy.enabled"><strong>提现尚未开放</strong> · 收益仍可转入站内余额。</p>
    <p v-if="policy?.instructions" class="instructions">{{ policy.instructions }}</p>
    <form v-if="policy?.enabled" @submit.prevent="submit">
      <label>提现金额（USD）<input v-model="amount" :disabled="busy || !!pendingInput" inputmode="decimal" required pattern="(0|[1-9][0-9]{0,11})(\.[0-9]{1,12})?" maxlength="25" /></label>
      <label>打款渠道<select v-model="channel" :disabled="busy || !!pendingInput" required><option v-for="item in policy.channels" :key="item">{{ item }}</option></select></label>
      <label>收款账户与姓名<input v-model="recipient" :disabled="busy || !!pendingInput" required maxlength="500" autocomplete="off" /></label>
      <button :disabled="busy" type="submit">{{ pendingInput ? '重试原申请' : '申请人工提现' }}</button>
    </form>
    <p v-if="draftToken">原申请结果尚待确认，请勿重复创建申请。刷新后请填写原申请内容重试，同一申请不会重复扣款。</p>
    <p v-if="!busy && !rows.length && !error">暂无提现申请</p>
    <ol>
      <li v-for="row in rows" :key="row.id">
        <strong>#{{ row.id }} · {{ row.amount }} USD</strong><span>{{ withdrawalStatus[row.status] }}</span>
        <span>{{ row.channel }} · {{ row.recipient }}</span>
        <span v-if="row.reference">打款凭证：{{ row.reference }}</span>
        <span v-if="row.reason">{{ row.reason }}</span>
        <button v-if="row.status === 'pending'" :disabled="busy" @click="cancel(row.id)">撤销并退回收益</button>
      </li>
    </ol>
    <button v-if="more" :disabled="busy" @click="load(true)">加载更早记录</button>
  </section>
</template>

<style scoped>
.withdrawal-panel { padding-block: 20px; border-block: 1px solid var(--bd-ui-line); }
header { display:flex; align-items:center; justify-content:space-between; }
h2 { font-size:18px; margin:0; } p { font-size:13px; color:var(--bd-text-secondary); line-height:1.6; }
form { display:grid; grid-template-columns:1fr 1fr 2fr auto; gap:12px; align-items:end; }
label { display:grid; gap:6px; font-size:12px; min-width:0; }
input, select, button { min-height:36px; border:1px solid var(--bd-ui-line); border-radius:4px; background:var(--bd-surface); color:inherit; padding:8px; min-width:0; }
button:disabled { opacity:.5; } ol { list-style:none; padding:0; } li { display:flex; flex-wrap:wrap; gap:10px; align-items:center; border-bottom:1px solid var(--bd-ui-line); padding:12px 0; font-size:13px; overflow-wrap:anywhere; }
.instructions { white-space:pre-wrap; } [role=alert] { color:var(--bd-accent-red,#c33); }
@media(max-width:700px) { form { grid-template-columns:1fr; } }
</style>
