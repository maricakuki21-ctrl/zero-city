<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { availableReward, newRewardRequestID, tokenRewardsAPI, type RewardResource, type RewardRun, type RewardWallet } from '../api/tokenRewards'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
const props = defineProps<{ resourceId: string }>()
const auth = useAuthStore()
const prompt = ref('')
const maxOutput = ref(1024)
const resources = ref<RewardResource[]>([])
const wallet = ref<RewardWallet>()
const result = ref<RewardRun>()
const error = ref('')
const busy = ref(false)
let generation = 0
const attempt = ref<{ client_id: string; resource_id: string; prompt: string; max_output_tokens: number }>()
const resource = computed(() => resources.value.find(r => r.id === props.resourceId))
const available = computed(() => wallet.value?.grants.filter(g => g.resource_id === props.resourceId).reduce((sum, g) => sum + availableReward(g, Date.parse(wallet.value!.server_time)), 0) || 0)
async function load() {
  const epoch = generation
  try {
    const [rs, w] = await Promise.all([tokenRewardsAPI.resources(), tokenRewardsAPI.wallet()])
    if (epoch !== generation) return
    resources.value = rs; wallet.value = w
    if (attempt.value) result.value = w.runs.find(r => r.client_id === attempt.value?.client_id) || result.value
  } catch (e) { if (epoch === generation) error.value = extractActionableApiErrorMessage(e, '奖励资源加载失败') }
}
watch(() => [auth.user?.id, props.resourceId], () => {
  generation++; attempt.value = undefined; prompt.value = ''; result.value = undefined; wallet.value = undefined; error.value = ''; busy.value = false
  void load()
}, { immediate: true })
async function run() {
  if (busy.value || !resource.value || !prompt.value.trim()) return
  const epoch = generation
  attempt.value ??= { client_id: newRewardRequestID(), resource_id: props.resourceId, prompt: prompt.value, max_output_tokens: maxOutput.value }
  busy.value = true; error.value = ''
  try {
    const value = await tokenRewardsAPI.run(attempt.value)
    if (epoch !== generation) return
    result.value = value
    await load()
  } catch (e) {
    if (epoch === generation) {
      error.value = extractActionableApiErrorMessage(e, '结果未确认，请刷新记录；重试会核对同一次调用，不会再次扣费')
      const status = (e as { status?: number })?.status
      if (status === 400 || status === 401 || status === 403) attempt.value = undefined
    }
  } finally { if (epoch === generation) busy.value = false }
}
function newRun() {
  if (!result.value || !['succeeded', 'failed'].includes(result.value.status)) return
  attempt.value = undefined; result.value = undefined; error.value = ''; prompt.value = ''
}
onBeforeUnmount(() => { generation++ })
</script>
<template>
  <section class="reward-studio">
    <header><div><small>零号城 · 灵感补给</small><h1>让这份灵感，变成你的作品。</h1><p>仅使用红包奖励，不扣充值余额。当前支持单轮文本创作。</p></div><RouterLink to="/workbench">返回普通工作台</RouterLink></header>
    <div class="reward-studio__meta"><b>{{ resource?.label || '奖励资源' }} · {{ resource?.model || resourceId }}</b><span>{{ available.toLocaleString() }} Token 可用</span><RouterLink to="/wallet">查看奖励账本</RouterLink></div>
    <form @submit.prevent="run">
      <label for="reward-prompt">想用这份灵感做什么？</label>
      <textarea id="reward-prompt" v-model="prompt" rows="8" maxlength="4000" :disabled="busy || !!attempt" placeholder="写一段故事、推敲一个想法，或解释一段代码……" />
      <label>最大输出 Token <input v-model.number="maxOutput" type="number" min="1" max="2048" :disabled="busy || !!attempt" /></label>
      <p>开始时只预留赠送额度，完成后按真实输入＋输出 Token 结算。余额为零也可使用。</p>
      <button type="submit" :disabled="busy || !resource || !prompt.trim() || !!result">{{ busy ? '正在创作…' : attempt ? '核对同一次调用' : '仅用奖励生成' }}</button>
      <button type="button" @click="load">刷新调用记录</button>
      <button v-if="result && ['succeeded', 'failed'].includes(result.status)" type="button" @click="newRun">开始新创作</button>
    </form>
    <p v-if="error" role="alert">{{ error }}</p>
    <article v-if="result"><h2>{{ result.status === 'succeeded' ? '创作完成' : result.status === 'failed' ? '本次未完成，预留已退还' : '调用结果待核对' }}</h2><p>{{ result.note }}</p><pre>{{ result.result }}</pre><p>实际用量 {{ result.actual }} · 奖励抵扣 {{ result.covered }} Token · 你的充值余额不变</p></article>
  </section>
</template>
<style scoped>
.reward-studio{max-width:980px;margin:auto;padding:clamp(20px,4vw,48px);color:var(--text-primary,inherit)}header{display:flex;justify-content:space-between;gap:24px;align-items:start}h1{font-size:clamp(24px,3vw,34px);margin:12px 0}p{font-size:13px;line-height:1.7;opacity:.75}.reward-studio__meta{display:flex;gap:16px;flex-wrap:wrap;margin:30px 0;padding:18px;border:1px solid #8883;border-radius:12px}label{display:block;font-size:14px;margin:15px 0}textarea{width:100%;resize:vertical;background:transparent;border:1px solid #8885;border-radius:16px;padding:20px;color:inherit}input{width:100px;background:transparent;color:inherit;border:1px solid #8885;border-radius:6px;padding:6px}button{background:var(--bg-secondary,transparent);border:1px solid #8884;border-radius:9px;padding:10px 16px;margin:8px 8px 0 0}button[type=submit]{background:#9b384e;color:#fff}button:disabled{opacity:.5}a{color:inherit}pre{white-space:pre-wrap;overflow-wrap:anywhere;font-family:inherit;line-height:1.8}article{border-top:1px solid #8883;margin-top:28px;padding-top:24px}@media(max-width:600px){header{display:block}}
</style>
