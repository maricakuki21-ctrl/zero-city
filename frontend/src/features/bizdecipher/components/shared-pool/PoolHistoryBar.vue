<template>
  <div ref="element" class="pool-history" :aria-label="description">
    <div class="pool-history-label">
      <span :title="modelLabel">{{ error ? '检测记录加载失败' : modelLabel ? `检测 · ${modelLabel}` : '最近检测' }}</span>
      <button
        v-if="error"
        type="button"
        data-testid="pool-history-retry"
        :disabled="loading"
        @click.stop.prevent="loadHistory(true)"
      >重试</button>
      <small v-else>{{ observation.count ? `${observation.passed}/${observation.count} 次通过 · ${passRate}%` : loading ? '加载中' : '待检测' }}</small>
    </div>
    <p v-if="observation.state === 'stale'" class="pool-history-notice" role="status">历史样本 · 超过 1 小时未更新，不代表当前状态</p>
    <p v-else-if="observation.smallSample" class="pool-history-notice">样本较少，仅供参考</p>
    <div class="pool-history-bars">
      <span v-for="(sample, index) in samples" :key="index" :class="sample ? sample.success ? 'healthy' : 'failed' : 'unknown'" :title="sample ? `${formatTime(sample.checked_at || sample.created_at)} · ${sample.model_name} · ${sample.success ? '通过' : '失败'} · ${sample.latency_ms}ms` : '无检测样本'" />
    </div>
    <div class="pool-history-label pool-history-footer"><small>较早</small><small>最近</small></div>
    <details class="pool-history-explanation" @click.stop>
      <summary>检测说明<span v-if="records.length"> · {{ formatTime(records.at(-1)!.checked_at || records.at(-1)!.created_at) }}</span></summary>
      <p>绿：检测通过；红：这次检测失败；灰：没有样本。最近 {{ records.length }} 次自动或手动检测，不是全天可用率，也不代表真实调用成功率或所有模型。</p>
      <p v-if="modelOptions.length > 1">包含多个模型，合计通过率不能代表其中每个模型。</p>
      <label v-if="modelOptions.length > 1" class="pool-history-model-filter">查看模型
        <select v-model="selectedModel" aria-label="筛选检测模型">
          <option value="">全部模型</option>
          <option v-for="model in modelOptions" :key="model" :value="model">{{ model }}</option>
        </select>
      </label>
      <p v-if="records.length">样本区间：{{ formatTime(records[0].checked_at || records[0].created_at) }} — {{ formatTime(records.at(-1)!.checked_at || records.at(-1)!.created_at) }}</p>
      <ul><li v-for="sample in detailRecords" :key="sample.id">{{ formatTime(sample.checked_at || sample.created_at) }} · {{ sample.model_name }} · {{ sample.success ? '通过' : '失败' }} · {{ sample.latency_ms }}ms</li></ul>
    </details>
  </div>
</template>
<script lang="ts">
import type { SharedPoolProbeHistory } from '@/features/bizdecipher/api/bizdecipher'

const historyCache = new Map<number, { records: SharedPoolProbeHistory[]; fetchedAt: number }>()
const HISTORY_CACHE_MS = 60_000
const pendingHistories = new Map<number, Promise<SharedPoolProbeHistory[]>>()
</script>
<script setup lang="ts">
import { computed, ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { useIntersectionObserver } from '@vueuse/core'
import { listSharedPoolProbeHistories } from '@/features/bizdecipher/api/bizdecipher'
import { summarizeProbeObservations, type ProbeObservation } from './probeObservation'

const props = defineProps<{ poolId: number }>()
const emit = defineEmits<{ observed: [summary: ProbeObservation] }>()
const element = ref<HTMLElement | null>(null)
const visible = ref(false)
const records = ref<SharedPoolProbeHistory[]>([])
const loading = ref(false)
const error = ref(false)
const now = ref(Date.now())
const selectedModel = ref('')
const observation = computed(() => summarizeProbeObservations(records.value, now.value, error.value))
let generation = 0
let refreshTimer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  refreshTimer = setInterval(() => {
    now.value = Date.now()
    if (visible.value && document.visibilityState === 'visible' && !loading.value) void loadHistory(true)
  }, 60000)
})
onBeforeUnmount(() => { clearInterval(refreshTimer); generation++ })
watch(observation, summary => emit('observed', summary), { immediate: true })
useIntersectionObserver(element, entries => { visible.value = entries.some(entry => entry.isIntersecting) }, { rootMargin: '100px' })

async function requestHistory(id: number): Promise<SharedPoolProbeHistory[]> {
  const pending = pendingHistories.get(id)
  if (pending) return pending

  const request = listSharedPoolProbeHistories(id, { limit: 24 })
    .then(data => [...data]
      .filter(row => {
        const time = Date.parse(row.checked_at || row.created_at)
        return typeof row.success === 'boolean' && Number.isFinite(time) && time <= Date.now()
      })
      .sort((a, b) => Date.parse(a.checked_at || a.created_at) - Date.parse(b.checked_at || b.created_at))
      .slice(-24))
  pendingHistories.set(id, request)
  try {
    const data = await request
    historyCache.set(id, { records: data, fetchedAt: Date.now() })
    if (historyCache.size > 200) historyCache.delete(historyCache.keys().next().value!)
    return data
  } finally {
    if (pendingHistories.get(id) === request) pendingHistories.delete(id)
  }
}

async function loadHistory(force = false): Promise<void> {
  now.value = Date.now()
  const id = props.poolId
  const cached = historyCache.get(id)
  if (!force && cached && Date.now() - cached.fetchedAt < HISTORY_CACHE_MS) {
    records.value = cached.records
    error.value = false
    return
  }

  const request = ++generation
  loading.value = true
  error.value = false
  try {
    const data = await requestHistory(id)
    if (request === generation && id === props.poolId) records.value = data
  } catch {
    if (request === generation && id === props.poolId) {
      error.value = true
      records.value = []
    }
  } finally {
    if (request === generation && id === props.poolId) loading.value = false
  }
}

watch([() => props.poolId, visible], async ([id, isVisible], previous) => {
  if (previous?.[0] !== id) {
    ++generation
    records.value = []
    loading.value = false
    error.value = false
    selectedModel.value = ''
  }
  if (!isVisible || error.value) return
  await loadHistory()
}, { immediate: true })
const samples = computed(() => Array.from({ length: 24 }, (_, i) => records.value[i - (24 - records.value.length)] ?? null))
const passRate = computed(() => observation.value.count ? (observation.value.passed * 100 / observation.value.count).toFixed(1) : '')
const modelOptions = computed(() => [...new Set(records.value.map(row => row.model_name).filter(Boolean))])
const modelLabel = computed(() => modelOptions.value.length > 1 ? `${modelOptions.value.length} 个模型合计` : modelOptions.value[0] || '')
const detailRecords = computed(() => [...records.value].reverse().filter(row => !selectedModel.value || row.model_name === selectedModel.value))
const description = computed(() => error.value ? '检测记录加载失败，请重试；灰色不代表检测通过。' : `最近${records.value.length}次检测，从左至右由早到晚；灰色为无样本。`)
function formatTime(value: string): string { return new Date(value).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }) }
</script>
<style scoped>
.pool-history { padding:10px 0; }
.pool-history-label { display:flex; justify-content:space-between; gap:8px; font-size:11px; color:var(--bd-text-secondary); }
.pool-history-label small { font-size:10px; }
.pool-history-label button { min-height:20px; padding:0; border:0; background:transparent; color:var(--bd-accent-teal); font-size:10px; cursor:pointer; }
.pool-history-label button:focus-visible { outline:2px solid var(--bd-accent-teal); outline-offset:2px; }
.pool-history-bars { display:grid; grid-template-columns:repeat(24,minmax(0,1fr)); gap:3px; margin-top:7px; }
.pool-history-bars span { height:18px; border-radius:2px; background:var(--bd-ui-line); }
.pool-history-bars .healthy { background:var(--bd-accent-teal); }
.pool-history-bars .failed { background:var(--bd-status-danger); }
.pool-history-footer { margin-top:4px; }
.pool-history-explanation{margin-top:6px;color:var(--bd-text-secondary);font-size:10px;line-height:1.7}
.pool-history-explanation summary{cursor:pointer}.pool-history-explanation summary:focus-visible{outline:2px solid var(--bd-accent-teal)}
.pool-history-explanation p{margin:6px 0}.pool-history-explanation ul{max-height:150px;overflow:auto;padding-left:16px;margin:0}
.pool-history-notice{margin:4px 0 0;color:var(--bd-text-secondary);font-size:10px;line-height:1.5}
.pool-history-model-filter{display:flex;align-items:center;gap:8px;margin:6px 0}
.pool-history-model-filter select{min-width:0;max-width:100%;border:1px solid var(--bd-ui-line);border-radius:5px;padding:3px 6px;background:var(--bd-bg-surface, transparent);color:inherit;font:inherit}
</style>
