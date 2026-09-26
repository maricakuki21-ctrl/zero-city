<template>
  <dialog ref="dialog" class="resource-picker" aria-labelledby="resource-title" @close="handleClose" @cancel="preventBusyClose">
    <header><h2 id="resource-title">模型与资源</h2><button type="button" :disabled="busy || resourceBusy" aria-label="关闭资源" title="关闭" @click="dialog?.close()"><X :size="18" /></button></header>
    <nav class="slots" aria-label="模型用途"><button type="button" :disabled="busy || resourceBusy" :aria-pressed="slot === 'language'" :class="{ active: slot === 'language' }" @click="resetSlot('language')">规划模型</button><button type="button" :disabled="busy || resourceBusy" :aria-pressed="slot === 'image'" :class="{ active: slot === 'image' }" @click="resetSlot('image')">图片生成</button></nav>
    <ResourcePicker v-if="opened" :key="pickerRevision" v-model="resource" :disabled="busy" @busy="resourceBusy = $event" />
    <button v-if="resource?.ready" type="button" class="primary load-models" :disabled="busy || resourceBusy" @click="useResource">{{ busy ? '正在读取…' : '读取此资源的模型' }}</button>
    <details class="existing-connection">
      <summary>使用已有密钥</summary>
      <label>搜索我的密钥<input v-model="keyQuery" :disabled="busy || resourceBusy" type="search" placeholder="密钥名称" /></label>
      <p v-if="keyError" class="error" role="alert">{{ keyError }}<button type="button" @click="loadKeys()">重试</button></p>
      <div class="key-list">
        <button v-for="key in keys" :key="key.id" type="button" :disabled="busy || resourceBusy || keysLoading" @click="chooseKey(key.id, key.name)">
          <KeyRound :size="16" /><span>{{ key.name }}<small>{{ key.shared_pool_managed ? '共享资源' : key.group?.name || '未分组' }}</small></span><Check v-if="selectedId === key.id" :size="16" />
        </button>
        <p v-if="!keysLoading && !keys.length" class="muted">没有匹配的可用密钥</p>
      </div>
      <div class="key-pages"><button type="button" :disabled="keysLoading || keyPage <= 1 || busy || resourceBusy" @click="loadKeys(keyPage - 1)">上一页</button><span>{{ keyPage }} / {{ keyPages }}</span><button type="button" :disabled="keysLoading || keyPage >= keyPages || busy || resourceBusy" @click="loadKeys(keyPage + 1)">下一页</button></div>
      <label>粘贴本站密钥<input v-model="customKey" :disabled="busy || resourceBusy" type="password" autocomplete="off" placeholder="sk-…" @input="clearModels" /></label>
      <p class="muted">仅用于当前页面，不保存到本地任务。第三方上游地址需先接入资源管理。</p>
      <button type="button" :disabled="busy || resourceBusy || !customKey.trim()" @click="loadCustom">读取模型</button>
    </details>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <section v-if="selectedLabel" class="model-section">
      <strong>{{ selectedLabel }}</strong>
      <label>模型<select v-model="model" aria-label="选择模型" :disabled="busy || !models.length"><option value="">{{ busy ? '正在读取模型…' : models.length ? '选择模型' : '暂无可用模型' }}</option><option v-for="item in models" :key="item" :value="item">{{ item }}</option></select></label>
      <p class="muted">模型调用按所选资源的权限与价格执行。</p>
    </section>
    <footer><button v-if="slot === 'image'" type="button" :disabled="busy || resourceBusy" @click="disableImage">不启用图片生成</button><button type="button" class="primary" :disabled="busy || resourceBusy || !model" @click="confirm">使用此资源</button></footer>
  </dialog>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { Check, KeyRound, X } from '@lucide/vue'
import ResourcePicker from '@/components/keys/ResourcePicker.vue'
import type { ResourceChoice } from '@/components/keys/resourceChoice'
import { apiClient } from '@/api/client'
import { create as createKey, list as listKeys } from '@/api/keys'
import { createSharedPoolAccessKey } from '@/features/bizdecipher/api/bizdecipher'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
import type { ApiKey } from '@/types'

type Slot = 'language' | 'image'
interface Choice { slot: Slot; keyId: number; model: string; customKey: string; label: string }
const emit = defineEmits<{ select: [value: Choice] }>()
const dialog = ref<HTMLDialogElement | null>(null)
const opened = ref(false)
const slot = ref<Slot>('language')
const resource = ref<ResourceChoice | null>(null)
const resourceBusy = ref(false)
const pickerRevision = ref(0)
const busy = ref(false)
const error = ref('')
const model = ref('')
const models = ref<string[]>([])
const selectedId = ref(0)
const selectedLabel = ref('')
const selectedCustomKey = ref('')
const customKey = ref('')
const keys = ref<ApiKey[]>([])
const keyQuery = ref('')
const keyPage = ref(1)
const keyPages = ref(1)
const keysLoading = ref(false)
const keyError = ref('')
let generation = 0
let keyRequest = 0
let timer: ReturnType<typeof setTimeout> | undefined

function clearModels() {
  generation++
  model.value = ''; models.value = []; selectedId.value = 0
  selectedLabel.value = ''; selectedCustomKey.value = ''; error.value = ''
}
function resetSlot(value: Slot) { if (slot.value !== value) { clearModels(); slot.value = value } }
function preventBusyClose(event: Event) { if (busy.value || resourceBusy.value) event.preventDefault() }
function handleClose() { opened.value = false; clearModels(); customKey.value = ''; resource.value = null }
async function loadKeys(page = 1) {
  const request = ++keyRequest
  keysLoading.value = true
  keyError.value = ''
  try {
    const result = await listKeys(page, 20, { scope: 'all', status: 'active', search: keyQuery.value })
    if (request !== keyRequest) return
    keys.value = result.items; keyPage.value = page; keyPages.value = Math.max(1, result.pages)
  } catch (cause) {
    if (request === keyRequest) { keys.value = []; keyError.value = extractActionableApiErrorMessage(cause, '密钥读取失败') }
  } finally { if (request === keyRequest) keysLoading.value = false }
}
async function chooseKey(id: number, label: string) {
  const current = ++generation
  selectedId.value = id; selectedLabel.value = label; selectedCustomKey.value = ''
  model.value = ''; models.value = []; busy.value = true; error.value = ''
  try {
    const result = await apiClient.get<{ models: string[] }>('/biz/harness/models', { params: { key_id: id } })
    if (current === generation) { models.value = result.data.models; model.value = models.value[0] || '' }
  } catch (cause) { if (current === generation) error.value = extractActionableApiErrorMessage(cause, '模型读取失败，请检查资源状态') }
  finally { if (current === generation) busy.value = false }
}
async function useResource() {
  const selected = resource.value
  if (!selected?.ready || busy.value || resourceBusy.value) return
  busy.value = true; error.value = ''
  const current = ++generation
  try {
    let id: number
    if (selected.kind === 'shared') {
      id = (await createSharedPoolAccessKey(selected.id, `工作台 · ${selected.name}`)).access_key.api_key_id
    } else {
      const existing = await listKeys(1, 1, { group_id: selected.id, status: 'active' })
      id = existing.items[0]?.id ?? (await createKey(`工作台 · ${selected.name}`, selected.id)).id
    }
    if (current === generation) await chooseKey(id, selected.name)
  } catch (cause) {
    if (current === generation) { error.value = extractActionableApiErrorMessage(cause, '暂时无法使用此资源'); busy.value = false }
  }
}
async function loadCustom() {
  const current = ++generation
  busy.value = true; error.value = ''; models.value = []; model.value = ''
  selectedId.value = 0; selectedLabel.value = '自定义密钥'; selectedCustomKey.value = ''
  try {
    const result = await apiClient.post<{ models: string[] }>('/biz/harness/models', { customKey: customKey.value.trim() })
    if (current === generation) { models.value = result.data.models; model.value = models.value[0] || ''; selectedCustomKey.value = customKey.value.trim() }
  } catch (cause) { if (current === generation) error.value = extractActionableApiErrorMessage(cause, '密钥无效、无权限或没有可用模型') }
  finally { if (current === generation) busy.value = false }
}
function confirm() {
  if (busy.value || resourceBusy.value || !model.value) return
  emit('select', { slot: slot.value, keyId: selectedId.value, model: model.value, customKey: selectedCustomKey.value, label: selectedLabel.value })
  dialog.value?.close()
}
function disableImage() { emit('select', { slot: 'image', keyId: 0, model: '', customKey: '', label: '' }); dialog.value?.close() }
function showModal() { opened.value = true; dialog.value?.showModal(); void loadKeys() }
function showRequested(kind: 'official' | 'shared', id: number) {
  resource.value = { kind, id, name: kind === 'official' ? '所选官方资源' : '所选共享资源', ready: false }
  pickerRevision.value++
  showModal()
}
function resetForAccountSwitch() {
  generation++; keyRequest++; clearTimeout(timer)
  busy.value = false; resourceBusy.value = false; keys.value = []; keyQuery.value = ''
  handleClose(); dialog.value?.close()
}
watch(resource, clearModels)
watch(keyQuery, () => { clearTimeout(timer); timer = setTimeout(() => void loadKeys(), 250) })
onBeforeUnmount(() => { generation++; keyRequest++; clearTimeout(timer) })
defineExpose({ showModal, showRequested, resetForAccountSwitch })
</script>

<style scoped>
.resource-picker{margin:auto;width:min(620px,calc(100vw - 24px));max-height:90dvh;overflow:auto;border:1px solid var(--bd-ui-line);border-radius:8px;padding:24px;color:var(--bd-text-primary);background:var(--bd-surface)}.resource-picker::backdrop{background:rgb(0 0 0 / .3)}
header,footer,.slots,.key-pages{display:flex;align-items:center;gap:12px}header{justify-content:space-between}h2{font-size:18px;font-weight:600}header button{width:32px;height:32px;display:grid;place-items:center}.slots{margin:12px 0 18px;border-bottom:1px solid var(--bd-ui-line);font-size:13px}.slots button{padding:10px 4px;border-bottom:2px solid transparent}.slots .active{color:var(--bd-accent-teal);border-color:var(--bd-accent-teal)}
label{display:grid;gap:8px;margin:12px 0;font-size:13px}input,select{width:100%;min-width:0;padding:9px;background:var(--bd-surface);border:1px solid var(--bd-ui-line);border-radius:6px}
.muted{font-size:12px;color:var(--bd-text-secondary);line-height:1.6;margin:12px 0}.error{color:var(--bd-status-danger);font-size:13px;margin:12px 0}.primary{padding:9px 14px;background:var(--bd-accent-teal);color:white;border-radius:6px}.load-models{margin-top:14px;font-size:13px}
.existing-connection{margin-top:20px;border-top:1px solid var(--bd-ui-line);padding-top:14px;font-size:13px}.existing-connection summary{cursor:pointer;color:var(--bd-text-secondary)}.key-list{max-height:180px;overflow:auto}.key-list button{display:flex;align-items:center;width:100%;gap:10px;text-align:left;padding:10px 4px;border-bottom:1px solid var(--bd-ui-line)}.key-list span{flex:1;min-width:0;overflow-wrap:anywhere}.key-list small{display:block;color:var(--bd-text-secondary);margin-top:3px}.key-pages{justify-content:flex-end;font-size:12px;margin-top:8px}.model-section{margin-top:16px;padding-top:16px;border-top:1px solid var(--bd-ui-line);font-size:13px}
footer{justify-content:flex-end;margin-top:20px;font-size:13px}button:disabled{opacity:.5;cursor:not-allowed}button:focus-visible{outline:2px solid var(--bd-accent-teal);outline-offset:2px}
@media(max-width:560px){.resource-picker{padding:16px}}
</style>
