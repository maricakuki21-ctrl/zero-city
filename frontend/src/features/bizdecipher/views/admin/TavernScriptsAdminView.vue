<template>
  <AppLayout>
    <div class="admin-tavern-page">
      <header class="admin-page-header">
        <div>
          <span class="admin-kicker">酒馆内容治理</span>
          <h1>酒馆剧本审核</h1>
          <p>核对剧本内容、游玩规格、定价与质量评分，决定公开上架、驳回或下架。</p>
        </div>
        <button type="button" class="admin-refresh-button" @click="loadScripts">
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          刷新
        </button>
      </header>

      <section class="admin-filter-bar">
        <div class="admin-search">
          <Icon name="search" size="sm" />
          <input v-model="keyword" type="search" placeholder="搜索标题、作者、简介" @keyup.enter="loadScripts" />
        </div>
        <select v-model="filterStatus" class="admin-select" @change="loadScripts">
          <option value="all">全部状态</option>
          <option value="pending">待审核</option>
          <option value="listed">已上架</option>
          <option value="rejected">已拒绝</option>
          <option value="draft">草稿</option>
          <option value="archived">已归档</option>
          <option value="delisted">已下架</option>
        </select>
        <select v-model="filterGenre" class="admin-select" @change="loadScripts">
          <option value="all">全部题材</option>
          <option v-for="opt in genreOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
        </select>
      </section>

      <section class="admin-stats-bar">
        <div class="admin-stat-item">
          <span>待审核</span>
          <strong class="stat-pending">{{ statusCounts.pending }}</strong>
        </div>
        <div class="admin-stat-item">
          <span>已上架</span>
          <strong class="stat-listed">{{ statusCounts.listed }}</strong>
        </div>
        <div class="admin-stat-item">
          <span>已拒绝</span>
          <strong class="stat-rejected">{{ statusCounts.rejected }}</strong>
        </div>
        <div class="admin-stat-item">
          <span>总计</span>
          <strong>{{ scripts.length }}</strong>
        </div>
      </section>

      <section class="admin-table-section">
        <div v-if="loading" class="admin-loading">
          <Icon name="refresh" size="lg" class="animate-spin" />
          <span>加载中...</span>
        </div>

        <div v-else-if="scripts.length === 0" class="admin-empty">
          <Icon name="grid" size="xl" />
          <p>没有符合条件的酒馆剧本</p>
        </div>

        <table v-else class="admin-table">
          <thead>
            <tr>
              <th>剧本</th>
              <th>题材</th>
              <th>作者</th>
              <th>状态</th>
              <th>规格</th>
              <th>质量分</th>
              <th>提交时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="script in scripts" :key="script.id">
              <td class="col-script">
                <div class="cell-script">
                  <div class="cell-script-mark">{{ genreLabel(script.genre).slice(0, 2) }}</div>
                  <div class="cell-script-info">
                    <strong>{{ script.title }}</strong>
                    <span>{{ script.summary || '暂无简介' }}</span>
                  </div>
                </div>
              </td>
              <td><span class="cell-type-pill">{{ genreLabel(script.genre) }}</span></td>
              <td class="col-author">{{ script.author || '-' }}</td>
              <td>
                <span class="cell-status-pill" :data-status="script.status">{{ statusLabel(script.status) }}</span>
              </td>
              <td class="col-spec">{{ script.player_min }}-{{ script.player_max }} 人 / {{ script.estimated_minutes }} 分钟</td>
              <td class="col-score">{{ formatScore(script.quality_score) }}</td>
              <td class="col-date">{{ formatDate(script.created_at) }}</td>
              <td class="col-actions">
                <button type="button" class="action-button action-view" @click="openReviewPanel(script)">
                  <Icon name="eye" size="sm" />
                  审核
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </section>

      <Transition name="drawer-slide">
        <div v-if="reviewPanelVisible" class="review-overlay" @click.self="closeReviewPanel">
          <aside class="review-panel">
            <header class="review-header">
              <div>
                <div class="review-header-pills">
                  <span class="cell-type-pill">{{ genreLabel(reviewScript?.genre || 'other') }}</span>
                  <span class="cell-status-pill" :data-status="reviewScript?.status">{{ statusLabel(reviewScript?.status || 'pending') }}</span>
                </div>
                <h2>{{ reviewScript?.title }}</h2>
                <p>作者: {{ reviewScript?.author || '-' }} | 提交: {{ formatDate(reviewScript?.created_at) }}</p>
              </div>
              <button type="button" class="review-close" @click="closeReviewPanel">
                <Icon name="x" size="sm" />
              </button>
            </header>

            <div v-if="reviewScript" class="review-body">
              <div class="review-meta-grid">
                <div>
                  <span>人数</span>
                  <strong>{{ reviewScript.player_min }}-{{ reviewScript.player_max }} 人</strong>
                </div>
                <div>
                  <span>时长</span>
                  <strong>{{ reviewScript.estimated_minutes }} 分钟</strong>
                </div>
                <div>
                  <span>难度</span>
                  <strong>{{ difficultyLabel(reviewScript.difficulty) }}</strong>
                </div>
                <div>
                  <span>计费</span>
                  <strong>{{ pricingLabel(reviewScript) }}</strong>
                </div>
              </div>

              <div v-if="reviewScript.summary" class="review-section">
                <h4>简介</h4>
                <p>{{ reviewScript.summary }}</p>
              </div>

              <div v-if="reviewScript.description" class="review-section">
                <h4>详细描述</h4>
                <pre>{{ reviewScript.description }}</pre>
              </div>

              <div v-if="reviewScript.tags?.length" class="review-section">
                <h4>标签</h4>
                <div class="review-tags">
                  <span v-for="tag in reviewScript.tags" :key="tag" class="review-tag">{{ tag }}</span>
                </div>
              </div>

              <div v-if="reviewScript.npc_cards?.length" class="review-section">
                <h4>NPC 卡</h4>
                <div class="review-tags">
                  <span v-for="npc in reviewScript.npc_cards" :key="npc" class="review-tag review-tag-npc">{{ npc }}</span>
                </div>
              </div>

              <div v-if="reviewScript.host_brief" class="review-section">
                <h4>主持人说明</h4>
                <pre>{{ reviewScript.host_brief }}</pre>
              </div>

              <div v-if="reviewScript.opening_prompt" class="review-section">
                <h4>开场提示</h4>
                <pre>{{ reviewScript.opening_prompt }}</pre>
              </div>

              <div v-if="reviewScript.safety_notes" class="review-section">
                <h4>安全边界</h4>
                <pre>{{ reviewScript.safety_notes }}</pre>
              </div>

              <div class="review-form">
                <h4>审核操作</h4>

                <div class="review-form-row">
                  <label>审核结果</label>
                  <div class="review-action-buttons">
                    <button
                      type="button"
                      :class="['review-action-btn', reviewStatus === 'listed' && 'active']"
                      @click="reviewStatus = 'listed'"
                    >
                      <Icon name="check" size="sm" />
                      通过上架
                    </button>
                    <button
                      type="button"
                      :class="['review-action-btn', 'review-action-reject', reviewStatus === 'rejected' && 'active']"
                      @click="reviewStatus = 'rejected'"
                    >
                      <Icon name="x" size="sm" />
                      拒绝
                    </button>
                    <button
                      type="button"
                      :class="['review-action-btn', 'review-action-delist', reviewStatus === 'delisted' && 'active']"
                      @click="reviewStatus = 'delisted'"
                    >
                      <Icon name="x" size="sm" />
                      下架
                    </button>
                  </div>
                </div>

                <div class="review-form-row">
                  <label>质量分 0-100</label>
                  <input v-model.number="qualityScore" type="number" min="0" max="100" step="0.5" />
                </div>

                <div class="review-form-row">
                  <label>审核备注</label>
                  <textarea v-model="reviewNote" rows="3" placeholder="给投稿人的审核反馈，或内部备注..."></textarea>
                </div>

                <div class="review-form-submit">
                  <button
                    type="button"
                    class="admin-submit-button"
                    :disabled="submitting || !reviewStatus"
                    @click="handleReview"
                  >
                    {{ submitting ? '处理中...' : '确认审核' }}
                  </button>
                </div>
              </div>
            </div>
          </aside>
        </div>
      </Transition>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  adminListTavernScripts,
  adminReviewTavernScript,
  type TavernScript,
  type TavernScriptReviewPayload,
} from '@/features/bizdecipher/api/bizdecipher'

const appStore = useAppStore()

const loading = ref(false)
const scripts = ref<TavernScript[]>([])
const keyword = ref('')
const filterStatus = ref<string>('pending')
const filterGenre = ref<string>('all')

const genreOptions = [
  { value: 'mystery', label: '推理' },
  { value: 'sci_fi', label: '科幻' },
  { value: 'fantasy', label: '奇幻' },
  { value: 'horror', label: '惊悚' },
  { value: 'workplace', label: '职场' },
  { value: 'historical', label: '历史' },
  { value: 'open_world', label: '开放世界' },
  { value: 'other', label: '其他' },
]

const statusCounts = computed(() => {
  const counts: Record<string, number> = { pending: 0, listed: 0, rejected: 0, draft: 0, archived: 0, delisted: 0 }
  for (const script of scripts.value) {
    if (counts[script.status] !== undefined) counts[script.status]++
  }
  return counts
})

async function loadScripts() {
  loading.value = true
  try {
    scripts.value = await adminListTavernScripts({
      keyword: keyword.value || undefined,
      status: filterStatus.value === 'all' ? undefined : filterStatus.value,
      genre: filterGenre.value === 'all' ? undefined : filterGenre.value,
      sort: 'updated',
      limit: 200,
    })
  } catch {
    appStore.showError('加载酒馆剧本列表失败')
  } finally {
    loading.value = false
  }
}

const reviewPanelVisible = ref(false)
const reviewScript = ref<TavernScript | null>(null)
const reviewStatus = ref<'listed' | 'rejected' | 'delisted'>('listed')
const reviewNote = ref('')
const qualityScore = ref<number | null>(null)
const submitting = ref(false)

function openReviewPanel(script: TavernScript) {
  reviewScript.value = script
  reviewStatus.value = 'listed'
  reviewNote.value = script.review_note || ''
  qualityScore.value = typeof script.quality_score === 'number' ? script.quality_score : null
  reviewPanelVisible.value = true
}

function closeReviewPanel() {
  reviewPanelVisible.value = false
  reviewScript.value = null
}

async function handleReview() {
  if (!reviewScript.value || !reviewStatus.value) return
  submitting.value = true
  try {
    const payload: TavernScriptReviewPayload = {
      status: reviewStatus.value,
      review_note: reviewNote.value || undefined,
    }
    if (typeof qualityScore.value === 'number' && Number.isFinite(qualityScore.value)) {
      payload.quality_score = qualityScore.value
    }
    await adminReviewTavernScript(reviewScript.value.id, payload)
    appStore.showSuccess('酒馆剧本审核已完成')
    closeReviewPanel()
    await loadScripts()
  } catch {
    appStore.showError('酒馆剧本审核失败')
  } finally {
    submitting.value = false
  }
}

function genreLabel(genre: string): string {
  const found = genreOptions.find(o => o.value === genre)
  return found?.label ?? '其他'
}

function statusLabel(status: string): string {
  const map: Record<string, string> = {
    draft: '草稿',
    pending: '待审核',
    listed: '已上架',
    rejected: '已拒绝',
    archived: '已归档',
    delisted: '已下架',
  }
  return map[status] ?? status
}

function difficultyLabel(difficulty: string): string {
  const map: Record<string, string> = {
    easy: '轻量',
    normal: '标准',
    hard: '困难',
    expert: '专家',
  }
  return map[difficulty] ?? difficulty
}

function pricingLabel(script: TavernScript): string {
  if (script.pricing_mode === 'free') return '免费'
  if (script.pricing_mode === 'credit') return `${script.entry_credit_cost || 0} 积分`
  if (script.pricing_mode === 'balance') return `¥${script.entry_balance_cost || 0}`
  if (script.pricing_mode === 'hybrid') return `${script.entry_credit_cost || 0} 积分 / ¥${script.entry_balance_cost || 0}`
  return script.pricing_mode || '--'
}

function formatScore(score?: number | null): string {
  if (typeof score !== 'number' || !Number.isFinite(score)) return '--'
  return score.toFixed(1)
}

function formatDate(dateStr?: string | null): string {
  if (!dateStr) return '--'
  const date = new Date(dateStr)
  if (Number.isNaN(date.getTime())) return '--'
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

onMounted(() => {
  loadScripts()
})
</script>

<style scoped>
.admin-tavern-page {
  --admin-accent: var(--module-accent, var(--zc-accent));
  --admin-accent-ink: var(--module-accent-ink, var(--zc-accent-ink));
  --admin-canvas: var(--module-canvas, var(--zc-bg));
  --admin-panel: var(--module-panel, var(--zc-card-strong));
  --admin-text: var(--module-text, var(--zc-text-strong));
  --admin-muted: var(--module-muted, var(--zc-muted));
  --admin-subtle: var(--module-subtle, var(--zc-subtle));
  --admin-line: var(--module-line, var(--zc-line));
  max-width: 1200px;
  margin: 0 auto;
  padding: 24px 20px 60px;
  color: var(--admin-text);
}

.admin-page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 24px;
}
.admin-kicker {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--admin-accent);
}
.admin-page-header h1 { font-size: 24px; font-weight: 700; margin: 4px 0 6px; }
.admin-page-header p { font-size: 13px; color: var(--admin-muted); }
.admin-refresh-button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 13px;
  background: var(--admin-panel);
  color: var(--admin-text);
  border: 1px solid var(--admin-line);
  box-shadow: var(--module-shadow-soft, var(--zc-shadow-soft));
  cursor: pointer;
}
.admin-refresh-button:hover { border-color: var(--admin-accent); color: var(--admin-accent); }

.admin-filter-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.admin-search {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border: 1px solid var(--admin-line);
  border-radius: 8px;
  background: color-mix(in srgb, var(--admin-panel) 82%, var(--admin-canvas));
  box-shadow: var(--module-shadow-inset, var(--zc-shadow-inset));
  flex: 1;
  min-width: 220px;
}
.admin-search input {
  background: transparent;
  border: none;
  outline: none;
  color: var(--admin-text);
  font-size: 13px;
  width: 100%;
}
.admin-select {
  padding: 6px 12px;
  border: 1px solid var(--admin-line);
  border-radius: 8px;
  background: var(--admin-panel);
  color: var(--admin-text);
  font-size: 13px;
  cursor: pointer;
}

.admin-stats-bar {
  display: flex;
  gap: 24px;
  margin-bottom: 20px;
  padding: 12px 20px;
  border: 1px solid var(--admin-line);
  border-radius: 10px;
  background: var(--admin-panel);
  box-shadow: var(--module-shadow-soft, var(--zc-shadow-soft));
}
.admin-stat-item { display: flex; flex-direction: column; gap: 2px; }
.admin-stat-item span { font-size: 11px; color: var(--admin-muted); }
.admin-stat-item strong { font-size: 20px; font-weight: 700; }
.stat-pending { color: #f59e0b; }
.stat-listed { color: #22c55e; }
.stat-rejected { color: #ef4444; }

.admin-table-section { overflow-x: auto; }
.admin-loading, .admin-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 60px;
  color: var(--admin-muted);
}
.admin-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.admin-table th {
  text-align: left;
  padding: 10px 12px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--admin-muted);
  border-bottom: 1px solid var(--admin-line);
}
.admin-table td {
  padding: 12px;
  border-bottom: 1px solid var(--admin-line);
  vertical-align: middle;
}
.admin-table tr:hover td { background: color-mix(in srgb, var(--admin-panel) 68%, var(--admin-canvas)); }
.col-script { min-width: 300px; }
.cell-script { display: flex; gap: 10px; align-items: center; }
.cell-script-mark {
  width: 42px;
  height: 42px;
  border-radius: 8px;
  flex-shrink: 0;
  background: rgba(20, 184, 166, 0.12);
  color: #14b8a6;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 700;
}
.cell-script-info { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.cell-script-info strong { font-size: 13px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cell-script-info span { font-size: 11px; color: var(--admin-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 260px; }
.col-author, .col-spec, .col-score { font-size: 12px; color: var(--admin-muted); white-space: nowrap; }
.col-date { font-size: 12px; color: var(--admin-subtle); white-space: nowrap; }
.col-actions { white-space: nowrap; }

.cell-type-pill {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--admin-panel) 82%, var(--admin-canvas));
  color: var(--admin-muted);
  white-space: nowrap;
}
.cell-status-pill {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
  white-space: nowrap;
}
.cell-status-pill[data-status="draft"] { background: color-mix(in srgb, var(--admin-panel) 82%, var(--admin-canvas)); color: var(--admin-muted); }
.cell-status-pill[data-status="pending"] { background: rgba(245, 158, 11, 0.12); color: #f59e0b; }
.cell-status-pill[data-status="listed"] { background: rgba(34, 197, 94, 0.12); color: #22c55e; }
.cell-status-pill[data-status="rejected"] { background: rgba(239, 68, 68, 0.12); color: #ef4444; }
.cell-status-pill[data-status="archived"] { background: color-mix(in srgb, var(--admin-panel) 82%, var(--admin-canvas)); color: var(--admin-subtle); }
.cell-status-pill[data-status="delisted"] { background: rgba(239, 68, 68, 0.12); color: #ef4444; }

.action-button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 5px 12px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
  border: 1px solid var(--admin-line);
  background: transparent;
  color: var(--admin-text);
  cursor: pointer;
  transition: all 0.15s;
}
.action-view:hover { border-color: var(--admin-accent); color: var(--admin-accent); }

.review-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: flex-end;
  z-index: 1000;
}
.review-panel {
  width: min(560px, 92vw);
  height: 100%;
  background: var(--admin-canvas);
  border-left: 1px solid var(--admin-line);
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}
.review-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 20px 24px;
  border-bottom: 1px solid var(--admin-line);
  position: sticky;
  top: 0;
  background: var(--admin-canvas);
  z-index: 1;
}
.review-header-pills { display: flex; gap: 6px; margin-bottom: 8px; }
.review-header h2 { font-size: 18px; font-weight: 700; }
.review-header p { font-size: 12px; color: var(--admin-muted); margin-top: 4px; }
.review-close {
  background: transparent;
  border: none;
  color: var(--admin-muted);
  cursor: pointer;
  padding: 4px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.review-close:hover { background: color-mix(in srgb, var(--admin-panel) 68%, var(--admin-canvas)); }
.review-body { padding: 20px 24px; display: flex; flex-direction: column; gap: 18px; }
.review-meta-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}
.review-meta-grid div {
  padding: 10px 12px;
  border: 1px solid var(--admin-line);
  border-radius: 8px;
  background: var(--admin-panel);
}
.review-meta-grid span { display: block; font-size: 11px; color: var(--admin-muted); margin-bottom: 4px; }
.review-meta-grid strong { font-size: 13px; font-weight: 600; }
.review-section h4 { font-size: 12px; font-weight: 600; margin-bottom: 6px; color: var(--admin-muted); }
.review-section p { font-size: 13px; line-height: 1.6; color: var(--admin-text); }
.review-section pre {
  font-family: inherit;
  font-size: 13px;
  line-height: 1.6;
  color: var(--admin-text);
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
}
.review-tags { display: flex; gap: 6px; flex-wrap: wrap; }
.review-tag {
  font-size: 10px;
  padding: 2px 8px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--admin-panel) 82%, var(--admin-canvas));
  color: var(--admin-muted);
}
.review-tag-npc { background: rgba(20, 184, 166, 0.1); color: #14b8a6; }

.review-form {
  padding: 16px;
  border: 1px solid var(--admin-line);
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.review-form h4 { font-size: 13px; font-weight: 600; }
.review-form-row { display: flex; flex-direction: column; gap: 6px; }
.review-form-row label { font-size: 12px; font-weight: 500; color: var(--admin-muted); display: flex; align-items: center; gap: 6px; }
.review-form-row input[type="number"],
.review-form-row textarea {
  padding: 8px 12px;
  border: 1px solid var(--admin-line);
  border-radius: 8px;
  background: var(--admin-panel);
  color: var(--admin-text);
  font-size: 13px;
  outline: none;
}
.review-form-row input:focus,
.review-form-row textarea:focus { border-color: var(--admin-accent); }
.review-form-row textarea { resize: vertical; font-family: inherit; }
.review-action-buttons { display: flex; gap: 8px; flex-wrap: wrap; }
.review-action-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  border: 1px solid var(--admin-line);
  background: transparent;
  color: var(--admin-text);
  cursor: pointer;
  transition: all 0.15s;
}
.review-action-btn.active { border-color: var(--admin-accent); background: rgba(99, 102, 241, 0.1); color: var(--admin-accent); }
.review-action-reject.active,
.review-action-delist.active { border-color: #ef4444; background: rgba(239, 68, 68, 0.1); color: #ef4444; }
.review-form-submit { display: flex; justify-content: flex-end; }
.admin-submit-button {
  padding: 10px 24px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  background: var(--admin-accent);
  color: var(--admin-accent-ink);
  border: none;
  cursor: pointer;
  transition: opacity 0.15s;
}
.admin-submit-button:hover { opacity: 0.88; }
.admin-submit-button:disabled { opacity: 0.5; cursor: not-allowed; }
.drawer-slide-enter-active, .drawer-slide-leave-active { transition: opacity 0.2s; }
.drawer-slide-enter-active .review-panel, .drawer-slide-leave-active .review-panel { transition: transform 0.25s ease; }
.drawer-slide-enter-from, .drawer-slide-leave-to { opacity: 0; }
.drawer-slide-enter-from .review-panel, .drawer-slide-leave-to .review-panel { transform: translateX(100%); }
.animate-spin { animation: spin 1s linear infinite; }
@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }

@media (max-width: 720px) {
  .admin-page-header { align-items: flex-start; flex-direction: column; gap: 12px; }
  .admin-stats-bar { gap: 14px; overflow-x: auto; }
  .review-meta-grid { grid-template-columns: 1fr; }
}
</style>
