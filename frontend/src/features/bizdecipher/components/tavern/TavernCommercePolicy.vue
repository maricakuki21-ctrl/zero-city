<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import { Save } from '@lucide/vue'
import { useAuthStore } from '@/stores/auth'
import { tavernCommerceAPI } from '../../api/tavernCommerce'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
const auth = useAuthStore()
const enabled = ref(false)
const ready = ref(false)
const busy = ref(false)
const reason = ref('')
const error = ref('')
let epoch = 0
watch(() => auth.user?.id, async () => {
  const version = ++epoch; ready.value = false; busy.value = false; reason.value = ''; error.value = ''
  try { const p = await tavernCommerceAPI.policy(); if (version === epoch) { enabled.value = p.enabled; ready.value = true } }
  catch (e) { if (version === epoch) error.value = extractActionableApiErrorMessage(e, '收费设置读取失败') }
}, { immediate: true })
onBeforeUnmount(() => { ++epoch })
async function save() {
  if (!ready.value || busy.value || !reason.value.trim()) return
  const version = epoch; busy.value = true; error.value = ''
  try { await tavernCommerceAPI.setPolicy(enabled.value, reason.value); if (version === epoch) reason.value = '' }
  catch (e) { if (version === epoch) error.value = extractActionableApiErrorMessage(e, '收费设置未保存') }
  finally { if (version === epoch) busy.value = false }
}
</script>
<template>
  <form class="tavern-policy" aria-label="酒馆收费设置" @submit.prevent="save">
    <label><input v-model="enabled" type="checkbox" :disabled="!ready || busy" />开放 USD 入场券</label>
    <p>关闭不影响已购票房间的开局结算和未开局退款。</p>
    <input v-model="reason" required maxlength="1000" placeholder="设置变更原因" aria-label="设置变更原因" :disabled="!ready || busy" />
    <button :disabled="!ready || busy || !reason.trim()"><Save :size="15" />保存设置</button>
    <p v-if="error" role="alert">{{ error }}</p>
  </form>
</template>
<style scoped>
.tavern-policy{display:grid;gap:8px;padding:12px 0;border-block:1px solid var(--zc-line,#ddd);font-size:.75rem}.tavern-policy p{margin:0;color:var(--zc-muted,#666)}.tavern-policy>input{padding:8px;border:1px solid var(--zc-line,#ddd);border-radius:5px;background:var(--zc-surface,#fff);color:var(--zc-text,#222)}.tavern-policy button{display:flex;gap:6px;align-items:center;justify-self:start;padding:7px 10px;border:1px solid var(--zc-line,#ddd);border-radius:5px;background:var(--zc-surface,#fff);color:var(--zc-text,#222)}.tavern-policy [role=alert]{color:var(--zc-danger,#c22)}
</style>
