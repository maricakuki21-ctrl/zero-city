<template>
  <AppLayout>
    <div class="admin-capability-assets-page">
      <header class="admin-page-header">
        <div>
          <span class="admin-kicker">能力资产治理</span>
          <h1>能力资产审核</h1>
          <p>核对资产说明、作者信息与复用价值，管理公开上架、精选推荐和下架处置。</p>
        </div>
        <button type="button" class="admin-refresh-button" @click="loadAssets">
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          刷新
        </button>
      </header>

      <!-- Filter bar -->
      <AssetCommercePanel />
      <section class="admin-filter-bar">
        <div class="admin-search">
          <Icon name="search" size="sm" />
          <input v-model="keyword" type="search" placeholder="搜索标题、作者" @keyup.enter="loadAssets" />
        </div>
        <select v-model="filterStatus" class="admin-select" @change="loadAssets">
          <option value="all">全部状态</option>
          <option value="pending">待审核</option>
          <option value="listed">已上架</option>
          <option value="rejected">已拒绝</option>
          <option value="draft">草稿</option>
          <option value="archived">已归档</option>
          <option value="delisted">已下架</option>
        </select>
        <select v-model="filterType" class="admin-select" @change="loadAssets">
          <option value="all">全部类型</option>
          <option v-for="opt in typeOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
        </select>
      </section>

      <!-- Stats -->
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
          <strong>{{ assets.length }}</strong>
        </div>
      </section>

      <!-- Table -->
      <section class="admin-table-section">
        <div v-if="loading" class="admin-loading">
          <Icon name="refresh" size="lg" class="animate-spin" />
          <span>加载中...</span>
        </div>

        <div v-else-if="assets.length === 0" class="admin-empty">
          <Icon name="grid" size="xl" />
          <p>没有符合条件的资产</p>
        </div>

        <table v-else class="admin-table">
          <thead>
            <tr>
              <th>资产</th>
              <th>类型</th>
              <th>作者</th>
              <th>状态</th>
              <th>推荐</th>
              <th>提交时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="asset in assets" :key="asset.id">
              <td class="col-asset">
                <div class="cell-asset">
                  <div class="cell-asset-cover">
                    <img v-if="asset.cover_url" :src="asset.cover_url" :alt="asset.title" />
                    <span v-else>{{ typeLabel(asset.asset_type).slice(0, 2) }}</span>
                  </div>
                  <div class="cell-asset-info">
                    <strong>{{ asset.title }}</strong>
                    <span>{{ asset.summary }}</span>
                  </div>
                </div>
              </td>
              <td><span class="cell-type-pill">{{ typeLabel(asset.asset_type) }}</span></td>
              <td class="col-author">{{ asset.author }}</td>
              <td>
                <span class="cell-status-pill" :data-status="asset.status">{{ statusLabel(asset.status) }}</span>
              </td>
              <td>
                <span v-if="asset.is_featured" class="cell-featured-yes">推荐 #{{ asset.featured_weight }}</span>
                <span v-else class="cell-featured-no">--</span>
              </td>
              <td class="col-date">{{ formatDate(asset.created_at) }}</td>
              <td class="col-actions">
                <button type="button" class="action-button action-view" @click="openReviewPanel(asset)">
                  <Icon name="eye" size="sm" />
                  审核
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </section>

      <!-- Review panel -->
      <Transition name="drawer-slide">
        <div v-if="reviewPanelVisible" class="review-overlay" @click.self="closeReviewPanel">
          <aside class="review-panel">
            <header class="review-header">
              <div>
                <div class="review-header-pills">
                  <span class="cell-type-pill">{{ typeLabel(reviewAsset?.asset_type || 'other') }}</span>
                  <span class="cell-status-pill" :data-status="reviewAsset?.status">{{ statusLabel(reviewAsset?.status || 'pending') }}</span>
                </div>
                <h2>{{ reviewAsset?.title }}</h2>
                <p>作者: {{ reviewAsset?.author }} | 提交: {{ formatDate(reviewAsset?.created_at) }}</p>
              </div>
              <button type="button" class="review-close" @click="closeReviewPanel">
                <Icon name="x" size="sm" />
              </button>
            </header>

            <div v-if="reviewAsset" class="review-body">
              <AssetCommercePanel :asset-id="reviewAsset.id" />
              <div v-if="reviewAsset.cover_url" class="review-cover">
                <img :src="reviewAsset.cover_url" :alt="reviewAsset.title" />
              </div>

              <div v-if="reviewAsset.summary" class="review-section">
                <h4>简介</h4>
                <p>{{ reviewAsset.summary }}</p>
              </div>

              <div v-if="reviewAsset.description" class="review-section">
                <h4>详细描述</h4>
                <pre>{{ reviewAsset.description }}</pre>
              </div>

              <div v-if="reviewAsset.tags?.length || reviewAsset.scenario_tags?.length" class="review-section">
                <h4>标签</h4>
                <div class="review-tags">
                  <span v-for="tag in (reviewAsset.tags || [])" :key="'t-' + tag" class="review-tag">{{ tag }}</span>
                  <span v-for="tag in (reviewAsset.scenario_tags || [])" :key="'s-' + tag" class="review-tag review-tag-scenario">{{ tag }}</span>
                </div>
              </div>

              <div class="review-links">
                <a v-if="reviewAsset.demo_url" :href="reviewAsset.demo_url" target="_blank" rel="noopener" class="review-link">演示</a>
                <a v-if="reviewAsset.doc_url" :href="reviewAsset.doc_url" target="_blank" rel="noopener" class="review-link">文档</a>
                <a v-if="reviewAsset.source_url" :href="reviewAsset.source_url" target="_blank" rel="noopener" class="review-link">源码</a>
                <a v-if="reviewAsset.template_url" :href="reviewAsset.template_url" target="_blank" rel="noopener" class="review-link">模板</a>
              </div>

              <!-- Review form -->
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

                <div v-if="reviewStatus === 'listed'" class="review-form-row">
                  <label>
                    <input v-model="reviewFeatured" type="checkbox" />
                    设为推荐资产
                  </label>
                  <div v-if="reviewFeatured" class="review-form-sub-row">
                    <label>推荐权重 (数字越大越靠前)</label>
                    <input v-model.number="reviewFeaturedWeight" type="number" min="0" max="9999" />
                  </div>
                </div>

                <div class="review-form-row">
                  <label>审核备注 (可选)</label>
                  <textarea v-model="reviewNote" rows="3" placeholder="给用户的反馈..."></textarea>
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
import AssetCommercePanel from '@/features/bizdecipher/components/assets/AssetCommercePanel.vue'
import { useAppStore } from '@/stores/app'
import {
  adminListCapabilityAssets,
  adminReviewCapabilityAsset,
  type CapabilityAsset,
  type CapabilityAssetReviewPayload,
} from '@/features/bizdecipher/api/bizdecipher'

const appStore = useAppStore()

const loading = ref(false)
const assets = ref<CapabilityAsset[]>([])
const keyword = ref('')
const filterStatus = ref<string>('pending')
const filterType = ref<string>('all')

const typeOptions = [
  { value: 'product_app', label: '产品 / 应用' },
  { value: 'game', label: '游戏' },
  { value: 'workflow', label: '工作流' },
  { value: 'agent', label: 'Agent' },
  { value: 'api_tool', label: 'API / 工具' },
  { value: 'plugin_template', label: '插件 / 模板' },
  { value: 'prompt_solution', label: '提示词 / 方案' },
  { value: 'dataset_report', label: '数据集 / 报告' },
  { value: 'other', label: '其他' },
]

const statusCounts = computed(() => {
  const counts: Record<string, number> = { pending: 0, listed: 0, rejected: 0, draft: 0, archived: 0, delisted: 0 }
  for (const a of assets.value) {
    if (counts[a.status] !== undefined) counts[a.status]++
  }
  return counts
})

async function loadAssets() {
  loading.value = true
  try {
    assets.value = await adminListCapabilityAssets({
      keyword: keyword.value || undefined,
      status: filterStatus.value === 'all' ? undefined : filterStatus.value,
      asset_type: filterType.value === 'all' ? undefined : filterType.value,
      limit: 200,
    })
  } catch {
    appStore.showError('加载资产列表失败')
  } finally {
    loading.value = false
  }
}

// --- review panel ---
const reviewPanelVisible = ref(false)
const reviewAsset = ref<CapabilityAsset | null>(null)
const reviewStatus = ref<'listed' | 'rejected' | 'delisted'>('listed')
const reviewNote = ref('')
const reviewFeatured = ref(false)
const reviewFeaturedWeight = ref(0)
const submitting = ref(false)

function openReviewPanel(asset: CapabilityAsset) {
  reviewAsset.value = asset
  reviewStatus.value = 'listed'
  reviewNote.value = asset.review_note || ''
  reviewFeatured.value = asset.is_featured
  reviewFeaturedWeight.value = asset.featured_weight || 0
  reviewPanelVisible.value = true
}
function closeReviewPanel() {
  reviewPanelVisible.value = false
  reviewAsset.value = null
}

async function handleReview() {
  if (!reviewAsset.value || !reviewStatus.value) return
  submitting.value = true
  try {
    const payload: CapabilityAssetReviewPayload = {
      status: reviewStatus.value,
      review_note: reviewNote.value || undefined,
      is_featured: reviewStatus.value === 'listed' ? reviewFeatured.value : false,
      featured_weight: reviewStatus.value === 'listed' && reviewFeatured.value ? reviewFeaturedWeight.value : 0,
    }
    await adminReviewCapabilityAsset(reviewAsset.value.id, payload)
    appStore.showSuccess('审核操作已完成')
    closeReviewPanel()
    await loadAssets()
  } catch {
    appStore.showError('审核操作失败')
  } finally {
    submitting.value = false
  }
}

// --- helpers ---
function typeLabel(type: string): string {
  const found = typeOptions.find(o => o.value === type)
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
function formatDate(dateStr?: string | null): string {
  if (!dateStr) return '--'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return '--'
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

onMounted(() => {
  loadAssets()
})
</script>

<style scoped>
.admin-capability-assets-page {
  --admin-accent: var(--module-accent, var(--zc-accent));
  --admin-accent-ink: var(--module-accent-ink, var(--zc-accent-ink));
  --admin-canvas: var(--module-canvas, var(--zc-bg));
  --admin-panel: var(--module-panel, var(--zc-card-strong));
  --admin-text: var(--module-text, var(--zc-text-strong));
  --admin-muted: var(--module-muted, var(--zc-muted));
  --admin-subtle: var(--module-subtle, var(--zc-subtle));
  --admin-line: var(--module-line, var(--zc-line));
  --admin-line-strong: var(--module-line-strong, var(--zc-line-strong));
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

/* Filter */
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
  min-width: 200px;
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

/* Stats */
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

/* Table */
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
.col-asset { min-width: 260px; }
.cell-asset { display: flex; gap: 10px; align-items: center; }
.cell-asset-cover {
  width: 40px;
  height: 40px;
  border-radius: 6px;
  overflow: hidden;
  flex-shrink: 0;
  background: color-mix(in srgb, var(--admin-panel) 72%, var(--admin-canvas));
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 700;
  color: var(--admin-subtle);
}
.cell-asset-cover img { width: 100%; height: 100%; object-fit: cover; }
.cell-asset-info { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.cell-asset-info strong { font-size: 13px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cell-asset-info span { font-size: 11px; color: var(--admin-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 220px; }
.col-author { font-size: 12px; color: var(--admin-muted); }
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
.cell-featured-yes { font-size: 11px; color: var(--admin-accent); font-weight: 600; }
.cell-featured-no { font-size: 11px; color: var(--admin-subtle); }

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

/* Review panel */
.review-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: flex-end;
  z-index: 1000;
}
.review-panel {
  width: min(520px, 90vw);
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
.review-cover { width: 100%; border-radius: 10px; overflow: hidden; }
.review-cover img { width: 100%; display: block; }
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
.review-tag-scenario { background: rgba(99, 102, 241, 0.1); color: var(--admin-accent); }
.review-links { display: flex; gap: 8px; flex-wrap: wrap; }
.review-link {
  font-size: 12px;
  padding: 6px 12px;
  border-radius: 6px;
  background: var(--admin-panel);
  color: var(--admin-text);
  border: 1px solid var(--admin-line);
  text-decoration: none;
}
.review-link:hover { border-color: var(--admin-accent); }

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
.review-form-row label { font-size: 12px; font-weight: 500; color: var(--admin-muted); display: flex; align-items: center; gap: 6px; cursor: pointer; }
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
.review-form-sub-row { display: flex; flex-direction: column; gap: 4px; margin-top: 8px; padding-left: 24px; }

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
.review-action-reject.active { border-color: #ef4444; background: rgba(239, 68, 68, 0.1); color: #ef4444; }
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

/* Transitions */
.drawer-slide-enter-active, .drawer-slide-leave-active { transition: opacity 0.2s; }
.drawer-slide-enter-active .review-panel, .drawer-slide-leave-active .review-panel { transition: transform 0.25s ease; }
.drawer-slide-enter-from, .drawer-slide-leave-to { opacity: 0; }
.drawer-slide-enter-from .review-panel, .drawer-slide-leave-to .review-panel { transform: translateX(100%); }

.animate-spin { animation: spin 1s linear infinite; }
@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
</style>
