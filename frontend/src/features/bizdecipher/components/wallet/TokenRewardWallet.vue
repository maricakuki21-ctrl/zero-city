<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { availableReward, tokenRewardsAPI, type RewardWallet } from '../../api/tokenRewards'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
const auth = useAuthStore()
const wallet = ref<RewardWallet>()
const error = ref('')
const loading = ref(false)
let epoch = 0
const total = computed(() => wallet.value?.grants.reduce((sum, g) => sum + availableReward(g, Date.parse(wallet.value!.server_time)), 0) || 0)
const labels = { pending: '处理中', review: '待核对', succeeded: '已结算', failed: '已退还预留' }
async function refresh() {
  const generation = ++epoch
  loading.value = true
  try {
    const result = await tokenRewardsAPI.wallet()
    if (generation === epoch) { wallet.value = result; error.value = '' }
  } catch (e) { if (generation === epoch) error.value = extractActionableApiErrorMessage(e, '奖励额度加载失败') }
  finally { if (generation === epoch) loading.value = false }
}
watch(() => auth.user?.id, () => { ++epoch; wallet.value = undefined; error.value = ''; if (auth.user?.id) void refresh() }, { immediate: true })
onBeforeUnmount(() => { epoch++ })
</script>
<template>
  <section class="reward-wallet" aria-label="Token 奖励额度">
    <header><div><small>灵感补给</small><h2>奖励额度</h2></div><button type="button" :disabled="loading" @click="refresh">刷新</button></header>
    <strong>{{ total.toLocaleString() }} <small>Token 可用</small></strong>
    <p>来自聊天红包。与充值余额分开；仅用于指定文本模型，不可提现或转赠。</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-else-if="wallet && !wallet.grants.length">还没有奖励，去零号城聊天看看下一场灵感补给。</p>
    <article v-for="grant in wallet?.grants" :key="grant.id">
      <div><b>{{ grant.model }}</b><span>获得 {{ grant.tokens.toLocaleString() }} · 已用 {{ grant.used.toLocaleString() }} · 处理中 {{ grant.reserved.toLocaleString() }}</span><small>有效至 {{ new Date(grant.expires_at).toLocaleString() }}</small></div>
      <RouterLink v-if="availableReward(grant, Date.parse(wallet!.server_time)) > 0" :to="{ path: '/workbench', query: { reward: grant.resource_id } }">用奖励创作</RouterLink>
      <span v-else>{{ Date.parse(grant.expires_at) <= Date.parse(wallet!.server_time) ? '已到期' : '暂无可用额度' }}</span>
    </article>
    <details v-if="wallet?.runs.length"><summary>最近调用与结算</summary><article v-for="run in wallet.runs" :key="run.id"><div><b>#{{ run.id }} · {{ labels[run.status] }}</b><span>{{ run.model }} · 实际 {{ run.actual }} / 奖励抵扣 {{ run.covered }} Token</span><small>{{ run.note }}</small><pre v-if="run.result">{{ run.result }}</pre></div></article></details>
  </section>
</template>
<style scoped>
.reward-wallet{border:1px solid #8883;border-radius:16px;padding:22px;margin:20px 0;background:var(--bg-primary,transparent)}header,article{display:flex;justify-content:space-between;gap:16px;align-items:center}h2{font-size:18px;margin:4px 0 18px}strong{font-size:28px}strong small,p,small{font-size:12px;opacity:.8}p{line-height:1.7}article{border-top:1px solid #8882;padding:14px 0;font-size:13px}article div{display:grid;gap:5px;min-width:0}a,button{color:inherit;border:1px solid #8884;padding:7px 10px;border-radius:8px;white-space:nowrap}pre{white-space:pre-wrap;overflow-wrap:anywhere;max-height:220px;overflow:auto}summary{cursor:pointer;padding:12px 0}
</style>
