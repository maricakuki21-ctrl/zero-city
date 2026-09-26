<template>
  <div class="observation-strip" aria-label="官方资源检测历史">
    <div class="observation-head">
      <span>{{ monitor?.primary_model ? `检测 · ${monitor.primary_model}` : '模型观测' }}</span>
      <button v-if="error" type="button" @click="$emit('retry')">加载失败 · 重试</button>
      <span v-else :class="{ unavailable: monitor?.observation_mode === 'unavailable' }" :title="monitor?.observation_mode === 'unavailable' ? '当前没有可调度上游，请池主检查账号状态、额度或密钥' : undefined">{{ loading ? '加载中' : monitor?.observation_mode === 'unavailable' ? '上游不可用' : sampleCount ? `${sampleCount} 次样本` : monitor?.observation_mode === 'passive_media' ? '媒体待观测' : '待检测' }}</span>
    </div>
    <div class="observation-bars" aria-label="从左至右由早到晚，灰色表示没有样本">
      <span v-for="(sample, index) in samples" :key="index" :class="sample?.status || 'unknown'" :title="sample ? sampleTitle(sample) : '暂无检测样本'" />
    </div>
    <details class="observation-explanation" @click.stop>
      <summary>{{ validSamples.length ? `${passedCount}/${validSamples.length} 次通过 · ${(passedCount * 100 / validSamples.length).toFixed(1)}%` : '暂无有效检测' }} · 检测说明</summary>
      <p>绿：通过；黄：可用但较慢；红：检测失败；斜纹：检测系统异常；灰：无样本。系统异常不计入通过率，不代表上游失败。</p>
      <p>最近 {{ sampleCount }} 次样本，仅代表该检测模型，不是全天或全池可用率。</p>
      <ul><li v-for="(sample, index) in recentSamples" :key="index">{{ sampleTitle(sample) }}</li></ul>
    </details>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { UserMonitorView, MonitorTimelinePoint } from '@/api/channelMonitor'
import { observationSamples } from './officialObservation'

const props = defineProps<{ monitor?: UserMonitorView; loading?: boolean; error?: boolean }>()
defineEmits<{ retry: [] }>()
const samples = computed(() => observationSamples(props.monitor))
const sampleCount = computed(() => samples.value.filter(Boolean).length)
const recentSamples = computed(() => samples.value.filter((sample): sample is MonitorTimelinePoint => Boolean(sample)).reverse())
const validSamples = computed(() => recentSamples.value.filter(sample => ['operational', 'degraded', 'failed'].includes(sample.status)))
const passedCount = computed(() => validSamples.value.filter(sample => sample.status === 'operational' || sample.status === 'degraded').length)
function sampleTitle(sample: MonitorTimelinePoint): string {
  const label = { operational: '通过', degraded: '响应变慢', failed: '失败', error: '检测异常' }[sample.status] || '未知'
  const latency = sample.latency_ms == null ? '' : ` · ${sample.latency_ms}ms`
  return `${new Date(sample.checked_at).toLocaleString('zh-CN')} · ${label}${latency}`
}
</script>

<style scoped>
.observation-strip { padding-top: 0; color: var(--bd-text-secondary); font-size: 11px; }
.observation-head { display: flex; justify-content: space-between; gap: 8px; }
.observation-head > span:first-child { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }
.observation-head > :last-child { flex-shrink: 0; }
.observation-head button { color: var(--bd-accent-teal); }
.observation-head .unavailable { color: var(--bd-status-danger); }
.observation-head button:focus-visible { outline: 2px solid var(--bd-accent-teal); outline-offset: 2px; }
.observation-bars { display: grid; grid-template-columns: repeat(24, minmax(0, 1fr)); gap: 3px; margin-top: 7px; }
.observation-bars span { height: 14px; border-radius: 2px; background: var(--bd-ui-line); }
.observation-bars .operational { background: var(--bd-accent-teal); }
.observation-bars .degraded { background: var(--bd-accent-gold); }
.observation-bars .failed { background: var(--bd-status-danger); }
.observation-bars .error { background: repeating-linear-gradient(135deg,var(--bd-text-secondary) 0 2px,var(--bd-ui-line) 2px 5px); }
.observation-explanation{margin-top:6px;font-size:10px;line-height:1.7}
.observation-explanation summary{cursor:pointer}.observation-explanation summary:focus-visible{outline:2px solid var(--bd-accent-teal)}
.observation-explanation p{margin:6px 0}.observation-explanation ul{max-height:150px;overflow:auto;padding-left:16px;margin:0}
</style>
