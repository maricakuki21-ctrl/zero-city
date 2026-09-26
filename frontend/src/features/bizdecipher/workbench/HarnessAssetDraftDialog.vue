<template>
  <dialog ref="dialog" class="harness-dialog asset-draft-dialog" aria-labelledby="asset-draft-title">
    <header>
      <div>
        <h2 id="asset-draft-title">保存为能力资产</h2>
        <p class="muted">保存到“我的资产”草稿，不会提交审核。</p>
      </div>
      <button class="icon-button" type="button" aria-label="关闭资产草稿" :disabled="saving" @click="dialog?.close()"><X :size="18" /></button>
    </header>
    <form @submit.prevent="save">
      <label>资产标题<input v-model="title" aria-label="资产标题" maxlength="160" required /></label>
      <label>一句话简介<input v-model="summary" aria-label="一句话简介" maxlength="360" required /></label>
      <label>资产类型
        <select v-model="assetType" aria-label="资产类型">
          <option v-for="option in assetTypes" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
      </label>
      <div v-if="error" class="notice error" role="alert">{{ error }}</div>
      <footer>
        <button class="model-button" type="button" :disabled="saving" @click="dialog?.close()">取消</button>
        <button class="new-session" type="submit" aria-label="保存资产草稿" :disabled="!canSave">{{ saving ? '保存中…' : createdAssetId ? '重新确认' : '保存草稿' }}</button>
      </footer>
    </form>
  </dialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { X } from '@lucide/vue'
import {
  createCapabilityAsset,
  listMyCapabilityAssets,
  type CapabilityAssetType,
} from '@/features/bizdecipher/api/bizdecipher'
import { buildHarnessAssetDraft, summarizeHarnessOutput } from './harnessAssetDraft'

type CompletedHarnessResult = {
  readonly title: string
  readonly output: string
  readonly imageUrls: readonly string[]
}

const props = defineProps<{ readonly result: CompletedHarnessResult }>()
const emit = defineEmits<{ saved: [assetId: number] }>()
const dialog = ref<HTMLDialogElement | null>(null)
const title = ref('')
const summary = ref('')
const assetType = ref<CapabilityAssetType>('prompt_solution')
const saving = ref(false)
const error = ref('')
const createdAssetId = ref<number | null>(null)
const notifiedAssetId = ref<number | null>(null)
const resultFingerprint = ref('')
const snapshot = ref<CompletedHarnessResult | null>(null)
let operationEpoch = 0
const assetTypes = [
  { value: 'product_app', label: '产品 / 应用' },
  { value: 'game', label: '游戏' },
  { value: 'workflow', label: '工作流' },
  { value: 'agent', label: 'Agent' },
  { value: 'api_tool', label: 'API / 工具' },
  { value: 'plugin_template', label: '插件 / 模板' },
  { value: 'prompt_solution', label: '提示词 / 方案' },
  { value: 'dataset_report', label: '数据集 / 报告' },
  { value: 'other', label: '其他' },
] as const satisfies readonly { readonly value: CapabilityAssetType; readonly label: string }[]

const canSave = computed(() => !saving.value && title.value.trim().length > 0 && summary.value.trim().length > 0)

function showModal() {
  operationEpoch += 1
  const fingerprint = JSON.stringify(props.result)
  if (fingerprint !== resultFingerprint.value) {
    snapshot.value = {
      title: props.result.title,
      output: props.result.output,
      imageUrls: [...props.result.imageUrls],
    }
    resultFingerprint.value = fingerprint
    createdAssetId.value = null
    notifiedAssetId.value = null
    title.value = props.result.title.trim() || 'Harness 完成结果'
    summary.value = summarizeHarnessOutput(props.result.output)
    assetType.value = 'prompt_solution'
  }
  error.value = ''
  dialog.value?.showModal()
}

async function save() {
  if (!canSave.value) return
  const capturedResult = snapshot.value
  if (!capturedResult) return
  const epoch = operationEpoch
  saving.value = true
  error.value = ''
  try {
    if (createdAssetId.value === null) {
      const created = await createCapabilityAsset(buildHarnessAssetDraft({
        title: title.value,
        summary: summary.value,
        assetType: assetType.value,
        output: capturedResult.output,
        imageUrls: capturedResult.imageUrls,
      }))
      if (epoch !== operationEpoch) return
      createdAssetId.value = created.id
    }
    const assetId = createdAssetId.value
    if (notifiedAssetId.value !== assetId) {
      notifiedAssetId.value = assetId
      emit('saved', assetId)
    }
    const drafts = await listMyCapabilityAssets({ status: 'draft', limit: 50 })
    if (epoch !== operationEpoch) return
    if (!drafts.some(asset => asset.id === assetId)) {
      error.value = `资产草稿 #${assetId} 已创建，但未能在列表中确认。可重新确认或前往“我的资产”查看。`
      return
    }
    dialog.value?.close()
  } catch {
    error.value = createdAssetId.value === null
      ? '资产草稿创建失败，请稍后重试。'
      : `资产草稿 #${createdAssetId.value} 已创建，但回读确认失败。重试只会重新确认，不会重复创建。`
  } finally {
    if (epoch === operationEpoch) saving.value = false
  }
}

function resetForAccountSwitch() {
  operationEpoch += 1
  snapshot.value = null
  resultFingerprint.value = ''
  createdAssetId.value = null
  notifiedAssetId.value = null
  title.value = ''
  summary.value = ''
  error.value = ''
  saving.value = false
  dialog.value?.close()
}

defineExpose({ showModal, resetForAccountSwitch })
</script>

<style scoped>
.harness-dialog{margin:auto;padding:24px;width:min(560px,calc(100vw - 32px));max-height:85dvh;overflow:auto;background:var(--bd-surface);color:var(--bd-text-primary);border:1px solid var(--bd-ui-line);border-radius:8px}.harness-dialog::backdrop{background:color-mix(in srgb,var(--bd-text-primary) 30%,transparent)}.harness-dialog header{display:flex;align-items:flex-start;justify-content:space-between;margin-bottom:20px}.harness-dialog h2{font-size:18px;font-weight:600}.harness-dialog input,.harness-dialog select{width:100%;min-width:0;padding:10px;border:1px solid var(--bd-ui-line);border-radius:6px;background:var(--bd-surface);color:inherit}
.asset-draft-dialog header p{margin-top:6px;font-size:12px}.asset-draft-dialog form>label{display:grid;gap:8px;margin:16px 0;font-size:14px}.asset-draft-dialog footer{display:flex;justify-content:flex-end;gap:10px}.asset-draft-dialog footer button{min-height:38px;padding:8px 16px}
</style>
