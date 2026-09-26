<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { Play, Save, RefreshCw, Download } from '@lucide/vue'
import { apiClient } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { extractActionableApiErrorMessage, extractApiErrorCode } from '@/utils/apiError'

const props = defineProps<{ assetId: number; version: string }>()
const auth = useAuthStore()
interface Plan { plan_digest: string; package_digest: string; prompt: string; permissions: string[]; steps?: { action: string; prompt?: string; module_file?: string }[] }
interface Workflow { request_id: string; state: string; total: number; steps: { index: number; action: string; state: string; run_id?: string; output?: string }[]; output?: string; error?: string }
interface Resource { id: string; version: string; digest: string; title: string; protocol: string; canonical_model_id: string; canonical_model_version: string; accepted_quote_id: string; accepted_quote_sha: string; estimate?: { currency: string; amount: string } }
interface Run { id: string; state: string; artifacts: { artifact_id: string }[]; input?: { intent: string }; failure?: { message: string } }
interface Snapshot { id: string; run_id: string; label: string; input?: { intent: string } }
const plan = ref<Plan | null>(null)
const resources = ref<Resource[]>([])
const selected = ref('')
const input = ref('')
const confirmed = ref(false)
const renewConfirmed = ref(false)
const busy = ref(false)
const error = ref('')
const run = ref<Run | null>(null)
const output = ref('')
const saved = ref(false)
const pending = ref<Record<string, unknown> | null>(null)
const snapshots = ref<Snapshot[]>([])
const workflow = ref<Workflow | null>(null)
const completedRequestId = ref('')
const workflowHistory = ref<Workflow[]>([])
const needsModel = computed(() => plan.value?.permissions.includes('model.generate') ?? true)
const isWorkflow = computed(() => (plan.value?.steps?.length ?? 1) > 1 || plan.value?.steps?.[0]?.action === 'plugin.wasm')
const pendingKey = () => `asset-run:${auth.user?.id}:${props.assetId}:${props.version}`
let pendingScope = ''
let epoch = 0
let timer: ReturnType<typeof setTimeout> | undefined
const root = () => `/biz/assets/${props.assetId}/versions/${encodeURIComponent(props.version)}/execution`
async function load() {
  const generation = ++epoch
  clearTimeout(timer)
  plan.value = null
  run.value = null
  output.value = ''
  confirmed.value = false
  renewConfirmed.value = false
  saved.value = false
  error.value = ''
  snapshots.value = []
  workflow.value = null
  workflowHistory.value = []
  if (pendingScope !== pendingKey()) {
    pendingScope = pendingKey()
    pending.value = null
    completedRequestId.value = ''
  }
  if (!props.version || !auth.user) return
  try {
    if (!pending.value) {
      const stored = JSON.parse(sessionStorage.getItem(pendingKey()) || 'null')
      if (typeof stored?.request_id === 'string' && typeof stored?.plan_digest === 'string') pending.value = stored
    }
  } catch { /* A blocked session store does not prevent an in-memory retry. */ }
  try {
    const [p, workspace] = await Promise.all([
      apiClient.get<Plan>(root()),
      apiClient.get<{ capabilities: Resource[]; current_run?: Run; saved_snapshots?: Snapshot[] }>('/biz/workbench/workspace').catch(() => ({ data: { capabilities: [] as Resource[], current_run: undefined, saved_snapshots: [] as Snapshot[] } })),
    ])
    if (generation !== epoch) return
    plan.value = p.data
    resources.value = workspace.data.capabilities.filter(r => ['chat', 'responses'].includes(r.protocol) && r.accepted_quote_id && r.accepted_quote_sha)
    selected.value = resources.value[0]?.id || ''
    if (isWorkflow.value) {
      try {
        const history = await apiClient.get<Workflow[]>(`${root()}?history=1`)
        if (generation !== epoch) return
        workflowHistory.value = Array.isArray(history.data) ? history.data : []
      } catch { /* The current request remains recoverable independently. */ }
      let requestID = typeof pending.value?.request_id === 'string' ? pending.value.request_id : completedRequestId.value
      try { requestID ||= sessionStorage.getItem(`${pendingKey()}:completed`) || '' } catch { /* Optional history. */ }
      if (requestID) {
        try {
          const status = await apiClient.get<Workflow>(`${root()}?request_id=${encodeURIComponent(String(requestID))}`)
          if (generation !== epoch) return
          workflow.value = status.data
          output.value = status.data.output || ''
          if (['succeeded', 'failed'].includes(status.data.state)) {
            completedRequestId.value = status.data.request_id
            pending.value = null
            if (status.data.error) error.value = status.data.error
            try { sessionStorage.removeItem(pendingKey()) } catch { /* In-memory terminal state is authoritative. */ }
          }
        } catch { /* A failed first dispatch may not yet have a checkpoint. */ }
      }
      return
    }
    snapshots.value = (workspace.data.saved_snapshots ?? []).filter(s =>
      s.input?.intent.startsWith('[asset-declarative/v1]') && s.input.intent.includes(`plan=${p.data.plan_digest}\n`))
    const currentRun = workspace.data.current_run
    if (currentRun?.input?.intent.startsWith('[asset-declarative/v1]') && currentRun.input.intent.includes(`plan=${p.data.plan_digest}\n`)) {
      run.value = currentRun
      await readRun(currentRun.id, generation)
    }
  } catch (e) {
    if (generation === epoch) error.value = extractActionableApiErrorMessage(e, '该版本未提供可运行的声明式技能，或执行资源暂不可用')
  }
}
async function readRun(id: string, generation: number) {
  try {
    const result = await apiClient.get<{ run: Run }>(`/biz/workbench/runs/${encodeURIComponent(id)}`)
    if (generation !== epoch) return
    run.value = result.data.run
    if (run.value.state === 'succeeded') {
      const parts = await Promise.all(run.value.artifacts.map(a => apiClient.get<string>(`/biz/workbench/runs/${encodeURIComponent(id)}/artifacts/${encodeURIComponent(a.artifact_id)}/content`, { responseType: 'text' })))
      if (generation === epoch) output.value = parts.map(p => typeof p.data === 'string' ? p.data : JSON.stringify(p.data)).join('\n\n')
    } else if (!['failed', 'cancelled'].includes(run.value.state)) {
      timer = setTimeout(() => void readRun(id, generation), 1500)
    } else error.value = run.value.failure?.message || '运行已终止'
  } catch (e) {
    if (generation === epoch) error.value = extractActionableApiErrorMessage(e, '运行状态读取失败')
  }
}
async function launch() {
  const resource = resources.value.find(r => r.id === selected.value)
  if (busy.value || (!pending.value && ((!resource && needsModel.value) || !plan.value || !confirmed.value))) return
  const generation = epoch
  const storageKey = pendingKey()
  const endpoint = root()
  busy.value = true
  error.value = ''
  if (!pending.value && plan.value) pending.value = {
    plan_digest: plan.value.plan_digest, permissions: plan.value.permissions, intent: input.value,
    capability_id: resource?.id || '', capability_version: resource?.version || '', capability_digest: resource?.digest || '',
    canonical_model_id: resource?.canonical_model_id || '', canonical_model_version: resource?.canonical_model_version || '',
    accepted_quote_id: resource?.accepted_quote_id || '', accepted_quote_sha: resource?.accepted_quote_sha || '',
    request_id: crypto.randomUUID(),
  }
  try { sessionStorage.setItem(storageKey, JSON.stringify(pending.value)) } catch { /* Keep the immutable request in memory. */ }
  try {
    let result = await apiClient.post<{ run?: Run; workflow?: Workflow }>(endpoint, pending.value, { timeout: 180000 })
    if (generation !== epoch) return
    while (result.data.workflow) {
      const status = result.data.workflow
      workflow.value = status
      output.value = status.output || ''
      if (['succeeded', 'failed'].includes(status.state)) {
        completedRequestId.value = status.request_id
        workflowHistory.value = [status, ...workflowHistory.value.filter(item => item.request_id !== status.request_id)].slice(0, 20)
        try {
          sessionStorage.setItem(`${storageKey}:completed`, status.request_id)
          sessionStorage.removeItem(storageKey)
        } catch { /* The result is already persisted server-side. */ }
        pending.value = null
        confirmed.value = false
        if (status.error) error.value = status.error
        return
      }
      const completed = status.steps.length
      result = await apiClient.post(endpoint, pending.value, { timeout: 180000 })
      if (generation !== epoch) return
      if (result.data.workflow?.state === 'running' && result.data.workflow.steps.length === completed) {
        workflow.value = result.data.workflow
        timer = setTimeout(() => { if (generation === epoch) void launch() }, 2000)
        return
      }
    }
    if (!result.data.run) throw new Error('运行响应缺少结果')
    try { sessionStorage.removeItem(storageKey) } catch { /* The response has already confirmed the run. */ }
    pending.value = null
    run.value = result.data.run
    confirmed.value = false
    saved.value = false
    await readRun(run.value.id, generation)
  } catch (e) {
    if (generation === epoch) {
      const code = extractApiErrorCode(e)
      if (!isWorkflow.value && code && ['WORKBENCH_CATALOG_UNAVAILABLE', 'WORKBENCH_CATALOG_CHANGED', 'ASSET_COMMERCE_CONFLICT', 'ASSET_COMMERCE_FORBIDDEN', 'ASSET_COMMERCE_INVALID'].includes(code)) {
        pending.value = null
        confirmed.value = false
        try { sessionStorage.removeItem(storageKey) } catch { /* The pre-dispatch rejection is conclusive. */ }
      }
      error.value = extractActionableApiErrorMessage(e, '请求尚未确认，请重试原请求')
    }
  } finally { if (generation === epoch) busy.value = false }
}
async function renewAuthorization() {
  if (!pending.value || busy.value || !renewConfirmed.value) return
  const generation = epoch
  busy.value = true
  let renewed = false
  try {
    const response = await apiClient.get<{ capabilities: Resource[] }>('/biz/workbench/workspace')
    if (generation !== epoch || !pending.value) return
    const original = pending.value
    const resource = response.data.capabilities.find(r =>
      r.id === original.capability_id && r.version === original.capability_version &&
      r.digest === original.capability_digest && r.canonical_model_id === original.canonical_model_id &&
      r.canonical_model_version === original.canonical_model_version)
    if (!resource || resource.version !== 'native-metered-v1') throw new Error('原密钥或模型暂不可用，工作流与已完成结果已保留；不会自动切换付费资源。')
    pending.value = { ...original, accepted_quote_id: resource.accepted_quote_id, accepted_quote_sha: resource.accepted_quote_sha }
    try { sessionStorage.setItem(pendingKey(), JSON.stringify(pending.value)) } catch { /* Keep renewed request in memory. */ }
    renewConfirmed.value = false
    renewed = true
  } catch (e) {
    if (generation === epoch) error.value = extractActionableApiErrorMessage(e, '资源授权刷新失败')
  } finally { if (generation === epoch) busy.value = false }
  if (renewed && generation === epoch) await launch()
}
function downloadOutput() {
  const url = URL.createObjectURL(new Blob([output.value], { type: 'text/plain;charset=utf-8' }))
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = `asset-${props.assetId}-${props.version}.txt`
  anchor.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
async function readWorkflow(requestID: string) {
  if (busy.value) return
  const generation = epoch
  busy.value = true
  try {
    const response = await apiClient.get<Workflow>(`${root()}?request_id=${encodeURIComponent(requestID)}`)
    if (generation !== epoch) return
    workflow.value = response.data
    output.value = response.data.output || ''
    error.value = response.data.error || ''
  } catch (e) {
    if (generation === epoch) error.value = extractActionableApiErrorMessage(e, '历史结果读取失败')
  } finally { if (generation === epoch) busy.value = false }
}
async function save() {
  if (!run.value || busy.value) return
  busy.value = true
  const generation = epoch
  try {
    const result = await apiClient.post<{ snapshot?: Snapshot }>(`/biz/workbench/runs/${encodeURIComponent(run.value.id)}/save`, { label: `资产 ${props.assetId} ${props.version}` }, { headers: { 'Idempotency-Key': `asset-save-${run.value.id}` } })
    if (generation === epoch) {
      saved.value = true
      if (result.data.snapshot && !snapshots.value.some(s => s.id === result.data.snapshot!.id)) snapshots.value.unshift(result.data.snapshot)
    }
  } catch (e) { if (generation === epoch) error.value = extractActionableApiErrorMessage(e, '保存失败') }
  finally { if (generation === epoch) busy.value = false }
}
watch(() => [props.assetId, props.version, auth.user?.id], () => { busy.value = false; void load() }, { immediate: true })
watch(selected, () => { confirmed.value = false })
onBeforeUnmount(() => { epoch++; clearTimeout(timer) })
</script>

<template>
  <section class="asset-execution" aria-label="声明式资产运行">
    <h4>运行技能 / 工作流</h4>
    <p v-if="error" role="alert">{{ error }} <button type="button" :disabled="busy" @click="load"><RefreshCw :size="14" />刷新</button></p>
    <template v-if="plan && !pending">
      <details><summary>固定版本 {{ version }} · {{ plan.steps?.length || 1 }} 步 · {{ needsModel ? '模型生成' : '隔离插件' }}</summary><code>{{ plan.package_digest }}</code><pre>{{ plan.prompt }}</pre><ol v-if="isWorkflow"><li v-for="(step, index) in plan.steps" :key="index">{{ index + 1 }}. {{ step.action === 'model.generate' ? '模型生成' : 'WASI 插件' }} {{ step.module_file }}</li></ol></details>
      <label v-if="needsModel">执行资源<select v-model="selected" :disabled="busy"><option v-for="r in resources" :key="r.id" :value="r.id">{{ r.title }} · {{ r.estimate ? `预估 ${r.estimate.currency} ${r.estimate.amount}` : '按量计费' }}</option></select></label>
      <label>输入<textarea v-model="input" maxlength="16000" rows="3" :disabled="busy" /></label>
      <label><input v-model="confirmed" type="checkbox" :disabled="busy" />{{ needsModel ? '允许使用所选密钥生成，按现行价格及实际用量结算，无固定金额封顶' : '允许此固定版本插件在隔离沙箱处理输入' }}</label>
      <button type="button" :disabled="busy || !confirmed || (needsModel && !selected) || !input.trim() || (!!run && ['queued', 'running', 'cancel_requested'].includes(run.state))" @click="launch"><Play :size="14" />运行</button>
    </template>
    <div v-if="pending"><p>上一笔运行尚待确认。</p><button type="button" :disabled="busy" @click="launch"><RefreshCw :size="14" />重试原请求</button></div>
    <div v-if="pending && isWorkflow && error && needsModel">
      <label><input v-model="renewConfirmed" type="checkbox" :disabled="busy" />继续使用原密钥和模型，后续步骤按现行价格及实际用量结算</label>
      <button type="button" :disabled="busy || !renewConfirmed" @click="renewAuthorization"><RefreshCw :size="14" />刷新资源授权并继续</button>
    </div>
    <div v-if="workflow" aria-label="工作流进度">
      <strong>{{ workflow.steps.filter(s => s.state === 'succeeded').length }} / {{ workflow.total }} · {{ workflow.state }}</strong>
      <ol><li v-for="step in workflow.steps" :key="step.index">{{ step.index + 1 }}. {{ step.action === 'model.generate' ? '模型生成' : 'WASI 插件' }} · {{ step.state }}</li></ol>
    </div>
    <div v-if="workflowHistory.length" aria-label="工作流历史">
      <h5>最近运行</h5>
      <button v-for="item in workflowHistory" :key="item.request_id" type="button" :disabled="busy" @click="readWorkflow(item.request_id)">{{ item.request_id.slice(-8) }} · {{ item.steps.length }}/{{ item.total }} · {{ item.state }}</button>
    </div>
    <div v-if="snapshots.length" class="saved-runs" aria-label="已保存的运行">
      <h5>已保存</h5>
      <button v-for="snapshot in snapshots" :key="snapshot.id" type="button" :disabled="busy" @click="saved = true; readRun(snapshot.run_id, epoch)"><Save :size="14" />{{ snapshot.label || snapshot.run_id }}</button>
    </div>
    <div v-if="run"><span>运行 {{ run.id }} · {{ run.state }}</span><button type="button" :disabled="busy" @click="readRun(run.id, epoch)"><RefreshCw :size="14" />读取结果</button></div>
    <pre v-if="output">{{ output }}</pre>
    <button v-if="output" type="button" @click="downloadOutput"><Download :size="14" />下载结果</button>
    <button v-if="run?.state === 'succeeded'" type="button" :disabled="busy || saved" @click="save"><Save :size="14" />{{ saved ? '已保存' : '保存运行结果' }}</button>
  </section>
</template>

<style scoped>
.asset-execution { display: grid; gap: 12px; border-top: 1px solid #9ca3af; padding-block: 12px; min-width: 0; }
h4 { margin: 0; font-size: 16px; }
label { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
select, textarea { max-width: 100%; min-width: 0; border: 1px solid #9ca3af; padding: 6px; border-radius: 4px; color: inherit; background: transparent; }
textarea { width: 100%; }
pre, code { white-space: pre-wrap; overflow-wrap: anywhere; max-height: 360px; overflow: auto; }
button { display: inline-flex; align-items: center; gap: 6px; padding: 6px; }
button:disabled { opacity: .5; }
[role=alert] { color: #b91c1c; }
</style>
