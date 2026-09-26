<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { Save } from '@lucide/vue'
import { useAuthStore } from '@/stores/auth'
import { marketplaceFundsAPI } from '../../api/marketplaceFunds'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
const auth = useAuthStore()
const enabled = ref(false)
const current = ref(false)
const ready = ref(false)
const reason = ref('')
const saving = ref(false)
const error = ref('')
let epoch = 0
watch(() => auth.user?.id, async () => {
  const request = ++epoch
  ready.value = false; saving.value = false; error.value = ''; reason.value = ''
  try {
    const p = await marketplaceFundsAPI.policy()
    if (request !== epoch) return
    enabled.value = current.value = p.enabled; ready.value = true
  } catch (e) { if (request === epoch) error.value = extractActionableApiErrorMessage(e, '收款设置读取失败') }
}, { immediate: true })
onBeforeUnmount(() => { ++epoch })
async function save() {
  if (!ready.value || saving.value || !reason.value.trim()) return
  const request = epoch
  saving.value = true; error.value = ''
  try {
    const p = await marketplaceFundsAPI.setPolicy(enabled.value, reason.value)
    if (request !== epoch) return
    enabled.value = current.value = p.enabled; reason.value = ''
  } catch (e) { if (request === epoch) error.value = extractActionableApiErrorMessage(e, '收款设置未保存') }
  finally { if (request === epoch) saving.value = false }
}
</script>
<template>
  <form class="funds-policy" aria-label="市场收款设置" @submit.prevent="save">
    <label><input v-model="enabled" type="checkbox" :disabled="!ready || saving" />开放 USD 站内余额付款 <small>当前{{ current ? '开放' : '关闭' }}</small></label>
    <p>关闭后不接受新付款，已付款订单仍可交付、验收和退款。</p>
    <div><input v-model="reason" required maxlength="1000" placeholder="设置变更原因" aria-label="设置变更原因" :disabled="!ready || saving" /><button :disabled="!ready || saving || !reason.trim()"><Save :size="15" />保存设置</button></div>
    <p v-if="error" role="alert">{{ error }}</p>
  </form>
</template>
<style scoped>
.funds-policy{display:grid;gap:9px;border-block:1px solid var(--zc-line);padding:12px 0}.funds-policy label{font-size:.8125rem}.funds-policy small,.funds-policy p{font-size:.75rem;color:var(--zc-muted);margin:0}.funds-policy div{display:flex;gap:8px;flex-wrap:wrap}.funds-policy div input{flex:1;min-width:140px;padding:8px;border:1px solid var(--zc-line);border-radius:5px;background:var(--zc-bg);color:var(--zc-text)}.funds-policy button{display:flex;align-items:center;gap:6px;padding:8px;border:1px solid var(--zc-line);border-radius:5px;background:var(--zc-surface);color:var(--zc-text)}.funds-policy [role=alert]{color:var(--zc-danger)}.funds-policy button:disabled{opacity:.5}
</style>
