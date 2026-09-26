<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import { Save } from '@lucide/vue'
import { useAuthStore } from '@/stores/auth'
import { tavernCommerceAPI } from '../../api/tavernCommerce'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
const props = defineProps<{ scriptId: number; initialPrice: string }>()
const emit = defineEmits<{ changed: [] }>()
const auth = useAuthStore()
const price = ref('0')
const busy = ref(false)
const error = ref('')
let epoch = 0
watch(() => [props.scriptId, props.initialPrice, auth.user?.id], () => { ++epoch; price.value = props.initialPrice || '0'; busy.value = false; error.value = '' }, { immediate: true })
onBeforeUnmount(() => { ++epoch })
async function save() {
  if (busy.value) return
  const version = epoch
  busy.value = true; error.value = ''
  try { await tavernCommerceAPI.price(props.scriptId, price.value); if (version === epoch) emit('changed') }
  catch (e) { if (version === epoch) error.value = extractActionableApiErrorMessage(e, '入场价未保存') }
  finally { if (version === epoch) busy.value = false }
}
</script>
<template>
  <form class="script-price" aria-label="剧本入场定价" @submit.prevent="save">
    <label>每人入场价 · USD<input v-model="price" required inputmode="decimal" pattern="(0|[1-9][0-9]{0,11})(\.[0-9]{1,8})?" :disabled="busy" /></label>
    <p>0 为免费，只影响新建房间。收费另需平台开放；开局后全额进入作者收益钱包。</p>
    <button :disabled="busy"><Save :size="15" />保存入场价</button>
    <p v-if="error" role="alert">{{ error }}</p>
  </form>
</template>
<style scoped>
.script-price{display:grid;gap:8px;margin:12px 0;padding:12px 0;border-block:1px solid var(--zc-line,#ddd);font-size:.75rem}.script-price input{display:block;width:180px;max-width:100%;padding:8px;margin-top:5px;border:1px solid var(--zc-line,#ddd);border-radius:5px;background:var(--zc-surface,#fff);color:var(--zc-text,#222)}.script-price p{margin:0;color:var(--zc-muted,#666)}.script-price button{display:flex;align-items:center;gap:6px;justify-self:start;padding:7px 10px;border:1px solid var(--zc-line,#ddd);border-radius:5px;background:var(--zc-surface,#fff);color:var(--zc-text,#222)}.script-price [role=alert]{color:var(--zc-danger,#c22)}
</style>
