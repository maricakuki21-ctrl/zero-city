<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import { RefreshCw } from '@lucide/vue'
import { adminInspectSharedPoolPricing, type SharedPoolModelEndpointPricing } from '@/features/bizdecipher/api/bizdecipher'
import { sharedPoolEndpointShortLabel, sharedPoolEndpointHasCompatiblePrice } from '@/features/bizdecipher/components/shared-pool/sharedPoolPricing'

const props = defineProps<{ poolId: number }>()
const rows = ref<SharedPoolModelEndpointPricing[]>([])
const loading = ref(false)
const error = ref('')
let generation = 0
async function load() {
  const current = ++generation
  const poolId = props.poolId
  loading.value = true
  error.value = ''
  rows.value = []
  try {
    const result = await adminInspectSharedPoolPricing(poolId)
    if (current === generation) rows.value = result
  } catch {
    if (current === generation) error.value = '端点记录读取失败，请重试。'
  } finally {
    if (current === generation) loading.value = false
  }
}
function gateLabel(gate: string) {
  return ({ passed: '通过', failed: '失败', stale: '已过期', unverified: '未验证' } as Record<string, string>)[gate] || '未提供端点检测'
}
watch(() => props.poolId, load, { immediate: true })
onBeforeUnmount(() => { generation += 1 })
</script>

<template>
  <section class="endpoint-inspection" aria-label="模型端点记录">
    <header><h3>模型与端点</h3><button type="button" title="刷新端点记录" aria-label="刷新端点记录" :disabled="loading" @click="load"><RefreshCw :size="15" /></button></header>
    <p v-if="loading" role="status">正在读取端点记录…</p>
    <p v-else-if="error" role="alert">{{ error }}</p>
    <p v-else-if="!rows.length">该池尚无端点记录。</p>
    <div v-else class="endpoint-scroll">
      <table><thead><tr><th>供应商 / 模型</th><th>端点</th><th>开放</th><th>价格</th><th>端点检测</th></tr></thead>
        <tbody><tr v-for="row in rows" :key="`${row.pool_model_id}:${row.endpoint_id}`">
          <td>{{ row.display_name || row.model_name }}<small>{{ row.provider }} · {{ row.model_name }}</small></td>
          <td>{{ sharedPoolEndpointShortLabel(row.endpoint_type) }}</td>
          <td>{{ row.enabled ? '已开放' : '关闭' }}</td>
          <td>{{ sharedPoolEndpointHasCompatiblePrice(row) ? '报价已匹配' : '待核价' }}<small>{{ row.current_price?.base_price.currency || '币种未提供' }} · {{ row.pricing_source === 'official_catalog' ? '目录价' : '池主定价' }}</small></td>
          <td>{{ gateLabel(row.gate_status) }}</td>
        </tr></tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
.endpoint-inspection{padding:20px 0;border-bottom:1px solid var(--bd-ui-line);min-width:0}
header{display:flex;align-items:center;justify-content:space-between;margin-bottom:12px}h3{font-size:14px;font-weight:600}header button{display:grid;place-items:center;width:32px;height:32px;border:1px solid var(--bd-ui-line);border-radius:6px}
p{font-size:12px;color:var(--bd-text-secondary)}p[role=alert]{color:var(--bd-status-danger)}.endpoint-scroll{max-width:100%;overflow:auto}
table{width:100%;min-width:600px;border-collapse:collapse;text-align:left;font-size:12px}th,td{padding:10px 8px;border-bottom:1px solid var(--bd-ui-line);vertical-align:top}th{font-weight:500;color:var(--bd-text-secondary)}
td:first-child{max-width:260px;overflow-wrap:anywhere}small{display:block;font-size:11px;color:var(--bd-text-secondary);margin-top:4px}button:focus-visible{outline:2px solid var(--bd-accent-teal)}
</style>
