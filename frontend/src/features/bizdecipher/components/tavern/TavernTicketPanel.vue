<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import { Ticket, Undo2 } from '@lucide/vue'
import { useAuthStore } from '@/stores/auth'
import { tavernCommerceAPI, type TavernTicketQuote } from '../../api/tavernCommerce'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
import { joinTavernRoom } from '../../api/bizdecipher'
const props = defineProps<{ roomId: number; status?: string; ownerId?: number }>()
const emit = defineEmits<{ changed: [] }>()
const auth = useAuthStore()
const quote = ref<TavernTicketQuote | null>(null)
const agreed = ref(false)
const busy = ref(false)
const error = ref('')
let epoch = 0
let operation = ''
watch(() => [props.roomId, props.status, auth.user?.id], async () => {
  const version = ++epoch
  quote.value = null; agreed.value = false; busy.value = false; error.value = ''; operation = ''
  try { const q = await tavernCommerceAPI.quote(props.roomId); if (version === epoch) quote.value = q }
  catch (e) { if (version === epoch) error.value = extractActionableApiErrorMessage(e, '入场信息读取失败') }
}, { immediate: true })
onBeforeUnmount(() => { ++epoch })
async function act(refund: boolean) {
  if (!quote.value || busy.value || !refund && (!quote.value.enabled || !agreed.value)) return
  const version = epoch
  busy.value = true; error.value = ''
  try {
    if (!operation) {
      const bytes = globalThis.crypto.getRandomValues(new Uint8Array(20))
      operation = `tavern_${Array.from(bytes, x => x.toString(16).padStart(2, '0')).join('')}`
    }
    const t = refund ? await tavernCommerceAPI.refund(props.roomId, quote.value.ticket!.id) : await tavernCommerceAPI.buy(props.roomId, quote.value.price, operation)
    if (version !== epoch) return
    quote.value.ticket = t; agreed.value = false; if (refund) operation = ''; emit('changed')
  } catch (e) { if (version === epoch) error.value = extractActionableApiErrorMessage(e, '入场操作未完成，可重试') }
  finally { if (version === epoch) busy.value = false }
}
async function joinFree() {
  if (busy.value || !quote.value || quote.value.author_user_id) return
  const version = epoch
  busy.value = true; error.value = ''
  try { await joinTavernRoom(props.roomId); if (version === epoch) emit('changed') }
  catch (e) { if (version === epoch) error.value = extractActionableApiErrorMessage(e, '加入失败') }
  finally { if (version === epoch) busy.value = false }
}
</script>
<template>
  <section class="ticket-panel" aria-label="游戏入场券">
    <p v-if="error" role="alert">{{ error }}</p>
    <template v-if="quote">
      <strong>{{ quote.title }} · #{{ roomId }}</strong>
      <template v-if="!quote.author_user_id"><p>此房间未设置余额入场券。</p><button v-if="quote.status === 'lobby'" :disabled="busy" type="button" @click="joinFree">加入房间</button></template>
      <template v-else>
      <strong><Ticket :size="16" />USD {{ quote.price }} / 人</strong>
      <p v-if="quote.owner_id === auth.user?.id">房主免入场费。开局后入场费进入剧本作者收益钱包，模型调用费用另行授权。</p>
      <p v-else-if="quote.author_user_id === auth.user?.id">这是你的作品。可自行开房担任房主，不向自己购买入场券。</p>
      <template v-else-if="quote.ticket">
        <p>{{ { held: '已购票 · 等待开局', released: '已开局 · 作者收益已入账', refunded: '已退款 · 已退出房间' }[quote.ticket.status] }}</p>
        <button v-if="quote.ticket.status === 'held' && ['draft', 'lobby'].includes(quote.status)" type="button" :disabled="busy" @click="act(true)"><Undo2 :size="15" />退票并退出</button>
      </template>
      <form v-if="quote.owner_id !== auth.user?.id && quote.author_user_id !== auth.user?.id && (!quote.ticket || quote.ticket.status === 'refunded') && quote.status === 'lobby'" @submit.prevent="act(false)">
        <label><input v-model="agreed" type="checkbox" :disabled="!quote.enabled || busy" />确认支付 USD {{ quote.price }}，未开局可退，开局后结算给作者。</label>
        <button :disabled="!quote.enabled || !agreed || busy"><Ticket :size="15" />购票并加入</button>
        <p v-if="!quote.enabled">平台暂未开放付费入场。</p>
      </form>
      </template>
    </template>
  </section>
</template>
<style scoped>
.ticket-panel{display:grid;gap:8px;padding:12px 0;border-top:1px solid var(--zc-line,#ddd);font-size:.75rem}.ticket-panel p{margin:0;color:var(--zc-muted,#666);line-height:1.6}.ticket-panel strong,.ticket-panel button{display:flex;gap:6px;align-items:center}.ticket-panel form{display:grid;gap:8px}.ticket-panel button{justify-self:start;padding:7px 10px;border:1px solid var(--zc-line,#ddd);border-radius:6px;background:var(--zc-surface,#fff);color:var(--zc-text,#222)}.ticket-panel button:disabled{opacity:.5}.ticket-panel [role=alert]{color:var(--zc-danger,#c22)}
</style>
