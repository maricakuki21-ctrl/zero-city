<script setup lang="ts">
import { computed } from 'vue'
import CreatorWorkbenchView from '../CreatorWorkbenchView.vue'
import StudioWorkbenchView from '../StudioWorkbenchView.vue'
import type { CreatorWorkbenchService } from '../contracts'
import { normalizeWorkbenchRun, normalizeWorkbenchSavedReplay, normalizeWorkbenchWorkspace } from '../contracts'

const scenario = computed(() => new URLSearchParams(window.location.search).get('scenario') || 'success')
const successArtifact = '这是一段专门用于CJK自动换行验证的超长文案示例没有空格也应该在卡片内自然折行避免按钮和卡片被撑破同时保留可读性与节奏感。'

function createRawService(mode: string) {
  if (mode === 'loading') {
    return {
      loadWorkspace: () => new Promise(() => {}),
      createRun: async () => ({ id: 'run-loading', status: 'running', capability_id: 'cap-copy' }),
      refreshRun: async () => ({ id: 'run-loading', status: 'running', capability_id: 'cap-copy' }),
      cancelRun: async () => ({ id: 'run-loading', status: 'cancelled', capability_id: 'cap-copy' }),
      saveRun: async () => ({ id: 'save-loading', label: 'loading', capability_id: 'cap-copy', intent: 'loading' }),
      replayRun: async () => ({ id: 'run-loading', status: 'running', capability_id: 'cap-copy' }),
    }
  }
  if (mode === 'error') {
    return {
      loadWorkspace: async () => {
        throw new Error('模拟网关暂时不可用，请稍后再试')
      },
      createRun: async () => ({ id: 'run-error', status: 'failed', capability_id: 'cap-copy' }),
      refreshRun: async () => ({ id: 'run-error', status: 'failed', capability_id: 'cap-copy' }),
      cancelRun: async () => ({ id: 'run-error', status: 'cancelled', capability_id: 'cap-copy' }),
      saveRun: async () => ({ id: 'save-error', label: 'error', capability_id: 'cap-copy', intent: 'error' }),
      replayRun: async () => ({ id: 'run-error', status: 'running', capability_id: 'cap-copy' }),
    }
  }
  if (mode === 'cancel') {
    return {
      loadWorkspace: async () => ({
        workspace_token: 'wb_tok_cancel',
        workspace_version: 4,
        draft: { intent: '请生成一段用于取消态验证的文案。', capability_id: 'cap-copy', locale: 'zh-CN', updated_at: '2026-08-19T10:05:00Z' },
        capabilities: [
          { id: 'cap-copy', version: '1', canonical_model_id: 'gpt-5.5', canonical_model_version: '2026-08-19', capability_digest: 'sha256:cap-copy', title: '品牌文案生成', summary: '输出短文案、活动标题与 CTA。', token_budget: 12000, average_seconds: 18, price: { currency: 'CNY', amount: '0.28', unit_label: '每次', accepted_quote_id: 'quote-copy', accepted_quote_sha: 'sha256:quote-copy' } },
        ],
        current_run: {
          id: 'run-cancel',
          status: 'running',
          capability_id: 'cap-copy',
          intent: '请生成一段用于取消态验证的文案。',
          requested_at: '2026-08-19T10:06:00Z',
          updated_at: '2026-08-19T10:06:00Z',
          cost: { currency: 'CNY', estimate: '0.28', actual: null, explanation: '运行中，等待最终结算。' },
          cancellable: true,
        },
        saved_replays: [],
      }),
      createRun: async () => ({ id: 'run-cancel', status: 'running', capability_id: 'cap-copy' }),
      refreshRun: async () => ({ id: 'run-cancel', status: 'running', capability_id: 'cap-copy' }),
      cancelRun: async () => ({
        id: 'run-cancel',
        status: 'cancelled',
        capability_id: 'cap-copy',
        intent: '请生成一段用于取消态验证的文案。',
        requested_at: '2026-08-19T10:06:00Z',
        updated_at: '2026-08-19T10:07:00Z',
        cost: { currency: 'CNY', estimate: '0.28', actual: '0.12', explanation: '在取消前已产生部分 token 成本。' },
        retryable: true,
      }),
      saveRun: async () => ({ id: 'save-cancel', label: 'cancel', capability_id: 'cap-copy', intent: 'cancel' }),
      replayRun: async () => ({ id: 'run-cancel-replay', status: 'running', capability_id: 'cap-copy' }),
    }
  }
  return {
    loadWorkspace: async () => ({
      workspace_token: 'wb_tok_success',
      workspace_version: 9,
      draft: { intent: '请生成一段面向中文创作者的首页卡片文案，强调速度、成本透明和可回放。', capability_id: 'cap-copy', locale: 'zh-CN', updated_at: '2026-08-19T10:00:00Z' },
      capabilities: [
        { id: 'cap-copy', version: '1', canonical_model_id: 'gpt-5.5', canonical_model_version: '2026-08-19', capability_digest: 'sha256:cap-copy', title: '品牌文案生成', summary: '输出短文案、活动标题与 CTA。', token_budget: 12000, average_seconds: 18, price: { currency: 'CNY', amount: '0.28', unit_label: '每次', accepted_quote_id: 'quote-copy', accepted_quote_sha: 'sha256:quote-copy' } },
        { id: 'cap-image', version: '1', canonical_model_id: 'gpt-image-2', canonical_model_version: '2026-08-19', capability_digest: 'sha256:cap-image', title: '封面提示词打样', summary: '输出图片提示词与构图说明。', token_budget: 24000, average_seconds: 32, price: { currency: 'CNY', amount: '0.66', unit_label: '每次', accepted_quote_id: 'quote-image', accepted_quote_sha: 'sha256:quote-image' } },
      ],
      current_run: {
        id: 'run-success',
        status: 'succeeded',
        capability_id: 'cap-copy',
        intent: '请生成一段面向中文创作者的首页卡片文案，强调速度、成本透明和可回放。',
        requested_at: '2026-08-19T10:01:00Z',
        updated_at: '2026-08-19T10:02:00Z',
        cost: { currency: 'CNY', estimate: '0.28', actual: '0.26', explanation: '实际输出更短，因此最终成本低于预估。' },
        saveable: true,
        replayable: true,
        artifact: { kind: 'text', title: '首页首屏文案', preview: successArtifact, download_label: '复制文本' },
      },
      saved_replays: [
        { id: 'save-001', label: '首页首屏文案', capability_id: 'cap-copy', intent: successArtifact, saved_at: '2026-08-19T10:03:00Z' },
      ],
    }),
    createRun: async () => ({ id: 'run-success-new', status: 'running', capability_id: 'cap-copy' }),
    refreshRun: async () => ({ id: 'run-success-new', status: 'succeeded', capability_id: 'cap-copy' }),
    cancelRun: async () => ({ id: 'run-success-new', status: 'cancelled', capability_id: 'cap-copy' }),
    saveRun: async () => ({ id: 'save-002', label: '再次保存的文案', capability_id: 'cap-copy', intent: successArtifact, saved_at: '2026-08-19T10:04:00Z' }),
    replayRun: async () => ({ id: 'run-success-replay', status: 'running', capability_id: 'cap-copy' }),
  }
}
function createService(mode: string): CreatorWorkbenchService {
  const raw = createRawService(mode)
  const run = (value: unknown) => {
    const parsed = normalizeWorkbenchRun(value, [])
    if (!parsed) throw new Error('Invalid harness run fixture')
    return parsed
  }
  return {
    loadWorkspace: async () => normalizeWorkbenchWorkspace(await raw.loadWorkspace()),
    createRun: async () => run(await raw.createRun()),
    refreshRun: async () => run(await raw.refreshRun()),
    cancelRun: async () => run(await raw.cancelRun()),
    replayRun: async () => run(await raw.replayRun()),
    forkRun: async () => run(await raw.createRun()),
    saveRun: async () => {
      const parsed = normalizeWorkbenchSavedReplay(await raw.saveRun(), [])
      if (!parsed) throw new Error('Invalid harness replay fixture')
      return parsed
    },
    streamRun: async input => ({ kind: input.signal.aborted ? 'aborted' : 'exhausted', cursor: input.cursor }),
  }
}
</script>

<template>
  <div>
    <div class="harness-nav">
      <a href="?scenario=success">success</a>
      <a href="?scenario=studio">studio</a>
      <a href="?scenario=loading">loading</a>
      <a href="?scenario=error">error</a>
      <a href="?scenario=cancel">cancel</a>
    </div>
    <StudioWorkbenchView v-if="scenario === 'studio'" :service="createService('success')" />
    <CreatorWorkbenchView v-else :service="createService(scenario)" />
  </div>
</template>

<style scoped>
.harness-nav {
  --harness-line: var(--module-line, color-mix(in srgb, var(--module-ink) 16%, transparent));
  --harness-surface: var(--module-panel-raised, var(--module-panel));
  --harness-ink: var(--module-ink-strong, var(--module-ink));
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  gap: 0.75rem;
  padding: 0.9rem 1rem;
  border-bottom: 1px solid var(--harness-line);
  background: color-mix(in srgb, var(--harness-surface) 88%, transparent);
  backdrop-filter: blur(8px);
}

.harness-nav a {
  color: var(--harness-ink);
}
</style>
