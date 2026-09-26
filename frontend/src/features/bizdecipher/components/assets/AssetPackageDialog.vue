<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { saveAs } from 'file-saver'
import Icon from '@/components/icons/Icon.vue'
import AssetCommercePanel from './AssetCommercePanel.vue'
import { useAppStore } from '@/stores/app'
import {
  createCapabilityAssetVersion,
  downloadCapabilityAssetPackage,
  finalizeCapabilityAssetVersion,
  listCapabilityAssetVersions,
  publishCapabilityAssetVersion,
  putCapabilityAssetFile,
  revokeCapabilityAssetVersion,
  type CapabilityAsset,
  type CapabilityAssetVersion,
} from '@/features/bizdecipher/api/bizdecipher'
import { extractActionableApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  open: boolean
  asset: CapabilityAsset | null
}>()

const emit = defineEmits<{
  close: []
}>()

const appStore = useAppStore()
const versions = ref<CapabilityAssetVersion[]>([])
const loading = ref(false)
const loadError = ref('')
const operationVersion = ref('')
const selectedFiles = ref<File[]>([])

const draft = reactive({
  version: 'v1.0.0',
  runtimeKind: 'workflow',
  manifest: '',
})

const runtimeOptions = [
  ['workflow', '工作流'],
  ['declarative', '声明式工具'],
  ['external_api', '外部 API 适配器'],
  ['game_script', '游戏 / 剧本'],
  ['template', '模板'],
  ['dataset', '数据集'],
  ['other', '其他'],
] as const

const ownerAsset = computed(() => props.asset)
const canMutate = computed(() => Boolean(ownerAsset.value && props.open))

function resetDraft() {
  draft.version = 'v1.0.0'
  draft.runtimeKind = 'workflow'
  draft.manifest = JSON.stringify(
    {
      schema_version: 1,
      name: ownerAsset.value?.title || '',
      description: ownerAsset.value?.summary || '',
      capabilities: [],
      permissions: [],
      entrypoint: null,
      input_schema: {},
      output_schema: {},
    },
    null,
    2,
  )
}

async function loadVersions() {
  if (!ownerAsset.value) return
  loading.value = true
  loadError.value = ''
  try {
    versions.value = await listCapabilityAssetVersions(ownerAsset.value.id)
  } catch (error) {
    loadError.value = extractActionableApiErrorMessage(error, '版本列表加载失败')
  } finally {
    loading.value = false
  }
}

async function createVersion() {
  if (!ownerAsset.value || operationVersion.value) return
  const version = draft.version.trim()
  if (!/^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/.test(version)) {
    appStore.showError('版本号只能使用字母、数字、点、短横线和下划线')
    return
  }
  let manifest: Record<string, unknown>
  try {
    const parsed = JSON.parse(draft.manifest || '{}')
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error('manifest must be an object')
    manifest = parsed as Record<string, unknown>
  } catch {
    appStore.showError('清单必须是合法的 JSON 对象')
    return
  }

  operationVersion.value = `create:${version}`
  try {
    const created = await createCapabilityAssetVersion(ownerAsset.value.id, {
      version,
      runtime_kind: draft.runtimeKind,
      manifest,
    })
    versions.value = [created, ...versions.value.filter(item => item.version !== created.version)]
    appStore.showSuccess(`版本 ${created.version} 已创建`)
  } catch (error) {
    appStore.showError(extractActionableApiErrorMessage(error, '创建版本失败'))
  } finally {
    operationVersion.value = ''
  }
}

function handleFilesSelected(event: Event) {
  const input = event.target as HTMLInputElement
  selectedFiles.value = Array.from(input.files || [])
}

async function uploadFiles(version: CapabilityAssetVersion) {
  if (!ownerAsset.value || selectedFiles.value.length === 0 || operationVersion.value) return
  const oversized = selectedFiles.value.find(file => file.size > 8 * 1024 * 1024)
  if (oversized) {
    appStore.showError(`文件 ${oversized.name} 超过 8 MiB 上限`)
    return
  }

  operationVersion.value = `upload:${version.version}`
  let uploaded = 0
  try {
    for (const file of selectedFiles.value) {
      const path = ('webkitRelativePath' in file && file.webkitRelativePath) || file.name
      await putCapabilityAssetFile(ownerAsset.value.id, version.version, {
        path,
        content_type: file.type || 'application/octet-stream',
        content_base64: await fileToBase64(file),
      })
      uploaded += 1
    }
    selectedFiles.value = []
    await loadVersions()
    appStore.showSuccess(`已写入 ${uploaded} 个文件`)
  } catch (error) {
    appStore.showError(extractActionableApiErrorMessage(error, `上传中断，已成功写入 ${uploaded} 个文件`))
    await loadVersions()
  } finally {
    operationVersion.value = ''
  }
}

async function finalizeVersion(version: CapabilityAssetVersion) {
  await runVersionAction(version, 'finalize', async () => {
    await finalizeCapabilityAssetVersion(ownerAsset.value!.id, version.version)
    appStore.showSuccess(`版本 ${version.version} 已封包`)
  })
}

async function publishVersion(version: CapabilityAssetVersion) {
  await runVersionAction(version, 'publish', async () => {
    await publishCapabilityAssetVersion(ownerAsset.value!.id, version.version)
    appStore.showSuccess(`版本 ${version.version} 已发布`)
  })
}

async function revokeVersion(version: CapabilityAssetVersion) {
  if (!window.confirm(`确认撤回版本 ${version.version}？撤回后不可重新发布。`)) return
  await runVersionAction(version, 'revoke', async () => {
    await revokeCapabilityAssetVersion(ownerAsset.value!.id, version.version)
    appStore.showSuccess(`版本 ${version.version} 已撤回`)
  })
}

async function downloadVersion(version: CapabilityAssetVersion) {
  if (!ownerAsset.value || operationVersion.value) return
  operationVersion.value = `download:${version.version}`
  try {
    const blob = await downloadCapabilityAssetPackage(ownerAsset.value.id, version.version)
    saveAs(blob, `asset-${ownerAsset.value.id}-${version.version}.zip`)
  } catch (error) {
    appStore.showError(extractActionableApiErrorMessage(error, '下载交付包失败'))
  } finally {
    operationVersion.value = ''
  }
}

async function runVersionAction(
  version: CapabilityAssetVersion,
  action: 'finalize' | 'publish' | 'revoke',
  operation: () => Promise<void>,
) {
  if (!ownerAsset.value || operationVersion.value) return
  operationVersion.value = `${action}:${version.version}`
  try {
    await operation()
    await loadVersions()
  } catch (error) {
    appStore.showError(extractActionableApiErrorMessage(error, '版本操作失败'))
  } finally {
    operationVersion.value = ''
  }
}

function fileToBase64(file: File): Promise<string> {
  return file.arrayBuffer().then((buffer) => {
    const bytes = new Uint8Array(buffer)
    let binary = ''
    const chunkSize = 0x8000
    for (let offset = 0; offset < bytes.length; offset += chunkSize) {
      binary += String.fromCharCode(...bytes.subarray(offset, offset + chunkSize))
    }
    return btoa(binary)
  })
}

function statusLabel(status: string): string {
  return {
    draft: '草稿',
    ready: '已封包',
    published: '已发布',
    revoked: '已撤回',
  }[status] || status
}

function runtimeLabel(kind: string): string {
  return runtimeOptions.find(([value]) => value === kind)?.[1] || kind
}

function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB']
  let size = value
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit += 1
  }
  return `${size >= 10 || unit === 0 ? size.toFixed(0) : size.toFixed(1)} ${units[unit]}`
}

function formatDate(value?: string | null): string {
  if (!value) return '--'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '--'
  return date.toLocaleString('zh-CN', { hour12: false })
}

watch(
  () => [props.open, props.asset?.id] as const,
  ([open]) => {
    if (!open) {
      selectedFiles.value = []
      return
    }
    resetDraft()
    void loadVersions()
  },
  { immediate: true },
)
</script>

<template>
  <Transition name="package-dialog">
    <div v-if="open" class="package-overlay" @click.self="emit('close')">
      <section class="package-dialog" role="dialog" aria-modal="true" aria-labelledby="asset-package-title">
        <header class="package-header">
          <div>
            <span class="package-kicker">Versioned Delivery</span>
            <h2 id="asset-package-title">版本与交付包</h2>
            <p>{{ ownerAsset?.title }}</p>
          </div>
          <div class="package-header-actions">
            <button type="button" class="package-icon-button" title="刷新版本" :disabled="loading" @click="loadVersions">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button type="button" class="package-icon-button" title="关闭" @click="emit('close')">
              <Icon name="x" size="sm" />
            </button>
          </div>
        </header>

        <div class="package-body">
          <AssetCommercePanel v-if="ownerAsset" :asset-id="ownerAsset.id" />
          <form class="package-create" @submit.prevent="createVersion">
            <div class="package-create-head">
              <div>
                <strong>建立新版本</strong>
                <small>版本发布后不可修改，升级请创建新版本。</small>
              </div>
              <button type="submit" :disabled="!canMutate || Boolean(operationVersion)">
                <Icon name="plus" size="sm" />
                {{ operationVersion.startsWith('create:') ? '创建中…' : '创建版本' }}
              </button>
            </div>
            <div class="package-form-grid">
              <label>
                版本号
                <input v-model="draft.version" maxlength="64" placeholder="v1.0.0" />
              </label>
              <label>
                运行类型
                <select v-model="draft.runtimeKind">
                  <option v-for="option in runtimeOptions" :key="option[0]" :value="option[0]">{{ option[1] }}</option>
                </select>
              </label>
              <label class="package-manifest">
                能力清单
                <textarea v-model="draft.manifest" rows="8" spellcheck="false" />
              </label>
            </div>
          </form>

          <div class="package-list-head">
            <div>
              <strong>版本记录</strong>
              <small>文件、摘要和下载记录都由后端校验。</small>
            </div>
            <span>{{ versions.length }} 个版本</span>
          </div>

          <div v-if="loading" class="package-state">
            <Icon name="refresh" size="lg" class="animate-spin" />
            <span>正在加载版本…</span>
          </div>
          <div v-else-if="loadError" class="package-state package-state-error">
            <Icon name="exclamationCircle" size="lg" />
            <span>{{ loadError }}</span>
            <button type="button" @click="loadVersions">重试</button>
          </div>
          <div v-else-if="versions.length === 0" class="package-state">
            <Icon name="document" size="lg" />
            <strong>还没有交付版本</strong>
            <span>先创建一个草稿版本，再上传文件并封包。</span>
          </div>

          <div v-else class="package-version-list">
            <article v-for="version in versions" :key="version.id" class="package-version">
              <div class="package-version-main">
                <div class="package-version-title">
                  <strong>{{ version.version }}</strong>
                  <span class="package-status" :data-status="version.status">{{ statusLabel(version.status) }}</span>
                  <span class="package-runtime">{{ runtimeLabel(version.runtime_kind) }}</span>
                </div>
                <div class="package-version-meta">
                  <span>{{ version.file_count }} 个文件</span>
                  <span>{{ formatBytes(version.total_bytes) }}</span>
                  <span>更新 {{ formatDate(version.updated_at) }}</span>
                  <span v-if="version.published_at">发布 {{ formatDate(version.published_at) }}</span>
                </div>
                <code v-if="version.package_digest" class="package-digest">SHA-256 {{ version.package_digest }}</code>
              </div>

              <div v-if="version.status === 'draft'" class="package-draft-actions">
                <label class="package-file-picker">
                  <Icon name="upload" size="sm" />
                  <span>{{ selectedFiles.length ? `已选择 ${selectedFiles.length} 个文件` : '选择文件' }}</span>
                  <input type="file" multiple @change="handleFilesSelected" />
                </label>
                <button
                  type="button"
                  class="package-action"
                  :disabled="selectedFiles.length === 0 || Boolean(operationVersion)"
                  @click="uploadFiles(version)"
                >
                  {{ operationVersion === `upload:${version.version}` ? '上传中…' : '写入文件' }}
                </button>
                <button
                  type="button"
                  class="package-action package-action-primary"
                  :disabled="version.file_count === 0 || Boolean(operationVersion)"
                  @click="finalizeVersion(version)"
                >
                  {{ operationVersion === `finalize:${version.version}` ? '封包中…' : '封包' }}
                </button>
              </div>

              <div v-else class="package-draft-actions">
                <button
                  v-if="version.status === 'ready'"
                  type="button"
                  class="package-action package-action-primary"
                  :disabled="Boolean(operationVersion)"
                  @click="publishVersion(version)"
                >
                  <Icon name="check" size="sm" />
                  {{ operationVersion === `publish:${version.version}` ? '发布中…' : '发布版本' }}
                </button>
                <button
                  v-if="version.status === 'published'"
                  type="button"
                  class="package-action package-action-primary"
                  :disabled="Boolean(operationVersion)"
                  @click="downloadVersion(version)"
                >
                  <Icon name="download" size="sm" />
                  {{ operationVersion === `download:${version.version}` ? '下载中…' : '下载交付包' }}
                </button>
                <button
                  v-if="version.status === 'ready' || version.status === 'published'"
                  type="button"
                  class="package-action package-action-danger"
                  :disabled="Boolean(operationVersion)"
                  @click="revokeVersion(version)"
                >
                  {{ operationVersion === `revoke:${version.version}` ? '撤回中…' : '撤回' }}
                </button>
              </div>
            </article>
          </div>
        </div>
      </section>
    </div>
  </Transition>
</template>

<style scoped>
.package-overlay {
  position: fixed;
  inset: 0;
  z-index: 1200;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: rgba(7, 15, 28, 0.58);
}

.package-dialog {
  display: flex;
  flex-direction: column;
  width: min(920px, 96vw);
  max-height: min(860px, 92vh);
  overflow: hidden;
  color: var(--zc-text, #dce8f6);
  background: var(--zc-bg, #0c1726);
  border: 1px solid var(--zc-line, #22364e);
  border-radius: 12px;
  box-shadow: 0 30px 90px rgba(3, 9, 18, 0.52);
}

.package-header {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  padding: 20px 22px;
  border-bottom: 1px solid var(--zc-line, #22364e);
}
.package-header h2 {
  margin: 3px 0 4px;
  color: var(--zc-text-strong, #f4f8ff);
  font-size: 20px;
}
.package-header p {
  margin: 0;
  color: var(--zc-muted, #8fa2b8);
  font-size: 13px;
}
.package-kicker {
  color: var(--zc-accent-2, #4fd1c5);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}
.package-header-actions {
  display: flex;
  gap: 6px;
}
.package-icon-button {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  color: var(--zc-muted, #8fa2b8);
  background: transparent;
  border: 1px solid var(--zc-line, #22364e);
  border-radius: 8px;
  cursor: pointer;
}
.package-icon-button:hover:not(:disabled) {
  color: var(--zc-text-strong, #f4f8ff);
  border-color: var(--zc-accent, #4c9ffe);
}
.package-icon-button:disabled {
  cursor: wait;
  opacity: 0.5;
}

.package-body {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 18px 22px 24px;
  overflow-y: auto;
}

.package-create {
  padding: 16px;
  background: color-mix(in srgb, var(--zc-bg, #0c1726) 82%, #09111e);
  border: 1px solid var(--zc-line, #22364e);
  border-radius: 10px;
}
.package-create-head,
.package-list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.package-create-head strong,
.package-list-head strong {
  display: block;
  color: var(--zc-text-strong, #f4f8ff);
  font-size: 14px;
}
.package-create-head small,
.package-list-head small {
  display: block;
  margin-top: 3px;
  color: var(--zc-muted, #8fa2b8);
  font-size: 11px;
}
.package-create-head button,
.package-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  min-height: 34px;
  padding: 0 12px;
  color: var(--zc-text, #dce8f6);
  font-size: 12px;
  font-weight: 650;
  background: transparent;
  border: 1px solid var(--zc-line, #22364e);
  border-radius: 7px;
  cursor: pointer;
}
.package-create-head button {
  color: var(--zc-accent-ink, #fff);
  background: var(--zc-accent, #4c9ffe);
  border-color: var(--zc-accent, #4c9ffe);
}
.package-create-head button:hover:not(:disabled),
.package-action:hover:not(:disabled) {
  border-color: var(--zc-accent, #4c9ffe);
}
.package-create-head button:disabled,
.package-action:disabled {
  cursor: not-allowed;
  opacity: 0.48;
}

.package-form-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 12px;
  margin-top: 14px;
}
.package-form-grid label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: var(--zc-muted, #8fa2b8);
  font-size: 11px;
  font-weight: 650;
}
.package-form-grid input,
.package-form-grid select,
.package-form-grid textarea {
  width: 100%;
  padding: 9px 10px;
  color: var(--zc-text, #dce8f6);
  font: inherit;
  font-size: 12px;
  background: var(--zc-bg, #0c1726);
  border: 1px solid var(--zc-line, #22364e);
  border-radius: 7px;
  outline: none;
}
.package-form-grid input:focus,
.package-form-grid select:focus,
.package-form-grid textarea:focus {
  border-color: var(--zc-accent, #4c9ffe);
}
.package-form-grid textarea {
  resize: vertical;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  line-height: 1.5;
}
.package-manifest {
  grid-column: 1 / -1;
}

.package-list-head > span {
  color: var(--zc-muted, #8fa2b8);
  font-size: 11px;
}

.package-state {
  display: flex;
  min-height: 190px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--zc-muted, #8fa2b8);
  font-size: 12px;
  border: 1px dashed var(--zc-line, #22364e);
  border-radius: 10px;
}
.package-state strong {
  color: var(--zc-text-strong, #f4f8ff);
  font-size: 14px;
}
.package-state button {
  color: var(--zc-accent-2, #4fd1c5);
  background: transparent;
  border: 0;
  cursor: pointer;
}
.package-state-error {
  color: var(--zc-danger, #f87171);
  border-color: color-mix(in srgb, var(--zc-danger, #f87171) 42%, var(--zc-line, #22364e));
}

.package-version-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.package-version {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 16px;
  padding: 14px;
  border: 1px solid var(--zc-line, #22364e);
  border-radius: 10px;
}
.package-version-main {
  min-width: 0;
}
.package-version-title {
  display: flex;
  align-items: center;
  gap: 7px;
  flex-wrap: wrap;
}
.package-version-title strong {
  color: var(--zc-text-strong, #f4f8ff);
  font-size: 14px;
}
.package-status,
.package-runtime {
  padding: 2px 7px;
  color: var(--zc-muted, #8fa2b8);
  font-size: 10px;
  font-weight: 700;
  border: 1px solid var(--zc-line, #22364e);
  border-radius: 999px;
}
.package-status[data-status="ready"] {
  color: var(--zc-warning, #f3c969);
  border-color: color-mix(in srgb, var(--zc-warning, #f3c969) 42%, var(--zc-line, #22364e));
}
.package-status[data-status="published"] {
  color: var(--zc-success, #56d59a);
  border-color: color-mix(in srgb, var(--zc-success, #56d59a) 42%, var(--zc-line, #22364e));
}
.package-status[data-status="revoked"] {
  color: var(--zc-danger, #f87171);
  border-color: color-mix(in srgb, var(--zc-danger, #f87171) 42%, var(--zc-line, #22364e));
}
.package-version-meta {
  display: flex;
  gap: 12px;
  margin-top: 8px;
  color: var(--zc-muted, #8fa2b8);
  font-size: 11px;
  flex-wrap: wrap;
}
.package-digest {
  display: block;
  margin-top: 8px;
  overflow: hidden;
  color: var(--zc-subtle, #6f849d);
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.package-draft-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 7px;
  flex-wrap: wrap;
}
.package-action-primary {
  color: var(--zc-accent-ink, #fff);
  background: var(--zc-accent, #4c9ffe);
  border-color: var(--zc-accent, #4c9ffe);
}
.package-action-danger {
  color: var(--zc-danger, #f87171);
  border-color: color-mix(in srgb, var(--zc-danger, #f87171) 42%, var(--zc-line, #22364e));
}
.package-file-picker {
  position: relative;
  display: inline-flex;
  min-height: 34px;
  align-items: center;
  gap: 5px;
  padding: 0 11px;
  color: var(--zc-muted, #8fa2b8);
  font-size: 11px;
  border: 1px dashed var(--zc-line, #22364e);
  border-radius: 7px;
  cursor: pointer;
}
.package-file-picker:hover {
  color: var(--zc-text, #dce8f6);
  border-color: var(--zc-accent, #4c9ffe);
}
.package-file-picker input {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
}

.animate-spin {
  animation: package-spin 1s linear infinite;
}
@keyframes package-spin {
  to { transform: rotate(360deg); }
}

.package-dialog-enter-active,
.package-dialog-leave-active {
  transition: opacity 0.18s ease;
}
.package-dialog-enter-active .package-dialog,
.package-dialog-leave-active .package-dialog {
  transition: transform 0.2s ease;
}
.package-dialog-enter-from,
.package-dialog-leave-to {
  opacity: 0;
}
.package-dialog-enter-from .package-dialog,
.package-dialog-leave-to .package-dialog {
  transform: translateY(10px) scale(0.985);
}

@media (max-width: 720px) {
  .package-overlay {
    padding: 10px;
  }
  .package-dialog {
    width: 100%;
    max-height: 96vh;
  }
  .package-header,
  .package-body {
    padding-right: 14px;
    padding-left: 14px;
  }
  .package-form-grid,
  .package-version {
    grid-template-columns: minmax(0, 1fr);
  }
  .package-create-head,
  .package-list-head {
    align-items: flex-start;
  }
  .package-draft-actions {
    justify-content: flex-start;
  }
}
</style>
