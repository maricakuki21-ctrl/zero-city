<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { apiClient } from '@/api/client'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'

type Capability = {
  id: string; version: string; digest: string; title: string; protocol: string
  canonical_model_id: string; canonical_model_version: string
  accepted_quote_id: string; accepted_quote_sha: string
  estimate?: { currency: string; amount: string }
}
const props = defineProps<{ roomId: number; throughTurn: number }>()
const emit = defineEmits<{ recorded: [] }>()
const capabilities = ref<Capability[]>([])
const selectedId = ref('')
const instruction = ref('')
const busy = ref(false)
const loading = ref(false)
const error = ref('')
const notice = ref('')
const consent = ref(false)
const pending = ref<Record<string, unknown> | null>(null)
const selected = computed(() => capabilities.value.find(item => item.id === selectedId.value))
const storageKey = computed(() => `tavern-ai-request:${props.roomId}`)

async function loadModels() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await apiClient.get<{ capabilities: Capability[] }>('/biz/workbench/workspace')
    capabilities.value = (data.capabilities ?? []).filter(item =>
      ['chat', 'responses'].includes(item.protocol) && item.accepted_quote_id && item.accepted_quote_sha,
    )
    if (!capabilities.value.some(item => item.id === selectedId.value)) selectedId.value = capabilities.value[0]?.id ?? ''
  } catch (cause) {
    error.value = extractApiErrorMessage(cause, '模型暂时无法读取')
  } finally {
    loading.value = false
  }
}

async function generate() {
  if (busy.value || (!pending.value && (!selected.value || !consent.value))) return
  const model = selected.value
  if (!pending.value && model) {
    pending.value = {
      capability_id: model.id, capability_version: model.version, capability_digest: model.digest,
      canonical_model_id: model.canonical_model_id, canonical_model_version: model.canonical_model_version,
      accepted_quote_id: model.accepted_quote_id, accepted_quote_sha: model.accepted_quote_sha,
      through_turn: props.throughTurn, intent: instruction.value.trim() || '继续当前场景',
      request_id: crypto.randomUUID(),
    }
    try { sessionStorage.setItem(storageKey.value, JSON.stringify(pending.value)) } catch { /* In-memory retries remain stable. */ }
  }
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const { data } = await apiClient.post<{ recorded: boolean; message?: string; run: { state: string; failure?: { message: string } } }>(
      `/biz/tavern/rooms/${props.roomId}/ai-turns`, pending.value, { timeout: 180000 },
    )
    if (data.recorded) {
      notice.value = '主持回合已写入房间。'
      emit('recorded')
      clearPending()
      instruction.value = ''
      consent.value = false
    } else {
      notice.value = data.run.failure?.message || data.message || `运行状态：${data.run.state}`
      if (['failed', 'cancelled'].includes(data.run.state)) {
        clearPending()
        consent.value = false
      }
    }
  } catch (cause) {
    if (['WORKBENCH_CATALOG_UNAVAILABLE', 'WORKBENCH_CATALOG_CHANGED'].includes(extractApiErrorCode(cause) || '')) {
      clearPending()
      consent.value = false
    }
    error.value = extractApiErrorMessage(cause, '请求未确认，可重试同一请求')
  } finally {
    busy.value = false
  }
}
function clearPending() {
  pending.value = null
  try { sessionStorage.removeItem(storageKey.value) } catch { /* Storage may be unavailable. */ }
}
watch(selectedId, () => { consent.value = false })
onMounted(() => {
  try {
    const raw = sessionStorage.getItem(storageKey.value)
    if (raw) {
      const stored = JSON.parse(raw)
      if (typeof stored?.request_id === 'string') pending.value = stored
    }
  } catch { /* Ignore an invalid local draft. */ }
  void loadModels()
})
</script>

<template>
  <section class="tavern-ai-host" aria-labelledby="tavern-ai-title">
    <header><h3 id="tavern-ai-title">AI 主持</h3><button type="button" :disabled="loading || busy" @click="loadModels">刷新模型</button></header>
    <template v-if="!pending">
      <label>主持模型
        <select v-model="selectedId" :disabled="loading || busy">
          <option v-if="!capabilities.length" value="">{{ loading ? '读取中…' : '暂无可用模型资源' }}</option>
          <option v-for="model in capabilities" :key="model.id" :value="model.id">{{ model.title }}</option>
        </select>
      </label>
      <textarea v-model="instruction" rows="2" maxlength="1000" placeholder="让主持人推进情节、回应行动或给出线索" aria-label="主持要求" :disabled="busy" />
      <label class="ai-consent"><input v-model="consent" type="checkbox" :disabled="busy" />
        本次费用由我承担<span v-if="selected?.estimate">，预估 {{ selected.estimate.amount }} {{ selected.estimate.currency }}</span>，按所选密钥现行价格及实际用量结算，无固定金额封顶。
      </label>
    </template>
    <p v-else>有一笔主持请求等待确认。重试会沿用原请求，不新建收费任务。</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="notice" role="status">{{ notice }}</p>
    <footer>
      <button type="button" class="ai-generate" :disabled="busy || (!pending && (!selected || !consent))" @click="generate">{{ busy ? '主持生成中…' : pending ? '重试原请求' : '生成主持回合' }}</button>
      <RouterLink to="/operator">工作台</RouterLink>
    </footer>
  </section>
</template>

<style scoped>
.tavern-ai-host { display: grid; gap: 12px; padding: 20px 0; border-block: 1px solid var(--bd-ui-line, #ddd); }
header, footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
footer { justify-content: flex-start; }
h3, p { margin: 0; } p, label { font-size: 13px; line-height: 1.6; }
label { display: grid; gap: 6px; }
select, textarea { width: 100%; padding: 10px; border: 1px solid var(--bd-ui-line, #ddd); border-radius: 6px; background: var(--bd-surface, #fff); color: inherit; }
.ai-consent { display: block; } .ai-consent input { margin-right: 6px; }
button { min-height: 36px; padding: 6px 12px; border: 1px solid var(--bd-ui-line, #ddd); border-radius: 6px; }
.ai-generate { background: var(--bd-accent-teal, #16756c); color: white; } button:disabled { opacity: .5; }
</style>
