<script setup lang="ts">
import { ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { Copy, RefreshCw } from '@lucide/vue'
import MemberKeyList from '@/features/bizdecipher/components/shared-pool/MemberKeyList.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { listMySharedPoolAccessKeys, deleteSharedPoolAccessKey, type SharedPoolAccessKey } from '@/features/bizdecipher/api/bizdecipher'
import { useClipboard } from '@/composables/useClipboard'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
const props = defineProps<{ revision: number; baseUrl: string }>()
const keys = ref<SharedPoolAccessKey[]>([])
const loading = ref(false)
const error = ref('')
const pendingDelete = ref<SharedPoolAccessKey | null>(null)
const deletingId = ref<number | null>(null)
const { copyToClipboard } = useClipboard()
async function load() {
  loading.value = true
  error.value = ''
  try { keys.value = await listMySharedPoolAccessKeys() }
  catch (cause) { error.value = extractActionableApiErrorMessage(cause, '共享池密钥加载失败') }
  finally { loading.value = false }
}
async function remove() {
  if (!pendingDelete.value || deletingId.value !== null) return
  deletingId.value = pendingDelete.value.api_key_id
  try {
    await deleteSharedPoolAccessKey(deletingId.value)
    pendingDelete.value = null
    await load()
  } catch (cause) { error.value = extractActionableApiErrorMessage(cause, '撤销密钥失败') }
  finally { deletingId.value = null }
}
watch(() => props.revision, load, { immediate: true })
</script>

<template>
  <section class="space-y-4 py-4" aria-label="共享池密钥列表">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-base font-semibold">共享池密钥</h2>
      <div class="flex items-center gap-3">
        <RouterLink to="/account-square/my" class="text-sm text-primary-600">管理绑定资源</RouterLink>
        <button type="button" class="btn btn-secondary" :disabled="loading" aria-label="刷新共享池密钥" title="刷新共享池密钥" @click="load"><RefreshCw :size="16" /></button>
      </div>
    </header>
    <div class="flex min-w-0 items-center gap-3 border-y border-gray-200 py-3 dark:border-gray-700">
      <span class="shrink-0 text-xs text-gray-500">API 地址</span>
      <code class="min-w-0 flex-1 break-all text-xs">{{ baseUrl }}</code>
      <button type="button" class="shrink-0 p-2" aria-label="复制 API 地址" title="复制 API 地址" @click="copyToClipboard(baseUrl)"><Copy :size="16" /></button>
    </div>
    <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    <p v-if="loading" role="status" class="py-6 text-sm text-gray-500">正在读取密钥…</p>
    <MemberKeyList v-else-if="!error && keys.length" :keys="keys" :deleting-id="deletingId" @copy="copyToClipboard" @delete="pendingDelete = $event" />
    <p v-else-if="!error" class="py-6 text-sm text-gray-500">还没有共享池密钥</p>
    <ConfirmDialog :show="pendingDelete !== null" title="撤销共享池密钥" message="撤销后，这把密钥绑定的所有共享池都无法再通过它调用。已有消费记录保留。" :confirm-text="deletingId !== null ? '正在撤销…' : '撤销密钥'" danger @confirm="remove" @cancel="deletingId === null && (pendingDelete = null)" />
  </section>
</template>
