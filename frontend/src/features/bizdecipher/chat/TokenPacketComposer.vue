<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { newRewardRequestID, tokenRewardsAPI, type RewardResource, type TokenPacketInput } from '../api/tokenRewards'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
const props = defineProps<{ channel: string }>()
const emit = defineEmits<{ sent: []; close: [] }>()
const resources = ref<RewardResource[]>([])
const busy = ref(false)
const error = ref('')
const input = ref<TokenPacketInput>({
  client_id: newRewardRequestID(), resource_id: '', total_tokens: 100000000, portions: 64,
  mode: 'random', blessing: '今晚的灵感，我请了。', delay_seconds: 60, claim_hours: 24, use_hours: 168,
})
const attempt = ref<TokenPacketInput>()
onMounted(async () => {
  try { resources.value = await tokenRewardsAPI.resources(); input.value.resource_id = resources.value[0]?.id || '' }
  catch (e) { error.value = extractActionableApiErrorMessage(e, '奖励资源加载失败') }
})
async function send() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  // Preserve the exact request on uncertain transport failure. Never mint a
  // second giveaway simply because the first response was lost.
  attempt.value ??= { ...input.value }
  try { await tokenRewardsAPI.create(props.channel, attempt.value); emit('sent') }
  catch (e) {
    error.value = extractActionableApiErrorMessage(e, '发布结果未确认，再次发送将核对同一个红包')
    // Validation/auth errors happen before creation. Let the operator correct
    // their form; network and server failures keep the same immutable attempt.
    const status = (e as { status?: number })?.status
    if (status === 400 || status === 401 || status === 403) attempt.value = undefined
  }
  finally { busy.value = false }
}
</script>
<template>
  <form class="packet-composer" @submit.prevent="send">
    <header><strong>发一份灵感补给</strong><button type="button" :disabled="busy" @click="emit('close')">关闭</button></header>
    <p>总量分成多份，不是每人领取总量。费用由运营方承担。</p>
    <fieldset :disabled="busy || !!attempt">
      <label>官方奖励资源<select v-model="input.resource_id" required><option v-for="r in resources" :key="r.id" :value="r.id">{{ r.label }} · {{ r.model }}</option></select></label>
      <p v-if="!resources.length">尚未配置并验收奖励资源，暂不能发放。</p>
      <div class="packet-composer__grid">
        <label>总 Token<input v-model.number="input.total_tokens" type="number" min="1" max="1000000000" required /></label>
        <label>份数<input v-model.number="input.portions" type="number" min="1" max="500" required /></label>
        <label>分配<select v-model="input.mode"><option value="random">拼手气</option><option value="equal">等额</option></select></label>
        <label>倒计时（秒）<input v-model.number="input.delay_seconds" type="number" min="0" max="3600" /></label>
        <label>领取窗口（小时）<input v-model.number="input.claim_hours" type="number" min="1" max="168" /></label>
        <label>领取后有效（小时）<input v-model.number="input.use_hours" type="number" min="1" max="720" /></label>
      </div>
      <label>祝福语<input v-model="input.blessing" maxlength="160" required /></label>
      <p v-if="input.mode === 'random'">随机份额约为平均值的 0.5～1.5 倍，总量守恒。不可提现或转赠。</p>
    </fieldset>
    <p v-if="error" role="alert">{{ error }}</p>
    <button type="submit" :disabled="busy || !input.resource_id">{{ busy ? '发布中…' : attempt ? '核对并重试同一红包' : '确认发放' }}</button>
  </form>
</template>
<style scoped>
.packet-composer{padding:16px;border-block:1px solid var(--border-color,#d8d9dc);max-height:55vh;overflow:auto;font-size:12px}
header{display:flex;justify-content:space-between;align-items:center}fieldset{border:0;padding:0;margin:12px 0}
label{display:grid;gap:5px;margin-bottom:10px}input,select{width:100%;min-width:0;border:1px solid #8885;border-radius:7px;background:transparent;color:inherit;padding:7px}.packet-composer__grid{display:grid;grid-template-columns:1fr 1fr;gap:10px}
button{padding:7px 12px;border:1px solid #8885;border-radius:8px}p{line-height:1.6;opacity:.8}
</style>
