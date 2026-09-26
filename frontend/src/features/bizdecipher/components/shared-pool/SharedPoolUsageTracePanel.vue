<template>
  <section class="trace-panel" aria-labelledby="shared-pool-trace-title">
    <div class="trace-head">
      <div>
        <h3 id="shared-pool-trace-title">共享池调用明细</h3>
        <p>一眼看懂用了哪个池、哪个模型、多少缓存，以及时间花在了哪里。线路只显示安全别名，不会暴露池主密钥。</p>
      </div>
      <button class="trace-button" type="button" :disabled="loading || refreshing" @click="refresh">
        {{ refreshing ? '正在刷新…' : '刷新明细' }}
      </button>
    </div>

    <div v-if="loading" class="trace-state" role="status" aria-live="polite">
      <span class="trace-spinner" aria-hidden="true" />
      <span>正在读取调用明细，请稍候…</span>
    </div>
    <div v-else-if="errorMessage" class="trace-state trace-error" role="alert">
      <span>{{ errorMessage }}</span>
      <button class="trace-button" type="button" @click="refresh">重新加载</button>
    </div>
    <div v-else-if="items.length === 0" class="trace-state">
      还没有共享池调用。使用共享池 Key 发起一次请求后，这里会自动出现明细。
    </div>

    <div v-else class="trace-list">
      <article v-for="item in items" :key="item.id" class="trace-card">
        <header class="trace-card-head">
          <div>
            <div class="trace-title-line">
              <span :class="['trace-status', `is-${item.status}`]">{{ statusLabel(item) }}</span>
              <strong>{{ item.pool_name_snapshot || `共享池 #${item.pool_id}` }}</strong>
            </div>
            <p>{{ formatDateTime(item.created_at) }}</p>
          </div>
          <strong class="trace-total">{{ latencyText(item.total_latency_ms) }}</strong>
        </header>

        <div class="trace-route">
          <span><b>模型</b>{{ item.model || '未识别' }}</span>
          <span><b>接口</b>{{ endpointLabel(item.endpoint) }}</span>
          <span><b>线路</b>{{ item.account_alias || '共享线路' }}</span>
          <span><b>结算</b>{{ settlementLabel(item.settlement_outcome) }}</span>
        </div>

        <div class="trace-tokens" aria-label="Token 用量">
          <span>输入 <b>{{ formatNumber(item.input_tokens) }}</b></span>
          <span>输出 <b>{{ formatNumber(item.output_tokens) }}</b></span>
          <span>缓存读取 <b>{{ formatNumber(item.cache_read_tokens) }}</b></span>
          <span>缓存写入 <b>{{ formatNumber(item.cache_creation_tokens) }}</b></span>
          <span>合计 <b>{{ formatNumber(totalTokens(item)) }}</b></span>
        </div>

        <div class="trace-timeline" aria-label="请求各阶段耗时">
          <div><span>身份校验</span><b>{{ latencyText(item.auth_latency_ms, '未单独记录') }}</b></div>
          <div><span>席位校验</span><b>{{ latencyText(item.seat_latency_ms, '随选路完成') }}</b></div>
          <div><span>选路</span><b>{{ latencyText(item.routing_latency_ms) }}</b></div>
          <div><span>等待并发</span><b>{{ latencyText(item.concurrency_latency_ms) }}</b></div>
          <div><span>费用预留</span><b>{{ latencyText(item.reservation_latency_ms) }}</b></div>
          <div><span>上游处理</span><b>{{ latencyText(item.upstream_latency_ms) }}</b></div>
          <div><span>首字返回</span><b>{{ latencyText(item.first_token_ms, item.upstream_started ? '未返回首字' : '未发往上游') }}</b></div>
          <div><span>费用结算</span><b>{{ latencyText(item.settlement_latency_ms) }}</b></div>
        </div>

        <p v-if="item.status === 'failed'" class="trace-explain trace-explain-error">
          {{ failureExplanation(item) }}
        </p>
        <p v-else-if="item.status === 'pending'" class="trace-explain">
          这条请求仍在处理中，或上次服务中断后尚未补齐结果。余额以消费账本为准；长时间不变时可复制请求编号联系客服。
        </p>
        <p v-else class="trace-explain">
          请求已完成，共使用 {{ formatNumber(totalTokens(item)) }} Token<span v-if="item.retry_count > 0">，结算重试 {{ item.retry_count }} 次</span>。
        </p>

        <footer class="trace-footer">
          <code :title="item.request_id">请求编号：{{ shortRequestID(item.request_id) }}</code>
          <span v-if="item.http_status">HTTP {{ item.http_status }}</span>
          <button class="trace-copy" type="button" @click="copyRequestID(item.request_id)">
            {{ copiedRequestID === item.request_id ? '已复制' : '复制编号' }}
          </button>
        </footer>
      </article>

      <button v-if="hasMore" class="trace-more" type="button" :disabled="loadingMore" @click="loadMore">
        {{ loadingMore ? '正在加载…' : '加载更早的调用' }}
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  listMySharedPoolUsageTraces,
  type SharedPoolSettlementOutcome,
  type SharedPoolUsageTrace,
} from '@/features/bizdecipher/api/sharedPoolUsageTrace'

const items = ref<SharedPoolUsageTrace[]>([])
const loading = ref(true)
const refreshing = ref(false)
const loadingMore = ref(false)
const errorMessage = ref('')
const hasMore = ref(false)
const nextBeforeID = ref<number | undefined>()
const copiedRequestID = ref('')

async function fetchFirstPage(): Promise<void> {
  const page = await listMySharedPoolUsageTraces({ limit: 20 })
  items.value = page.items ?? []
  hasMore.value = Boolean(page.has_more)
  nextBeforeID.value = page.next_before_id
}

async function refresh(): Promise<void> {
  refreshing.value = true
  errorMessage.value = ''
  try {
    await fetchFirstPage()
  } catch {
    errorMessage.value = '调用明细暂时没有加载成功。你的账本和余额不受影响，请稍后重试。'
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function loadMore(): Promise<void> {
  if (!hasMore.value || loadingMore.value || !nextBeforeID.value) return
  loadingMore.value = true
  errorMessage.value = ''
  try {
    const page = await listMySharedPoolUsageTraces({ beforeId: nextBeforeID.value, limit: 20 })
    items.value.push(...(page.items ?? []))
    hasMore.value = Boolean(page.has_more)
    nextBeforeID.value = page.next_before_id
  } catch {
    errorMessage.value = '更早的调用暂时没有加载成功，请稍后重试。'
  } finally {
    loadingMore.value = false
  }
}

function statusLabel(item: SharedPoolUsageTrace): string {
  if (item.status === 'succeeded') return '调用成功'
  if (item.status === 'pending') return '处理中'
  return item.failure_stage === 'settlement' ? '结算待处理' : '调用失败'
}

function settlementLabel(value: SharedPoolSettlementOutcome): string {
  switch (value) {
    case 'settled': return '已结算'
    case 'released': return '已释放预留'
    case 'failed': return '待系统重试'
    case 'not_required': return '无需结算'
    case 'pending': return '处理中'
    default: return '待确认'
  }
}

function failureExplanation(item: SharedPoolUsageTrace): string {
  const stage: Record<string, string> = {
    auth: '身份校验没有通过',
    seat: '席位校验没有通过',
    routing: '暂时没有找到可用线路',
    concurrency: '当时请求较多，未获得并发名额',
    reservation: '费用预留没有完成，上游没有收到请求',
    upstream: item.usage_observed ? '上游返回失败，但已经报告了部分用量' : '上游处理失败，没有可结算的用量',
    settlement: '上游已返回，但费用结算仍需系统重试',
    cancelled: '请求已取消',
  }
  return `${stage[item.failure_stage || ''] || '请求没有正常完成'}。${settlementLabel(item.settlement_outcome)}，最终余额请以消费账本为准。`
}

function endpointLabel(endpoint: string): string {
  if (endpoint === '/v1/chat/completions') return '聊天补全'
  if (endpoint === '/v1/responses') return 'Responses'
  return endpoint || '未识别'
}

function latencyText(value?: number, empty = '未记录'): string {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return empty
  if (value < 1000) return `${Math.round(value)} ms`
  return `${(value / 1000).toFixed(value < 10_000 ? 2 : 1)} 秒`
}

function totalTokens(item: SharedPoolUsageTrace): number {
  return Math.max(0, item.input_tokens || 0)
    + Math.max(0, item.output_tokens || 0)
    + Math.max(0, item.cache_read_tokens || 0)
    + Math.max(0, item.cache_creation_tokens || 0)
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat('zh-CN').format(Math.max(0, value || 0))
}

function formatDateTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value || '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

function shortRequestID(value: string): string {
  if (!value) return '-'
  if (value.length <= 24) return value
  return `${value.slice(0, 12)}…${value.slice(-8)}`
}

async function copyRequestID(value: string): Promise<void> {
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
    copiedRequestID.value = value
    window.setTimeout(() => {
      if (copiedRequestID.value === value) copiedRequestID.value = ''
    }, 1800)
  } catch {
    copiedRequestID.value = ''
  }
}

onMounted(async () => {
  try {
    await fetchFirstPage()
  } catch {
    errorMessage.value = '调用明细暂时没有加载成功。你的账本和余额不受影响，请稍后重试。'
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.trace-panel { margin: 0 0 16px; padding: 16px; border: 1px solid color-mix(in srgb, var(--teal) 25%, var(--line)); border-radius: 16px; background: color-mix(in srgb, var(--teal) 4%, var(--card)); }
.trace-head, .trace-card-head, .trace-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.trace-head h3 { margin: 0; font-size: 16px; }
.trace-head p, .trace-card-head p { margin: 4px 0 0; color: var(--muted); font-size: 12px; line-height: 1.6; }
.trace-button, .trace-more, .trace-copy { border: 1px solid var(--line); border-radius: 10px; background: var(--card); color: var(--text); cursor: pointer; font-weight: 800; }
.trace-button { flex: 0 0 auto; padding: 8px 12px; }
.trace-button:disabled, .trace-more:disabled { cursor: wait; opacity: .6; }
.trace-state { display: flex; min-height: 90px; align-items: center; justify-content: center; gap: 10px; margin-top: 12px; border: 1px dashed var(--line); border-radius: 12px; color: var(--muted); font-size: 13px; text-align: center; }
.trace-error { color: var(--danger); }
.trace-spinner { width: 18px; height: 18px; border: 2px solid var(--line); border-top-color: var(--teal); border-radius: 999px; animation: trace-spin .8s linear infinite; }
.trace-list { display: grid; gap: 12px; margin-top: 14px; }
.trace-card { padding: 14px; border: 1px solid var(--line); border-radius: 14px; background: var(--card); }
.trace-title-line { display: flex; align-items: center; gap: 8px; }
.trace-title-line strong { font-size: 14px; }
.trace-status { padding: 3px 8px; border-radius: 999px; font-size: 11px; font-weight: 900; }
.trace-status.is-succeeded { background: color-mix(in srgb, var(--teal) 14%, transparent); color: var(--teal); }
.trace-status.is-failed { background: color-mix(in srgb, var(--danger) 12%, transparent); color: var(--danger); }
.trace-status.is-pending { background: color-mix(in srgb, #d97706 14%, transparent); color: #b45309; }
.trace-total { font-size: 15px; color: var(--teal); white-space: nowrap; }
.trace-route, .trace-tokens { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 12px; }
.trace-route span, .trace-tokens span { padding: 6px 9px; border-radius: 9px; background: color-mix(in srgb, var(--line) 28%, transparent); color: var(--muted); font-size: 12px; }
.trace-route b { margin-right: 5px; color: var(--text); }
.trace-tokens b { margin-left: 4px; color: var(--text); }
.trace-timeline { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; margin-top: 12px; }
.trace-timeline div { padding: 8px; border: 1px solid var(--line); border-radius: 9px; }
.trace-timeline span, .trace-timeline b { display: block; }
.trace-timeline span { color: var(--muted); font-size: 11px; }
.trace-timeline b { margin-top: 3px; font-size: 12px; }
.trace-explain { margin: 12px 0 0; padding: 9px 10px; border-radius: 9px; background: color-mix(in srgb, var(--teal) 7%, transparent); color: var(--muted); font-size: 12px; line-height: 1.6; }
.trace-explain-error { background: color-mix(in srgb, var(--danger) 7%, transparent); color: var(--danger); }
.trace-footer { margin-top: 12px; color: var(--muted); font-size: 11px; }
.trace-footer code { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.trace-copy { padding: 5px 8px; font-size: 11px; }
.trace-more { width: 100%; padding: 10px; }
@keyframes trace-spin { to { transform: rotate(360deg); } }
@media (max-width: 760px) {
  .trace-head, .trace-card-head { align-items: flex-start; }
  .trace-timeline { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .trace-footer { align-items: flex-start; flex-wrap: wrap; }
}
@media (prefers-reduced-motion: reduce) { .trace-spinner { animation: none; } }
</style>
