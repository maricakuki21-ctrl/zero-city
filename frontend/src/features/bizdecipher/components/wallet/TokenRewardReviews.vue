<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { tokenRewardsAPI, type RewardReview } from '../../api/tokenRewards'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
const reviews = ref<RewardReview[]>([])
const selected = ref<RewardReview>()
const actual = ref<number>()
const evidence = ref('')
const error = ref('')
const busy = ref(false)
async function load() {
  try { reviews.value = await tokenRewardsAPI.reviews() }
  catch (e) { error.value = extractActionableApiErrorMessage(e, '奖励核对队列加载失败') }
}
function select(item: RewardReview) { selected.value = item; actual.value = undefined; evidence.value = ''; error.value = '' }
async function resolve() {
  if (!selected.value || actual.value === undefined || busy.value) return
  busy.value = true
  try { await tokenRewardsAPI.resolve(selected.value.id, actual.value, evidence.value); selected.value = undefined; await load() }
  catch (e) { error.value = extractActionableApiErrorMessage(e, '核对提交失败') }
  finally { busy.value = false }
}
onMounted(load)
</script>
<template>
  <details class="reward-reviews">
    <summary>运营：奖励用量核对 <b v-if="reviews.length">{{ reviews.length }}</b></summary>
    <p>只处理结果未知的调用。先用追踪编号核查网关及供应商记录；没有日志不等于零消耗。</p>
    <button type="button" @click="load">刷新待核对</button>
    <article v-for="item in reviews" :key="item.id"><span>#{{ item.id }} · 用户 {{ item.user_id }} · {{ item.model }} · 预留 {{ item.reserved }}</span><code>{{ item.gateway_request_id || `token-reward-${item.id}` }}</code><button type="button" @click="select(item)">核对</button></article>
    <form v-if="selected" @submit.prevent="resolve"><strong>核对调用 #{{ selected.id }}</strong><label>已核实的实际 Token<input v-model.number="actual" type="number" min="0" max="2000000000" required /></label><label>核对依据（供应商记录/追踪编号及结论）<textarea v-model="evidence" minlength="10" maxlength="2000" required /></label><button type="submit" :disabled="busy">{{ busy ? '提交中…' : '确认结算并留档' }}</button></form>
    <p v-if="error" role="alert">{{ error }}</p>
  </details>
</template>
<style scoped>
.reward-reviews{padding:18px;border:1px solid #8883;border-radius:12px;margin:16px 0;font-size:13px}summary{cursor:pointer}p{line-height:1.6;opacity:.8}article{display:grid;gap:8px;border-top:1px solid #8883;padding:14px 0}code{overflow-wrap:anywhere}form,label{display:grid;gap:10px;margin:14px 0}input,textarea,button{padding:8px;border:1px solid #8885;border-radius:6px;color:inherit;background:transparent}b{background:#a33449;color:white;border-radius:10px;padding:1px 6px}
</style>
