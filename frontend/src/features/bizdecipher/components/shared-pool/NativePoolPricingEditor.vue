<template>
  <section class="native-pricing" aria-label="原生池报价">
    <header class="native-pricing__head">
      <div>
        <h3>报价与调用验证</h3>
        <p>价格单位为美元（USD）；报价就绪不代表实际推理已经验证。</p>
      </div>
      <button class="native-pricing__refresh" type="button" :disabled="loading" @click="$emit('refresh')">
        <RefreshCw :size="14" />{{ loading ? '读取中…' : '刷新报价' }}
      </button>
    </header>

    <p v-if="message" class="native-pricing__message" :class="{ 'is-error': status === 'error' }" :role="status === 'error' ? 'alert' : 'status'">
      {{ message }}
    </p>
    <p v-if="loading && !items.length" class="native-pricing__empty">正在读取模型报价…</p>
    <p v-else-if="!items.length" class="native-pricing__empty">尚无可配置的服务。先完成模型目录识别。</p>

    <ul v-else class="native-pricing__list">
      <li v-for="item in items" :key="pricingKey(item)" class="native-pricing__item">
        <div class="native-pricing__summary">
          <div>
            <strong>{{ item.display_name || item.model_name }}</strong>
            <span>{{ endpointLabel(item.endpoint_type) }}</span>
          </div>
          <div class="native-pricing__badges">
            <span :class="compatible(item) ? 'tone-ready' : 'tone-warning'">{{ compatible(item) ? '报价就绪' : '报价待完善' }}</span>
            <span :class="item.pricing_source === 'official_catalog' ? 'source-official' : 'source-custom'">
              {{ item.pricing_source === 'official_catalog' ? '官方价格' : '池主价格' }}
            </span>
          </div>
        </div>
        <p class="native-pricing__detail">{{ item.enabled ? '服务已启用' : '服务已关闭' }} · {{ inferenceLabel(item.gate_status) }}</p>
        <div v-if="item.current_price && compatible(item)" class="native-pricing__facts">
          <span>基础单价 <b>{{ formatComponents(item.current_price.base_price) }}</b></span>
          <span>倍率 <b>{{ formatMultiplier(item.current_price.multiplier) }}</b></span>
          <span v-if="item.current_price.owner_price">池主原价 <b>{{ formatComponents(item.current_price.owner_price) }}</b></span>
          <span v-if="item.current_price.fee_mode === 'buyer_surcharge_v1'">平台另加 <b>{{ item.current_price.platform_fee_percent ?? 0 }}%</b></span>
          <span>用户实付 <b>{{ formatComponents(item.current_price.user_price) }}</b></span>
          <span>生效于 <b>{{ formatDate(item.current_price.effective_from) }}</b></span>
        </div>
        <p v-else class="native-pricing__pending">价格未完整，系统不会按 0 元放行。</p>

        <details v-if="item.pricing_source === 'owner_custom' && canEdit(item) && drafts[pricingKey(item)]" class="native-pricing__editor">
          <summary>{{ item.current_price ? '调整池主价格' : '补齐模型报价' }}</summary>
          <fieldset :disabled="Boolean(savingKey)">
          <div class="native-pricing__fields">
            <label><span>收费方式</span><select :value="drafts[pricingKey(item)].billing_mode" @change="changeBilling(item, $event)">
              <option v-for="option in billingOptions(item.endpoint_type)" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select></label>
            <label v-if="drafts[pricingKey(item)].billing_mode === 'token'"><span>输入 / 百万 token</span><input :value="drafts[pricingKey(item)].input_per_million" type="number" min="0" step="0.01" @input="changeNumber(item, 'input_per_million', $event)" /></label>
            <label v-if="drafts[pricingKey(item)].billing_mode === 'token'"><span>输出 / 百万 token</span><input :value="drafts[pricingKey(item)].output_per_million" type="number" min="0" step="0.01" @input="changeNumber(item, 'output_per_million', $event)" /></label>
            <label v-if="drafts[pricingKey(item)].billing_mode === 'token'"><span>缓存读取 / 百万（可选）</span><input :value="drafts[pricingKey(item)].cache_read_per_million" type="number" min="0" step="0.01" @input="changeNumber(item, 'cache_read_per_million', $event)" /></label>
            <label v-if="drafts[pricingKey(item)].billing_mode === 'token'"><span>缓存写入 / 百万（可选）</span><input :value="drafts[pricingKey(item)].cache_write_per_million" type="number" min="0" step="0.01" @input="changeNumber(item, 'cache_write_per_million', $event)" /></label>
            <label v-if="drafts[pricingKey(item)].billing_mode === 'image'"><span>每张图片</span><input :value="drafts[pricingKey(item)].image_item_price" type="number" min="0" step="0.001" @input="changeNumber(item, 'image_item_price', $event)" /></label>
            <label v-if="drafts[pricingKey(item)].billing_mode === 'video'"><span>每秒视频</span><input :value="drafts[pricingKey(item)].video_second_price" type="number" min="0" step="0.001" @input="changeNumber(item, 'video_second_price', $event)" /></label>
            <label v-if="drafts[pricingKey(item)].billing_mode === 'per_request'"><span>每次请求</span><input :value="drafts[pricingKey(item)].per_request_price" type="number" min="0" step="0.001" @input="changeNumber(item, 'per_request_price', $event)" /></label>
            <label><span>池主原价倍率</span><input :value="drafts[pricingKey(item)].multiplier" type="number" min="0.0001" step="0.0001" @input="changeNumber(item, 'multiplier', $event)" /></label>
            <label><span>最低收费（可选）</span><input :value="drafts[pricingKey(item)].minimum_charge" type="number" min="0" step="0.001" @input="changeNumber(item, 'minimum_charge', $event)" /></label>
            <label><span>最高收费（可选）</span><input :value="drafts[pricingKey(item)].maximum_charge" type="number" min="0" step="0.001" @input="changeNumber(item, 'maximum_charge', $event)" /></label>
          </div>
          <button class="native-pricing__save" type="button" :disabled="Boolean(savingKey)" @click="$emit('save', item)">
            {{ savingKey === pricingKey(item) ? '发布中…' : '发布价格版本' }}
          </button>
          </fieldset>
        </details>
        <p v-else-if="item.pricing_source === 'official_catalog'" class="native-pricing__readonly">官方目录价格由平台维护，随目录更新。</p>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { RefreshCw } from '@lucide/vue'
import type { SharedPoolBillingMode, SharedPoolModelEndpointPricing, SharedPoolPriceComponents } from '@/features/bizdecipher/api/bizdecipher'
import {
  sharedPoolBillingOptions,
  sharedPoolBillingModeAllowed,
  sharedPoolEndpointHasCompatiblePrice,
  sharedPoolEndpointLabel,
  sharedPoolEndpointOrder,
} from './sharedPoolPricing'

export type NativePricingDraft = {
  billing_mode: SharedPoolBillingMode
  input_per_million: number | null
  output_per_million: number | null
  cache_read_per_million: number | null
  cache_write_per_million: number | null
  image_item_price: number | null
  video_second_price: number | null
  per_request_price: number | null
  multiplier: number
  minimum_charge: number | null
  maximum_charge: number | null
}

const props = defineProps<{
  items: SharedPoolModelEndpointPricing[]
  drafts: Record<string, NativePricingDraft>
  loading?: boolean
  savingKey?: string
  message?: string
  status?: 'success' | 'error'
}>()

const emit = defineEmits<{
  refresh: []
  save: [item: SharedPoolModelEndpointPricing]
  updateDraft: [item: SharedPoolModelEndpointPricing, draft: NativePricingDraft]
}>()

function pricingKey(item: Pick<SharedPoolModelEndpointPricing, 'pool_model_id' | 'endpoint_type'>) {
  return `${item.pool_model_id}:${item.endpoint_type}`
}
function endpointLabel(endpoint: string) { return sharedPoolEndpointLabel(endpoint) }
function compatible(item: SharedPoolModelEndpointPricing) { return sharedPoolEndpointHasCompatiblePrice(item) }
function canEdit(item: SharedPoolModelEndpointPricing) { return sharedPoolEndpointOrder.includes(item.endpoint_type) }
function billingOptions(endpoint: string) { return sharedPoolBillingOptions(endpoint) }
function changeBilling(item: SharedPoolModelEndpointPricing, event: Event) {
  const value = (event.target as HTMLSelectElement).value
  const draft = props.drafts[pricingKey(item)]
  if (!props.savingKey && draft && sharedPoolBillingModeAllowed(item.endpoint_type, value)) {
    emit('updateDraft', item, { ...draft, billing_mode: value })
  }
}
function changeNumber(item: SharedPoolModelEndpointPricing, field: Exclude<keyof NativePricingDraft, 'billing_mode'>, event: Event) {
  const input = event.target as HTMLInputElement
  const draft = props.drafts[pricingKey(item)]
  if (!props.savingKey && draft) {
    const value = input.value === '' ? (field === 'multiplier' ? Number.NaN : null) : input.valueAsNumber
    emit('updateDraft', item, { ...draft, [field]: value })
  }
}
function inferenceLabel(status: string) {
  if (status === 'passed') return '调用验证通过'
  if (status === 'failed') return '调用验证失败'
  if (status === 'stale') return '调用证据已过期'
  return '调用未验证'
}
function formatMultiplier(value: number | null | undefined) {
  return Number.isFinite(Number(value)) ? Number(value).toLocaleString('en-US', { maximumFractionDigits: 6 }) : '—'
}
function formatMoney(value: number | null | undefined) {
  const amount = Number(value || 0)
  return amount < 0.01 ? `$${amount.toFixed(5)}` : `$${amount.toFixed(4)}`
}
function formatComponents(price: SharedPoolPriceComponents) {
  if (price.billing_mode === 'token') return `输入 ${formatMoney(Number(price.input_price) * 1_000_000)} / 输出 ${formatMoney(Number(price.output_price) * 1_000_000)} 每百万 token`
  if (price.billing_mode === 'image') return `${formatMoney(price.image_item_price)} / 张`
  if (price.billing_mode === 'video') return `${formatMoney(price.video_second_price)} / 秒`
  return `${formatMoney(price.per_request_price)} / 次`
}
function formatDate(value: string | undefined) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleDateString('zh-CN')
}
</script>

<style scoped>
.native-pricing { margin-top: 24px; border-top: 1px solid var(--bd-ui-line); padding-top: 20px; }
.native-pricing__head, .native-pricing__summary { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.native-pricing h3 { margin: 0; font-size: 14px; font-weight: 600; color: var(--bd-text-primary); }
.native-pricing__head p, .native-pricing__detail, .native-pricing__readonly, .native-pricing__pending { color: var(--bd-text-secondary); font-size: 12px; line-height: 1.6; }
.native-pricing__head p { margin: 5px 0 0; max-width: 620px; }
.native-pricing__refresh, .native-pricing__save { display: inline-flex; align-items: center; justify-content: center; gap: 6px; min-height: 32px; padding: 0 10px; border: 1px solid var(--bd-ui-line); border-radius: 6px; background: var(--bd-surface); color: var(--bd-accent-teal); font: inherit; font-size: 12px; cursor: pointer; }
.native-pricing__refresh:disabled, .native-pricing__save:disabled { opacity: .55; cursor: wait; }
.native-pricing__message { margin: 12px 0 0; color: var(--bd-accent-teal); font-size: 12px; }
.native-pricing__message.is-error { color: var(--bd-status-danger); }
.native-pricing__empty { padding: 14px 0; color: var(--bd-text-secondary); font-size: 12px; }
.native-pricing__list { display: grid; gap: 10px; margin: 16px 0 0; padding: 0; list-style: none; }
.native-pricing__item { display: grid; gap: 10px; min-width: 0; padding: 14px 0; border-bottom: 1px solid var(--bd-ui-line); }
.native-pricing__summary > div:first-child { display: grid; gap: 3px; min-width: 0; }
.native-pricing__summary strong { overflow-wrap: anywhere; color: var(--bd-text-primary); font-size: 13px; font-weight: 600; }
.native-pricing__summary span, .native-pricing__badges span { color: var(--bd-text-secondary); font-size: 11px; }
.native-pricing__badges { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 5px; }
.native-pricing__badges span { padding: 3px 6px; border-radius: 5px; }
.tone-ready { color: var(--bd-accent-teal) !important; background: color-mix(in srgb, var(--bd-accent-teal) 10%, transparent); }
.tone-warning { color: var(--bd-accent-gold) !important; background: color-mix(in srgb, var(--bd-accent-gold) 10%, transparent); }
.tone-danger { color: var(--bd-status-danger) !important; background: color-mix(in srgb, var(--bd-status-danger) 10%, transparent); }
.tone-muted { background: color-mix(in srgb, var(--bd-text-secondary) 10%, transparent); }
.source-official { color: var(--bd-accent-blue) !important; background: color-mix(in srgb, var(--bd-accent-blue) 10%, transparent); }
.source-custom { color: var(--bd-accent-teal) !important; background: color-mix(in srgb, var(--bd-accent-teal) 10%, transparent); }
.native-pricing__detail, .native-pricing__readonly, .native-pricing__pending { margin: 0; }
.native-pricing__pending { color: var(--bd-status-danger); }
.native-pricing__facts { display: flex; flex-wrap: wrap; gap: 8px 18px; color: var(--bd-text-secondary); font-size: 11px; }
.native-pricing__facts b { color: var(--bd-text-primary); font-weight: 500; }
.native-pricing__editor { border-top: 1px solid var(--bd-ui-line); padding-top: 10px; }
.native-pricing__editor fieldset { border: 0; padding: 0; margin: 0; min-width: 0; }
.native-pricing__editor summary { cursor: pointer; color: var(--bd-accent-teal); font-size: 12px; }
.native-pricing__fields { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 9px; margin-top: 12px; }
.native-pricing__fields label { display: grid; gap: 5px; min-width: 0; color: var(--bd-text-secondary); font-size: 11px; }
.native-pricing__fields input, .native-pricing__fields select { min-width: 0; min-height: 32px; padding: 0 8px; border: 1px solid var(--bd-ui-line); border-radius: 5px; background: var(--bd-surface); color: var(--bd-text-primary); font: inherit; }
.native-pricing__save { margin-top: 12px; background: var(--bd-accent-teal); color: #fff; border-color: transparent; }
@media (max-width: 600px) {
  .native-pricing__head, .native-pricing__summary { flex-direction: column; }
  .native-pricing__badges { justify-content: flex-start; }
  .native-pricing__fields { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 380px) { .native-pricing__fields { grid-template-columns: minmax(0, 1fr); } }
</style>
