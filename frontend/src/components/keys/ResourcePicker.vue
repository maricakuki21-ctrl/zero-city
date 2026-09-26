<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Check, Search, RefreshCw, Server, Users } from '@lucide/vue'
import { getAvailable } from '@/api/groups'
import {
  getSharedPool, joinSharedPool, listMySeats, listSharedPools, listSharedPoolModelPricing,
  type PoolSeat, type SharedPool, type SharedPoolModelEndpointPricing,
} from '@/features/bizdecipher/api/bizdecipher'
import { sharedPoolEndpointAvailability, sharedPoolEndpointShortLabel } from '@/features/bizdecipher/components/shared-pool/sharedPoolPricing'
import type { Group } from '@/types'
import type { ResourceChoice } from './resourceChoice'
import { extractActionableApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ modelValue: ResourceChoice | null; disabled?: boolean }>()
const emit = defineEmits<{
  'update:modelValue': [choice: ResourceChoice | null]
  busy: [value: boolean]
}>()
const query = ref('')
const groups = ref<Group[]>([])
const pools = ref<SharedPool[]>([])
const seats = ref<PoolSeat[]>([])
const total = ref(0)
const loading = ref(false)
const selecting = ref(false)
const joining = ref(false)
const error = ref('')
const actionError = ref('')
const selectedPool = ref<SharedPool | null>(null)
const pricing = ref<SharedPoolModelEndpointPricing[]>([])
const pricingError = ref('')
const joinedHere = ref(false)
const agreed = ref(false)
let loadSequence = 0
let selectionSequence = 0
let timer: ReturnType<typeof setTimeout> | undefined
const locked = computed(() => Boolean(props.disabled || selecting.value || joining.value))
const matching = (text: string) => text.toLocaleLowerCase().includes(query.value.trim().toLocaleLowerCase())
const activeSeats = computed(() => seats.value.filter(s => s.status === 'active'))
const visibleGroups = computed(() => groups.value.filter(g => matching(`${g.name} ${g.platform} ${g.description || ''}`)))
const visiblePools = computed(() => {
  const publicRows = pools.value.map(p => ({ id: p.id, name: p.name, detail: p.models?.join(' · ') || p.owner_label }))
  const known = new Set(publicRows.map(p => p.id))
  return [...activeSeats.value.filter(s => !known.has(s.pool_id) && matching(s.pool_name))
    .map(s => ({ id: s.pool_id, name: s.pool_name, detail: '已加入' })), ...publicRows]
})
const hasSeat = (id: number) => activeSeats.value.some(s => s.pool_id === id)
const isSelected = (kind: ResourceChoice['kind'], id: number) => props.modelValue?.kind === kind && props.modelValue.id === id
const available = computed(() => {
  const p = selectedPool.value
  return Boolean(p && !p.owner_paused && p.listed !== false &&
    ['active', 'healthy', 'limited'].includes(p.status) &&
    (!p.native_onboarding_state || ['legacy_existing', 'billing_active'].includes(p.native_onboarding_state)))
})
const joinable = computed(() => available.value && selectedPool.value &&
  (selectedPool.value.max_users <= 0 || selectedPool.value.current_users < selectedPool.value.max_users))
const enabledPricing = computed(() => pricing.value.filter(item => item.enabled))
const money = (n: number) => `$${Number(n || 0).toFixed(6).replace(/0+$/, '').replace(/\.$/, '')}`

async function load() {
  const seq = ++loadSequence
  loading.value = true
  error.value = ''
  const results = await Promise.allSettled([
    getAvailable(), listSharedPools({ keyword: query.value.trim(), limit: 100 }), listMySeats(),
  ])
  if (seq !== loadSequence) return
  const [official, shared, memberships] = results
  groups.value = official.status === 'fulfilled' ? official.value : []
  pools.value = shared.status === 'fulfilled' ? shared.value.pools ?? [] : []
  total.value = shared.status === 'fulfilled' ? shared.value.total : 0
  seats.value = memberships.status === 'fulfilled' ? memberships.value : []
  error.value = [
    official.status === 'rejected' ? '官方资源读取失败' : '',
    shared.status === 'rejected' ? '共享资源读取失败' : '',
    memberships.status === 'rejected' ? '加入状态读取失败，请刷新后再选择共享资源' : '',
  ].filter(Boolean).join('；')
  loading.value = false
}
function selectGroup(group: Group) {
  selectionSequence++
  selectedPool.value = null
  pricing.value = []
  pricingError.value = ''
  joinedHere.value = false
  actionError.value = ''
  emit('update:modelValue', { kind: 'official', id: group.id, name: group.name, ready: true })
}
async function selectPool(id: number, name: string) {
  const seq = ++selectionSequence
  selectedPool.value = null
  pricing.value = []
  pricingError.value = ''
  actionError.value = ''
  agreed.value = false
  joinedHere.value = false
  emit('update:modelValue', { kind: 'shared', id, name, ready: false })
  selecting.value = true
  emit('busy', true)
  try {
    const [pool, memberships, prices] = await Promise.all([
      getSharedPool(id), listMySeats(),
      listSharedPoolModelPricing(id).then(
        value => ({ value, failed: false }),
        () => ({ value: [] as SharedPoolModelEndpointPricing[], failed: true }),
      ),
    ])
    if (seq !== selectionSequence) return
    selectedPool.value = pool
    seats.value = memberships
    pricing.value = prices.value
    pricingError.value = prices.failed ? '模型报价暂不可读，请重新选择资源刷新；不能据此判断服务故障。' : ''
    emit('update:modelValue', { kind: 'shared', id, name: pool.name, ready: hasSeat(id) && available.value })
  } catch (cause) {
    if (seq === selectionSequence) actionError.value = extractActionableApiErrorMessage(cause, '资源条件读取失败，请重新选择')
  } finally {
    if (seq === selectionSequence) { selecting.value = false; emit('busy', false) }
  }
}
async function join() {
  const pool = selectedPool.value
  if (!pool || joining.value || !agreed.value || !joinable.value) return
  joining.value = true
  emit('busy', true)
  actionError.value = ''
  try {
    const result = await joinSharedPool(pool.id)
    seats.value = [...seats.value.filter(s => s.pool_id !== pool.id), result.seat]
    joinedHere.value = true
    emit('update:modelValue', { kind: 'shared', id: pool.id, name: pool.name, ready: true })
  } catch (cause) {
    // A timed-out write may have committed. Reconcile membership, never
    // automatically repeat a potentially paid join.
    try {
      seats.value = await listMySeats()
      if (hasSeat(pool.id)) {
        joinedHere.value = true
        emit('update:modelValue', { kind: 'shared', id: pool.id, name: pool.name, ready: true })
      }
    } catch { /* Keep creation disabled until membership can be verified. */ }
    actionError.value = hasSeat(pool.id)
      ? '已核实加入成功，可以继续创建密钥；无需重复加入。'
      : extractActionableApiErrorMessage(cause, '未收到加入成功确认，请重新选择资源核对席位后再试')
  } finally {
    joining.value = false
    emit('busy', false)
  }
}
watch(query, () => { clearTimeout(timer); timer = setTimeout(load, 250) })
onMounted(async () => {
  await load()
  if (props.modelValue?.kind === 'shared') await selectPool(props.modelValue.id, props.modelValue.name)
  else if (props.modelValue?.kind === 'official') {
    const group = groups.value.find(g => g.id === props.modelValue?.id)
    if (group) selectGroup(group)
    else emit('update:modelValue', null)
  }
})
onBeforeUnmount(() => { clearTimeout(timer); loadSequence++; selectionSequence++ })
</script>

<template>
  <section class="resource-choice" aria-label="选择使用资源">
    <div class="resource-search">
      <Search :size="16" aria-hidden="true" />
      <input v-model="query" type="search" :disabled="locked" placeholder="搜索资源、提供者或模型" aria-label="搜索使用资源" />
      <button type="button" :disabled="loading || locked" title="刷新资源" aria-label="刷新资源" @click="load"><RefreshCw :size="16" /></button>
    </div>
    <p v-if="error" role="alert" class="resource-error">{{ error }}</p>
    <p v-if="loading" class="resource-note" role="status">正在读取资源…</p>
    <div class="resource-options" role="group" aria-label="可选资源" :aria-busy="loading">
      <button v-for="group in visibleGroups" :key="`official-${group.id}`" type="button" class="resource-option" :class="{ selected: isSelected('official', group.id) }" :aria-pressed="isSelected('official', group.id)" :disabled="locked || loading" @click="selectGroup(group)">
        <Server :size="17" aria-hidden="true" /><span><strong>{{ group.name }}</strong><small>{{ group.platform }} · {{ group.subscription_type === 'subscription' ? '订阅资源' : '按用量计费' }}</small></span><em>官方</em><Check v-if="isSelected('official', group.id)" :size="16" />
      </button>
      <button v-for="pool in visiblePools" :key="`shared-${pool.id}`" type="button" class="resource-option" :class="{ selected: isSelected('shared', pool.id) }" :aria-pressed="isSelected('shared', pool.id)" :disabled="locked || loading" @click="selectPool(pool.id, pool.name)">
        <Users :size="17" aria-hidden="true" /><span><strong>{{ pool.name }}</strong><small>{{ pool.detail }}</small></span><em>{{ hasSeat(pool.id) ? '已加入' : '共享' }}</em><Check v-if="isSelected('shared', pool.id)" :size="16" />
      </button>
      <p v-if="!loading && !visibleGroups.length && !visiblePools.length" class="resource-note">没有找到匹配资源</p>
    </div>
    <p v-if="total > pools.length" class="resource-note">显示 {{ pools.length }} 个共享资源，共 {{ total }} 个；搜索可缩小范围。</p>
    <p v-if="selecting" role="status" class="resource-note">正在核对资源与加入条件…</p>
    <div v-if="selectedPool" class="resource-terms">
      <strong>{{ selectedPool.name }}</strong>
      <p v-if="!available" role="status" class="resource-error">资源已停用、未上架或尚未开通计费，现有席位不代表现在可以调用；请换一个资源。</p>
      <dl><div><dt>席位费用</dt><dd>{{ money(selectedPool.hourly_seat_fee) }} / 小时</dd></div><div><dt>最低余额</dt><dd>{{ money(selectedPool.min_balance_admission) }}</dd></div></dl>
      <p v-if="selectedPool.hourly_min_usage_waiver" class="resource-note">每小时用量达到 {{ money(selectedPool.hourly_min_usage_waiver) }} 可按资源规则减免席位费。</p>
      <p class="resource-note">模型调用另按实际用量计费，平台服务费在池主定价之上计算。</p>
      <details class="resource-capabilities">
        <summary>查看模型与接口条件</summary>
        <p class="resource-note">共享密钥使用 OpenAI 兼容接口；具体模型、接口和价格以本资源为准。Messages、Gemini 原生接口、Embeddings 和 WebSocket 尚未开放，不会自动转换。</p>
        <p v-if="pricingError" role="status" class="resource-note">{{ pricingError }}</p>
        <ul v-else-if="enabledPricing.length">
          <li v-for="item in enabledPricing" :key="`${item.pool_model_id}-${item.endpoint_id}`">
            <strong>{{ item.display_name || item.model_name }}</strong>
            <span>{{ sharedPoolEndpointShortLabel(item.endpoint_type) }} · {{ sharedPoolEndpointAvailability(item).label }}</span>
          </li>
        </ul>
        <p v-else class="resource-note">暂未读取到已启用的接口报价；有模型名称不等于该模型的所有接口都可调用。</p>
        <p class="resource-note">以上是配置条件，不是实时在线保证；调用时仍会核对资源状态、模型授权与余额。</p>
      </details>
      <template v-if="!modelValue?.ready">
        <p v-if="!joinable" class="resource-error">当前资源不可加入或席位已满。</p>
        <template v-else>
          <label class="resource-consent"><input v-model="agreed" type="checkbox" :disabled="locked" />确认加入条件；加入后席位按资源规则计费，关闭弹窗不会退出席位。</label>
          <button type="button" class="resource-join" :disabled="locked || !agreed" @click="join">{{ joining ? '正在加入…' : '确认加入此资源' }}</button>
        </template>
      </template>
      <p v-else class="resource-note" role="status">{{ joinedHere ? '已加入。现在可继续创建密钥；关闭窗口不会取消席位。' : '已加入，可以直接创建密钥。' }}</p>
    </div>
    <p v-if="actionError" role="alert" class="resource-error">{{ actionError }}</p>
  </section>
</template>

<style scoped>
.resource-choice{display:grid;gap:10px;font-size:13px;color:var(--bd-text-primary)}
.resource-search{display:flex;align-items:center;gap:10px;border:1px solid var(--bd-ui-line);border-radius:6px;padding:6px 10px}
.resource-search input{width:100%;min-width:0;background:transparent;border:0;outline:none;padding:4px}
.resource-search button{width:30px;height:30px;display:grid;place-items:center;flex-shrink:0}
.resource-options{max-height:240px;overflow:auto;overscroll-behavior:contain;border-block:1px solid var(--bd-ui-line)}
.resource-option{display:flex;align-items:center;gap:12px;width:100%;min-height:58px;padding:10px;text-align:left;border-bottom:1px solid var(--bd-ui-line);border-left:3px solid transparent}
.resource-option:last-child{border-bottom:0}.resource-option:hover{background:var(--bd-canvas)}.resource-option.selected{border-left-color:var(--bd-accent-teal);background:var(--bd-canvas)}
.resource-option>span{flex:1;min-width:0}.resource-option strong{display:block;overflow-wrap:anywhere;font-weight:550}.resource-option small{display:block;color:var(--bd-text-secondary);overflow-wrap:anywhere;line-height:1.5;margin-top:3px}.resource-option em{font-style:normal;font-size:11px;color:var(--bd-text-secondary);white-space:nowrap}.resource-option svg{flex-shrink:0}
.resource-note{font-size:12px;color:var(--bd-text-secondary);line-height:1.6;margin:0;padding:4px 0}.resource-error{font-size:12px;line-height:1.6;color:var(--bd-status-danger);margin:0}
.resource-terms{border-left:2px solid var(--bd-accent-teal);padding:4px 0 4px 12px}.resource-terms dl{display:flex;flex-wrap:wrap;gap:12px 28px;margin:10px 0}.resource-terms dt{font-size:11px;color:var(--bd-text-secondary)}.resource-terms dd{margin:3px 0 0;font-variant-numeric:tabular-nums}
.resource-consent{display:flex;align-items:flex-start;gap:8px;font-size:12px;line-height:1.7;margin:8px 0}.resource-consent input{margin-top:4px;accent-color:var(--bd-accent-teal)}
.resource-capabilities{margin:10px 0;font-size:12px;line-height:1.6}.resource-capabilities summary{cursor:pointer;color:var(--bd-accent-teal)}.resource-capabilities ul{padding:0;list-style:none;max-height:160px;overflow:auto}.resource-capabilities li{display:flex;flex-wrap:wrap;justify-content:space-between;gap:4px 12px;padding:6px 0;border-bottom:1px solid var(--bd-ui-line);overflow-wrap:anywhere}.resource-capabilities li strong{font-weight:500}.resource-capabilities li span{color:var(--bd-text-secondary)}
.resource-join{padding:8px 12px;border-radius:6px;background:var(--bd-accent-teal);color:white}button:disabled{opacity:.5;cursor:not-allowed}button:focus-visible,input:focus-visible{outline:2px solid var(--bd-accent-teal);outline-offset:2px}
</style>
