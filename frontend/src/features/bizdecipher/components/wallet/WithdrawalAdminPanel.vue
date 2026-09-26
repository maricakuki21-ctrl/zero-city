<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { RefreshCw, Save, ShieldCheck } from '@lucide/vue'
import { getWithdrawalPolicy, saveWithdrawalPolicy, listWithdrawals, actWithdrawal, withdrawalStatus, type Withdrawal, type WithdrawalPolicy } from '../../api/withdrawals'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
import { useAuthStore } from '@/stores/auth'
const auth = useAuthStore()
let epoch = 0
onBeforeUnmount(() => { epoch++ })
const policy = ref<WithdrawalPolicy>({ enabled: false, channels: [], instructions: '' })
const channels = ref('')
const rows = ref<Withdrawal[]>([])
const busy = ref(false)
const loaded = ref(false)
const savedEnabled = ref(false)
const error = ref('')
const notice = ref('')
const more = ref(false)
const selected = ref<Withdrawal>()
const action = ref('processing')
const reason = ref('')
const reference = ref('')
const confirmed = ref(false)
async function load(append = false) {
  if (auth.user?.role !== 'admin') return
  const current = epoch
  busy.value = true; error.value = ''
  try {
    const config = await getWithdrawalPolicy(true)
    if (current !== epoch) return
    policy.value = config; savedEnabled.value = config.enabled; channels.value = policy.value.channels.join('\n')
    const page = await listWithdrawals(true, append ? rows.value.at(-1)?.id : 0)
    if (current !== epoch) return
    rows.value = append ? [...rows.value, ...page] : page; more.value = page.length === 100; loaded.value = true
  } catch (e) { if (current === epoch) error.value = extractActionableApiErrorMessage(e, '提现运营数据加载失败') }
  finally { if (current === epoch) busy.value = false }
}
async function save() {
  if (auth.user?.role !== 'admin' || busy.value) return
  const current = epoch
  busy.value = true; error.value = ''; notice.value = ''
  try {
    await saveWithdrawalPolicy({ ...policy.value, channels: channels.value.split('\n').map(s => s.trim()).filter(Boolean) })
    if (current === epoch) { notice.value = '提现配置已保存。'; await load() }
  }
  catch (e) { if (current === epoch) error.value = extractActionableApiErrorMessage(e, '保存失败') }
  finally { if (current === epoch) busy.value = false }
}
function select(row: Withdrawal, next: string) {
  selected.value = row; action.value = next; reason.value = ''; reference.value = ''; confirmed.value = false; notice.value = ''
}
async function apply() {
  if (auth.user?.role !== 'admin' || !selected.value || busy.value || (selected.value.status === 'processing' && !confirmed.value)) return
  const current = epoch
  busy.value = true; error.value = ''
  try {
    await actWithdrawal(selected.value.id, action.value, reason.value, reference.value, action.value === 'rejected' && selected.value.status === 'processing' && confirmed.value)
    if (current !== epoch) return
    selected.value = undefined; notice.value = '处理结果已记录。'; await load()
  } catch (e) { if (current === epoch) error.value = extractActionableApiErrorMessage(e, '处理结果未确认，请刷新核对，勿重复打款。') }
  finally { if (current === epoch) busy.value = false }
}
watch(() => [auth.user?.id, auth.user?.role], () => {
  epoch++; rows.value = []; selected.value = undefined; loaded.value = false; savedEnabled.value = false
  policy.value = { enabled: false, channels: [], instructions: '' }; channels.value = ''
  reason.value = ''; reference.value = ''; confirmed.value = false; busy.value = false; error.value = ''; notice.value = ''
  void load()
}, { immediate: true, flush: 'sync' })
</script>

<template>
  <section class="withdrawal-admin" aria-label="人工提现运营">
    <header><div class="heading"><ShieldCheck :size="20" /><h2>人工提现运营</h2></div><span v-if="loaded" class="policy-state" :class="{ enabled: savedEnabled }">{{ savedEnabled ? '申请已开放' : '申请已关闭' }}</span></header>
    <p>仅扣除已入账经营与创作可用收益（USD 账本美元）。本页面不调用外部支付；标记已打款仅记录管理员人工核实结果。未知或失败不等于成功，处理中申请禁止自动退回。</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="notice" role="status" class="notice">{{ notice }}</p>
    <form class="policy-form" @submit.prevent="save">
      <label class="toggle"><input v-model="policy.enabled" type="checkbox" :disabled="!loaded || busy" />开放人工提现申请</label>
      <label>收款渠道（每行一个）<textarea v-model="channels" rows="3" maxlength="1620" :disabled="!loaded || busy" /></label>
      <label>币种、兑换与费用说明<textarea v-model="policy.instructions" rows="3" maxlength="2000" :disabled="!loaded || busy" /></label>
      <div class="form-actions"><button class="primary" :disabled="!loaded || busy"><Save :size="16" />保存提现配置</button></div>
    </form>
    <div class="list-heading"><h3>提现申请</h3><button class="icon-button" title="刷新申请" aria-label="刷新申请" :disabled="busy" @click="load()"><RefreshCw :size="16" /></button></div>
    <p v-if="busy && !loaded" role="status">正在读取提现配置与申请…</p>
    <p v-else-if="loaded && !rows.length && !error" class="empty">暂无提现申请</p>
    <ol><li v-for="row in rows" :key="row.id">
      <strong>#{{ row.id }} · 用户 {{ row.owner_id }} · {{ row.amount }} USD</strong>
      <span>{{ withdrawalStatus[row.status] }} · {{ row.channel }} · {{ row.recipient }}</span>
      <span v-if="row.processing_by">处理管理员 #{{ row.processing_by }}（仅此管理员可确认打款）</span>
      <span v-if="row.reference">凭证：{{ row.reference }}</span><span v-if="row.reason">{{ row.reason }}</span>
      <button v-if="row.status === 'pending'" :disabled="busy" @click="select(row, 'processing')">开始人工处理</button>
      <button v-if="row.status === 'pending'" :disabled="busy" @click="select(row, 'rejected')">拒绝并退回</button>
      <button v-if="row.status === 'processing'" :disabled="busy" @click="select(row, 'paid')">核实外部打款</button>
      <button v-if="row.status === 'processing'" :disabled="busy" @click="select(row, 'rejected')">核实未付款并退回</button>
    </li></ol>
    <button v-if="more" :disabled="busy" @click="load(true)">加载更早申请</button>
    <form v-if="selected" class="action-form" @submit.prevent="apply">
      <h3>申请 #{{ selected.id }} · {{ action === 'paid' ? '人工确认已打款' : action === 'rejected' ? '拒绝并退回收益' : '开始人工处理' }}</h3>
      <label>处理原因<input v-model="reason" maxlength="1000" :required="action === 'rejected'" :disabled="busy" /></label>
      <label v-if="selected.status === 'processing'">外部打款凭证 / 未付款核查编号<input v-model="reference" maxlength="200" required :disabled="busy" /></label>
      <label v-if="action === 'paid'"><input v-model="confirmed" type="checkbox" required :disabled="busy" />我已在外部渠道核实打款成功；此操作不会发送款项。</label>
      <label v-if="action === 'rejected' && selected.status === 'processing'"><input v-model="confirmed" type="checkbox" required :disabled="busy" />我已核实外部渠道没有付款且不会再付款，允许退回收益；未知状态不能退回。</label>
      <button :disabled="busy || (selected.status === 'processing' && !confirmed)">确认处理</button>
      <button type="button" :disabled="busy" @click="selected = undefined">关闭</button>
    </form>
  </section>
</template>

<style scoped>
.withdrawal-admin { padding-block:24px; border-block:1px solid var(--bd-ui-line); min-width:0; color:var(--bd-text-primary); }
header,.heading,.list-heading,.form-actions { display:flex; align-items:center; gap:10px; flex-wrap:wrap; }
header,.list-heading { justify-content:space-between; }.heading svg { color:var(--bd-accent-teal); }
h2 { font-size:18px; margin:0; } h3 { font-size:15px; margin:0; } p { font-size:13px; line-height:1.7; color:var(--bd-text-secondary); }
.policy-state { font-size:12px; color:var(--bd-text-secondary); }.policy-state.enabled { color:var(--bd-accent-teal); }
form { display:grid; gap:16px; margin:20px 0; } .policy-form { grid-template-columns:repeat(2,minmax(0,1fr)); padding-bottom:20px; border-bottom:1px solid var(--bd-ui-line); }
label { display:grid; gap:7px; font-size:13px; min-width:0; }.toggle,.form-actions { grid-column:1 / -1; }.toggle { display:flex; align-items:center; gap:8px; }
input:not([type=checkbox]),textarea { display:block; width:100%; min-width:0; box-sizing:border-box; font:inherit; line-height:1.6; } textarea { resize:vertical; }
input,textarea,button { padding:9px 12px; border:1px solid var(--bd-ui-line); border-radius:4px; background:var(--bd-surface); color:inherit; }
input[type=checkbox] { accent-color:var(--bd-accent-teal); width:16px; height:16px; margin:0; }
button { min-height:36px; display:inline-flex; align-items:center; justify-content:center; gap:7px; cursor:pointer; font-size:13px; }
button.primary { background:var(--bd-accent-teal); border-color:var(--bd-accent-teal); color:#fff; }.icon-button { width:36px; padding:0; }
button:disabled { opacity:.5; cursor:not-allowed; } button:focus-visible,input:focus-visible,textarea:focus-visible { outline:2px solid var(--bd-accent-teal); outline-offset:3px; }
ol { padding:0; margin:0; list-style:none; } li { display:flex; flex-wrap:wrap; align-items:center; gap:10px; padding:16px 0; border-bottom:1px solid var(--bd-ui-line); overflow-wrap:anywhere; font-size:13px; }
.empty { padding:20px 0; margin:0; }.notice { color:var(--bd-accent-teal); }.action-form { border-top:1px solid var(--bd-ui-line); padding-top:20px; }
.action-form label:has(input[type=checkbox]) { display:flex; align-items:flex-start; gap:8px; line-height:1.7; }.action-form input[type=checkbox] { flex:0 0 16px; margin-top:3px; }
.action-form>button { justify-self:start; }
[role=alert] { color:var(--bd-accent-red,#b83333); }
@media(max-width:640px) { .policy-form { grid-template-columns:minmax(0,1fr); } }
</style>
