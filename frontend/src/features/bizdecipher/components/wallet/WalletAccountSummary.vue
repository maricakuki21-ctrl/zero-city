<script setup lang="ts">
import { RouterLink } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'

defineProps<{
  balance?: number
  credits?: number
  earningsAvailable?: number
  earningsTotal?: number
  earningsPending?: number
  loading: boolean
  earningsLoading?: boolean
  error?: string
  earningsError?: string
}>()

defineEmits<{ refresh: []; 'refresh-earnings': [] }>()

function formatAmount(value: number | undefined): string {
  if (value === undefined || !Number.isFinite(Number(value))) return '—'
  return Number(value).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 6 })
}
</script>

<template>
  <section class="wallet-account" aria-labelledby="wallet-account-title" :aria-busy="loading">
    <header>
      <div>
        <p>账户资产</p>
        <h2 id="wallet-account-title">我的资产</h2>
      </div>
      <button type="button" :disabled="loading" @click="$emit('refresh')"><Icon name="refresh" size="sm" />{{ loading ? '同步中…' : '同步账户' }}</button>
    </header>
    <p v-if="error" class="wallet-account-error" role="alert">{{ error }} 当前显示会话中最近一次余额。</p>
    <div class="wallet-account-metrics">
      <div>
        <span>站内可消费余额</span>
        <strong data-testid="site-balance">{{ formatAmount(balance) }}</strong>
        <small>充值、返利或收益转入后用于站内消费</small>
        <div class="wallet-account-actions">
          <RouterLink to="/purchase">充值</RouterLink>
          <RouterLink to="/usage">消费流水</RouterLink>
        </div>
      </div>
      <div>
        <span>创作与供给收益</span>
        <strong data-testid="earnings-available">{{ formatAmount(earningsAvailable) }}</strong>
        <small>
          可转站内余额 · 累计已结算 {{ formatAmount(earningsTotal) }}
          <template v-if="earningsPending && earningsPending > 0"> · 待结算 {{ formatAmount(earningsPending) }}</template>
        </small>
        <div class="wallet-account-actions">
          <a href="#shared-pool-earnings">收益明细</a>
          <button type="button" :disabled="earningsLoading" @click="$emit('refresh-earnings')">
            {{ earningsLoading ? '同步中…' : '同步收益' }}
          </button>
        </div>
      </div>
      <div>
        <span>积分</span>
        <strong data-testid="credit-balance">{{ formatAmount(credits) }}</strong>
        <small>签到和活动激励，独立于现金余额</small>
        <div class="wallet-account-actions">
          <RouterLink to="/incentives">获取方式</RouterLink>
        </div>
      </div>
    </div>
    <p v-if="earningsError" class="wallet-account-error" role="alert">{{ earningsError }}</p>
  </section>
</template>

<style scoped>
.wallet-account { display: grid; gap: var(--bd-space-4); padding-bottom: var(--bd-space-6); border-bottom: 1px solid var(--bd-ui-line); }
.wallet-account header { display: flex; align-items: center; justify-content: space-between; gap: var(--bd-space-4); }
.wallet-account header p { margin: 0 0 var(--bd-space-1); color: var(--bd-accent-teal); font-size: var(--bd-type-overline); font-weight: var(--bd-weight-overline); }
.wallet-account h2 { margin: 0 0 4px; font-size: 18px; color: var(--bd-text-primary); }
.wallet-account-note { display: block; color: var(--bd-text-secondary); font-size: var(--bd-type-caption); line-height: 1.6; }
.wallet-account header button { min-height: 36px; display: inline-flex; align-items: center; gap: var(--bd-space-2); padding: 0 var(--bd-space-3); color: var(--bd-text-primary); background: var(--bd-surface); border: 1px solid var(--bd-ui-line); border-radius: var(--bd-radius-sm); font-size: var(--bd-type-caption); }
.wallet-account header button:disabled { opacity: .55; cursor: wait; }
.wallet-account-metrics { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); border: 1px solid var(--bd-ui-line); border-radius: var(--bd-radius-md); background: var(--bd-surface); }
.wallet-account-metrics > div { min-width: 0; padding: var(--bd-space-4); }
.wallet-account-metrics > div + div { border-left: 1px solid var(--bd-ui-line); }
.wallet-account-metrics span, .wallet-account-metrics small { display: block; color: var(--bd-text-secondary); font-size: var(--bd-type-caption); }
.wallet-account-metrics strong { display: block; margin: var(--bd-space-2) 0; color: var(--bd-text-primary); font-size: 26px; line-height: 1.2; overflow-wrap: anywhere; }
.wallet-account-actions { display: flex; flex-wrap: wrap; gap: 8px; margin-top: var(--bd-space-3); }
.wallet-account-actions a, .wallet-account-actions button { min-height: 30px; display: inline-flex; align-items: center; padding: 0 10px; color: var(--bd-accent-teal); background: transparent; border: 1px solid var(--bd-ui-line); border-radius: var(--bd-radius-sm); font-size: 12px; text-decoration: none; }
.wallet-account-actions button:disabled { opacity: .55; }
.wallet-account-sources { display: grid; gap: 4px; margin: var(--bd-space-3) 0 0; padding: var(--bd-space-3) 0 0; list-style: none; border-top: 1px dashed var(--bd-ui-line); }
.wallet-account-sources li { display: flex; align-items: center; justify-content: space-between; gap: 8px; font-size: 12px; color: var(--bd-text-secondary); }
.wallet-account-sources b { color: var(--bd-accent-teal); font-weight: 500; }
.wallet-account-sources b.is-off { color: var(--bd-text-secondary); }
.wallet-account-error { margin: 0; padding: var(--bd-space-3); color: var(--bd-status-danger); background: color-mix(in srgb, var(--bd-status-danger) 8%, var(--bd-surface)); border-left: 3px solid var(--bd-status-danger); font-size: var(--bd-type-caption); }
@media (max-width: 900px) { .wallet-account-metrics { grid-template-columns: 1fr; } .wallet-account-metrics > div + div { border-left: 0; border-top: 1px solid var(--bd-ui-line); } .wallet-account header { align-items: flex-start; flex-direction: column; } }
</style>
