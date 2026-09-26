<template>
  <AppLayout>
    <main class="wb-page" data-visual-domain="asset">
      <section class="wb-shell">
        <header class="wb-hero zc-material-scene">
          <div>
            <p class="wb-kicker">BizDecipher Creator Workbench</p>
            <h1>从创作意图到工件回放的最小闭环</h1>
            <p class="wb-copy">
              这个切片专注于创作者路径：意图、能力、运行状态、费用说明、结果工件，以及保存 / 回放。
            </p>
            <p v-if="sourceAssetLabel" data-testid="workbench-entry-source" class="wb-copy">
              {{ sourceAssetLabel }}
            </p>
          </div>
          <div class="wb-hero-side">
            <span class="wb-token-pill">token: {{ workspaceToken }}</span>
            <span v-if="dirty" data-testid="workbench-dirty-pill" class="wb-token-pill is-dirty">未保存更改</span>
            <span v-else class="wb-token-pill">草稿同步于 {{ draftUpdatedAt }}</span>
          </div>
        </header>

        <section v-if="bootstrapLoading" class="wb-banner">
          正在加载工作台…
        </section>

        <section v-else-if="bootstrapError" class="wb-banner is-error" data-testid="workbench-alert" role="alert">
          <div>
            <strong>工作台加载失败</strong>
            <p>{{ bootstrapError }}</p>
          </div>
          <button data-testid="workbench-reload" type="button" class="btn btn-secondary" @click="loadWorkspace">
            重新加载
          </button>
        </section>

        <template v-else>
          <section v-if="warnings.length || actionError || statusMessage" class="wb-banner-stack">
            <article
              v-for="warning in warnings"
              :key="warning"
              class="wb-banner is-warning"
              data-testid="workbench-alert"
              role="status"
            >
              {{ warning }}
            </article>
            <article v-if="statusMessage" class="wb-banner is-success" data-testid="workbench-alert" role="status">
              {{ statusMessage }}
            </article>
            <article v-if="actionError" class="wb-banner is-error" data-testid="workbench-alert" role="alert">
              {{ actionError }}
            </article>
            <article v-if="streamError" class="wb-banner is-error" data-testid="workbench-stream-error" role="alert">
              {{ streamError }}
            </article>
          </section>

          <section class="wb-grid">
            <WorkbenchPanel title="创作意图" kicker="intent">
              <label class="wb-label" for="wb-intent">要让这次运行完成什么结果</label>
              <textarea
                id="wb-intent"
                v-model="intent"
                data-testid="workbench-intent"
                class="wb-textarea wb-break-anywhere"
                aria-label="创作意图输入"
                rows="6"
                placeholder="例如：给首页英雄卡片写一段面向中文用户的文案，并说明为什么值得点击。"
              />
              <div class="wb-field-row">
                <label class="wb-inline-field">
                  <span>输出语言</span>
                  <select v-model="locale" class="wb-select" aria-label="输出语言">
                    <option value="zh-CN">简体中文</option>
                    <option value="en-US">English</option>
                    <option value="ja-JP">日本語</option>
                  </select>
                </label>
                <p class="wb-field-hint">长句与 CJK 文本默认开启任意换行，避免卡片、表单与工件预览被撑破。</p>
              </div>
            </WorkbenchPanel>

            <WorkbenchPanel title="能力与成本" kicker="capability">
              <label class="wb-label" for="wb-capability">选择一项执行能力</label>
              <select
                id="wb-capability"
                v-model="capabilityId"
                data-testid="workbench-capability-select"
                class="wb-select"
                aria-label="能力选择"
              >
                <option v-for="item in capabilities" :key="item.id" :value="item.id">
                  {{ item.title }}
                </option>
              </select>
              <article v-if="selectedCapability" class="wb-capability-card">
                <strong>{{ selectedCapability.title }}</strong>
                <p class="wb-copy wb-break-anywhere">{{ selectedCapability.summary }}</p>
                <ul class="wb-bullet-list">
                  <li>预估成本：{{ formatMoney(selectedCapability.price ?? null) }} / {{ selectedCapability.price?.unitLabel || '每次' }}</li>
                  <li>预计时长：{{ selectedCapability.averageSeconds }} 秒</li>
                  <li>token 预算：{{ (selectedCapability.tokenBudget ?? 0).toLocaleString('zh-CN') }}</li>
                </ul>
              </article>
              <p v-else class="wb-empty-note">当前工作区没有可用能力，请稍后重试。</p>
              <p class="wb-cost-preview">{{ costPreview }}</p>
              <button
                data-testid="workbench-launch"
                type="button"
                class="btn btn-primary"
                :disabled="!canLaunch"
                @click="launchRun()"
              >
                {{ launchPending ? '正在发起运行…' : '发起运行' }}
              </button>
            </WorkbenchPanel>

            <WorkbenchPanel title="运行状态" kicker="run">
              <template #meta>
                <WorkbenchStatusPill
                  v-if="currentRun"
                  :status="currentRun.status"
                  :label="runStatusText"
                />
              </template>

              <WorkbenchStepRail :active-index="stepIndex" />

              <div v-if="currentRun" class="wb-run-box">
                <div class="wb-run-topline">
                  <strong data-testid="workbench-run-status">{{ runStatusText }}</strong>
                  <span>{{ currentRun.capabilityLabel }}</span>
                </div>
                <p class="wb-copy wb-break-anywhere">{{ currentRun.intent }}</p>
                <p class="wb-run-meta">{{ currentRunCost }}</p>
                <p data-testid="workbench-connection" class="wb-run-meta">连接：{{ connectionState }}</p>
                <p v-if="currentRun.staleCapability" class="wb-run-warning">需重新选择能力：当前运行只保留历史证据，不建议直接复用。</p>
                <p v-if="currentRun.failureMessage" class="wb-run-warning">{{ currentRun.failureMessage }}</p>
                <div class="wb-action-row">
                  <button
                    v-if="canCancel"
                    data-testid="workbench-cancel"
                    type="button"
                    class="btn btn-secondary"
                    :disabled="cancelPending"
                    @click="cancelRun"
                  >
                    {{ cancelPending ? '正在取消…' : '取消运行' }}
                  </button>
                  <button
                    v-if="canFork"
                    data-testid="workbench-fork"
                    type="button"
                    class="btn btn-secondary"
                    @click="forkRun"
                  >
                    派生运行
                  </button>
                  <button
                    v-if="connectionState === 'exhausted' || connectionState === 'offline'"
                    data-testid="workbench-reconnect"
                    type="button"
                    class="btn btn-secondary"
                    @click="reconnect"
                  >
                    重新连接
                  </button>
                  <button
                    v-if="canRetry"
                    data-testid="workbench-retry"
                    type="button"
                    class="btn btn-secondary"
                    :disabled="launchPending"
                    @click="retryRun"
                  >
                    {{ launchPending ? '正在重试…' : '重试' }}
                  </button>
                  <button
                    v-if="canSave"
                    data-testid="workbench-save"
                    type="button"
                    class="btn btn-primary"
                    :disabled="savePending"
                    @click="saveRun"
                  >
                    {{ savePending ? '正在保存…' : '保存结果' }}
                  </button>
                </div>
              </div>
              <p v-else class="wb-empty-note">先确定意图并选择能力，运行状态才会开始流转。</p>
            </WorkbenchPanel>

            <WorkbenchPanel title="结果工件" kicker="artifact">
              <WorkbenchArtifactCard :title="artifactTitle" :artifact-preview="artifactPreview" />
              <WorkbenchLineage v-if="currentRun" :lineage="currentRun.lineage" />
            </WorkbenchPanel>

            <WorkbenchPanel title="已保存回放" kicker="replay">
              <div v-if="savedReplays.length" class="wb-replay-list">
                <article v-for="item in savedReplays" :key="item.id" class="wb-replay-card">
                  <div>
                    <strong>{{ item.label }}</strong>
                    <p class="wb-copy wb-break-anywhere">{{ item.intent }}</p>
                    <small>{{ item.capabilityLabel }} · {{ formatDateTime(item.savedAt) }}</small>
                  </div>
                  <button
                    :data-testid="`workbench-replay-${item.id}`"
                    type="button"
                    class="btn btn-secondary"
                    :disabled="replayingId === item.id"
                    @click="replaySaved(item)"
                  >
                    {{ replayingId === item.id ? '正在回放…' : '重放' }}
                  </button>
                </article>
              </div>
              <p v-else class="wb-empty-note">还没有已保存回放。完成一次运行后，可以把稳定结果留在这里重复使用。</p>
            </WorkbenchPanel>
          </section>
        </template>
      </section>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import WorkbenchArtifactCard from '@/features/bizdecipher/components/workbench/WorkbenchArtifactCard.vue'
import WorkbenchPanel from '@/features/bizdecipher/components/workbench/WorkbenchPanel.vue'
import WorkbenchStatusPill from '@/features/bizdecipher/components/workbench/WorkbenchStatusPill.vue'
import WorkbenchStepRail from '@/features/bizdecipher/components/workbench/WorkbenchStepRail.vue'
import WorkbenchLineage from '@/features/bizdecipher/components/workbench/WorkbenchLineage.vue'
import type { CreatorWorkbenchService } from './contracts'
import { useCreatorWorkbench } from './useCreatorWorkbench'

const props = defineProps<{
  service?: CreatorWorkbenchService
}>()

const {
  artifactPreview,
  artifactTitle,
  bootstrapError,
  bootstrapLoading,
  capabilities,
  capabilityId,
  canCancel,
  canFork,
  canLaunch,
  canRetry,
  canSave,
  cancelPending,
  connectionState,
  costPreview,
  currentRun,
  currentRunCost,
  dirty,
  draftUpdatedAt,
  formatDateTime,
  formatMoney,
  forkRun,
  intent,
  launchPending,
  locale,
  reconnect,
  replayingId,
  runStatusText,
  savedReplays,
  savePending,
  selectedCapability,
  sourceAssetLabel,
  statusMessage,
  stepIndex,
  streamError,
  warnings,
  workspaceToken,
  actionError,
  cancelRun,
  launchRun,
  loadWorkspace,
  replaySaved,
  retryRun,
  saveRun,
} = useCreatorWorkbench(props.service)
</script>

<style scoped>
.wb-page {
  --wb-ink: var(--module-ink);
  --wb-ink-strong: var(--module-ink-strong);
  --wb-muted: var(--module-muted);
  --wb-line: var(--module-line);
  --wb-accent: var(--module-accent);
  --wb-overlay: color-mix(in srgb, var(--module-panel-raised) 9%, transparent);
  --wb-warning-soft: color-mix(in srgb, var(--zc-warning) 12%, transparent);
  --wb-danger-soft: color-mix(in srgb, var(--zc-danger) 12%, transparent);
  --wb-success-soft: color-mix(in srgb, var(--zc-success) 12%, transparent);
  min-height: 100vh;
  background:
    radial-gradient(120% 100% at 0% 0%, color-mix(in srgb, var(--wb-accent) 12%, transparent), transparent 58%),
    linear-gradient(180deg, var(--module-canvas), var(--module-canvas-2));
}

.wb-shell {
  margin: 0 auto;
  max-width: 1440px;
  padding: 1.5rem;
}

.wb-hero {
  display: flex;
  justify-content: space-between;
  gap: 1.25rem;
  align-items: flex-start;
  border-radius: 30px;
  padding: 1.5rem;
}

.wb-kicker {
  margin: 0;
  color: var(--module-art-ink);
  font-size: 0.78rem;
  font-weight: 800;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.wb-hero h1 {
  margin: 0.55rem 0 0;
  font-size: clamp(1.6rem, 2.6vw, 2.5rem);
  font-weight: 900;
}

.wb-copy {
  margin: 0.65rem 0 0;
  color: var(--wb-ink);
  line-height: 1.7;
}

.wb-break-anywhere {
  overflow-wrap: anywhere;
  word-break: break-word;
}

.wb-hero-side {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.75rem;
}

.wb-token-pill {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 0.45rem 0.8rem;
  background: var(--wb-overlay);
  color: var(--module-art-ink);
  font-size: 0.82rem;
  font-weight: 700;
}

.wb-token-pill.is-dirty {
  background: color-mix(in srgb, var(--zc-warning) 18%, transparent);
}

.wb-banner-stack {
  display: grid;
  gap: 0.75rem;
  margin-top: 1rem;
}

.wb-banner {
  margin-top: 1rem;
  border-radius: 20px;
  border: 1px solid var(--wb-line);
  background: var(--wb-overlay);
  color: var(--wb-ink-strong);
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: center;
  padding: 1rem 1.1rem;
}

.wb-banner.is-warning {
  background: var(--wb-warning-soft);
}

.wb-banner.is-error {
  background: var(--wb-danger-soft);
}

.wb-banner.is-success {
  background: var(--wb-success-soft);
}

.wb-banner p,
.wb-banner strong {
  margin: 0;
}

.wb-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
  margin-top: 1rem;
}

.wb-label {
  display: block;
  margin-bottom: 0.55rem;
  color: var(--wb-ink-strong);
  font-size: 0.9rem;
  font-weight: 700;
}

.wb-textarea,
.wb-select {
  width: 100%;
  border-radius: 18px;
  border: 1px solid var(--wb-line);
  background: var(--wb-overlay);
  color: var(--wb-ink);
  padding: 0.9rem 1rem;
}

.wb-textarea:focus,
.wb-select:focus {
  outline: 2px solid color-mix(in srgb, var(--wb-accent) 48%, transparent);
  outline-offset: 2px;
}

.wb-field-row {
  display: grid;
  gap: 0.8rem;
  margin-top: 0.8rem;
}

.wb-inline-field {
  display: grid;
  gap: 0.35rem;
  color: var(--wb-ink);
  font-size: 0.88rem;
}

.wb-field-hint,
.wb-empty-note,
.wb-cost-preview,
.wb-run-meta,
.wb-run-warning {
  margin: 0.8rem 0 0;
  color: var(--wb-muted);
  line-height: 1.6;
}

.wb-capability-card,
.wb-run-box,
.wb-replay-card {
  border-radius: 18px;
  border: 1px solid var(--wb-line);
  background: var(--wb-overlay);
  padding: 1rem;
  margin-top: 0.9rem;
}

.wb-capability-card strong,
.wb-run-topline strong,
.wb-replay-card strong {
  color: var(--wb-ink-strong);
}

.wb-bullet-list {
  margin: 0.8rem 0 0;
  padding-left: 1.1rem;
  color: var(--wb-ink);
  line-height: 1.7;
}

.wb-action-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin-top: 0.95rem;
}

.wb-run-topline {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  align-items: center;
}

.wb-replay-list {
  display: grid;
  gap: 0.85rem;
}

.wb-replay-card {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: flex-start;
}

.wb-replay-card small {
  display: block;
  margin-top: 0.45rem;
  color: var(--wb-muted);
}

@media (max-width: 960px) {
  .wb-grid {
    grid-template-columns: 1fr;
  }

  .wb-hero,
  .wb-banner,
  .wb-replay-card {
    flex-direction: column;
  }

  .wb-hero-side {
    justify-content: flex-start;
  }
}
</style>
