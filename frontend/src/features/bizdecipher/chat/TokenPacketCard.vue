<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { tokenRewardsAPI, type TokenPacket } from '../api/tokenRewards'
import { useAuthStore } from '@/stores/auth'
import { zeroCityMascots } from '@/constants/zeroCityMascots'
import { extractActionableApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ packetId: number }>()
const auth = useAuthStore()
const packet = ref<TokenPacket>()
const error = ref('')
const busy = ref(false)
const now = ref(Date.now())
let offset = 0
let alive = true
let timer: ReturnType<typeof setInterval> | undefined
let tick = 0
let loading = false
const state = computed(() => {
  const p = packet.value
  if (!p) return 'loading'
  if (p.mine) return 'claimed'
  if (now.value < Date.parse(p.opens_at)) return 'waiting'
  if (now.value >= Date.parse(p.closes_at)) return 'expired'
  if (p.claimed >= p.portions) return 'empty'
  if (p.sender_id === auth.user?.id) return 'own'
  return 'ready'
})
const label = computed(() => ({
  loading: '加载红包', claimed: '已领取', waiting: `${Math.max(0, Math.ceil((Date.parse(packet.value?.opens_at || '') - now.value) / 1000))} 秒后开抢`,
  expired: '已过期', empty: '已抢完', own: '你发出的红包', ready: '抢红包',
})[state.value])
async function refresh() {
  if (loading) return
  loading = true
  const userId = auth.user?.id
  try {
    const p = await tokenRewardsAPI.get(props.packetId)
    if (!alive || auth.user?.id !== userId) return
    packet.value = p
    offset = Date.parse(p.server_time) - Date.now()
    now.value = Date.now() + offset
    error.value = ''
  } catch (e) {
    if (alive && auth.user?.id === userId) error.value = extractActionableApiErrorMessage(e, '红包加载失败')
  } finally { loading = false }
}
async function claim() {
  if (busy.value || state.value !== 'ready') return
  busy.value = true
  const userId = auth.user?.id
  try {
    const grant = await tokenRewardsAPI.claim(props.packetId)
    if (!alive || auth.user?.id !== userId) return
    if (packet.value) packet.value.mine = grant
    await refresh()
  } catch (e) {
    if (alive && auth.user?.id === userId) error.value = extractActionableApiErrorMessage(e, '领取失败，请刷新红包状态')
  } finally { busy.value = false }
}
onMounted(() => {
  void refresh()
  timer = setInterval(() => {
    if (document.visibilityState === 'hidden') return
    now.value = Date.now() + offset
    if (++tick % 5 === 0 && !['expired', 'empty'].includes(state.value)) void refresh()
  }, 1000)
})
onBeforeUnmount(() => { alive = false; if (timer) clearInterval(timer) })
</script>

<template>
  <section class="token-packet" aria-label="Token 红包">
    <img :src="zeroCityMascots.communityAnnouncer" alt="" class="token-packet__mascot" />
    <div class="token-packet__content">
      <small>零号城 · 灵感补给</small>
      <h3>{{ packet?.blessing || '有人请你，让灵感继续。' }}</h3>
      <strong>{{ packet ? packet.total_tokens.toLocaleString() : '…' }} <small>Token</small></strong>
      <p v-if="packet">{{ packet.portions }} 份 · {{ packet.mode === 'random' ? '拼手气' : '等额' }} · 已领 {{ packet.claimed }}</p>
      <p v-if="packet" class="token-packet__terms">{{ packet.model }} · 领取后 {{ packet.use_hours }} 小时可用<br />免费奖励，不可提现，不扣充值余额</p>
      <p v-if="packet?.mine">你抢到 <b>{{ packet.mine.tokens.toLocaleString() }}</b> Token</p>
      <RouterLink v-if="packet?.mine" class="token-packet__button" :to="{ path: '/workbench', query: { reward: packet.resource_id } }">用奖励开始创作</RouterLink>
      <button v-else type="button" class="token-packet__button" :disabled="state !== 'ready' || busy" @click="claim">{{ busy ? '领取中…' : label }}</button>
      <p v-if="error" role="alert">{{ error }} <button type="button" @click="refresh">刷新</button></p>
    </div>
  </section>
</template>

<style scoped>
.token-packet{position:relative;isolation:isolate;overflow:hidden;width:min(290px,100%);border-radius:16px;background:linear-gradient(140deg,#68263d,#a23447 60%,#c66548);color:#fff4e9;border:1px solid #dfb78b55}
.token-packet__mascot{position:absolute;width:165px;right:-38px;top:-9px;opacity:.2;z-index:-1;mask-image:linear-gradient(#000,transparent)}
.token-packet__content{position:relative;padding:18px}
.token-packet small{font-size:11px;letter-spacing:.04em;opacity:.87}
.token-packet h3{font-size:17px;line-height:1.45;margin:9px 0 16px;max-width:220px}
.token-packet strong{font-size:25px;font-variant-numeric:tabular-nums}
.token-packet p{font-size:12px;line-height:1.65;margin:7px 0}
.token-packet__terms{opacity:.88;overflow-wrap:anywhere}
.token-packet__button{display:block;width:100%;text-align:center;border:0;border-radius:9px;padding:10px;background:#ffdfad;color:#653225;font-weight:700;margin-top:13px;text-decoration:none}
.token-packet__button:disabled{opacity:.68;cursor:default}
.token-packet button:focus-visible,.token-packet a:focus-visible{outline:2px solid white;outline-offset:3px}
</style>
