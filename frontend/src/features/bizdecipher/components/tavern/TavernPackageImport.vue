<script setup lang="ts">
import { onScopeDispose, ref, watch } from 'vue'
import { Download, Upload } from '@lucide/vue'
import { useAuthStore } from '@/stores/auth'
import { createTavernGamePackage, type TavernGamePackagePayload } from '@/features/bizdecipher/api/bizdecipher'
import { extractActionableApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ scriptId: number }>()
const emit = defineEmits<{ imported: [] }>()
const auth = useAuthStore()
const payload = ref<TavernGamePackagePayload | null>(null)
const filename = ref('')
const error = ref('')
const notice = ref('')
const busy = ref(false)
let generation = 0
let fileRequest = 0

async function choose(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  const session = generation
  const request = ++fileRequest
  payload.value = null
  filename.value = ''
  error.value = ''
  notice.value = ''
  if (!file) return
  if (file.size > 270 * 1024) { error.value = '游戏包文件不能超过 270 KB。'; return }
  try {
    const value = JSON.parse(await file.text()) as Partial<TavernGamePackagePayload> | null
    if (session !== generation || request !== fileRequest) return
    if (!value || typeof value !== 'object' || typeof value.version !== 'string' || !value.version.trim() || value.version.length > 64 || !value.manifest || typeof value.manifest !== 'object') throw new Error('文件需要包含 version 和 manifest。')
    if (value.manifest.runtime_kind !== 'declarative' || value.manifest.schema_version !== 'tavern.package.v1') throw new Error('只支持 tavern.package.v1 声明式游戏包。')
    if (value.manifest.permissions?.purchases) throw new Error('游戏包暂不支持内购权限。')
    payload.value = { version: value.version.trim(), manifest: value.manifest }
    filename.value = file.name
  } catch (value) {
    if (session === generation && request === fileRequest) error.value = value instanceof SyntaxError ? '文件不是有效的 JSON。' : extractActionableApiErrorMessage(value, '游戏包无法读取。')
  }
}
async function upload() {
  if (!payload.value || busy.value) return
  const session = generation
  busy.value = true
  error.value = ''
  try {
    const saved = await createTavernGamePackage(props.scriptId, payload.value)
    if (session !== generation) return
    notice.value = `版本 ${saved.version} 已保存为草稿。`
    payload.value = null
    filename.value = ''
    emit('imported')
  } catch (value) {
    if (session === generation) error.value = extractActionableApiErrorMessage(value, '未收到上传成功确认，请刷新版本列表核对。')
  } finally { if (session === generation) busy.value = false }
}
function template() {
  const sample: TavernGamePackagePayload = {
    version: '1.0.0',
    manifest: {
      schema_version: 'tavern.package.v1', runtime_kind: 'declarative', protocol_version: '2026-09-13.package.v1',
      entry: { kind: 'prompt_flow', ref: 'main' },
      permissions: { ai_gateway: false, save: true, score: false, purchases: false, presence: false },
      content: { description: '', host_brief: '', opening_prompt: '', safety_notes: '', npc_cards: [] },
      limits: { max_turns: 48, max_scenes: 12 },
    },
  }
  const url = URL.createObjectURL(new Blob([JSON.stringify(sample, null, 2)], { type: 'application/json' }))
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = 'tavern-package.json'
  anchor.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
watch(() => [props.scriptId, auth.user?.id], () => {
  generation++; fileRequest++
  payload.value = null; filename.value = ''; error.value = ''; notice.value = ''; busy.value = false
})
onScopeDispose(() => { generation++; fileRequest++ })
</script>

<template>
  <section class="package-import" aria-label="导入本地游戏包">
    <header><h3>导入本地游戏包</h3><button type="button" @click="template"><Download :size="15" />下载模板</button></header>
    <label>游戏包 JSON<input type="file" accept=".json,application/json" :disabled="busy" @change="choose" /></label>
    <div v-if="payload" class="preview"><span>{{ filename }} · 版本 {{ payload.version }}</span><button type="button" :disabled="busy" @click="upload"><Upload :size="15" />{{ busy ? '上传中…' : '上传为草稿' }}</button></div>
    <p v-if="error" role="alert">{{ error }}</p><p v-if="notice" role="status">{{ notice }}</p>
  </section>
</template>

<style scoped>
.package-import{display:grid;gap:12px;padding-block:16px;border-block:1px solid var(--bd-ui-line);color:var(--bd-text-primary)}header,.preview{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}h3{font-size:14px;margin:0}label{display:grid;gap:8px;font-size:13px;min-width:0}input{max-width:100%;font-size:13px}button{display:inline-flex;gap:6px;align-items:center;min-height:34px;padding:6px 10px;border:1px solid var(--bd-ui-line);border-radius:6px;background:var(--bd-surface);color:var(--bd-text-primary);font-size:12px;cursor:pointer}button:disabled{opacity:.5;cursor:wait}.preview,p{font-size:13px;overflow-wrap:anywhere}p{margin:0;color:var(--bd-text-secondary)}button:focus-visible,input:focus-visible{outline:2px solid var(--bd-accent-teal);outline-offset:2px}
</style>
