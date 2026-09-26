<template>
  <div class="pool-setup">
    <nav class="setup-tabs" aria-label="开池步骤">
      <button v-for="item in steps" :key="item.id" type="button" :class="{ active: step === item.id }" :aria-current="step === item.id ? 'step' : undefined" @click="step = item.id">
        <span>{{ item.number }}</span>{{ item.label }}
      </button>
    </nav>
    <div class="setup-layout">
      <div class="setup-main">
        <fieldset v-show="step === 'connect'" :disabled="writeLocked" aria-label="接入资源">
          <slot name="connection" />
          <button v-if="accounts.length" class="setup-next" type="button" @click="step = 'models'">查看模型与检测<ArrowRight :size="16" /></button>
        </fieldset>
        <fieldset v-show="step === 'models'" :disabled="writeLocked" aria-label="模型与检测">
          <div class="setup-heading"><h2>模型与检测</h2><button type="button" @click="connect">添加资源<Plus :size="16" /></button></div>
          <SharedPoolNativeOnboardingStatus :state="pool.native_onboarding_state || 'draft'" :accounts="accounts" :readiness-pending-account-id="readinessPendingAccountId" @retry="repair" @readiness="emit('readiness', $event)" />
          <div class="model-pricing" aria-label="模型报价与调用验证">
            <NativePoolPricingEditor
              :items="pricing || []"
              :drafts="pricingDrafts || {}"
              :loading="pricingLoading"
              :saving-key="pricingSavingKey"
              :message="pricingMessage || pricingError"
              :status="pricingStatus"
              @refresh="emit('refreshPricing')"
              @save="emit('savePricing', $event)"
              @update-draft="(item, draft) => emit('updatePricingDraft', item, draft)"
            />
          </div>
          <button v-if="accounts.length" class="setup-next" type="button" @click="step = 'preview'">查看发布预览<ArrowRight :size="16" /></button>
          <button v-else class="setup-next" type="button" @click="connect">先接入资源<ArrowRight :size="16" /></button>
        </fieldset>
        <section v-show="step === 'preview'" aria-label="发布预览">
          <div class="setup-heading"><h2>发布预览</h2></div>
          <fieldset :disabled="writeLocked"><slot name="appearance" /></fieldset>
          <dl class="publish-summary">
            <div><dt>池子名称</dt><dd>{{ pool.name }}</dd></div>
            <div><dt>已接入资源</dt><dd>{{ accounts.length }} 个</dd></div>
            <div><dt>已拉取模型</dt><dd>{{ models.length }} 个</dd></div>
            <div><dt>当前状态</dt><dd>{{ pool.listed ? '已上架' : '尚未发布' }}</dd></div>
          </dl>
          <div v-if="!pool.listed" class="publish-note" role="status">
            <Info :size="18" />
            <div><strong>{{ publicationNotice.title }}</strong><p>{{ publicationNotice.detail }}</p></div>
          </div>
          <p v-if="publicationMessage" class="publication-result" :class="{ 'publication-result-error': publicationState === 'error' }" :role="publicationState === 'error' ? 'alert' : 'status'">{{ publicationMessage }}</p>
          <button v-if="canPublish || publishing || unresolvedPublication" class="publish-button" type="button" data-testid="pool-onboarding-publish" :disabled="publishing || !canPublish" @click="emit('publish')">
            <Upload :size="16" />{{ publishing ? '正在确认发布…' : unresolvedPublication ? '重试确认发布结果' : '开通计费并上架' }}
          </button>
          <button class="setup-next" type="button" @click="step = accounts.length ? 'models' : 'connect'">返回继续配置<ArrowRight :size="16" /></button>
        </section>
      </div>
      <aside class="setup-preview"><div class="preview-heading"><span>市场卡片</span><span>预览</span></div><PoolListingPreview :name="pool.name" :description="pool.description" :models="models" :image="image" :listed="pool.listed" :price="previewPrice" /><p v-if="!pool.listed" class="preview-note">未发布时，仅你自己可见。</p></aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ArrowRight, Info, Plus, Upload } from '@lucide/vue'
import type { SharedPool, SharedPoolAccount, SharedPoolModelEndpointPricing } from '@/features/bizdecipher/api/bizdecipher'
import { sharedPoolEndpointHasCompatiblePrice } from './sharedPoolPricing'
import PoolListingPreview from './PoolListingPreview.vue'
import SharedPoolNativeOnboardingStatus from './SharedPoolNativeOnboardingStatus.vue'
import NativePoolPricingEditor, { type NativePricingDraft } from './NativePoolPricingEditor.vue'
type Step = 'connect' | 'models' | 'preview'
const props = defineProps<{
  pool: SharedPool
  accounts: SharedPoolAccount[]
  readinessPendingAccountId?: number | null
  connecting?: boolean
  pricing?: SharedPoolModelEndpointPricing[]
  pricingLoading?: boolean
  pricingError?: string
  pricingMessage?: string
  pricingStatus?: 'success' | 'error'
  pricingDrafts?: Record<string, NativePricingDraft>
  pricingSavingKey?: string
  canPublish?: boolean
  publishing?: boolean
  unresolvedPublication?: boolean
  writeLocked?: boolean
  publicationMessage?: string
  publicationState?: string
}>()
const emit = defineEmits<{
  retry: [account: SharedPoolAccount]
  readiness: [account: SharedPoolAccount]
  connect: []
  publish: []
  refreshPricing: []
  savePricing: [item: SharedPoolModelEndpointPricing]
  updatePricingDraft: [item: SharedPoolModelEndpointPricing, draft: NativePricingDraft]
}>()
const step = ref<Step>(props.connecting ? 'connect' : props.accounts.length ? 'models' : 'connect')
const steps = [{ id: 'connect', number: '1', label: '接入资源' }, { id: 'models', number: '2', label: '模型与检测' }, { id: 'preview', number: '3', label: '外观与发布' }] as const
const publicationNotice = computed(() => {
  if (props.pool.native_onboarding_state === 'supply_needs_attention') {
    return { title: '资源连接需要处理', detail: '返回模型与检测查看具体失败项，修复后重新检查。' }
  }
  if (!props.accounts.length) return { title: '尚未接入资源', detail: '先保存资源连接，再拉取实际模型目录。' }
  if (props.pool.native_onboarding_state === 'supply_ready_billing_blocked') {
    return { title: '待开通计费与上架', detail: '发布时由服务器核对资源、有效报价和结算配置；开通不等于所有模型已通过实际推理检测。' }
  }
  return { title: '尚未发布', detail: '返回模型与检测查看服务器验证结果；资源已连接不代表价格和上架条件均已满足。' }
})
const models = computed(() => [...new Set(props.accounts.flatMap(account => account.native_models || []))])
const previewPrice = computed(() => props.pricingLoading ? '读取中' : props.pricingError ? '读取失败' : props.pricing?.some(sharedPoolEndpointHasCompatiblePrice) ? '按模型计价' : '待配置')
const image = computed(() => props.pool.card_skin_key && props.pool.card_skin_rarity ? `/assets/zero-point-city/cards/collectible/${props.pool.card_skin_rarity}/${props.pool.card_skin_key}.png` : undefined)
watch(() => props.accounts.length, (count, previous) => { if (count > previous) step.value = 'models' })
watch(() => props.connecting, value => { if (value) step.value = 'connect' })
watch(() => props.unresolvedPublication, value => { if (value) step.value = 'preview' }, { immediate: true })
function connect() { step.value = 'connect'; emit('connect') }
function repair(account: SharedPoolAccount) { step.value = 'connect'; emit('retry', account) }
</script>

<style scoped>
.pool-setup { color: var(--bd-text-primary); min-width: 0; }
.setup-tabs { display: flex; gap: 24px; border-bottom: 1px solid var(--bd-ui-line); margin-bottom: 28px; }
.setup-tabs button { display: flex; align-items: center; gap: 8px; padding: 14px 0; font-size: 14px; border-bottom: 2px solid transparent; color: var(--bd-text-secondary); }
.setup-tabs button.active { border-color: var(--bd-accent-teal); color: var(--bd-accent-teal); }
.setup-tabs button>span { display: grid; place-items: center; height: 22px; width: 22px; border: 1px solid currentColor; border-radius: 50%; font-size: 12px; }
.setup-layout { display: grid; grid-template-columns: minmax(0, 1fr) 300px; gap: 36px; align-items: start; }
.setup-main { min-width: 0; }
.setup-main fieldset { min-width: 0; border: 0; padding: 0; margin: 0; }
.setup-main fieldset:disabled { opacity: .7; }
.model-pricing { margin-top: 24px; font-size: 13px; color: var(--bd-text-secondary); }
.model-pricing h3 { color: var(--bd-text-primary); font-size: 14px; font-weight: 600; }
.publish-button { display: flex; align-items: center; justify-content: center; gap: 8px; min-height: 42px; margin-top: 20px; padding: 8px 16px; background: var(--bd-accent-teal); color: #fff; border-radius: 6px; font-size: 14px; }
.publish-button:disabled { opacity: .6; cursor: wait; }
.publication-result { margin: 16px 0 0; font-size: 13px; line-height: 1.7; overflow-wrap: anywhere; }
.publication-result-error { color: var(--bd-status-danger); }
.setup-heading { display: flex; gap: 12px; align-items: center; justify-content: space-between; margin-bottom: 20px; }
.setup-heading h2 { font-size: 18px; font-weight: 600; }
.setup-heading button,.setup-next { display: inline-flex; align-items: center; gap: 8px; color: var(--bd-accent-teal); font-size: 13px; }
.setup-next { margin-top: 20px; min-height: 40px; }
.preview-heading { display: flex; justify-content: space-between; font-size: 12px; color: var(--bd-text-secondary); margin-bottom: 12px; }
.preview-note { font-size: 12px; color: var(--bd-text-secondary); margin-top: 12px; }
.publish-summary { margin: 0 0 24px; }
.publish-summary>div { display: flex; justify-content: space-between; gap: 20px; padding: 16px 0; border-bottom: 1px solid var(--bd-ui-line); font-size: 14px; }
.publish-summary dt { color: var(--bd-text-secondary); }.publish-summary dd { margin: 0; overflow-wrap: anywhere; text-align: right; }
.publish-note { display: flex; gap: 12px; font-size: 13px; line-height: 1.7; border-left: 3px solid var(--bd-accent-amber, #976317); padding-left: 16px; }.publish-note>svg { flex-shrink: 0; margin-top: 3px; }.publish-note strong { font-weight: 500; }.publish-note p { color: var(--bd-text-secondary); margin-top: 4px; }
button:focus-visible { outline: 2px solid var(--bd-accent-teal); outline-offset: 3px; }
@media(max-width: 900px) { .setup-layout { grid-template-columns: minmax(0,1fr) 260px; gap: 24px; } }
@media(max-width: 767px) { .setup-layout { grid-template-columns: minmax(0,1fr); }.setup-tabs { gap: 16px; }.setup-tabs button { font-size: 12px; gap: 6px; }.setup-preview { width: min(100%,360px); }.setup-tabs button>span { width: 20px; height: 20px; } }
</style>
