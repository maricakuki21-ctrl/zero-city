<script setup lang="ts">
import { computed, ref } from 'vue'
import { Search, ChevronRight, RefreshCw, Layers, AlertCircle, Archive, MessageSquare, Cable, Activity, ReceiptText } from '@lucide/vue'
import type { SharedPool } from '@/features/bizdecipher/api/bizdecipher'
import { sharedPoolCompatibilitySummary, sharedPoolOpsBucket, type SharedPoolOpsBucket } from './sharedPoolGovernanceView'

const props = defineProps<{ pools: readonly SharedPool[]; loading: boolean; error: string; selectedId: number | null }>()
const emit = defineEmits<{ select: [pool: SharedPool]; refresh: [] }>()
const query = ref('')
const queue = ref<SharedPoolOpsBucket | 'all'>('all')
const queues = [
  { key: 'all', label: '全部资源', icon: Layers },
  { key: 'attention', label: '运营待处理', icon: AlertCircle },
  { key: 'supply', label: '资源待接入', icon: Cable },
  { key: 'pricing', label: '报价待核', icon: ReceiptText },
  { key: 'probe', label: '检测待核', icon: Activity },
  { key: 'community', label: '投诉与观察', icon: MessageSquare },
  { key: 'current', label: '已发布', icon: Layers },
  { key: 'archived', label: '历史归档', icon: Archive },
] as const
const classified = computed(() => props.pools.map(pool => ({ pool, bucket: sharedPoolOpsBucket(pool) })))
const visible = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  return classified.value.filter(({ pool, bucket }) =>
    (queue.value === 'all' || queue.value === bucket)
    && (!keyword || [pool.name, pool.owner_label, String(pool.id), ...pool.models].some(value => value.toLowerCase().includes(keyword))))
})
function count(key: string) {
  return classified.value.filter(row => key === 'all' || row.bucket === key).length
}
function label(key: SharedPoolOpsBucket) {
  return queues.find(item => item.key === key)?.label || key
}
</script>

<template>
  <section class="ops-workspace" aria-label="共享池运营列表">
    <nav class="ops-queues" aria-label="运营队列">
      <button v-for="item in queues" :key="item.key" type="button" :aria-pressed="queue === item.key"
        :class="{ active: queue === item.key }" @click="queue = item.key">
        <component :is="item.icon" :size="16" /><span>{{ item.label }}</span>
        <small>{{ loading || error ? '—' : count(item.key) }}</small>
      </button>
    </nav>
    <div class="ops-inventory">
      <div class="ops-toolbar">
        <label class="ops-search"><Search :size="16" /><input v-model="query" aria-label="搜索已载入的共享池" placeholder="搜索池名、池主、模型或编号" type="search" /></label>
        <button class="ops-refresh" title="刷新共享池" aria-label="刷新共享池" type="button" :disabled="loading" @click="emit('refresh')"><RefreshCw :size="16" /></button>
      </div>
      <p class="ops-scope">已载入 {{ loading ? '—' : pools.length }} 项 · 当前列表筛选{{ pools.length >= 100 ? ' · 已达 100 项读取上限' : '' }}</p>
      <div v-if="error" class="ops-error" role="alert">{{ error }}<button type="button" @click="emit('refresh')">重试</button></div>
      <p v-else-if="loading" role="status" class="ops-empty">正在读取共享池…</p>
      <div v-else-if="visible.length" class="ops-table-scroll">
        <table class="ops-table">
          <thead><tr><th>资源 / 池主</th><th>待办</th><th>检测与模型</th><th>成员</th><th><span class="sr-only">操作</span></th></tr></thead>
          <tbody><tr v-for="{ pool, bucket } in visible" :key="pool.id" :class="{ selected: selectedId === pool.id }">
            <td><button type="button" class="ops-pool-name" @click="emit('select', pool)">{{ pool.name }}</button><small>{{ pool.owner_label || '未设置昵称' }} · #{{ pool.id }}</small></td>
            <td><span class="ops-bucket" :data-tone="bucket">{{ label(bucket) }}</span></td>
            <td><span>{{ sharedPoolCompatibilitySummary(pool).label }}</span><small>{{ pool.last_probe_at ? new Date(pool.last_probe_at).toLocaleString('zh-CN') : '暂无检测记录' }}</small></td>
            <td>{{ pool.current_users ?? '—' }}</td>
            <td><button class="ops-refresh" type="button" :aria-label="`查看${pool.name}`" title="查看池详情" @click="emit('select', pool)"><ChevronRight :size="16" /></button></td>
          </tr></tbody>
        </table>
      </div>
      <p v-else class="ops-empty">当前筛选没有共享池</p>
    </div>
  </section>
</template>

<style scoped>
.ops-workspace{display:grid;grid-template-columns:168px minmax(0,1fr);gap:24px;color:var(--bd-text-primary)}
.ops-queues{display:flex;flex-direction:column;gap:4px;align-self:start}
.ops-queues button{display:flex;align-items:center;gap:8px;min-height:40px;text-align:left;padding:8px;border-radius:6px;font-size:12px;color:var(--bd-text-secondary);white-space:nowrap}
.ops-queues button small{margin-left:auto;font-variant-numeric:tabular-nums}.ops-queues button.active{background:var(--bd-surface-raised);color:var(--bd-accent-teal)}
.ops-inventory{min-width:0}.ops-toolbar{display:flex;gap:8px}.ops-search{display:flex;align-items:center;gap:8px;border:1px solid var(--bd-ui-line);border-radius:6px;background:var(--bd-surface);padding:0 12px;flex:1;min-width:0}
.ops-search input{min-height:40px;min-width:0;width:100%;background:transparent;font-size:13px;outline:none}.ops-search:focus-within{outline:2px solid var(--bd-accent-teal)}
.ops-refresh{display:inline-flex;align-items:center;justify-content:center;flex:0 0 36px;width:36px;height:36px;border:1px solid var(--bd-ui-line);border-radius:6px;background:var(--bd-surface)}
.ops-scope{font-size:11px;color:var(--bd-text-secondary);padding:10px 0}.ops-table-scroll{overflow:auto;max-width:100%}.ops-table{width:100%;min-width:630px;text-align:left;font-size:12px;border-collapse:collapse}
.ops-table th{font-weight:500;color:var(--bd-text-secondary);background:var(--bd-surface-raised);padding:10px}.ops-table td{padding:14px 10px;border-bottom:1px solid var(--bd-ui-line);vertical-align:middle}
.ops-table td:first-child{max-width:270px}.ops-table td:nth-child(3){max-width:260px}.ops-table small{display:block;margin-top:5px;color:var(--bd-text-secondary);font-size:11px}
.ops-pool-name{text-align:left;font-weight:600;overflow-wrap:anywhere}.ops-table tr.selected{background:var(--bd-surface-raised)}.ops-bucket{font-size:11px;white-space:nowrap;color:var(--bd-text-secondary)}
.ops-bucket[data-tone=attention],.ops-bucket[data-tone=community]{color:var(--bd-status-danger)}.ops-bucket[data-tone=pricing],.ops-bucket[data-tone=probe]{color:var(--bd-accent-gold)}.ops-bucket[data-tone=current]{color:var(--bd-accent-teal)}
.ops-empty{padding:40px 12px;text-align:center;color:var(--bd-text-secondary);font-size:13px}.ops-error{padding:16px 0;color:var(--bd-status-danger);font-size:13px}.ops-error button{margin-left:12px;text-decoration:underline}
button:focus-visible{outline:2px solid var(--bd-accent-teal);outline-offset:2px}button:disabled{opacity:.5}
@media(max-width:900px){.ops-workspace{grid-template-columns:minmax(0,1fr);gap:12px}.ops-queues{flex-direction:row;overflow:auto}.ops-queues button{flex:0 0 auto}.ops-queues button small{min-width:16px}}
</style>
