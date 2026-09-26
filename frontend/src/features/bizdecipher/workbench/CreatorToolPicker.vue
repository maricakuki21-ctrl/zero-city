<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ArrowLeft, Search, RefreshCw, Blocks } from '@lucide/vue'
import { listCapabilityAssets, listCapabilityAssetVersions, type CapabilityAsset, type CapabilityAssetVersion } from '../api/bizdecipher'
import AssetExecutionPanel from '../components/assets/AssetExecutionPanel.vue'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
const query = ref('')
const items = ref<CapabilityAsset[]>([])
const selected = ref<CapabilityAsset | null>(null)
const versions = ref<CapabilityAssetVersion[]>([])
const version = ref('')
const loading = ref(false)
const versionLoading = ref(false)
const error = ref('')
let generation = 0
let selectionGeneration = 0
let timer: ReturnType<typeof setTimeout> | undefined
async function load() {
  const current = ++generation
  loading.value = true; error.value = ''
  try {
    const result = await listCapabilityAssets({ keyword: query.value, sort: 'popular', limit: 50 })
    if (current === generation) items.value = result
  } catch (cause) {
    if (current === generation) { items.value = []; error.value = extractActionableApiErrorMessage(cause, '工具目录加载失败') }
  } finally { if (current === generation) loading.value = false }
}
async function open(item: CapabilityAsset) {
  const current = ++selectionGeneration
  selected.value = item; versions.value = []; version.value = ''; versionLoading.value = true; error.value = ''
  try {
    const result = await listCapabilityAssetVersions(item.id)
    if (current !== selectionGeneration) return
    versions.value = result.filter(v => v.status === 'published').sort((a, b) => Date.parse(b.published_at || b.created_at) - Date.parse(a.published_at || a.created_at))
    version.value = versions.value[0]?.version || ''
  } catch (cause) { if (current === selectionGeneration) error.value = extractActionableApiErrorMessage(cause, '工具版本加载失败') }
  finally { if (current === selectionGeneration) versionLoading.value = false }
}
function back() { selectionGeneration++; selected.value = null; version.value = ''; error.value = '' }
watch(query, () => { clearTimeout(timer); timer = setTimeout(load, 250) })
onMounted(load)
onBeforeUnmount(() => { generation++; selectionGeneration++; clearTimeout(timer) })
</script>

<template>
  <section class="creator-tools" aria-label="创作者工具">
    <template v-if="!selected">
      <div class="creator-search"><Search :size="15" /><input v-model="query" type="search" aria-label="搜索创作者工具" placeholder="搜索工具、工作流或作者" /><button type="button" :disabled="loading" title="刷新工具" aria-label="刷新工具" @click="load"><RefreshCw :size="15" /></button></div>
      <p v-if="loading" class="muted" role="status">正在读取创作者工具…</p>
      <div class="creator-results"><button v-for="item in items" :key="item.id" type="button" @click="open(item)"><Blocks :size="16" /><span><strong>{{ item.title }}</strong><small>{{ item.summary }}</small><small>{{ item.author || '创作者' }} · {{ item.asset_type }}</small></span><em>{{ item.pricing_type === 'free' || item.pricing_type === 'open_source' ? '免费' : item.pricing_type === 'paid' ? '付费' : '查看权益' }}</em></button></div>
      <p v-if="!loading && !items.length && !error" class="muted">暂无匹配内容</p>
      <p v-if="items.length === 50" class="muted">显示前 50 项，搜索可缩小范围。</p>
    </template>
    <template v-else>
      <button type="button" class="creator-back" @click="back"><ArrowLeft :size="15" />返回工具列表</button>
      <h3>{{ selected.title }}</h3><p class="muted">{{ selected.summary }}</p>
      <p v-if="versionLoading" class="muted" role="status">正在读取可用版本…</p>
      <label v-else-if="versions.length">版本<select v-model="version"><option v-for="item in versions" :key="item.id" :value="item.version">{{ item.version }}</option></select></label>
      <p v-else-if="!error" class="muted">此资产尚无已发布的运行版本。</p>
      <AssetExecutionPanel v-if="version" :key="`${selected.id}:${version}`" :asset-id="selected.id" :version="version" />
    </template>
    <p v-if="error" class="creator-error" role="alert">{{ error }}<button type="button" @click="selected ? open(selected) : load()">重试</button></p>
  </section>
</template>

<style scoped>
.creator-tools{padding-top:14px;margin-top:16px;border-top:1px solid var(--bd-ui-line);font-size:13px}
.creator-search{display:flex;align-items:center;gap:8px;border:1px solid var(--bd-ui-line);border-radius:6px;padding:8px}.creator-search input{flex:1;min-width:0;background:transparent}.creator-search button{width:28px;height:28px;display:grid;place-items:center}
.creator-results{max-height:300px;overflow:auto;margin-top:8px}.creator-results>button{display:flex;align-items:flex-start;text-align:left;gap:10px;width:100%;padding:12px 4px;border-bottom:1px solid var(--bd-ui-line)}.creator-results span{flex:1;min-width:0}.creator-results strong,.creator-results small{display:block;overflow-wrap:anywhere}.creator-results small{color:var(--bd-text-secondary);line-height:1.6;margin-top:4px}.creator-results em{font-size:11px;font-style:normal;white-space:nowrap}.creator-results svg{flex-shrink:0;margin-top:3px}.creator-back{display:flex;align-items:center;gap:6px;color:var(--bd-accent-teal);margin-bottom:14px}
.creator-tools h3{font-size:16px}.muted{font-size:12px;color:var(--bd-text-secondary);line-height:1.7;margin:10px 0}.creator-tools label{display:flex;gap:12px;align-items:center;margin:12px 0}.creator-tools select{padding:6px;border:1px solid var(--bd-ui-line);background:var(--bd-surface);border-radius:4px}.creator-error{color:var(--bd-status-danger)}.creator-error button{margin-left:12px;text-decoration:underline}
</style>
